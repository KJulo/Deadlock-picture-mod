package photo

import "testing"

func TestSealValidationAndOverlap(t *testing.T) {
	p := DefaultParams()
	p.Seals = []Seal{{Photo: 0, Dir: [3]float64{2, 0, 0}, Center: [2]float64{.5, .5}, Radius: .5}, {Photo: 1, Dir: [3]float64{1, 0, 0}, Center: [2]float64{.5, .5}, Radius: .5}}
	if err := p.ValidatePhotos(2); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidatePhotos(1); err == nil {
		t.Fatal("missing photo accepted")
	}
	red := &RGB{2, 2, []uint8{255, 0, 0, 255, 0, 0, 255, 0, 0, 255, 0, 0}}
	blue := &RGB{2, 2, []uint8{0, 0, 255, 0, 0, 255, 0, 0, 255, 0, 0, 255}}
	sample := stickerTexelPhotos([]*RGB{red, blue}, p)
	if got := sample(vec{1, 0, 0}); got != [3]float64{255, 0, 0} {
		t.Fatalf("first seal must win tie: %v", got)
	}
	p.Seals[0].Dir = [3]float64{0, 1, 0}
	if got := stickerTexelPhotos([]*RGB{red, blue}, p)(vec{1, 0, 0}); got != [3]float64{0, 0, 255} {
		t.Fatalf("nearest seal must win: %v", got)
	}
	p.Seals[0].Dir = [3]float64{}
	if p.Validate() == nil {
		t.Fatal("zero direction accepted")
	}
}
