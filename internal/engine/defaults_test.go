package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultModelNeverSelectsDevelopmentPackage(t *testing.T) {
	t.Setenv("ONEOCR_MODEL", "")
	t.Setenv("ONEOCR_HOME", t.TempDir())
	t.Chdir(t.TempDir())
	if err := os.WriteFile("oneocr-extended.ocrpack", []byte("development-only"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := DefaultConfig(); err == nil {
		t.Fatal("selected the development package as the default model")
	}
}

func TestNativeDefaultInstallAndOpen(t *testing.T) {
	useRepositoryFixtures(t)
	library := os.Getenv("ONEOCR_RUNTIME")
	if library == "" {
		t.Skip("set ONEOCR_RUNTIME for native integration")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "models", DefaultModelName)); err != nil {
		t.Skip("requires the repository's default model")
	}
	t.Setenv("ONEOCR_MODEL", "")
	t.Setenv("ONEOCR_HOME", t.TempDir())
	installed, err := Install(InstallOptions{RuntimeLibrary: library})
	if err != nil {
		t.Fatal(err)
	}
	if installed.Profile != "cjk-en" {
		t.Fatal("installed a nondefault model", installed.Profile)
	}
	t.Setenv("ONEOCR_RUNTIME", "")
	// The repository-local model must still find the installed runtime.
	local, err := Open(Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { local.Close() })
	actual, actualErr := os.Stat(local.config.RuntimeLibrary)
	expected, expectedErr := os.Stat(installed.RuntimeLibrary)
	if actualErr != nil || expectedErr != nil || !os.SameFile(actual, expected) {
		t.Fatalf("local model did not reuse the installed runtime: %q != %q", local.config.RuntimeLibrary, installed.RuntimeLibrary)
	}
	if err = local.Close(); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	engine, err := Open(Config{Threads: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	result, err := engine.Recognize(context.Background(), FromFile(filepath.Join(root, "testdata", "CJK.png")), Options{})
	if err != nil || result.Text != "你好世界 日本語テスト 한국어 123" {
		t.Fatal(result.Text, err)
	}
	if engine.Diagnostics().Threads != 1 {
		t.Fatal("default discovery overwrote caller options")
	}
}
