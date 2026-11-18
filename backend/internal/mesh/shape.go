package mesh

import "math"

// Shape bends the sphere into another solid by scaling its radius per direction: a vertex
// that would sit at d·radius (|d| = 1) moves to d·radius·shape(d). Topology, UVs and the photo
// projection stay those of the sphere, so every shape fits the same vertex/index budgets and
// uses the same texture. nil is the plain sphere (and keeps the output byte-identical to it).
type Shape func(d Vec3) float64

// ShapeNames lists the shapes in display order; "sphere" is the plain sphere.
var ShapeNames = []string{"sphere", "cube", "gem", "coin", "egg"}

// ShapeByName returns the shape for a name in ShapeNames.
func ShapeByName(name string) (Shape, bool) {
	switch name {
	case "sphere", "":
		return nil, true
	case "cube": // a rounded die
		return superellipsoid(0.8, 0.8, 0.8, 0.32, 0.32), true
	case "gem": // a soft octahedron, a little taller than wide
		return superellipsoid(0.94, 0.94, 1, 1.7, 1.7), true
	case "coin": // a thick disc lying flat: photos on both faces
		return superellipsoid(1, 1, 0.46, 0.25, 1), true
	case "egg":
		ellipsoid := superellipsoid(0.8, 0.8, 0.92, 1, 1)
		return func(d Vec3) float64 { return ellipsoid(d) * (1 + 0.08*d[2]) }, true
	}
	return nil, false
}

// superellipsoid returns the radial function of
// (|x/a|^(2/e2) + |y/b|^(2/e2))^(e2/e1) + |z/c|^(2/e1) = 1.
// Small exponents square the shape off (cube, cylinder); 1 is an ellipsoid; 2 an octahedron.
// The implicit function is homogeneous of degree 2/e1, so the radius has a closed form.
func superellipsoid(a, b, c, e1, e2 float64) Shape {
	return func(d Vec3) float64 {
		xy := math.Pow(math.Abs(d[0]/a), 2/e2) + math.Pow(math.Abs(d[1]/b), 2/e2)
		f := math.Pow(xy, e2/e1) + math.Pow(math.Abs(d[2]/c), 2/e1)
		return math.Pow(f, -e1/2)
	}
}

// shapedGrid reserves columns for all eight octants, then fills the triangle and vertex
// budgets with rings. The plain sphere continues to use SphereGrid unchanged.
func shapedGrid(vertices, triangles int) Grid {
	base := SphereGrid(vertices, triangles)
	nu := base.NU / 8 * 8
	if nu < 8 {
		return base
	}
	nv := min(vertices/(nu+1)-1, triangles/(2*nu))
	g := Grid{nu, nv}
	// Small budgets can lose too much detail when rounded down.
	if g.Vertices()*100 < base.Vertices()*92 {
		return base
	}
	return g
}

// sampleCurve equalizes a blend of surface arc length and tangent rotation. The latter
// gives extra samples to rounded edges while arc length retains coverage on flat faces.
func sampleCurve(count int, start, end float64, at func(float64) Vec3) []float64 {
	const steps = 2048
	points := make([]Vec3, steps+1)
	lengths := make([]float64, steps+1)
	for i := range points {
		points[i] = at(start + (end-start)*float64(i)/steps)
	}
	turn := make([]float64, steps+1)
	for i := 1; i < steps; i++ {
		prev, next := points[i].Sub(points[i-1]), points[i+1].Sub(points[i])
		if prev.Dot(prev) > 1e-24 && next.Dot(next) > 1e-24 {
			turn[i] = math.Acos(math.Max(-1, math.Min(1, prev.Normalize().Dot(next.Normalize()))))
		}
	}
	for i := 1; i <= steps; i++ {
		delta := points[i].Sub(points[i-1])
		lengths[i] = lengths[i-1] + math.Sqrt(delta.Dot(delta)) + 0.125*(turn[i-1]+turn[i])
	}
	out := make([]float64, count+1)
	out[count] = end
	k := 1
	for i := 1; i < count; i++ {
		target := lengths[steps] * float64(i) / float64(count)
		for k < steps && lengths[k] < target {
			k++
		}
		fraction := (target - lengths[k-1]) / (lengths[k] - lengths[k-1])
		out[i] = start + (end-start)*(float64(k-1)+fraction)/steps
	}
	out[0] = start
	return out
}

// ShapedSphere samples the continuous radial surface more densely around its bends.
// Each octant has matching reflected samples when the column count permits it.
func ShapedSphere(radius float64, g Grid, shape Shape) Sphere {
	s := UVSphere(radius, g)
	if shape == nil {
		return s
	}
	at := func(th, ph float64) Vec3 {
		d, _, _ := (Equirect{}).TexelDir(ph/(2*math.Pi), th/math.Pi)
		return d.Scale(shape(d))
	}
	theta := sampleCurve(g.NV, 0, math.Pi, func(th float64) Vec3 { return at(th, math.Pi/4) })
	// Allocate segments to octants as evenly as the budget permits. If NU is not
	// divisible by eight, exact mirror symmetry is impossible with a fixed seam.
	phi := make([]float64, 0, g.NU+1)
	for oct := 0; oct < 8; oct++ {
		count := (oct+1)*g.NU/8 - oct*g.NU/8
		if count == 0 {
			continue
		}
		base := sampleCurve(count, 0, math.Pi/4, func(ph float64) Vec3 { return at(math.Pi/2, ph) })
		for i := 0; i < count; i++ {
			x := base[i]
			if oct%2 == 1 {
				x = math.Pi/4 - base[count-i]
			}
			phi = append(phi, float64(oct)*math.Pi/4+x)
		}
	}
	phi = append(phi, 2*math.Pi)
	const h = 1e-4
	for j, th := range theta {
		for i, ph := range phi {
			k := j*(g.NU+1) + i
			s.Pos[k] = at(th, ph).Scale(radius)
			s.UV[k] = [2]float64{ph / (2 * math.Pi), th / math.Pi}
			tc := math.Min(math.Max(th, 2*h), math.Pi-2*h)
			du := at(tc, ph+h).Sub(at(tc, ph-h))
			dv := at(tc+h, ph).Sub(at(tc-h, ph))
			n := dv.Cross(du).Normalize()
			if j == 0 || j == g.NV {
				n = Vec3{0, 0, math.Cos(th)}
			}
			t := Vec3{-math.Sin(ph), math.Cos(ph), 0}
			t = t.Sub(n.Scale(t.Dot(n))).Normalize()
			s.Normal[k], s.Tangent[k] = n, t
		}
	}
	// The nondegenerate top triangle uses b, while the bottom one uses c.
	for i := 0; i < g.NU; i++ {
		center := (phi[i] + phi[i+1]) / (4 * math.Pi)
		s.UV[i+1][0] = center
		s.UV[g.NV*(g.NU+1)+i][0] = center
	}
	return s
}

// MaxExtent returns, in sphere radii, the largest distance from the centre and the largest
// single coordinate of a shape (both 1 for the sphere), sampled on a fine grid. Tests use it to
// keep every shape inside the original model's bounds (a ±1.006-radii box).
func MaxExtent(shape Shape) (radial, axis float64) {
	if shape == nil {
		return 1, 1
	}
	for j := 0; j <= 180; j++ {
		for i := 0; i < 360; i++ {
			th, ph := float64(j)*math.Pi/180, float64(i)*math.Pi/180
			d := Vec3{math.Sin(th) * math.Cos(ph), math.Sin(th) * math.Sin(ph), math.Cos(th)}
			r := shape(d)
			radial = math.Max(radial, r)
			axis = math.Max(axis, r*math.Max(math.Abs(d[0]), math.Max(math.Abs(d[1]), math.Abs(d[2]))))
		}
	}
	return radial, axis
}
