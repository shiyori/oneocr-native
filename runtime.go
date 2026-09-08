package oneocr

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	ort "github.com/yalue/onnxruntime_go"
)

var environment struct {
	sync.Mutex
	references int
	path       string
	owned      bool
}

func acquireRuntime(library string, existing bool) error {
	environment.Lock()
	defer environment.Unlock()
	var e error
	if library != "" && !strings.HasPrefix(library, "@rpath/") && strings.ContainsAny(library, "/\\") {
		library, e = filepath.Abs(library)
		if e != nil {
			return e
		}
	}
	if environment.references > 0 {
		if library != "" && environment.path != "" && library != environment.path {
			return fmt.Errorf("oneocr: a different ONNX Runtime is already in use")
		}
		environment.references++
		return nil
	}
	if ort.IsInitialized() {
		if !existing {
			return fmt.Errorf("oneocr: host already initialized ONNX Runtime; set UseExistingORT and keep the host environment alive")
		}
		environment.owned = false
		environment.path = ""
	} else {
		if existing {
			return fmt.Errorf("oneocr: UseExistingORT requires an initialized host environment")
		}
		if library == "" {
			return fmt.Errorf("oneocr: RuntimeLibrary is required (platform ONNX Runtime 1.29)")
		}
		ort.SetSharedLibraryPath(library)
		if e = ort.InitializeEnvironment(ort.WithLogLevelError()); e != nil {
			return e
		}
		environment.owned = true
		environment.path = library
	}
	var major, minor int
	if _, e = fmt.Sscanf(ort.GetVersion(), "%d.%d", &major, &minor); e != nil || major != 1 || minor < 29 {
		if environment.owned {
			ort.DestroyEnvironment()
		}
		environment.path = ""
		environment.owned = false
		return fmt.Errorf("oneocr: ONNX Runtime 1.29 or compatible newer 1.x is required")
	}
	environment.references = 1
	return nil
}
func releaseRuntime() error {
	environment.Lock()
	defer environment.Unlock()
	if environment.references <= 0 {
		return nil
	}
	environment.references--
	if environment.references > 0 {
		return nil
	}
	var e error
	if environment.owned {
		e = ort.DestroyEnvironment()
	}
	environment.owned = false
	environment.path = ""
	return e
}

type floatTensor struct {
	shape []int64
	data  []float32
}
type network struct {
	session                          *ort.DynamicAdvancedSession
	inputs, outputs, originalOutputs []string
	config                           Config
	original                         func() ([]byte, error)
	selected                         func() ([]byte, error)
	shapeSessions                    []*shapeSession
	profileDir                       string
	diagnostics                      StageDiagnostics
	fallbackUsed                     bool
}

func (n *network) destroySession() error {
	if n == nil || n.session == nil {
		return nil
	}
	err := n.session.Destroy()
	n.session = nil
	if n.profileDir != "" {
		n.collectProfile(n.profileDir)
		n.profileDir = ""
	}
	return err
}
func (n *network) close() error {
	if n == nil {
		return nil
	}
	return n.closeAllSessions()
}

func (n *network) fallBack(reason error) error {
	if n.fallbackUsed || n.config.Fallback != FallbackCPU || (n.diagnostics.Registered == BackendCPU && n.diagnostics.Recipe == "") {
		return reason
	}
	n.fallbackUsed = true
	model, err := n.original()
	if err != nil {
		return fmt.Errorf("%v; original model reload failed: %w", reason, err)
	}
	if err = n.destroySession(); err != nil {
		return fmt.Errorf("%v; close failed session: %w", reason, err)
	}
	if err = n.createSession(model, BackendCPU, n.originalOutputs); err != nil {
		return fmt.Errorf("%v; CPU fallback failed: %w", reason, err)
	}
	n.diagnostics.FallbackReason = reason.Error()
	n.diagnostics.Recipe = ""
	n.diagnostics.Experimental = false
	return nil
}

// runBorrowed owns every ORT value until consume returns. A consumer must never
// return or retain tensor-backed slices. Recognition decodes in this scope;
// detector/classifier callers use run(), which returns owned float copies.
func (n *network) runBorrowed(ctx context.Context, data floatTensor, extraFloat *floatTensor, sequence *int32, allowed []bool, consume func([]ort.Value) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if n == nil {
		return fmt.Errorf("oneocr: unavailable model session")
	}
	if n.config.ShapeCacheSize > 0 && n.diagnostics.Registered != BackendCPU {
		return n.runForShape(ctx, data, extraFloat, sequence, allowed, consume)
	}
	if n.session == nil {
		return fmt.Errorf("oneocr: unavailable model session")
	}
	for attempt := 0; attempt < 2; attempt++ {
		err := n.runAttempt(ctx, data, extraFloat, sequence, allowed, consume)
		if err == nil || ctx.Err() != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		// Only native Run failures trigger provider fallback. Decoder/validation
		// failures and invalid caller inputs are returned without retrying.
		native, ok := err.(*nativeRunError)
		if !ok || attempt > 0 {
			return err
		}
		if err = n.fallBack(native.err); err != nil {
			return err
		}
	}
	return fmt.Errorf("oneocr: exhausted backend attempts")
}

type nativeRunError struct{ err error }

func (e *nativeRunError) Error() string { return e.err.Error() }
func (e *nativeRunError) Unwrap() error { return e.err }

func (n *network) runAttempt(ctx context.Context, data floatTensor, extraFloat *floatTensor, sequence *int32, allowed []bool, consume func([]ort.Value) error) error {
	inputs := []ort.Value{}
	outputs := make([]ort.Value, len(n.outputs))
	defer func() {
		for _, v := range inputs {
			if v != nil {
				v.Destroy()
			}
		}
		for _, v := range outputs {
			if v != nil {
				v.Destroy()
			}
		}
	}()
	t, err := ort.NewTensor(ort.Shape(data.shape), data.data)
	if err != nil {
		return err
	}
	inputs = append(inputs, t)
	if extraFloat != nil {
		v, err := ort.NewTensor(ort.Shape(extraFloat.shape), extraFloat.data)
		if err != nil {
			return err
		}
		inputs = append(inputs, v)
	}
	if sequence != nil {
		v, err := ort.NewTensor(ort.NewShape(1), []int32{*sequence})
		if err != nil {
			return err
		}
		inputs = append(inputs, v)
	}
	if n.diagnostics.CompactOutput {
		if len(allowed) == 0 {
			return fmt.Errorf("oneocr: compact recognizer requires an alphabet mask")
		}
		v, err := ort.NewTensor(ort.NewShape(int64(len(allowed))), allowed)
		if err != nil {
			return err
		}
		inputs = append(inputs, v)
	}
	start := time.Now()
	if ctx.Done() == nil {
		err = n.session.Run(inputs, outputs)
	} else {
		options, e := ort.NewRunOptions()
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
		return &nativeRunError{err}
	}
	return consume(outputs)
}

func (n *network) run(ctx context.Context, data floatTensor, extraFloat *floatTensor, sequence *int32) (map[string]floatTensor, error) {
	var result map[string]floatTensor
	err := n.runBorrowed(ctx, data, extraFloat, sequence, nil, func(outputs []ort.Value) error {
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
