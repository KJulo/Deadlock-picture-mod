package mesh

import (
	"math"
	"testing"
)

func TestExtraShapes(t *testing.T) {
	for _, name := range []string{"d20", "capsule", "heart", "brilliant"} {
		t.Run(name, func(t *testing.T) {
			s, ok := ShapeByName(name)
			if !ok {
				t.Fatal("shape missing")
			}
			radial, axis := MaxExtent(s)
			if axis > 1.006 || radial > 1.2 {
				t.Fatalf("extent %g %g", radial, axis)
			}
			for j := 0; j <= 180; j++ {
				for i := 0; i < 360; i++ {
					th, ph := math.Pi*float64(j)/180, 2*math.Pi*float64(i)/360
					d := Vec3{math.Sin(th) * math.Cos(ph), math.Sin(th) * math.Sin(ph), math.Cos(th)}
					next := Vec3{math.Sin(th) * math.Cos(ph+0.01), math.Sin(th) * math.Sin(ph+0.01), math.Cos(th)}
					r := s(d)
					if math.IsNaN(r) || math.IsInf(r, 0) || r <= 0 || math.Abs(r-s(next)) > 0.05 {
						t.Fatalf("invalid or discontinuous radius %g", r)
					}
					if math.Abs(r-s(Vec3{d[0], -d[1], d[2]})) > 1e-9 {
						t.Fatal("Y reflection")
					}
					if (name == "d20" || name == "capsule") && math.Abs(r-s(Vec3{d[0], d[1], -d[2]})) > 1e-9 {
						t.Fatal("Z reflection")
					}
				}
			}
		})
	}
}
