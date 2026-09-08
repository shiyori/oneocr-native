package oneocr

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

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
		t.Fatal("token decoding bypassed mask")
	}
	if _, err = a.decodeIDs([]int64{5}, "Latin"); err == nil {
		t.Fatal("out of range token ID")
	}
}
func TestQueuedCancellationAndClosedDiagnostics(t *testing.T) {
	e := &Engine{gate: make(chan struct{}, 1)}
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
