package oneocr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	ort "github.com/yalue/onnxruntime_go"
)

func TestNativeLifecycle(t *testing.T) {
	bundle, library, fixtures := os.Getenv("ONEOCR_BUNDLE"), os.Getenv("ONEOCR_RUNTIME"), os.Getenv("ONEOCR_FIXTURES")
	if bundle == "" || library == "" || fixtures == "" {
		t.Skip("set ONEOCR_BUNDLE, ONEOCR_RUNTIME and ONEOCR_FIXTURES")
	}
	config := Config{BundleDir: bundle, RuntimeLibrary: library, Threads: 1}
	first, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := first.RecognizeFile(ctx, filepath.Join(fixtures, "Latin.png"), Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := first.RecognizeEncoded(context.Background(), nil, Options{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := second.RecognizeFile(context.Background(), filepath.Join(fixtures, "Latin.png"), Options{})
			if err != nil || result.Text != "Hello World 123" {
				t.Errorf("shared engine: %q %v", result.Text, err)
			}
		}()
	}
	wg.Wait()
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if ort.IsInitialized() {
		t.Fatal("SDK-owned environment was not released")
	}
	// A host-owned environment must survive SDK cleanup.
	ort.SetSharedLibraryPath(library)
	if err := ort.InitializeEnvironment(ort.WithLogLevelError()); err != nil {
		t.Fatal(err)
	}
	defer ort.DestroyEnvironment()
	if _, err := Open(config); err == nil {
		t.Fatal("silently acquired host environment")
	}
	config.UseExistingORT = true
	shared, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := shared.Close(); err != nil {
		t.Fatal(err)
	}
	if !ort.IsInitialized() {
		t.Fatal("SDK destroyed host environment")
	}
}
