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

// ShapedSphere is UVSphere with every vertex moved by shape. Normals come from the surface
// itself (finite differences on the continuous parametrization, so the seam stays smooth), and
// tangents keep following dP/du, re-orthogonalized against the new normal.
func ShapedSphere(radius float64, g Grid, shape Shape) Sphere {
	s := UVSphere(radius, g)
	if shape == nil {
		return s
	}
	at := func(th, ph float64) Vec3 {
		d := Vec3{math.Sin(th) * math.Cos(ph), math.Sin(th) * math.Sin(ph), math.Cos(th)}
		return d.Scale(radius * shape(d))
	}
	const h = 1e-4
	for k, uv := range s.UV {
		th, ph := uv[1]*math.Pi, uv[0]*2*math.Pi
		s.Pos[k] = at(th, ph)
		// at the poles dP/du vanishes: take the normal from just beside the pole
		tc := math.Min(math.Max(th, 2*h), math.Pi-2*h)
		du := at(tc, ph+h).Sub(at(tc, ph-h))
		dv := at(tc+h, ph).Sub(at(tc-h, ph))
		n := dv.Cross(du).Normalize() // outward (v grows downward from +Z)
		if th == 0 || th == math.Pi { // poles are axis-symmetric for every shape here
			n = Vec3{0, 0, math.Cos(th)}
		}
		t := s.Tangent[k]
		t = t.Sub(n.Scale(t.Dot(n))).Normalize()
		s.Normal[k], s.Tangent[k] = n, t
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
