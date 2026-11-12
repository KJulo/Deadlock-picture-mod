package vpk

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

const testdata = "../../../testdata"

// read is a minimal VPK v2 reader used only to check what Build writes.
func read(t *testing.T, vpk []byte) map[string][]byte {
	t.Helper()
	le := binary.LittleEndian
	if le.Uint32(vpk[0:]) != signature || le.Uint32(vpk[4:]) != version {
		t.Fatal("bad signature/version")
	}
	treeSize := int(le.Uint32(vpk[8:]))
	dataSize := int(le.Uint32(vpk[12:]))
	if want := headerSize + treeSize + dataSize + selfHashSize; len(vpk) != want {
		t.Fatalf("length %d, header says %d", len(vpk), want)
	}
	tree := vpk[headerSize : headerSize+treeSize]
	data := vpk[headerSize+treeSize : headerSize+treeSize+dataSize]

	// self hashes
	hashes := vpk[len(vpk)-selfHashSize:]
	if ts := md5.Sum(tree); !bytes.Equal(hashes[:16], ts[:]) {
		t.Error("tree md5 mismatch")
	}
	if fs := md5.Sum(vpk[:len(vpk)-16]); !bytes.Equal(hashes[32:], fs[:]) {
		t.Error("file md5 mismatch")
	}

	pos := 0
	cstr := func() string {
		end := bytes.IndexByte(tree[pos:], 0)
		s := string(tree[pos : pos+end])
		pos += end + 1
		return s
	}
	files := map[string][]byte{}
	for ext := cstr(); ext != ""; ext = cstr() {
		for dir := cstr(); dir != ""; dir = cstr() {
			for name := cstr(); name != ""; name = cstr() {
				m := tree[pos : pos+18]
				pos += 18
				if le.Uint16(m[6:]) != embedded || le.Uint16(m[16:]) != terminator {
					t.Fatalf("%s: bad entry %x", name, m)
				}
				off, size := le.Uint32(m[8:]), le.Uint32(m[12:])
				content := data[off : off+size]
				if crc32.ChecksumIEEE(content) != le.Uint32(m[0:]) {
					t.Errorf("%s: crc mismatch", name)
				}
				p := name + "." + ext
				if dir != " " {
					p = dir + "/" + p
				}
				files[p] = content
			}
		}
	}
	if pos != len(tree) {
		t.Errorf("tree has %d trailing bytes", len(tree)-pos)
	}
	return files
}

func loadSynthetic(t *testing.T) map[string][]byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(testdata, "vpk_synthetic_files.json"))
	if err != nil {
		t.Fatal(err)
	}
	var enc map[string]string
	if err := json.Unmarshal(raw, &enc); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for p, b64 := range enc {
		files[p], _ = base64.StdEncoding.DecodeString(b64)
	}
	return files
}

// The files we put in come back out, with valid CRCs and MD5s.
func TestRoundtrip(t *testing.T) {
	files := loadSynthetic(t)
	out, err := Build(files)
	if err != nil {
		t.Fatal(err)
	}
	got := read(t, out)
	if len(got) != len(files) {
		t.Fatalf("got %d files, want %d", len(got), len(files))
	}
	for p, want := range files {
		if !bytes.Equal(got[p], want) {
			t.Errorf("%s: content differs", p)
		}
	}
}

// Byte-for-byte identical to what the python `vpk` library writes (the VPK format the game and
// DMM already accepted).
func TestMatchesPythonReference(t *testing.T) {
	want, err := os.ReadFile(filepath.Join(testdata, "vpk_synthetic.vpk"))
	if err != nil {
		t.Skip("no golden file: run `make golden`")
	}
	got, err := Build(loadSynthetic(t))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		i := 0
		for i < len(got) && i < len(want) && got[i] == want[i] {
			i++
		}
		t.Fatalf("differs from python at byte %d (len got %d, want %d)", i, len(got), len(want))
	}
}

func TestEmpty(t *testing.T) {
	out, err := Build(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, out); len(got) != 0 {
		t.Errorf("got %d files", len(got))
	}
}

func TestRejectsBadPaths(t *testing.T) {
	for _, p := range []string{"", "noext", "dir/noext", "/abs.txt", `win\path.txt`, "dir/.hidden", "x."} {
		if _, err := Build(map[string][]byte{p: nil}); err == nil {
			t.Errorf("%q: expected an error", p)
		}
	}
}
