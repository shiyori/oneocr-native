package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	ort "github.com/shiyori/oneocr-native/internal/ort"
)

var environment struct {
	sync.Mutex
	references int
	path       string
	runtime    *ort.Runtime
}

func acquireRuntime(library string) error {
	environment.Lock()
	defer environment.Unlock()
	if environment.references > 0 {
		if library != environment.path && !environment.runtime.Matches(library) {
			return fmt.Errorf("oneocr: a different ONNX Runtime is already in use")
		}
		environment.references++
		return nil
	}
	rt, err := ort.Open(library)
	if err != nil {
		return fmt.Errorf("oneocr: runtime %q: %w", library, err)
	}
	environment.runtime, environment.path, environment.references = rt, library, 1
	return nil
}
func releaseRuntime() error {
	environment.Lock()
	defer environment.Unlock()
	if environment.references <= 0 {
		return nil
	}
	environment.references--
	if environment.references == 0 {
		environment.runtime.Close()
		environment.runtime, environment.path = nil, ""
	}
	return nil
}

type floatTensor struct {
	shape []int64
	data  []float32
}
type network struct {
	session     *ort.Session
	outputs     []string
	diagnostics StageDiagnostics
}

func (n *network) close() error {
	if n == nil || n.session == nil {
		return nil
	}
	err := n.session.Destroy()
	n.session = nil
	return err
}

// runBorrowed owns every ORT value until consume returns. Consumers must not
// retain tensor-backed slices; detector/classifier callers use owned copies.
func (n *network) runBorrowed(ctx context.Context, data floatTensor, extraFloat *floatTensor, sequence *int32, consume func([]ort.Value) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if n == nil || n.session == nil {
		return fmt.Errorf("oneocr: unavailable model session")
	}
	inputs := []ort.Value{}
	outputs := make([]ort.Value, len(n.outputs))
	defer func() {
		// Outputs may alias input storage. Release them while inputs remain pinned.
		for _, v := range outputs {
			if v != nil {
				v.Destroy()
			}
		}
		for _, v := range inputs {
			if v != nil {
				v.Destroy()
			}
		}
	}()

	t, err := ort.NewTensor(environment.runtime, ort.Shape(data.shape), data.data)
	if err != nil {
		return err
	}
	inputs = append(inputs, t)
	if extraFloat != nil {
		v, err := ort.NewTensor(environment.runtime, ort.Shape(extraFloat.shape), extraFloat.data)
		if err != nil {
			return err
		}
		inputs = append(inputs, v)
	}
	if sequence != nil {
		v, err := ort.NewTensor(environment.runtime, ort.NewShape(1), []int32{*sequence})
		if err != nil {
			return err
		}
		inputs = append(inputs, v)
	}
	start := time.Now()
	if ctx.Done() == nil {
		err = n.session.Run(inputs, outputs)
	} else {
		options, e := environment.runtime.NewRunOptions()
		if e != nil {
			return e
		}
		done, stopped := make(chan struct{}), make(chan struct{})
		go func() {
			defer close(stopped)
			select {
			case <-ctx.Done():
				options.Terminate()
			case <-done:
			}
		}()
		err = n.session.RunWithOptions(inputs, outputs, options)
		close(done)
		<-stopped
		options.Destroy()
	}
	n.recordRun(start)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	return consume(outputs)
}

func (n *network) run(ctx context.Context, data floatTensor, extraFloat *floatTensor, sequence *int32) (map[string]floatTensor, error) {
	var result map[string]floatTensor
	err := n.runBorrowed(ctx, data, extraFloat, sequence, func(outputs []ort.Value) error {
		result = make(map[string]floatTensor, len(outputs))
		for i, v := range outputs {
			tensor, ok := v.(*ort.Tensor[float32])
			if !ok {
				return fmt.Errorf("oneocr: unexpected output type for %s", n.outputs[i])
			}
			result[n.outputs[i]] = floatTensor{append([]int64(nil), tensor.GetShape()...), append([]float32(nil), tensor.GetData()...)}
		}
		return nil
	})
	return result, err
}
