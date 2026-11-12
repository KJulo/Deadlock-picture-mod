// Package meshopt implements the meshoptimizer vertex (codec v0) and index (codec v1) encoders,
// the compression Source 2 uses for model geometry (MVTX / MIDX blocks).
//
// Port of meshoptimizer src/vertexcodec.cpp and src/indexcodec.cpp (MIT, Arseny Kapoulkine): v0.22,
// plus the newer equal-size tie-break in encodeBytes.
// The output is byte-identical to the C library (checked against the python binding in tests).
//
// Why vertex codec v0 and not v1: the game's own buffers are v1, but v0 is decoded identically by
// every meshopt version, and it is what we verified in game.
package meshopt

import (
	"errors"
	"fmt"
)

// ---------------------------------------------------------------------------------------------
// Vertex codec v0
//
// Vertices are processed in blocks of up to 256. Inside a block, each byte position k of the
// vertex (0..vertexSize-1) becomes its own stream: the delta from the same byte of the previous
// vertex, zigzag-encoded so small negative and positive changes are both small numbers. Each stream
// is cut into groups of 16 bytes, and each group is stored with 0, 2, 4 or 8 bits per byte
// (whichever is smallest); values that don't fit are escaped and written as full bytes.
// ---------------------------------------------------------------------------------------------

const (
	vertexHeader                = 0xa0 // | version
	vertexBlockSizeBytes        = 8192
	vertexBlockMaxSize          = 256
	byteGroupSize               = 16
	tailMaxSize                 = 32
	maxVertexSize               = 256
	indexHeader                 = 0xe0 // | version
	indexCodecVersion           = 1
	vertexFifoSize              = 16
	edgeFifoSize                = 16
	fecMax                      = 13 // version >= 1
	invalidIndex         uint32 = 0xFFFFFFFF
)

func vertexBlockSize(vertexSize int) int {
	n := vertexBlockSizeBytes / vertexSize
	n &^= byteGroupSize - 1
	return min(n, vertexBlockMaxSize)
}

func zigzag8(v byte) byte { return byte(int8(v)>>7) ^ (v << 1) }

// groupSize returns how many bytes a 16-byte group takes with the given bits per value, or -1 if
// the encoding is impossible (bits == 1 means "all zero", stored as 0 bytes).
func groupSize(group []byte, bits int) int {
	switch bits {
	case 1:
		for _, b := range group {
			if b != 0 {
				return -1
			}
		}
		return 0
	case 8:
		return byteGroupSize
	}
	size := byteGroupSize * bits / 8
	sentinel := byte(1<<bits - 1)
	for _, b := range group {
		if b >= sentinel {
			size++ // escaped: written again as a full byte
		}
	}
	return size
}

func encodeGroup(out []byte, group []byte, bits int) []byte {
	switch bits {
	case 1:
		return out
	case 8:
		return append(out, group...)
	}
	perByte := 8 / bits
	sentinel := byte(1<<bits - 1)
	for i := 0; i < byteGroupSize; i += perByte {
		var packed byte
		for k := 0; k < perByte; k++ {
			packed <<= bits
			packed |= min(group[i+k], sentinel) // first value ends up in the high bits
		}
		out = append(out, packed)
	}
	for _, b := range group {
		if b >= sentinel {
			out = append(out, b)
		}
	}
	return out
}

// encodeBytes writes one stream (a multiple of 16 bytes): a 2-bit header per group, then groups.
func encodeBytes(out []byte, stream []byte) []byte {
	groups := len(stream) / byteGroupSize
	headerAt := len(out)
	out = append(out, make([]byte, (groups+3)/4)...)
	lastBits := -1
	for g := 0; g < groups; g++ {
		group := stream[g*byteGroupSize : (g+1)*byteGroupSize]
		bestBits, bestSize := 8, groupSize(group, 8)
		for bits := 1; bits < 8; bits *= 2 { // 1 (= zero), 2, 4
			size := groupSize(group, bits)
			if size < 0 {
				continue
			}
			// On a tie, keep the previous group's width (compresses better downstream), but never
			// replace literals. This tie-break was added after v0.22; the python binding has it.
			if size < bestSize || (size == bestSize && bits == lastBits && bestBits != 8) {
				bestBits, bestSize = bits, size
			}
		}
		bitsLog2 := map[int]byte{1: 0, 2: 1, 4: 2, 8: 3}[bestBits]
		out[headerAt+g/4] |= bitsLog2 << ((g % 4) * 2)
		out = encodeGroup(out, group, bestBits)
		lastBits = bestBits
	}
	return out
}

func encodeVertexBlock(out, vertices []byte, count, vertexSize int, last []byte) []byte {
	aligned := (count + byteGroupSize - 1) &^ (byteGroupSize - 1)
	stream := make([]byte, aligned) // tail beyond count stays zero
	for k := 0; k < vertexSize; k++ {
		p := last[k]
		for i := 0; i < count; i++ {
			v := vertices[i*vertexSize+k]
			stream[i] = zigzag8(v - p)
			p = v
		}
		out = encodeBytes(out, stream)
	}
	copy(last, vertices[(count-1)*vertexSize:count*vertexSize])
	return out
}

// EncodeVertexBuffer compresses len(vertices)/vertexSize vertices with vertex codec v0.
func EncodeVertexBuffer(vertices []byte, vertexSize int) ([]byte, error) {
	if vertexSize <= 0 || vertexSize > maxVertexSize || vertexSize%4 != 0 {
		return nil, fmt.Errorf("meshopt: invalid vertex size %d", vertexSize)
	}
	if len(vertices)%vertexSize != 0 {
		return nil, fmt.Errorf("meshopt: %d bytes is not a multiple of vertex size %d", len(vertices), vertexSize)
	}
	count := len(vertices) / vertexSize
	out := []byte{vertexHeader | 0}

	first := make([]byte, vertexSize)
	copy(first, vertices) // zeros if there are no vertices
	last := append([]byte(nil), first...)

	block := vertexBlockSize(vertexSize)
	for off := 0; off < count; off += block {
		n := min(block, count-off)
		out = encodeVertexBlock(out, vertices[off*vertexSize:], n, vertexSize, last)
	}

	// The first vertex goes at the end, padded to 32 bytes (simplifies bounds checks in the decoder).
	if vertexSize < tailMaxSize {
		out = append(out, make([]byte, tailMaxSize-vertexSize)...)
	}
	return append(out, first...), nil
}

// ---------------------------------------------------------------------------------------------
// Index codec v1
//
// Triangles are encoded one by one, remembering the last 16 edges and the last 16 vertices
// (FIFOs). A triangle sharing an edge with a recent one costs 1 byte (edge position + where the
// third vertex comes from); otherwise a code byte plus an optional "codeaux" byte. Vertices that are
// new and sequential ("next") are free; anything else is written as a varint delta from the last
// explicitly written index.
// ---------------------------------------------------------------------------------------------

var triangleIndexOrder = [3][3]int{{0, 1, 2}, {1, 2, 0}, {2, 0, 1}}

var codeAuxTable = [16]byte{
	0x00, 0x76, 0x87, 0x56, 0x67, 0x78, 0xa9, 0x86, 0x65, 0x89, 0x68, 0x98, 0x01, 0x69,
	0, 0, // the last two entries are not used for encoding
}

type indexEncoder struct {
	edges      [edgeFifoSize][2]uint32
	edgeOff    int
	vertices   [vertexFifoSize]uint32
	vertexOff  int
	next, last uint32
	data       []byte
}

func (e *indexEncoder) findEdge(a, b, c uint32) int {
	for i := 0; i < edgeFifoSize; i++ {
		ed := e.edges[(e.edgeOff-1-i)&(edgeFifoSize-1)]
		switch {
		case ed[0] == a && ed[1] == b:
			return i<<2 | 0
		case ed[0] == b && ed[1] == c:
			return i<<2 | 1
		case ed[0] == c && ed[1] == a:
			return i<<2 | 2
		}
	}
	return -1
}

func (e *indexEncoder) pushEdge(a, b uint32) {
	e.edges[e.edgeOff] = [2]uint32{a, b}
	e.edgeOff = (e.edgeOff + 1) & (edgeFifoSize - 1)
}

func (e *indexEncoder) findVertex(v uint32) int {
	for i := 0; i < vertexFifoSize; i++ {
		if e.vertices[(e.vertexOff-1-i)&(vertexFifoSize-1)] == v {
			return i
		}
	}
	return -1
}

func (e *indexEncoder) pushVertex(v uint32) {
	e.vertices[e.vertexOff] = v
	e.vertexOff = (e.vertexOff + 1) & (vertexFifoSize - 1)
}

func (e *indexEncoder) resetVertices() {
	for i := range e.vertices {
		e.vertices[i] = invalidIndex
	}
}

// writeIndex appends index as a zigzag varint delta from the last written index.
func (e *indexEncoder) writeIndex(index uint32) {
	d := index - e.last
	v := d<<1 ^ uint32(int32(d)>>31)
	for {
		b := byte(v & 127)
		if v > 127 {
			b |= 128
		}
		e.data = append(e.data, b)
		v >>= 7
		if v == 0 {
			break
		}
	}
	e.last = index
}

// orNext returns 0 and advances next if v is the next new vertex, otherwise 15 ("free index").
func (e *indexEncoder) orNext(v uint32) int {
	if v == e.next {
		e.next++
		return 0
	}
	return 15
}

// EncodeIndexBuffer compresses a triangle list (len(indices) % 3 == 0) with index codec v1.
func EncodeIndexBuffer(indices []uint32) ([]byte, error) {
	if len(indices)%3 != 0 {
		return nil, errors.New("meshopt: index count must be a multiple of 3")
	}
	e := &indexEncoder{}
	for i := range e.edges {
		e.edges[i] = [2]uint32{invalidIndex, invalidIndex}
	}
	e.resetVertices()

	codes := make([]byte, 0, len(indices)/3)
	for i := 0; i < len(indices); i += 3 {
		tri := indices[i : i+3]
		if fer := e.findEdge(tri[0], tri[1], tri[2]); fer >= 0 && fer>>2 < 15 {
			// Shares an edge with a recent triangle: rotate so (a, b) is that edge.
			o := triangleIndexOrder[fer&3]
			a, b, c := tri[o[0]], tri[o[1]], tri[o[2]]
			fe := fer >> 2
			fec := 15
			if fc := e.findVertex(c); fc >= 1 && fc < fecMax {
				fec = fc
			} else {
				fec = e.orNext(c)
			}
			if fec == 15 { // strip-like sequences: last-1 / last+1
				if c+1 == e.last {
					fec, e.last = 13, c
				}
				if c == e.last+1 {
					fec, e.last = 14, c
				}
			}
			codes = append(codes, byte(fe<<4|fec))
			if fec == 15 {
				e.writeIndex(c)
			}
			if fec == 0 || fec >= fecMax {
				e.pushVertex(c)
			}
			e.pushEdge(c, b)
			e.pushEdge(a, c)
			continue
		}

		// No shared edge: rotate so that a is the next new vertex if possible.
		rotation := 0
		if tri[1] == e.next {
			rotation = 1
		} else if tri[2] == e.next {
			rotation = 2
		}
		o := triangleIndexOrder[rotation]
		a, b, c := tri[o[0]], tri[o[1]], tri[o[2]]

		reset := false
		if a == 0 && b == 1 && c == 2 && e.next > 0 {
			reset = true
			e.next = 0
			e.resetVertices()
		}

		fb, fc := e.findVertex(b), e.findVertex(c)
		fea := e.orNext(a)
		feb := 15
		if fb >= 0 && fb < 14 {
			feb = fb + 1
		} else {
			feb = e.orNext(b)
		}
		fec := 15
		if fc >= 0 && fc < 14 {
			fec = fc + 1
		} else {
			fec = e.orNext(c)
		}

		codeAux := byte(feb<<4 | fec)
		auxIndex := -1
		for k, v := range codeAuxTable {
			if v == codeAux {
				auxIndex = k
				break
			}
		}
		if fea == 0 && auxIndex >= 0 && auxIndex < 14 && !reset {
			codes = append(codes, byte(15<<4|auxIndex))
		} else {
			codes = append(codes, byte(15<<4|14|fea))
			e.data = append(e.data, codeAux)
		}

		if fea == 15 {
			e.writeIndex(a)
		}
		if feb == 15 {
			e.writeIndex(b)
		}
		if fec == 15 {
			e.writeIndex(c)
		}
		if fea == 0 || fea == 15 {
			e.pushVertex(a)
		}
		if feb == 0 || feb == 15 {
			e.pushVertex(b)
		}
		if fec == 0 || fec == 15 {
			e.pushVertex(c)
		}
		e.pushEdge(b, a)
		e.pushEdge(c, b)
		e.pushEdge(a, c)
	}

	out := make([]byte, 0, 1+len(codes)+len(e.data)+len(codeAuxTable))
	out = append(out, indexHeader|indexCodecVersion)
	out = append(out, codes...)
	out = append(out, e.data...)
	// The table doubles as padding so the decoder can read 16 bytes past any triangle.
	return append(out, codeAuxTable[:]...), nil
}
