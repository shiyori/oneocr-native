package engine

import (
	"context"
	"fmt"
	"math"
	"sort"
)

var neighbors = [8][2]int{{-1, -1}, {-1, 0}, {-1, 1}, {0, -1}, {0, 1}, {1, -1}, {1, 0}, {1, 1}}

type detection struct {
	quad     Quad
	score    float64
	vertical bool
}

func detectorOutputs() []string {
	var names []string
	for _, level := range []int{2, 3, 4} {
		for _, direction := range []string{"hori", "vert"} {
			for _, head := range []string{"scores", "bbox_deltas", "link_scores"} {
				names = append(names, fmt.Sprintf("%s_%s_fpn%d", head, direction, level))
			}
		}
	}
	return names
}
func segmentGroups(ctx context.Context, scores, links []float32, w, h int, threshold float64) ([][]int, error) {
	n := w * h
	if len(scores) != n || len(links) != 8*n {
		return nil, fmt.Errorf("invalid Seglink tensor dimensions")
	}
	remaining := make([]bool, n)
	for i, score := range scores {
		remaining[i] = float64(score) >= threshold
	}
	var groups [][]int
	for seed := 0; seed < n; seed++ {
		if seed%1024 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !remaining[seed] {
			continue
		}
		remaining[seed] = false
		stack := []int{seed}
		var group []int
		for len(stack) > 0 {
			index := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			group = append(group, index)
			y, x := index/w, index%w
			for c, step := range neighbors {
				ny, nx := y+step[0], x+step[1]
				if ny < 0 || ny >= h || nx < 0 || nx >= w {
					continue
				}
				neighbor := ny*w + nx
				if !remaining[neighbor] {
					continue
				}
				if links[c*n+index] >= .8 || links[(7-c)*n+neighbor] >= .8 {
					remaining[neighbor] = false
					stack = append(stack, neighbor)
				}
			}
		}
		groups = append(groups, group)
	}
	return groups, nil
}
func decodeSegments(ctx context.Context, scores, delta, links floatTensor, stride int, threshold, lineThreshold float64) ([]detection, error) {
	if len(scores.shape) != 4 || scores.shape[0] != 1 || scores.shape[1] != 1 {
		return nil, fmt.Errorf("unexpected detector score shape")
	}
	h, w := int(scores.shape[2]), int(scores.shape[3])
	n := h * w
	if n <= 0 || len(delta.data) != 8*n {
		return nil, fmt.Errorf("unexpected detector regression shape")
	}
	groups, e := segmentGroups(ctx, scores.data, links.data, w, h, threshold)
	if e != nil {
		return nil, e
	}
	var out []detection
	for _, group := range groups {
		var sum float32
		for _, index := range group {
			sum += scores.data[index]
		}
		confidence := float64(sum / float32(len(group)))
		if confidence < lineThreshold {
			continue
		}
		points := make([]Point, 0, len(group)*4)
		for _, index := range group {
			cx := float64(index%w*stride) + float64(stride-1)/2
			cy := float64(index/w*stride) + float64(stride-1)/2
			for corner := 0; corner < 4; corner++ {
				x := cx + float64(delta.data[(corner*2)*n+index])*float64(8*stride-1)
				y := cy + float64(delta.data[(corner*2+1)*n+index])*float64(8*stride-1)
				if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) {
					return nil, fmt.Errorf("nonfinite detector output")
				}
				points = append(points, Point{x, y})
			}
		}
		quad := minimumRectangle(points)
		if polygonArea(quad[:]) >= 12 {
			out = append(out, detection{quad: quad, score: confidence})
		}
	}
	return out, nil
}
func (e *Engine) detect(ctx context.Context, r raster) ([]detection, error) {
	scale := math.Min(1, float64(e.maxSide)/float64(max(r.width, r.height)))
	w, h := max(1, round(float64(r.width)*scale)), max(1, round(float64(r.height)*scale))
	small := r.downscale(w, h)
	pw, ph := (w+31)/32*32, (h+31)/32*32
	padded := newRaster(pw, ph, true)
	for y := 0; y < h; y++ {
		copy(padded.pixels[y*pw*3:], small.pixels[y*w*3:(y+1)*w*3])
	}
	info := floatTensor{[]int64{1, 3}, []float32{float32(h), float32(w), 1}}
	outputs, err := e.detector.run(ctx, padded.nchw(1), &info, nil)
	if err != nil {
		return nil, err
	}
	var candidates []detection
	for _, level := range []int{2, 3, 4} {
		for _, direction := range []string{"hori", "vert"} {
			suffix := fmt.Sprintf("_%s_fpn%d", direction, level)
			decoded, err := decodeSegments(ctx, outputs["scores"+suffix], outputs["bbox_deltas"+suffix], outputs["link_scores"+suffix], 1<<level, e.bundle.Pipeline.SegmentThreshold, e.bundle.Pipeline.LineThresholds[level])
			if err != nil {
				return nil, err
			}
			for _, d := range decoded {
				for i := range d.quad {
					d.quad[i][0] = clamp(d.quad[i][0]*float64(r.width)/float64(w), 0, float64(r.width-1))
					d.quad[i][1] = clamp(d.quad[i][1]*float64(r.height)/float64(h), 0, float64(r.height-1))
				}
				d.vertical = direction == "vert"
				if polygonArea(d.quad[:]) >= 12 {
					candidates = append(candidates, d)
				}
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	kept := make([]detection, 0, len(candidates))
	for _, d := range candidates {
		suppress := false
		for _, other := range kept {
			if polygonIoU(d.quad, other.quad) > .2 {
				suppress = true
				break
			}
		}
		if !suppress {
			kept = append(kept, d)
		}
	}
	return kept, nil
}
