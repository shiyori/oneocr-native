package oneocr

import "errors"

var ErrClosed = errors.New("oneocr: engine is closed")

type Point [2]float64
type Quad [4]Point

type Line struct {
	Text       string   `json:"text"`
	Quad       Quad     `json:"quad"`
	Script     string   `json:"script"`
	Confidence *float64 `json:"confidence"`
	Words      []Word   `json:"words"`
}
type Word struct {
	Text       string   `json:"text"`
	Quad       Quad     `json:"quad"`
	Confidence *float64 `json:"confidence"`
}
type Result struct {
	Text           string   `json:"text"`
	Lines          []Line   `json:"lines"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	ElapsedSeconds float64  `json:"elapsed_seconds"`
	ModelSHA256    string   `json:"model_sha256"`
	Warnings       []string `json:"warnings"`
}

type Config struct {
	ModelPath      string `json:"model_path,omitempty"` // optional; defaults to oneocr-cjk-en.ocrpack; exclusive with BundleDir
	BundleDir      string `json:"bundle_dir,omitempty"`
	RuntimeLibrary string `json:"runtime_library,omitempty"`
	Threads        int    `json:"threads,omitempty"`  // default 2, range 1..16, per Engine
	MaxSide        int    `json:"max_side,omitempty"` // default 1600, range 128..4096
	// CharacterClasses restricts CJK/Latin decoding, not language identification.
	// Empty selects han,kana,hangul,latin,digits. Spaces and punctuation remain.
	CharacterClasses []CharacterClass `json:"character_classes,omitempty"`
}

// PackageInfo describes an uncompressed ONEOCRPK container. Offsets in Files
// are relative to the header's data section, not absolute file offsets.
type PackageInfo struct {
	Schema  string        `json:"schema"`
	Profile string        `json:"profile"`
	Bundle  Bundle        `json:"bundle"`
	Files   []PackageFile `json:"files"`
}
type PackageFile struct {
	FileInfo
	Offset int64 `json:"offset"`
}
type PackOptions struct {
	ModelPath string // original .onemodel input
	BundleDir string // alternative standard resource directory
	Profile   string // defaults to cjk-en; other profiles are reserved for development
	Output    string // new .ocrpack output, never overwritten
}
type Options struct {
	Script string // empty selects the script automatically
}

const maxImagePixels = 40_000_000
const BundleSchema = "oneocr.bundle.v1"

type CharacterModel struct {
	Name            string `json:"name"`
	Script          string `json:"script"`
	ModelPath       string `json:"model_path"`
	AlphabetPath    string `json:"alphabet_path"`
	PhysicalMapPath string `json:"physical_map_path"`
	CompositePath   string `json:"composite_path"`
	PriorPath       string `json:"prior_path"`
	PixelsPerFrame  int    `json:"pixels_per_frame"`
}
type Pipeline struct {
	DetectorPath     string           `json:"detector_path"`
	ClassifierPath   string           `json:"classifier_path"`
	Characters       []CharacterModel `json:"characters"`
	SegmentThreshold float64          `json:"segment_threshold"`
	LineThresholds   map[int]float64  `json:"line_thresholds"`
}
type TensorInfo struct {
	Name  string `json:"name"`
	DType string `json:"dtype"`
	Shape []any  `json:"shape"`
}
type ModelInterface struct {
	Inputs    []TensorInfo   `json:"inputs"`
	Outputs   []TensorInfo   `json:"outputs"`
	Opsets    map[string]int `json:"opsets"`
	Operators []string       `json:"operators"`
}
type FileInfo struct {
	File   string `json:"file"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type Resource struct {
	FileInfo
	ID           int             `json:"id"`
	OriginalName string          `json:"original_name"`
	Kind         string          `json:"kind"`
	Interface    *ModelInterface `json:"interface,omitempty"`
}
type RuntimeInfo struct {
	Name              string `json:"name"`
	TestedVersion     string `json:"tested_version"`
	Provider          string `json:"provider"`
	RequireContribOps bool   `json:"require_contrib_ops"`
}
type Bundle struct {
	Schema       string      `json:"schema"`
	SourceSHA256 string      `json:"source_sha256"`
	Runtime      RuntimeInfo `json:"runtime"`
	Config       FileInfo    `json:"config"`
	Pipeline     Pipeline    `json:"pipeline"`
	Resources    []Resource  `json:"resources"`
	directory    string
}

var scripts = []string{"Latin", "CJK", "Cyrillic", "Arabic", "Devanagari", "Greek", "Thai", "Hebrew", "Tamil"}
var classifierScripts = []string{"", "CJK", "Latin", "Cyrillic", "Arabic", "Devanagari", "Greek", "Thai", "Hebrew", "Tamil"}
