package oneocr

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestShapeSpecializationPreservesWeightsAndRejectsFixedMismatch(t *testing.T) {
	dim := func(value protoValue) protoValue { return protoMessage(protoFields{1: []protoValue{value}}) }
	shape := protoFields{1: []protoValue{dim(protoValue{wire: 0, integer: 1}), dim(protoValue{wire: 0, integer: 3}), dim(protoValue{wire: 0, integer: 60}), protoMessage(protoFields{2: []protoValue{{wire: 2, bytes: []byte("width")}}})}}
	tensor := protoFields{1: []protoValue{{wire: 0, integer: 1}}, 2: []protoValue{protoMessage(shape)}}
	typ := protoFields{1: []protoValue{protoMessage(tensor)}}
	input := protoFields{1: []protoValue{{wire: 2, bytes: []byte("data")}}, 2: []protoValue{protoMessage(typ)}}
	weights := []byte("opaque-initializer-payload-preserved-exactly")
	graph := protoFields{5: []protoValue{{wire: 2, bytes: weights}}, 11: []protoValue{protoMessage(input)}}
	original := marshalProto(protoFields{7: []protoValue{protoMessage(graph)}})
	adapted, err := specializeInputShapes(original, map[string][]int64{"data": {1, 3, 60, 256}})
	if err != nil {
		t.Fatal(err)
	}
	m, err := parseProto(adapted)
	if err != nil {
		t.Fatal(err)
	}
	g, err := m.nested(7)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(g[5][0].bytes, weights) {
		t.Fatal("modified initializer")
	}
	in, _ := parseProto(g[11][0].bytes)
	tp, _ := in.nested(2)
	tt, _ := tp.nested(1)
	s, _ := tt.nested(2)
	last, _ := parseProto(s[1][3].bytes)
	value, _ := last.number(1, 0)
	if value != 256 || len(last[2]) != 0 {
		t.Fatal("width not specialized")
	}
	if _, err = specializeInputShapes(original, map[string][]int64{"data": {2, 3, 60, 256}}); err == nil {
		t.Fatal("changed fixed batch")
	}
	if _, err = specializeInputShapes(original, map[string][]int64{"wrong": {1}}); err == nil {
		t.Fatal("silently ignored unknown input")
	}
}
func TestShapeKeySeparatesAllInputs(t *testing.T) {
	first := inputShapeKey(map[string][]int64{"data": {1, 3, 60, 200}, "seq_lengths": {1}})
	same := inputShapeKey(map[string][]int64{"seq_lengths": {1}, "data": {1, 3, 60, 200}})
	other := inputShapeKey(map[string][]int64{"data": {1, 3, 60, 208}, "seq_lengths": {1}})
	if first != same || first == other {
		t.Fatal(first, same, other)
	}
}
func TestNativeCoreMLShapeCache(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("requires CoreML")
	}
	model, lib, fixtures, adaptation := os.Getenv("ONEOCR_MODEL_PACKAGE"), os.Getenv("ONEOCR_RUNTIME"), os.Getenv("ONEOCR_FIXTURES"), os.Getenv("ONEOCR_COREML_ADAPTATION")
	if model == "" || lib == "" || fixtures == "" || adaptation == "" {
		t.Skip("set CoreML validation paths")
	}
	cfg := Config{ModelPath: model, RuntimeLibrary: lib, Backend: BackendCoreML, Fallback: FallbackError, Threads: 2, AdaptationDir: adaptation, ShapeCacheSize: 2, CacheDir: t.TempDir(), ProfilingDir: t.TempDir(), StageBackends: map[string]Backend{"classifier": BackendCPU, "recognizer/CJK": BackendCPU, "recognizer/Latin": BackendCPU}}
	e, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err = e.Warmup(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"CJK.png", "Latin_rotate_90.png", "CJK.png"} {
		r, err := e.RecognizeFile(context.Background(), filepath.Join(fixtures, name), Options{})
		if err != nil || r.Text == "" {
			t.Fatal(name, r.Text, err)
		}
	}
	var detector StageDiagnostics
	for _, s := range e.Diagnostics().Stages {
		if s.Stage == "detector" {
			detector = s
		}
	}
	if detector.ShapeCacheMisses < 3 || detector.ShapeCacheHits < 1 || detector.ShapeEvictions < 1 || len(detector.ShapeSessions) != 2 {
		t.Fatal(detector)
	}
	if err = e.Close(); err != nil {
		t.Fatal(err)
	}
	for _, s := range e.Diagnostics().Stages {
		if s.Stage == "detector" && (!s.ExecutionMeasured || s.Providers["CoreMLExecutionProvider"].KernelEvents == 0) {
			t.Fatal(s)
		}
	}
}
