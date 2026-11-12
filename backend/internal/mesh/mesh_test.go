package mesh

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

const testdata = "../../../testdata"

type goldenLOD struct {
	VertexCount int    `json:"vertex_count"`
	IndexCount  int    `json:"index_count"`
	Grid        [2]int `json:"grid"`
	Offset      int    `json:"offset"`
}

func loadGolden(t *testing.T) (radius float64, lods []goldenLOD, blob []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(testdata, "mesh_buffers.json"))
	if err != nil {
		t.Skip("no golden file: run `make golden`")
	}
	var meta struct {
		Radius float64     `json:"radius"`
		LODs   []goldenLOD `json:"lods"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(testdata, "mesh_buffers.bin.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	blob, err = io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return meta.Radius, meta.LODs, blob
}

// The Go sphere must produce the same vertex and index bytes as reference/catmod/mesh.py.
func TestMatchesPythonReference(t *testing.T) {
	radius, lods, blob := loadGolden(t)
	le := binary.LittleEndian
	for n, lod := range lods {
		vb, idx, g, err := BuildBuffers(lod.VertexCount, lod.IndexCount, radius)
		if err != nil {
			t.Fatal(err)
		}
		if g != (Grid{lod.Grid[0], lod.Grid[1]}) {
			t.Fatalf("LOD%d grid %v, want %v", n, g, lod.Grid)
		}
		wantVB := blob[lod.Offset : lod.Offset+lod.VertexCount*VertexSize]
		wantIdx := blob[lod.Offset+len(wantVB) : lod.Offset+len(wantVB)+lod.IndexCount*4]

		gotIdx := make([]byte, 4*len(idx))
		for i, v := range idx {
			le.PutUint32(gotIdx[4*i:], v)
		}
		if !bytes.Equal(gotIdx, wantIdx) {
			t.Errorf("LOD%d: indices differ", n)
		}

		diffs := map[string]int{}
		for k := 0; k < lod.VertexCount; k++ {
			g, w := vb[k*VertexSize:(k+1)*VertexSize], wantVB[k*VertexSize:(k+1)*VertexSize]
			if !bytes.Equal(g[0:12], w[0:12]) {
				diffs["position"]++
			}
			if !bytes.Equal(g[12:16], w[12:16]) {
				diffs["uv"]++
			}
			if !bytes.Equal(g[16:20], w[16:20]) {
				diffs["tangent frame"]++
				if diffs["tangent frame"] <= 3 {
					t.Logf("LOD%d v%d tangent frame %08x, want %08x", n, k, le.Uint32(g[16:]), le.Uint32(w[16:]))
				}
			}
			if !bytes.Equal(g[20:24], w[20:24]) {
				diffs["blend"]++
			}
		}
		for field, count := range diffs {
			t.Errorf("LOD%d: %s differs in %d of %d vertices", n, field, count, lod.VertexCount)
		}
	}
}

func TestFloat16(t *testing.T) {
	cases := map[float64]uint16{
		0: 0x0000, 1: 0x3c00, 0.5: 0x3800, -2: 0xc000, 65504: 0x7bff,
		1.0 / 3:               0x3555, // rounds down
		6.103515625e-05:       0x0400, // smallest normal
		5.960464477539063e-08: 0x0001, // smallest subnormal
		1 + 1.0/2048:          0x3c00, // exactly halfway -> ties to even (down)
		1 + 3.0/2048:          0x3c02, // halfway -> ties to even (up)
	}
	for x, want := range cases {
		if got := Float16(x); got != want {
			t.Errorf("Float16(%v) = %#04x, want %#04x", x, got, want)
		}
	}
}

func angleDeg(a, b Vec3) float64 {
	c := a.Normalize().Dot(b.Normalize())
	return math.Acos(math.Max(-1, math.Min(1, c))) * 180 / math.Pi
}

// Decoding a packed frame must give back (almost) the original normal and tangent. Same limits as
// the python test: 10-bit octahedral normals are good to ~0.25°, 11-bit tangent angles to ~0.5°.
func TestTangentFrameRoundtrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	gauss := func() Vec3 { return Vec3{rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64()} }
	for k := 0; k < 5000; k++ {
		n := gauss().Normalize()
		tan := n.Cross(gauss()).Normalize()
		p := PackTangentFrame(n, tan, 1)
		if p&1 != 1 {
			t.Fatal("bitangent sign bit not set")
		}
		nd := UnpackNormal(p>>12&1023, p>>22&1023)
		if a := angleDeg(nd, n); a > 0.25 {
			t.Fatalf("normal %v decoded as %v (%.3f°)", n, nd, a)
		}
		ang := float64(p>>1&2047) / 2047 * 2 * math.Pi
		b1, b2 := FrisvadBasis(nd)
		td := b1.Scale(math.Cos(ang)).Add(b2.Scale(math.Sin(ang)))
		tp := tan.Sub(nd.Scale(tan.Dot(nd)))
		if a := angleDeg(td, tp); a > 0.5 {
			t.Fatalf("tangent %v decoded as %v (%.3f°)", tp, td, a)
		}
	}
}

// Triangles must face outward: cross(B-A, C-A) points away from the centre.
func TestWindingOutward(t *testing.T) {
	s := UVSphere(6.3, Grid{20, 10})
	for _, tr := range s.Tris {
		a, b, c := s.Pos[tr[0]], s.Pos[tr[1]], s.Pos[tr[2]]
		n := b.Sub(a).Cross(c.Sub(a))
		if n.Dot(n) < 1e-12 {
			continue // degenerate triangle at a pole
		}
		if n.Dot(a.Add(b).Add(c)) <= 0 {
			t.Fatalf("triangle %v faces inward", tr)
		}
	}
}

func TestBudgets(t *testing.T) {
	for _, c := range [][2]int{{6084, 17997}, {3196, 9000}, {1531, 3576}, {1444, 3264}} {
		vb, idx, g, err := BuildBuffers(c[0], c[1], 6.3)
		if err != nil {
			t.Fatal(err)
		}
		if len(vb) != c[0]*VertexSize || len(idx) != c[1] {
			t.Errorf("%v: got %d bytes / %d indices", c, len(vb), len(idx))
		}
		for _, i := range idx {
			if int(i) >= g.Vertices() {
				t.Fatalf("%v: index %d out of range", c, i)
			}
		}
	}
}
