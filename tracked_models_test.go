package oneocr

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTrackedModelPackages(t *testing.T) {
	for _, profile := range []string{"cjk-en", "extended"} {
		path := filepath.Join("models", "oneocr-"+profile+".ocrpack")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Skip("model data is distributed separately")
		}
		info, err := ReadPackage(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Profile != profile {
			t.Fatal("profile mismatch")
		}
	}
}

func TestTrackedPackageRecognition(t *testing.T) {
	library := os.Getenv("ONEOCR_RUNTIME")
	if library == "" {
		t.Skip("set ONEOCR_RUNTIME for CPU inference")
	}
	if _, err := os.Stat("models/oneocr-cjk-en.ocrpack"); os.IsNotExist(err) {
		t.Skip("model data is distributed separately")
	}
	data, err := os.ReadFile("testdata/annotations.json")
	if err != nil {
		t.Fatal(err)
	}
	var labels []struct {
		File   string `json:"file"`
		Script string `json:"script"`
		Text   string `json:"text"`
	}
	if err = json.Unmarshal(data, &labels); err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"cjk-en", "extended"} {
		t.Run(profile, func(t *testing.T) {
			engine, err := Open(Config{ModelPath: filepath.Join("models", "oneocr-"+profile+".ocrpack"), RuntimeLibrary: library, Threads: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer engine.Close()
			for _, label := range labels {
				if label.File == "columns.png" || label.File == "columns_rotate_90.png" || label.File == "multilingual.png" {
					continue
				}
				script := label.Script
				if script == "" {
					script = "Latin"
				}
				result, err := engine.RecognizeFile(context.Background(), filepath.Join("testdata", label.File), Options{})
				if err != nil {
					t.Fatal(label.File, err)
				}
				if _, ok := engine.characters[script]; ok {
					if result.Text != label.Text {
						t.Fatalf("%s: %q != %q", label.File, result.Text, label.Text)
					}
				} else if result.Text != "" || len(result.Warnings) < 2 {
					t.Fatal("missing skip/warning", label.File)
				}
			}
			if _, err := engine.RecognizeFile(context.Background(), "testdata/Latin.png", Options{Script: "Thai"}); err == nil {
				t.Fatal("accepted missing script")
			}
			descriptor := engine.source.file
			if err = engine.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err = descriptor.Stat(); err == nil {
				t.Fatal("descriptor leaked")
			}
			if _, err = engine.RecognizeFile(context.Background(), "testdata/Latin.png", Options{}); err == nil {
				t.Fatal("recognition after close")
			}
		})
	}
}
