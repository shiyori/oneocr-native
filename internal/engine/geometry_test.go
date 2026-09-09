package engine

import (
	"context"
	"math"
	"reflect"
	"testing"
)

func rectangle(x, y, w, h float64) Quad { return Quad{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}} }
func TestGeometry(t *testing.T) {
	q := minimumRectangle([]Point{{1, 2}, {11, 2}, {11, 7}, {1, 7}, {5, 4}})
	if q != rectangle(1, 2, 10, 5) {
		t.Fatal(q)
	}
	if v := polygonIoU(rectangle(0, 0, 10, 10), rectangle(5, 0, 10, 10)); math.Abs(v-1./3) > 1e-6 {
		t.Fatal(v)
	}
	quads := []Quad{rectangle(0, 0, 100, 20), rectangle(200, 0, 100, 20), rectangle(0, 80, 100, 20), rectangle(200, 80, 100, 20)}
	if got := readingOrder(quads, false); !reflect.DeepEqual(got, []int{0, 2, 1, 3}) {
		t.Fatal(got)
	}
	if got := readingOrder(quads, true); !reflect.DeepEqual(got, []int{1, 3, 0, 2}) {
		t.Fatal(got)
	}
}
func TestBidirectionalLinks(t *testing.T) {
	links := make([]float32, 16)
	groups, e := segmentGroups(context.Background(), []float32{.9, .9}, links, 2, 1, .7)
	if e != nil || len(groups) != 2 {
		t.Fatal(groups, e)
	}
	links[3*2+1] = .9
	groups, e = segmentGroups(context.Background(), []float32{.9, .9}, links, 2, 1, .7)
	if e != nil || len(groups) != 1 {
		t.Fatal(groups, e)
	}
}
func TestAnchorCoordinates(t *testing.T) {
	scores := make([]float32, 64)
	scores[3*8+3] = .9
	scores[3*8+4] = .9
	deltas := make([]float32, 8*64)
	offsets := []float32{-.5, -.5, .5, -.5, .5, .5, -.5, .5}
	for c, v := range offsets {
		deltas[c*64+3*8+3] = v
		deltas[c*64+3*8+4] = v
	}
	links := make([]float32, 8*64)
	for i := range links {
		links[i] = 1
	}
	boxes, e := decodeSegments(context.Background(), floatTensor{[]int64{1, 1, 8, 8}, scores}, floatTensor{[]int64{1, 8, 8, 8}, deltas}, floatTensor{[]int64{1, 8, 8, 8}, links}, 4, .7, .8)
	if e != nil || len(boxes) != 1 {
		t.Fatal(boxes, e)
	}
	if boxes[0].quad != rectangle(-2, -2, 35, 31) {
		t.Fatal(boxes[0].quad)
	}
}
