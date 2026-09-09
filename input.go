package oneocr

import (
	"fmt"
	"image"
	"os"
	"reflect"
)

// Input borrows its image or bytes until a synchronous operation returns.
// File and encoded inputs apply JPEG EXIF orientation. Pixels use sRGB.
type Input struct {
	kind   uint8
	path   string
	data   []byte
	image  image.Image
	pixels Pixels
}

type PixelFormat uint8

const (
	RGB PixelFormat = iota + 1
	RGBA
	BGRA
	RGBX
	BGRX
)

// Pixels describes rows with an explicit byte stride. Alpha is composited onto
// white; Premultiplied declares whether RGB channels already contain alpha.
type Pixels struct {
	Data                  []byte
	Width, Height, Stride int
	Format                PixelFormat
	Premultiplied         bool
}

func FromFile(path string) Input      { return Input{kind: 1, path: path} }
func FromEncoded(data []byte) Input   { return Input{kind: 2, data: data} }
func FromImage(img image.Image) Input { return Input{kind: 3, image: img} }
func FromPixels(pixels Pixels) Input  { return Input{kind: 4, pixels: pixels} }

func (in Input) raster() (raster, error) {
	switch in.kind {
	case 1:
		f, err := os.Open(in.path)
		if err != nil {
			return raster{}, err
		}
		defer f.Close()
		data, err := readLimited(f, 128*1024*1024)
		if err != nil {
			return raster{}, err
		}
		return decodeImage(data)
	case 2:
		return decodeImage(in.data)
	case 3:
		if in.image == nil || (reflect.ValueOf(in.image).Kind() == reflect.Pointer && reflect.ValueOf(in.image).IsNil()) {
			return raster{}, fmt.Errorf("oneocr: nil image")
		}
		return fromImage(in.image)
	case 4:
		return pixelRaster(in.pixels)
	default:
		return raster{}, fmt.Errorf("oneocr: empty input")
	}
}

func pixelRaster(p Pixels) (raster, error) {
	channels := 4
	if p.Format == RGB {
		if p.Premultiplied {
			return raster{}, fmt.Errorf("oneocr: RGB has no alpha")
		}
		return rgbRaster(p.Data, p.Width, p.Height, p.Stride)
	}
	if p.Format != RGBA && p.Format != BGRA && p.Format != RGBX && p.Format != BGRX {
		return raster{}, fmt.Errorf("oneocr: unsupported pixel format")
	}
	if p.Width < 2 || p.Height < 2 || p.Width > maxImagePixels || p.Height > maxImagePixels || int64(p.Width)*int64(p.Height) > maxImagePixels || p.Stride < p.Width*channels || p.Stride > len(p.Data) || int64(p.Height-1)*int64(p.Stride)+int64(p.Width)*int64(channels) > int64(len(p.Data)) {
		return raster{}, fmt.Errorf("oneocr: invalid pixel buffer dimensions")
	}
	r := newRaster(p.Width, p.Height, false)
	for y := 0; y < p.Height; y++ {
		for x := 0; x < p.Width; x++ {
			i, out := y*p.Stride+x*4, (y*p.Width+x)*3
			a := uint32(p.Data[i+3])
			if p.Format == RGBX || p.Format == BGRX {
				a = 255
			}
			for c := 0; c < 3; c++ {
				src := c
				if p.Format == BGRA || p.Format == BGRX {
					src = 2 - c
				}
				v := uint32(p.Data[i+src])
				if p.Premultiplied {
					if v > a {
						return raster{}, fmt.Errorf("oneocr: premultiplied channel exceeds alpha")
					}
					v += 255 - a
				} else {
					// Match image.NRGBA.RGBA and fromImage's 16-bit conversion.
					v = ((v*257*a)/255 + (255-a)*257) / 257
				}
				r.pixels[out+c] = uint8(v)
			}
		}
	}
	return r, nil
}
