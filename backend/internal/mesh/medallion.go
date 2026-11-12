package mesh

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Medallions: only the six seals of the cube6 layout, as loose curved caps of the sphere with
// nothing in between. The game's material has no transparency (alpha 0 renders black), so the
// gaps have to come from the geometry itself.
//
// Each cap uses its own disc of a texture atlas (3 columns × 2 rows of the square texture)
// instead of the sphere's equirectangular UVs, which avoids the u seam and the poles. The
// mapping lives here and is used both by the mesh (vertex → UV) and by the texture
// (texel → direction), so the two always agree.
//
// Every cap is emitted twice, the back copy with reversed winding and flipped normals: the
// game culls back faces, and without it a cap would vanish when seen through a gap.

// MedallionsShape is the shape name of the medallion geometry (it is not a radial Shape).
const MedallionsShape = "medallions"

// MedallionAxes are the cap centres, in the cube6 sticker order.
var MedallionAxes = [6]Vec3{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}

// MaxMedallionRad keeps neighbouring caps (90° apart) from overlapping.
const MaxMedallionRad = math.Pi / 4

const (
	atlasCols  = 3
	atlasCell  = 1.0 / atlasCols // cell size in UV units (square cells; rows use the top 2/3)
	atlasDiscR = 0.47 * atlasCell
)

// medallionFrame returns the cap centre c and two tangent axes (e1, e2 = c × e1).
func medallionFrame(k int) (c, e1, e2 Vec3) {
	c = MedallionAxes[k]
	ref := Vec3{0, 0, 1}
	if math.Abs(c[2]) > 0.5 {
		ref = Vec3{1, 0, 0}
	}
	e1 = ref.Sub(c.Scale(ref.Dot(c))).Normalize()
	return c, e1, c.Cross(e1)
}

// MedallionDir maps a point (x, y) of cap k's unit disc to its direction on the sphere
// (equidistant: the disc radius corresponds to capRad).
func MedallionDir(k int, x, y, capRad float64) Vec3 {
	c, e1, e2 := medallionFrame(k)
	rho := math.Hypot(x, y)
	if rho < 1e-12 {
		return c
	}
	a := rho * capRad
	return c.Scale(math.Cos(a)).Add(e1.Scale(math.Sin(a) * x / rho)).Add(e2.Scale(math.Sin(a) * y / rho))
}

// MedallionUV maps a point of cap k's unit disc to atlas UVs (v grows downwards).
func MedallionUV(k int, x, y float64) (u, v float64) {
	cu := (float64(k%atlasCols) + 0.5) * atlasCell
	cv := (float64(k/atlasCols) + 0.5) * atlasCell
	return cu + x*atlasDiscR, cv - y*atlasDiscR
}

// MedallionCell is the inverse of MedallionUV: the cap and unit-disc point of an atlas UV.
// rho may exceed 1 near a disc (the texture bleeds the edge colour outwards); ok is false
// outside every cell.
func MedallionCell(u, v float64) (k int, x, y float64, ok bool) {
	col, row := int(u/atlasCell), int(v/atlasCell)
	if u < 0 || v < 0 || col >= atlasCols || row >= 2 {
		return 0, 0, 0, false
	}
	k = row*atlasCols + col
	cu, cv := MedallionUV(k, 0, 0)
	return k, (u - cu) / atlasDiscR, (cv - v) / atlasDiscR, true
}

// Buffers are one LOD's vertex and index buffers, padded to the LOD's fixed sizes. Verts and
// Tris count the real (non-padding) vertices and triangles.
type Buffers struct {
	Vertices    []byte
	Indices     []uint32
	Verts, Tris int
	Grid        Grid // sphere resolution (zero for medallions)
}

// Geometry builds the buffers of one LOD from its budgets and UV format.
type Geometry func(vertexCount, indexCount int, uv UVFormat, remap UVRemap) (Buffers, error)

// SphereGeometry is the sphere bent by shape (nil = the plain sphere).
func SphereGeometry(radius float64, shape Shape) Geometry {
	return func(vertexCount, indexCount int, uv UVFormat, remap UVRemap) (Buffers, error) {
		vb, idx, g, err := BuildShapeBuffers(vertexCount, indexCount, radius, shape, uv, remap)
		return Buffers{vb, idx, g.Vertices(), g.Triangles(), g}, err
	}
}

// medallionRes picks the polar resolution of one disc (segments around, rings out) that fits a
// per-disc budget, keeping segments ≈ 4 × rings so the triangles stay well shaped.
func medallionRes(maxVerts, maxTris int) (segs, rings int) {
	for r := 1; r < 64; r++ {
		s := 4 * r
		if s < 8 {
			s = 8
		}
		if 1+r*s > maxVerts || s*(2*r-1) > maxTris {
			break
		}
		segs, rings = s, r
	}
	return
}

// MedallionGeometry is the six caps of angular radius capRad (clamped to MaxMedallionRad).
func MedallionGeometry(radius, capRad float64) Geometry {
	capRad = math.Max(0.05, math.Min(capRad, MaxMedallionRad))
	return func(vertexCount, indexCount int, uv UVFormat, _ UVRemap) (Buffers, error) {
		const discs = 2 * len(MedallionAxes) // front and back of each cap
		segs, rings := medallionRes(vertexCount/discs, indexCount/3/discs)
		if segs == 0 {
			return Buffers{}, fmt.Errorf("mesh: budget %d vertices / %d indices too small for medallions", vertexCount, indexCount)
		}
		le := binary.LittleEndian
		vb := make([]byte, vertexCount*VertexSize)
		idx := make([]uint32, indexCount)
		nv, nt := 0, 0
		put := func(p, n, t Vec3, u, v float64) {
			b := vb[nv*VertexSize:]
			for c := 0; c < 3; c++ {
				le.PutUint32(b[4*c:], math.Float32bits(float32(p[c])))
			}
			le.PutUint16(b[12:], uv.Encode(u))
			le.PutUint16(b[14:], uv.Encode(v))
			le.PutUint32(b[16:], PackTangentFrame(n, t, 1))
			nv++
		}
		var pos []Vec3 // positions of the current disc, to orient its triangles
		tri := func(base int, a, b, c int, out Vec3) {
			pa, pb, pc := pos[a], pos[b], pos[c]
			if pb.Sub(pa).Cross(pc.Sub(pa)).Dot(out) < 0 {
				b, c = c, b
			}
			idx[3*nt], idx[3*nt+1], idx[3*nt+2] = uint32(base+a), uint32(base+b), uint32(base+c)
			nt++
		}
		for k := range MedallionAxes {
			_, e1, _ := medallionFrame(k)
			for _, side := range []float64{1, -1} {
				base := nv
				pos = pos[:0]
				// vertex 0 is the centre; then ring by ring, segment by segment
				emit := func(x, y float64) {
					d := MedallionDir(k, x, y, capRad)
					n := d.Scale(side)
					t := e1.Sub(n.Scale(e1.Dot(n))).Normalize()
					u, v := MedallionUV(k, x, y)
					p := d.Scale(radius)
					pos = append(pos, p)
					put(p, n, t, u, v)
				}
				emit(0, 0)
				for r := 1; r <= rings; r++ {
					rho := float64(r) / float64(rings)
					for s := 0; s < segs; s++ {
						a := 2 * math.Pi * float64(s) / float64(segs)
						emit(rho*math.Cos(a), rho*math.Sin(a))
					}
				}
				at := func(r, s int) int { return 1 + (r-1)*segs + s%segs }
				out := MedallionAxes[k].Scale(side)
				for s := 0; s < segs; s++ {
					tri(base, 0, at(1, s), at(1, s+1), out)
				}
				for r := 1; r < rings; r++ {
					for s := 0; s < segs; s++ {
						tri(base, at(r, s), at(r+1, s), at(r+1, s+1), out)
						tri(base, at(r, s), at(r+1, s+1), at(r, s+1), out)
					}
				}
			}
		}
		for k := nv; k < vertexCount; k++ { // padding: like the sphere, repeat vertex 0
			copy(vb[k*VertexSize:], vb[:VertexSize])
		}
		return Buffers{vb, idx, nv, nt, Grid{}}, nil
	}
}
