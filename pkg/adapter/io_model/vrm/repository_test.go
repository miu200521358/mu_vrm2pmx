// 指示: miu200521358
package vrm

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
	"github.com/miu200521358/mlib_go/pkg/shared/base/merr"
	"gonum.org/v1/gonum/spatial/r3"
)

// mmathVec3ForTest は頂点オフセット比較用の簡易ベクトルを表す。
type mmathVec3ForTest struct {
	X float64
	Y float64
	Z float64
}

func TestVrmRepositoryCanLoad(t *testing.T) {
	repository := NewVrmRepository()

	if !repository.CanLoad("sample.vrm") {
		t.Fatalf("expected sample.vrm to be loadable")
	}
	if !repository.CanLoad("sample.VRM") {
		t.Fatalf("expected sample.VRM to be loadable")
	}
	if repository.CanLoad("sample.pmx") {
		t.Fatalf("expected sample.pmx to be not loadable")
	}
}

func TestVrmRepositoryInferName(t *testing.T) {
	repository := NewVrmRepository()

	got := repository.InferName("C:/work/avatar.vrm")
	if got != "avatar" {
		t.Fatalf("expected avatar, got %s", got)
	}
}

func TestVrmRepositoryLoadReturnsExtInvalid(t *testing.T) {
	repository := NewVrmRepository()

	_, err := repository.Load("sample.pmx")
	if err == nil {
		t.Fatalf("expected error to be not nil")
	}
	if merr.ExtractErrorID(err) != "14102" {
		t.Fatalf("expected error id 14102, got %s", merr.ExtractErrorID(err))
	}
}

func TestVrmRepositoryLoadReturnsFileNotFound(t *testing.T) {
	repository := NewVrmRepository()

	_, err := repository.Load(filepath.Join(t.TempDir(), "missing.vrm"))
	if err == nil {
		t.Fatalf("expected error to be not nil")
	}
	if merr.ExtractErrorID(err) != "14101" {
		t.Fatalf("expected error id 14101, got %s", merr.ExtractErrorID(err))
	}
}

func TestVrmRepositoryLoadVrm1PreferredAndRawNodeBoneNames(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "avatar.vrm")

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRoid Studio v1.0.0",
		},
		"extensionsUsed": []string{"VRM", "VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name":        "hips_node",
				"translation": []float64{0, 0.9, 0},
				"children":    []int{1},
			},
			map[string]any{
				"name":        "spine_node",
				"translation": []float64{0, 0.2, 0},
				"children":    []int{2},
			},
			map[string]any{
				"name":        "chest_node",
				"translation": []float64{0, 0.2, 0},
			},
			map[string]any{
				"name":        "extra_node",
				"translation": []float64{0.1, 0.3, 0.2},
			},
		},
		"extensions": map[string]any{
			"VRM": map[string]any{
				"exporterVersion": "VRoid Studio v0.14.0",
				"humanoid": map[string]any{
					"humanBones": []any{
						map[string]any{"bone": "hips", "node": 0},
						map[string]any{"bone": "spine", "node": 1},
						map[string]any{"bone": "chest", "node": 2},
					},
				},
			},
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips":       map[string]any{"node": 0},
						"spine":      map[string]any{"node": 1},
						"upperChest": map[string]any{"node": 2},
					},
				},
			},
		},
	}
	writeGLBFileForTest(t, path, doc)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}
	if pmxModel.VrmData == nil {
		t.Fatalf("expected vrm data")
	}
	if pmxModel.VrmData.Version != vrm.VRM_VERSION_1 {
		t.Fatalf("expected VRM_VERSION_1, got %s", pmxModel.VrmData.Version)
	}
	if pmxModel.VrmData.Profile != vrm.VRM_PROFILE_VROID {
		t.Fatalf("expected VRM_PROFILE_VROID, got %s", pmxModel.VrmData.Profile)
	}
	if pmxModel.VrmData.Vrm1 == nil {
		t.Fatalf("expected Vrm1 to be not nil")
	}
	if pmxModel.VrmData.Vrm0 != nil {
		t.Fatalf("expected Vrm0 to be nil when VRM1 is selected")
	}

	hips, err := pmxModel.Bones.GetByName("hips_node")
	if err != nil || hips == nil {
		t.Fatalf("expected hips_node bone: %v", err)
	}
	spine, err := pmxModel.Bones.GetByName("spine_node")
	if err != nil || spine == nil {
		t.Fatalf("expected spine_node bone: %v", err)
	}
	upperBody2, err := pmxModel.Bones.GetByName("chest_node")
	if err != nil || upperBody2 == nil {
		t.Fatalf("expected chest_node bone: %v", err)
	}
	if pmxModel.Bones.ContainsByName("下半身") {
		t.Fatalf("unexpected mapped bone name in Load result")
	}
	if spine.ParentIndex != hips.Index() {
		t.Fatalf("expected spine_node parent to be hips_node")
	}
	if upperBody2.ParentIndex != spine.Index() {
		t.Fatalf("expected chest_node parent to be spine_node")
	}

	extra, err := pmxModel.Bones.GetByName("extra_node")
	if err != nil || extra == nil {
		t.Fatalf("expected extra_node bone: %v", err)
	}
	if extra.Position.X <= 0 {
		t.Fatalf("expected x to keep plus direction for VRM1 VRoid profile, got %f", extra.Position.X)
	}
	if extra.Position.Z >= 0 {
		t.Fatalf("expected z to be converted to minus for VRM1 VRoid profile, got %f", extra.Position.Z)
	}
	if extra.Position.X < 1.2 || extra.Position.X > 1.3 {
		t.Fatalf("expected x to be scaled by 12.5, got %f", extra.Position.X)
	}
}

func TestVrmRepositoryLoadVrm0VroidKeepsLegacyAxisConversion(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "avatar.vrm")

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRoid Studio v0.14.0",
		},
		"extensionsUsed": []string{"VRM"},
		"nodes": []any{
			map[string]any{
				"name":        "hips_node",
				"translation": []float64{0, 0.9, 0},
			},
			map[string]any{
				"name":        "extra_node",
				"translation": []float64{0.1, 0.3, 0.2},
			},
		},
		"extensions": map[string]any{
			"VRM": map[string]any{
				"exporterVersion": "VRoid Studio v0.14.0",
				"humanoid": map[string]any{
					"humanBones": []any{
						map[string]any{"bone": "hips", "node": 0},
					},
				},
			},
		},
	}
	writeGLBFileForTest(t, path, doc)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}
	if pmxModel.VrmData == nil {
		t.Fatalf("expected vrm data")
	}
	if pmxModel.VrmData.Version != vrm.VRM_VERSION_0 {
		t.Fatalf("expected VRM_VERSION_0, got %s", pmxModel.VrmData.Version)
	}
	if pmxModel.VrmData.Profile != vrm.VRM_PROFILE_VROID {
		t.Fatalf("expected VRM_PROFILE_VROID, got %s", pmxModel.VrmData.Profile)
	}

	extra, err := pmxModel.Bones.GetByName("extra_node")
	if err != nil || extra == nil {
		t.Fatalf("expected extra_node bone: %v", err)
	}
	if extra.Position.X >= 0 {
		t.Fatalf("expected x to be converted to minus for VRM0 VRoid profile, got %f", extra.Position.X)
	}
	if extra.Position.Z <= 0 {
		t.Fatalf("expected z to keep plus direction for VRM0 VRoid profile, got %f", extra.Position.Z)
	}
	if extra.Position.X > -1.2 || extra.Position.X < -1.3 {
		t.Fatalf("expected x to be scaled by 12.5, got %f", extra.Position.X)
	}
}

func TestVrmRepositoryLoadVrm0UniVrmUsesMmdScaleConversion(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "avatar.vrm")

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "UniGLTF-1.28",
		},
		"extensionsUsed": []string{"VRM"},
		"nodes": []any{
			map[string]any{
				"name":        "hips_node",
				"translation": []float64{0, 0.9, 0},
			},
			map[string]any{
				"name":        "extra_node",
				"translation": []float64{0.1, 0.3, 0.2},
			},
		},
		"extensions": map[string]any{
			"VRM": map[string]any{
				"exporterVersion": "UniVRM-0.51.0",
				"humanoid": map[string]any{
					"humanBones": []any{
						map[string]any{"bone": "hips", "node": 0},
					},
				},
			},
		},
	}
	writeGLBFileForTest(t, path, doc)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}
	if pmxModel.VrmData == nil {
		t.Fatalf("expected vrm data")
	}
	if pmxModel.VrmData.Version != vrm.VRM_VERSION_0 {
		t.Fatalf("expected VRM_VERSION_0, got %s", pmxModel.VrmData.Version)
	}
	if pmxModel.VrmData.Profile != vrm.VRM_PROFILE_STANDARD {
		t.Fatalf("expected VRM_PROFILE_STANDARD, got %s", pmxModel.VrmData.Profile)
	}

	extra, err := pmxModel.Bones.GetByName("extra_node")
	if err != nil || extra == nil {
		t.Fatalf("expected extra_node bone: %v", err)
	}
	if extra.Position.X >= 0 {
		t.Fatalf("expected x to be converted to minus for UniVRM conversion, got %f", extra.Position.X)
	}
	if extra.Position.Z <= 0 {
		t.Fatalf("expected z to keep plus direction for UniVRM conversion, got %f", extra.Position.Z)
	}
	if extra.Position.X > -1.2 || extra.Position.X < -1.3 {
		t.Fatalf("expected x to be scaled by 12.5, got %f", extra.Position.X)
	}
}

func TestExportArtifactsWritesGltfAndTextures(t *testing.T) {
	tempDir := t.TempDir()
	vrmPath := filepath.Join(tempDir, "avatar.vrm")
	gltfDir := filepath.Join(tempDir, "out", "glTF")
	textureDir := filepath.Join(tempDir, "out", "tex")
	pngBytes := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0x60, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x01, 0xE5, 0x27, 0xD4, 0xA2, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}

	doc := map[string]any{
		"asset": map[string]any{
			"version": "2.0",
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(pngBytes),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": 0,
				"byteLength": len(pngBytes),
			},
		},
		"images": []any{
			map[string]any{
				"name":       "face",
				"bufferView": 0,
				"mimeType":   "image/png",
			},
		},
	}
	writeGLBFileForTestWithBin(t, vrmPath, doc, pngBytes)

	result, err := ExportArtifacts(vrmPath, gltfDir, textureDir)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if result == nil {
		t.Fatalf("result is nil")
	}
	if _, err := os.Stat(result.GltfPath); err != nil {
		t.Fatalf("gltf output not found: %v", err)
	}
	if _, err := os.Stat(result.BinPath); err != nil {
		t.Fatalf("bin output not found: %v", err)
	}
	if len(result.TextureNames) != 1 {
		t.Fatalf("texture name length mismatch: %d", len(result.TextureNames))
	}
	if strings.TrimSpace(result.TextureNames[0]) == "" {
		t.Fatalf("texture name is empty")
	}
	if _, err := os.Stat(filepath.Join(textureDir, result.TextureNames[0])); err != nil {
		t.Fatalf("texture output not found: %v", err)
	}
}

func TestVrmRepositoryLoadCreatesMeshFromPrimitive(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh.vrm")

	positions := []float32{
		0.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		1.0, 0.0, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.0, 1.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}

	binChunk := buildInterleavedBinForMeshTest(t, positions, normals, uvs, indices)
	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
			map[string]any{
				"name": "J_Adj_R_FaceEyeLight",
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "body",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{1.0, 1.0, 1.0, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": 0,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": len(positions) * 4,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": (len(positions) + len(normals)) * 4,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": (len(positions) + len(normals) + len(uvs)) * 4,
				"byteLength": len(indices) * 2,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips": map[string]any{"node": 0},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}
	if pmxModel.Vertices.Len() != 3 {
		t.Fatalf("expected 3 vertices, got %d", pmxModel.Vertices.Len())
	}
	if pmxModel.Faces.Len() != 1 {
		t.Fatalf("expected 1 face, got %d", pmxModel.Faces.Len())
	}
	if pmxModel.Materials.Len() != 1 {
		t.Fatalf("expected 1 material, got %d", pmxModel.Materials.Len())
	}
}

func TestVrmRepositoryLoadBuildsExpressionMorphsFromVrm1Definitions(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_expression.vrm")

	positions := []float32{
		0.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		1.0, 0.0, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.0, 1.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}
	targetPositions := []float32{
		0.0, 0.0, 0.0,
		0.0, 0.1, 0.0,
		0.0, 0.0, 0.0,
	}

	var buf bytes.Buffer
	for _, value := range positions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write position failed: %v", err)
		}
	}
	positionOffset := 0
	for _, value := range normals {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write normal failed: %v", err)
		}
	}
	normalOffset := len(positions) * 4
	for _, value := range uvs {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write uv failed: %v", err)
		}
	}
	uvOffset := normalOffset + len(normals)*4
	for _, value := range indices {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write index failed: %v", err)
		}
	}
	indexOffset := uvOffset + len(uvs)*4
	if padding := buf.Len() % 4; padding != 0 {
		buf.Write(bytes.Repeat([]byte{0x00}, 4-padding))
	}
	targetOffset := buf.Len()
	for _, value := range targetPositions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write target position failed: %v", err)
		}
	}
	binChunk := buf.Bytes()

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
			map[string]any{
				"name": "J_Adj_R_FaceEyeLight",
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
						"extras": map[string]any{
							"targetNames": []string{"Fcl_ALL_Angry"},
						},
						"targets": []any{
							map[string]any{
								"POSITION": 4,
							},
						},
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "body",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{1.0, 1.0, 1.0, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": positionOffset,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": normalOffset,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": uvOffset,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": indexOffset,
				"byteLength": len(indices) * 2,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": targetOffset,
				"byteLength": len(targetPositions) * 4,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
			map[string]any{
				"bufferView":    4,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips": map[string]any{"node": 0},
					},
				},
				"expressions": map[string]any{
					"custom": map[string]any{
						"Fcl_ALL_Angry": map[string]any{
							"morphTargetBinds": []any{
								map[string]any{
									"node":   1,
									"index":  0,
									"weight": 1.0,
								},
							},
						},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}
	expressionMorph, err := pmxModel.Morphs.GetByName("Fcl_ALL_Angry")
	if err != nil || expressionMorph == nil {
		t.Fatalf("expression morph not found: err=%v", err)
	}
	if expressionMorph.MorphType != model.MORPH_TYPE_VERTEX {
		t.Fatalf("expression morph type mismatch: got=%d want=%d", expressionMorph.MorphType, model.MORPH_TYPE_VERTEX)
	}
	if len(expressionMorph.Offsets) != 1 {
		t.Fatalf("expression morph offset count mismatch: got=%d want=1", len(expressionMorph.Offsets))
	}
	vertexOffset, ok := expressionMorph.Offsets[0].(*model.VertexMorphOffset)
	if !ok {
		t.Fatalf("expression morph offset type mismatch: got=%T", expressionMorph.Offsets[0])
	}
	if vertexOffset.VertexIndex < 0 || vertexOffset.VertexIndex >= pmxModel.Vertices.Len() {
		t.Fatalf(
			"expression vertex offset index out of range: got=%d vertices=%d",
			vertexOffset.VertexIndex,
			pmxModel.Vertices.Len(),
		)
	}
	deltaLength := math.Abs(vertexOffset.Position.X) + math.Abs(vertexOffset.Position.Y) + math.Abs(vertexOffset.Position.Z)
	if deltaLength < 1e-6 {
		t.Fatalf(
			"expression vertex offset should not be zero: x=%.7f y=%.7f z=%.7f",
			vertexOffset.Position.X,
			vertexOffset.Position.Y,
			vertexOffset.Position.Z,
		)
	}
	internalMorphCount := 0
	for _, morphData := range pmxModel.Morphs.Values() {
		if morphData == nil {
			continue
		}
		if strings.HasPrefix(morphData.Name(), "__vrm_target_m000_t000_") {
			internalMorphCount++
		}
	}
	if internalMorphCount == 0 {
		t.Fatal("internal target morph should be generated")
	}
}

func TestVrmRepositoryLoadMapsVrm1PresetBlinkRightToCanonicalMorphName(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_expression_vrm1_preset_blink_right.vrm")

	positions := []float32{
		0.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		1.0, 0.0, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.0, 1.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}
	targetPositions := []float32{
		0.0, 0.0, 0.0,
		0.0, 0.1, 0.0,
		0.0, 0.0, 0.0,
	}

	var buf bytes.Buffer
	for _, value := range positions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write position failed: %v", err)
		}
	}
	positionOffset := 0
	for _, value := range normals {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write normal failed: %v", err)
		}
	}
	normalOffset := len(positions) * 4
	for _, value := range uvs {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write uv failed: %v", err)
		}
	}
	uvOffset := normalOffset + len(normals)*4
	for _, value := range indices {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write index failed: %v", err)
		}
	}
	indexOffset := uvOffset + len(uvs)*4
	if padding := buf.Len() % 4; padding != 0 {
		buf.Write(bytes.Repeat([]byte{0x00}, 4-padding))
	}
	targetOffset := buf.Len()
	for _, value := range targetPositions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write target position failed: %v", err)
		}
	}
	binChunk := buf.Bytes()

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
			map[string]any{
				"name": "J_Adj_R_FaceEyeLight",
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
						"extras": map[string]any{
							"targetNames": []string{"blink_right_src"},
						},
						"targets": []any{
							map[string]any{
								"POSITION": 4,
							},
						},
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "body",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{1.0, 1.0, 1.0, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": positionOffset,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": normalOffset,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": uvOffset,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": indexOffset,
				"byteLength": len(indices) * 2,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": targetOffset,
				"byteLength": len(targetPositions) * 4,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
			map[string]any{
				"bufferView":    4,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips": map[string]any{"node": 0},
					},
				},
				"expressions": map[string]any{
					"preset": map[string]any{
						"blinkRight": map[string]any{
							"morphTargetBinds": []any{
								map[string]any{
									"node":   1,
									"index":  0,
									"weight": 1.0,
								},
							},
						},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}
	if _, err := pmxModel.Morphs.GetByName("ｳｨﾝｸ２右"); err != nil {
		t.Fatalf("canonical morph ｳｨﾝｸ２右 should exist: err=%v", err)
	}
	winkBoneMorph, err := pmxModel.Morphs.GetByName("ｳｨﾝｸ２右ボーン")
	if err != nil || winkBoneMorph == nil {
		t.Fatalf("bone morph ｳｨﾝｸ２右ボーン should exist: err=%v", err)
	}
	if winkBoneMorph.MorphType != model.MORPH_TYPE_BONE {
		t.Fatalf("ｳｨﾝｸ２右ボーン morph type mismatch: got=%d want=%d", winkBoneMorph.MorphType, model.MORPH_TYPE_BONE)
	}
	if len(winkBoneMorph.Offsets) == 0 {
		t.Fatal("ｳｨﾝｸ２右ボーン should have at least one offset")
	}
	winkBoneOffset, ok := winkBoneMorph.Offsets[0].(*model.BoneMorphOffset)
	if !ok || winkBoneOffset == nil {
		t.Fatalf("ｳｨﾝｸ２右ボーン offset type mismatch: got=%T", winkBoneMorph.Offsets[0])
	}
	hasMove := math.Abs(winkBoneOffset.Position.X)+math.Abs(winkBoneOffset.Position.Y)+math.Abs(winkBoneOffset.Position.Z) > 1e-9
	hasRotate := math.Abs(winkBoneOffset.Rotation.X())+math.Abs(winkBoneOffset.Rotation.Y())+math.Abs(winkBoneOffset.Rotation.Z()) > 1e-9
	if !hasMove && !hasRotate {
		t.Fatal("ｳｨﾝｸ２右ボーン offset should not be zero")
	}
	if _, err := pmxModel.Morphs.GetByName("blinkRight"); err == nil {
		t.Fatal("preset raw name blinkRight should not remain after canonical mapping")
	}
}

func TestVrmRepositoryLoadMapsVrm0PresetBlinkRToCanonicalMorphName(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_expression_vrm0_preset_blink_r.vrm")

	positions := []float32{
		0.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		1.0, 0.0, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.0, 1.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}
	targetPositions := []float32{
		0.0, 0.0, 0.0,
		0.0, 0.1, 0.0,
		0.0, 0.0, 0.0,
	}

	var buf bytes.Buffer
	for _, value := range positions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write position failed: %v", err)
		}
	}
	positionOffset := 0
	for _, value := range normals {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write normal failed: %v", err)
		}
	}
	normalOffset := len(positions) * 4
	for _, value := range uvs {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write uv failed: %v", err)
		}
	}
	uvOffset := normalOffset + len(normals)*4
	for _, value := range indices {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write index failed: %v", err)
		}
	}
	indexOffset := uvOffset + len(uvs)*4
	if padding := buf.Len() % 4; padding != 0 {
		buf.Write(bytes.Repeat([]byte{0x00}, 4-padding))
	}
	targetOffset := buf.Len()
	for _, value := range targetPositions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write target position failed: %v", err)
		}
	}
	binChunk := buf.Bytes()

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRM"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
			map[string]any{
				"name": "J_Adj_C_Tongue1",
			},
			map[string]any{
				"name": "J_Adj_C_Tongue2",
			},
			map[string]any{
				"name": "J_Adj_C_Tongue3",
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
						"extras": map[string]any{
							"targetNames": []string{"blink_r_src"},
						},
						"targets": []any{
							map[string]any{
								"POSITION": 4,
							},
						},
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "body",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{1.0, 1.0, 1.0, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": positionOffset,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": normalOffset,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": uvOffset,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": indexOffset,
				"byteLength": len(indices) * 2,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": targetOffset,
				"byteLength": len(targetPositions) * 4,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
			map[string]any{
				"bufferView":    4,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
		},
		"extensions": map[string]any{
			"VRM": map[string]any{
				"humanoid": map[string]any{
					"humanBones": []any{
						map[string]any{"bone": "hips", "node": 0},
					},
				},
				"blendShapeMaster": map[string]any{
					"blendShapeGroups": []any{
						map[string]any{
							"name":       "",
							"presetName": "blink_r",
							"binds": []any{
								map[string]any{
									"mesh":   0,
									"index":  0,
									"weight": 100.0,
								},
							},
						},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}
	if _, err := pmxModel.Morphs.GetByName("ｳｨﾝｸ２右"); err != nil {
		t.Fatalf("canonical morph ｳｨﾝｸ２右 should exist: err=%v", err)
	}
	if _, err := pmxModel.Morphs.GetByName("blink_r"); err == nil {
		t.Fatal("preset raw name blink_r should not remain after canonical mapping")
	}
}

func TestVrmRepositoryLoadBuildsAiueoBoneAndGroupFromVrm0NameAlias(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_expression_vrm0_name_a.vrm")

	positions := []float32{
		0.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		1.0, 0.0, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.0, 1.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}
	targetPositions := []float32{
		0.0, 0.0, 0.0,
		0.0, 0.1, 0.0,
		0.0, 0.0, 0.0,
	}

	var buf bytes.Buffer
	for _, value := range positions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write position failed: %v", err)
		}
	}
	positionOffset := 0
	for _, value := range normals {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write normal failed: %v", err)
		}
	}
	normalOffset := len(positions) * 4
	for _, value := range uvs {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write uv failed: %v", err)
		}
	}
	uvOffset := normalOffset + len(normals)*4
	for _, value := range indices {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write index failed: %v", err)
		}
	}
	indexOffset := uvOffset + len(uvs)*4
	if padding := buf.Len() % 4; padding != 0 {
		buf.Write(bytes.Repeat([]byte{0x00}, 4-padding))
	}
	targetOffset := buf.Len()
	for _, value := range targetPositions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write target position failed: %v", err)
		}
	}
	binChunk := buf.Bytes()

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRM"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
			map[string]any{
				"name": "J_Adj_C_Tongue1",
			},
			map[string]any{
				"name": "J_Adj_C_Tongue2",
			},
			map[string]any{
				"name": "J_Adj_C_Tongue3",
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
						"extras": map[string]any{
							"targetNames": []string{"a_src"},
						},
						"targets": []any{
							map[string]any{
								"POSITION": 4,
							},
						},
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "body",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{1.0, 1.0, 1.0, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": positionOffset,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": normalOffset,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": uvOffset,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": indexOffset,
				"byteLength": len(indices) * 2,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": targetOffset,
				"byteLength": len(targetPositions) * 4,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
			map[string]any{
				"bufferView":    4,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
		},
		"extensions": map[string]any{
			"VRM": map[string]any{
				"humanoid": map[string]any{
					"humanBones": []any{
						map[string]any{"bone": "hips", "node": 0},
					},
				},
				"blendShapeMaster": map[string]any{
					"blendShapeGroups": []any{
						map[string]any{
							"name":       "A",
							"presetName": "",
							"binds": []any{
								map[string]any{
									"mesh":   0,
									"index":  0,
									"weight": 100.0,
								},
							},
						},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}

	aiueoVertex, err := pmxModel.Morphs.GetByName("あ頂点")
	if err != nil || aiueoVertex == nil {
		t.Fatalf("canonical morph あ頂点 should exist: err=%v", err)
	}
	if aiueoVertex.MorphType != model.MORPH_TYPE_VERTEX {
		t.Fatalf("あ頂点 morph type mismatch: got=%d want=%d", aiueoVertex.MorphType, model.MORPH_TYPE_VERTEX)
	}

	aiueoBone, err := pmxModel.Morphs.GetByName("あボーン")
	if err != nil || aiueoBone == nil {
		t.Fatalf("bone morph あボーン should exist: err=%v", err)
	}
	if aiueoBone.MorphType != model.MORPH_TYPE_BONE {
		t.Fatalf("あボーン morph type mismatch: got=%d want=%d", aiueoBone.MorphType, model.MORPH_TYPE_BONE)
	}
	if len(aiueoBone.Offsets) == 0 {
		t.Fatal("あボーン should have at least one offset")
	}
	aiueoBoneOffset, ok := aiueoBone.Offsets[0].(*model.BoneMorphOffset)
	if !ok || aiueoBoneOffset == nil {
		t.Fatalf("あボーン offset type mismatch: got=%T", aiueoBone.Offsets[0])
	}
	if math.Abs(aiueoBoneOffset.Rotation.X())+math.Abs(aiueoBoneOffset.Rotation.Y())+math.Abs(aiueoBoneOffset.Rotation.Z()) <= 1e-9 {
		t.Fatal("あボーン rotation should not be zero")
	}

	aiueoGroup, err := pmxModel.Morphs.GetByName("あ")
	if err != nil || aiueoGroup == nil {
		t.Fatalf("group morph あ should exist: err=%v", err)
	}
	if aiueoGroup.MorphType != model.MORPH_TYPE_GROUP {
		t.Fatalf("あ morph type mismatch: got=%d want=%d", aiueoGroup.MorphType, model.MORPH_TYPE_GROUP)
	}
	hasVertexBind := false
	hasBoneBind := false
	for _, rawOffset := range aiueoGroup.Offsets {
		groupOffset, ok := rawOffset.(*model.GroupMorphOffset)
		if !ok || groupOffset == nil {
			continue
		}
		bindMorph, getErr := pmxModel.Morphs.Get(groupOffset.MorphIndex)
		if getErr != nil || bindMorph == nil {
			continue
		}
		if bindMorph.Name() == "あ頂点" {
			hasVertexBind = true
		}
		if bindMorph.Name() == "あボーン" {
			hasBoneBind = true
		}
	}
	if !hasVertexBind || !hasBoneBind {
		t.Fatalf("あ group should bind both vertex/bone morphs: hasVertex=%t hasBone=%t", hasVertexBind, hasBoneBind)
	}

	if _, err := pmxModel.Morphs.GetByName("A"); err == nil {
		t.Fatal("raw name A should not remain after canonical mapping")
	}
}

func TestVrmRepositoryLoadBuildsAiueoBoneAndGroupFromRawTargetAlias(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_expression_raw_target_alias_a.vrm")

	positions := []float32{
		0.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		1.0, 0.0, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.0, 1.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}
	targetPositions := []float32{
		0.0, 0.0, 0.0,
		0.0, 0.1, 0.0,
		0.0, 0.0, 0.0,
	}

	var buf bytes.Buffer
	for _, value := range positions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write position failed: %v", err)
		}
	}
	positionOffset := 0
	for _, value := range normals {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write normal failed: %v", err)
		}
	}
	normalOffset := len(positions) * 4
	for _, value := range uvs {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write uv failed: %v", err)
		}
	}
	uvOffset := normalOffset + len(normals)*4
	for _, value := range indices {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write index failed: %v", err)
		}
	}
	indexOffset := uvOffset + len(uvs)*4
	if padding := buf.Len() % 4; padding != 0 {
		buf.Write(bytes.Repeat([]byte{0x00}, 4-padding))
	}
	targetOffset := buf.Len()
	for _, value := range targetPositions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write target position failed: %v", err)
		}
	}
	binChunk := buf.Bytes()

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
						"extras": map[string]any{
							"targetNames": []string{"A"},
						},
						"targets": []any{
							map[string]any{
								"POSITION": 4,
							},
						},
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "Face_00_SKIN",
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": positionOffset,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": normalOffset,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": uvOffset,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": indexOffset,
				"byteLength": len(indices) * 2,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": targetOffset,
				"byteLength": len(targetPositions) * 4,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
			map[string]any{
				"bufferView":    4,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips": map[string]any{"node": 0},
					},
				},
				"expressions": map[string]any{
					"preset": map[string]any{},
					"custom": map[string]any{},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}

	aiueoBone, err := pmxModel.Morphs.GetByName("あボーン")
	if err != nil || aiueoBone == nil {
		t.Fatalf("bone morph あボーン should exist: err=%v", err)
	}
	if len(aiueoBone.Offsets) == 0 {
		t.Fatal("あボーン should have at least one offset")
	}

	aiueoGroup, err := pmxModel.Morphs.GetByName("あ")
	if err != nil || aiueoGroup == nil {
		t.Fatalf("group morph あ should exist: err=%v", err)
	}
	hasBoneBind := false
	for _, rawOffset := range aiueoGroup.Offsets {
		groupOffset, ok := rawOffset.(*model.GroupMorphOffset)
		if !ok || groupOffset == nil {
			continue
		}
		bindMorph, getErr := pmxModel.Morphs.Get(groupOffset.MorphIndex)
		if getErr != nil || bindMorph == nil {
			continue
		}
		if bindMorph.Name() == "あボーン" {
			hasBoneBind = true
		}
	}
	if !hasBoneBind {
		t.Fatal("あ group should bind あボーン morph")
	}
}

func TestVrmRepositoryLoadKeepsMmdComponentMorphNames(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_expression_mmd_component_names.vrm")

	positions := []float32{
		0.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		1.0, 0.0, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.0, 1.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}
	targetPositions := []float32{
		0.0, 0.0, 0.0,
		0.0, 0.1, 0.0,
		0.0, 0.0, 0.0,
	}

	var buf bytes.Buffer
	for _, value := range positions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write position failed: %v", err)
		}
	}
	positionOffset := 0
	for _, value := range normals {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write normal failed: %v", err)
		}
	}
	normalOffset := len(positions) * 4
	for _, value := range uvs {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write uv failed: %v", err)
		}
	}
	uvOffset := normalOffset + len(normals)*4
	for _, value := range indices {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write index failed: %v", err)
		}
	}
	indexOffset := uvOffset + len(uvs)*4
	if padding := buf.Len() % 4; padding != 0 {
		buf.Write(bytes.Repeat([]byte{0x00}, 4-padding))
	}
	targetOffset := buf.Len()
	for _, value := range targetPositions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write target position failed: %v", err)
		}
	}
	binChunk := buf.Bytes()

	doc := map[string]any{
		"asset": map[string]any{
			"version": "2.0",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
						"extras": map[string]any{
							"targetNames": []string{"legacy_component_src"},
						},
						"targets": []any{
							map[string]any{
								"POSITION": 4,
							},
						},
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "body",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{1.0, 1.0, 1.0, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": positionOffset,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": normalOffset,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": uvOffset,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": indexOffset,
				"byteLength": len(indices) * 2,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": targetOffset,
				"byteLength": len(targetPositions) * 4,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
			map[string]any{
				"bufferView":    4,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips": map[string]any{"node": 0},
					},
				},
				"expressions": map[string]any{
					"custom": map[string]any{
						"上": map[string]any{
							"morphTargetBinds": []any{
								map[string]any{
									"node":   1,
									"index":  0,
									"weight": 1.0,
								},
							},
						},
						"驚き": map[string]any{
							"morphTargetBinds": []any{
								map[string]any{
									"node":   1,
									"index":  0,
									"weight": 1.0,
								},
							},
						},
						"▲ボーン": map[string]any{
							"morphTargetBinds": []any{
								map[string]any{
									"node":   1,
									"index":  0,
									"weight": 1.0,
								},
							},
						},
						"まばたき連動": map[string]any{
							"morphTargetBinds": []any{
								map[string]any{
									"node":   1,
									"index":  0,
									"weight": 1.0,
								},
							},
						},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}
	sorrowBone, err := pmxModel.Morphs.GetByName("▲ボーン")
	if err != nil || sorrowBone == nil {
		t.Fatalf("mmd morph ▲ボーン should exist: err=%v", err)
	}
	if sorrowBone.EnglishName != "▲ボーン" {
		t.Fatalf("english name mismatch for ▲ボーン: got=%s want=▲ボーン", sorrowBone.EnglishName)
	}

	blinkGroup, err := pmxModel.Morphs.GetByName("まばたき連動")
	if err != nil || blinkGroup == nil {
		t.Fatalf("mmd morph まばたき連動 should exist: err=%v", err)
	}
	if blinkGroup.EnglishName != "まばたき連動" {
		t.Fatalf("english name mismatch for まばたき連動: got=%s want=まばたき連動", blinkGroup.EnglishName)
	}

	browAbove, err := pmxModel.Morphs.GetByName("上")
	if err != nil || browAbove == nil {
		t.Fatalf("mmd morph 上 should exist: err=%v", err)
	}
	if browAbove.EnglishName != "上" {
		t.Fatalf("english name mismatch for 上: got=%s want=上", browAbove.EnglishName)
	}

	browSurprised, err := pmxModel.Morphs.GetByName("驚き")
	if err != nil || browSurprised == nil {
		t.Fatalf("mmd morph 驚き should exist: err=%v", err)
	}
	if browSurprised.EnglishName != "驚き" {
		t.Fatalf("english name mismatch for 驚き: got=%s want=驚き", browSurprised.EnglishName)
	}
}

func TestAppendExpressionBoneFallbackMorphsCreatesBoneMorphOffsets(t *testing.T) {
	modelData := model.NewPmxModel()

	rightEyeBone := model.NewBoneByName("J_Adj_R_FaceEyeLight")
	modelData.Bones.AppendRaw(rightEyeBone)

	sourceMorph := &model.Morph{
		Panel:     model.MORPH_PANEL_EYE_UPPER_LEFT,
		MorphType: model.MORPH_TYPE_VERTEX,
		Offsets: []model.IMorphOffset{
			&model.VertexMorphOffset{
				VertexIndex: 0,
				Position:    mmath.Vec3{Vec: r3.Vec{Y: 0.1}},
			},
		},
	}
	sourceMorph.SetName("ｳｨﾝｸ２右")
	sourceMorph.EnglishName = "ｳｨﾝｸ２右"
	modelData.Morphs.AppendRaw(sourceMorph)

	appendExpressionBoneFallbackMorphs(modelData)

	boneMorph, err := modelData.Morphs.GetByName("ｳｨﾝｸ２右ボーン")
	if err != nil || boneMorph == nil {
		t.Fatalf("bone morph ｳｨﾝｸ２右ボーン should be created: err=%v", err)
	}
	if boneMorph.MorphType != model.MORPH_TYPE_BONE {
		t.Fatalf("morph type mismatch: got=%d want=%d", boneMorph.MorphType, model.MORPH_TYPE_BONE)
	}
	if len(boneMorph.Offsets) == 0 {
		t.Fatal("bone morph should have offsets")
	}
	boneOffset, ok := boneMorph.Offsets[0].(*model.BoneMorphOffset)
	if !ok || boneOffset == nil {
		t.Fatalf("offset type mismatch: got=%T", boneMorph.Offsets[0])
	}
	hasMove := math.Abs(boneOffset.Position.X)+math.Abs(boneOffset.Position.Y)+math.Abs(boneOffset.Position.Z) > 1e-9
	hasRotate := math.Abs(boneOffset.Rotation.X())+math.Abs(boneOffset.Rotation.Y())+math.Abs(boneOffset.Rotation.Z()) > 1e-9
	if !hasMove && !hasRotate {
		t.Fatal("bone morph offset should not be zero")
	}
}

func TestVrmRepositoryLoadBuildsCreateEyeScaleMorphsFromFallbackRules(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_expression_create_eye.vrm")

	positions := []float32{
		0.5, 0.0, 0.0,
		0.5, 0.2, 0.0,
		0.3, 0.1, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.5, 1.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}
	targetPositions := []float32{
		0.0, 0.03, 0.0,
		0.0, 0.04, 0.0,
		0.0, 0.05, 0.0,
	}

	var buf bytes.Buffer
	for _, value := range positions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write position failed: %v", err)
		}
	}
	positionOffset := 0
	for _, value := range normals {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write normal failed: %v", err)
		}
	}
	normalOffset := len(positions) * 4
	for _, value := range uvs {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write uv failed: %v", err)
		}
	}
	uvOffset := normalOffset + len(normals)*4
	for _, value := range indices {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write index failed: %v", err)
		}
	}
	indexOffset := uvOffset + len(uvs)*4
	if padding := buf.Len() % 4; padding != 0 {
		buf.Write(bytes.Repeat([]byte{0x00}, 4-padding))
	}
	targetOffset := buf.Len()
	for _, value := range targetPositions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write target position failed: %v", err)
		}
	}
	binChunk := buf.Bytes()

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
						"extras": map[string]any{
							"targetNames": []string{"びっくり右"},
						},
						"targets": []any{
							map[string]any{
								"POSITION": 4,
							},
						},
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "EyeIris_00_EYE",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{1.0, 1.0, 1.0, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": positionOffset,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": normalOffset,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": uvOffset,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": indexOffset,
				"byteLength": len(indices) * 2,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": targetOffset,
				"byteLength": len(targetPositions) * 4,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
			map[string]any{
				"bufferView":    4,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips": map[string]any{"node": 0},
					},
				},
				"expressions": map[string]any{
					"custom": map[string]any{
						"びっくり右": map[string]any{
							"morphTargetBinds": []any{
								map[string]any{
									"node":   1,
									"index":  0,
									"weight": 1.0,
								},
							},
						},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}

	baseMorph, err := pmxModel.Morphs.GetByName("びっくり右")
	if err != nil || baseMorph == nil {
		t.Fatalf("base morph not found: err=%v", err)
	}
	smallMorph, err := pmxModel.Morphs.GetByName("瞳小右")
	if err != nil || smallMorph == nil {
		names := []string{}
		for _, morphData := range pmxModel.Morphs.Values() {
			if morphData == nil {
				continue
			}
			names = append(names, morphData.Name())
		}
		t.Fatalf("create morph 瞳小右 not found: err=%v morphs=%v", err, names)
	}
	bigMorph, err := pmxModel.Morphs.GetByName("瞳大右")
	if err != nil || bigMorph == nil {
		t.Fatalf("create morph 瞳大右 not found: err=%v", err)
	}
	if smallMorph.MorphType != model.MORPH_TYPE_VERTEX {
		t.Fatalf("瞳小右 morph type mismatch: got=%d want=%d", smallMorph.MorphType, model.MORPH_TYPE_VERTEX)
	}
	if bigMorph.MorphType != model.MORPH_TYPE_VERTEX {
		t.Fatalf("瞳大右 morph type mismatch: got=%d want=%d", bigMorph.MorphType, model.MORPH_TYPE_VERTEX)
	}
	if len(baseMorph.Offsets) == 0 {
		t.Fatalf("base morph offsets should not be empty")
	}
	if len(smallMorph.Offsets) != len(baseMorph.Offsets) {
		t.Fatalf("瞳小右 offsets mismatch: got=%d want=%d", len(smallMorph.Offsets), len(baseMorph.Offsets))
	}
	if len(bigMorph.Offsets) != len(baseMorph.Offsets) {
		t.Fatalf("瞳大右 offsets mismatch: got=%d want=%d", len(bigMorph.Offsets), len(baseMorph.Offsets))
	}

	baseOffsets := map[int]mmathVec3ForTest{}
	for _, rawOffset := range baseMorph.Offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil {
			continue
		}
		baseOffsets[offsetData.VertexIndex] = mmathVec3ForTest{
			X: offsetData.Position.X,
			Y: offsetData.Position.Y,
			Z: offsetData.Position.Z,
		}
	}
	smallOffsets := map[int]mmathVec3ForTest{}
	for _, rawOffset := range smallMorph.Offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil {
			continue
		}
		smallOffsets[offsetData.VertexIndex] = mmathVec3ForTest{
			X: offsetData.Position.X,
			Y: offsetData.Position.Y,
			Z: offsetData.Position.Z,
		}
	}
	bigOffsets := map[int]mmathVec3ForTest{}
	for _, rawOffset := range bigMorph.Offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil {
			continue
		}
		bigOffsets[offsetData.VertexIndex] = mmathVec3ForTest{
			X: offsetData.Position.X,
			Y: offsetData.Position.Y,
			Z: offsetData.Position.Z,
		}
	}
	for vertexIndex, baseOffset := range baseOffsets {
		smallOffset, exists := smallOffsets[vertexIndex]
		if !exists {
			t.Fatalf("瞳小右 missing vertex offset: vertex=%d", vertexIndex)
		}
		if math.Abs(smallOffset.X-baseOffset.X) > 1e-6 ||
			math.Abs(smallOffset.Y-baseOffset.Y) > 1e-6 ||
			math.Abs(smallOffset.Z-baseOffset.Z) > 1e-6 {
			t.Fatalf("瞳小右 offset mismatch: vertex=%d got=%+v want=%+v", vertexIndex, smallOffset, baseOffset)
		}
		bigOffset, exists := bigOffsets[vertexIndex]
		if !exists {
			t.Fatalf("瞳大右 missing vertex offset: vertex=%d", vertexIndex)
		}
		if math.Abs(bigOffset.X+baseOffset.X) > 1e-6 ||
			math.Abs(bigOffset.Y+baseOffset.Y) > 1e-6 ||
			math.Abs(bigOffset.Z+baseOffset.Z) > 1e-6 {
			t.Fatalf("瞳大右 offset mismatch: vertex=%d got=%+v base=%+v", vertexIndex, bigOffset, baseOffset)
		}
	}
}

func TestVrmRepositoryLoadBuildsGroupMorphFromMorphPairBindFallbackRules(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_expression_bind_fallback.vrm")

	positions := []float32{
		0.5, 0.0, 0.0,
		0.5, 0.2, 0.0,
		0.3, 0.1, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.5, 1.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}
	targetPositions := []float32{
		0.0, 0.03, 0.0,
		0.0, 0.04, 0.0,
		0.0, 0.05, 0.0,
	}

	var buf bytes.Buffer
	for _, value := range positions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write position failed: %v", err)
		}
	}
	positionOffset := 0
	for _, value := range normals {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write normal failed: %v", err)
		}
	}
	normalOffset := len(positions) * 4
	for _, value := range uvs {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write uv failed: %v", err)
		}
	}
	uvOffset := normalOffset + len(normals)*4
	for _, value := range indices {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write index failed: %v", err)
		}
	}
	indexOffset := uvOffset + len(uvs)*4
	if padding := buf.Len() % 4; padding != 0 {
		buf.Write(bytes.Repeat([]byte{0x00}, 4-padding))
	}
	targetOffset := buf.Len()
	for _, value := range targetPositions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write target position failed: %v", err)
		}
	}
	binChunk := buf.Bytes()

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
						"extras": map[string]any{
							"targetNames": []string{"びっくり右"},
						},
						"targets": []any{
							map[string]any{
								"POSITION": 4,
							},
						},
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "EyeIris_00_EYE",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{1.0, 1.0, 1.0, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": positionOffset,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": normalOffset,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": uvOffset,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": indexOffset,
				"byteLength": len(indices) * 2,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": targetOffset,
				"byteLength": len(targetPositions) * 4,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
			map[string]any{
				"bufferView":    4,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips": map[string]any{"node": 0},
					},
				},
				"expressions": map[string]any{
					"custom": map[string]any{
						"びっくり右": map[string]any{
							"morphTargetBinds": []any{
								map[string]any{
									"node":   1,
									"index":  0,
									"weight": 1.0,
								},
							},
						},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}

	smallRight, err := pmxModel.Morphs.GetByName("瞳小右")
	if err != nil || smallRight == nil {
		t.Fatalf("瞳小右 morph not found: err=%v", err)
	}
	smallGroup, err := pmxModel.Morphs.GetByName("瞳小")
	if err != nil || smallGroup == nil {
		t.Fatalf("瞳小 group morph not found: err=%v", err)
	}
	if smallGroup.MorphType != model.MORPH_TYPE_GROUP {
		t.Fatalf("瞳小 morph type mismatch: got=%d want=%d", smallGroup.MorphType, model.MORPH_TYPE_GROUP)
	}
	if len(smallGroup.Offsets) != 1 {
		t.Fatalf("瞳小 group offset count mismatch: got=%d want=1", len(smallGroup.Offsets))
	}
	groupOffset, ok := smallGroup.Offsets[0].(*model.GroupMorphOffset)
	if !ok || groupOffset == nil {
		t.Fatalf("瞳小 group offset type mismatch: got=%T", smallGroup.Offsets[0])
	}
	if groupOffset.MorphIndex != smallRight.Index() {
		t.Fatalf("瞳小 group target mismatch: got=%d want=%d", groupOffset.MorphIndex, smallRight.Index())
	}
	if math.Abs(groupOffset.MorphFactor-1.0) > 1e-6 {
		t.Fatalf("瞳小 group factor mismatch: got=%f want=1.0", groupOffset.MorphFactor)
	}
}

func TestVrmRepositoryLoadBuildsSplitMorphFromMorphPairSplitFallbackRules(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_expression_split_fallback.vrm")

	positions := []float32{
		1.0, 0.0, 0.0,
		-1.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		1.0, 0.0,
		0.5, 1.0,
	}
	indices := []uint16{0, 1, 2}
	targetPositions := []float32{
		0.0, 0.04, 0.0,
		0.0, 0.03, 0.0,
		0.0, 0.0, 0.0,
	}

	var buf bytes.Buffer
	for _, value := range positions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write position failed: %v", err)
		}
	}
	positionOffset := 0
	for _, value := range normals {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write normal failed: %v", err)
		}
	}
	normalOffset := len(positions) * 4
	for _, value := range uvs {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write uv failed: %v", err)
		}
	}
	uvOffset := normalOffset + len(normals)*4
	for _, value := range indices {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write index failed: %v", err)
		}
	}
	indexOffset := uvOffset + len(uvs)*4
	if padding := buf.Len() % 4; padding != 0 {
		buf.Write(bytes.Repeat([]byte{0x00}, 4-padding))
	}
	targetOffset := buf.Len()
	for _, value := range targetPositions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write target position failed: %v", err)
		}
	}
	binChunk := buf.Bytes()

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
						"extras": map[string]any{
							"targetNames": []string{"にこり"},
						},
						"targets": []any{
							map[string]any{
								"POSITION": 4,
							},
						},
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "FaceBrow_00_FACE",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{1.0, 1.0, 1.0, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": positionOffset,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": normalOffset,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": uvOffset,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": indexOffset,
				"byteLength": len(indices) * 2,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": targetOffset,
				"byteLength": len(targetPositions) * 4,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
			map[string]any{
				"bufferView":    4,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips": map[string]any{"node": 0},
					},
				},
				"expressions": map[string]any{
					"custom": map[string]any{
						"にこり": map[string]any{
							"morphTargetBinds": []any{
								map[string]any{
									"node":   1,
									"index":  0,
									"weight": 1.0,
								},
							},
						},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}

	splitRight, err := pmxModel.Morphs.GetByName("にこり右")
	if err != nil || splitRight == nil {
		t.Fatalf("split morph にこり右 not found: err=%v", err)
	}
	splitLeft, err := pmxModel.Morphs.GetByName("にこり左")
	if err != nil || splitLeft == nil {
		t.Fatalf("split morph にこり左 not found: err=%v", err)
	}
	if splitRight.MorphType != model.MORPH_TYPE_VERTEX {
		t.Fatalf("split right morph type mismatch: got=%d want=%d", splitRight.MorphType, model.MORPH_TYPE_VERTEX)
	}
	if splitLeft.MorphType != model.MORPH_TYPE_VERTEX {
		t.Fatalf("split left morph type mismatch: got=%d want=%d", splitLeft.MorphType, model.MORPH_TYPE_VERTEX)
	}
	if len(splitRight.Offsets) == 0 {
		t.Fatalf("split right offsets should not be empty")
	}
	if len(splitLeft.Offsets) == 0 {
		t.Fatalf("split left offsets should not be empty")
	}
	for _, rawOffset := range splitRight.Offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil {
			continue
		}
		vertex, err := pmxModel.Vertices.Get(offsetData.VertexIndex)
		if err != nil || vertex == nil {
			t.Fatalf("split right vertex not found: index=%d err=%v", offsetData.VertexIndex, err)
		}
		if vertex.Position.X >= 0 {
			t.Fatalf("split right should contain only negative X vertices: vertex=%d posX=%f", offsetData.VertexIndex, vertex.Position.X)
		}
	}
	for _, rawOffset := range splitLeft.Offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil {
			continue
		}
		vertex, err := pmxModel.Vertices.Get(offsetData.VertexIndex)
		if err != nil || vertex == nil {
			t.Fatalf("split left vertex not found: index=%d err=%v", offsetData.VertexIndex, err)
		}
		if vertex.Position.X <= 0 {
			t.Fatalf("split left should contain only positive X vertices: vertex=%d posX=%f", offsetData.VertexIndex, vertex.Position.X)
		}
	}
}

func TestVrmRepositoryLoadBuildsMaterialMorphFromVrm1MaterialColorBinds(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_expression_material_vrm1.vrm")

	positions := []float32{
		0.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		1.0, 0.0, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.0, 1.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}
	binChunk := buildInterleavedBinForMeshTest(t, positions, normals, uvs, indices)

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "body",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{0.6, 0.5, 0.4, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": 0,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": len(positions) * 4,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": (len(positions) + len(normals)) * 4,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": (len(positions) + len(normals) + len(uvs)) * 4,
				"byteLength": len(indices) * 2,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips": map[string]any{"node": 0},
					},
				},
				"expressions": map[string]any{
					"custom": map[string]any{
						"mat_only": map[string]any{
							"materialColorBinds": []any{
								map[string]any{
									"material":    0,
									"type":        "color",
									"targetValue": []float64{0.2, 0.3, 0.4, 0.8},
								},
							},
						},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}
	expressionMorph, err := pmxModel.Morphs.GetByName("mat_only")
	if err != nil || expressionMorph == nil {
		t.Fatalf("expression morph not found: err=%v", err)
	}
	if expressionMorph.MorphType != model.MORPH_TYPE_MATERIAL {
		t.Fatalf("expression morph type mismatch: got=%d want=%d", expressionMorph.MorphType, model.MORPH_TYPE_MATERIAL)
	}
	if len(expressionMorph.Offsets) != 1 {
		t.Fatalf("material morph offset count mismatch: got=%d want=1", len(expressionMorph.Offsets))
	}
	materialOffset, ok := expressionMorph.Offsets[0].(*model.MaterialMorphOffset)
	if !ok {
		t.Fatalf("material morph offset type mismatch: got=%T", expressionMorph.Offsets[0])
	}
	if materialOffset.MaterialIndex != 0 {
		t.Fatalf("material index mismatch: got=%d want=0", materialOffset.MaterialIndex)
	}
	if materialOffset.CalcMode != model.CALC_MODE_ADDITION {
		t.Fatalf("calc mode mismatch: got=%d want=%d", materialOffset.CalcMode, model.CALC_MODE_ADDITION)
	}
	if math.Abs(materialOffset.Diffuse.X+0.4) > 1e-6 ||
		math.Abs(materialOffset.Diffuse.Y+0.2) > 1e-6 ||
		math.Abs(materialOffset.Diffuse.Z-0.0) > 1e-6 ||
		math.Abs(materialOffset.Diffuse.W+0.2) > 1e-6 {
		t.Fatalf("unexpected diffuse delta: %+v", materialOffset.Diffuse)
	}
}

func TestVrmRepositoryLoadBuildsMaterialMorphFromVrm0MaterialValues(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_expression_material_vrm0.vrm")

	positions := []float32{
		0.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		1.0, 0.0, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.0, 1.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}
	binChunk := buildInterleavedBinForMeshTest(t, positions, normals, uvs, indices)

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRM"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "body",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{0.4, 0.4, 0.4, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": 0,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": len(positions) * 4,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": (len(positions) + len(normals)) * 4,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": (len(positions) + len(normals) + len(uvs)) * 4,
				"byteLength": len(indices) * 2,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
		},
		"extensions": map[string]any{
			"VRM": map[string]any{
				"humanoid": map[string]any{
					"humanBones": []any{
						map[string]any{"bone": "hips", "node": 0},
					},
				},
				"blendShapeMaster": map[string]any{
					"blendShapeGroups": []any{
						map[string]any{
							"name": "mat_v0",
							"materialValues": []any{
								map[string]any{
									"materialName": "body",
									"propertyName": "_Color",
									"targetValue":  []float64{0.1, 0.2, 0.3, 0.5},
								},
							},
						},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}
	expressionMorph, err := pmxModel.Morphs.GetByName("mat_v0")
	if err != nil || expressionMorph == nil {
		t.Fatalf("expression morph not found: err=%v", err)
	}
	if expressionMorph.MorphType != model.MORPH_TYPE_MATERIAL {
		t.Fatalf("expression morph type mismatch: got=%d want=%d", expressionMorph.MorphType, model.MORPH_TYPE_MATERIAL)
	}
	if len(expressionMorph.Offsets) != 1 {
		t.Fatalf("material morph offset count mismatch: got=%d want=1", len(expressionMorph.Offsets))
	}
	materialOffset, ok := expressionMorph.Offsets[0].(*model.MaterialMorphOffset)
	if !ok {
		t.Fatalf("material morph offset type mismatch: got=%T", expressionMorph.Offsets[0])
	}
	if materialOffset.MaterialIndex != 0 {
		t.Fatalf("material index mismatch: got=%d want=0", materialOffset.MaterialIndex)
	}
	if math.Abs(materialOffset.Diffuse.X+0.3) > 1e-6 ||
		math.Abs(materialOffset.Diffuse.Y+0.2) > 1e-6 ||
		math.Abs(materialOffset.Diffuse.Z+0.1) > 1e-6 ||
		math.Abs(materialOffset.Diffuse.W+0.5) > 1e-6 {
		t.Fatalf("unexpected diffuse delta: %+v", materialOffset.Diffuse)
	}
}

func TestVrmRepositoryLoadBuildsUvMorphFromVrm1TextureTransformBinds(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_expression_uv_vrm1.vrm")

	positions := []float32{
		0.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		1.0, 0.0, 0.0,
	}
	normals := []float32{
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.5, 0.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}
	binChunk := buildInterleavedBinForMeshTest(t, positions, normals, uvs, indices)

	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     1,
							"TEXCOORD_0": 2,
						},
						"indices":  3,
						"material": 0,
						"mode":     4,
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "body",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{1.0, 1.0, 1.0, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": 0,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": len(positions) * 4,
				"byteLength": len(normals) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": (len(positions) + len(normals)) * 4,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": (len(positions) + len(normals) + len(uvs)) * 4,
				"byteLength": len(indices) * 2,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    3,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips": map[string]any{"node": 0},
					},
				},
				"expressions": map[string]any{
					"custom": map[string]any{
						"uv_only": map[string]any{
							"textureTransformBinds": []any{
								map[string]any{
									"material": 0,
									"scale":    []float64{2.0, 1.0},
									"offset":   []float64{0.1, 0.0},
								},
							},
						},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}
	expressionMorph, err := pmxModel.Morphs.GetByName("uv_only")
	if err != nil || expressionMorph == nil {
		t.Fatalf("expression morph not found: err=%v", err)
	}
	if expressionMorph.MorphType != model.MORPH_TYPE_EXTENDED_UV1 {
		t.Fatalf("expression morph type mismatch: got=%d want=%d", expressionMorph.MorphType, model.MORPH_TYPE_EXTENDED_UV1)
	}
	if len(expressionMorph.Offsets) != 3 {
		t.Fatalf("uv morph offset count mismatch: got=%d want=3", len(expressionMorph.Offsets))
	}
	uvOffset, ok := expressionMorph.Offsets[0].(*model.UvMorphOffset)
	if !ok {
		t.Fatalf("uv morph offset type mismatch: got=%T", expressionMorph.Offsets[0])
	}
	if uvOffset.UvType != model.MORPH_TYPE_EXTENDED_UV1 {
		t.Fatalf("uv morph offset uvType mismatch: got=%d want=%d", uvOffset.UvType, model.MORPH_TYPE_EXTENDED_UV1)
	}
	if math.Abs(uvOffset.Uv.X-0.1) > 1e-6 || math.Abs(uvOffset.Uv.Y) > 1e-6 {
		t.Fatalf("unexpected uv delta: %+v", uvOffset.Uv)
	}
	for _, rawOffset := range expressionMorph.Offsets {
		offsetData, ok := rawOffset.(*model.UvMorphOffset)
		if !ok {
			continue
		}
		vertex, err := pmxModel.Vertices.Get(offsetData.VertexIndex)
		if err != nil || vertex == nil {
			t.Fatalf("vertex not found for uv offset: index=%d err=%v", offsetData.VertexIndex, err)
		}
		if len(vertex.ExtendedUvs) < 1 {
			t.Fatalf("vertex extended uv1 is not initialized: index=%d", offsetData.VertexIndex)
		}
	}
}

func TestVrmRepositoryLoadContinuesWhenNormalAccessorIsInvalid(t *testing.T) {
	repository := NewVrmRepository()
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "mesh_invalid_normal.vrm")

	positions := []float32{
		0.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		1.0, 0.0, 0.0,
	}
	uvs := []float32{
		0.0, 0.0,
		0.0, 1.0,
		1.0, 0.0,
	}
	indices := []uint16{0, 1, 2}

	binChunk := buildInterleavedBinForMeshTest(t, positions, nil, uvs, indices)
	doc := map[string]any{
		"asset": map[string]any{
			"version":   "2.0",
			"generator": "VRM Test",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name": "hips_node",
			},
			map[string]any{
				"name": "mesh_node",
				"mesh": 0,
				"skin": 0,
			},
		},
		"skins": []any{
			map[string]any{
				"joints": []int{0},
			},
		},
		"meshes": []any{
			map[string]any{
				"name": "mesh0",
				"primitives": []any{
					map[string]any{
						"attributes": map[string]any{
							"POSITION":   0,
							"NORMAL":     99,
							"TEXCOORD_0": 1,
						},
						"indices":  2,
						"material": 0,
						"mode":     4,
					},
				},
			},
		},
		"materials": []any{
			map[string]any{
				"name": "body",
				"pbrMetallicRoughness": map[string]any{
					"baseColorFactor": []float64{1.0, 1.0, 1.0, 1.0},
				},
			},
		},
		"buffers": []any{
			map[string]any{
				"byteLength": len(binChunk),
			},
		},
		"bufferViews": []any{
			map[string]any{
				"buffer":     0,
				"byteOffset": 0,
				"byteLength": len(positions) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": len(positions) * 4,
				"byteLength": len(uvs) * 4,
			},
			map[string]any{
				"buffer":     0,
				"byteOffset": (len(positions) + len(uvs)) * 4,
				"byteLength": len(indices) * 2,
			},
		},
		"accessors": []any{
			map[string]any{
				"bufferView":    0,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC3",
			},
			map[string]any{
				"bufferView":    1,
				"componentType": 5126,
				"count":         3,
				"type":          "VEC2",
			},
			map[string]any{
				"bufferView":    2,
				"componentType": 5123,
				"count":         3,
				"type":          "SCALAR",
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips": map[string]any{"node": 0},
					},
				},
			},
		},
	}
	writeGLBFileForUsecaseMeshTest(t, path, doc, binChunk)

	hashableModel, err := repository.Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		t.Fatalf("expected *model.PmxModel, got %T", hashableModel)
	}
	if pmxModel.Vertices.Len() != 3 {
		t.Fatalf("expected 3 vertices, got %d", pmxModel.Vertices.Len())
	}
	vertex, err := pmxModel.Vertices.Get(0)
	if err != nil {
		t.Fatalf("get vertex failed: %v", err)
	}
	if vertex == nil {
		t.Fatalf("vertex is nil")
	}
	if vertex.Normal.Y < 0.99 {
		t.Fatalf("expected fallback normal Y close to 1.0, got %f", vertex.Normal.Y)
	}
}

func TestShouldSkipPrimitiveForUnsupportedTargets(t *testing.T) {
	indices := 0
	material := 0
	mode := 4
	seen := map[string]int{}
	withTargets := gltfPrimitive{
		Attributes: map[string]int{"POSITION": 1},
		Indices:    &indices,
		Material:   &material,
		Mode:       &mode,
		Targets: []map[string]int{
			{"POSITION": 2},
		},
	}
	withoutTargets := gltfPrimitive{
		Attributes: map[string]int{"POSITION": 1},
		Indices:    &indices,
		Material:   &material,
		Mode:       &mode,
	}

	if shouldSkipPrimitiveForUnsupportedTargets(withTargets, 0, seen) {
		t.Fatalf("first target primitive should not be skipped")
	}
	if !shouldSkipPrimitiveForUnsupportedTargets(withTargets, 1, seen) {
		t.Fatalf("duplicated target primitive should be skipped")
	}
	if shouldSkipPrimitiveForUnsupportedTargets(withoutTargets, 2, seen) {
		t.Fatalf("primitive without targets should not be skipped")
	}
}

// writeGLBFileForTest はテスト用のJSONをGLBとして書き込む。
func writeGLBFileForTest(t *testing.T, path string, doc map[string]any) {
	t.Helper()
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	padSize := (4 - (len(jsonBytes) % 4)) % 4
	if padSize > 0 {
		jsonBytes = append(jsonBytes, bytes.Repeat([]byte(" "), padSize)...)
	}
	totalLength := uint32(12 + 8 + len(jsonBytes))

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(0x46546C67)); err != nil {
		t.Fatalf("write magic failed: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(2)); err != nil {
		t.Fatalf("write version failed: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, totalLength); err != nil {
		t.Fatalf("write length failed: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(jsonBytes))); err != nil {
		t.Fatalf("write chunk length failed: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(0x4E4F534A)); err != nil {
		t.Fatalf("write chunk type failed: %v", err)
	}
	if _, err := buf.Write(jsonBytes); err != nil {
		t.Fatalf("write chunk body failed: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
}

// writeGLBFileForTestWithBin はテスト用のJSON/BINをGLBとして書き込む。
func writeGLBFileForTestWithBin(t *testing.T, path string, doc map[string]any, binChunk []byte) {
	t.Helper()
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	jsonPadSize := (4 - (len(jsonBytes) % 4)) % 4
	if jsonPadSize > 0 {
		jsonBytes = append(jsonBytes, bytes.Repeat([]byte(" "), jsonPadSize)...)
	}
	binBytes := append([]byte(nil), binChunk...)
	if len(binBytes) > 0 {
		binPadSize := (4 - (len(binBytes) % 4)) % 4
		if binPadSize > 0 {
			binBytes = append(binBytes, bytes.Repeat([]byte{0x00}, binPadSize)...)
		}
	}

	totalLength := uint32(12 + 8 + len(jsonBytes))
	if len(binBytes) > 0 {
		totalLength += uint32(8 + len(binBytes))
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(0x46546C67)); err != nil {
		t.Fatalf("write magic failed: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(2)); err != nil {
		t.Fatalf("write version failed: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, totalLength); err != nil {
		t.Fatalf("write length failed: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(jsonBytes))); err != nil {
		t.Fatalf("write chunk length failed: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(0x4E4F534A)); err != nil {
		t.Fatalf("write json chunk type failed: %v", err)
	}
	if _, err := buf.Write(jsonBytes); err != nil {
		t.Fatalf("write json chunk body failed: %v", err)
	}
	if len(binBytes) > 0 {
		if err := binary.Write(&buf, binary.LittleEndian, uint32(len(binBytes))); err != nil {
			t.Fatalf("write bin chunk length failed: %v", err)
		}
		if err := binary.Write(&buf, binary.LittleEndian, uint32(0x004E4942)); err != nil {
			t.Fatalf("write bin chunk type failed: %v", err)
		}
		if _, err := buf.Write(binBytes); err != nil {
			t.Fatalf("write bin chunk body failed: %v", err)
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
}

// buildInterleavedBinForMeshTest はメッシュ検証用のBINチャンクを構築する。
func buildInterleavedBinForMeshTest(t *testing.T, positions []float32, normals []float32, uvs []float32, indices []uint16) []byte {
	t.Helper()
	var buf bytes.Buffer
	for _, value := range positions {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write position failed: %v", err)
		}
	}
	for _, value := range normals {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write normal failed: %v", err)
		}
	}
	for _, value := range uvs {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write uv failed: %v", err)
		}
	}
	for _, value := range indices {
		if err := binary.Write(&buf, binary.LittleEndian, value); err != nil {
			t.Fatalf("write index failed: %v", err)
		}
	}
	return buf.Bytes()
}

// writeGLBFileForUsecaseMeshTest はテスト用JSON/BINをGLBとして書き込む。
func writeGLBFileForUsecaseMeshTest(t *testing.T, path string, doc map[string]any, binChunk []byte) {
	t.Helper()
	writeGLBFileForTestWithBin(t, path, doc, binChunk)
}
