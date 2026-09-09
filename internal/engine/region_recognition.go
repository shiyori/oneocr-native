package engine

import (
	"context"
	"math"
	"unicode/utf8"
)

// Near-square proposals do not establish whether a glyph belongs to a vertical
// line. In particular, the vertical detector also fires on isolated digits.
func regionAspect(q Quad) float64 {
	q = orderedQuad(q)
	w := math.Max(length(sub(q[1], q[0])), length(sub(q[2], q[3])))
	h := math.Max(length(sub(q[3], q[0])), length(sub(q[2], q[1])))
	if w <= 0 || h <= 0 {
		return math.Inf(1)
	}
	return math.Max(w, h) / math.Min(w, h)
}

func compactRegion(q Quad) bool { return regionAspect(q) <= 2 }

func rotateQuarter(r raster, turns int) raster {
	switch turns % 4 {
	case 1:
		return r.orient(8)
	case 2:
		return r.orient(3)
	case 3:
		return r.orient(6)
	}
	return r
}

type regionRecognition struct {
	line        Line
	recognition recognitionResult
	script      string
	angle       float64
	anchor      bool
}
type pageOrientation struct {
	angle float64
	valid bool
}

func inferPageOrientation(regions []regionRecognition) pageOrientation {
	var x, y, weight float64
	for _, r := range regions {
		if !r.anchor {
			continue
		}
		w := float64(min(20, utf8.RuneCountInString(r.recognition.text)))
		x += w * math.Cos(r.angle)
		y += w * math.Sin(r.angle)
		weight += w
	}
	if weight == 0 || math.Hypot(x, y)/weight < .90 {
		return pageOrientation{}
	}
	return pageOrientation{math.Atan2(y, x), true}
}
func (p pageOrientation) turns(q Quad) int {
	if !p.valid {
		return 0
	}
	q = orderedQuad(q)
	edge := sub(q[1], q[0])
	delta := p.angle - math.Atan2(edge[1], edge[0])
	delta = math.Atan2(math.Sin(delta), math.Cos(delta))
	n := int(math.Round(delta / (math.Pi / 2)))
	if math.Abs(delta-float64(n)*math.Pi/2) > math.Pi/6 {
		return 0
	}
	return (n + 4) % 4
}

func asciiDigits(text string) bool {
	if len(text) == 0 || len(text) > 3 {
		return false
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Both models must agree on a short numeral. Keep the Latin model's actual
// score instead of maximizing confidence across recognizers or orientations.
func numericConsensus(latin, cjk recognitionResult) bool {
	a, b := latin.confidence(), cjk.confidence()
	return asciiDigits(latin.text) && latin.text == cjk.text && a != nil && b != nil && *a >= .90 && *b >= .50
}
func (e *Engine) recognizeUnknownNumeral(ctx context.Context, crop raster) (recognitionResult, string, error) {
	for _, s := range []string{"Latin", "CJK"} {
		if _, ok := e.characters[s]; !ok {
			return recognitionResult{}, "", nil
		}
	}
	a, err := e.getRecognizer("Latin")
	if err != nil {
		return recognitionResult{}, "", err
	}
	latin, err := a.run(ctx, crop)
	if err != nil {
		return recognitionResult{}, "", err
	}
	confidence := latin.confidence()
	if !asciiDigits(latin.text) || confidence == nil || *confidence < .90 {
		return recognitionResult{}, "", nil
	}
	b, err := e.getRecognizer("CJK")
	if err != nil {
		return recognitionResult{}, "", err
	}
	cjk, err := b.run(ctx, crop)
	if err != nil {
		return recognitionResult{}, "", err
	}
	if !numericConsensus(latin, cjk) {
		return recognitionResult{}, "", nil
	}
	return latin, "Latin", nil
}

func (e *Engine) recognizeRegion(ctx context.Context, image raster, d detection, options Options, prior pageOrientation) (regionRecognition, error) {
	var out regionRecognition
	compact := compactRegion(d.quad)
	useVertical := d.vertical && (!compact || (!prior.valid && regionAspect(d.quad) > 1.5))
	crop, err := rectify(image, d.quad, useVertical)
	if err != nil {
		return out, err
	}
	q := orderedQuad(d.quad)
	quarter := 0
	if useVertical {
		w := max(2, round(math.Max(length(sub(q[1], q[0])), length(sub(q[2], q[3])))))
		h := max(2, round(math.Max(length(sub(q[3], q[0])), length(sub(q[2], q[1])))))
		if h > w {
			quarter = 1
		}
	}
	if compact && prior.valid {
		quarter = prior.turns(q)
		crop = rotateQuarter(crop, quarter)
	}
	script, flip, err := e.classify(ctx, crop)
	if err != nil {
		return out, err
	}
	if options.Script != "" {
		script = options.Script
	}
	// The sign of a near-zero flip logit is not reliable for a compact token.
	// Preserve the page/input orientation unless the classifier has clear evidence.
	if flip < 0 && (!compact || math.Abs(flip) >= 2) {
		crop = crop.orient(3)
		quarter = (quarter + 2) % 4
	}
	out.script = script
	var recognized recognitionResult
	if script == "" {
		if compact && d.score >= .80 {
			recognized, script, err = e.recognizeUnknownNumeral(ctx, crop)
		}
		if err != nil {
			return out, err
		}
	} else if _, ok := e.characters[script]; ok {
		rec, err := e.getRecognizer(script)
		if err != nil {
			return out, err
		}
		recognized, err = rec.run(ctx, crop)
		if err != nil {
			return out, err
		}
	}
	if recognized.text == "" {
		return out, nil
	}
	quad := d.quad
	for i := range quad {
		for j := range quad[i] {
			quad[i][j] = math.RoundToEven(quad[i][j]*1000) / 1000
		}
	}
	out.recognition = recognized
	out.line = Line{Text: recognized.text, Quad: quad, BBox: quadBounds(quad), Script: script, Confidence: recognized.confidence(), DetectionScore: d.score, Vertical: d.vertical, Rotated180: quarter >= 2, RotationDegrees: (360 - quarter*90) % 360}
	edge := sub(q[1], q[0])
	out.angle = math.Atan2(edge[1], edge[0]) + float64(quarter)*math.Pi/2
	confidence := recognized.confidence()
	out.anchor = !compact && math.Abs(flip) >= 2 && utf8.RuneCountInString(recognized.text) >= 3 && confidence != nil && *confidence >= .80
	return out, nil
}
