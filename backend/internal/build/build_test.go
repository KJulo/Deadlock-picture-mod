package build

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/bc7"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/photo"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/templates"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/vtex"
)

const repoRoot = "../../.."

func loadTemplates(t *testing.T) *Templates {
	t.Helper()
	tpl, err := LoadTemplates(templates.Dir(filepath.Join(repoRoot, "templates")))
	if err != nil {
		t.Skip(err)
	}
	return tpl
}

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// End to end: photo -> 4 game files -> VPK. Needs the game files (skipped in CI).
func TestModEndToEnd(t *testing.T) {
	tpl := loadTemplates(t)
	input, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "photo_input.png"))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	res, err := Mod(tpl, input, photo.DefaultParams())
	if err != nil {
		t.Fatal(err)
	}
	pak, err := res.VPK()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("build + VPK: %v, VPK %.1f MB", time.Since(start), float64(len(pak))/1e6)

	// Same sizes as the originals (the game metadata describes them).
	for path, orig := range map[string][]byte{
		templates.Model.GamePath: tpl.Model, templates.Color.GamePath: tpl.Color,
		templates.NormalRough.GamePath: tpl.NormalRough, templates.AO.GamePath: tpl.AO,
	} {
		if path != templates.Model.GamePath && len(res.Files[path]) != len(orig) {
			t.Errorf("%s: %d bytes, original %d", path, len(res.Files[path]), len(orig))
		}
	}

	// Model and AO must be byte-identical to the python reference's.
	raw, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "model_golden.json"))
	if err == nil {
		var g struct {
			Model string `json:"sphere_model_sha256"`
			AO    string `json:"ao_white_sha256"`
		}
		json.Unmarshal(raw, &g)
		if got := sha(res.Files[templates.Model.GamePath]); got != g.Model {
			t.Errorf("model sha %s, want %s", got, g.Model)
		}
		if got := sha(res.Files[templates.AO.GamePath]); got != g.AO {
			t.Errorf("AO sha %s, want %s", got, g.AO)
		}
	}

	// The colour texture's largest mip must decode back close to the composed texture.
	info, cres, err := vtex.ReadInfo(res.Files[templates.Color.GamePath])
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 2048 || info.Format != vtex.FormatBC7 || info.Mips != 10 {
		t.Fatalf("unexpected texture info %+v", info)
	}
	top := cres.Trailer[len(cres.Trailer)-2048*2048:] // largest mip is stored last
	dec, err := bc7.Decode(top, 2048, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var se float64
	alphaOK := true
	for i := 0; i < len(dec); i++ {
		if i%4 == 3 {
			alphaOK = alphaOK && dec[i] == 255
			continue
		}
		d := float64(dec[i]) - float64(res.Texture.Pix[i])
		se += d * d
	}
	psnr := 10 * math.Log10(255*255/(se/float64(len(dec)*3/4)))
	t.Logf("colour texture PSNR %.1f dB", psnr)
	if psnr < 38 {
		t.Errorf("colour texture PSNR %.1f dB, want >= 38", psnr)
	}
	if !alphaOK {
		t.Error("colour texture alpha is not 255 everywhere")
	}

	// NormalRoughness decodes to the flat normal + roughness 0.69 (176) within 1 level.
	_, nres, _ := vtex.ReadInfo(res.Files[templates.NormalRough.GamePath])
	block, _ := bc7.Decode(nres.Trailer[:16], 4, 4)
	want := NormalRoughRGBA(0.69)
	for c := 0; c < 4; c++ {
		if d := int(block[c]) - int(want[c]); d < -1 || d > 1 {
			t.Errorf("normal/roughness channel %d = %d, want %d", c, block[c], want[c])
		}
	}
}

func TestModRejectsBadInput(t *testing.T) {
	tpl := &Templates{}
	bad := photo.DefaultParams()
	bad.Layout = "nope"
	if _, err := Mod(tpl, nil, bad); err == nil {
		t.Error("bad params accepted")
	}
	if _, err := Mod(tpl, []byte("not a photo"), photo.DefaultParams()); err == nil {
		t.Error("garbage photo accepted")
	}
}
