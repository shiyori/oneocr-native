package engine

import "testing"

func TestRTL(t *testing.T) {
	cases := map[string]string{"ملاعلاب ابحرم": "مرحبا بالعالم", "םלוע םולש": "שלום עולם", "USD 123.45 رعسلا": "السعر 123.45 USD", "USD 123.45 ריחמ": "מחיר 123.45 USD", "Hello World رعسلا": "السعر Hello World"}
	for visual, expected := range cases {
		if got := visualToLogical(visual); got != expected {
			t.Errorf("%q != %q", got, expected)
		}
	}
}
func TestCTCExplicitBlank(t *testing.T) {
	a := alphabet{characters: []string{"<space>", "!", "a", "<blank>"}, blank: 3, composites: map[string]string{}}
	ids := []int{2, 2, 3, 2, 0, 1, 3}
	data := make([]float32, len(ids)*4)
	for i := range data {
		data[i] = -12
	}
	for i, id := range ids {
		data[i*4+id] = 0
	}
	got, e := a.decode(floatTensor{[]int64{int64(len(ids)), 1, 4}, data}, "Latin")
	if e != nil || got != "aa !" {
		t.Fatal(got, e)
	}
}
func TestOrientation(t *testing.T) {
	r := newRaster(2, 3, false)
	for i := 0; i < 6; i++ {
		r.pixels[i*3] = uint8(i + 1)
	}
	cw := r.orient(6)
	ccw := r.orient(8)
	if cw.width != 3 || cw.height != 2 || cw.pixels[0] != 5 || cw.pixels[6] != 1 {
		t.Fatal(cw)
	}
	if ccw.pixels[0] != 2 || ccw.pixels[6] != 6 {
		t.Fatal(ccw)
	}
	back := cw.orient(8)
	for i := range r.pixels {
		if r.pixels[i] != back.pixels[i] {
			t.Fatal("rotation did not roundtrip")
		}
	}
}
