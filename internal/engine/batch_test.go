package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestBatchOrderAndPerItemErrors(t *testing.T) {
	e := &Engine{slots: make(chan struct{}, 2), maxInFlight: 2}
	inputs := []Input{{path: "0"}, {path: "1"}, {path: "2"}}
	values, err := runBatch(context.Background(), e, inputs, func(_ context.Context, input Input) (string, error) {
		if input.path == "1" {
			return "", fmt.Errorf("invalid input")
		}
		time.Sleep(2 * time.Millisecond)
		return input.path, nil
	})
	if err != nil || len(values) != 3 {
		t.Fatal(err, values)
	}
	if values[0].Value != "0" || values[2].Value != "2" || values[1].Err == nil || !values[1].Done {
		t.Fatalf("unexpected batch values: %+v", values)
	}
	if got := batchError(values[1].Err); got.Code != "invalid_input" {
		t.Fatalf("unexpected error code: %+v", got)
	}
	if e.batchRequests.Load() != 1 {
		t.Fatalf("batch metric not incremented: %d", e.batchRequests.Load())
	}
}

func TestBatchCancellationMarksPendingItems(t *testing.T) {
	e := &Engine{slots: make(chan struct{}, 1), maxInFlight: 1}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	values, err := runBatch(ctx, e, []Input{{}, {}}, func(context.Context, Input) (int, error) {
		return 1, nil
	})
	if err != nil || len(values) != 2 {
		t.Fatal(err, values)
	}
	for _, value := range values {
		if !value.Done || !errors.Is(value.Err, context.Canceled) {
			t.Fatalf("item was not canceled: %+v", value)
		}
	}
}

func TestCloseWaitsForActiveOperation(t *testing.T) {
	slots := make(chan struct{}, 1)
	e := &Engine{slots: slots, gate: slots, maxInFlight: 1}
	if err := e.lock(context.Background()); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- e.Close() }()
	select {
	case err := <-done:
		t.Fatalf("Close returned while operation was active: %v", err)
	case <-time.After(10 * time.Millisecond):
	}
	e.unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not wait for the operation to finish")
	}
}

func TestOptionsValidation(t *testing.T) {
	for _, order := range []ReadingOrderMode{"bad", ""} {
		err := validateOptions(Options{ReadingOrder: order})
		if order == "" && err != nil {
			t.Fatal(err)
		}
		if order == "bad" && err == nil {
			t.Fatal("accepted invalid reading order")
		}
	}
	if err := validateOptions(Options{MinLineConfidence: 1.1}); err == nil {
		t.Fatal("accepted invalid confidence")
	}
}
