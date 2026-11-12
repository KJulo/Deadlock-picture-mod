package build

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/mesh"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/templates"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/vmdl"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/vtex"
)

// LOD diagnostic: every LOD maps the whole sphere into its own horizontal band of the texture
// (LOD n -> v in [n/4, (n+1)/4]), each band a different colour labelled "LOD n". In game this
// shows which LOD is being drawn at each distance, and a LOD whose UV format were wrong would show
// garbage instead of its label.

var lodColors = []color.NRGBA{
	{200, 60, 50, 255},  // LOD0 red
	{60, 160, 70, 255},  // LOD1 green
	{50, 100, 200, 255}, // LOD2 blue
	{220, 170, 40, 255}, // LOD3 yellow
}

// LODTestTexture draws the 4 labelled bands.
func LODTestTexture(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	bands := len(vmdl.LODs)
	bandH := size / bands
	for n := 0; n < bands; n++ {
		band := image.Rect(0, n*bandH, size, (n+1)*bandH)
		draw.Draw(img, band, &image.Uniform{lodColors[n]}, image.Point{}, draw.Src)
		// meridian lines every 30° and a parallel at the equator, to see distortion
		for k := 0; k < 12; k++ {
			x := k * size / 12
			draw.Draw(img, image.Rect(x, band.Min.Y, x+3, band.Max.Y), image.Black, image.Point{}, draw.Src)
		}
		mid := band.Min.Y + bandH/2
		draw.Draw(img, image.Rect(0, mid-1, size, mid+2), image.Black, image.Point{}, draw.Src)
		// the label, 6 times around the sphere; drawn small then scaled up (8x wide, 4x tall:
		// a band covers 360° across but only 180° in a quarter of the height)
		label := fmt.Sprintf("LOD %d", n)
		small := image.NewNRGBA(image.Rect(0, 0, 7*len(label)+2, 15))
		d := font.Drawer{Dst: small, Src: image.White, Face: basicfont.Face7x13, Dot: fixed.P(1, 12)}
		d.DrawString(label)
		sx, sy := 8, 4*bandH/512
		for k := 0; k < 6; k++ {
			ox := k*size/6 + size/12 - small.Bounds().Dx()*sx/2
			oy := band.Min.Y + bandH/4 - small.Bounds().Dy()*sy/2
			for y := 0; y < small.Bounds().Dy()*sy; y++ {
				for x := 0; x < small.Bounds().Dx()*sx; x++ {
					if small.NRGBAAt(x/sx, y/sy).A > 0 {
						img.SetNRGBA(ox+x, oy+y, color.NRGBA{255, 255, 255, 255})
					}
				}
			}
		}
	}
	return img
}

// LODTest builds the diagnostic mod.
func LODTest(t *Templates) (*Result, error) {
	bands := float64(len(vmdl.LODs))
	model, lods, err := vmdl.BuildSphereModelRemap(t.Model, vmdl.SphereRadius, func(lod int) mesh.UVRemap {
		return func(u, v float64) (float64, float64) { return u, (float64(lod) + v) / bands }
	})
	if err != nil {
		return nil, err
	}
	info, _, err := vtex.ReadInfo(t.Color)
	if err != nil {
		return nil, err
	}
	res, err := ModFromTexture(t, LODTestTexture(info.Width), nil, 0.69)
	if err != nil {
		return nil, err
	}
	res.Files[templates.Model.GamePath] = model
	res.LODs = lods
	return res, nil
}
