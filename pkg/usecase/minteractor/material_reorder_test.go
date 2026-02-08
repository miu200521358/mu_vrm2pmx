// 指示: miu200521358
package minteractor

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
	"gonum.org/v1/gonum/spatial/r3"
)

func TestApplyBodyDepthMaterialOrderReordersTransparentByBodyProximity(t *testing.T) {
	modelData := buildBodyDepthReorderModel()

	applyBodyDepthMaterialOrder(modelData)

	gotNames := materialNames(modelData)
	wantNames := []string{"body", "near", "far"}
	for i := range wantNames {
		if i >= len(gotNames) || gotNames[i] != wantNames[i] {
			t.Fatalf("material order mismatch: got=%v want=%v", gotNames, wantNames)
		}
	}

	faceNear, err := modelData.Faces.Get(1)
	if err != nil || faceNear == nil {
		t.Fatalf("near face missing: err=%v", err)
	}
	if faceNear.VertexIndexes != [3]int{6, 7, 8} {
		t.Fatalf("near face vertices mismatch: got=%v", faceNear.VertexIndexes)
	}

	faceFar, err := modelData.Faces.Get(2)
	if err != nil || faceFar == nil {
		t.Fatalf("far face missing: err=%v", err)
	}
	if faceFar.VertexIndexes != [3]int{3, 4, 5} {
		t.Fatalf("far face vertices mismatch: got=%v", faceFar.VertexIndexes)
	}

	for _, idx := range []int{3, 4, 5} {
		vertex, vErr := modelData.Vertices.Get(idx)
		if vErr != nil || vertex == nil {
			t.Fatalf("far vertex missing: idx=%d err=%v", idx, vErr)
		}
		if len(vertex.MaterialIndexes) == 0 || vertex.MaterialIndexes[0] != 2 {
			t.Fatalf("far vertex material index mismatch: idx=%d got=%v", idx, vertex.MaterialIndexes)
		}
	}
	for _, idx := range []int{6, 7, 8} {
		vertex, vErr := modelData.Vertices.Get(idx)
		if vErr != nil || vertex == nil {
			t.Fatalf("near vertex missing: idx=%d err=%v", idx, vErr)
		}
		if len(vertex.MaterialIndexes) == 0 || vertex.MaterialIndexes[0] != 1 {
			t.Fatalf("near vertex material index mismatch: idx=%d got=%v", idx, vertex.MaterialIndexes)
		}
	}
}

func TestHasTransparentTextureAlphaUsesThreshold(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "out", "model.pmx")
	texDir := filepath.Join(filepath.Dir(modelPath), "tex")
	if err := os.MkdirAll(texDir, 0o755); err != nil {
		t.Fatalf("mkdir tex failed: %v", err)
	}

	alphaBelowPath := filepath.Join(texDir, "below.png")
	if err := writeAlphaTexture(alphaBelowPath, 10); err != nil {
		t.Fatalf("write texture failed: %v", err)
	}
	alphaAbovePath := filepath.Join(texDir, "above.png")
	if err := writeAlphaTexture(alphaAbovePath, 16); err != nil {
		t.Fatalf("write texture failed: %v", err)
	}

	modelData := model.NewPmxModel()
	modelData.SetPath(modelPath)
	textureBelow := model.NewTexture()
	textureBelow.SetName(filepath.Join("tex", "below.png"))
	textureBelow.SetValid(true)
	textureAbove := model.NewTexture()
	textureAbove.SetName(filepath.Join("tex", "above.png"))
	textureAbove.SetValid(true)
	modelData.Textures.AppendRaw(textureBelow)
	modelData.Textures.AppendRaw(textureAbove)

	cache := map[int]textureAlphaCacheEntry{}
	if !hasTransparentTextureAlpha(modelData, 0, cache) {
		t.Fatalf("expected texture alpha <= 0.05 to be transparent")
	}
	if hasTransparentTextureAlpha(modelData, 1, cache) {
		t.Fatalf("expected texture alpha > 0.05 to be opaque in threshold check")
	}
}

func TestCollectBodyBoneIndexesFromHumanoidUsesNodeIndexes(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Bones.AppendRaw(newBone("hips"))
	modelData.Bones.AppendRaw(newBone("spine"))

	vrmData := vrm.NewVrmData()
	vrmData.Vrm1 = vrm.NewVrm1Data()
	vrmData.Vrm1.Humanoid.HumanBones["hips"] = vrm.Vrm1HumanBone{Node: 0}
	vrmData.Vrm1.Humanoid.HumanBones["spine"] = vrm.Vrm1HumanBone{Node: 1}
	vrmData.Vrm1.Humanoid.HumanBones["leftHand"] = vrm.Vrm1HumanBone{Node: 4}
	modelData.VrmData = vrmData

	bodyBones := collectBodyBoneIndexesFromHumanoid(modelData)
	if len(bodyBones) != 2 {
		t.Fatalf("body bone count mismatch: got=%d", len(bodyBones))
	}
	if _, ok := bodyBones[0]; !ok {
		t.Fatalf("hips node index should be included")
	}
	if _, ok := bodyBones[1]; !ok {
		t.Fatalf("spine node index should be included")
	}
}

// buildBodyDepthReorderModel は材質並べ替え検証用のモデルを構築する。
func buildBodyDepthReorderModel() *ModelData {
	modelData := model.NewPmxModel()

	modelData.Bones.AppendRaw(newBone("hips"))
	modelData.Bones.AppendRaw(newBone("other"))

	vrmData := vrm.NewVrmData()
	vrmData.Vrm1 = vrm.NewVrm1Data()
	vrmData.Vrm1.Humanoid.HumanBones["hips"] = vrm.Vrm1HumanBone{Node: 0}
	modelData.VrmData = vrmData

	appendVertex(modelData, vec3(0, 0, 0), 0, []int{0})
	appendVertex(modelData, vec3(0, 1, 0), 0, []int{0})
	appendVertex(modelData, vec3(1, 0, 0), 0, []int{0})

	appendVertex(modelData, vec3(8, 0, 0), 1, []int{1})
	appendVertex(modelData, vec3(8, 1, 0), 1, []int{1})
	appendVertex(modelData, vec3(9, 0, 0), 1, []int{1})

	appendVertex(modelData, vec3(2, 0, 0), 1, []int{2})
	appendVertex(modelData, vec3(2, 1, 0), 1, []int{2})
	appendVertex(modelData, vec3(3, 0, 0), 1, []int{2})

	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{0, 1, 2}})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{3, 4, 5}})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{6, 7, 8}})

	modelData.Materials.AppendRaw(newMaterial("body", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("far", 0.7, 3))
	modelData.Materials.AppendRaw(newMaterial("near", 0.7, 3))

	return modelData
}

// appendVertex は並べ替え検証用の頂点を追加する。
func appendVertex(modelData *ModelData, position mmath.Vec3, boneIndex int, materialIndexes []int) {
	vertex := &model.Vertex{
		Position:        position,
		Normal:          vec3(0, 1, 0),
		Uv:              mmath.ZERO_VEC2,
		ExtendedUvs:     []mmath.Vec4{},
		DeformType:      model.BDEF1,
		Deform:          model.NewBdef1(boneIndex),
		EdgeFactor:      1.0,
		MaterialIndexes: append([]int(nil), materialIndexes...),
	}
	modelData.Vertices.AppendRaw(vertex)
}

// newBone は検証用ボーンを生成する。
func newBone(name string) *model.Bone {
	bone := &model.Bone{}
	bone.SetName(name)
	return bone
}

// newMaterial は検証用材質を生成する。
func newMaterial(name string, alpha float64, verticesCount int) *model.Material {
	material := model.NewMaterial()
	material.SetName(name)
	material.EnglishName = name
	material.Diffuse = mmath.Vec4{X: 1, Y: 1, Z: 1, W: alpha}
	material.VerticesCount = verticesCount
	return material
}

// materialNames は材質名配列を返す。
func materialNames(modelData *ModelData) []string {
	names := make([]string, 0, modelData.Materials.Len())
	for _, materialData := range modelData.Materials.Values() {
		if materialData == nil {
			names = append(names, "")
			continue
		}
		names = append(names, materialData.Name())
	}
	return names
}

// writeAlphaTexture は指定アルファ値の1x1テクスチャを書き込む。
func writeAlphaTexture(path string, alpha uint8) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: alpha})
	return png.Encode(file, img)
}

// vec3 はテスト用のVec3を生成する。
func vec3(x, y, z float64) mmath.Vec3 {
	return mmath.Vec3{Vec: r3.Vec{X: x, Y: y, Z: z}}
}
