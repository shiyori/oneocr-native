package engine

import (
	"context"
	"fmt"
)

// Warmup loads every recognizer included in the package and runs small, valid
// inputs through all stages. Reuse the Engine afterwards. For a production
// input shape, also run Recognize once with a representative image before
// accepting traffic. Context cancellation also cancels waiting for the Engine.
func (e *Engine) Warmup(ctx context.Context) error {
	if err := e.lock(ctx); err != nil {
		return err
	}
	defer e.unlock()
	image := newRaster(128, 128, true)
	if _, err := e.detect(ctx, image); err != nil {
		return fmt.Errorf("warmup detector: %w", err)
	}
	line := newRaster(96, 60, true)
	if _, _, err := e.classify(ctx, line); err != nil {
		return fmt.Errorf("warmup classifier: %w", err)
	}
	for _, script := range e.AvailableScripts() {
		if err := ctx.Err(); err != nil {
			return err
		}
		r, err := e.getRecognizer(script)
		if err != nil {
			return fmt.Errorf("warmup %s: %w", script, err)
		}
		if _, err = r.run(ctx, line); err != nil {
			return fmt.Errorf("warmup %s: %w", script, err)
		}
	}
	return nil
}
