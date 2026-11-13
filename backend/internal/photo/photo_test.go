package photo

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/mesh"
)

const testdata = "../../../testdata"

func readPNG(t *testing.T, name string) image.Image {
	t.Helper()
	f, err := os.Open(filepath.Join(testdata, name))
	if err != nil {
		t.Skipf("no golden %s: run `make golden`", name)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// Every layout must match the python texture. The math is the same but floats round slightly
// differently, so a texel may be off by a level or two, and a few texels exactly on a sticker
// edge may fall on the other side.
func TestMatchesPythonReference(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(testdata, "photo_cases.json"))
	if err != nil {
		t.Skip("no golden file: run `make golden`")
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	var size int
	json.Unmarshal(all["_size"], &size)
	delete(all, "_size")

	input, err := os.ReadFile(filepath.Join(testdata, "photo_input.png"))
	if err != nil {
		t.Fatal(err)
	}
	for name, rawParams := range all {
		t.Run(name, func(t *testing.T) {
			p := DefaultParams()
			if err := json.Unmarshal(rawParams, &p); err != nil {
				t.Fatal(err)
			}
			m, err := Load(input, p)
			if err != nil {
				t.Fatal(err)
			}
			got, err := SphereTexture(m, p, size)
			if err != nil {
				t.Fatal(err)
			}
			want := readPNG(t, "photo_"+name+".png").(*image.NRGBA)

			var sumDiff float64
			far := 0 // texels off by more than 2 levels in some channel
			for i := 0; i < len(got.Pix); i += 4 {
				worst := 0
				for k := 0; k < 4; k++ {
					d := int(got.Pix[i+k]) - int(want.Pix[i+k])
					if d < 0 {
						d = -d
					}
					sumDiff += float64(d)
					worst = max(worst, d)
				}
				if worst > 2 {
					far++
				}
			}
			texels := size * size
			mean := sumDiff / float64(len(got.Pix))
			t.Logf("mean |diff| %.4f, %d of %d texels off by > 2", mean, far, texels)
			if mean > 0.2 || far > texels/1000 {
				t.Errorf("too different from python: mean %.3f, %d texels off", mean, far)
			}
		})
	}
}

// EXIF orientations must turn images exactly like PIL's ImageOps.exif_transpose.
func TestOrientationMatchesPIL(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(testdata, "exif_orientations.json"))
	if err != nil {
		t.Skip("no golden file: run `make golden`")
	}
	type img struct {
		W, H int
		Pix  []uint8
	}
	var g struct {
		Input        img
		Orientations map[string]img
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	for o := 1; o <= 8; o++ {
		want := g.Orientations[string(rune('0'+o))]
		got := applyOrientation(&RGB{g.Input.W, g.Input.H, g.Input.Pix}, o)
		if got.W != want.W || got.H != want.H || !bytes.Equal(got.Pix, want.Pix) {
			t.Errorf("orientation %d: got %dx%d %v, want %dx%d %v", o, got.W, got.H, got.Pix[:6], want.W, want.H, want.Pix[:6])
		}
	}
}

// jpegWithOrientation builds a JPEG with an EXIF APP1 segment carrying the orientation tag.
func jpegWithOrientation(t *testing.T, w, h, orientation int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08\x00\x01")
	entry := make([]byte, 12)
	binary.BigEndian.PutUint16(entry[0:], 0x0112)
	binary.BigEndian.PutUint16(entry[2:], 3) // SHORT
	binary.BigEndian.PutUint32(entry[4:], 1)
	binary.BigEndian.PutUint16(entry[8:], uint16(orientation))
	tiff = append(append(tiff, entry...), 0, 0, 0, 0)
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	j := buf.Bytes()
	return append(append(append([]byte{}, j[:2]...), append(seg, payload...)...), j[2:]...)
}

func TestLoadAppliesEXIF(t *testing.T) {
	data := jpegWithOrientation(t, 40, 20, 6) // portrait photo stored sideways
	if o := jpegOrientation(data); o != 6 {
		t.Fatalf("orientation %d, want 6", o)
	}
	m, err := Load(data, DefaultParams())
	if err != nil {
		t.Fatal(err)
	}
	if m.W != 20 || m.H != 40 {
		t.Errorf("got %dx%d, want 20x40", m.W, m.H)
	}
}

func TestLoadRejectsGarbage(t *testing.T) {
	if _, err := Load([]byte("definitely not an image"), DefaultParams()); err == nil {
		t.Error("expected an error")
	}
	if o := jpegOrientation([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF}); o != 1 {
		t.Errorf("truncated EXIF: orientation %d, want 1", o)
	}
}

func TestValidate(t *testing.T) {
	if err := DefaultParams().Validate(); err != nil {
		t.Fatal(err)
	}
	bad := DefaultParams()
	bad.Layout = "pyramid"
	if bad.Validate() == nil {
		t.Error("unknown layout accepted")
	}
	bad = DefaultParams()
	bad.Radius = 99
	if bad.Validate() == nil {
		t.Error("radius 99 accepted")
	}
}

// Big photos are shrunk to MaxSide; JPEG decoding through the fast path matches image/draw.
func TestLargePhotoIsShrunk(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, MaxSide*2+10, 100))
	for i := range img.Pix {
		img.Pix[i] = 200
		if i%4 == 3 {
			img.Pix[i] = 255 // opaque (image.RGBA is premultiplied)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	p := DefaultParams()
	p.Contrast, p.Saturation = 1, 1
	m, err := Load(buf.Bytes(), p)
	if err != nil {
		t.Fatal(err)
	}
	if m.W > MaxSide || m.H != 100/3 {
		t.Errorf("got %dx%d", m.W, m.H)
	}
	if m.Pix[0] != 200 {
		t.Errorf("pixel %d, want 200", m.Pix[0])
	}
}

func TestJPEGFastPathMatchesDraw(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for i := range src.Pix {
		src.Pix[i] = byte(i * 37)
	}
	for i := 3; i < len(src.Pix); i += 4 {
		src.Pix[i] = 255
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, src, &jpeg.Options{Quality: 90})
	dec, _ := jpeg.Decode(bytes.NewReader(buf.Bytes()))
	ref := image.NewNRGBA(dec.Bounds())
	draw.Draw(ref, ref.Bounds(), dec, image.Point{}, draw.Src)
	got := toRGB(dec)
	for i := 0; i < 64*48; i++ {
		for c := 0; c < 3; c++ {
			if got.Pix[i*3+c] != ref.Pix[i*4+c] {
				t.Fatalf("pixel %d channel %d: %d vs draw %d", i, c, got.Pix[i*3+c], ref.Pix[i*4+c])
			}
		}
	}
}

// The photo lands inside each dome, right side up (photo top towards the dome's "up", -X), and
// texels outside the domes keep the original texture.
func TestPaintDiscs(t *testing.T) {
	const size = 512
	tex := image.NewNRGBA(image.Rect(0, 0, size, size))
	for i := range tex.Pix {
		tex.Pix[i] = 77
	}
	// photo: top half red, bottom half blue
	ph := &RGB{W: 100, H: 100, Pix: make([]uint8, 100*100*3)}
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			c := []uint8{0, 0, 255}
			if y < 50 {
				c = []uint8{255, 0, 0}
			}
			copy(ph.Pix[(y*100+x)*3:], c)
		}
	}
	p := DefaultParams()
	p.RingDeg = 0
	PaintDiscs(tex, ph, p, CageDiscs)
	at := func(u, v float64) []uint8 {
		i := tex.PixOffset(int(u*size), int(v*size))
		return tex.Pix[i : i+4]
	}
	for _, d := range CageDiscs {
		// a point halfway towards "up" (-X) must be red, halfway towards "down" (+X) blue
		up := [2]float64{d.Center[0] - 0.5*d.R*d.A[0][0], d.Center[1] - 0.5*d.R*d.A[0][1]}
		down := [2]float64{d.Center[0] + 0.5*d.R*d.A[0][0], d.Center[1] + 0.5*d.R*d.A[0][1]}
		if c := at(up[0], up[1]); c[0] != 255 || c[2] != 0 {
			t.Errorf("%s: up side is %v, want red", d.Name, c)
		}
		if c := at(down[0], down[1]); c[2] != 255 || c[0] != 0 {
			t.Errorf("%s: down side is %v, want blue", d.Name, c)
		}
		if c := at(d.Center[0], d.Center[1]); c[3] != 77 {
			t.Errorf("%s: alpha changed to %d", d.Name, c[3])
		}
	}
	if c := at(0.9, 0.1); c[0] != 77 || c[1] != 77 {
		t.Errorf("texel outside the domes changed: %v", c)
	}
}

func TestMedallionTexture(t *testing.T) {
	img := &RGB{W: 4, H: 4, Pix: make([]uint8, 4*4*3)}
	for i := range img.Pix {
		img.Pix[i] = 200 // photo: light grey
	}
	p := DefaultParams()
	p.Shape, p.BgColor, p.RingDeg = mesh.MedallionsShape, [3]uint8{1, 2, 3}, 0
	tex, err := Texture(img, p, 256)
	if err != nil {
		t.Fatal(err)
	}
	// every disc centre shows the photo; the unused bottom third is background
	for k := range mesh.MedallionAxes {
		u, v := mesh.MedallionUV(k, 0, 0)
		if c := tex.NRGBAAt(int(u*256), int(v*256)); c.R != 200 {
			t.Errorf("cap %d centre: %v", k, c)
		}
	}
	if c := tex.NRGBAAt(128, 250); c.R != 1 || c.G != 2 || c.B != 3 {
		t.Errorf("unused area: %v", c)
	}
}

func TestSingleLayoutSizeFollowsStickerDeg(t *testing.T) {
	img := &RGB{W: 4, H: 4, Pix: make([]uint8, 4*4*3)}
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	// equirect texel at θ = 90° (equator) and longitude φ from the front (+X)
	at := func(tex *image.NRGBA, phDeg float64) uint8 {
		size := tex.Bounds().Dx()
		col := int(math.Mod(phDeg+360, 360) / 360 * float64(size))
		return tex.NRGBAAt(col, size/2).R
	}
	for _, deg := range []float64{30, 60, 90} {
		p := DefaultParams()
		p.Layout, p.StickerDeg, p.RingDeg, p.BgColor = "single", deg, 0, [3]uint8{1, 1, 1}
		tex, err := SphereTexture(img, p, 360)
		if err != nil {
			t.Fatal(err)
		}
		if at(tex, deg-3) != 200 || at(tex, deg+3) != 1 || at(tex, 180) != 1 {
			t.Errorf("sticker_deg %v: inside %d, outside %d, back %d", deg, at(tex, deg-3), at(tex, deg+3), at(tex, 180))
		}
	}
}

// A clockwise quarter turn brings the top of the photo disc to the seal's right edge.
func TestTurnRotatesThePhotoClockwise(t *testing.T) {
	p := DefaultParams()
	p.TurnDeg = 90
	m := &RGB{W: 100, H: 100}
	x, y := photoPoint(m, p)(1, 0) // seal's right edge
	if math.Abs(x-50) > 1e-9 || math.Abs(y-0) > 1e-9 {
		t.Fatalf("right edge samples (%g, %g), want the top of the disc (50, 0)", x, y)
	}
	p.TurnDeg = 400
	if p.Validate() == nil {
		t.Fatal("turn_deg 400 accepted")
	}
}
