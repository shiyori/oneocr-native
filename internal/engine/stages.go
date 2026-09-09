package engine

import (
	"context"
	"fmt"
	"math"
	"time"
)

// Detection is a detector proposal, before script classification and recognition.
// Quad coordinates use the oriented input image with zero origin; Score is an
// uncalibrated detector score, not a recognition confidence. Order is unspecified.
type Detection struct {
	Quad     Quad    `json:"quad"`
	BBox     Box     `json:"bbox"`
	Score    float64 `json:"score"`
	Vertical bool    `json:"vertical"`
}
type DetectionResult struct {
	CoordinateSpace string      `json:"coordinate_space"`
	Regions         []Detection `json:"regions"`
	Width           int         `json:"width"`
	Height          int         `json:"height"`
	ElapsedSeconds  float64     `json:"elapsed_seconds"`
	ModelSHA256     string      `json:"model_sha256"`
}

// LineResult describes a single already cropped, horizontally laid-out line.
// Empty Script enables classification and automatic 180-degree correction.
// Explicit Script skips classification and assumes an upright crop.
type LineResult struct {
	Confidence       *float64 `json:"confidence"`
	ConfidenceMethod string   `json:"confidence_method"`
	Width            int      `json:"width"`
	Height           int      `json:"height"`
	Text             string   `json:"text"`
	Script           string   `json:"script"`
	Rotated180       bool     `json:"rotated_180"`
	RotationDegrees  int      `json:"rotation_degrees"`
	ElapsedSeconds   float64  `json:"elapsed_seconds"`
	ModelSHA256      string   `json:"model_sha256"`
}

func (e *Engine) detectOnly(ctx context.Context, r raster) (DetectionResult, error) {
	start := time.Now()
	ds, err := e.detect(ctx, r)
	if err != nil {
		return DetectionResult{}, err
	}
	if len(ds) > 1000 {
		return DetectionResult{}, fmt.Errorf("oneocr: more than 1000 detected regions; split image")
	}
	out := DetectionResult{CoordinateSpace: "oriented_image", Regions: make([]Detection, 0, len(ds)), Width: r.width, Height: r.height, ModelSHA256: e.bundle.SourceSHA256}
	for _, d := range ds {
		out.Regions = append(out.Regions, Detection{Quad: d.quad, BBox: quadBounds(d.quad), Score: d.score, Vertical: d.vertical})
	}
	out.ElapsedSeconds = time.Since(start).Seconds()
	return out, nil
}
func (e *Engine) recognizeLine(ctx context.Context, r raster, options Options) (LineResult, error) {
	start := time.Now()
	script := options.Script
	rotated := false
	compact := float64(max(r.width, r.height)) <= 2*float64(min(r.width, r.height))
	if script == "" {
		var flip float64
		var err error
		script, flip, err = e.classify(ctx, r)
		if err != nil {
			return LineResult{}, err
		}
		if flip < 0 && (!compact || math.Abs(flip) >= 2) {
			r = r.orient(3)
			rotated = true
		}
	}
	var recognized recognitionResult
	var err error
	if script == "" {
		if compact {
			recognized, script, err = e.recognizeUnknownNumeral(ctx, r)
		}
	} else {
		var rec *recognizer
		rec, err = e.getRecognizer(script)
		if err == nil {
			recognized, err = rec.run(ctx, r)
		}
	}
	if err != nil {
		return LineResult{}, err
	}
	rotation := 0
	if rotated {
		rotation = 180
	}
	return LineResult{Text: recognized.text, Confidence: recognized.confidence(), ConfidenceMethod: RecognitionConfidenceMethod, Width: r.width, Height: r.height, Script: script, Rotated180: rotated, RotationDegrees: rotation, ElapsedSeconds: time.Since(start).Seconds(), ModelSHA256: e.bundle.SourceSHA256}, nil
}

// rgbRaster copies the buffer; callers may reuse it after the synchronous call.
func rgbRaster(data []byte, width, height, stride int) (raster, error) {
	if width < 2 || height < 2 || width > maxImagePixels || height > maxImagePixels || int64(width)*int64(height) > maxImagePixels || stride < width*3 || stride > len(data) || int64(height-1)*int64(stride)+int64(width)*3 > int64(len(data)) {
		return raster{}, fmt.Errorf("oneocr: invalid RGB buffer dimensions")
	}
	r := newRaster(width, height, false)
	for y := 0; y < height; y++ {
		copy(r.pixels[y*width*3:(y+1)*width*3], data[y*stride:y*stride+width*3])
	}
	return r, nil
}

// Detect runs the requested stage with the Engine's cancellation gate.
func (e *Engine) Detect(ctx context.Context, input Input) (DetectionResult, error) {
	if err := e.lock(ctx); err != nil {
		return DetectionResult{}, err
	}
	defer e.unlock()
	r, err := input.raster()
	if err != nil {
		return DetectionResult{}, err
	}
	return e.detectOnly(ctx, r)
}

// RecognizeLine runs the requested stage with the Engine's cancellation gate.
func (e *Engine) RecognizeLine(ctx context.Context, input Input, options Options) (LineResult, error) {
	if err := e.lock(ctx); err != nil {
		return LineResult{}, err
	}
	defer e.unlock()
	r, err := input.raster()
	if err != nil {
		return LineResult{}, err
	}
	return e.recognizeLine(ctx, r, options)
}
