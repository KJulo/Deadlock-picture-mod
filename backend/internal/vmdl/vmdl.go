// Package vmdl patches soul_container.vmdl_c: every LOD's geometry is replaced with a sphere
// (port of reference/catmod/vmdl.py).
//
// Only the meshopt-compressed MVTX (vertices) and MIDX (indices) blocks change. Vertex/index
// counts, draw calls, bounds, physics (a sphere of radius 7) and material references stay
// untouched, so the binary KV3 metadata does not need rewriting.
package vmdl

import (
	"fmt"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/mesh"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/meshopt"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/resource"
)

// SphereRadius fits inside every LOD's bounding box (LOD0 bounds are ±6.34).
const SphereRadius = 6.3

// LOD describes where one level of detail lives in the model, how big it must stay and the UV
// format its vertex layout declares.
type LOD struct {
	VertexBlock, IndexBlock int
	VertexCount, IndexCount int
	UV                      mesh.UVFormat
}

// LODs of the pinned model (from its MDAT/CTRL data, dumped with Source2Viewer-CLI). The UV
// formats are the TEXCOORD m_Format of each LOD's m_inputLayoutFields: 37 (SNORM) for LODs 0-2,
// 34 (FLOAT) for LOD3.
var LODs = []LOD{
	{0, 1, 6084, 17997, mesh.UVSnorm},
	{3, 4, 3196, 9000, mesh.UVSnorm},
	{6, 7, 1531, 3576, mesh.UVSnorm},
	{9, 10, 1444, 3264, mesh.UVHalf},
}

// LODMesh is the uncompressed geometry written into one LOD (handy for previews).
type LODMesh struct {
	Grid     mesh.Grid
	Vertices []byte // VertexCount × mesh.VertexSize
	Indices  []uint32
}

// BuildSphereModel returns the patched model and the geometry of each LOD.
func BuildSphereModel(template []byte, radius float64) ([]byte, []LODMesh, error) {
	return BuildSphereModelRemap(template, radius, nil)
}

// BuildSphereModelRemap is BuildSphereModel with per-LOD UV remapping (nil = none). Used by the
// LOD diagnostic, which sends each LOD to its own labelled band of the texture.
func BuildSphereModelRemap(template []byte, radius float64, remap func(lod int) mesh.UVRemap) ([]byte, []LODMesh, error) {
	return BuildModel(template, mesh.SphereGeometry(radius, nil), remap)
}

// BuildModel writes a geometry (the sphere, a shape or the medallions) into every LOD.
func BuildModel(template []byte, geom mesh.Geometry, remap func(lod int) mesh.UVRemap) ([]byte, []LODMesh, error) {
	res, err := resource.Parse(template)
	if err != nil {
		return nil, nil, err
	}
	lods := make([]LODMesh, len(LODs))
	for n, lod := range LODs {
		if lod.IndexBlock >= len(res.Blocks) ||
			res.Blocks[lod.VertexBlock].Kind != "MVTX" || res.Blocks[lod.IndexBlock].Kind != "MIDX" {
			return nil, nil, fmt.Errorf("vmdl: LOD%d blocks not where expected (game update?)", n)
		}
		var r mesh.UVRemap
		if remap != nil {
			r = remap(n)
		}
		b, err := geom(lod.VertexCount, lod.IndexCount, lod.UV, r)
		if err != nil {
			return nil, nil, err
		}
		vb, idx := b.Vertices, b.Indices
		cv, err := meshopt.EncodeVertexBuffer(vb, mesh.VertexSize)
		if err != nil {
			return nil, nil, err
		}
		ci, err := meshopt.EncodeIndexBuffer(idx)
		if err != nil {
			return nil, nil, err
		}
		res.Blocks[lod.VertexBlock].Data = cv
		res.Blocks[lod.IndexBlock].Data = ci
		lods[n] = LODMesh{b.Grid, vb, idx}
	}
	return res.Serialize(), lods, nil
}
