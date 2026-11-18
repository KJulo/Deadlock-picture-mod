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

func TestAdaptiveGrids(t *testing.T) {
	budgets := [][2]int{{6084, 17997}, {3196, 9000}, {1531, 3576}, {1444, 3264}}
	for _, budget := range budgets {
		g := shapedGrid(budget[0], budget[1]/3)
		if g.NU%8 != 0 || g.Vertices() > budget[0] || g.Triangles()*3 > budget[1] {
			t.Fatalf("budget %v: grid %v", budget, g)
		}
		t.Logf("budget %v: grid %v, vertices %d/%d", budget, g, g.Vertices(), budget[0])
		for _, name := range ShapeNames[1:] {
			shape, _ := ShapeByName(name)
			s := ShapedSphere(1, g, shape)
			row := g.NU + 1
			for i := 0; i < g.NU; i++ {
				u := s.UV[row+i][0]
				next := s.UV[row+i+1][0]
				if next <= u {
					t.Fatalf("%s grid %v nonmonotonic phi", name, g)
				}
				// Rotations and reflections are exact when every octant has equal counts.
				oct := g.NU / 8
				if math.Abs(s.UV[row+i+1][0]-u-(s.UV[row+(i+2*oct)%g.NU+1][0]-s.UV[row+(i+2*oct)%g.NU][0])) > 1e-10 {
					t.Fatalf("%s rotation at %d", name, i)
				}
				if math.Abs(u+s.UV[row+g.NU-i][0]-1) > 1e-10 {
					t.Fatalf("%s reflection at %d", name, i)
				}
			}
			for oct := 1; oct < 8; oct += 2 {
				if math.Abs(s.UV[row+oct*g.NU/8][0]-float64(oct)/8) > 1e-12 {
					t.Fatalf("%s missing edge", name)
				}
			}
			for j := 1; j < g.NV; j++ {
				if s.UV[j*row][1] <= s.UV[(j-1)*row][1] {
					t.Fatalf("%s nonmonotonic theta", name)
				}
				if name != "egg" && math.Abs(s.UV[j*row][1]+s.UV[(g.NV-j)*row][1]-1) > 1e-6 {
					t.Fatalf("%s latitude symmetry %.12g", name, s.UV[j*row][1]+s.UV[(g.NV-j)*row][1]-1)
				}
				for i := 0; i <= g.NU; i++ {
					k := j*row + i
					p := s.Pos[k].Normalize()
					u := math.Atan2(p[1], p[0]) / (2 * math.Pi)
					if u < 0 {
						u++
					}
					if i == g.NU {
						u = 1
					}
					v := math.Acos(math.Max(-1, math.Min(1, p[2]))) / math.Pi
					if math.Abs(s.UV[k][0]-u) > 1e-10 || math.Abs(s.UV[k][1]-v) > 1e-10 {
						t.Fatalf("%s direction UV at %d", name, k)
					}
				}
			}
			for i := 0; i < g.NU; i++ {
				center := (s.UV[row+i][0] + s.UV[row+i+1][0]) / 2
				if math.Abs(s.UV[s.Tris[g.NU*g.NV+i][0]][0]-center) > 1e-12 || math.Abs(s.UV[s.Tris[(g.NV-1)*g.NU+i][1]][0]-center) > 1e-12 {
					t.Fatalf("%s pole centre", name)
				}
			}
		}
	}
}
