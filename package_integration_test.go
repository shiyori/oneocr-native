package oneocr

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPackageNativeParity(t *testing.T) {
	packages, bundle, library, fixtures := os.Getenv("ONEOCR_PACKAGES"), os.Getenv("ONEOCR_BUNDLE"), os.Getenv("ONEOCR_RUNTIME"), os.Getenv("ONEOCR_FIXTURES")
	if packages == "" || bundle == "" || library == "" || fixtures == "" {
		t.Skip("set ONEOCR_PACKAGES, ONEOCR_BUNDLE, ONEOCR_RUNTIME and ONEOCR_FIXTURES")
	}
	full, err := Open(Config{BundleDir: bundle, RuntimeLibrary: library, Threads: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	var annotations []struct {
		File   string          `json:"file"`
		Source json.RawMessage `json:"source"`
	}
	data, err := os.ReadFile(filepath.Join(fixtures, "annotations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &annotations); err != nil {
		t.Fatal(err)
	}
	baselines := map[string]Result{}
	for _, a := range annotations {
		if len(a.Source) > 0 {
			continue
		}
		r, err := full.RecognizeFile(context.Background(), filepath.Join(fixtures, a.File), Options{})
		if err != nil {
			t.Fatal(a.File, err)
		}
		baselines[a.File] = r
	}
	for _, profile := range []string{"cjk-en", "extended"} {
		t.Run(profile, func(t *testing.T) {
			path := filepath.Join(packages, "oneocr-"+profile+".ocrpack")
			engine, err := Open(Config{ModelPath: path, RuntimeLibrary: library, Threads: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer engine.Close()
			descriptor := engine.source.file
			if len(engine.recognizers) != 0 {
				t.Fatal("recognizers loaded eagerly")
			}
			for _, a := range annotations {
				base, ok := baselines[a.File]
				if !ok {
					continue
				}
				r, err := engine.RecognizeFile(context.Background(), filepath.Join(fixtures, a.File), Options{})
				if err != nil {
					t.Fatal(a.File, err)
				}
				expected := []Line{}
				text := []string{}
				for _, line := range base.Lines {
					if _, supported := engine.characters[line.Script]; supported {
						expected = append(expected, line)
						text = append(text, line.Text)
					}
				}
				if !reflect.DeepEqual(r.Lines, expected) || r.Text != strings.Join(text, "\n") {
					t.Fatalf("%s: package result differs from supported directory lines\ngot: %s\nwant: %s", a.File, r.Text, strings.Join(text, "\n"))
				}
				if len(expected) < len(base.Lines) && len(r.Warnings) < 2 {
					t.Fatal(a.File, "unsupported lines had no warning")
				}
			}
			if _, err = engine.RecognizeFile(context.Background(), filepath.Join(fixtures, "Latin.png"), Options{Script: "Thai"}); err == nil {
				t.Fatal("accepted unavailable forced script")
			}
			if len(engine.recognizers) != len(engine.characters) {
				t.Fatal("lazy recognizers not populated by fixtures")
			}
			if err = engine.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err = descriptor.ReadAt(make([]byte, 1), 0); !errors.Is(err, os.ErrClosed) {
				t.Fatal("package descriptor leaked")
			}
			t.Logf("%d fixture comparisons passed for %s", len(baselines), profile)
		})
	}
}
