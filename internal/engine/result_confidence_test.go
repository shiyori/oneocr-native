package engine

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestCTCConfidence(t *testing.T) {
	a := alphabet{characters: []string{"a", "<blank>"}, blank: 1}
	data := []float32{}
	for _, p := range []float64{0.8, 0.99, 0.1, 0.6} {
		data = append(data, float32(math.Log(p)), float32(math.Log(1-p)))
	}
	r, err := a.decodeScored(floatTensor{[]int64{4, 1, 2}, data}, "Latin")
	if err != nil || r.text != "aa" || r.tokens != 2 || r.confidence() == nil || math.Abs(*r.confidence()-math.Sqrt(0.8*0.6)) > 1e-7 {
		t.Fatalf("result=%+v, err=%v", r, err)
	}
	// Adding a constant to all logits must not change the probability.
	for i := range data {
		data[i] += 100
	}
	shifted, err := a.decodeScored(floatTensor{[]int64{4, 1, 2}, data}, "Latin")
	if err != nil || math.Abs(*shifted.confidence()-*r.confidence()) > 1e-5 {
		t.Fatal(shifted, err)
	}
}

func TestCTCConfidenceTrimmingAndFiltering(t *testing.T) {
	a := alphabet{characters: []string{"a", "<space>", "<blank>"}, blank: 2}
	data := []float32{}
	for _, row := range [][3]float64{{.1, .8, .1}, {.6, .2, .2}, {.1, .8, .1}} {
		for _, p := range row {
			data = append(data, float32(math.Log(p)))
		}
	}
	r, err := a.decodeScored(floatTensor{[]int64{3, 1, 3}, data}, "Latin")
	if err != nil || r.text != "a" || r.tokens != 1 || math.Abs(*r.confidence()-.6) > 1e-7 {
		t.Fatal(r, err)
	}
	a.allowed = []bool{true, false, true}
	r, err = a.decodeScored(floatTensor{[]int64{1, 1, 3}, []float32{0, 10, -1}}, "Latin")
	if err != nil || r.text != "a" || *r.confidence() > .001 {
		t.Fatal("filter inflated confidence", r, err)
	}
	for _, data := range [][]float32{{-2, -3, 0}, {}} {
		r, err = a.decodeScored(floatTensor{[]int64{int64(len(data) / 3), 1, 3}, data}, "Latin")
		if err != nil || r.text != "" || r.confidence() != nil {
			t.Fatal(r, err)
		}
	}
}

func TestQuadBounds(t *testing.T) {
	got := quadBounds(Quad{{3, 1}, {8, 4}, {5, 9}, {0, 6}})
	if got != (Box{X: 0, Y: 1, Width: 8, Height: 8}) {
		t.Fatal(got)
	}
}

func TestStructuredResults(t *testing.T) {
	useRepositoryFixtures(t)
	if testing.Short() || os.Getenv("ONEOCR_RUNTIME") == "" {
		t.Skip("requires native runtime")
	}
	e, err := Open(Config{ModelPath: "models/" + DefaultModelName, Threads: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	ctx := context.Background()
	r, err := e.Recognize(ctx, FromFile("testdata/CJK.png"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Text != "你好世界 日本語テスト 한국어 123" || r.Confidence == nil || *r.Confidence <= 0 || *r.Confidence > 1 || r.ConfidenceMethod != RecognitionConfidenceMethod || r.CoordinateSpace != "oriented_image" || len(r.Lines) == 0 {
		t.Fatalf("%+v", r)
	}
	for _, l := range r.Lines {
		if l.Confidence == nil || *l.Confidence < 0 || *l.Confidence > 1 || l.BBox != quadBounds(l.Quad) || l.BBox.Width <= 0 || l.DetectionScore <= 0 {
			t.Fatalf("%+v", l)
		}
	}
	d, err := e.Detect(ctx, FromFile("testdata/CJK.png"))
	if err != nil {
		t.Fatal(err)
	}
	if d.CoordinateSpace != "oriented_image" || len(d.Regions) == 0 || d.Regions[0].BBox != quadBounds(d.Regions[0].Quad) {
		t.Fatalf("%+v", d)
	}
	l, err := e.RecognizeLine(ctx, FromFile("scripts/testdata/cjk-line.png"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if l.Text != r.Text || l.Confidence == nil || *l.Confidence <= 0 || *l.Confidence > 1 || l.Width <= 0 || l.Height <= 0 || l.ConfidenceMethod != RecognitionConfidenceMethod {
		t.Fatalf("%+v", l)
	}
	empty, err := e.Recognize(ctx, FromFile("testdata/blank.png"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Text != "" || empty.Confidence != nil || len(empty.Lines) != 0 {
		t.Fatalf("%+v", empty)
	}
	for _, value := range []any{r, d, l, empty} {
		if _, err := json.Marshal(value); err != nil {
			t.Fatal(err)
		}
	}
}
