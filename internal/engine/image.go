package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
)

type raster struct {
	width, height int
	pixels        []uint8
}

func newRaster(w, h int, white bool) raster {
	r := raster{w, h, make([]uint8, w*h*3)}
	if white {
		for i := range r.pixels {
			r.pixels[i] = 255
		}
	}
	return r
}
func fromImage(img image.Image) (raster, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 2 || h < 2 || w > maxImagePixels || h > maxImagePixels || int64(w)*int64(h) > maxImagePixels {
		return raster{}, fmt.Errorf("oneocr: image must be 2x2 to 40 megapixels")
	}
	r := newRaster(w, h, false)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			red, green, blue, alpha := img.At(x+b.Min.X, y+b.Min.Y).RGBA()
			p := (y*w + x) * 3
			r.pixels[p] = uint8((red + 65535 - alpha) / 257)
			r.pixels[p+1] = uint8((green + 65535 - alpha) / 257)
			r.pixels[p+2] = uint8((blue + 65535 - alpha) / 257)
		}
	}
	return r, nil
}
func decodeImage(data []byte) (raster, error) {
	if len(data) > 128*1024*1024 {
		return raster{}, fmt.Errorf("oneocr: encoded image exceeds 128 MiB")
	}
	config, _, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil {
		return raster{}, e
	}
	if config.Width < 2 || config.Height < 2 || config.Width > maxImagePixels || config.Height > maxImagePixels || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return raster{}, fmt.Errorf("oneocr: image dimensions exceed limits")
	}
	img, _, e := image.Decode(bytes.NewReader(data))
	if e != nil {
		return raster{}, e
	}
	r, e := fromImage(img)
	if e != nil {
		return r, e
	}
	return r.orient(jpegOrientation(data)), nil
}
func (r raster) orient(orientation int) raster {
	if orientation < 2 || orientation > 8 {
		return r
	}
	w, h := r.width, r.height
	if orientation >= 5 {
		w, h = h, w
	}
	out := newRaster(w, h, false)
	for y := 0; y < r.height; y++ {
		for x := 0; x < r.width; x++ {
			dx, dy := x, y
			switch orientation {
			case 2:
				dx = r.width - 1 - x
			case 3:
				dx, dy = r.width-1-x, r.height-1-y
			case 4:
				dy = r.height - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = r.height-1-y, x
			case 7:
				dx, dy = r.height-1-y, r.width-1-x
			case 8:
				dx, dy = y, r.width-1-x
			}
			copy(out.pixels[(dy*w+dx)*3:], r.pixels[(y*r.width+x)*3:(y*r.width+x)*3+3])
		}
	}
	return out
}
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 255 || data[1] != 216 {
		return 1
	}
	pos := 2
	for pos+4 <= len(data) {
		if data[pos] != 255 {
			return 1
		}
		marker := data[pos+1]
		pos += 2
		if marker == 0xda || marker == 0xd9 {
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		if size < 2 || size > len(data)-pos {
			return 1
		}
		segment := data[pos+2 : pos+size]
		pos += size
		if marker != 0xe1 || len(segment) < 14 || string(segment[:6]) != "Exif\x00\x00" {
			continue
		}
		t := segment[6:]
		var order binary.ByteOrder
		if string(t[:2]) == "II" {
			order = binary.LittleEndian
		} else if string(t[:2]) == "MM" {
			order = binary.BigEndian
		} else {
			return 1
		}
		if order.Uint16(t[2:4]) != 42 {
			return 1
		}
		offset := uint64(order.Uint32(t[4:8]))
		if offset+2 > uint64(len(t)) {
			return 1
		}
		count := int(order.Uint16(t[offset : offset+2]))
		offset += 2
		for i := 0; i < count; i++ {
			if offset+12 > uint64(len(t)) {
				return 1
			}
			entry := t[offset : offset+12]
			offset += 12
			if order.Uint16(entry[:2]) == 0x112 && order.Uint16(entry[2:4]) == 3 && order.Uint32(entry[4:8]) == 1 {
				return int(order.Uint16(entry[8:10]))
			}
		}
		return 1
	}
	return 1
}
func clampInt(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
func clamp(v, low, high float64) float64 {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
func round(v float64) int { return int(math.RoundToEven(v)) }

func (r raster) sample(x, y float64, channel int, quantized bool) uint8 {
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	if quantized {
		fx = math.Floor(fx*32+.5) / 32
		fy = math.Floor(fy*32+.5) / 32
	}
	get := func(x, y int) float64 {
		return float64(r.pixels[(clampInt(y, 0, r.height-1)*r.width+clampInt(x, 0, r.width-1))*3+channel])
	}
	value := (1-fy)*((1-fx)*get(x0, y0)+fx*get(x0+1, y0)) + fy*((1-fx)*get(x0, y0+1)+fx*get(x0+1, y0+1))
	return uint8(clamp(math.Floor(value+.5), 0, 255))
}
func (r raster) resize(w, h int) raster {
	if w == r.width && h == r.height {
		return r
	}
	out := newRaster(w, h, false)
	for y := 0; y < h; y++ {
		sy := (float64(y)+.5)*float64(r.height)/float64(h) - .5
		for x := 0; x < w; x++ {
			sx := (float64(x)+.5)*float64(r.width)/float64(w) - .5
			for c := 0; c < 3; c++ {
				out.pixels[(y*w+x)*3+c] = r.sample(sx, sy, c, false)
			}
		}
	}
	return out
}

// Area filtering matches the reference downscale contract. Upscale/line
// normalization use bilinear interpolation instead.
func (r raster) downscale(w, h int) raster {
	if w == r.width && h == r.height {
		return r
	}
	out := newRaster(w, h, false)
	sx, sy := float64(r.width)/float64(w), float64(r.height)/float64(h)
	for y := 0; y < h; y++ {
		top, bottom := float64(y)*sy, float64(y+1)*sy
		for x := 0; x < w; x++ {
			left, right := float64(x)*sx, float64(x+1)*sx
			var sum [3]float64
			for iy := int(math.Floor(top)); iy < int(math.Ceil(bottom)); iy++ {
				wy := math.Min(bottom, float64(iy+1)) - math.Max(top, float64(iy))
				for ix := int(math.Floor(left)); ix < int(math.Ceil(right)); ix++ {
					wx := math.Min(right, float64(ix+1)) - math.Max(left, float64(ix))
					p := (clampInt(iy, 0, r.height-1)*r.width + clampInt(ix, 0, r.width-1)) * 3
					for c := 0; c < 3; c++ {
						sum[c] += float64(r.pixels[p+c]) * wx * wy
					}
				}
			}
			for c := 0; c < 3; c++ {
				out.pixels[(y*w+x)*3+c] = uint8(clamp(math.Floor(sum[c]/(sx*sy)+.5), 0, 255))
			}
		}
	}
	return out
}
func (r raster) nchw(divisor float32) floatTensor {
	n := r.width * r.height
	out := make([]float32, 3*n)
	for i := 0; i < n; i++ {
		for c := 0; c < 3; c++ {
			out[c*n+i] = float32(r.pixels[i*3+c]) / divisor
		}
	}
	return floatTensor{[]int64{1, 3, int64(r.height), int64(r.width)}, out}
}
func normalizeLine(r raster, stride int) (floatTensor, error) {
	w := max(8, round(float64(r.width)*60/float64(r.height)))
	if w+32 > 8192 {
		return floatTensor{}, fmt.Errorf("oneocr: line exceeds 8192 normalized pixels")
	}
	resized := r.resize(w, 60)
	total := w + 32
	if m := total % stride; m != 0 {
		total += stride - m
	}
	padded := newRaster(total, 60, true)
	for y := 0; y < 60; y++ {
		copy(padded.pixels[(y*total+16)*3:], resized.pixels[y*w*3:(y+1)*w*3])
	}
	return padded.nchw(255), nil
}
