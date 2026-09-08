// Evaluate the same generated labelled images used by the Python reference.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	oneocr "github.com/shiyori/oneocr-native"
	"golang.org/x/text/unicode/norm"
)

type annotation struct{ File, Text, Source string }
type measurement struct {
	File     string        `json:"file"`
	Expected string        `json:"expected"`
	Actual   string        `json:"actual"`
	Errors   int           `json:"character_errors"`
	Exact    bool          `json:"exact_match"`
	Result   oneocr.Result `json:"result"`
}

func distance(a, b string) int {
	x, y := []rune(a), []rune(b)
	prev := make([]int, len(y)+1)
	for i := range prev {
		prev[i] = i
	}
	for i, c := range x {
		next := make([]int, len(y)+1)
		next[0] = i + 1
		for j, d := range y {
			cost := 0
			if c != d {
				cost = 1
			}
			next[j+1] = min(next[j]+1, prev[j+1]+1, prev[j]+cost)
		}
		prev = next
	}
	return prev[len(y)]
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 5 {
		return fmt.Errorf("usage: evaluate BUNDLE RUNTIME FIXTURES OUTPUT_JSON")
	}
	data, e := os.ReadFile(filepath.Join(os.Args[3], "annotations.json"))
	if e != nil {
		return e
	}
	var labels []annotation
	if e = json.Unmarshal(data, &labels); e != nil {
		return e
	}
	start := time.Now()
	engine, e := oneocr.Open(oneocr.Config{BundleDir: os.Args[1], RuntimeLibrary: os.Args[2]})
	if e != nil {
		return e
	}
	defer engine.Close()
	initialization := time.Since(start).Seconds()
	cases := []measurement{}
	exact, totalErrors, totalCharacters := 0, 0, 0
	for _, label := range labels {
		if label.Source != "" {
			continue
		}
		result, e := engine.RecognizeFile(context.Background(), filepath.Join(os.Args[3], label.File), oneocr.Options{})
		if e != nil {
			return fmt.Errorf("%s: %w", label.File, e)
		}
		expected, actual := norm.NFC.String(label.Text), norm.NFC.String(result.Text)
		errors := distance(expected, actual)
		ok := errors == 0
		if ok {
			exact++
		}
		totalErrors += errors
		totalCharacters += len([]rune(expected))
		cases = append(cases, measurement{label.File, expected, actual, errors, ok, result})
		fmt.Fprintln(os.Stderr, label.File, "errors=", errors)
	}
	report := map[string]any{"platform": runtime.GOOS + "-" + runtime.GOARCH, "go_version": runtime.Version(), "provider": "CPUExecutionProvider", "initialization_seconds": initialization, "cases": cases, "exact_matches": exact, "reference_characters": totalCharacters, "character_errors": totalErrors, "character_error_rate": float64(totalErrors) / float64(max(1, totalCharacters))}
	encoded, e := json.MarshalIndent(report, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(os.Args[4], append(encoded, '\n'), 0644)
}
