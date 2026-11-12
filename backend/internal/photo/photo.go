// Package photo projects a user photo onto the sphere's equirectangular texture (port of
// reference/catmod/photo.py).
//
// Every texel (u, v) of the texture is a direction d on the sphere (+Z up, u = φ/2π, v = θ/π).
// In game the container lands in any orientation, so the default layout repeats the photo as round
// "stickers" and a complete photo is visible from every angle.
//
// Layouts:
//
//	cube6  6 stickers at the cube face centres (±X, ±Y, ±Z)   <- default, readable from anywhere
//	twice  2 stickers, front (+X) and back (-X)
//	wrap   one photo wrapped around the whole sphere (front +X, back per Params.Back)
//	single one sticker at the front (+X); sticker_deg up to 90 sets how much of the sphere it covers
package photo

import (
	"fmt"
	"image"
	"math"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/mesh"
)

// TexSize is the size of the game's colour texture.
const TexSize = 2048

// Params are the user's choices. Zero values are not meaningful: start from DefaultParams().
type Params struct {
	Variant     string     `json:"variant"`     // sphere (our sphere) | cage (the original model)
	Shape       string     `json:"shape"`       // sphere variant only: mesh.ShapeNames or "medallions"
	Layout      string     `json:"layout"`      // cube6 | twice | wrap | single (sphere only)
	Center      [2]float64 `json:"center"`      // photo point (0..1) placed at each sticker's centre
	Radius      float64    `json:"radius"`      // photo disc radius, fraction of the shorter side
	Projection  string     `json:"projection"`  // equidistant | ortho
	StickerDeg  float64    `json:"sticker_deg"` // angular radius of each sticker
	RingDeg     float64    `json:"ring_deg"`    // ring width around stickers (0 = none)
	RingColor   [3]uint8   `json:"ring_color"`
	BgColor     [3]uint8   `json:"bg_color"`
	Back        string     `json:"back"`        // wrap only: mirror | flip | color
	ColorAlpha  float64    `json:"color_alpha"` // colour texture alpha; 0 renders black in game
	Roughness   float64    `json:"roughness"`
	Contrast    float64    `json:"contrast"`
	Saturation  float64    `json:"saturation"`
	RotationDeg float64    `json:"rotation_deg"`
}

// DefaultParams matches PhotoParams() in the python reference.
func DefaultParams() Params {
	return Params{
		Variant: "sphere", Shape: "sphere", Layout: "cube6", Center: [2]float64{0.5, 0.5}, Radius: 0.5, Projection: "equidistant",
		StickerDeg: 40, RingDeg: 3, RingColor: [3]uint8{235, 235, 235}, BgColor: [3]uint8{24, 120, 140},
		Back: "mirror", ColorAlpha: 1, Roughness: 0.69, Contrast: 1.05, Saturation: 1.10,
	}
}

// Validate checks every field, so values coming from an HTTP request can be trusted afterwards.
func (p Params) Validate() error {
	in := func(v, lo, hi float64) bool { return v >= lo && v <= hi && !math.IsNaN(v) }
	switch {
	case p.Variant != "sphere" && p.Variant != "cage":
		return fmt.Errorf("%w: unknown variant %q", ErrBadParams, p.Variant)
	case !validShape(p.Shape):
		return fmt.Errorf("%w: unknown shape %q", ErrBadParams, p.Shape)
	case p.Layout != "cube6" && p.Layout != "twice" && p.Layout != "wrap" && p.Layout != "single":
		return fmt.Errorf("%w: unknown layout %q", ErrBadParams, p.Layout)
	case p.Projection != "equidistant" && p.Projection != "ortho":
		return fmt.Errorf("%w: unknown projection %q", ErrBadParams, p.Projection)
	case p.Back != "mirror" && p.Back != "flip" && p.Back != "color":
		return fmt.Errorf("%w: unknown back %q", ErrBadParams, p.Back)
	case !in(p.Center[0], 0, 1) || !in(p.Center[1], 0, 1):
		return fmt.Errorf("%w: center must be within 0..1", ErrBadParams)
	case !in(p.Radius, 0.05, 1.5):
		return fmt.Errorf("%w: radius must be within 0.05..1.5", ErrBadParams)
	case !in(p.StickerDeg, 5, 90):
		return fmt.Errorf("%w: sticker_deg must be within 5..90", ErrBadParams)
	case !in(p.RingDeg, 0, 30):
		return fmt.Errorf("%w: ring_deg must be within 0..30", ErrBadParams)
	case !in(p.ColorAlpha, 0, 1) || !in(p.Roughness, 0, 1):
		return fmt.Errorf("%w: color_alpha and roughness must be within 0..1", ErrBadParams)
	case !in(p.Contrast, 0, 3) || !in(p.Saturation, 0, 3):
		return fmt.Errorf("%w: contrast and saturation must be within 0..3", ErrBadParams)
	case !in(p.RotationDeg, -360, 360):
		return fmt.Errorf("%w: rotation_deg must be within -360..360", ErrBadParams)
	}
	return nil
}

type vec [3]float64

func (a vec) dot(b vec) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func cross(a, b vec) vec {
	return vec{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

type sticker struct{ centre, up vec }

// layouts: centre and "up" of each sticker. Up for the top/bottom stickers is arbitrary.
var layouts = map[string][]sticker{
	"cube6": {{vec{1, 0, 0}, vec{0, 0, 1}}, {vec{-1, 0, 0}, vec{0, 0, 1}}, {vec{0, 1, 0}, vec{0, 0, 1}},
		{vec{0, -1, 0}, vec{0, 0, 1}}, {vec{0, 0, 1}, vec{-1, 0, 0}}, {vec{0, 0, -1}, vec{1, 0, 0}}},
	"twice": {{vec{1, 0, 0}, vec{0, 0, 1}}, {vec{-1, 0, 0}, vec{0, 0, 1}}},
	// one seal whose size (sticker_deg, up to 90 = half the sphere) sets how much it covers
	"single": {{vec{1, 0, 0}, vec{0, 0, 1}}},
}

// discCoords maps direction d into a sticker's unit disc (sx right, sy up; radius 1 at maxAngle)
// and returns the angle between d and the sticker centre.
func discCoords(d vec, s sticker, maxAngle float64, projection string) (sx, sy, ang float64) {
	right := cross(s.up, s.centre) // seen from outside: right = up × centre
	x, y, z := d.dot(right), d.dot(s.up), d.dot(s.centre)
	lateral := math.Hypot(x, y)
	ang = math.Atan2(lateral, z)
	var rho float64
	if projection == "equidistant" {
		rho = ang / maxAngle
	} else {
		rho = math.Sin(math.Min(ang, math.Pi/2)) / math.Sin(maxAngle)
	}
	if lateral <= 1e-9 {
		return 0, 0, ang
	}
	k := rho / lateral
	return x * k, y * k, ang
}

// sampleBilinear reads the photo at fractional pixel coordinates (clamped to the edges).
func sampleBilinear(m *RGB, x, y float64) [3]float64 {
	x = math.Max(0, math.Min(float64(m.W)-1.001, x))
	y = math.Max(0, math.Min(float64(m.H)-1.001, y))
	x0, y0 := int(x), int(y)
	fx, fy := x-float64(x0), y-float64(y0)
	x1, y1 := min(x0+1, m.W-1), min(y0+1, m.H-1)
	a, b, c, d := m.at(x0, y0), m.at(x1, y0), m.at(x0, y1), m.at(x1, y1)
	var out [3]float64
	for k := 0; k < 3; k++ {
		top := float64(a[k])*(1-fx) + float64(b[k])*fx
		bottom := float64(c[k])*(1-fx) + float64(d[k])*fx
		out[k] = top*(1-fy) + bottom*fy
	}
	return out
}

// SphereTexture renders the size×size RGBA texture; alpha = p.ColorAlpha everywhere.
func SphereTexture(photo *RGB, p Params, size int) (*image.NRGBA, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	w, h := float64(photo.W), float64(photo.H)
	cx, cy, r := p.Center[0]*w, p.Center[1]*h, p.Radius*math.Min(w, h)
	alpha := uint8(math.RoundToEven(p.ColorAlpha * 255))
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	front := sticker{vec{1, 0, 0}, vec{0, 0, 1}}
	texel := stickerTexel(photo, p)

	for row := 0; row < size; row++ {
		th := (float64(row) + 0.5) / float64(size) * math.Pi
		for col := 0; col < size; col++ {
			ph := (float64(col) + 0.5) / float64(size) * 2 * math.Pi
			d := vec{math.Sin(th) * math.Cos(ph), math.Sin(th) * math.Sin(ph), math.Cos(th)}

			var px [3]float64
			if p.Layout == "wrap" {
				sx, sy, _ := discCoords(d, front, math.Pi/2, p.Projection)
				back := d[0] < 0
				if back && p.Back == "flip" {
					sx = -sx
				}
				if back && p.Back == "color" {
					px = [3]float64{float64(p.BgColor[0]), float64(p.BgColor[1]), float64(p.BgColor[2])}
				} else {
					px = sampleBilinear(photo, cx+sx*r, cy-sy*r)
				}
			} else {
				px = texel(d)
			}
			i := out.PixOffset(col, row)
			for k := 0; k < 3; k++ {
				out.Pix[i+k] = uint8(math.Max(0, math.Min(255, px[k]))) // truncates, like astype(uint8)
			}
			out.Pix[i+3] = alpha
		}
	}
	return out, nil
}

// stickerTexel returns the colour of direction d for the sticker layouts (cube6, twice): the
// photo inside the nearest sticker, then its ring, then the background.
func stickerTexel(photo *RGB, p Params) func(d vec) [3]float64 {
	w, h := float64(photo.W), float64(photo.H)
	cx, cy, r := p.Center[0]*w, p.Center[1]*h, p.Radius*math.Min(w, h)
	R, ring := p.StickerDeg*math.Pi/180, p.RingDeg*math.Pi/180
	stickers := layouts[p.Layout]
	return func(d vec) [3]float64 {
		// the nearest sticker centre owns this texel (first one wins a tie, like argmax)
		best, bestCos := 0, math.Inf(-1)
		for i, s := range stickers {
			if c := d.dot(s.centre); c > bestCos {
				best, bestCos = i, c
			}
		}
		sx, sy, ang := discCoords(d, stickers[best], R, p.Projection)
		switch {
		case ang <= R:
			return sampleBilinear(photo, cx+sx*r, cy-sy*r)
		case ang <= R+ring:
			return [3]float64{float64(p.RingColor[0]), float64(p.RingColor[1]), float64(p.RingColor[2])}
		default:
			return [3]float64{float64(p.BgColor[0]), float64(p.BgColor[1]), float64(p.BgColor[2])}
		}
	}
}

func validShape(name string) bool {
	_, ok := mesh.ShapeByName(name)
	return ok || name == mesh.MedallionsShape
}

// MedallionRad is the angular radius of each medallion: the seal plus its ring, capped so
// neighbouring medallions never overlap.
func (p Params) MedallionRad() float64 {
	return math.Min((p.StickerDeg+p.RingDeg)*math.Pi/180, mesh.MaxMedallionRad)
}

// Texture renders the colour texture the chosen shape expects: the equirectangular sphere
// texture, or the medallion atlas.
func Texture(photo *RGB, p Params, size int) (*image.NRGBA, error) {
	if p.Shape == mesh.MedallionsShape {
		return MedallionTexture(photo, p, size)
	}
	return SphereTexture(photo, p, size)
}

// MedallionTexture renders the six seals into the atlas discs of mesh.MedallionUV. Each texel
// is coloured exactly like the sphere texture colours that direction (always with the cube6
// layout), so seals look the same as on the sphere. Texels just outside a disc repeat its edge
// colour, so filtering and mipmaps do not bleed the background into the ring.
func MedallionTexture(photo *RGB, p Params, size int) (*image.NRGBA, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	p.Layout = "cube6"
	texel := stickerTexel(photo, p)
	capRad := p.MedallionRad()
	alpha := uint8(math.RoundToEven(p.ColorAlpha * 255))
	bg := [3]float64{float64(p.BgColor[0]), float64(p.BgColor[1]), float64(p.BgColor[2])}
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	for row := 0; row < size; row++ {
		v := (float64(row) + 0.5) / float64(size)
		for col := 0; col < size; col++ {
			u := (float64(col) + 0.5) / float64(size)
			px := bg
			if k, x, y, ok := mesh.MedallionCell(u, v); ok {
				if rho := math.Hypot(x, y); rho <= 1.1 {
					if rho > 1 {
						x, y = x/rho, y/rho
					}
					d := mesh.MedallionDir(k, x, y, capRad)
					px = texel(vec(d))
				}
			}
			i := out.PixOffset(col, row)
			for c := 0; c < 3; c++ {
				out.Pix[i+c] = uint8(math.Max(0, math.Min(255, px[c])))
			}
			out.Pix[i+3] = alpha
		}
	}
	return out, nil
}
