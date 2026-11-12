package photo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif" // registers decoders with image.Decode
	_ "image/jpeg"
	_ "image/png"
	"math"

	_ "golang.org/x/image/webp"
)

// RGB is an 8-bit RGB image, 3 bytes per pixel, rows top to bottom.
type RGB struct {
	W, H int
	Pix  []uint8
}

func (m *RGB) at(x, y int) []uint8 { i := (y*m.W + x) * 3; return m.Pix[i : i+3] }

// MaxPixels limits decoded photo size (a 12 MP phone photo is ~12e6).
const MaxPixels = 50_000_000

// Load decodes a photo (JPEG, PNG, GIF or WebP), applies its EXIF orientation, the rotation and
// the contrast/saturation tweaks of p — the same steps as photo.load_photo in the python reference.
func Load(data []byte, p Params) (*RGB, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("photo: unsupported or corrupt image: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > MaxPixels {
		return nil, fmt.Errorf("photo: %dx%d is too large", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("photo: decode %s: %w", format, err)
	}
	m := toRGB(img)
	if format == "jpeg" {
		m = applyOrientation(m, jpegOrientation(data))
	}
	if p.RotationDeg != 0 {
		m = rotate(m, p.RotationDeg)
	}
	m = contrast(m, p.Contrast)
	return saturation(m, p.Saturation), nil
}

func toRGB(img image.Image) *RGB {
	b := img.Bounds()
	nrgba := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(nrgba, nrgba.Bounds(), img, b.Min, draw.Src)
	m := &RGB{b.Dx(), b.Dy(), make([]uint8, b.Dx()*b.Dy()*3)}
	for i := 0; i < b.Dx()*b.Dy(); i++ {
		copy(m.Pix[i*3:i*3+3], nrgba.Pix[i*4:i*4+3]) // transparency is dropped, like convert("RGB")
	}
	return m
}

// ---------------------------------------------------------------------------------------------
// EXIF orientation. Phones store photos as the sensor saw them plus a tag saying how to turn
// them; without this, portrait photos come out sideways.
// ---------------------------------------------------------------------------------------------

// jpegOrientation returns the EXIF orientation (1..8) of a JPEG, or 1 if absent/unreadable.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	pos := 2
	for pos+4 <= len(data) {
		if data[pos] != 0xFF {
			return 1
		}
		marker := data[pos+1]
		if marker == 0xDA || marker == 0xD9 { // start of scan / end: no more metadata
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[pos+2:]))
		if size < 2 || pos+2+size > len(data) {
			return 1
		}
		seg := data[pos+4 : pos+2+size]
		if marker == 0xE1 && len(seg) > 6 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		pos += 2 + size
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	ifd := int(bo.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[ifd:]))
	for i := 0; i < n; i++ {
		e := ifd + 2 + i*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 { // Orientation, type SHORT
			if v := int(bo.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// applyOrientation turns the image upright (same mapping as PIL's ImageOps.exif_transpose).
func applyOrientation(m *RGB, o int) *RGB {
	if o <= 1 || o > 8 {
		return m
	}
	w, h := m.W, m.H
	ow, oh := w, h
	if o >= 5 { // 5..8 swap width and height
		ow, oh = h, w
	}
	out := &RGB{ow, oh, make([]uint8, len(m.Pix))}
	for y := 0; y < oh; y++ {
		for x := 0; x < ow; x++ {
			var sx, sy int
			switch o {
			case 2: // mirror horizontally
				sx, sy = w-1-x, y
			case 3: // rotate 180
				sx, sy = w-1-x, h-1-y
			case 4: // mirror vertically
				sx, sy = x, h-1-y
			case 5: // transpose
				sx, sy = y, x
			case 6: // rotate 90° clockwise
				sx, sy = y, h-1-x
			case 7: // transverse
				sx, sy = w-1-y, h-1-x
			case 8: // rotate 90° counter-clockwise
				sx, sy = w-1-y, x
			}
			copy(out.at(x, y), m.at(sx, sy))
		}
	}
	return out
}

// rotate turns the image clockwise by deg, growing the canvas to fit (black corners), like
// PIL's img.rotate(-deg, BICUBIC, expand=True) but with bilinear sampling.
func rotate(m *RGB, deg float64) *RGB {
	rad := deg * math.Pi / 180
	c, s := math.Cos(rad), math.Sin(rad)
	ow := int(math.Ceil(math.Abs(float64(m.W)*c) + math.Abs(float64(m.H)*s) - 1e-9))
	oh := int(math.Ceil(math.Abs(float64(m.W)*s) + math.Abs(float64(m.H)*c) - 1e-9))
	out := &RGB{ow, oh, make([]uint8, ow*oh*3)}
	cx, cy := float64(m.W)/2, float64(m.H)/2
	ocx, ocy := float64(ow)/2, float64(oh)/2
	for y := 0; y < oh; y++ {
		for x := 0; x < ow; x++ {
			dx, dy := float64(x)+0.5-ocx, float64(y)+0.5-ocy
			// inverse rotation: where in the source does this output pixel come from?
			sx, sy := c*dx+s*dy+cx-0.5, -s*dx+c*dy+cy-0.5
			if sx < -0.5 || sy < -0.5 || sx > float64(m.W)-0.5 || sy > float64(m.H)-0.5 {
				continue
			}
			px := sampleBilinear(m, sx, sy)
			for k := 0; k < 3; k++ {
				out.Pix[(y*ow+x)*3+k] = uint8(math.Round(px[k]))
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------------------------
// Contrast / saturation, matching Pillow's ImageEnhance (blend against a "degenerate" image).
// ---------------------------------------------------------------------------------------------

// luma is Pillow's RGB -> L conversion (ITU-R 601, fixed point).
func luma(r, g, b uint8) uint8 {
	return uint8((uint32(r)*19595 + uint32(g)*38470 + uint32(b)*7471 + 0x8000) >> 16)
}

// blend is Pillow's ImagingBlend: in1 + alpha*(in2-in1) in float32, truncated (clamped when
// alpha is outside 0..1).
func blend(in1, in2 uint8, alpha float32) uint8 {
	t := float32(int(in1)) + alpha*float32(int(in2)-int(in1))
	if alpha >= 0 && alpha <= 1 {
		return uint8(int(t))
	}
	switch {
	case t <= 0:
		return 0
	case t >= 255:
		return 255
	}
	return uint8(t)
}

func contrast(m *RGB, factor float64) *RGB {
	if factor == 1 {
		return m
	}
	var sum float64
	for i := 0; i < len(m.Pix); i += 3 {
		sum += float64(luma(m.Pix[i], m.Pix[i+1], m.Pix[i+2]))
	}
	mean := uint8(int(sum/float64(m.W*m.H) + 0.5))
	out := &RGB{m.W, m.H, make([]uint8, len(m.Pix))}
	for i, v := range m.Pix {
		out.Pix[i] = blend(mean, v, float32(factor))
	}
	return out
}

func saturation(m *RGB, factor float64) *RGB {
	if factor == 1 {
		return m
	}
	out := &RGB{m.W, m.H, make([]uint8, len(m.Pix))}
	for i := 0; i < len(m.Pix); i += 3 {
		l := luma(m.Pix[i], m.Pix[i+1], m.Pix[i+2])
		for k := 0; k < 3; k++ {
			out.Pix[i+k] = blend(l, m.Pix[i+k], float32(factor))
		}
	}
	return out
}
