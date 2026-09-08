package oneocr

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

type Installation struct {
	Schema         string `json:"schema"`
	ModelPath      string `json:"model_path,omitempty"`
	BundleDir      string `json:"bundle_dir,omitempty"`
	RuntimeLibrary string `json:"runtime_library"`
	RuntimeSHA256  string `json:"runtime_sha256"`
	Platform       string `json:"platform"`
	ModelSHA256    string `json:"model_sha256"` // original OneModel provenance
	PackageSHA256  string `json:"package_sha256,omitempty"`
	Profile        string `json:"profile,omitempty"`
}
type InstallOptions struct {
	ModelPath, RuntimeLibrary, Home, BundleDir string
	Profile                                    string // original OneModel conversion; defaults to cjk-en
}

func (i Installation) Config() Config {
	return Config{ModelPath: i.ModelPath, BundleDir: i.BundleDir, RuntimeLibrary: i.RuntimeLibrary}
}

// OpenInstalled uses an installation without requiring resource/runtime paths.
func OpenInstalled(home string) (*Engine, error) {
	installed, err := LoadInstallation(home)
	if err != nil {
		return nil, err
	}
	return Open(installed.Config())
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

// Install copies one model package and the platform ORT library. Original
// OneModel inputs are converted to cjk-en by default. BundleDir explicitly
// requests the legacy expanded-directory installation for original inputs.
func Install(options InstallOptions) (Installation, error) {
	result := Installation{Schema: "oneocr.install.v2", Platform: runtime.GOOS + "-" + runtime.GOARCH}
	if options.ModelPath == "" {
		defaults, err := DefaultConfig()
		if err != nil {
			return result, err
		}
		options.ModelPath = defaults.ModelPath
		if options.RuntimeLibrary == "" && os.Getenv("ONEOCR_RUNTIME") == "" {
			options.RuntimeLibrary = defaults.RuntimeLibrary
		}
	}
	home, err := installationHome(options.Home)
	if err != nil {
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
		return result, fmt.Errorf("installation requires a runtime file; set --runtime, ONEOCR_RUNTIME, or use the SDK with its lib directory")
	}
	runtimeHash, err := fileHash(library)
	if err != nil {
		return result, err
	}
	result.RuntimeLibrary = filepath.Join(home, "runtime", result.Platform, runtimeHash, runtimeLibraryName())
	result.RuntimeSHA256 = runtimeHash
	if err = copyVerifiedFile(library, result.RuntimeLibrary, runtimeHash); err != nil {
		return result, err
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
	temporary := f.Name()
	defer os.Remove(temporary)
	defer f.Close()
	if _, err = f.Write(append(encoded, '\n')); err != nil {
		return result, err
	}
	if err = f.Close(); err != nil {
		return result, err
	}
	if err = os.Rename(temporary, filepath.Join(home, "config.json")); err != nil {
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
		return result, fmt.Errorf("oneocr: not installed; run oneocr install --runtime ...: %w", err)
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
	hash, err := fileHash(result.RuntimeLibrary)
	if err != nil {
		return result, err
	}
	if hash != result.RuntimeSHA256 {
		return result, fmt.Errorf("installed runtime checksum mismatch")
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
