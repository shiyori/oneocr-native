package oneocr

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	ort "github.com/shiyori/oneocr-native/internal/ort"
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
	if _, err := first.Recognize(ctx, FromFile(filepath.Join(fixtures, "Latin.png")), Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Recognize(context.Background(), FromEncoded(nil), Options{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := second.Recognize(context.Background(), FromFile(filepath.Join(fixtures, "Latin.png")), Options{})
			if err != nil || result.Text != "Hello World 123" {
				t.Errorf("shared engine: %q %v", result.Text, err)
			}
		}()
	}
	wg.Wait()
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if environment.references != 0 {
		t.Fatal("SDK runtime reference was not released")
	}
	// An independently acquired host environment and session survive SDK cleanup.
	host, err := ort.Open(library)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	model, err := os.ReadFile(filepath.Join("testdata", "identity.onnx"))
	if err != nil {
		t.Fatal(err)
	}
	session, err := host.NewSession(model, []string{"x"}, []string{"y"}, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Destroy()
	checkHost := func() {
		input, err := ort.NewTensor(host, ort.Shape{1}, []float32{42})
		if err != nil {
			t.Fatal(err)
		}
		defer input.Destroy()
		output := make([]ort.Value, 1)
		if err := session.Run([]ort.Value{input}, output); err != nil {
			t.Fatal(err)
		}
		if value, ok := output[0].(*ort.Tensor[float32]); !ok || len(value.GetData()) != 1 || value.GetData()[0] != 42 {
			t.Fatal("host model output changed")
		}
		for _, value := range output {
			value.Destroy()
		}
	}
	checkHost()
	t.Setenv("ONEOCR_RUNTIME", "")
	config.RuntimeLibrary = ""
	shared, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := shared.Close(); err != nil {
		t.Fatal(err)
	}
	checkHost()
	// Fail after acquiring the runtime, during SDK session creation.
	invalid := filepath.Join(t.TempDir(), "invalid-bundle")
	corrupted, err := Unpack(filepath.Join("models", DefaultModelName), invalid)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("not an ONNX graph")
	for i := range corrupted.Resources {
		resource := &corrupted.Resources[i]
		if resource.File == corrupted.Pipeline.DetectorPath {
			if err = os.WriteFile(filepath.Join(invalid, filepath.FromSlash(resource.File)), payload, 0644); err != nil {
				t.Fatal(err)
			}
			resource.Bytes = int64(len(payload))
			resource.SHA256 = fmt.Sprintf("%x", sha256.Sum256(payload))
		}
	}
	metadata, err := json.Marshal(corrupted)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(invalid, "bundle.json"), metadata, 0644); err != nil {
		t.Fatal(err)
	}
	if failed, err := Open(Config{BundleDir: invalid}); err == nil {
		failed.Close()
		t.Fatal("accepted invalid ONNX model")
	}
	checkHost()
	if environment.references != 0 {
		t.Fatal("failed Open leaked an SDK runtime reference")
	}
}
