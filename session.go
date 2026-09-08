package oneocr

import (
	"crypto/sha256"
	"fmt"

	ort "github.com/yalue/onnxruntime_go"
)

func (e *Engine) openNetwork(resource, stage string, inputs, outputs []string) (*network, error) {
	model, err := e.source.read(resource)
	if err != nil {
		return nil, err
	}
	options, err := ort.NewSessionOptions()
	if err != nil {
		return nil, err
	}
	defer options.Destroy()
	settings := []func() error{
		func() error { return options.SetIntraOpNumThreads(e.threads) },
		func() error { return options.SetInterOpNumThreads(1) },
		func() error { return options.SetExecutionMode(ort.ExecutionModeSequential) },
		func() error { return options.SetLogSeverityLevel(3) },
		func() error { return options.AddSessionConfigEntry("session.intra_op.allow_spinning", "0") },
		func() error { return options.AddSessionConfigEntry("session.inter_op.allow_spinning", "0") },
	}
	for _, set := range settings {
		if err = set(); err != nil {
			return nil, err
		}
	}
	session, err := ort.NewDynamicAdvancedSessionWithONNXData(model, inputs, outputs, options)
	if err != nil {
		return nil, fmt.Errorf("oneocr %s: %w", stage, err)
	}
	return &network{
		session:     session,
		outputs:     append([]string(nil), outputs...),
		diagnostics: StageDiagnostics{Stage: stage, ModelSHA256: fmt.Sprintf("%x", sha256.Sum256(model))},
	}, nil
}
