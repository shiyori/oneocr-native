package oneocr

import "github.com/shiyori/oneocr-native/internal/engine"

// Public types retain their methods and JSON representations across the internal boundary.
type (
	AndroidInstallOptions = engine.AndroidInstallOptions
	AndroidInstallation   = engine.AndroidInstallation
	Bundle                = engine.Bundle
	CharacterClass        = engine.CharacterClass
	CharacterModel        = engine.CharacterModel
	Config                = engine.Config
	Detection             = engine.Detection
	DetectionResult       = engine.DetectionResult
	Diagnostics           = engine.Diagnostics
	Engine                = engine.Engine
	FileInfo              = engine.FileInfo
	Input                 = engine.Input
	InstallOptions        = engine.InstallOptions
	Installation          = engine.Installation
	Line                  = engine.Line
	LineResult            = engine.LineResult
	ModelInterface        = engine.ModelInterface
	Options               = engine.Options
	PackOptions           = engine.PackOptions
	PackageFile           = engine.PackageFile
	PackageInfo           = engine.PackageInfo
	Pipeline              = engine.Pipeline
	PixelFormat           = engine.PixelFormat
	Pixels                = engine.Pixels
	Point                 = engine.Point
	Quad                  = engine.Quad
	Resource              = engine.Resource
	Result                = engine.Result
	RuntimeInfo           = engine.RuntimeInfo
	StageDiagnostics      = engine.StageDiagnostics
	TensorInfo            = engine.TensorInfo
	Word                  = engine.Word
)
