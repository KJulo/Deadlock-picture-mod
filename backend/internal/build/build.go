// Package build is the high-level API: photo in -> mod files / VPK out (port of
// reference/catmod/build.py). This is what the HTTP handlers call.
package build

import (
	"fmt"
	"image"
	"math"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/bc7"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/photo"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/templates"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/vmdl"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/vpk"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/vtex"
)

// Result holds the compiled game files plus what a preview needs.
type Result struct {
	Files   map[string][]byte // game path -> compiled file
	Texture *image.NRGBA      // colour texture before compression
	LODs    []vmdl.LODMesh    // uncompressed sphere geometry
}

// VPK packs the files into the archive Deadlock Mod Manager imports.
func (r *Result) VPK() ([]byte, error) { return vpk.Build(r.Files) }

// NormalRoughRGBA is the raw texel of the NormalRoughness texture: R,G = flat normal (128),
// B = roughness, A = 255. (Source2Viewer displays it rearranged as RGB normal + A roughness.)
func NormalRoughRGBA(roughness float64) [4]byte {
	return [4]byte{128, 128, byte(math.RoundToEven(roughness * 255)), 255}
}

// Templates are the original game files, loaded once at startup and shared by every build.
type Templates struct {
	Model, Color, NormalRough, AO []byte
}

// LoadTemplates reads and verifies the game files in dir.
func LoadTemplates(dir string) (*Templates, error) {
	var t Templates
	for _, x := range []struct {
		tpl templates.Template
		dst *[]byte
	}{{templates.Model, &t.Model}, {templates.Color, &t.Color}, {templates.NormalRough, &t.NormalRough}, {templates.AO, &t.AO}} {
		data, err := templates.Load(dir, x.tpl)
		if err != nil {
			return nil, err
		}
		*x.dst = data
	}
	return &t, nil
}

// Mod builds the sphere mod from encoded photo bytes (JPEG/PNG/GIF/WebP).
func Mod(t *Templates, photoData []byte, p photo.Params) (*Result, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	img, err := photo.Load(photoData, p)
	if err != nil {
		return nil, err
	}
	info, _, err := vtex.ReadInfo(t.Color)
	if err != nil {
		return nil, err
	}
	texture, err := photo.SphereTexture(img, p, info.Width)
	if err != nil {
		return nil, err
	}
	return ModFromTexture(t, texture, p.Roughness)
}

// ModFromTexture builds the mod from an already composed colour texture.
func ModFromTexture(t *Templates, texture *image.NRGBA, roughness float64) (*Result, error) {
	model, lods, err := vmdl.BuildSphereModel(t.Model, vmdl.SphereRadius)
	if err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}
	color, err := vtex.RebuildBC7(t.Color, texture, bc7.Perceptual)
	if err != nil {
		return nil, fmt.Errorf("colour texture: %w", err)
	}
	normalRough, err := vtex.RebuildBC7Constant(t.NormalRough, NormalRoughRGBA(roughness), bc7.Linear)
	if err != nil {
		return nil, fmt.Errorf("normal/roughness texture: %w", err)
	}
	ao, err := vtex.RebuildBC4Constant(t.AO, 255) // white = no baked occlusion
	if err != nil {
		return nil, fmt.Errorf("AO texture: %w", err)
	}
	return &Result{
		Files: map[string][]byte{
			templates.Model.GamePath:       model,
			templates.Color.GamePath:       color,
			templates.NormalRough.GamePath: normalRough,
			templates.AO.GamePath:          ao,
		},
		Texture: texture,
		LODs:    lods,
	}, nil
}
