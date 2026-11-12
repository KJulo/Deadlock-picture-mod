package mesh

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestMedallionAtlasRoundTrip(t *testing.T) {
	for k := range MedallionAxes {
		for _, p := range [][2]float64{{0, 0}, {0.5, -0.3}, {-0.7, 0.7}, {0.99, 0}} {
			u, v := MedallionUV(k, p[0], p[1])
			if u < 0 || u > 1 || v < 0 || v > 1 {
				t.Fatalf("cap %d %v: uv (%g, %g) outside the texture", k, p, u, v)
			}
			k2, x, y, ok := MedallionCell(u, v)
			if !ok || k2 != k || math.Abs(x-p[0]) > 1e-9 || math.Abs(y-p[1]) > 1e-9 {
				t.Fatalf("cap %d %v: came back as cap %d (%g, %g) ok=%v", k, p, k2, x, y, ok)
			}
		}
		// the disc edge is capRad away from the centre, and centres are the axes
		if d := MedallionDir(k, 0, 0, 0.7); d != MedallionAxes[k] {
			t.Errorf("cap %d centre %v", k, d)
		}
		if a := math.Acos(MedallionDir(k, 0, 1, 0.7).Dot(MedallionAxes[k])); math.Abs(a-0.7) > 1e-9 {
			t.Errorf("cap %d edge at %g rad", k, a)
		}
	}
}

func TestMedallionGeometryFitsEveryLOD(t *testing.T) {
	budgets := [][2]int{{6084, 17997}, {3196, 9000}, {1531, 3576}, {1444, 3264}} // vmdl.LODs
	for _, b := range budgets {
		geom := MedallionGeometry(6.3, 43*math.Pi/180)
		buf, err := geom(b[0], b[1], UVSnorm, nil)
		if err != nil {
			t.Fatal(err)
		}
		if buf.Verts > b[0] || 3*buf.Tris > b[1] || len(buf.Vertices) != b[0]*VertexSize || len(buf.Indices) != b[1] {
			t.Fatalf("budget %v: %d verts, %d tris", b, buf.Verts, buf.Tris)
		}
		if buf.Tris < 12*8 {
			t.Fatalf("budget %v: only %d triangles", b, buf.Tris)
		}
		pos := func(i uint32) Vec3 {
			v := buf.Vertices[int(i)*VertexSize:]
			var p Vec3
			for c := 0; c < 3; c++ {
				p[c] = float64(math.Float32frombits(binary.LittleEndian.Uint32(v[4*c:])))
			}
			return p
		}
		// half the triangles face outward (fronts), half inward (backs), none degenerate
		out := 0
		for n := 0; n < buf.Tris; n++ {
			a, bb, c := pos(buf.Indices[3*n]), pos(buf.Indices[3*n+1]), pos(buf.Indices[3*n+2])
			cr := bb.Sub(a).Cross(c.Sub(a))
			if cr.Dot(cr) < 1e-10 {
				t.Fatalf("triangle %d degenerate", n)
			}
			if cr.Dot(a) > 0 {
				out++
			}
		}
		if out*2 != buf.Tris {
			t.Errorf("budget %v: %d of %d triangles face outward, want half", b, out, buf.Tris)
		}
	}
}
