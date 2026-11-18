package photo

import (
	"math"
	"testing"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/mesh"
)

func TestAtlasPaddingAndWrap(t *testing.T) {
	p := DefaultParams()
	p.Layout = "wrap"
	p.Relief = .6
	img := &RGB{W: 2, H: 2, Pix: []byte{10, 20, 30, 80, 90, 100, 120, 130, 140, 220, 230, 240}}
	const size = 256
	tex, err := SphereTexturePhotos([]*RGB{img}, p, size)
	if err != nil {
		t.Fatal(err)
	}
	normal, ao, err := ReliefTextures(p, size)
	if err != nil {
		t.Fatal(err)
	}
	front := sticker{vec{1, 0, 0}, vec{0, 0, 1}}
	at := photoPoint(img, p)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			u, v := (float64(x)+.5)/size, (float64(y)+.5)/size
			d, _, inside := (mesh.CubeAtlas{}).TexelDir(u, v)
			sx, sy, _ := discCoords(vec(d), front, math.Pi/2, p.Projection)
			px, py := at(sx, sy)
			want := sampleBilinear(img, px, py)
			got := tex.NRGBAAt(x, y)
			if got.R != uint8(want[0]) || got.G != uint8(want[1]) || got.B != uint8(want[2]) {
				t.Fatalf("wrap mismatch at %d,%d", x, y)
			}
			if !inside {
				n, a := normal.NRGBAAt(x, y), ao.NRGBAAt(x, y)
				if n.R != 128 || n.G != 128 || a.R != 255 {
					t.Fatalf("non-flat padding at %d,%d", x, y)
				}
			}
		}
	}
}
