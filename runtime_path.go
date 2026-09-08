package oneocr

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func runtimeLibraryName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libonnxruntime.dylib"
	case "windows":
		return "onnxruntime.dll"
	default:
		return "libonnxruntime.so"
	}
}

// An explicit path or ONEOCR_RUNTIME overrides discovery. Distributed SDKs
// place lib/ next to bin/ or next to the model. Android uses the app linker
// namespace, which supports libraries stored directly inside the APK.
func resolveRuntimeLibrary(explicit, modelPath, bundleDir string) (string, error) {
	if explicit == "" {
		explicit = os.Getenv("ONEOCR_RUNTIME")
	}
	if explicit != "" {
		if runtime.GOOS == "android" && !strings.ContainsAny(explicit, "/\\") {
			return explicit, nil
		}
		return filepath.Abs(explicit)
	}
	name := runtimeLibraryName()
	if runtime.GOOS == "android" {
		return name, nil
	}
	var roots []string
	if modelPath != "" {
		roots = append(roots, filepath.Dir(modelPath))
	}
	if bundleDir != "" {
		roots = append(roots, bundleDir, filepath.Dir(bundleDir))
	}
	if executable, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(executable), filepath.Dir(filepath.Dir(executable)))
	}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd)
	}
	for _, root := range roots {
		for _, relative := range []string{name, filepath.Join("lib", name), filepath.Join("runtime", runtime.GOOS+"-"+runtime.GOARCH, name)} {
			filename := filepath.Join(root, relative)
			if stat, err := os.Stat(filename); err == nil && stat.Mode().IsRegular() {
				return filepath.Abs(filename)
			}
		}
	}
	if runtime.GOOS == "darwin" {
		return "@rpath/" + name, nil
	}
	return name, nil
}
