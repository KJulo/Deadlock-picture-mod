// Package build is the high-level API: photo in -> mod files / VPK out (port of
// reference/catmod/build.py). This is what the HTTP handlers call.
package build

import (
	"fmt"
	"image"
	"math"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/bc7"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/mesh"
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

// Mod builds the mod from encoded photo bytes (JPEG/PNG/GIF/WebP) for p.Variant.
func Mod(t *Templates, photoData []byte, p photo.Params) (*Result, error) {
	return ModPhotos(t, [][]byte{photoData}, p)
}

// ModPhotos builds from all uploaded photos using the same texture as the preview.
func ModPhotos(t *Templates, photoData [][]byte, p photo.Params) (*Result, error) {
	if err := p.ValidatePhotos(len(photoData)); err != nil {
		return nil, err
	}
	imgs := make([]*photo.RGB, len(photoData))
	for i, data := range photoData {
		var err error
		imgs[i], err = photo.Load(data, p)
		if err != nil {
			return nil, err
		}
	}
	img := imgs[0]
	var err error
	if p.Variant == "cage" {
		return cage(t, img, p)
	}
	info, _, err := vtex.ReadInfo(t.Color)
	if err != nil {
		return nil, err
	}
	texture, err := photo.TexturePhotos(imgs, p, info.Width)
	if err != nil {
		return nil, err
	}
	return ModFromTexture(t, texture, Geometry(p), p.Roughness)
}

// Geometry returns the mesh for p.Shape (validated): a radial shape or the medallions.
func Geometry(p photo.Params) mesh.Geometry {
	if p.Shape == mesh.MedallionsShape {
		return mesh.MedallionGeometry(vmdl.SphereRadius, p.MedallionRad())
	}
	shape, _ := mesh.ShapeByName(p.Shape)
	return mesh.SphereGeometry(vmdl.SphereRadius, shape)
}

// ModFromTexture builds the mod from an already composed colour texture.
func ModFromTexture(t *Templates, texture *image.NRGBA, geom mesh.Geometry, roughness float64) (*Result, error) {
	model, lods, err := vmdl.BuildModel(t.Model, geom, nil)
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

// cage keeps the original model and textures and only paints the photo into the lens domes of
// the original colour texture, so the VPK contains a single file.
func cage(t *Templates, img *photo.RGB, p photo.Params) (*Result, error) {
	texture, err := vtex.DecodeTop(t.Color)
	if err != nil {
		return nil, fmt.Errorf("original colour texture: %w", err)
	}
	photo.PaintDiscs(texture, img, p, photo.CageDiscs)
	color, err := vtex.RebuildBC7(t.Color, texture, bc7.Perceptual)
	if err != nil {
		return nil, fmt.Errorf("colour texture: %w", err)
	}
	return &Result{Files: map[string][]byte{templates.Color.GamePath: color}, Texture: texture}, nil
}
