package engine

import (
	"fmt"
	"math"
	"sort"
)

func sub(a, b Point) Point     { return Point{a[0] - b[0], a[1] - b[1]} }
func dot(a, b Point) float64   { return a[0]*b[0] + a[1]*b[1] }
func cross(a, b Point) float64 { return a[0]*b[1] - a[1]*b[0] }
func length(a Point) float64   { return math.Hypot(a[0], a[1]) }
func orderedQuad(q Quad) Quad {
	center := Point{}
	for _, p := range q {
		center[0] += p[0] / 4
		center[1] += p[1] / 4
	}
	points := q
	sort.SliceStable(points[:], func(i, j int) bool {
		return math.Atan2(points[i][1]-center[1], points[i][0]-center[0]) < math.Atan2(points[j][1]-center[1], points[j][0]-center[0])
	})
	start := 0
	for i := 1; i < 4; i++ {
		if points[i][0]+points[i][1] < points[start][0]+points[start][1] {
			start = i
		}
	}
	for i := 0; i < 4; i++ {
		q[i] = points[(start+i)%4]
	}
	return q
}
func hull(points []Point) []Point {
	points = append([]Point(nil), points...)
	sort.Slice(points, func(i, j int) bool {
		if points[i][0] == points[j][0] {
			return points[i][1] < points[j][1]
		}
		return points[i][0] < points[j][0]
	})
	unique := points[:0]
	for _, p := range points {
		if len(unique) == 0 || p != unique[len(unique)-1] {
			unique = append(unique, p)
		}
	}
	points = unique
	if len(points) <= 2 {
		return points
	}
	appendHull := func(dst []Point, p Point) []Point {
		for len(dst) >= 2 && cross(sub(dst[len(dst)-1], dst[len(dst)-2]), sub(p, dst[len(dst)-1])) <= 0 {
			dst = dst[:len(dst)-1]
		}
		return append(dst, p)
	}
	var lower, upper []Point
	for _, p := range points {
		lower = appendHull(lower, p)
	}
	for i := len(points) - 1; i >= 0; i-- {
		upper = appendHull(upper, points[i])
	}
	return append(lower[:len(lower)-1], upper[:len(upper)-1]...)
}
func minimumRectangle(points []Point) Quad {
	outline := hull(points)
	best := math.Inf(1)
	var result Quad
	for i, p := range outline {
		edge := sub(outline[(i+1)%len(outline)], p)
		norm := length(edge)
		if norm < 1e-12 {
			continue
		}
		u := Point{edge[0] / norm, edge[1] / norm}
		v := Point{-u[1], u[0]}
		minU, maxU, minV, maxV := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
		for _, q := range outline {
			x, y := dot(q, u), dot(q, v)
			minU = math.Min(minU, x)
			maxU = math.Max(maxU, x)
			minV = math.Min(minV, y)
			maxV = math.Max(maxV, y)
		}
		area := (maxU - minU) * (maxV - minV)
		if area < best-1e-7 {
			best = area
			for j, pair := range [4]Point{{minU, minV}, {maxU, minV}, {maxU, maxV}, {minU, maxV}} {
				result[j] = Point{u[0]*pair[0] + v[0]*pair[1], u[1]*pair[0] + v[1]*pair[1]}
			}
		}
	}
	return orderedQuad(result)
}
func polygonArea(points []Point) float64 {
	sum := 0.
	for i, p := range points {
		sum += cross(p, points[(i+1)%len(points)])
	}
	return math.Abs(sum) / 2
}
func polygonIoU(a, b Quad) float64 {
	aa, bb := polygonArea(a[:]), polygonArea(b[:])
	if aa <= 0 || bb <= 0 {
		return 0
	}
	clipped := append([]Point(nil), a[:]...)
	for edge, c := range b {
		next := b[(edge+1)%4]
		dir := sub(next, c)
		input := clipped
		clipped = nil
		if len(input) == 0 {
			break
		}
		previous := input[len(input)-1]
		previousD := cross(dir, sub(previous, c))
		for _, current := range input {
			d := cross(dir, sub(current, c))
			inside, wasInside := d >= -1e-7, previousD >= -1e-7
			if inside != wasInside {
				ratio := previousD / (previousD - d)
				clipped = append(clipped, Point{previous[0] + ratio*(current[0]-previous[0]), previous[1] + ratio*(current[1]-previous[1])})
			}
			if inside {
				clipped = append(clipped, current)
			}
			previous, previousD = current, d
		}
	}
	intersection := polygonArea(clipped)
	return intersection / math.Max(aa+bb-intersection, 1e-6)
}

func solve8(a [8][9]float64) ([8]float64, error) {
	for col := 0; col < 8; col++ {
		pivot := col
		for row := col + 1; row < 8; row++ {
			if math.Abs(a[row][col]) > math.Abs(a[pivot][col]) {
				pivot = row
			}
		}
		if math.Abs(a[pivot][col]) < 1e-12 {
			return [8]float64{}, fmt.Errorf("degenerate text quadrilateral")
		}
		a[col], a[pivot] = a[pivot], a[col]
		div := a[col][col]
		for k := col; k < 9; k++ {
			a[col][k] /= div
		}
		for row := 0; row < 8; row++ {
			if row == col {
				continue
			}
			factor := a[row][col]
			for k := col; k < 9; k++ {
				a[row][k] -= factor * a[col][k]
			}
		}
	}
	var result [8]float64
	for i := range result {
		result[i] = a[i][8]
	}
	return result, nil
}
func rectify(r raster, quad Quad, vertical bool) (raster, error) {
	q := orderedQuad(quad)
	w := max(2, round(math.Max(length(sub(q[1], q[0])), length(sub(q[2], q[3])))))
	h := max(2, round(math.Max(length(sub(q[3], q[0])), length(sub(q[2], q[1])))))
	if int64(w)*int64(h) > maxImagePixels {
		return raster{}, fmt.Errorf("text crop exceeds pixel limit")
	}
	target := Quad{{0, 0}, {float64(w - 1), 0}, {float64(w - 1), float64(h - 1)}, {0, float64(h - 1)}}
	var equations [8][9]float64
	for i, p := range target {
		x, y := p[0], p[1]
		sx, sy := q[i][0], q[i][1]
		equations[2*i] = [9]float64{x, y, 1, 0, 0, 0, -sx * x, -sx * y, sx}
		equations[2*i+1] = [9]float64{0, 0, 0, x, y, 1, -sy * x, -sy * y, sy}
	}
	m, e := solve8(equations)
	if e != nil {
		return raster{}, e
	}
	out := newRaster(w, h, false)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			xx, yy := float64(x), float64(y)
			den := m[6]*xx + m[7]*yy + 1
			sx, sy := (m[0]*xx+m[1]*yy+m[2])/den, (m[3]*xx+m[4]*yy+m[5])/den
			for c := 0; c < 3; c++ {
				out.pixels[(y*w+x)*3+c] = r.sample(sx, sy, c, true)
			}
		}
	}
	if vertical && h > w {
		out = out.orient(8)
	}
	return out, nil
}

func median(values []float64) float64 {
	a := append([]float64(nil), values...)
	sort.Float64s(a)
	if len(a)%2 == 1 {
		return a[len(a)/2]
	}
	return (a[len(a)/2-1] + a[len(a)/2]) / 2
}
func readingOrder(quads []Quad, rtl bool) []int {
	if len(quads) == 0 {
		return []int{}
	}
	bounds := make([][4]float64, len(quads))
	all := make([]int, len(quads))
	for i, q := range quads {
		all[i] = i
		bounds[i] = [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
		for _, p := range q {
			bounds[i][0] = math.Min(bounds[i][0], p[0])
			bounds[i][1] = math.Min(bounds[i][1], p[1])
			bounds[i][2] = math.Max(bounds[i][2], p[0])
			bounds[i][3] = math.Max(bounds[i][3], p[1])
		}
	}
	var cut func([]int) []int
	cut = func(ids []int) []int {
		if len(ids) <= 1 {
			return ids
		}
		heights := make([]float64, len(ids))
		for j, i := range ids {
			heights[j] = bounds[i][3] - bounds[i][1]
		}
		height := median(heights)
		type candidate struct {
			ratio       float64
			axis, split int
			ids         []int
		}
		var candidates []candidate
		for axis := 0; axis < 2; axis++ {
			ordered := append([]int(nil), ids...)
			sort.SliceStable(ordered, func(i, j int) bool { return bounds[ordered[i]][axis] < bounds[ordered[j]][axis] })
			edge := bounds[ordered[0]][axis+2]
			required := math.Max(2, height*.2)
			if axis == 0 {
				required = math.Max(2, height*1.5)
			}
			for split := 1; split < len(ordered); split++ {
				i := ordered[split]
				gap := bounds[i][axis] - edge
				if gap > required {
					candidates = append(candidates, candidate{gap / required, axis, split, ordered})
				}
				edge = math.Max(edge, bounds[i][axis+2])
			}
		}
		if len(candidates) > 0 {
			hasColumn := false
			for _, c := range candidates {
				if c.axis == 0 {
					hasColumn = true
				}
			}
			best := candidate{ratio: -1}
			for _, c := range candidates {
				if hasColumn && c.axis != 0 {
					continue
				}
				if c.ratio > best.ratio {
					best = c
				}
			}
			first, second := best.ids[:best.split], best.ids[best.split:]
			if best.axis == 0 && rtl {
				first, second = second, first
			}
			return append(cut(first), cut(second)...)
		}
		ordered := append([]int(nil), ids...)
		sort.SliceStable(ordered, func(i, j int) bool {
			a, b := bounds[ordered[i]], bounds[ordered[j]]
			if a[1] != b[1] {
				return a[1] < b[1]
			}
			if rtl {
				return a[0] > b[0]
			}
			return a[0] < b[0]
		})
		return ordered
	}
	return cut(all)
}

func quadBounds(quad Quad) Box {
	minX, maxX, minY, maxY := quad[0][0], quad[0][0], quad[0][1], quad[0][1]
	for _, point := range quad[1:] {
		minX, maxX = math.Min(minX, point[0]), math.Max(maxX, point[0])
		minY, maxY = math.Min(minY, point[1]), math.Max(maxY, point[1])
	}
	return Box{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
}
