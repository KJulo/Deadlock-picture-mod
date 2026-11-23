package mesh

import "math"

var (
	d20Planes     [10]Vec3
	brilliantAxes [8][2]float64
)

func init() {
	phi := (1 + math.Sqrt(5)) / 2
	inv := 1 / phi
	raw := [10]Vec3{
		{1, 1, 1}, {1, 1, -1}, {1, -1, 1}, {-1, 1, 1},
		{0, inv, phi}, {0, inv, -phi},
		{inv, phi, 0}, {inv, -phi, 0},
		{phi, 0, inv}, {phi, 0, -inv},
	}
	for i, p := range raw {
		d20Planes[i] = p.Normalize()
	}
	for i := range brilliantAxes {
		a := float64(i) * math.Pi / 4
		brilliantAxes[i] = [2]float64{math.Cos(a), math.Sin(a)}
	}
}

// roundedPlanes approximates the maximum of opposing plane projections with an even p-norm.
func roundedPlanes(d Vec3, planes []Vec3, power float64) float64 {
	sum := 0.0
	for _, n := range planes {
		sum += math.Pow(math.Abs(n.Dot(d)), power)
	}
	return math.Pow(sum, 1/power)
}

func d20Shape(d Vec3) float64 {
	return 0.78 / roundedPlanes(d, d20Planes[:], 16)
}

func capsuleShape(d Vec3) float64 {
	// Blend a cylindrical middle into the spherical ends using a smooth axial profile.
	return superellipsoid(0.64, 0.64, 0.98, 0.62, 1)(d)
}

func heartShape(d Vec3) float64 {
	// Cube-rooting the Taubin equation gives A*r² - B*r^(5/3) - 1 = 0.
	// X is depth, Y width, and Z up; smoothing B removes both axial cusps.
	a := d[1]*d[1] + 2.25*d[0]*d[0] + d[2]*d[2]
	b := d[2] * math.Cbrt(d[1]*d[1]+0.1125*d[0]*d[0]+0.002)
	// The equation has exactly one positive root: after its sole possible
	// positive turning point it rises from below zero to infinity.
	lo, hi := 0.0, 4.0
	r := 1 / math.Sqrt(a)
	for i := 0; i < 20; i++ {
		p := math.Cbrt(r * r)
		f := a*r*r - b*r*p - 1
		if math.Abs(f) < 1e-14 {
			break
		}
		if f > 0 {
			hi = r
		} else {
			lo = r
		}
		next := r - f/(2*a*r-(5.0/3)*b*p)
		if next <= lo || next >= hi || math.IsNaN(next) {
			next = (lo + hi) / 2
		}
		r = next
	}
	return 0.72 * r
}

func brilliantShape(d Vec3) float64 {
	// Smooth plane maxima form the table, eight crown facets and a pointed pavilion.
	// Softplus p-norm over positive projections keeps all edges differentiable.
	softplus12 := func(p float64) float64 {
		q := (p + math.Sqrt(p*p+0.0004)) / 2
		return math.Pow(q, 12)
	}
	sum := softplus12(d[2]/0.58) + softplus12(-d[2]/0.88)
	for _, ax := range brilliantAxes {
		side := (d[0]*ax[0] + d[1]*ax[1]) / 0.88
		sum += softplus12(side+d[2]*0.55) + softplus12(side-d[2]*0.75)
	}
	return 0.91 / math.Pow(sum, 1.0/12)
}
