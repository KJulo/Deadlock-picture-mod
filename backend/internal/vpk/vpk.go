// Package vpk writes Valve Pak (VPK v2) archives: the .vpk file that Deadlock Mod Manager imports.
//
// It produces the same layout as the python `vpk` library used by reference/catmod/build.py
// (everything embedded in the _dir file, no separate _000.vpk archives), but with a deterministic
// file order so the output only depends on the input.
//
// Layout (little-endian):
//
//	header (28 B)  u32 signature 0x55aa1234 | u32 version 2 | u32 tree size
//	               u32 file data size | u32 chunk hashes size (0) | u32 self hashes size (48)
//	               u32 signature size (0)
//	tree           for each extension:  "ext\0"
//	                 for each directory:  "dir\0"          (" " = root directory)
//	                   for each file:       "name\0" + 18-byte entry
//	                   "\0"                 (end of files in this directory)
//	                 "\0"                 (end of directories for this extension)
//	               "\0"                 (end of tree)
//	data           the file contents, back to back
//	hashes (48 B)  md5(tree) | md5(chunk hashes = empty) | md5(everything before + the two above)
//
// 18-byte entry: u32 crc32 | u16 preload size (0) | u16 archive index (0x7fff = "in this file")
//
//	| u32 offset (from start of data) | u32 size | u16 terminator 0xffff
package vpk

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"path"
	"sort"
	"strings"
)

const (
	signature    = 0x55aa1234
	version      = 2
	headerSize   = 28
	embedded     = 0x7fff // archive index meaning "data is inside the _dir file"
	terminator   = 0xffff
	selfHashSize = 3 * md5.Size
)

// split turns "models/a/b.vmdl_c" into ("vmdl_c", "models/a", "b").
func split(p string) (ext, dir, name string, err error) {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || strings.Contains(p, "\x00") {
		return "", "", "", fmt.Errorf("vpk: invalid path %q", p)
	}
	dir, file := path.Split(p)
	dir = strings.TrimSuffix(dir, "/")
	if dir == "" {
		dir = " " // how VPK spells the root directory
	}
	dot := strings.LastIndexByte(file, '.')
	if dot <= 0 || dot == len(file)-1 {
		return "", "", "", fmt.Errorf("vpk: %q needs a name and an extension", p)
	}
	return file[dot+1:], dir, file[:dot], nil
}

// Build returns a VPK containing files (path inside the game -> contents). Paths use "/".
func Build(files map[string][]byte) ([]byte, error) {
	// Group as extension -> directory -> names, the order the tree is written in.
	type entry struct{ ext, dir, name, path string }
	entries := make([]entry, 0, len(files))
	for p := range files {
		ext, dir, name, err := split(p)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry{ext, dir, name, p})
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.ext != b.ext {
			return a.ext < b.ext
		}
		if a.dir != b.dir {
			return a.dir < b.dir
		}
		return a.name < b.name
	})

	// Tree and data are built in separate buffers, then joined.
	var tree, data bytes.Buffer
	le := binary.LittleEndian
	cstr := func(s string) { tree.WriteString(s); tree.WriteByte(0) }
	for i, e := range entries {
		newExt := i == 0 || e.ext != entries[i-1].ext
		newDir := newExt || e.dir != entries[i-1].dir
		if i > 0 && newDir {
			tree.WriteByte(0) // close previous directory
		}
		if i > 0 && newExt {
			tree.WriteByte(0) // close previous extension
		}
		if newExt {
			cstr(e.ext)
		}
		if newDir {
			cstr(e.dir)
		}
		cstr(e.name)

		content := files[e.path]
		var meta [18]byte
		le.PutUint32(meta[0:], crc32.ChecksumIEEE(content))
		le.PutUint16(meta[4:], 0)
		le.PutUint16(meta[6:], embedded)
		le.PutUint32(meta[8:], uint32(data.Len()))
		le.PutUint32(meta[12:], uint32(len(content)))
		le.PutUint16(meta[16:], terminator)
		tree.Write(meta[:])
		data.Write(content)
	}
	if len(entries) > 0 {
		tree.Write([]byte{0, 0}) // close last directory and extension
	}
	tree.WriteByte(0) // end of tree

	if uint64(tree.Len())+uint64(data.Len()) > 0xFFFF_FFFF {
		return nil, fmt.Errorf("vpk: archive too large (%d bytes)", tree.Len()+data.Len())
	}

	out := make([]byte, headerSize, headerSize+tree.Len()+data.Len()+selfHashSize)
	le.PutUint32(out[0:], signature)
	le.PutUint32(out[4:], version)
	le.PutUint32(out[8:], uint32(tree.Len()))
	le.PutUint32(out[12:], uint32(data.Len()))
	le.PutUint32(out[16:], 0) // chunk hashes size
	le.PutUint32(out[20:], selfHashSize)
	le.PutUint32(out[24:], 0) // signature size
	out = append(out, tree.Bytes()...)
	out = append(out, data.Bytes()...)

	treeSum := md5.Sum(tree.Bytes())
	chunkSum := md5.Sum(nil) // no chunk hashes
	whole := md5.New()
	whole.Write(out)
	whole.Write(treeSum[:])
	whole.Write(chunkSum[:])
	out = append(out, treeSum[:]...)
	out = append(out, chunkSum[:]...)
	return whole.Sum(out), nil
}
