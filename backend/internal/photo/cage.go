package photo

import (
	"image"
	"math"
)

// Disc is a round, roughly flat patch of the original cage model's texture where a photo can go.
// UV = Center + x·A[0] + y·A[1] (A[0] = d(u,v)/dx, A[1] = d(u,v)/dy) maps a point (x, y) of the patch (game units, seen from outside)
// to the texture; R is the patch radius in game units.
type Disc struct {
	Name   string
	Center [2]float64
	A      [2][2]float64
	R      float64
}

// CageDiscs: the lens domes of soul_container.vmdl_c (pinned by sha256). Each dome's texture patch
// is shared by the top (+Z) and bottom (-Z) faces, so the photo shows on both (mirrored on one).
// Found by analysing the model's UV islands (docs/lessons/10-variantes.md): LOD0-1 sample these
// patches, measured with a least-squares fit of UV against position.
var CageDiscs = []Disc{
	{Name: "outer dome", Center: [2]float64{0.1930, 0.4151},
		A: [2][2]float64{{-0.02302, 0.03239}, {0.03238, 0.02301}}, R: 2.845},
	{Name: "inner dome", Center: [2]float64{0.6113, 0.8963},
		A: [2][2]float64{{0.02141, 0.03195}, {-0.03196, 0.02141}}, R: 2.689},
}

// PaintDiscs draws the photo into each disc of an existing texture (the original cage texture),
// keeping its alpha. Same framing as the stickers: Center/Radius pick the part of the photo, with
// an optional ring of RingDeg/StickerDeg of the radius.
func PaintDiscs(dst *image.NRGBA, photo *RGB, p Params, discs []Disc) {
	size := dst.Bounds().Dx()
	w, h := float64(photo.W), float64(photo.H)
	cx, cy, r := p.Center[0]*w, p.Center[1]*h, p.Radius*math.Min(w, h)
	ring := p.RingDeg / p.StickerDeg
	for _, d := range discs {
		// inverse of the 2x2 map: texture offset -> (x, y)
		a, b, c, e := d.A[0][0], d.A[1][0], d.A[0][1], d.A[1][1] // u = a x + b y, v = c x + e y
		det := a*e - b*c
		extent := 1.05 * d.R * math.Hypot(a, c) * float64(size) * 1.5
		x0, x1 := int(d.Center[0]*float64(size)-extent), int(d.Center[0]*float64(size)+extent)
		y0, y1 := int(d.Center[1]*float64(size)-extent), int(d.Center[1]*float64(size)+extent)
		for ty := max(y0, 0); ty < min(y1, size); ty++ {
			for tx := max(x0, 0); tx < min(x1, size); tx++ {
				du := (float64(tx)+0.5)/float64(size) - d.Center[0]
				dv := (float64(ty)+0.5)/float64(size) - d.Center[1]
				x := (e*du - b*dv) / det
				y := (-c*du + a*dv) / det
				rho := math.Hypot(x, y) / d.R
				if rho > 1.04 { // a little past the edge so no original texel bleeds in
					continue
				}
				// same orientation as the sphere's top sticker: up = -X, right = +Y
				sx, sy := y/d.R, -x/d.R
				var px [3]float64
				if rho > 1-ring {
					px = [3]float64{float64(p.RingColor[0]), float64(p.RingColor[1]), float64(p.RingColor[2])}
				} else {
					s := 1 / (1 - ring) // the photo fills the disc inside the ring
					px = sampleBilinear(photo, cx+sx*s*r, cy-sy*s*r)
				}
				i := dst.PixOffset(tx, ty)
				for k := 0; k < 3; k++ {
					dst.Pix[i+k] = uint8(math.Max(0, math.Min(255, px[k])))
				}
			}
		}
	}
}
