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

func TestHeartSmoothSlopes(t *testing.T) {
	const h = 1e-4
	for j := 0; j <= 90; j++ {
		for i := 0; i < 180; i++ {
			th, ph := math.Pi*float64(j)/90, 2*math.Pi*float64(i)/180
			d := Vec3{math.Sin(th) * math.Cos(ph), math.Sin(th) * math.Sin(ph), math.Cos(th)}
			tangents := []Vec3{{math.Cos(th) * math.Cos(ph), math.Cos(th) * math.Sin(ph), -math.Sin(th)}, {-math.Sin(ph), math.Cos(ph), 0}}
			for _, tangent := range tangents {
				plus := d.Scale(math.Cos(h)).Add(tangent.Scale(math.Sin(h)))
				minus := d.Scale(math.Cos(h)).Sub(tangent.Scale(math.Sin(h)))
				left := (heartShape(d) - heartShape(minus)) / h
				right := (heartShape(plus) - heartShape(d)) / h
				if math.Abs(left-right) > 0.01 {
					t.Fatalf("slope jump at theta=%g phi=%g: %g vs %g", th, ph, left, right)
				}
			}
		}
	}
}

func TestHeartLobes(t *testing.T) {
	center := heartShape(Vec3{0, 0, 1})
	for _, sign := range []float64{-1, 1} {
		angle := 25 * math.Pi / 180
		d := Vec3{0, sign * math.Sin(angle), math.Cos(angle)}
		r := heartShape(d)
		if r <= center || r*d[2] <= center {
			t.Fatalf("lobe radius %g and height %g do not exceed top center %g", r, r*d[2], center)
		}
	}
}
