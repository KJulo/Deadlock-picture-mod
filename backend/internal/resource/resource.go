// Package resource reads and writes the Source 2 compiled-resource container: the "*_c" files
// such as soul_container.vmdl_c (model) or ..._color_png_1da67bfe.vtex_c (texture).
//
// It is a Go port of reference/catmod/resource.py, the source of truth. Read
// docs/lessons/01-resource.md before starting.
//
// File layout (all integers are little-endian):
//
//	offset 0   u32  file size          (covers header + table + blocks, NOT the trailer)
//	offset 4   u16  header version
//	offset 6   u16  version
//	offset 8   u32  block table offset (relative to offset 8 itself, so it is usually 8 -> table at 16)
//	offset 12  u32  block count
//	table      count × 12-byte entries: char[4] kind | u32 offset | u32 size
//	           entry offset is relative to the position of that offset field (entry start + 4)
//	blocks     the block payloads, each aligned (usually to 16; RERL in vmdl_c is aligned to 4)
//	trailer    whatever comes after the last block (texture mips in vtex_c)
package resource

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// DefaultAlign is used for blocks whose Align is 0 (e.g. blocks you create yourself).
const DefaultAlign = 16

// Block is one entry of the block table together with its payload.
type Block struct {
	Kind  string // always 4 ASCII characters: "RED2", "DATA", "MVTX", "MIDX", ...
	Data  []byte
	Align int // 16, 8, 4, 2 or 1; inferred by Parse so Serialize reproduces the original file
}

// Resource is a parsed *_c file.
type Resource struct {
	HeaderVersion uint16
	Version       uint16
	Blocks        []Block
	Trailer       []byte
}

// ErrTruncated is returned when the data is too short for what the header or table claims.
var ErrTruncated = errors.New("resource: truncated or corrupt data")

// Parse reads a *_c file.
//
// Unlike Python, Go panics on an out-of-range slice. Data uploaded by users (or a changed game
// file) can be garbage, so every read must be bounds-checked and return ErrTruncated instead of
// panicking. TestParseRejectsGarbage checks this.
func Parse(data []byte) (*Resource, error) {
	if len(data) < 16 {
		return nil, ErrTruncated
	}
	le := binary.LittleEndian
	table := uint64(8) + uint64(le.Uint32(data[8:12]))
	count := uint64(le.Uint32(data[12:16]))
	length := uint64(len(data))
	if table > length || count > (length-table)/12 {
		return nil, ErrTruncated
	}
	end := table + count*12
	prevEnd := end
	r := &Resource{
		HeaderVersion: le.Uint16(data[4:6]),
		Version:       le.Uint16(data[6:8]),
		Blocks:        make([]Block, int(count)),
	}
	for i := range r.Blocks {
		entry := table + uint64(i)*12
		kind := data[entry : entry+4]
		for _, c := range kind {
			if c >= 0x80 {
				return nil, ErrTruncated
			}
		}
		start := entry + 4 + uint64(le.Uint32(data[entry+4:entry+8]))
		size := uint64(le.Uint32(data[entry+8 : entry+12]))
		if start > length || size > length-start {
			return nil, ErrTruncated
		}
		alignment := 0
		for _, a := range [...]uint64{16, 8, 4, 2, 1} {
			if (prevEnd+a-1)&^(a-1) == start {
				alignment = int(a)
				break
			}
		}
		if alignment == 0 {
			return nil, ErrTruncated
		}
		prevEnd = start + size
		if prevEnd > end {
			end = prevEnd
		}
		r.Blocks[i] = Block{Kind: string(kind), Data: data[start:prevEnd], Align: alignment}
	}
	r.Trailer = data[end:]
	return r, nil
}

// Serialize writes the resource back. For a resource returned by Parse and left unmodified, the
// output must be byte-for-byte identical to the input.
func (r *Resource) Serialize() []byte {
	le := binary.LittleEndian
	const table = 16 // the header is 16 bytes and the block table follows it

	// 1) Layout: where each block starts. Pad up to the block's alignment, then place the payload.
	starts := make([]int, len(r.Blocks))
	pos := table + len(r.Blocks)*12
	for i, b := range r.Blocks {
		align := b.Align
		if align == 0 {
			align = DefaultAlign
		}
		pos = alignUp(pos, align)
		starts[i] = pos
		pos += len(b.Data)
	}

	// 2) Allocate once (zeros = padding), with spare capacity for the trailer.
	out := make([]byte, pos, pos+len(r.Trailer))

	// 3) Header. The size field covers header + table + blocks, not the trailer.
	le.PutUint32(out[0:], uint32(pos))
	le.PutUint16(out[4:], r.HeaderVersion)
	le.PutUint16(out[6:], r.Version)
	le.PutUint32(out[8:], table-8) // relative to offset 8
	le.PutUint32(out[12:], uint32(len(r.Blocks)))

	// 4) Table entries and payloads. Offsets are relative to the offset field itself (entry+4).
	for i, b := range r.Blocks {
		entry := table + i*12
		copy(out[entry:entry+4], b.Kind)
		le.PutUint32(out[entry+4:], uint32(starts[i]-(entry+4)))
		le.PutUint32(out[entry+8:], uint32(len(b.Data)))
		copy(out[starts[i]:], b.Data)
	}
	return append(out, r.Trailer...)
}

// BlockIndex returns the index of the nth block (0-based) with the given kind.
func (r *Resource) BlockIndex(kind string, nth int) (int, error) {
	seen := 0
	for i, b := range r.Blocks {
		if b.Kind != kind {
			continue
		}
		if seen == nth {
			return i, nil
		}
		seen++
	}
	return -1, fmt.Errorf("resource: block %s #%d not found (%d present)", kind, nth, seen)
}

// alignUp rounds n up to a multiple of a (a > 0). Example: alignUp(17, 16) == 32.
func alignUp(n, a int) int {
	return (n + a - 1) / a * a
}
