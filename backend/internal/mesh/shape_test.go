package mesh

import (
	"math"
	"testing"
)

func TestShapesStayInBounds(t *testing.T) {
	for _, name := range ShapeNames {
		shape, ok := ShapeByName(name)
		if !ok {
			t.Fatalf("%s: unknown", name)
		}
		// The original cage fills a ±6.34-unit box (±1.006 sphere radii): stay inside it so the
		// game neither culls nor sinks the shape, and keep corners close to the rolling sphere.
		radial, axis := MaxExtent(shape)
		if axis > 1.006 || radial > 1.2 || axis < 0.4 {
			t.Errorf("%s: extent radial %.3f, axis %.3f", name, radial, axis)
		}
		t.Logf("%-6s radial %.3f axis %.3f", name, radial, axis)
	}
	if _, ok := ShapeByName("teapot"); ok {
		t.Error("unknown shape accepted")
	}
}

func TestShapeNormalsAreOutwardUnitVectors(t *testing.T) {
	g := Grid{60, 30}
	for _, name := range ShapeNames[1:] {
		shape, _ := ShapeByName(name)
		s := ShapedSphere(6.3, g, shape)
		for k := range s.Pos {
			n, tg := s.Normal[k], s.Tangent[k]
			if math.Abs(n.Dot(n)-1) > 1e-9 || math.Abs(tg.Dot(tg)-1) > 1e-9 {
				t.Fatalf("%s: vertex %d not unit (n·n=%g t·t=%g)", name, k, n.Dot(n), tg.Dot(tg))
			}
			if math.Abs(n.Dot(tg)) > 1e-9 {
				t.Fatalf("%s: vertex %d tangent not perpendicular", name, k)
			}
			// star-shaped solids: the normal never points back towards the centre
			if n.Dot(s.Pos[k].Normalize()) < 0.1 {
				t.Fatalf("%s: vertex %d normal points inward", name, k)
			}
		}
		// every real triangle still faces outward (same winding as the sphere)
		for _, tri := range s.Tris {
			a, b, c := s.Pos[tri[0]], s.Pos[tri[1]], s.Pos[tri[2]]
			cr := b.Sub(a).Cross(c.Sub(a))
			if cr.Dot(cr) < 1e-12 {
				continue // pole rows are degenerate on purpose
			}
			if cr.Dot(a.Add(b).Add(c)) <= 0 {
				t.Fatalf("%s: triangle %v faces inward", name, tri)
			}
		}
	}
}

func TestSphereShapeIsTheSphere(t *testing.T) {
	g := Grid{20, 10}
	a, b := UVSphere(6.3, g), ShapedSphere(6.3, g, nil)
	for k := range a.Pos {
		if a.Pos[k] != b.Pos[k] || a.Normal[k] != b.Normal[k] {
			t.Fatal("nil shape changed the sphere")
		}
	}
}
