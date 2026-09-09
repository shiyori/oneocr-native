package oneocr_test

import (
	"context"
	"errors"
	"image"
	"testing"

	oneocr "github.com/shiyori/oneocr-native"
)

// Keep the root import usable by external consumers after implementation moves.
var (
	_ func(oneocr.Config) (*oneocr.Engine, error)                            = oneocr.Open
	_ func() (oneocr.Config, error)                                          = oneocr.DefaultConfig
	_ func() (string, error)                                                 = oneocr.DefaultHome
	_ func(oneocr.InstallOptions) (oneocr.Installation, error)               = oneocr.Install
	_ func(string) (oneocr.Installation, error)                              = oneocr.LoadInstallation
	_ func(oneocr.AndroidInstallOptions) (oneocr.AndroidInstallation, error) = oneocr.InstallAndroid
	_ func(string) (*oneocr.PackageInfo, error)                              = oneocr.ReadPackage
	_ func(oneocr.PackOptions) (*oneocr.PackageInfo, error)                  = oneocr.Pack
	_ func(string, string) (*oneocr.Bundle, error)                           = oneocr.Unpack
	_ func(string) (*oneocr.Bundle, error)                                   = oneocr.ReadBundle
	_ func(string, string) (*oneocr.Bundle, error)                           = oneocr.ExportModel
	_ func(string, string) error                                             = oneocr.ArchiveBundle
)

func TestPublicEngineBoundary(t *testing.T) {
	inputs := []oneocr.Input{
		oneocr.FromFile("missing.png"),
		oneocr.FromEncoded([]byte("invalid image")),
		oneocr.FromImage(image.NewRGBA(image.Rect(0, 0, 2, 2))),
		oneocr.FromPixels(oneocr.Pixels{Data: make([]byte, 16), Width: 2, Height: 2, Stride: 8, Format: oneocr.RGBA}),
	}
	var engine *oneocr.Engine
	for _, input := range inputs {
		if _, err := engine.Recognize(context.Background(), input, oneocr.Options{}); !errors.Is(err, oneocr.ErrClosed) {
			t.Fatalf("Recognize closed error: %v", err)
		}
		if _, err := engine.Detect(context.Background(), input); !errors.Is(err, oneocr.ErrClosed) {
			t.Fatalf("Detect closed error: %v", err)
		}
		if _, err := engine.RecognizeLine(context.Background(), input, oneocr.Options{}); !errors.Is(err, oneocr.ErrClosed) {
			t.Fatalf("RecognizeLine closed error: %v", err)
		}
	}
	if !engine.Diagnostics().Closed || engine.Close() != nil {
		t.Fatal("nil engine lifecycle changed")
	}
	if _, err := oneocr.Open(oneocr.Config{Threads: -1}); err == nil {
		t.Fatal("Open lost configuration validation")
	}
	config := (oneocr.Installation{ModelPath: "model.ocrpack", RuntimeLibrary: "runtime"}).Config()
	if config.ModelPath != "model.ocrpack" || config.RuntimeLibrary != "runtime" {
		t.Fatal("Installation.Config changed")
	}
}
