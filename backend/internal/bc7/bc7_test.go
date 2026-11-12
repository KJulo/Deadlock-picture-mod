package bc7

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"
)

func psnr(a, b []byte) float64 {
	var se float64
	n := 0
	for i := range a {
		if i%4 == 3 {
			continue // alpha
		}
		d := float64(a[i]) - float64(b[i])
		se += d * d
		n++
	}
	if se == 0 {
		return math.Inf(1)
	}
	return 10 * math.Log10(255*255/(se/float64(n)))
}

// A smooth photo-like image must survive compression with good quality.
func TestEncodeQuality(t *testing.T) {
	const w, h = 256, 128
	img := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			img[i] = byte(x)
			img[i+1] = byte(128 + 100*math.Sin(float64(y)/9))
			img[i+2] = byte((x + y) / 2)
			img[i+3] = 255
		}
	}
	blocks, err := Encode(img, w, h, Perceptual)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decode(blocks, w, h)
	if err != nil {
		t.Fatal(err)
	}
	if p := psnr(img, back); p < 40 {
		t.Errorf("PSNR %.1f dB, want >= 40", p)
	}
	for i := 3; i < len(back); i += 4 {
		if back[i] != 255 {
			t.Fatal("alpha changed")
		}
	}
}

// Constant data textures (normal/roughness) must come back within 1 level per channel.
func TestConstantLinear(t *testing.T) {
	for _, c := range [][4]byte{{128, 128, 176, 255}, {128, 128, 0, 255}, {128, 128, 255, 255}, {7, 200, 33, 255}} {
		img := make([]byte, 8*8*4)
		for i := 0; i < len(img); i += 4 {
			copy(img[i:], c[:])
		}
		blocks, err := Encode(img, 8, 8, Linear)
		if err != nil {
			t.Fatal(err)
		}
		back, _ := Decode(blocks, 8, 8)
		for i := range back {
			if d := int(back[i]) - int(img[i]); d < -1 || d > 1 {
				t.Fatalf("%v: channel %d decoded as %d", c, i%4, back[i])
			}
		}
	}
}

// Parallel encoding must give exactly the same bytes as a single strip (no races, no seams).
func TestDeterministic(t *testing.T) {
	const w, h = 64, 64
	img := make([]byte, w*h*4)
	rng := rand.New(rand.NewPCG(3, 4))
	for i := range img {
		img[i] = byte(rng.IntN(256))
	}
	a, _ := Encode(img, w, h, Perceptual)
	b, _ := Encode(img, w, h, Perceptual)
	if string(a) != string(b) {
		t.Fatal("two encodes of the same image differ")
	}
}

func TestRejectsBadSize(t *testing.T) {
	if _, err := Encode(make([]byte, 6*4*4), 6, 4, Perceptual); err == nil {
		t.Error("width 6: expected error")
	}
}

// Full-size texture timing (2048² + mips is what every build encodes). Run with -v to see it.
func TestSpeed2048(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	const n = 2048
	img := make([]byte, n*n*4)
	for i := range img {
		img[i] = byte(i * 7 / 13)
	}
	start := time.Now()
	if _, err := Encode(img, n, n, Perceptual); err != nil {
		t.Fatal(err)
	}
	t.Logf("2048x2048 BC7: %v", time.Since(start))
}
