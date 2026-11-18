package photo

import (
	"math"
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
