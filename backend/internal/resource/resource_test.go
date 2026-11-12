package resource

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot is relative to this package directory (backend/internal/resource).
const repoRoot = "../../.."

// templateFiles returns every *_c file in templates/ (or $CATMOD_TEMPLATES). The game files are
// never committed, so the test is skipped when they are missing (e.g. in CI).
func templateFiles(t *testing.T) []string {
	t.Helper()
	dir := os.Getenv("CATMOD_TEMPLATES")
	if dir == "" {
		dir = filepath.Join(repoRoot, "templates")
	}
	var files []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, "_c") {
			files = append(files, path)
		}
		return nil
	})
	if len(files) == 0 {
		t.Skipf("no game files in %s: run `make fetch` first", dir)
	}
	return files
}

// Level 1: no game files needed. Build a tiny resource by hand and check the header bytes.
func TestSerializeSynthetic(t *testing.T) {
	r := &Resource{
		HeaderVersion: 12,
		Version:       0,
		Blocks: []Block{
			{Kind: "RED2", Data: []byte("hello"), Align: 16},
			{Kind: "RERL", Data: []byte("abcdefg"), Align: 4},
			{Kind: "DATA", Data: []byte("xyz")}, // Align 0 -> DefaultAlign
		},
		Trailer: []byte("PIXELS"),
	}
	out := r.Serialize()
	if out == nil {
		t.Fatal("Serialize returned nil")
	}

	// header 16 + table 3*12 = 52 -> RED2 at 64 (16-aligned), ends 69 -> RERL at 72 (4-aligned),
	// ends 79 -> DATA at 80 (16-aligned), ends 83. Size field = 83, then 6 trailer bytes.
	le := binary.LittleEndian
	checks := []struct {
		name      string
		got, want uint32
	}{
		{"file size", le.Uint32(out[0:]), 83},
		{"header version", uint32(le.Uint16(out[4:])), 12},
		{"table offset", le.Uint32(out[8:]), 8},
		{"block count", le.Uint32(out[12:]), 3},
		{"RED2 rel offset", le.Uint32(out[16+4:]), 64 - (16 + 4)},
		{"RERL rel offset", le.Uint32(out[28+4:]), 72 - (28 + 4)},
		{"DATA rel offset", le.Uint32(out[40+4:]), 80 - (40 + 4)},
		{"RERL size", le.Uint32(out[28+8:]), 7},
		{"total length", uint32(len(out)), 83 + 6},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}
	if string(out[72:79]) != "abcdefg" || string(out[83:]) != "PIXELS" {
		t.Errorf("payload/trailer not where expected")
	}
}

// Level 2: Parse what Serialize wrote and get the same thing back.
func TestParseSynthetic(t *testing.T) {
	want := &Resource{
		HeaderVersion: 12,
		Blocks: []Block{
			// header 16 + table 24 = 40 -> RED2 at 48, ends 59 -> RERL at 60. 60 is a multiple of 4
			// but not of 8, so Parse can only infer Align 4. (With a 5-byte RED2 the RERL would land
			// on 56, which is also 8-aligned, and Parse would correctly say 8: it picks the largest.)
			{Kind: "RED2", Data: []byte("hello world"), Align: 16},
			{Kind: "RERL", Data: []byte("abcdefg"), Align: 4},
		},
		Trailer: []byte("PIXELS"),
	}
	got, err := Parse(want.Serialize())
	if err != nil {
		t.Fatal(err)
	}
	if got.HeaderVersion != 12 || len(got.Blocks) != 2 || string(got.Trailer) != "PIXELS" {
		t.Fatalf("got %+v", got)
	}
	for i, b := range got.Blocks {
		w := want.Blocks[i]
		if b.Kind != w.Kind || !bytes.Equal(b.Data, w.Data) || b.Align != w.Align {
			t.Errorf("block %d: got %s/%q/align %d, want %s/%q/align %d",
				i, b.Kind, b.Data, b.Align, w.Kind, w.Data, w.Align)
		}
	}
	if i, err := got.BlockIndex("RERL", 0); err != nil || i != 1 {
		t.Errorf("BlockIndex(RERL, 0) = %d, %v; want 1, nil", i, err)
	}
	if _, err := got.BlockIndex("MVTX", 0); err == nil {
		t.Error("BlockIndex(MVTX, 0) should return an error")
	}
}

// Level 3: garbage input must return an error, never panic.
func TestParseRejectsGarbage(t *testing.T) {
	valid := (&Resource{Blocks: []Block{{Kind: "DATA", Data: []byte("0123456789")}}}).Serialize()
	if len(valid) < 32 {
		t.Fatal("implement Serialize first (this test uses it to build its inputs)")
	}
	hugeCount := bytes.Clone(valid)
	binary.LittleEndian.PutUint32(hugeCount[12:], 1_000_000)
	badOffset := bytes.Clone(valid)
	binary.LittleEndian.PutUint32(badOffset[16+4:], 0xFFFF_FF00)

	cases := map[string][]byte{
		"empty":            {},
		"short header":     make([]byte, 10),
		"huge block count": hugeCount,
		"offset past end":  badOffset,
		"truncated block":  valid[:len(valid)-3],
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("Parse panicked: %v", p)
				}
			}()
			if _, err := Parse(data); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

// Level 4: the real game files. Parse + Serialize must reproduce them byte for byte.
func TestRoundtripGameFiles(t *testing.T) {
	for _, path := range templateFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			r, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			out := r.Serialize()
			if !bytes.Equal(out, data) {
				i := 0
				for i < len(out) && i < len(data) && out[i] == data[i] {
					i++
				}
				t.Fatalf("output differs from the original at byte %d (len got %d, want %d)",
					i, len(out), len(data))
			}
		})
	}
}

// Level 5: the parsed structure must match what the Python reference sees (testdata golden file,
// regenerate with `make golden`).
func TestMatchesPythonReference(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "resource_layout.json"))
	if err != nil {
		t.Skip("no golden file: run `make golden`")
	}
	var golden map[string]struct {
		HeaderVersion uint16 `json:"header_version"`
		Version       uint16 `json:"version"`
		Blocks        []struct {
			Kind  string `json:"kind"`
			Size  int    `json:"size"`
			Align int    `json:"align"`
		} `json:"blocks"`
		TrailerSize int `json:"trailer_size"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	for _, path := range templateFiles(t) {
		want, ok := golden[filepath.Base(path)]
		if !ok {
			continue
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, _ := os.ReadFile(path)
			r, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			if r.HeaderVersion != want.HeaderVersion || r.Version != want.Version {
				t.Errorf("versions: got %d/%d, want %d/%d",
					r.HeaderVersion, r.Version, want.HeaderVersion, want.Version)
			}
			if len(r.Blocks) != len(want.Blocks) {
				t.Fatalf("got %d blocks, want %d", len(r.Blocks), len(want.Blocks))
			}
			for i, b := range r.Blocks {
				w := want.Blocks[i]
				if b.Kind != w.Kind || len(b.Data) != w.Size || b.Align != w.Align {
					t.Errorf("block %d: got %s size %d align %d, want %s size %d align %d",
						i, b.Kind, len(b.Data), b.Align, w.Kind, w.Size, w.Align)
				}
			}
			if len(r.Trailer) != want.TrailerSize {
				t.Errorf("trailer: got %d bytes, want %d", len(r.Trailer), want.TrailerSize)
			}
		})
	}
}
