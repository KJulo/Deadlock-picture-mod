// Package mesh generates the sphere in the soul_container vertex layout (port of
// reference/catmod/mesh.py).
//
// Vertex layout (24 bytes):
//
//	0  POSITION      3 × f32                 Source units, Z up
//	12 TEXCOORD      2 × 16 bit              in the format each LOD declares (UVFormat)
//	16 NORMAL        u32                     CompressedTangentFrame (PackTangentFrame)
//	20 BLENDINDICES  4 × u8                  all zero (single bone)
//
// UV convention (shared with the photo projection): direction
// d(θ, φ) = (sinθ cosφ, sinθ sinφ, cosθ), u = φ/2π, v = θ/π (v = 0 at the top pole, +Z).
package mesh

import (
	"encoding/binary"
	"fmt"
	"math"
)

// VertexSize is the size in bytes of one vertex.
const VertexSize = 24

// Vec3 is a 3D vector.
type Vec3 [3]float64

func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }
func (a Vec3) Sub(b Vec3) Vec3      { return Vec3{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func (a Vec3) Scale(s float64) Vec3 { return Vec3{a[0] * s, a[1] * s, a[2] * s} }
func (a Vec3) Dot(b Vec3) float64   { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func (a Vec3) Normalize() Vec3      { return a.Scale(1 / math.Sqrt(a.Dot(a))) }
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

// Grid is the resolution of a UV sphere: NU segments around, NV rings from pole to pole.
type Grid struct{ NU, NV int }

// Vertices of a UV sphere with the seam duplicated: (NU+1) × (NV+1).
func (g Grid) Vertices() int { return (g.NU + 1) * (g.NV + 1) }

// Triangles of a UV sphere (the pole rows are degenerate quads, kept for simplicity).
func (g Grid) Triangles() int { return 2 * g.NU * g.NV }

// SphereGrid returns the largest sphere (NU = 2·NV) that fits both budgets.
func SphereGrid(maxVertices, maxTriangles int) Grid {
	best := Grid{4, 2}
	for nv := 2; nv < 400; nv++ {
		g := Grid{2 * nv, nv}
		if g.Vertices() <= maxVertices && g.Triangles() <= maxTriangles {
			best = g
		}
	}
	return best
}

func signNonNeg(x float64) float64 { // +1 for x >= 0, -1 otherwise
	if x >= 0 {
		return 1
	}
	return -1
}

func sign(x float64) float64 { // like numpy.sign: 0 for 0
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return 0
}

// FrisvadBasis returns two unit vectors that, with n, form an orthonormal basis (Duff et al. 2017).
// The game rebuilds tangents from the normal with this exact basis, so it must match bit for bit
// in spirit: same formulas, same branch on the sign of n.z.
func FrisvadBasis(n Vec3) (b1, b2 Vec3) {
	s := signNonNeg(n[2])
	a := -1 / (s + n[2])
	b := n[0] * n[1] * a
	b1 = Vec3{1 + s*n[0]*n[0]*a, s * b, -s * n[0]}
	b2 = Vec3{b, s + n[1]*n[1]*a, -n[1]}
	return
}

// UnpackNormal decodes the 10+10-bit octahedral normal of a CompressedTangentFrame.
func UnpackNormal(xb, yb uint32) Vec3 {
	x := float64(xb)/1023*2 - 1
	y := float64(yb)/1023*2 - 1
	z := 1 - math.Abs(x) - math.Abs(y)
	if z < 0 {
		x, y = (1-math.Abs(y))*sign(x), (1-math.Abs(x))*sign(y)
	}
	return Vec3{x, y, z}.Normalize()
}

func quantize(v, steps float64) uint32 {
	q := math.RoundToEven((v + 1) / 2 * steps)
	return uint32(math.Max(0, math.Min(steps, q)))
}

// PackTangentFrame packs a unit normal n, a tangent t and the bitangent sign into Source 2's
// 32-bit CompressedTangentFrame:
//
//	bit 0      bitangent sign (1 -> +1)
//	bits 1-11  tangent angle inside FrisvadBasis(n), angle/2π × 2047
//	bits 12-21 octahedral normal X, (x+1)/2 × 1023
//	bits 22-31 octahedral normal Y
func PackTangentFrame(n, t Vec3, bitangentSign float64) uint32 {
	n = n.Normalize()
	// octahedral projection: onto the |x|+|y|+|z| = 1 diamond, lower half folded over
	l1 := math.Abs(n[0]) + math.Abs(n[1]) + math.Abs(n[2])
	px, py := n[0]/l1, n[1]/l1
	if n[2] < 0 {
		px, py = (1-math.Abs(py))*signNonNeg(px), (1-math.Abs(px))*signNonNeg(py)
	}
	xb, yb := quantize(px, 1023), quantize(py, 1023)

	// Tangent angle measured around the *quantized* normal: that is what the shader rebuilds.
	nq := UnpackNormal(xb, yb)
	b1, b2 := FrisvadBasis(nq)
	t = t.Sub(nq.Scale(t.Dot(nq)))
	ang := math.Mod(math.Atan2(t.Dot(b2), t.Dot(b1)), 2*math.Pi)
	if ang < 0 {
		ang += 2 * math.Pi // numpy.mod is always >= 0 for a positive divisor
	}
	tb := uint32(math.RoundToEven(ang/(2*math.Pi)*2047)) % 2048
	tb = min(tb, 2047)

	var sb uint32
	if bitangentSign > 0 {
		sb = 1
	}
	return sb | tb<<1 | xb<<12 | yb<<22
}

// Float16 converts x to IEEE 754 half precision (round to nearest, ties to even), the same as
// numpy's float64 -> float16. Only finite values with |x| < 65520 are supported (UVs are 0..1).
func Float16(x float64) uint16 {
	var signBit uint16
	if math.Signbit(x) {
		signBit, x = 0x8000, -x
	}
	if x == 0 {
		return signBit
	}
	_, e := math.Frexp(x) // x = frac × 2^e, frac in [0.5, 1) -> unbiased exponent is e-1
	exp := e - 1
	if exp < -14 { // subnormal: value = m × 2^-24
		return signBit | uint16(math.RoundToEven(math.Ldexp(x, 24)))
	}
	m := uint16(math.RoundToEven(math.Ldexp(x, 10-exp))) // 1024..2048
	if m == 2048 {                                       // rounding carried into the next power of two
		m, exp = 1024, exp+1
	}
	return signBit | uint16(exp+15)<<10 | (m - 1024)
}

// Sphere holds the geometry of a UV sphere before packing.
type Sphere struct {
	Pos, Normal, Tangent []Vec3
	UV                   [][2]float64
	Tris                 [][3]uint32
}

// UVSphere builds a UV sphere with the seam duplicated (port of mesh.uv_sphere, full sphere).
func UVSphere(radius float64, g Grid) Sphere {
	var s Sphere
	for j := 0; j <= g.NV; j++ {
		for i := 0; i <= g.NU; i++ {
			u, v := float64(i)/float64(g.NU), float64(j)/float64(g.NV)
			th, ph := v*math.Pi, u*2*math.Pi
			n := Vec3{math.Sin(th) * math.Cos(ph), math.Sin(th) * math.Sin(ph), math.Cos(th)}
			s.Normal = append(s.Normal, n)
			s.Pos = append(s.Pos, n.Scale(radius))
			s.Tangent = append(s.Tangent, Vec3{-math.Sin(ph), math.Cos(ph), 0}) // dP/du
			s.UV = append(s.UV, [2]float64{u, v})
		}
	}
	// Winding like the original model: cross(B-A, C-A) points outward. All (a,c,b) triangles
	// first, then all (b,c,d), in the same order as the python reference.
	row := uint32(g.NU + 1)
	quad := func(j, i int) (a, b, c, d uint32) {
		a = uint32(j)*row + uint32(i)
		return a, a + 1, a + row, a + row + 1
	}
	for j := 0; j < g.NV; j++ {
		for i := 0; i < g.NU; i++ {
			a, b, c, _ := quad(j, i)
			s.Tris = append(s.Tris, [3]uint32{a, c, b})
		}
	}
	for j := 0; j < g.NV; j++ {
		for i := 0; i < g.NU; i++ {
			_, b, c, d := quad(j, i)
			s.Tris = append(s.Tris, [3]uint32{b, c, d})
		}
	}
	return s
}

func clamp01(x float64) float64 { return math.Max(0, math.Min(1, x)) }

// UVFormat is how a LOD stores its TEXCOORD stream. Each LOD's vertex layout declares its own:
// in soul_container.vmdl_c LODs 0-2 are R16G16_SNORM (format 37) and LOD3 is R16G16_FLOAT (34).
// The game honours the declaration, so the bytes must match it.
type UVFormat int

const (
	UVSnorm UVFormat = iota // R16G16_SNORM: value × 32767 as int16
	UVHalf                  // R16G16_FLOAT: IEEE half float
)

func (f UVFormat) String() string {
	if f == UVHalf {
		return "f16"
	}
	return "snorm"
}

// Encode stores a UV coordinate (clamped to 0..1) in this format.
func (f UVFormat) Encode(x float64) uint16 {
	x = clamp01(x)
	if f == UVHalf {
		return Float16(x)
	}
	return uint16(int16(math.RoundToEven(x * 32767))) // numpy.rint rounds half to even too
}

// Decode reads a stored UV coordinate the way the GPU does.
func (f UVFormat) Decode(b uint16) float64 {
	if f == UVHalf {
		return HalfToFloat(b)
	}
	return math.Max(-1, float64(int16(b))/32767)
}

// UVRemap optionally moves the sphere's UVs (used by the LOD diagnostic). nil = unchanged.
type UVRemap func(u, v float64) (float64, float64)

// BuildBuffers returns the vertex bytes (vertexCount × 24) and indices (indexCount) of the
// largest sphere that fits. The counts must equal the original LOD's so the model metadata stays
// valid: unused vertices repeat vertex 0, unused indices are 0 (invisible degenerate triangles).
func BuildBuffers(vertexCount, indexCount int, radius float64, uv UVFormat, remap UVRemap) ([]byte, []uint32, Grid, error) {
	return BuildShapeBuffers(vertexCount, indexCount, radius, nil, uv, remap)
}

// BuildShapeBuffers is BuildBuffers for any Shape (nil = the sphere).
func BuildShapeBuffers(vertexCount, indexCount int, radius float64, shape Shape, uv UVFormat, remap UVRemap) ([]byte, []uint32, Grid, error) {
	g := SphereGrid(vertexCount, indexCount/3)
	if g.Vertices() > vertexCount || 3*g.Triangles() > indexCount {
		return nil, nil, g, fmt.Errorf("mesh: budget %d vertices / %d indices too small", vertexCount, indexCount)
	}
	s := ShapedSphere(radius, g, shape)
	le := binary.LittleEndian
	vb := make([]byte, vertexCount*VertexSize)
	for k := range s.Pos {
		v := vb[k*VertexSize : (k+1)*VertexSize]
		for c := 0; c < 3; c++ {
			le.PutUint32(v[4*c:], math.Float32bits(float32(s.Pos[k][c])))
		}
		u, w := s.UV[k][0], s.UV[k][1]
		if remap != nil {
			u, w = remap(u, w)
		}
		le.PutUint16(v[12:], uv.Encode(u))
		le.PutUint16(v[14:], uv.Encode(w))
		le.PutUint32(v[16:], PackTangentFrame(s.Normal[k], s.Tangent[k], 1))
		// v[20:24] blend indices stay 0
	}
	for k := len(s.Pos); k < vertexCount; k++ {
		copy(vb[k*VertexSize:], vb[:VertexSize])
	}
	idx := make([]uint32, indexCount)
	for k, t := range s.Tris {
		copy(idx[3*k:], t[:])
	}
	return vb, idx, g, nil
}

// HalfToFloat decodes an IEEE 754 half-precision value (what the game reads from the UV bytes).
func HalfToFloat(h uint16) float64 {
	sign := 1.0
	if h&0x8000 != 0 {
		sign = -1
	}
	exp, frac := int(h>>10&0x1f), float64(h&0x3ff)
	switch exp {
	case 0: // zero / subnormal
		return sign * math.Ldexp(frac, -24)
	case 31:
		if frac == 0 {
			return sign * math.Inf(1)
		}
		return math.NaN()
	}
	return sign * math.Ldexp(1+frac/1024, exp-15)
}
