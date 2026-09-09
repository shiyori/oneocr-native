package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// Use an official archive obtained separately; this check never downloads data.
func TestAndroidOfficialArchiveInstallation(t *testing.T) {
	source := os.Getenv("ONEOCR_UPSTREAM_DIR")
	if source == "" {
		t.Skip("set ONEOCR_UPSTREAM_DIR to the official Android AAR directory")
	}
	useRepositoryFixtures(t)
	model, err := filepath.Abs("models/oneocr-cjk-en.ocrpack")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ONEOCR_MODEL", model)
	t.Setenv("ONEOCR_HOME", t.TempDir())
	project := t.TempDir()
	if err = os.WriteFile(filepath.Join(project, "build.gradle.kts"), []byte("plugins { id(\"com.android.application\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := InstallAndroid(AndroidInstallOptions{ProjectDirectory: project, SourceDirectory: source, Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RuntimeLibraries) != 2 || result.HostRuntime {
		t.Fatal("missing official runtime libraries", result)
	}
	if _, err = ReadPackage(result.ModelPath); err != nil {
		t.Fatal(err)
	}
	for _, abi := range []string{"arm64-v8a", "x86_64"} {
		if err = validateAndroidLibrary(filepath.Join(project, "src/main/jniLibs", abi, "libonnxruntime.so"), abi); err != nil {
			t.Fatal(err)
		}
	}
	result, err = InstallAndroid(AndroidInstallOptions{ProjectDirectory: project, Offline: true})
	if err != nil || !result.HostRuntime || len(result.RuntimeLibraries) != 2 {
		t.Fatal("repeat installation did not reuse prepared dependencies", result, err)
	}
}
