package mesh

import (
	"math"
	"testing"
)

func TestMeasuredSamplingChoice(t *testing.T) {
	for _, tc := range []struct {
		name  string
		power float64
	}{{"sphere", 1}, {"cube", .5}, {"gem", 0}, {"coin", 0}, {"egg", 0}} {
		shape, ok := ShapeByName(tc.name)
		if !ok {
			t.Fatalf("missing shape %s", tc.name)
		}
		eq, atlas, warped := CompareSamplingQuality(shape)
		if got := AtlasPower(shape); got != tc.power {
			t.Errorf("%s: power %g, want %g", tc.name, got, tc.power)
		}
		choice := eq
		if tc.power == 1 {
			choice = atlas
		} else if tc.power == .5 {
			choice = warped
		}
		if tc.power != 0 && choice[3].Max > eq[3].Max {
			t.Errorf("%s: choice is worse at LOD3", tc.name)
		}
		for i := 0; i < 4; i++ {
			if math.IsNaN(choice[i].Max) || math.IsNaN(choice[i].NormalMax) {
				t.Fatalf("%s LOD%d: non-finite error", tc.name, i)
			}
			if tc.power != 0 && choice[i].Max > 1.5*eq[i].Max {
				t.Errorf("%s LOD%d: atlas is much worse", tc.name, i)
			}
		}
	}
}

func TestWarpedCubeVerticesMatchAtlas(t *testing.T) {
	shape, _ := ShapeByName("cube")
	s := CubeSphereWarped(1, 9, shape, .5)
	a := CubeAtlas{}
	for i, p := range s.Pos {
		d, tangent, _ := a.TexelDir(s.UV[i][0], s.UV[i][1])
		want := d.Scale(shape(d))
		if p.Sub(want).Dot(p.Sub(want)) > 1e-20 {
			t.Fatalf("vertex %d mismatch", i)
		}
		normal, tangent := SurfaceFrame(shape, d, tangent)
		if s.Normal[i].Dot(normal) < .99999 || s.Tangent[i].Dot(tangent) < .99999 {
			t.Fatalf("vertex %d: relief frame disagrees", i)
		}
	}
}

func TestArbitraryShapeSelection(t *testing.T) {
	shape := func(d Vec3) float64 { return .8 + .05*d[0]*d[1] }
	eq, atlas, warped := CompareSamplingQuality(shape)
	power := AtlasPower(shape)
	got := eq
	if power == 1 {
		got = atlas
	} else if power == .5 {
		got = warped
	} else if power != 0 {
		t.Fatalf("unknown exponent %g", power)
	}
	if got[3].Max > eq[3].Max {
		t.Fatal("selection increased LOD3 error")
	}
}
