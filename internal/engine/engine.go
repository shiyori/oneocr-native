package engine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

type Engine struct {
	bundle               *Bundle
	source               *modelSource
	detector, classifier *network
	characters           map[string]CharacterModel
	recognizers          map[string]*recognizer
	threads, maxSide     int
	gate                 chan struct{}
	closed, heldRuntime  bool
	config               Config
	runtimeVersion       string
}

// Open validates the complete bundle and initializes native model sessions.
// One process can have multiple Engines sharing the same ORT environment.
func Open(config Config) (*Engine, error) {
	config.CharacterClasses = append([]CharacterClass(nil), config.CharacterClasses...)
	if _, err := characterClassSet(config.CharacterClasses); err != nil {
		return nil, err
	}
	if config.Threads == 0 {
		config.Threads = 2
	}
	if config.MaxSide == 0 {
		config.MaxSide = 1600
	}
	if config.Threads < 1 || config.Threads > 16 || config.MaxSide < 128 || config.MaxSide > 4096 {
		return nil, fmt.Errorf("oneocr: invalid Threads or MaxSide")
	}
	if err := applyDefaultConfig(&config); err != nil {
		return nil, err
	}
	source, err := openSource(config)
	if err != nil {
		return nil, err
	}
	bundle := source.bundle
	config.RuntimeLibrary, err = resolveRuntimeLibrary(config.RuntimeLibrary, config.ModelPath, config.BundleDir)
	if err != nil {
		source.close()
		return nil, err
	}
	if err = acquireRuntime(config.RuntimeLibrary); err != nil {
		source.close()
		return nil, err
	}
	e := &Engine{bundle: bundle, source: source, characters: map[string]CharacterModel{}, recognizers: map[string]*recognizer{}, threads: config.Threads, maxSide: config.MaxSide, gate: make(chan struct{}, 1), heldRuntime: true, config: config, runtimeVersion: environment.runtime.Version()}
	success := false
	defer func() {
		if !success {
			e.Close()
		}
	}()
	if e.detector, err = e.openNetwork(bundle.Pipeline.DetectorPath, "detector", []string{"data", "im_info"}, detectorOutputs()); err != nil {
		return nil, err
	}
	if e.classifier, err = e.openNetwork(bundle.Pipeline.ClassifierPath, "classifier", []string{"data"}, []string{"script_id_score", "flip_score"}); err != nil {
		return nil, err
	}
	for _, character := range bundle.Pipeline.Characters {
		e.characters[character.Script] = character
	}
	success = true
	return e, nil
}
func (e *Engine) lock(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("oneocr: nil context")
	}
	if e == nil || e.gate == nil {
		return ErrClosed
	}
	select {
	case e.gate <- struct{}{}:
		if e.closed {
			<-e.gate
			return ErrClosed
		}
		if ctx.Err() != nil {
			<-e.gate
			return ctx.Err()
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (e *Engine) unlock() { <-e.gate }

// Close waits for an active recognition to finish. It is idempotent. An ORT
// environment owned by the SDK is released when its last Engine closes.
func (e *Engine) Close() error {
	if e == nil || e.gate == nil {
		return nil
	}
	e.gate <- struct{}{}
	defer e.unlock()
	if e.closed {
		return nil
	}
	e.closed = true
	var failures []error
	failures = append(failures, e.detector.close(), e.classifier.close())
	for _, r := range e.recognizers {
		failures = append(failures, r.model.close())
	}
	if e.heldRuntime {
		e.heldRuntime = false
		failures = append(failures, releaseRuntime())
	}
	failures = append(failures, e.source.close())
	return errors.Join(failures...)
}
func (e *Engine) AvailableScripts() []string {
	if e == nil || e.bundle == nil {
		return nil
	}
	out := make([]string, 0, len(e.bundle.Pipeline.Characters))
	for _, c := range e.bundle.Pipeline.Characters {
		out = append(out, c.Script)
	}
	return out
}
func (e *Engine) getRecognizer(script string) (*recognizer, error) {
	if r := e.recognizers[script]; r != nil {
		return r, nil
	}
	config, ok := e.characters[script]
	if !ok {
		return nil, fmt.Errorf("oneocr: unavailable script %q", script)
	}
	letters, err := e.source.read(config.AlphabetPath)
	if err != nil {
		return nil, err
	}
	var composites []byte
	if config.CompositePath != "" {
		composites, err = e.source.read(config.CompositePath)
		if err != nil {
			return nil, err
		}
	}
	alphabet, err := readAlphabet(letters, composites)
	if err != nil {
		return nil, err
	}
	if script == "CJK" || script == "Latin" {
		if err = alphabet.setClasses(e.config.CharacterClasses); err != nil {
			return nil, err
		}
	}
	n, err := e.openNetwork(config.ModelPath, "recognizer/"+script, []string{"data", "seq_lengths"}, []string{"logsoftmax"})
	if err != nil {
		return nil, err
	}
	r := &recognizer{model: n, config: config, alphabet: alphabet}
	e.recognizers[script] = r
	return r, nil
}
func (e *Engine) classify(ctx context.Context, crop raster) (string, float64, error) {
	data, err := normalizeLine(crop, 4)
	if err != nil {
		return "", 0, err
	}
	out, err := e.classifier.run(ctx, data, nil, nil)
	if err != nil {
		return "", 0, err
	}
	scores := out["script_id_score"].data
	flip := out["flip_score"].data
	if len(scores) != 10 || len(flip) != 1 || math.IsNaN(float64(flip[0])) || math.IsInf(float64(flip[0]), 0) {
		return "", 0, fmt.Errorf("unexpected script classifier output")
	}
	best := 0
	for i, v := range scores {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return "", 0, fmt.Errorf("nonfinite classifier output")
		}
		if v > scores[best] {
			best = i
		}
	}
	return classifierScripts[best], float64(flip[0]), nil
}

// Recognize detects and reads text from an input. Coordinates have a zero origin.
func (e *Engine) Recognize(ctx context.Context, input Input, options Options) (Result, error) {
	if err := e.lock(ctx); err != nil {
		return Result{}, err
	}
	defer e.unlock()
	r, err := input.raster()
	if err != nil {
		return Result{}, err
	}
	return e.recognize(ctx, r, options)
}

func (e *Engine) recognize(ctx context.Context, r raster, options Options) (Result, error) {
	start := time.Now()
	if options.Script != "" {
		if _, ok := e.characters[options.Script]; !ok {
			return Result{}, fmt.Errorf("oneocr: unknown script %q", options.Script)
		}
	}
	detections, err := e.detect(ctx, r)
	if err != nil {
		return Result{}, err
	}
	if len(detections) > 1000 {
		return Result{}, fmt.Errorf("oneocr: more than 1000 detected regions; split image")
	}
	lines := []Line{}
	summary := recognitionResult{}
	unsupported := map[string]int{}
	quads := []Quad{}
	angles := []float64{}
	regions := make([]regionRecognition, len(detections))
	// Read reliable long text first; only metadata is retained, not pixel crops.
	for i, d := range detections {
		if err = ctx.Err(); err != nil {
			return Result{}, err
		}
		if compactRegion(d.quad) {
			continue
		}
		regions[i], err = e.recognizeRegion(ctx, r, d, options, pageOrientation{})
		if err != nil {
			return Result{}, err
		}
	}
	prior := inferPageOrientation(regions)
	for i, d := range detections {
		if err = ctx.Err(); err != nil {
			return Result{}, err
		}
		if compactRegion(d.quad) {
			regions[i], err = e.recognizeRegion(ctx, r, d, options, prior)
			if err != nil {
				return Result{}, err
			}
		}
		region := regions[i]
		if region.recognition.text == "" {
			if region.script != "" {
				if _, ok := e.characters[region.script]; !ok {
					unsupported[region.script]++
				}
			}
			continue
		}
		lines = append(lines, region.line)
		summary.logProbability += region.recognition.logProbability
		summary.tokens += region.recognition.tokens
		quads = append(quads, d.quad)
		angles = append(angles, region.angle)
	}

	sin, cos := 0., 0.
	for _, a := range angles {
		sin += math.Sin(a)
		cos += math.Cos(a)
	}
	direction := math.Atan2(sin, cos)
	sin, cos = math.Sincos(direction)
	layout := make([]Quad, len(quads))
	for i, q := range quads {
		for j, p := range q {
			layout[i][j] = Point{p[0]*cos + p[1]*sin, -p[0]*sin + p[1]*cos}
		}
	}
	rtl := 0
	for _, line := range lines {
		if line.Script == "Arabic" || line.Script == "Hebrew" {
			rtl++
		}
	}
	order := readingOrder(layout, rtl*2 > len(lines))
	ordered := make([]Line, 0, len(lines))
	text := make([]string, 0, len(lines))
	for _, i := range order {
		ordered = append(ordered, lines[i])
		text = append(text, lines[i].Text)
	}
	warnings := []string{"Experimental final quad fitting, normalization and reading order; original rejection/calibration are not applied."}
	for _, script := range scripts {
		if count := unsupported[script]; count > 0 {
			warnings = append(warnings, fmt.Sprintf("Skipped %d line(s) classified as %s: recognizer not included in this model package.", count, script))
		}
	}
	summary.text = strings.Join(text, "\n")
	return Result{Text: summary.text, Confidence: summary.confidence(), ConfidenceMethod: RecognitionConfidenceMethod, CoordinateSpace: "oriented_image", Lines: ordered, Width: r.width, Height: r.height, ElapsedSeconds: time.Since(start).Seconds(), ModelSHA256: e.bundle.SourceSHA256, Warnings: warnings}, nil
}
