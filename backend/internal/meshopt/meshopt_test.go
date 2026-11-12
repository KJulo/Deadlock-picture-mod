package meshopt

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const testdata = "../../../testdata"

type goldenCase struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	VertexSize  int    `json:"vertex_size"`
	VertexCount int    `json:"vertex_count"`
	Input       [2]int `json:"input"`
	Output      [2]int `json:"output"`
}

func loadCases(t *testing.T) ([]goldenCase, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(testdata, "meshopt_cases.json"))
	if err != nil {
		t.Skip("no golden file: run `make golden`")
	}
	var cases []goldenCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(testdata, "meshopt_cases.bin.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return cases, blob
}

// Every case must compress to exactly the bytes the C library (python binding) produced.
func TestMatchesPythonReference(t *testing.T) {
	cases, blob := loadCases(t)
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			in := blob[c.Input[0] : c.Input[0]+c.Input[1]]
			want := blob[c.Output[0] : c.Output[0]+c.Output[1]]
			var got []byte
			var err error
			switch c.Kind {
			case "vertex":
				got, err = EncodeVertexBuffer(in, c.VertexSize)
			case "index":
				idx := make([]uint32, len(in)/4)
				for i := range idx {
					idx[i] = binary.LittleEndian.Uint32(in[4*i:])
				}
				got, err = EncodeIndexBuffer(idx)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				i := 0
				for i < len(got) && i < len(want) && got[i] == want[i] {
					i++
				}
				t.Fatalf("differs at byte %d (len got %d, want %d)", i, len(got), len(want))
			}
		})
	}
}

func TestRejectsBadInput(t *testing.T) {
	if _, err := EncodeVertexBuffer(make([]byte, 25), 24); err == nil {
		t.Error("vertex data not a multiple of vertex size: expected error")
	}
	if _, err := EncodeVertexBuffer(make([]byte, 6), 6); err == nil {
		t.Error("vertex size not a multiple of 4: expected error")
	}
	if _, err := EncodeIndexBuffer([]uint32{0, 1}); err == nil {
		t.Error("index count not a multiple of 3: expected error")
	}
}
