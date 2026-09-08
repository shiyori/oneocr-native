package oneocr

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultModelName is the model shared by the public SDKs.
const DefaultModelName = "oneocr-cjk-en.ocrpack"

// DefaultConfig locates the default model in the working directory, beside the
// executable, or in an installation created by oneocr install. It never scans
// for alternate model packages.
func DefaultConfig() (Config, error) {
	if model := os.Getenv("ONEOCR_MODEL"); model != "" {
		return Config{ModelPath: model}, nil
	}
	roots := []string{"."}
	if executable, err := os.Executable(); err == nil {
		directory := filepath.Dir(executable)
		roots = append(roots, directory, filepath.Dir(directory))
	}
	for _, root := range roots {
		for _, relative := range []string{DefaultModelName, filepath.Join("models", DefaultModelName)} {
			candidate := filepath.Join(root, relative)
			if stat, err := os.Stat(candidate); err == nil && stat.Mode().IsRegular() {
				model, err := filepath.Abs(candidate)
				return Config{ModelPath: model}, err
			}
		}
	}
	installed, err := LoadInstallation("")
	if err != nil {
		return Config{}, fmt.Errorf("oneocr: default model %s not found; place it in models/ or run oneocr install: %w", DefaultModelName, err)
	}
	if installed.Profile != "cjk-en" || installed.ModelPath == "" {
		return Config{}, fmt.Errorf("oneocr: install the default %s model", DefaultModelName)
	}
	return installed.Config(), nil
}

func applyDefaultConfig(config *Config) error {
	if config.ModelPath != "" || config.BundleDir != "" {
		return nil
	}
	defaults, err := DefaultConfig()
	if err != nil {
		return err
	}
	config.ModelPath = defaults.ModelPath
	if config.RuntimeLibrary == "" && os.Getenv("ONEOCR_RUNTIME") == "" {
		config.RuntimeLibrary = defaults.RuntimeLibrary
	}
	return nil
}
