package engine

import (
	"crypto/sha256"
	"fmt"

	"runtime"
)

func (e *Engine) openNetwork(resource, stage string, inputs, outputs []string) (*network, error) {
	model, err := e.source.read(resource)
	if err != nil {
		return nil, err
	}
	session, err := environment.runtime.NewSession(model, inputs, outputs, e.threads, runtime.GOOS == "android" && runtime.GOARCH == "arm64")
	if err != nil {
		return nil, fmt.Errorf("oneocr %s: %w", stage, err)
	}
	return &network{
		session:     session,
		outputs:     append([]string(nil), outputs...),
		diagnostics: StageDiagnostics{Stage: stage, ModelSHA256: fmt.Sprintf("%x", sha256.Sum256(model))},
	}, nil
}
