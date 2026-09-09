package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/shiyori/oneocr-native/internal/installlock"
	ort "github.com/shiyori/oneocr-native/internal/ort"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

type Installation struct {
	Schema         string `json:"schema"`
	ModelPath      string `json:"model_path,omitempty"`
	BundleDir      string `json:"bundle_dir,omitempty"`
	RuntimeLibrary string `json:"runtime_library,omitempty"`
	RuntimeSHA256  string `json:"runtime_sha256,omitempty"`
	Platform       string `json:"platform"`
	ModelSHA256    string `json:"model_sha256"` // original OneModel provenance
	PackageSHA256  string `json:"package_sha256,omitempty"`
	Profile        string `json:"profile,omitempty"`
}
type InstallOptions struct {
	ModelPath, RuntimeLibrary, Home, BundleDir string
	Profile                                    string // original OneModel conversion; defaults to cjk-en
	SourceDirectory                            string // offline model metadata and runtime archives
	Offline                                    bool   // never download missing assets
}

func (i Installation) Config() Config {
	return Config{ModelPath: i.ModelPath, BundleDir: i.BundleDir, RuntimeLibrary: i.RuntimeLibrary}
}

func DefaultHome() (string, error) {
	if value := os.Getenv("ONEOCR_HOME"); value != "" {
		return filepath.Abs(value)
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "oneocr"), nil
}
func installationHome(home string) (string, error) {
	if home == "" {
		return DefaultHome()
	}
	return filepath.Abs(home)
}
func fileHash(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func copyVerifiedFile(source, destination, expected string) error {
	if actual, err := fileHash(destination); err == nil && actual == expected {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(destination), ".install-")
	if err != nil {
		return err
	}
	temp := out.Name()
	defer os.Remove(temp)
	defer out.Close()
	h := sha256.New()
	if _, err = io.Copy(io.MultiWriter(out, h), in); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != expected {
		return fmt.Errorf("source changed while copying: %s", filepath.Base(source))
	}
	if err = out.Chmod(0755); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	return os.Rename(temp, destination)
}
func isPackageFile(filename string) (bool, error) {
	f, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer f.Close()
	header := make([]byte, 8)
	_, err = io.ReadFull(f, header)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}
	return string(header) == packageMagic, nil
}

// Install prepares the model and runtime from local resources, this version's
// Release, or pinned upstream runtime archives. Recognition never downloads.
func Install(options InstallOptions) (Installation, error) {
	result := Installation{Schema: "oneocr.install.v2", Platform: runtime.GOOS + "-" + runtime.GOARCH}
	referenceRuntime := options.RuntimeLibrary != "" || os.Getenv("ONEOCR_RUNTIME") != ""
	home, err := installationHome(options.Home)
	if err != nil {
		return result, err
	}
	if err = os.MkdirAll(home, 0755); err != nil {
		return result, err
	}
	unlock, err := installlock.Acquire(filepath.Join(home, "install.lock"))
	if err != nil {
		return result, err
	}
	defer unlock()
	temporary, err := os.MkdirTemp(home, ".prepare-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(temporary)
	if err = prepareInstallResources(&options, temporary); err != nil {
		return result, err
	}
	incomingHash, err := fileHash(options.ModelPath)
	if err != nil {
		return result, err
	}
	packed, err := isPackageFile(options.ModelPath)
	if err != nil {
		return result, err
	}
	if packed && options.BundleDir != "" {
		return result, fmt.Errorf("BundleDir is only for legacy OneModel installation; packages stay as one file")
	}
	var info *PackageInfo
	packageSource := options.ModelPath
	if !packed && options.BundleDir != "" {
		b, err := ExportModel(options.ModelPath, options.BundleDir)
		if err != nil {
			return result, err
		}
		result.BundleDir = b.Directory()
		result.ModelSHA256 = b.SourceSHA256
	} else {
		if packed {
			info, err = ReadPackage(packageSource)
			if err != nil {
				return result, err
			}
			if options.Profile != "" && options.Profile != info.Profile {
				return result, fmt.Errorf("package profile is %s; install cannot change it", info.Profile)
			}
		} else {
			profile := options.Profile
			if profile == "" {
				profile = "cjk-en"
			}
			if _, err = profileScripts(profile); err != nil {
				return result, err
			}
			temp, err := os.MkdirTemp("", "oneocr-install-model-")
			if err != nil {
				return result, err
			}
			defer os.RemoveAll(temp)
			packageSource = filepath.Join(temp, "model.ocrpack")
			info, err = Pack(PackOptions{ModelPath: options.ModelPath, Profile: profile, Output: packageSource})
			if err != nil {
				return result, err
			}
			incomingHash, err = fileHash(packageSource)
			if err != nil {
				return result, err
			}
		}
		result.Profile = info.Profile
		result.PackageSHA256 = incomingHash
		result.ModelSHA256 = info.Bundle.SourceSHA256
		result.ModelPath = filepath.Join(home, "models", incomingHash, "oneocr-"+info.Profile+".ocrpack")
		if err = copyVerifiedFile(packageSource, result.ModelPath, incomingHash); err != nil {
			return result, err
		}
	}
	library, err := resolveRuntimeLibrary(options.RuntimeLibrary, options.ModelPath, options.BundleDir)
	if err != nil {
		return result, err
	}
	if !filepath.IsAbs(library) {
		return result, fmt.Errorf("oneocr: no usable runtime was prepared; run oneocr install")
	}
	runtimeHash, err := fileHash(library)
	if err != nil {
		return result, err
	}
	loaded, discoveryErr := ort.LoadedPath()
	result.RuntimeSHA256 = runtimeHash
	// resolveRuntimeLibrary has already selected a path. Keep it when loaded
	// modules are ambiguous; Open below validates an explicit selection without
	// copying it or introducing a second runtime.
	if loaded != "" || discoveryErr != nil || referenceRuntime {
		result.RuntimeLibrary = library
	} else {
		runtimeDir := filepath.Join(home, "runtime", result.Platform, runtimeHash)
		result.RuntimeLibrary = filepath.Join(runtimeDir, runtimeLibraryName())
		manifestFile := filepath.Join(filepath.Dir(library), "runtime.json")
		if _, statErr := os.Stat(manifestFile); statErr == nil {
			record, err := readRuntimeManifest(filepath.Dir(library), result.Platform)
			if err != nil {
				return result, err
			}
			for _, file := range record.Files {
				if err = copyVerifiedFile(filepath.Join(filepath.Dir(library), filepath.FromSlash(file.File)), filepath.Join(runtimeDir, filepath.FromSlash(file.File)), file.SHA256); err != nil {
					return result, err
				}
			}
			manifestHash, err := fileHash(manifestFile)
			if err != nil {
				return result, err
			}
			if err = copyVerifiedFile(manifestFile, filepath.Join(runtimeDir, "runtime.json"), manifestHash); err != nil {
				return result, err
			}
		} else if !os.IsNotExist(statErr) {
			return result, statErr
		} else if err = copyVerifiedFile(library, result.RuntimeLibrary, runtimeHash); err != nil {
			return result, err
		}
	}
	engine, err := Open(result.Config())
	if err != nil {
		return result, err
	}
	if err = engine.Close(); err != nil {
		return result, err
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return result, err
	}
	if err = os.MkdirAll(home, 0755); err != nil {
		return result, err
	}
	f, err := os.CreateTemp(home, ".config-")
	if err != nil {
		return result, err
	}
	configTemporary := f.Name()
	defer os.Remove(configTemporary)
	defer f.Close()
	if _, err = f.Write(append(encoded, '\n')); err != nil {
		return result, err
	}
	if err = f.Close(); err != nil {
		return result, err
	}
	if err = os.Rename(configTemporary, filepath.Join(home, "config.json")); err != nil {
		return result, err
	}
	return result, nil
}
func LoadInstallation(home string) (Installation, error) {
	var result Installation
	home, err := installationHome(home)
	if err != nil {
		return result, err
	}
	f, err := os.Open(filepath.Join(home, "config.json"))
	if err != nil {
		return result, fmt.Errorf("oneocr: not installed; run oneocr install: %w", err)
	}
	data, err := readLimited(f, 1024*1024)
	f.Close()
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	if (result.Schema != "oneocr.install.v1" && result.Schema != "oneocr.install.v2") || result.Platform != runtime.GOOS+"-"+runtime.GOARCH {
		return result, fmt.Errorf("installation is for a different platform or version")
	}
	if (result.ModelPath == "") == (result.BundleDir == "") || !validHash(result.ModelSHA256) {
		return result, fmt.Errorf("invalid installed model configuration")
	}
	var hash string
	if result.RuntimeLibrary != "" {
		hash, err = fileHash(result.RuntimeLibrary)
		if err != nil {
			return result, err
		}
		if hash != result.RuntimeSHA256 {
			return result, fmt.Errorf("installed runtime checksum mismatch")
		}
	}
	if result.ModelPath != "" {
		hash, err = fileHash(result.ModelPath)
		if err != nil {
			return result, err
		}
		if hash != result.PackageSHA256 {
			return result, fmt.Errorf("installed model package checksum mismatch")
		}
	}
	return result, nil
}
