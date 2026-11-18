package mesh

import (
	"math"
	"testing"
)

func TestCubeAtlasMapping(t *testing.T) {
	a := CubeAtlas{}
	for face := 0; face < 6; face++ {
		for j := 0; j <= 10; j++ {
			for i := 0; i <= 10; i++ {
				x, y := float64(i)/10, float64(j)/10
				u, v := a.VertexUV(face, x, y)
				d, tangent, _ := a.TexelDir(u, v)
				n, r := atlasNormals[face], atlasRight[face]
				want := n.Add(r.Scale(math.Tan((x - .5) * math.Pi / 2))).Add(n.Cross(r).Scale(math.Tan((y - .5) * math.Pi / 2))).Normalize()
				if d.Sub(want).Dot(d.Sub(want)) > 1e-20 {
					t.Fatalf("face %d (%g,%g): %v != %v", face, x, y, d, want)
				}
				if math.Abs(tangent.Dot(d)) > 1e-12 || d.Cross(tangent).Dot(n.Cross(r)) <= 0 {
					t.Fatalf("face %d: mirrored tangent frame", face)
				}
			}
		}
	}
}

func TestCubeSphereWinding(t *testing.T) {
	s := CubeSphere(1, 8, nil)
	for _, tri := range s.Tris {
		a, b, c := s.Pos[tri[0]], s.Pos[tri[1]], s.Pos[tri[2]]
		if b.Sub(a).Cross(c.Sub(a)).Dot(a) <= 0 {
			t.Fatal("inward triangle")
		}
	}
}

func TestCubeAtlasPaddingClampsToNearestIsland(t *testing.T) {
	a := CubeAtlas{}
	for face := 0; face < 6; face++ {
		u, v := a.VertexUV(face, 0, .5)
		edge, _, _ := a.TexelDir(u, v)
		bleed, _, ok := a.TexelDir(u-0.001, v)
		if ok || edge.Sub(bleed).Dot(edge.Sub(bleed)) > 1e-20 {
			t.Fatalf("face %d: padding leaks", face)
		}
	}
}

func TestCubeSphereVerticesMatchAtlas(t *testing.T) {
	a := CubeAtlas{}
	s := CubeSphere(1, 12, nil)
	for i, p := range s.Pos {
		uv := s.UV[i]
		d, tangent, _ := a.TexelDir(uv[0], uv[1])
		if p.Sub(d).Dot(p.Sub(d)) > 1e-20 || s.Tangent[i].Dot(tangent) < 1-1e-12 {
			t.Fatalf("vertex %d does not match atlas", i)
		}
	}
}
