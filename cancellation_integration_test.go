package oneocr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeRunCancellationAndReuse(t *testing.T) {
	library := os.Getenv("ONEOCR_RUNTIME")
	if library == "" {
		t.Skip("set ONEOCR_RUNTIME")
	}
	if err := acquireRuntime(library); err != nil {
		t.Fatal(err)
	}
	defer releaseRuntime()
	model, err := os.ReadFile(filepath.Join("testdata", "cancelable_loop.onnx"))
	if err != nil {
		t.Fatal(err)
	}
	session, err := environment.runtime.NewSession(model, []string{"data", "seq_lengths"}, []string{"y"}, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	n := &network{session: session, outputs: []string{"y"}}
	defer n.close()
	count := int32(1_000_000_000)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = n.run(ctx, floatTensor{shape: []int64{1}, data: []float32{42}}, nil, &count)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("native run did not cancel", err)
	}
	if n.diagnostics.Runs != 1 {
		t.Fatal("cancellation did not reach the native run path")
	}
	count = 1
	result, err := n.run(context.Background(), floatTensor{shape: []int64{1}, data: []float32{42}}, nil, &count)
	if err != nil || len(result["y"].data) != 1 || result["y"].data[0] != 43 {
		t.Fatal("session was not reusable after cancellation", result, err)
	}
	// A zero-trip Loop can alias the input value; outputs must be consumed and
	// released while that input's Go storage remains pinned.
	count = 0
	result, err = n.run(context.Background(), floatTensor{shape: []int64{1}, data: []float32{42}}, nil, &count)
	if err != nil || len(result["y"].data) != 1 || result["y"].data[0] != 42 {
		t.Fatal("aliased output changed", result, err)
	}
}
