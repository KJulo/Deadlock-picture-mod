package photo

import (
	"image"
	"math"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/mesh"
)

// ReliefTextures renders the normal/roughness and ambient occlusion maps using the same
// directions and nearest-seal ownership as the colour texture. XY use the Source 2 tangent
// frame, with positive bitangent n cross t; the game shader's sign requires in-game checking.
func ReliefTextures(p Params, size int) (*image.NRGBA, *image.NRGBA, error) {
	if err := p.Validate(); err != nil {
		return nil, nil, err
	}
	normal := image.NewNRGBA(image.Rect(0, 0, size, size))
	ao := image.NewNRGBA(normal.Rect)
	if p.Shape == mesh.MedallionsShape {
		p.Layout = "cube6"
	}
	stickers := sealStickers(p)
	rough := byte(math.RoundToEven(p.Roughness * 255))
	inner := p.StickerDeg * math.Pi / 180
	width := p.RingDeg * math.Pi / 180
	shape, _ := mesh.ShapeByName(p.Shape)
	useAtlas := p.Shape != mesh.MedallionsShape && mesh.AtlasPowerByName(p.Shape) != 0
	medallionRad := p.MedallionRad()
	parallelRows(size, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < size; x++ {
				i := (y*size + x) * 4
				normal.Pix[i], normal.Pix[i+1], normal.Pix[i+2], normal.Pix[i+3] = 128, 128, rough, 255
				ao.Pix[i], ao.Pix[i+1], ao.Pix[i+2], ao.Pix[i+3] = 255, 255, 255, 255
				if p.Relief == 0 || width == 0 || len(stickers) == 0 {
					continue
				}
				u, v := (float64(x)+0.5)/float64(size), (float64(y)+0.5)/float64(size)
				var d, t, n mesh.Vec3
				if p.Shape == mesh.MedallionsShape {
					k, a, b, ok := mesh.MedallionCell(u, v)
					if !ok || math.Hypot(a, b) > 1 {
						continue
					}
					d = mesh.MedallionDir(k, a, b, medallionRad)
					n = d
					e := mesh.MedallionDir(k, 0.00001, 0, medallionRad).Sub(mesh.MedallionAxes[k].Scale(math.Cos(0.00001 * medallionRad))).Normalize()
					t = e.Sub(n.Scale(e.Dot(n))).Normalize()
				} else {
					if useAtlas {
						var ok bool
						d, t, ok = (mesh.CubeAtlas{}).TexelDir(u, v)
						if !ok {
							continue
						}
					} else {
						d, t, _ = (mesh.Equirect{}).TexelDir(u, v)
					}
					n = d
					if useAtlas && shape != nil {
						n, t = mesh.SurfaceFrame(shape, d, t)
					}
					if shape != nil && !useAtlas {
						th, ph := v*math.Pi, u*2*math.Pi
						at := func(a, b float64) mesh.Vec3 {
							q, _, _ := (mesh.Equirect{}).TexelDir(b/(2*math.Pi), a/math.Pi)
							return q.Scale(shape(q))
						}
						const h = 1e-4
						tc := math.Min(math.Max(th, 2*h), math.Pi-2*h)
						du := at(tc, ph+h).Sub(at(tc, ph-h))
						dv := at(tc+h, ph).Sub(at(tc-h, ph))
						n = dv.Cross(du).Normalize()
						t = t.Sub(n.Scale(t.Dot(n))).Normalize()
					}
				}
				c := mesh.Vec3(stickers[nearestSticker(vec(d), stickers)].centre)
				best := d.Dot(c)
				angle := math.Acos(math.Max(-1, math.Min(1, best)))
				q := (angle - inner) / width
				if q <= 0 || q >= 1 {
					continue
				}
				// The ring is a smooth raised arch. Its angular derivative changes sign at the crest.
				grad := c.Sub(d.Scale(best)).Scale(-1 / math.Sin(angle))
				slope := p.Relief * 0.22 * math.Pi * math.Sin(2*math.Pi*q)
				// The frame is orthonormalized exactly as in the mesh; +Y is n cross t.
				b := n.Cross(t)
				nx, ny := -slope*grad.Dot(t), -slope*grad.Dot(b)
				inv := 1 / math.Sqrt(1+nx*nx+ny*ny)
				normal.Pix[i] = byte(math.Round((nx*inv*0.5 + 0.5) * 255))
				normal.Pix[i+1] = byte(math.Round((ny*inv*0.5 + 0.5) * 255))
				shadow := p.Relief * 0.18 * math.Pow(math.Abs(math.Cos(math.Pi*q)), 8)
				value := byte(math.Round(255 * (1 - shadow)))
				ao.Pix[i], ao.Pix[i+1], ao.Pix[i+2] = value, value, value
			}
		}
	})
	return normal, ao, nil
}
