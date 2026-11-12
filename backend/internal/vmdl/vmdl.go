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

// LOD describes where one level of detail lives in the model and how big it must stay.
type LOD struct {
	VertexBlock, IndexBlock int
	VertexCount, IndexCount int
}

// LODs of the pinned model (from its MDAT/CTRL data, dumped with Source2Viewer-CLI).
var LODs = []LOD{
	{0, 1, 6084, 17997},
	{3, 4, 3196, 9000},
	{6, 7, 1531, 3576},
	{9, 10, 1444, 3264},
}

// LODMesh is the uncompressed geometry written into one LOD (handy for previews).
type LODMesh struct {
	Grid     mesh.Grid
	Vertices []byte // VertexCount × mesh.VertexSize
	Indices  []uint32
}

// BuildSphereModel returns the patched model and the geometry of each LOD.
func BuildSphereModel(template []byte, radius float64) ([]byte, []LODMesh, error) {
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
		vb, idx, grid, err := mesh.BuildBuffers(lod.VertexCount, lod.IndexCount, radius)
		if err != nil {
			return nil, nil, err
		}
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
		lods[n] = LODMesh{grid, vb, idx}
	}
	return res.Serialize(), lods, nil
}
