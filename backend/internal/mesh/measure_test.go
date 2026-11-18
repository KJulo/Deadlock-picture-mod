package mesh

import (
	"math"
	"testing"
)

func TestMeasureAdaptive(t *testing.T) {
	for _, b := range [][2]int{{6084, 17997}, {1444, 3264}} {
		old := SphereGrid(b[0], b[1]/3)
		g := shapedGrid(b[0], b[1]/3)
		cube, _ := ShapeByName("cube")
		s := ShapedSphere(1, g, cube)
		r := g.NU + 1
		edge := g.NU / 8
		cubeGap := math.Max(s.UV[r+edge][0]-s.UV[r+edge-1][0], s.UV[r+edge+1][0]-s.UV[r+edge][0]) * 360
		coin, _ := ShapeByName("coin")
		c := ShapedSphere(1, g, coin)
		target := math.Atan(1/0.46) / math.Pi
		gap := 0.0
		for j := 0; j < g.NV; j++ {
			a, z := c.UV[j*r][1], c.UV[(j+1)*r][1]
			if a <= target && target <= z {
				gap = (z - a) * 180
			}
		}
		t.Logf("grid old=%v new=%v cube edge adjacent max %.3f -> %.3f deg coin rim containing gap %.3f -> %.3f deg", old, g, 360/float64(old.NU), cubeGap, 180/float64(old.NV), gap)
	}
}
