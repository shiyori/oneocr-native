package oneocr

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackendConfigValidation(t *testing.T) {
	for _, c := range []Config{{Backend: "gpu"}, {Backend: BackendCUDA, DeviceID: -1}, {Fallback: "silent"}, {StageBackends: map[string]Backend{"wrong": BackendCPU}}, {CharacterClasses: []CharacterClass{"chinese"}}} {
		if err := normalizeBackendConfig(&c); err == nil {
			t.Fatalf("accepted invalid config: %+v", c)
		}
	}
	c := Config{StageBackends: map[string]Backend{"detector": BackendCPU}}
	if err := normalizeBackendConfig(&c); err != nil {
		t.Fatal(err)
	}
	if c.Backend != BackendCPU || c.Fallback != FallbackCPU || c.CoreMLComputeUnits != "ALL" {
		t.Fatal(c)
	}
}
func TestClassSelectionAndCTCTies(t *testing.T) {
	a := &alphabet{characters: []string{"A", "1", "汉", "!", "<blank>"}, blank: 4, composites: map[string]string{}}
	if err := a.setClasses([]CharacterClass{CharactersDigits}); err != nil {
		t.Fatal(err)
	}
	got, err := a.decode(floatTensor{[]int64{2, 1, 5}, []float32{5, 4, 3, 2, 1, 5, 4, 3, 4, 1}}, "Latin")
	if err != nil || got != "1" {
		t.Fatalf("mask/tie: %q %v", got, err)
	}
	// Nonfinite values are rejected even when the corresponding token is masked.
	_, err = a.decode(floatTensor{[]int64{1, 1, 5}, []float32{float32(math.NaN()), 4, 3, 2, 1}}, "Latin")
	if err == nil {
		t.Fatal("masked NaN bypassed validation")
	}
	if _, err = a.decodeIDs([]int64{0}, "Latin"); err == nil {
		t.Fatal("compact output bypassed mask")
	}
	if _, err = a.decodeIDs([]int64{5}, "Latin"); err == nil {
		t.Fatal("out of range compact ID")
	}
}
func TestProfileOnlyCountsExecutedKernels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	raw := `[{"name":"conv_kernel_time","dur":5,"args":{"provider":"CoreMLExecutionProvider"}},{"name":"copy_fence_before","dur":99,"args":{"provider":"CPUExecutionProvider"}},{"name":"lstm_kernel_time","dur":7,"args":{"provider":"CPUExecutionProvider"}}]`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	usage, measured, err := readProviderProfile(path)
	if err != nil || !measured || usage["CoreMLExecutionProvider"].KernelEvents != 1 || usage["CPUExecutionProvider"].KernelMicroseconds != 7 {
		t.Fatal(usage, measured, err)
	}
}
func TestQueuedCancellationAndClosedDiagnostics(t *testing.T) {
	e := &Engine{gate: make(chan struct{}, 1), config: Config{Backend: BackendCPU}}
	e.gate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := e.lock(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	e.unlock()
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if !e.Diagnostics().Closed {
		t.Fatal("closed engine diagnostics unavailable")
	}
}
func TestAdaptationRejectsForeignSourceAndEscapingFiles(t *testing.T) {
	directory := t.TempDir()
	manifest := AdaptationManifest{Schema: AdaptationSchema, SourceSHA256: strings.Repeat("b", 64), Backend: BackendCPU, Models: map[string]AdaptedModel{"model.onnx": {FileInfo: FileInfo{File: "../model.onnx", Bytes: 1, SHA256: strings.Repeat("c", 64)}, SourceSHA256: strings.Repeat("d", 64), Recipe: "test", Outputs: []string{"logsoftmax"}}}}
	b := &Bundle{SourceSHA256: strings.Repeat("a", 64), Resources: []Resource{{FileInfo: FileInfo{File: "model.onnx", SHA256: strings.Repeat("d", 64)}, Kind: "onnx"}}}
	write := func() {
		raw, _ := json.Marshal(manifest)
		if err := os.WriteFile(filepath.Join(directory, "adaptation.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	if _, err := readAdaptation(directory, b); err == nil {
		t.Fatal("accepted unrelated model source")
	}
	manifest.SourceSHA256 = b.SourceSHA256
	write()
	if _, err := readAdaptation(directory, b); err == nil {
		t.Fatal("accepted path traversal")
	}
}

func TestAdaptationRejectsRetiredDetectorRecipes(t *testing.T) {
	for _, recipe := range []string{"v2-detector-integer-grid-coreml-grid", "v2.1-detector-integer-grid-cuda-grid", "v1-coreml-grid", "v3-original-quantized-detector-directml-grid"} {
		t.Run(recipe, func(t *testing.T) {
			directory := t.TempDir()
			sourceHash, modelHash := strings.Repeat("a", 64), strings.Repeat("b", 64)
			b := &Bundle{SourceSHA256: sourceHash, Resources: []Resource{{FileInfo: FileInfo{File: "detector.onnx", SHA256: modelHash}, Kind: "onnx"}}}
			manifest := AdaptationManifest{Schema: AdaptationSchema, SourceSHA256: sourceHash, Backend: BackendDirectML, Models: map[string]AdaptedModel{"detector.onnx": {FileInfo: FileInfo{File: "models/detector.onnx", Bytes: 1, SHA256: modelHash}, SourceSHA256: modelHash, Recipe: recipe, Outputs: []string{"scores"}}}}
			if strings.Contains(recipe, "coreml") {
				manifest.Backend = BackendCoreML
			}
			data, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(directory, "adaptation.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			_, err = readAdaptation(directory, b)
			if strings.Contains(recipe, "-detector-integer-grid") {
				if err == nil || !strings.Contains(err.Error(), "retired detector integer-grid") {
					t.Fatalf("expected explicit retirement error, got %v", err)
				}
			} else if manifest.Backend == BackendCoreML {
				if err == nil || !strings.Contains(err.Error(), "CoreML model adaptation has been retired") {
					t.Fatalf("expected explicit CoreML retirement error, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestNativeRetiredCoreMLFallbackAndProfiling(t *testing.T) {
	bundle, library, fixtures := os.Getenv("ONEOCR_BUNDLE"), os.Getenv("ONEOCR_RUNTIME"), os.Getenv("ONEOCR_FIXTURES")
	if bundle == "" || library == "" || fixtures == "" {
		t.Skip("set native integration paths")
	}
	cfg := Config{BundleDir: bundle, RuntimeLibrary: library, Backend: BackendCoreML, Threads: 1, Fallback: FallbackError}
	if e, err := Open(cfg); err == nil {
		e.Close()
		t.Fatal("strict unsupported provider succeeded")
	} else if !strings.Contains(err.Error(), "CoreML acceleration has been retired") {
		t.Fatal(err)
	}
	cfg.Fallback = FallbackCPU
	cfg.ProfilingDir = t.TempDir()
	e, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err = e.Warmup(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := e.RecognizeFile(context.Background(), filepath.Join(fixtures, "CJK.png"), Options{})
	if err != nil || result.Text != "你好世界 日本語テスト 한국어 123" {
		t.Fatal(result.Text, err)
	}
	for _, stage := range e.Diagnostics().Stages {
		if stage.Requested != BackendCoreML || stage.Registered != BackendCPU || !strings.Contains(stage.FallbackReason, "CoreML acceleration has been retired") || stage.ExecutionMeasured {
			t.Fatal(stage)
		}
	}
	if err = e.Close(); err != nil {
		t.Fatal(err)
	}
	for _, stage := range e.Diagnostics().Stages {
		if !stage.ExecutionMeasured || stage.Providers["CPUExecutionProvider"].KernelEvents == 0 {
			t.Fatal(stage)
		}
	}
}
