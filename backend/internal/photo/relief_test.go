package photo

import (
	"bytes"
	"math"
	"runtime"
	"sync"
	"testing"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/mesh"
)

func TestReliefBevel(t *testing.T) {
	p := DefaultParams()
	p.Layout = "single"
	p.Relief = 1
	p.StickerDeg = 30
	p.RingDeg = 10
	n, ao, err := ReliefTextures(p, 720)
	if err != nil {
		t.Fatal(err)
	}
	at := func(deg float64) (byte, byte, byte) {
		u, v := (mesh.CubeAtlas{}).VertexUV(0, .5+deg/90, .5)
		c := n.NRGBAAt(int(u*720), int(v*720))
		a := ao.NRGBAAt(int(u*720), int(v*720))
		return c.R, c.G, a.R
	}
	r, g, a := at(10)
	if r != 128 || g != 128 || a != 255 {
		t.Fatalf("outside bevel %d %d %d", r, g, a)
	}
	inner, _, _ := at(32)
	outer, _, _ := at(37)
	if inner >= 128 || outer <= 128 {
		t.Fatalf("bevel sides %d %d", inner, outer)
	}
	_, _, a = at(30.5)
	if a >= 255 {
		t.Fatal("missing groove shadow")
	}
	p.Relief = math.NaN()
	if p.Validate() == nil {
		t.Fatal("NaN relief accepted")
	}
}

func TestCubeReliefUsesAtlasFrame(t *testing.T) {
	p := DefaultParams()
	p.Shape = "cube"
	p.Layout = "single"
	p.Relief = 1
	p.StickerDeg = 30
	p.RingDeg = 10
	if mesh.AtlasPowerByName(p.Shape) == 0 {
		t.Fatal("cube did not select atlas")
	}
	n, _, err := ReliefTextures(p, 720)
	if err != nil {
		t.Fatal(err)
	}
	u, v := (mesh.CubeAtlas{}).VertexUV(0, .5+32.0/90, .5)
	c := n.NRGBAAt(int(u*720), int(v*720))
	if c.R >= 128 {
		t.Fatalf("incorrect relief tangent: %+v", c)
	}
}

func TestParallelReliefDeterminism(t *testing.T) {
	multiSeals := make([]Seal, 6)
	for i, s := range layouts["cube6"] {
		multiSeals[i] = Seal{
			Photo:  0,
			Dir:    s.centre,
			Center: [2]float64{0.5, 0.5},
			Radius: 0.5,
		}
	}

	cases := []struct {
		name string
		mod  func(*Params)
	}{
		{"sphere_relief", func(p *Params) { p.Relief = 0.8 }},
		{"cube_relief", func(p *Params) { p.Shape, p.Relief = "cube", 0.9 }},
		{"egg_relief", func(p *Params) { p.Shape, p.Relief = "egg", 0.7 }},
		{"medallions_relief", func(p *Params) { p.Shape, p.Relief, p.Seals = mesh.MedallionsShape, 1, multiSeals }},
		{"zero_relief", func(p *Params) { p.Relief = 0 }},
	}

	prev := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(prev)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := DefaultParams()
			p.RingDeg = 8
			tc.mod(&p)

			runtime.GOMAXPROCS(1)
			wantN, wantAO, err := ReliefTextures(p, 128)
			if err != nil {
				t.Fatal(err)
			}

			runtime.GOMAXPROCS(4)
			var wg sync.WaitGroup
			for g := 0; g < 4; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					gotN, gotAO, err := ReliefTextures(p, 128)
					if err != nil {
						t.Errorf("ReliefTextures failed: %v", err)
						return
					}
					if !bytes.Equal(gotN.Pix, wantN.Pix) {
						t.Errorf("normal map differed from GOMAXPROCS(1)")
					}
					if !bytes.Equal(gotAO.Pix, wantAO.Pix) {
						t.Errorf("AO map differed from GOMAXPROCS(1)")
					}
				}()
			}
			wg.Wait()
		})
	}
}
