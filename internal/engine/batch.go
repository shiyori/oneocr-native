package engine

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
)

type batchValue[T any] struct {
	Index int
	Value T
	Err   error
	Done  bool
}

// runBatch executes a bounded, order-preserving batch. Individual operation
// failures are retained in the returned values; only invalid batch setup is
// returned as the method error.
func runBatch[T any](ctx context.Context, e *Engine, inputs []Input, run func(context.Context, Input) (T, error)) ([]batchValue[T], error) {
	if ctx == nil {
		return nil, errors.New("oneocr: nil context")
	}
	if e == nil || e.slots == nil {
		return nil, ErrClosed
	}
	values := make([]batchValue[T], len(inputs))
	for i := range values {
		values[i].Index = i
	}
	if len(inputs) == 0 {
		return values, nil
	}
	e.batchRequests.Add(1)

	jobs := make(chan int)
	workers := e.maxInFlight
	if workers < 1 {
		workers = 1
	}
	if workers > len(inputs) {
		workers = len(inputs)
	}
	var done sync.WaitGroup
	done.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer done.Done()
			for index := range jobs {
				values[index].Value, values[index].Err = run(ctx, inputs[index])
				values[index].Done = true
			}
		}()
	}

	for index := range inputs {
		select {
		case jobs <- index:
		case <-ctx.Done():
			close(jobs)
			done.Wait()
			markBatchCanceled(values, ctx.Err())
			return values, nil
		}
	}
	close(jobs)
	done.Wait()
	if ctx.Err() != nil {
		markBatchCanceled(values, ctx.Err())
	}
	return values, nil
}

func markBatchCanceled[T any](values []batchValue[T], cause error) {
	for i := range values {
		if !values[i].Done {
			values[i].Err = cause
			values[i].Done = true
		}
	}
}

func batchError(err error) *BatchError {
	if err == nil {
		return nil
	}
	code := "inference"
	var pathErr *os.PathError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		code = "timeout"
	case errors.Is(err, context.Canceled):
		code = "canceled"
	case errors.Is(err, ErrClosed):
		code = "closed"
	case errors.As(err, &pathErr):
		code = "invalid_input"
	case strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "empty input") || strings.Contains(err.Error(), "unknown script") || strings.Contains(err.Error(), "unknown format") || strings.HasPrefix(err.Error(), "image:"):
		code = "invalid_input"
	case strings.Contains(err.Error(), "unsupported") || strings.Contains(err.Error(), "unavailable script"):
		code = "unsupported"
	}
	return &BatchError{Code: code, Message: err.Error()}
}

// RecognizeBatch recognizes all inputs with bounded concurrency and preserves
// the input order in the returned per-item results.
func (e *Engine) RecognizeBatch(ctx context.Context, inputs []Input, options Options) ([]RecognitionBatchItem, error) {
	if err := validateOptions(options); err != nil {
		return nil, err
	}
	values, err := runBatch(ctx, e, inputs, func(ctx context.Context, input Input) (Result, error) {
		return e.Recognize(ctx, input, options)
	})
	if err != nil {
		return nil, err
	}
	items := make([]RecognitionBatchItem, len(values))
	for i, value := range values {
		items[i].Index = value.Index
		if value.Err != nil {
			items[i].Error = batchError(value.Err)
		} else {
			items[i].Result = &value.Value
		}
	}
	return items, nil
}

// DetectBatch runs detection for all inputs with bounded concurrency.
func (e *Engine) DetectBatch(ctx context.Context, inputs []Input) ([]DetectionBatchItem, error) {
	values, err := runBatch(ctx, e, inputs, func(ctx context.Context, input Input) (DetectionResult, error) {
		return e.Detect(ctx, input)
	})
	if err != nil {
		return nil, err
	}
	items := make([]DetectionBatchItem, len(values))
	for i, value := range values {
		items[i].Index = value.Index
		if value.Err != nil {
			items[i].Error = batchError(value.Err)
		} else {
			items[i].Result = &value.Value
		}
	}
	return items, nil
}

// RecognizeLineBatch recognizes cropped lines with bounded concurrency.
func (e *Engine) RecognizeLineBatch(ctx context.Context, inputs []Input, options Options) ([]LineBatchItem, error) {
	if err := validateOptions(options); err != nil {
		return nil, err
	}
	values, err := runBatch(ctx, e, inputs, func(ctx context.Context, input Input) (LineResult, error) {
		return e.RecognizeLine(ctx, input, options)
	})
	if err != nil {
		return nil, err
	}
	items := make([]LineBatchItem, len(values))
	for i, value := range values {
		items[i].Index = value.Index
		if value.Err != nil {
			items[i].Error = batchError(value.Err)
		} else {
			items[i].Result = &value.Value
		}
	}
	return items, nil
}
