package oneocr

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndependentStagesValidation(t *testing.T) {
	var e *Engine
	if _, err := e.Detect(context.Background(), FromEncoded(nil)); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if _, err := e.RecognizeLine(context.Background(), FromPixels(Pixels{Data: nil, Width: 2, Height: 2, Stride: 6, Format: RGB}), Options{}); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	live := &Engine{gate: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := live.Detect(ctx, FromImage(image.NewRGBA(image.Rect(0, 0, 2, 2)))); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := live.RecognizeLine(ctx, FromImage(image.NewRGBA(image.Rect(0, 0, 2, 2))), Options{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	live.gate <- struct{}{}
	ctx, cancel = context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := live.Detect(ctx, FromEncoded(nil)); done <- err }()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	<-live.gate
	if _, err := live.Detect(context.Background(), FromEncoded([]byte("invalid"))); err == nil {
		t.Fatal("accepted invalid image")
	}
	if _, err := live.RecognizeLine(context.Background(), FromPixels(Pixels{Data: make([]byte, 11), Width: 2, Height: 2, Stride: 6, Format: RGB}), Options{}); err == nil {
		t.Fatal("accepted short buffer")
	}
}
func TestIndependentStagesNative(t *testing.T) {
	bundle, library := os.Getenv("ONEOCR_BUNDLE"), os.Getenv("ONEOCR_RUNTIME")
	if bundle == "" || library == "" {
		t.Skip("set ONEOCR_BUNDLE and ONEOCR_RUNTIME")
	}
	e, err := Open(Config{BundleDir: bundle, RuntimeLibrary: library, Threads: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	ctx := context.Background()
	data, err := os.ReadFile(filepath.Join("testdata", "Latin.png"))
	if err != nil {
		t.Fatal(err)
	}
	detected, err := e.Detect(ctx, FromEncoded(data))
	if err != nil || len(detected.Regions) != 1 {
		t.Fatalf("detection: %+v %v", detected, err)
	}
	for _, d := range e.Diagnostics().Stages {
		if d.Stage != "detector" && d.Runs != 0 {
			t.Fatalf("detector ran %s", d.Stage)
		}
	}
	r, err := decodeImage(data)
	if err != nil {
		t.Fatal(err)
	}
	d := detected.Regions[0]
	crop, err := rectify(r, d.Quad, d.Vertical)
	if err != nil {
		t.Fatal(err)
	}
	line, err := e.RecognizeLine(ctx, FromPixels(Pixels{Data: crop.pixels, Width: crop.width, Height: crop.height, Stride: crop.width * 3, Format: RGB}), Options{Script: "Latin"})
	if err != nil || line.Text != "Hello World 123" {
		t.Fatalf("line: %+v %v", line, err)
	}
	for _, d := range e.Diagnostics().Stages {
		if d.Stage == "classifier" && d.Runs != 0 {
			t.Fatal("explicit script ran classifier")
		}
		if d.Stage == "detector" && d.Runs != 1 {
			t.Fatal("line ran detector")
		}
	}
	auto, err := e.RecognizeLine(ctx, FromPixels(Pixels{Data: crop.pixels, Width: crop.width, Height: crop.height, Stride: crop.width * 3, Format: RGB}), Options{})
	if err != nil || auto.Text != line.Text {
		t.Fatalf("auto: %+v %v", auto, err)
	}
	rotated := crop.orient(3)
	auto, err = e.RecognizeLine(ctx, FromPixels(Pixels{Data: rotated.pixels, Width: rotated.width, Height: rotated.height, Stride: rotated.width * 3, Format: RGB}), Options{})
	if err != nil || auto.Text != line.Text || !auto.Rotated180 {
		t.Fatalf("rotated: %+v %v", auto, err)
	}
	full, err := e.Recognize(ctx, FromEncoded(data), Options{Script: "Latin"})
	if err != nil || full.Text != line.Text {
		t.Fatalf("full differs: %+v %v", full, err)
	}
	if _, err = e.RecognizeLine(ctx, FromEncoded(data), Options{Script: "bad"}); err == nil || !strings.Contains(err.Error(), "unavailable script") {
		t.Fatal(err)
	}
	if err = e.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = e.Detect(ctx, FromEncoded(data)); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if _, err = e.RecognizeLine(ctx, FromEncoded(data), Options{}); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}
