#define _GNU_SOURCE
#include "bridge.h"
#include <stdio.h>
#ifdef _WIN32
#include <windows.h>
#include <tlhelp32.h>
#else
#include <dlfcn.h>
#ifdef __APPLE__
#include <mach-o/dyld.h>
#else
#include <link.h>
#endif
#endif
struct OCRRuntime { void *module; const OrtApi *api; const OrtApiBase *base; OrtEnv *env; OrtMemoryInfo *memory; };
static char *status(OCRRuntime *r, OrtStatus *s) {
 if (!s) return NULL;
 char *message = strdup(r->api->GetErrorMessage(s)); r->api->ReleaseStatus(s); return message;
}
#ifdef _WIN32
static void *load(const char *path) {
 int n = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, NULL, 0);
 if (!n) return NULL;
 wchar_t *wide = calloc(n, sizeof(wchar_t)); if (!wide) return NULL;
 MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, wide, n);
 HMODULE m = LoadLibraryExW(wide, NULL, LOAD_LIBRARY_SEARCH_DEFAULT_DIRS | LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR);
 if (!m && !strchr(path, '\\') && !strchr(path, '/')) m = LoadLibraryW(wide);
 free(wide); return m;
}
static void *loaded(const char *path) {
 int n = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, NULL, 0);
 if (!n) return NULL;
 wchar_t *wide = calloc(n, sizeof(wchar_t)); if (!wide) return NULL;
 MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, wide, n);
 HMODULE m = NULL; GetModuleHandleExW(0, wide, &m); free(wide); return m;
}
static void unload(void *m) { FreeLibrary(m); }
static void *symbol(void *m) { return (void *)GetProcAddress(m, "OrtGetApiBase"); }
#else
static void *load(const char *path) { return dlopen(path, RTLD_NOW | RTLD_LOCAL); }
static void *loaded(const char *path) { return dlopen(path, RTLD_NOW | RTLD_LOCAL | RTLD_NOLOAD); }
static void unload(void *m) { dlclose(m); }
static void *symbol(void *m) { return dlsym(m, "OrtGetApiBase"); }
#endif
// Inspect only existing modules. Alias paths resolving to the same entry point
// count once. The acquired module reference is released after inspection.
struct found { char *path; void *entry; int multiple; };
static void inspect(struct found *f, const char *path) {
 if (!path || !*path) return;
 void *m = loaded(path);
 if (!m) return;
 void *entry = symbol(m);
 if (entry) {
  if (f->entry && f->entry != entry) f->multiple = 1;
  if (!f->entry) { f->entry = entry; f->path = strdup(path); }
 }
 unload(m);
}
#if !defined(_WIN32) && !defined(__APPLE__)
struct paths { char **items; size_t count; int failed; };
static int visit(struct dl_phdr_info *info, size_t size, void *data) {
 (void)size; struct paths *paths = data;
 if (!info->dlpi_name || !*info->dlpi_name) return 0;
 char **items = realloc(paths->items, (paths->count + 1) * sizeof(char *));
 if (!items) { paths->failed = 1; return 1; }
 paths->items = items;
 char *name = strdup(info->dlpi_name);
 if (!name) { paths->failed = 1; return 1; }
 paths->items[paths->count++] = name; return 0;
}
#endif
static char *scan(struct found *result) {
 struct found f = {0};
#ifdef _WIN32
 HANDLE snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPMODULE | TH32CS_SNAPMODULE32, GetCurrentProcessId());
 if (snapshot == INVALID_HANDLE_VALUE) return strdup("oneocr: cannot enumerate loaded modules");
 MODULEENTRY32W e = {0}; e.dwSize = sizeof(e);
 if (Module32FirstW(snapshot, &e)) do {
  char path[32768]; if (WideCharToMultiByte(CP_UTF8, 0, e.szExePath, -1, path, sizeof(path), NULL, NULL)) inspect(&f, path);
 } while (Module32NextW(snapshot, &e));
 CloseHandle(snapshot);
#elif defined(__APPLE__)
 uint32_t n = _dyld_image_count();
 for (uint32_t i = 0; i < n; ++i) inspect(&f, _dyld_get_image_name(i));
#else
 // Do not call dlopen while dl_iterate_phdr holds the loader's iteration
 // lock. Copy names first, then acquire references outside that callback.
 struct paths paths = {0}; dl_iterate_phdr(visit, &paths);
 for (size_t i = 0; i < paths.count; ++i) {
  if (!paths.failed) inspect(&f, paths.items[i]);
  free(paths.items[i]);
 }
 free(paths.items);
 if (paths.failed) { free(f.path); return strdup("oneocr: cannot enumerate loaded libraries"); }
#endif
 *result = f; return NULL;
}
char *ocr_loaded_path(char **out) {
 *out = NULL; struct found f = {0}; char *err = scan(&f); if (err) return err;
 if (f.multiple) { free(f.path); return strdup("oneocr: multiple ONNX Runtime libraries are loaded; select RuntimeLibrary explicitly"); }
 *out = f.path; return NULL;
}
char *ocr_runtime_open(const char *path, OCRRuntime **out) {
 *out = NULL;
 struct found f = {0}; char *scan_error = scan(&f); if (scan_error) return scan_error;
 if (f.entry) {
  void *selected = loaded(path);
  int valid = selected && symbol(selected);
  if (selected) unload(selected);
  free(f.path);
  if (!valid) return strdup("oneocr: another ONNX Runtime is already loaded; reuse it instead of loading a second copy");
 }
 OCRRuntime *r = calloc(1, sizeof(*r));
 if (!r) return strdup("oneocr: out of memory");
 r->module = load(path);
 if (!r->module) {
#ifdef _WIN32
  char buf[128]; snprintf(buf, sizeof(buf), "oneocr: cannot load ONNX Runtime (Windows error %lu)", GetLastError());
  free(r); return strdup(buf);
#else
  const char *detail = dlerror(); char *err = strdup(detail ? detail : "oneocr: cannot load ONNX Runtime"); free(r); return err;
#endif
 }
 const OrtApiBase *(ORT_API_CALL *get)(void) = (const OrtApiBase *(ORT_API_CALL *)(void))symbol(r->module);
 if (!get) { ocr_runtime_close(r); return strdup("oneocr: library does not export OrtGetApiBase"); }
 r->base = get(); r->api = r->base->GetApi(26);
 if (!r->api) { ocr_runtime_close(r); return strdup("oneocr: loaded ONNX Runtime does not support C API 26 (requires ORT 1.26 or newer)"); }
 char *err = status(r, r->api->CreateEnv(ORT_LOGGING_LEVEL_ERROR, "oneocr", &r->env));
 if (!err) err = status(r, r->api->CreateCpuMemoryInfo(OrtArenaAllocator, OrtMemTypeDefault, &r->memory));
 if (err) { ocr_runtime_close(r); return err; }
 *out = r; return NULL;
}
int ocr_runtime_matches(OCRRuntime *r, const char *path) {
 void *m = loaded(path); if (!m) return 0;
 const OrtApiBase *(ORT_API_CALL *get)(void) = (const OrtApiBase *(ORT_API_CALL *)(void))symbol(m);
 int same = get && get() == r->base; unload(m); return same;
}
void ocr_runtime_close(OCRRuntime *r) {
 if (!r) return;
 if (r->memory) r->api->ReleaseMemoryInfo(r->memory);
 if (r->env) r->api->ReleaseEnv(r->env);
 if (r->module) unload(r->module);
 free(r);
}
const char *ocr_runtime_version(OCRRuntime *r) { return r->base->GetVersionString(); }
#define CHECK(call) do { err = status(r, (call)); if (err) goto done; } while(0)
char *ocr_session(OCRRuntime *r, const void *data, size_t size, int threads, int android_arm64, OrtSession **out) {
 OrtSessionOptions *o = NULL; char *err = NULL; *out = NULL;
 CHECK(r->api->CreateSessionOptions(&o));
 CHECK(r->api->SetIntraOpNumThreads(o, threads));
 CHECK(r->api->SetInterOpNumThreads(o, 1));
 CHECK(r->api->SetSessionExecutionMode(o, ORT_SEQUENTIAL));
 CHECK(r->api->SetSessionLogSeverityLevel(o, 3));
 CHECK(r->api->AddSessionConfigEntry(o, "session.intra_op.allow_spinning", "0"));
 CHECK(r->api->AddSessionConfigEntry(o, "session.inter_op.allow_spinning", "0"));
 if (android_arm64) CHECK(r->api->AddSessionConfigEntry(o, "mlas.disable_kleidiai", "1"));
 CHECK(r->api->CreateSessionFromArray(r->env, data, size, o, out));
 done: if (o) r->api->ReleaseSessionOptions(o); return err;
}
void ocr_session_close(OCRRuntime *r, OrtSession *s) { r->api->ReleaseSession(s); }
char *ocr_tensor(OCRRuntime *r, void *data, size_t bytes, const int64_t *shape, size_t rank, int type, OrtValue **out) { return status(r, r->api->CreateTensorWithDataAsOrtValue(r->memory, data, bytes, shape, rank, type, out)); }
void ocr_value_close(OCRRuntime *r, OrtValue *v) { r->api->ReleaseValue(v); }
char *ocr_tensor_info(OCRRuntime *r, OrtValue *v, void **data, int64_t *shape, size_t *rank, size_t *count, int *type) {
 OrtTensorTypeAndShapeInfo *info = NULL; char *err = NULL; ONNXTensorElementDataType dtype;
 CHECK(r->api->GetTensorTypeAndShape(v, &info));
 CHECK(r->api->GetDimensionsCount(info, rank));
 if (*rank > 16) { err = strdup("oneocr: tensor rank exceeds 16"); goto done; }
 CHECK(r->api->GetDimensions(info, shape, *rank));
 CHECK(r->api->GetTensorShapeElementCount(info, count));
 CHECK(r->api->GetTensorElementType(info, &dtype)); *type = dtype;
 CHECK(r->api->GetTensorMutableData(v, data));
 done: if (info) r->api->ReleaseTensorTypeAndShapeInfo(info); return err;
}
char *ocr_run_options(OCRRuntime *r, OrtRunOptions **out) { return status(r, r->api->CreateRunOptions(out)); }
char *ocr_terminate(OCRRuntime *r, OrtRunOptions *o) { return status(r, r->api->RunOptionsSetTerminate(o)); }
void ocr_run_options_close(OCRRuntime *r, OrtRunOptions *o) { r->api->ReleaseRunOptions(o); }
char *ocr_run(OCRRuntime *r, OrtSession *s, OrtRunOptions *o, const char *const *in_names, const OrtValue *const *inputs, size_t n, const char *const *out_names, size_t m, OrtValue **outputs) { return status(r, r->api->Run(s, o, in_names, inputs, n, out_names, m, outputs)); }

char *ocr_module_path(void) {
#ifdef _WIN32
 HMODULE module = NULL;
 if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS, (LPCWSTR)(uintptr_t)&ocr_module_path, &module)) return NULL;
 wchar_t wide[32768]; DWORD size = GetModuleFileNameW(module, wide, 32768); FreeLibrary(module);
 if (!size || size == 32768) return NULL;
 int n = WideCharToMultiByte(CP_UTF8, 0, wide, -1, NULL, 0, NULL, NULL);
 char *path = malloc(n); if (!path) return NULL;
 WideCharToMultiByte(CP_UTF8, 0, wide, -1, path, n, NULL, NULL); return path;
#else
 Dl_info info;
 if (!dladdr((void *)&ocr_module_path, &info) || !info.dli_fname) return NULL;
 return strdup(info.dli_fname);
#endif
}
