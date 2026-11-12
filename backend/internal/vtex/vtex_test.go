package vtex

import "testing"

func TestMipSizes(t *testing.T) {
	s := mipSizes(Info{Width: 2048, Height: 2048, Mips: 10})
	if s[0] != [2]int{2048, 2048} || s[9] != [2]int{4, 4} {
		t.Errorf("got %v ... %v", s[0], s[9])
	}
	s = mipSizes(Info{Width: 16, Height: 16, Mips: 5}) // never below one 4x4 block
	if s[4] != [2]int{4, 4} {
		t.Errorf("got %v", s[4])
	}
}

func TestHalve(t *testing.T) {
	// 2x2 image: average of the four pixels per channel, rounded
	src := []byte{0, 10, 255, 255, 1, 20, 255, 255, 2, 30, 0, 255, 3, 41, 0, 255}
	dst, w, h := halve(src, 2, 2)
	want := []byte{2, 25, 128, 255}
	if w != 1 || h != 1 || string(dst) != string(want) {
		t.Errorf("got %v %dx%d, want %v", dst, w, h, want)
	}
}

func TestReadInfoRejectsGarbage(t *testing.T) {
	if _, _, err := ReadInfo([]byte("nope")); err == nil {
		t.Error("expected an error")
	}
}
