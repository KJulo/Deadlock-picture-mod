package mesh

import (
	"encoding/binary"
	"fmt"
	"math"
)

// UseCubeAtlas chooses the measured atlas path; shaped surfaces keep adaptive sampling.
func UseCubeAtlas(shape Shape) bool { return shape == nil }

// CubeAtlas maps six equiangular cube faces to square islands in a 3 by 2 atlas.
// Eight texels at the native 2048 resolution surround each island for filtering.
type CubeAtlas struct{}

const atlasInset = 8.0 / 2048

var atlasNormals = [...]Vec3{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}
var atlasRight = [...]Vec3{{0, 1, 0}, {0, -1, 0}, {-1, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 1, 0}}

// VertexUV returns the atlas coordinates for a point on a face, with x and y in [0,1].
func (CubeAtlas) VertexUV(face int, x, y float64) (float64, float64) {
	s := 1.0/3 - 2*atlasInset
	return float64(face%3)/3 + atlasInset + x*s, float64(face/3)/2 + 1.0/12 + atlasInset + y*s
}

// TexelDir returns the face direction and its increasing-u tangent. Outside the
// islands it returns the nearest edge direction and ok=false for colour bleed.
func (a CubeAtlas) TexelDir(u, v float64) (d, tangent Vec3, ok bool) {
	best := math.Inf(1)
	face, x, y := 0, 0.0, 0.0
	s := 1.0/3 - 2*atlasInset
	for k := 0; k < 6; k++ {
		u0, v0 := a.VertexUV(k, 0, 0)
		px, py := (u-u0)/s, (v-v0)/s
		cx, cy := clamp01(px), clamp01(py)
		dist := (px-cx)*(px-cx) + (py-cy)*(py-cy)
		if dist < best {
			best, face, x, y = dist, k, cx, cy
			ok = px >= 0 && px <= 1 && py >= 0 && py <= 1
		}
	}
	n, r := atlasNormals[face], atlasRight[face]
	up := n.Cross(r)
	d = n.Add(r.Scale(math.Tan((x - 0.5) * math.Pi / 2))).Add(up.Scale(math.Tan((y - 0.5) * math.Pi / 2))).Normalize()
	tangent = r.Sub(d.Scale(r.Dot(d))).Normalize()
	return
}

// Equirect maps the original latitude-longitude texture.
type Equirect struct{}

// TexelDir returns the original direction and increasing-u tangent.
func (Equirect) TexelDir(u, v float64) (Vec3, Vec3, bool) {
	th, ph := v*math.Pi, u*2*math.Pi
	return Vec3{math.Sin(th) * math.Cos(ph), math.Sin(th) * math.Sin(ph), math.Cos(th)}, Vec3{-math.Sin(ph), math.Cos(ph), 0}, true
}

// CubeSphere samples each island independently, duplicating cube edges for UV seams.
func CubeSphere(radius float64, segments int, shape Shape) Sphere {
	var s Sphere
	a := CubeAtlas{}
	at := func(u, v float64) Vec3 {
		d, _, _ := a.TexelDir(u, v)
		if shape != nil {
			return d.Scale(shape(d))
		}
		return d
	}
	for face := 0; face < 6; face++ {
		base := uint32(len(s.Pos))
		for j := 0; j <= segments; j++ {
			for i := 0; i <= segments; i++ {
				u, v := a.VertexUV(face, float64(i)/float64(segments), float64(j)/float64(segments))
				d, t, _ := a.TexelDir(u, v)
				n := d
				if shape != nil {
					// One-sided differences at island boundaries avoid crossing seams.
					const h = 1e-6
					u0, v0 := a.VertexUV(face, 0, 0)
					u1, v1 := a.VertexUV(face, 1, 1)
					du := at(math.Min(u1, u+h), v).Sub(at(math.Max(u0, u-h), v))
					dv := at(u, math.Min(v1, v+h)).Sub(at(u, math.Max(v0, v-h)))
					n = du.Cross(dv).Normalize()
					t = du.Sub(n.Scale(du.Dot(n))).Normalize()
				}
				p := d.Scale(radius)
				if shape != nil {
					p = p.Scale(shape(d))
				}
				s.Pos = append(s.Pos, p)
				s.Normal = append(s.Normal, n)
				s.Tangent = append(s.Tangent, t)
				s.UV = append(s.UV, [2]float64{u, v})
			}
		}
		for j := 0; j < segments; j++ {
			for i := 0; i < segments; i++ {
				p := base + uint32(j*(segments+1)+i)
				q := p + uint32(segments+1)
				s.Tris = append(s.Tris, [3]uint32{p, p + 1, q}, [3]uint32{p + 1, q + 1, q})
			}
		}
	}
	return s
}

// CubeSphereGeometry packs the largest atlas grid that fits the fixed LOD budgets.
func CubeSphereGeometry(radius float64, shape Shape) Geometry {
	return func(vertexCount, indexCount int, uv UVFormat, remap UVRemap) (Buffers, error) {
		segments := 0
		for n := 1; 6*(n+1)*(n+1) <= vertexCount && 36*n*n <= indexCount; n++ {
			segments = n
		}
		if segments == 0 {
			return Buffers{}, fmt.Errorf("mesh: budget too small for cube atlas")
		}
		s := CubeSphere(radius, segments, shape)
		vb := make([]byte, vertexCount*VertexSize)
		for k, p := range s.Pos {
			v := vb[k*VertexSize : (k+1)*VertexSize]
			for c := 0; c < 3; c++ {
				binary.LittleEndian.PutUint32(v[4*c:], math.Float32bits(float32(p[c])))
			}
			u, w := s.UV[k][0], s.UV[k][1]
			if remap != nil {
				u, w = remap(u, w)
			}
			binary.LittleEndian.PutUint16(v[12:], uv.Encode(u))
			binary.LittleEndian.PutUint16(v[14:], uv.Encode(w))
			binary.LittleEndian.PutUint32(v[16:], PackTangentFrame(s.Normal[k], s.Tangent[k], 1))
		}
		for k := len(s.Pos); k < vertexCount; k++ {
			copy(vb[k*VertexSize:], vb[:VertexSize])
		}
		idx := make([]uint32, indexCount)
		for k, t := range s.Tris {
			copy(idx[3*k:], t[:])
		}
		return Buffers{vb, idx, len(s.Pos), len(s.Tris), Grid{segments, segments}}, nil
	}
}
