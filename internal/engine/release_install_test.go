package engine

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testReleaseManifest(t *testing.T, directory string, manifest releaseManifest) {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "release-manifest.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "SHA256SUMS"), []byte(fmt.Sprintf("%x  release-manifest.json\n", sha256.Sum256(data))), 0644); err != nil {
		t.Fatal(err)
	}
}
func TestReleaseMetadataAndDownloads(t *testing.T) {
	sourceDir := t.TempDir()
	source := releaseSource{directory: sourceDir, offline: true}
	data := []byte("asset bytes")
	asset := releaseAsset{Name: "model.ocrpack", Kind: "model", Bytes: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	manifest := releaseManifest{Schema: "oneocr.release.v1", Version: Version, Assets: []releaseAsset{asset}}
	testReleaseManifest(t, sourceDir, manifest)
	if _, err := source.manifest(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, asset.Name), data, 0644); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	path, err := source.download(asset, destination)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(data) {
		t.Fatal("asset was not installed", err)
	}
	if err = os.WriteFile(filepath.Join(sourceDir, asset.Name), []byte("broken"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = source.download(asset, t.TempDir()); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatal("accepted damaged download", err)
	}
	if _, err = source.download(asset, destination); err != nil {
		t.Fatal("did not reuse verified local asset", err)
	}
	if err = os.WriteFile(filepath.Join(sourceDir, "release-manifest.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = source.manifest(); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal("accepted corrupt metadata", err)
	}
	manifest.Version = "9.0.0"
	testReleaseManifest(t, sourceDir, manifest)
	if _, err = source.manifest(); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatal("accepted wrong version", err)
	}
	manifest.Version = Version
	manifest.Assets = append(manifest.Assets, asset)
	testReleaseManifest(t, sourceDir, manifest)
	if _, err = source.manifest(); err == nil {
		t.Fatal("accepted duplicate asset")
	}
	if _, err = (releaseSource{offline: true}).open("model.ocrpack"); err == nil {
		t.Fatal("offline source attempted a download")
	}
}
func TestRuntimeArchiveRejectsUnsafeEntries(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "folder\\escape", "duplicate", "symlink"} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			archive := filepath.Join(directory, "runtime.zip")
			file, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			writer := zip.NewWriter(file)
			header := &zip.FileHeader{Name: name, Method: zip.Store}
			if name == "symlink" {
				header.SetMode(os.ModeSymlink | 0777)
			}
			entry, err := writer.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			entry.Write([]byte("bad"))
			if name == "duplicate" {
				entry, err = writer.Create("duplicate")
				if err != nil {
					t.Fatal(err)
				}
				entry.Write([]byte("bad"))
			}
			writer.Close()
			file.Close()
			target := filepath.Join(directory, "output")
			os.Mkdir(target, 0755)
			if err = extractReleaseArchive(archive, target); err == nil {
				t.Fatal("accepted unsafe archive")
			}
			if _, err = os.Stat(filepath.Join(directory, "escape")); !os.IsNotExist(err) {
				t.Fatal("archive escaped target directory")
			}
		})
	}
}
func TestRuntimeManifestArchitecture(t *testing.T) {
	directory := t.TempDir()
	record := runtimeManifest{Schema: "oneocr.runtime.v1", Platform: "wrong-platform", Version: ManagedRuntimeVersion, Library: runtimeLibraryName()}
	data, _ := json.Marshal(record)
	os.WriteFile(filepath.Join(directory, "runtime.json"), data, 0644)
	if _, err := readRuntimeManifest(directory, "darwin-arm64"); err == nil || !strings.Contains(err.Error(), "platform") {
		t.Fatal("accepted wrong architecture", err)
	}
}
func TestInstallCorruptionLeavesConfigurationUntouched(t *testing.T) {
	home := t.TempDir()
	source := t.TempDir()
	t.Chdir(t.TempDir())
	t.Setenv("ONEOCR_HOME", home)
	t.Setenv("ONEOCR_MODEL", "")
	t.Setenv("ONEOCR_RUNTIME", "")
	before := []byte("existing configuration")
	if err := os.WriteFile(filepath.Join(home, "config.json"), before, 0644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(source, "release-manifest.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(source, "SHA256SUMS"), []byte("invalid"), 0644)
	if _, err := Install(InstallOptions{Home: home, SourceDirectory: source, Offline: true}); err == nil {
		t.Fatal("accepted damaged source")
	}
	after, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil || string(after) != string(before) {
		t.Fatal("failure changed installed configuration", err)
	}
}
func TestAndroidHostDependencyDetection(t *testing.T) {
	module := filepath.Join(t.TempDir(), "app")
	os.MkdirAll(module, 0755)
	if !androidDeclaresRuntime(module, `implementation("com.microsoft.onnxruntime:onnxruntime-android:1.26.0")`) {
		t.Fatal("direct host runtime not detected")
	}
	catalog := filepath.Join(filepath.Dir(module), "gradle")
	os.Mkdir(catalog, 0755)
	os.WriteFile(filepath.Join(catalog, "libs.versions.toml"), []byte(`[libraries]
ort-android = { module = "com.microsoft.onnxruntime:onnxruntime-android", version = "1.26.0" }
`), 0644)
	if !androidDeclaresRuntime(module, "implementation(libs.ort.android)") {
		t.Fatal("catalog runtime not detected")
	}
	if androidDeclaresRuntime(module, "implementation(libs.other)") {
		t.Fatal("unused catalog entry treated as host runtime")
	}
}
