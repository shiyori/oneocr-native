package engine

import (
	"context"
	"math"
	"os"
	"strconv"
	"testing"
)

func TestNumericConsensus(t *testing.T) {
	scored := func(text string, p float64) recognitionResult {
		return recognitionResult{text: text, logProbability: math.Log(p), tokens: 1}
	}
	cases := []struct {
		name string
		a, b recognitionResult
		want bool
	}{
		{"agree", scored("8", .95), scored("8", .60), true},
		{"ambiguous direction", scored("6", .99), scored("9", .99), false},
		{"weak primary", scored("8", .70), scored("8", .99), false},
		{"weak confirmation", scored("8", .99), scored("8", .30), false},
		{"non numeric", scored("*", .99), scored("*", .99), false},
		{"empty", recognitionResult{}, recognitionResult{}, false},
		{"too long", scored("1234", .99), scored("1234", .99), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := numericConsensus(c.a, c.b); got != c.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestReliablePageOrientation(t *testing.T) {
	q := Quad{{0, 0}, {10, 0}, {10, 12}, {0, 12}}
	for _, tc := range []struct {
		angle float64
		turns int
	}{{0, 0}, {math.Pi / 2, 1}, {math.Pi, 2}, {-math.Pi / 2, 3}} {
		r := []regionRecognition{{recognition: recognitionResult{text: "a reliable line"}, anchor: true, angle: tc.angle}}
		p := inferPageOrientation(r)
		if !p.valid || p.turns(q) != tc.turns {
			t.Fatal(p, tc)
		}
	}
	mixed := []regionRecognition{{recognition: recognitionResult{text: "one"}, anchor: true, angle: 0}, {recognition: recognitionResult{text: "two"}, anchor: true, angle: math.Pi}}
	if inferPageOrientation(mixed).valid {
		t.Fatal("conflicting page directions were trusted")
	}
	if inferPageOrientation(nil).valid {
		t.Fatal("invented a page direction")
	}
	if !compactRegion(q) || compactRegion(Quad{{0, 0}, {50, 0}, {50, 10}, {0, 10}}) {
		t.Fatal("compact region selection")
	}
}

func tableCell(q Quad, rotation int) (int, int) {
	var x, y float64
	for _, p := range q {
		x += p[0] / 4
		y += p[1] / 4
	}
	switch rotation {
	case 90:
		x, y = y, 719-x
	case 180:
		x, y = 1279-x, 719-y
	case 270:
		x, y = 1279-y, x
	}
	centers := []float64{392.63, 537.37, 687.90, 741.70, 795.42, 867.48}
	col := 0
	for i, c := range centers {
		if math.Abs(x-c) < math.Abs(x-centers[col]) {
			col = i
		}
	}
	return int(math.Round((y - 200.16) / 21.33)), col
}

func TestTableNumeralsNative(t *testing.T) {
	if testing.Short() || os.Getenv("ONEOCR_RUNTIME") == "" {
		t.Skip("requires native runtime")
	}
	useRepositoryFixtures(t)
	e, err := Open(Config{ModelPath: "models/" + DefaultModelName, Threads: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	ctx := context.Background()
	input, err := FromFile("testdata/paddleocr/720p-medal_table.png").raster()
	if err != nil {
		t.Fatal(err)
	}
	result, err := e.Recognize(ctx, FromImage(testRasterImage(input)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) != 96 {
		t.Fatalf("missing table cells: %d/96", len(result.Lines))
	}
	// Independently transcribed numeric cells, including the seven previously omitted digits.
	values := [][5]int{{1, 48, 22, 30, 100}, {2, 36, 39, 37, 112}, {3, 24, 13, 23, 60}, {4, 19, 13, 19, 51}, {5, 16, 11, 14, 41}, {6, 14, 15, 17, 46}, {7, 13, 11, 8, 32}, {8, 9, 8, 8, 25}, {9, 8, 9, 10, 27}, {10, 7, 16, 20, 43}, {11, 7, 5, 4, 16}, {12, 7, 4, 11, 22}, {13, 6, 4, 6, 16}, {14, 5, 11, 3, 19}, {15, 5, 4, 2, 11}}
	cells := map[[2]int]Line{}
	displayed := 0
	for _, l := range result.Lines {
		r, c := tableCell(l.Quad, 0)
		cells[[2]int{r, c}] = l
		if l.Confidence != nil && *l.Confidence >= .70 && l.DetectionScore >= .70 {
			displayed++
		}
	}
	for r, row := range values {
		for j, c := range []int{0, 2, 3, 4, 5} {
			l := cells[[2]int{r + 1, c}]
			if l.Text != strconv.Itoa(row[j]) {
				t.Errorf("row %d column %d: %q != %d", r+1, c, l.Text, row[j])
			}
		}
	}
	if displayed != 96 {
		t.Errorf("only %d cells pass the user-selected .70/.70 display thresholds", displayed)
	}
	// Page direction must disambiguate rank 9 from 6 in all right-angle rotations.
	for _, tc := range []struct{ degrees, orientation int }{{90, 6}, {180, 3}, {270, 8}} {
		t.Run(strconv.Itoa(tc.degrees), func(t *testing.T) {
			r, err := e.Recognize(ctx, FromImage(testRasterImage(input.orient(tc.orientation))), Options{})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, l := range r.Lines {
				row, col := tableCell(l.Quad, tc.degrees)
				if row == 9 && col == 0 {
					found = true
					if l.Text != "9" {
						t.Fatalf("rank 9 became %q", l.Text)
					}
					if l.RotationDegrees != (360-tc.degrees)%360 {
						t.Fatalf("incorrect correction metadata: %d", l.RotationDegrees)
					}
				}
			}
			if !found {
				t.Fatal("rank 9 was omitted")
			}
		})
	}
	// A single cropped numeral also needs the same guarded fallback.
	crop, err := rectify(input, cells[[2]int{8, 2}].Quad, false)
	if err != nil {
		t.Fatal(err)
	}
	line, err := e.RecognizeLine(ctx, FromImage(testRasterImage(crop)), Options{})
	if err != nil || line.Text != "9" || line.Rotated180 || line.RotationDegrees != 0 {
		t.Fatalf("cropped 9: %+v %v", line, err)
	}
	blank, err := e.RecognizeLine(ctx, FromImage(testRasterImage(newRaster(14, 14, true))), Options{})
	if err != nil || blank.Text != "" || blank.Confidence != nil {
		t.Fatalf("blank crop: %+v %v", blank, err)
	}
}
