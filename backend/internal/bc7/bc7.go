// Package bc7 compresses RGBA images to BC7, the GPU texture format of the soul container's
// colour and normal/roughness textures.
//
// It wraps Rich Geldreich's bc7enc (C, MIT or public domain, vendored: bc7enc.c, LICENSE-bc7enc)
// through cgo, which lets Go call C functions directly. The image is cut into horizontal strips of
// 4-pixel-high block rows and each strip is encoded on its own goroutine, so all CPU cores work
// at once. bc7decomp (C++) is included to decode blocks back for tests and quality checks.
package bc7

/*
#cgo CFLAGS: -O2
#cgo CXXFLAGS: -O2
#include "glue.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// BlockSize is the size of one compressed 4x4 block.
const BlockSize = 16

var initOnce sync.Once

// Mode picks the error metric the encoder minimises.
type Mode int

const (
	Perceptual Mode = iota // photos: weights luma more, like the eye does
	Linear                 // data (normals, roughness): every channel counts the same
)

// Encode compresses an RGBA8 image (4 bytes per pixel, rows top to bottom) whose width and
// height are multiples of 4. The result has (w/4)*(h/4) blocks in row-major order.
func Encode(rgba []byte, width, height int, mode Mode) ([]byte, error) {
	if width <= 0 || height <= 0 || width%4 != 0 || height%4 != 0 {
		return nil, fmt.Errorf("bc7: size %dx%d is not a positive multiple of 4", width, height)
	}
	if len(rgba) != width*height*4 {
		return nil, fmt.Errorf("bc7: got %d bytes, want %d", len(rgba), width*height*4)
	}
	initOnce.Do(func() { C.glue_init() })

	blockRows, blocksPerRow := height/4, width/4
	out := make([]byte, blockRows*blocksPerRow*BlockSize)
	perceptual := C.int(1)
	if mode == Linear {
		perceptual = 0
	}

	// Split the block rows into strips, one goroutine each (a few strips per core balances load).
	strips := min(blockRows, runtime.GOMAXPROCS(0)*4)
	var wg sync.WaitGroup
	for s := 0; s < strips; s++ {
		by0, by1 := s*blockRows/strips, (s+1)*blockRows/strips
		if by0 == by1 {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Go memory passed to C must not contain Go pointers; plain byte slices are fine, and
			// they stay valid for the duration of the call.
			C.glue_encode_rows(
				(*C.uint8_t)(unsafe.Pointer(&rgba[0])), C.int(width), C.int(by0), C.int(by1),
				(*C.uint8_t)(unsafe.Pointer(&out[by0*blocksPerRow*BlockSize])), perceptual)
		}()
	}
	wg.Wait()
	return out, nil
}

// Decode expands BC7 blocks back to an RGBA8 image (for tests and quality checks).
func Decode(blocks []byte, width, height int) ([]byte, error) {
	if width%4 != 0 || height%4 != 0 || len(blocks) != (width/4)*(height/4)*BlockSize {
		return nil, fmt.Errorf("bc7: %d bytes do not match %dx%d", len(blocks), width, height)
	}
	rgba := make([]byte, width*height*4)
	var px [64]byte
	k := 0
	for by := 0; by < height/4; by++ {
		for bx := 0; bx < width/4; bx++ {
			if C.glue_decode_block((*C.uint8_t)(unsafe.Pointer(&blocks[k])), (*C.uint8_t)(unsafe.Pointer(&px[0]))) == 0 {
				return nil, fmt.Errorf("bc7: invalid block at %d,%d", bx, by)
			}
			k += BlockSize
			for y := 0; y < 4; y++ {
				copy(rgba[((by*4+y)*width+bx*4)*4:], px[y*16:y*16+16])
			}
		}
	}
	return rgba, nil
}
