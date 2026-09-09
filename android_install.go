package oneocr

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shiyori/oneocr-native/internal/installlock"
)

type AndroidInstallOptions struct {
	ProjectDirectory string
	SourceDirectory  string
	Offline          bool
}
type AndroidInstallation struct {
	ProjectDirectory string   `json:"project_directory"`
	ModelPath        string   `json:"model_path"`
	RuntimeLibraries []string `json:"runtime_libraries"`
	HostRuntime      bool     `json:"host_runtime"`
}

// InstallAndroid prepares an app module without replacing existing resources.
func InstallAndroid(options AndroidInstallOptions) (AndroidInstallation, error) {
	result := AndroidInstallation{RuntimeLibraries: []string{}}
	module, err := filepath.Abs(options.ProjectDirectory)
	if err != nil {
		return result, err
	}
	result.ProjectDirectory = module
	var script string
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		if data, e := os.ReadFile(filepath.Join(module, name)); e == nil {
			script = string(data)
			break
		}
	}
	if script == "" {
		return result, fmt.Errorf("oneocr: --android-project must identify the app module containing build.gradle(.kts)")
	}
	home, err := DefaultHome()
	if err != nil {
		return result, err
	}
	lockDirectory := filepath.Join(home, "locks")
	if err = os.MkdirAll(lockDirectory, 0755); err != nil {
		return result, err
	}
	unlock, err := installlock.Acquire(filepath.Join(lockDirectory, fmt.Sprintf("android-%x.lock", sha256.Sum256([]byte(module)))))
	if err != nil {
		return result, err
	}
	defer unlock()
	temporary, err := os.MkdirTemp(module, ".oneocr-prepare-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(temporary)
	result.ModelPath = filepath.Join(module, "src/main/assets", DefaultModelName)
	model := result.ModelPath
	if _, err = os.Stat(model); os.IsNotExist(err) {
		if defaults, e := DefaultConfig(); e == nil {
			model = defaults.ModelPath
		} else {
			model = ""
		}
	} else if err != nil {
		return result, err
	}
	host := androidDeclaresRuntime(module, script)
	result.HostRuntime = host
	missing := []string{}
	for _, abi := range []string{"arm64-v8a", "x86_64"} {
		path := filepath.Join(module, "src/main/jniLibs", abi, "libonnxruntime.so")
		if _, err = os.Stat(path); err == nil {
			if err = validateAndroidLibrary(path, abi); err != nil {
				return result, err
			}
			result.HostRuntime = true
			result.RuntimeLibraries = append(result.RuntimeLibraries, path)
		} else if !os.IsNotExist(err) {
			return result, err
		} else if !host {
			missing = append(missing, abi)
		}
	}
	source := releaseSource{directory: options.SourceDirectory, offline: options.Offline}
	var manifest releaseManifest
	if model == "" || len(missing) > 0 {
		manifest, err = source.manifest()
		if err != nil {
			return result, err
		}
	}
	if model == "" {
		asset, e := manifest.asset("model", "")
		if e != nil {
			return result, e
		}
		model, err = source.download(asset, temporary)
		if err != nil {
			return result, err
		}
	}
	info, err := ReadPackage(model)
	if err != nil {
		return result, err
	}
	if info.Profile != "cjk-en" {
		return result, fmt.Errorf("oneocr: Android preparation requires the default model")
	}
	copies := map[string]string{result.ModelPath: model}
	if len(missing) > 0 {
		asset, e := manifest.asset("runtime", "android")
		if e != nil {
			return result, e
		}
		archive, e := source.download(asset, temporary)
		if e != nil {
			return result, e
		}
		directory := filepath.Join(temporary, "runtime")
		if err = os.Mkdir(directory, 0755); err != nil {
			return result, err
		}
		if err = extractReleaseArchive(archive, directory); err != nil {
			return result, err
		}
		for _, abi := range missing {
			library := filepath.Join(directory, "jni", abi, "libonnxruntime.so")
			if err = validateAndroidLibrary(library, abi); err != nil {
				return result, err
			}
			destination := filepath.Join(module, "src/main/jniLibs", abi, "libonnxruntime.so")
			copies[destination] = library
			result.RuntimeLibraries = append(result.RuntimeLibraries, destination)
		}
	}
	// Prepare all copies before publishing any new files. Existing files are
	// verified and retained. Hard links publish atomically without overwriting.
	staged := map[string]string{}
	for destination, source := range copies {
		expected, err := fileHash(source)
		if err != nil {
			return result, err
		}
		if actual, e := fileHash(destination); e == nil {
			if actual != expected {
				return result, fmt.Errorf("oneocr: existing Android resource differs: %s", destination)
			}
			continue
		} else if !os.IsNotExist(e) {
			return result, e
		}
		pending, err := os.CreateTemp(temporary, "file-")
		if err != nil {
			return result, err
		}
		pending.Close()
		os.Remove(pending.Name())
		if err = copyVerifiedFile(source, pending.Name(), expected); err != nil {
			return result, err
		}
		staged[destination] = pending.Name()
	}
	created := []string{}
	for destination, pending := range staged {
		if err = os.MkdirAll(filepath.Dir(destination), 0755); err == nil {
			err = os.Link(pending, destination)
		}
		if err != nil {
			for _, path := range created {
				os.Remove(path)
			}
			return result, err
		}
		created = append(created, destination)
	}
	return result, nil
}
func validateAndroidLibrary(path, abi string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	header := make([]byte, 20)
	if _, err = f.ReadAt(header, 0); err != nil {
		return err
	}
	machine := uint16(183)
	if abi == "x86_64" {
		machine = 62
	}
	if string(header[:4]) != "\x7fELF" || header[4] != 2 || header[5] != 1 || binary.LittleEndian.Uint16(header[18:20]) != machine {
		return fmt.Errorf("oneocr: runtime architecture does not match %s", abi)
	}
	return nil
}
func androidDeclaresRuntime(module, script string) bool {
	if strings.Contains(script, "com.microsoft.onnxruntime:onnxruntime-android") || strings.Contains(script, "com.microsoft.onnxruntime:onnxruntime-training-android") {
		return true
	}
	// Resolve the common Gradle version-catalog declaration without invoking
	// Gradle or downloading the host application's other dependencies.
	catalog, _ := os.ReadFile(filepath.Join(filepath.Dir(module), "gradle/libs.versions.toml"))
	for _, line := range strings.Split(string(catalog), "\n") {
		if !strings.Contains(line, "com.microsoft.onnxruntime") || !strings.Contains(line, "onnxruntime-android") {
			continue
		}
		alias, _, ok := strings.Cut(line, "=")
		if ok {
			alias = strings.Trim(strings.TrimSpace(alias), "\"'")
			alias = strings.NewReplacer("-", ".", "_", ".").Replace(alias)
			if strings.Contains(script, "libs."+alias) {
				return true
			}
		}
	}
	archives, _ := filepath.Glob(filepath.Join(module, "libs", "*.aar"))
	for _, path := range archives {
		reader, err := zip.OpenReader(path)
		if err != nil {
			continue
		}
		found := map[string]bool{}
		for _, file := range reader.File {
			if file.Name == "jni/arm64-v8a/libonnxruntime.so" {
				found["arm64-v8a"] = true
			}
			if file.Name == "jni/x86_64/libonnxruntime.so" {
				found["x86_64"] = true
			}
		}
		reader.Close()
		if len(found) == 2 {
			return true
		}
	}
	return false
}
