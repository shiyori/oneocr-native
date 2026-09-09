package oneocr

import (
	"image"

	"github.com/shiyori/oneocr-native/internal/engine"
)

// ErrClosed is returned when an operation uses a closed engine.
var ErrClosed = engine.ErrClosed

const (
	RecognitionConfidenceMethod = engine.RecognitionConfidenceMethod
	Version                     = engine.Version
	ReleaseTag                  = engine.ReleaseTag
	ManagedRuntimeVersion       = engine.ManagedRuntimeVersion
	DefaultModelName            = engine.DefaultModelName
	BundleSchema                = engine.BundleSchema
	PackageSchema               = engine.PackageSchema
	RGB                         = engine.RGB
	RGBA                        = engine.RGBA
	BGRA                        = engine.BGRA
	RGBX                        = engine.RGBX
	BGRX                        = engine.BGRX
	CharactersHan               = engine.CharactersHan
	CharactersKana              = engine.CharactersKana
	CharactersHangul            = engine.CharactersHangul
	CharactersLatin             = engine.CharactersLatin
	CharactersDigits            = engine.CharactersDigits
)

// InstallAndroid prepares an app module without replacing existing resources.
func InstallAndroid(options AndroidInstallOptions) (AndroidInstallation, error) {
	return engine.InstallAndroid(options)
}

// ReadBundle verifies every resource and required pipeline reference.
func ReadBundle(directory string) (*Bundle, error) {
	return engine.ReadBundle(directory)
}

// ExportModel exports all 67 resources of the validated model profile, without
// needing Python or ONNX Runtime. Resource counts come from the model itself.
// Existing valid bundles of the same source are reused; other targets are not overwritten.
func ExportModel(modelPath, destination string) (*Bundle, error) {
	return engine.ExportModel(modelPath, destination)
}

// ArchiveBundle packages an already verified bundle. Native runtime binaries
// are intentionally not included: each platform supplies its own ORT library.
func ArchiveBundle(directory, filename string) error {
	return engine.ArchiveBundle(directory, filename)
}

// DefaultConfig locates the default model in the working directory, beside the
// executable, or in an installation created by oneocr install. It never scans
// for alternate model packages.
func DefaultConfig() (Config, error) {
	return engine.DefaultConfig()
}

// FromFile borrows a file path; the file is read when an OCR operation runs.
func FromFile(path string) Input {
	return engine.FromFile(path)
}

// FromEncoded borrows encoded image bytes until the OCR operation returns.
func FromEncoded(data []byte) Input {
	return engine.FromEncoded(data)
}

// FromImage borrows a Go image until the OCR operation returns.
func FromImage(img image.Image) Input {
	return engine.FromImage(img)
}

// FromPixels borrows a pixel buffer with explicit format and row stride.
func FromPixels(pixels Pixels) Input {
	return engine.FromPixels(pixels)
}

// Open validates the complete bundle and initializes native model sessions.
// One process can have multiple Engines sharing the same ORT environment.
func Open(config Config) (*Engine, error) {
	return engine.Open(config)
}

// ReadPackage verifies the complete container without creating an ORT session.
func ReadPackage(filename string) (*PackageInfo, error) {
	return engine.ReadPackage(filename)
}

// Pack writes a new custom, uncompressed .ocrpack. ModelPath is an original
// .onemodel; BundleDir is a standard directory, including a developer-edited one.
// Conversion may use temporary files; recognition never extracts a package.
func Pack(options PackOptions) (*PackageInfo, error) {
	return engine.Pack(options)
}

// Unpack restores a new standard bundle directory for developer customization.
func Unpack(filename, directory string) (*Bundle, error) {
	return engine.Unpack(filename, directory)
}

// DefaultHome returns ONEOCR_HOME or the user configuration directory for OneOCR.
func DefaultHome() (string, error) {
	return engine.DefaultHome()
}

// Install prepares the model and runtime from local resources, this version's
// Release, or pinned upstream runtime archives. Recognition never downloads.
func Install(options InstallOptions) (Installation, error) {
	return engine.Install(options)
}

// LoadInstallation loads and verifies the installation stored in home; empty uses DefaultHome.
func LoadInstallation(home string) (Installation, error) {
	return engine.LoadInstallation(home)
}
