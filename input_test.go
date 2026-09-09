package oneocr

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestInputFormatsAndAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(3, 4, 8, 7))
	for y := 4; y < 7; y++ {
		for x := 3; x < 8; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 29), uint8(y * 31), 27, uint8((x + y) * 19)})
		}
	}
	expected, err := FromImage(img).raster()
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	inputs := []Input{FromEncoded(encoded.Bytes()), FromFile(path)}
	for _, format := range []PixelFormat{RGBA, BGRA} {
		data := make([]byte, 3*24)
		for y := 0; y < 3; y++ {
			for x := 0; x < 5; x++ {
				c := img.NRGBAAt(x+3, y+4)
				i := y*24 + x*4
				data[i], data[i+1], data[i+2], data[i+3] = c.R, c.G, c.B, c.A
				if format == BGRA {
					data[i], data[i+2] = data[i+2], data[i]
				}
			}
		}
		inputs = append(inputs, FromPixels(Pixels{Data: data[:68], Width: 5, Height: 3, Stride: 24, Format: format}))
	}
	for _, input := range inputs {
		actual, err := input.raster()
		if err != nil {
			t.Fatal(err)
		}
		if actual.width != expected.width || actual.height != expected.height || !bytes.Equal(actual.pixels, expected.pixels) {
			t.Fatal("input changes normalized pixels")
		}
	}
	// Premultiplied pixels match Go's RGBA white composition exactly.
	premul := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			premul.SetRGBA(x, y, color.RGBA{30, 50, 80, 128})
		}
	}
	expected, _ = FromImage(premul).raster()
	actual, err := FromPixels(Pixels{Data: premul.Pix, Width: 2, Height: 2, Stride: premul.Stride, Format: RGBA, Premultiplied: true}).raster()
	if err != nil || !bytes.Equal(actual.pixels, expected.pixels) {
		t.Fatalf("premultiplied mismatch: %v", err)
	}
}
func TestInvalidInputs(t *testing.T) {
	for _, input := range []Input{{}, FromImage(nil), FromImage((*image.RGBA)(nil)), FromEncoded(nil), FromFile(filepath.Join(t.TempDir(), "missing")), FromPixels(Pixels{Data: make([]byte, 16), Width: 2, Height: 2, Stride: 8, Format: 99}), FromPixels(Pixels{Data: make([]byte, 15), Width: 2, Height: 2, Stride: 8, Format: RGBA}), FromPixels(Pixels{Data: make([]byte, 16), Width: 2, Height: 2, Stride: 7, Format: RGBA}), FromPixels(Pixels{Data: make([]byte, 16), Width: 40000000, Height: 2, Stride: 8, Format: RGBA})} {
		if _, err := input.raster(); err == nil {
			t.Fatal("accepted invalid input")
		}
	}
}
