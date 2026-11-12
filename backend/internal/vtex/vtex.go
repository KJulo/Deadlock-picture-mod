// Package vtex rebuilds the soul container's vtex_c textures: the original resource header is
// kept and only the pixel data is replaced (port of reference/catmod/vtex.py).
//
// A vtex_c is a resource (RED2 + DATA blocks) followed by the mip chain, smallest mip first,
// uncompressed. Supported formats: BC7 (20) and ATI1N/BC4 (27).
package vtex

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/bc7"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/resource"
)

// Texture formats (byte 26 of the DATA block).
const (
	FormatBC7 = 20
	FormatBC4 = 27
)

// Info is what the DATA block says about the texture.
type Info struct {
	Width, Height int
	Format        int
	Mips          int
}

// ReadInfo parses the texture header. DATA layout: u16 version, u16 flags, f32 reflectivity[4],
// u16 width (offset 20), u16 height, u16 depth, u8 format (26), u8 mip count (27).
func ReadInfo(data []byte) (Info, *resource.Resource, error) {
	res, err := resource.Parse(data)
	if err != nil {
		return Info{}, nil, err
	}
	i, err := res.BlockIndex("DATA", 0)
	if err != nil {
		return Info{}, nil, err
	}
	d := res.Blocks[i].Data
	if len(d) < 28 {
		return Info{}, nil, fmt.Errorf("vtex: DATA block too short")
	}
	le := binary.LittleEndian
	return Info{int(le.Uint16(d[20:])), int(le.Uint16(d[22:])), int(d[26]), int(d[27])}, res, nil
}

// DecodeTop returns the largest mip of a BC7 texture as an image (e.g. to paint over the original).
func DecodeTop(data []byte) (*image.NRGBA, error) {
	info, res, err := ReadInfo(data)
	if err != nil {
		return nil, err
	}
	if info.Format != FormatBC7 {
		return nil, fmt.Errorf("vtex: format %d is not BC7", info.Format)
	}
	n := (info.Width / 4) * (info.Height / 4) * bc7.BlockSize
	if len(res.Trailer) < n {
		return nil, fmt.Errorf("vtex: pixel data too short")
	}
	pix, err := bc7.Decode(res.Trailer[len(res.Trailer)-n:], info.Width, info.Height) // largest mip is last
	if err != nil {
		return nil, err
	}
	return &image.NRGBA{Pix: pix, Stride: info.Width * 4, Rect: image.Rect(0, 0, info.Width, info.Height)}, nil
}

// mipSizes lists the size of every mip, largest first (never below 4x4: one block).
func mipSizes(info Info) [][2]int {
	sizes := make([][2]int, info.Mips)
	for i := range sizes {
		sizes[i] = [2]int{max(info.Width>>i, 4), max(info.Height>>i, 4)}
	}
	return sizes
}

// halve averages 2x2 pixel squares (box filter): the classic way to build the next mip.
func halve(src []byte, w, h int) ([]byte, int, int) {
	nw, nh := max(w/2, 1), max(h/2, 1)
	dst := make([]byte, nw*nh*4)
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			for c := 0; c < 4; c++ {
				sum := 0
				for dy := 0; dy < 2; dy++ {
					for dx := 0; dx < 2; dx++ {
						sx, sy := min(2*x+dx, w-1), min(2*y+dy, h-1)
						sum += int(src[(sy*w+sx)*4+c])
					}
				}
				dst[(y*nw+x)*4+c] = byte((sum + 2) / 4)
			}
		}
	}
	return dst, nw, nh
}

// replace swaps the template's pixel data for chain (largest mip first) and serializes.
func replace(res *resource.Resource, chain [][]byte) ([]byte, error) {
	var pixels bytes.Buffer
	for i := len(chain) - 1; i >= 0; i-- { // stored smallest mip first
		pixels.Write(chain[i])
	}
	if pixels.Len() != len(res.Trailer) {
		return nil, fmt.Errorf("vtex: pixel data is %d bytes, original %d", pixels.Len(), len(res.Trailer))
	}
	res.Trailer = pixels.Bytes()
	return res.Serialize(), nil
}

// RebuildBC7 encodes img (must match the template's size) and all its mips as BC7.
func RebuildBC7(template []byte, img *image.NRGBA, mode bc7.Mode) ([]byte, error) {
	info, res, err := ReadInfo(template)
	if err != nil {
		return nil, err
	}
	if info.Format != FormatBC7 {
		return nil, fmt.Errorf("vtex: template format %d is not BC7", info.Format)
	}
	if b := img.Bounds(); b.Dx() != info.Width || b.Dy() != info.Height {
		return nil, fmt.Errorf("vtex: image %dx%d, texture is %dx%d", b.Dx(), b.Dy(), info.Width, info.Height)
	}
	pix, w, h := img.Pix, info.Width, info.Height
	var chain [][]byte
	for _, size := range mipSizes(info) {
		for w > size[0] || h > size[1] {
			pix, w, h = halve(pix, w, h)
		}
		blocks, err := bc7.Encode(pix, w, h, mode)
		if err != nil {
			return nil, err
		}
		chain = append(chain, blocks)
	}
	return replace(res, chain)
}

// RebuildBC7Constant fills the texture with one colour. Every 4x4 block is identical, so one block
// is encoded and repeated (same bytes as encoding the full image, thousands of times faster).
func RebuildBC7Constant(template []byte, rgba [4]byte, mode bc7.Mode) ([]byte, error) {
	info, res, err := ReadInfo(template)
	if err != nil {
		return nil, err
	}
	if info.Format != FormatBC7 {
		return nil, fmt.Errorf("vtex: template format %d is not BC7", info.Format)
	}
	block, err := bc7.Encode(bytes.Repeat(rgba[:], 16), 4, 4, mode)
	if err != nil {
		return nil, err
	}
	var chain [][]byte
	for _, s := range mipSizes(info) {
		chain = append(chain, bytes.Repeat(block, (s[0]/4)*(s[1]/4)))
	}
	return replace(res, chain)
}

// RebuildBC4Constant fills a single-channel BC4 texture with value (e.g. 255 = no occlusion).
func RebuildBC4Constant(template []byte, value byte) ([]byte, error) {
	info, res, err := ReadInfo(template)
	if err != nil {
		return nil, err
	}
	if info.Format != FormatBC4 {
		return nil, fmt.Errorf("vtex: template format %d is not BC4", info.Format)
	}
	// BC4 block: two endpoints + 16 3-bit indices. Both endpoints = value -> every pixel = value.
	block := []byte{value, value, 0, 0, 0, 0, 0, 0}
	var chain [][]byte
	for _, s := range mipSizes(info) {
		chain = append(chain, bytes.Repeat(block, (s[0]/4)*(s[1]/4)))
	}
	return replace(res, chain)
}
