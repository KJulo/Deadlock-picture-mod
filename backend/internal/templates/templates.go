// Package templates locates the original game files the mod is built from (port of
// reference/catmod/templates.py). They are Valve assets: extracted locally with `make fetch` and
// never committed; in production they are baked into a private container image.
package templates

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// Template is one game file, pinned by sha256 when its layout matters.
type Template struct {
	GamePath string
	SHA256   string // empty = not pinned
}

const mat = "models/props_gameplay/soul_container/materials/"

var (
	Model = Template{"models/props_gameplay/soul_container/soul_container.vmdl_c",
		"20e3946fb0272c7b2740d04112374c643d5c51150d01789fd34a0f7461a134e6"}
	Color       = Template{GamePath: mat + "soul_container_color_png_1da67bfe.vtex_c"}
	NormalRough = Template{GamePath: mat + "soul_container_rough_png_59a0e409.vtex_c"}
	AO          = Template{GamePath: mat + "soul_container_ao_png_e9d127e1.vtex_c"}
	All         = []Template{Model, Color, NormalRough, AO}
)

// Dir returns the template directory: $CATMOD_TEMPLATES, or fallback.
func Dir(fallback string) string {
	if d := os.Getenv("CATMOD_TEMPLATES"); d != "" {
		return d
	}
	return fallback
}

// Load reads a template from dir and verifies its sha256.
func Load(dir string, t Template) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(t.GamePath)))
	if err != nil {
		return nil, fmt.Errorf("templates: %w (run `make fetch`)", err)
	}
	if t.SHA256 != "" {
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != t.SHA256 {
			return nil, fmt.Errorf("templates: %s differs from the version this mod was written for "+
				"(game update?); re-check the layout before trusting the output", t.GamePath)
		}
	}
	return data, nil
}
