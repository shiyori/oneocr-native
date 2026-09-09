package oneocr

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type referenceCase struct {
	File           string          `json:"file"`
	LineFile       string          `json:"line_file"`
	Recognize      json.RawMessage `json:"recognize"`
	Detect         json.RawMessage `json:"detect"`
	RecognizeLine  json.RawMessage `json:"recognize_line"`
	RecognizeError string          `json:"recognize_error"`
	DetectError    string          `json:"detect_error"`
	LineError      string          `json:"line_error"`
}

func testRasterImage(r raster) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, r.width, r.height))
	for y := 0; y < r.height; y++ {
		for x := 0; x < r.width; x++ {
			i, j := (y*r.width+x)*3, y*img.Stride+x*4
			copy(img.Pix[j:j+3], r.pixels[i:i+3])
			img.Pix[j+3] = 255
		}
	}
	return img
}
func TestExportReferenceCrops(t *testing.T) {
	directory := os.Getenv("ONEOCR_EXPORT_REFERENCE")
	if directory == "" {
		t.Skip("reference generation only")
	}
	engine, err := Open(Config{ModelPath: filepath.Join("models", DefaultModelName), Threads: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	if err = os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	cases := []referenceCase{}
	err = filepath.WalkDir("testdata", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".png" && extension != ".jpg" && extension != ".jpeg" {
			return nil
		}
		r, err := FromFile(path).raster()
		if err != nil {
			return err
		}
		detections, err := engine.Detect(context.Background(), FromFile(path))
		if err != nil {
			return err
		}
		crop := newRaster(128, 60, true)
		if len(detections.Regions) > 0 {
			region := detections.Regions[0]
			crop, err = rectify(r, region.Quad, region.Vertical)
			if err != nil {
				return err
			}
		}
		name := fmt.Sprintf("line-%x.png", sha256.Sum256([]byte(filepath.ToSlash(path))))
		f, err := os.Create(filepath.Join(directory, name))
		if err != nil {
			return err
		}
		err = png.Encode(f, testRasterImage(crop))
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		cases = append(cases, referenceCase{File: filepath.ToSlash(path), LineFile: name})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 42 {
		t.Fatalf("expected 42 fixtures, got %d", len(cases))
	}
	data, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, "cases.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}
func referenceForms(t *testing.T, path string) []Input {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := FromEncoded(data).raster()
	if err != nil {
		t.Fatal(err)
	}
	return []Input{FromFile(path), FromEncoded(data), FromImage(testRasterImage(r)), FromPixels(Pixels{Data: r.pixels, Width: r.width, Height: r.height, Stride: r.width * 3, Format: RGB})}
}
func referenceEqual(t *testing.T, got interface{}, err error, expected json.RawMessage, expectedError string) {
	t.Helper()
	if expectedError != "" {
		if err == nil || err.Error() != expectedError {
			t.Fatalf("error changed: %v != %q", err, expectedError)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var a, b map[string]interface{}
	if err = json.Unmarshal(encoded, &a); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(expected, &b); err != nil {
		t.Fatal(err)
	}
	delete(a, "elapsed_seconds")
	delete(b, "elapsed_seconds")
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("CPU output changed\ngot: %s\nwant: %s", encoded, expected)
	}
}
func TestAPIReference(t *testing.T) {
	directory := os.Getenv("ONEOCR_REFERENCE")
	if directory == "" {
		t.Skip("set ONEOCR_REFERENCE to a same-runtime baseline")
	}
	data, err := os.ReadFile(filepath.Join(directory, "reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []referenceCase
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 42 {
		t.Fatalf("expected 42 fixtures, got %d", len(cases))
	}
	engine, err := Open(Config{ModelPath: filepath.Join("models", DefaultModelName), Threads: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	ctx := context.Background()
	for _, c := range cases {
		t.Run(c.File, func(t *testing.T) {
			for i, input := range referenceForms(t, filepath.FromSlash(c.File)) {
				t.Run(fmt.Sprintf("recognize-%d", i), func(t *testing.T) {
					value, err := engine.Recognize(ctx, input, Options{})
					referenceEqual(t, value, err, c.Recognize, c.RecognizeError)
				})
				t.Run(fmt.Sprintf("detect-%d", i), func(t *testing.T) {
					value, err := engine.Detect(ctx, input)
					referenceEqual(t, value, err, c.Detect, c.DetectError)
				})
			}
			for i, input := range referenceForms(t, filepath.Join(directory, c.LineFile)) {
				t.Run(fmt.Sprintf("line-%d", i), func(t *testing.T) {
					value, err := engine.RecognizeLine(ctx, input, Options{})
					referenceEqual(t, value, err, c.RecognizeLine, c.LineError)
				})
			}
		})
	}
}
