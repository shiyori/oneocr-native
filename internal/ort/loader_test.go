package ort

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const fakeRuntimeSource = `
#include "onnxruntime_c_api.h"
static const OrtApi* ORT_API_CALL unsupported(uint32_t version) { (void)version; return NULL; }
static const char* ORT_API_CALL version(void) { return "1.99.fake"; }
static const OrtApiBase base = {unsupported,version};
#ifdef _WIN32
__declspec(dllexport)
#endif
const OrtApiBase* ORT_API_CALL OrtGetApiBase(void) { return &base; }
`
const loaderProbeSource = `
#include "bridge.h"
#include <assert.h>
#ifdef _WIN32
#include <windows.h>
static void* host_load(const char *path) {
 wchar_t wide[32768]; MultiByteToWideChar(CP_UTF8,0,path,-1,wide,32768);
 return LoadLibraryExW(wide,NULL,LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|LOAD_LIBRARY_SEARCH_DEFAULT_DIRS);
}
static void host_close(void *module) { FreeLibrary(module); }
#else
#include <dlfcn.h>
static void* host_load(const char *path) { return dlopen(path,RTLD_NOW|RTLD_LOCAL); }
static void host_close(void *module) { dlclose(module); }
#endif
int main(int argc,char **argv) {
 assert(argc==3 || argc==4);
 void *first=host_load(argv[1]); assert(first);
 char *path=NULL,*error=ocr_loaded_path(&path);assert(!error && path);free(path);
 OCRRuntime *runtime=NULL;
 error=ocr_runtime_open(argv[1],&runtime);assert(error && !runtime && strstr(error,"C API 26"));free(error);
 error=ocr_runtime_open(argv[2],&runtime);assert(error && !runtime && strstr(error,"already loaded"));free(error);
 error=ocr_loaded_path(&path);assert(!error && path);free(path);
 void *second=host_load(argv[2]);assert(second);
 error=ocr_loaded_path(&path);assert(error && !path && strstr(error,"multiple"));free(error);
 error=ocr_runtime_open(argv[1],&runtime);assert(error && !runtime && strstr(error,"C API 26"));free(error);
 if(argc==4){
  void *host=host_load(argv[3]);assert(host);
  error=ocr_runtime_open(argv[3],&runtime);assert(!error && runtime);
  assert(ocr_runtime_matches(runtime,argv[3]));
  ocr_runtime_close(runtime);host_close(host);
 }
 host_close(second);host_close(first);return 0;
}
`

func TestLoadedLibrarySelection(t *testing.T) {
	directory := t.TempDir()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	compiler, err := exec.Command("go", "env", "CC").Output()
	if err != nil {
		t.Fatal(err)
	}
	command := strings.TrimSpace(string(compiler))
	arguments := []string{}
	if _, err = os.Stat(command); err != nil {
		fields := strings.Fields(command)
		command = fields[0]
		arguments = fields[1:]
	}
	build := func(args ...string) {
		t.Helper()
		cmd := exec.Command(command, append(append([]string{}, arguments...), args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("native loader probe build: %v\n%s", err, output)
		}
	}
	source := filepath.Join(directory, "fake.c")
	if err = os.WriteFile(source, []byte(fakeRuntimeSource), 0644); err != nil {
		t.Fatal(err)
	}
	extension, shared := ".so", []string{"-shared", "-fPIC"}
	if runtime.GOOS == "darwin" {
		extension = ".dylib"
		shared = []string{"-dynamiclib", "-fPIC"}
	}
	if runtime.GOOS == "windows" {
		extension = ".dll"
		shared = []string{"-shared"}
	}
	first, second := filepath.Join(directory, "fake-a"+extension), filepath.Join(directory, "fake-b"+extension)
	for _, output := range []string{first, second} {
		args := append(append([]string{}, shared...), "-I", root, source, "-o", output)
		build(args...)
	}
	harness := filepath.Join(directory, "probe.c")
	if err = os.WriteFile(harness, []byte(loaderProbeSource), 0644); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(directory, "probe")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	args := []string{"-I", root, harness, filepath.Join(root, "bridge.c"), "-o", executable}
	if runtime.GOOS == "linux" {
		args = append(args, "-ldl")
	}
	build(args...)
	args = []string{first, second}
	if library := os.Getenv("ONEOCR_RUNTIME"); library != "" {
		args = append(args, library)
	}
	if output, err := exec.Command(executable, args...).CombinedOutput(); err != nil {
		t.Fatalf("loaded runtime policy: %v\n%s", err, output)
	}
}
