package mesh

import "math"

// roundedPlanes approximates the maximum of opposing plane projections with an even p-norm.
func roundedPlanes(d Vec3, planes []Vec3, power float64) float64 {
	sum := 0.0
	for _, n := range planes {
		sum += math.Pow(math.Abs(n.Dot(d)), power)
	}
	return math.Pow(sum, 1/power)
}

func d20Shape(d Vec3) float64 {
	phi := (1 + math.Sqrt(5)) / 2
	inv := 1 / phi
	planes := []Vec3{{1, 1, 1}, {1, 1, -1}, {1, -1, 1}, {-1, 1, 1}, {0, inv, phi}, {0, inv, -phi}, {inv, phi, 0}, {inv, -phi, 0}, {phi, 0, inv}, {phi, 0, -inv}}
	for i := range planes {
		planes[i] = planes[i].Normalize()
	}
	return 0.78 / roundedPlanes(d, planes, 16)
}

func capsuleShape(d Vec3) float64 {
	// Blend a cylindrical middle into the spherical ends using a smooth axial profile.
	return superellipsoid(0.64, 0.64, 0.98, 0.62, 1)(d)
}

func heartShape(d Vec3) float64 {
	// The YZ silhouette is indented between the upper lobes and tapers below.
	y, z := d[1], d[2]
	indentation := math.Exp(-y*y/0.09) * math.Pow(math.Max(z, 0), 2)
	return 0.78 * (1 + 0.12*z - 0.46*indentation + 0.13*y*y*math.Max(z, 0))
}

func brilliantShape(d Vec3) float64 {
	// Smooth plane maxima form the table, eight crown facets and a pointed pavilion.
	planes := []float64{d[2] / 0.58, -d[2] / 0.88}
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		side := d[0]*math.Cos(a) + d[1]*math.Sin(a)
		planes = append(planes, side/0.88+d[2]*0.55, side/0.88-d[2]*0.75)
	}
	// Softplus p-norm over positive projections keeps all edges differentiable.
	sum := 0.0
	for _, p := range planes {
		q := (p + math.Sqrt(p*p+0.0004)) / 2
		sum += math.Pow(q, 12)
	}
	return 0.91 / math.Pow(sum, 1.0/12)
}
