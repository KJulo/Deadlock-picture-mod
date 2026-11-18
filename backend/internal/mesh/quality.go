package mesh

import (
	"math"
	"sync"
)

// SurfaceQuality measures radial distance as a fraction of the sphere radius and normal error in degrees.
type SurfaceQuality struct{ Max, RMS, NormalMax float64 }

// SurfaceFrame evaluates a radial surface's normal and tangent along an increasing direction axis.
func SurfaceFrame(shape Shape, d, axis Vec3) (n, t Vec3) {
	t = axis.Sub(d.Scale(axis.Dot(d))).Normalize()
	if shape == nil {
		return d, t
	}
	b := d.Cross(t)
	const h = 1e-5
	at := func(v Vec3) Vec3 { v = v.Normalize(); return v.Scale(shape(v)) }
	du := at(d.Add(t.Scale(h))).Sub(at(d.Sub(t.Scale(h))))
	dv := at(d.Add(b.Scale(h))).Sub(at(d.Sub(b.Scale(h))))
	n = du.Cross(dv).Normalize()
	t = du.Sub(n.Scale(du.Dot(n))).Normalize()
	return
}

// MeasureSurface samples the three edge midpoints and centroid of each nondegenerate triangle.
func MeasureSurface(s Sphere, shape Shape) SurfaceQuality {
	samples := [][3]float64{{.5, .5, 0}, {0, .5, .5}, {.5, 0, .5}, {1.0 / 3, 1.0 / 3, 1.0 / 3}}
	var q SurfaceQuality
	var sum float64
	count := 0
	for _, tri := range s.Tris {
		p0, p1, p2 := s.Pos[tri[0]], s.Pos[tri[1]], s.Pos[tri[2]]
		area := p1.Sub(p0).Cross(p2.Sub(p0))
		if area.Dot(area) < 1e-24 {
			continue
		}
		for _, w := range samples {
			p := p0.Scale(w[0]).Add(p1.Scale(w[1])).Add(p2.Scale(w[2]))
			length := math.Sqrt(p.Dot(p))
			if length < 1e-12 {
				continue
			}
			d := p.Scale(1 / length)
			expected := 1.0
			if shape != nil {
				expected = shape(d)
			}
			delta := math.Abs(length - expected)
			q.Max = math.Max(q.Max, delta)
			sum += delta * delta
			count++
			interp := s.Normal[tri[0]].Scale(w[0]).Add(s.Normal[tri[1]].Scale(w[1])).Add(s.Normal[tri[2]].Scale(w[2])).Normalize()
			axis, _ := FrisvadBasis(d)
			normal, _ := SurfaceFrame(shape, d, axis)
			angle := math.Acos(math.Max(-1, math.Min(1, normal.Dot(interp)))) * 180 / math.Pi
			q.NormalMax = math.Max(q.NormalMax, angle)
		}
	}
	if count > 0 {
		q.RMS = math.Sqrt(sum / float64(count))
	}
	return q
}

// QualityBudgets are the fixed model vertex and index budgets, in LOD order.
var QualityBudgets = [...][2]int{{6084, 17997}, {3196, 9000}, {1531, 3576}, {1444, 3264}}

// CompareSamplingQuality evaluates all three samplings at the fixed LOD budgets.
func CompareSamplingQuality(shape Shape) (eq, atlas, warped [4]SurfaceQuality) {
	for i, b := range QualityBudgets {
		g := SphereGrid(b[0], b[1]/3)
		if shape != nil {
			g = shapedGrid(b[0], b[1]/3)
		}
		eq[i] = MeasureSurface(ShapedSphere(1, g, shape), shape)
		n := 0
		for k := 1; 6*(k+1)*(k+1) <= b[0] && 36*k*k <= b[1]; k++ {
			n = k
		}
		atlas[i] = MeasureSurface(CubeSphere(1, n, shape), shape)
		warped[i] = MeasureSurface(CubeSphereWarped(1, n, shape, .5), shape)
	}
	return
}

// AtlasPower selects an atlas sampling exponent using surface error at every LOD.
// Zero selects equirectangular sampling; one is the standard equiangular atlas,
// and 0.5 concentrates samples towards face edges. A candidate must not exceed
// 150% of equirectangular maximum error at any higher-detail LOD.
func AtlasPower(shape Shape) float64 {
	eq, atlas, warped := CompareSamplingQuality(shape)
	power, best := 0.0, eq[3].Max
	for _, candidate := range []struct {
		power   float64
		quality [4]SurfaceQuality
	}{{1, atlas}, {.5, warped}} {
		if candidate.quality[3].Max >= best {
			continue
		}
		acceptable := true
		for i := 0; i < 3; i++ {
			if candidate.quality[i].Max > 1.5*eq[i].Max {
				acceptable = false
				break
			}
		}
		if acceptable {
			power, best = candidate.power, candidate.quality[3].Max
		}
	}
	return power
}

// UseCubeAtlas chooses a mapping for any radial shape using measured surface error.
func UseCubeAtlas(shape Shape) bool { return AtlasPower(shape) != 0 }

var namedAtlasPowers sync.Map

// AtlasPowerByName caches the measured choice for the process lifetime; named
// radial definitions are immutable. Unknown names return zero.
func AtlasPowerByName(name string) float64 {
	if value, ok := namedAtlasPowers.Load(name); ok {
		return value.(float64)
	}
	shape, ok := ShapeByName(name)
	if !ok {
		return 0
	}
	power := AtlasPower(shape)
	value, _ := namedAtlasPowers.LoadOrStore(name, power)
	return value.(float64)
}
