package vmdl

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/templates"
)

const repoRoot = "../../.."

// The whole patched model must be byte-identical to the python reference's (which works in game).
// The golden stores only its sha256: the model contains Valve data and is never committed.
func TestMatchesPythonReference(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "model_golden.json"))
	if err != nil {
		t.Skip("no golden file: run `make golden`")
	}
	var golden struct {
		SphereModelSHA256 string `json:"sphere_model_sha256"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	tpl, err := templates.Load(templates.Dir(filepath.Join(repoRoot, "templates")), templates.Model)
	if err != nil {
		t.Skip(err)
	}
	model, lods, err := BuildSphereModel(tpl, SphereRadius)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(model)
	if got := hex.EncodeToString(sum[:]); got != golden.SphereModelSHA256 {
		t.Fatalf("model sha256 %s, want %s", got, golden.SphereModelSHA256)
	}
	if len(lods) != 4 || lods[0].Grid.NU != 76 {
		t.Errorf("unexpected LOD geometry: %+v", lods[0].Grid)
	}
}
