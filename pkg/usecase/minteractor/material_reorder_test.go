// 指示: miu200521358
package minteractor

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
	"github.com/miu200521358/mlib_go/pkg/infra/base/mlogging"
	"github.com/miu200521358/mlib_go/pkg/shared/base/logging"
	"gonum.org/v1/gonum/spatial/r3"
)

func TestApplyBodyDepthMaterialOrderReordersTransparentByBodyProximity(t *testing.T) {
	modelData := buildBodyDepthReorderModel()
	modelPath, err := prepareTransparentTestTextures(t, []string{"far.png", "near.png"})
	if err != nil {
		t.Fatalf("prepare textures failed: %v", err)
	}
	modelData.SetPath(modelPath)
	assignMaterialTextureIndex(modelData, 1, "far.png")
	assignMaterialTextureIndex(modelData, 2, "near.png")

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

func TestApplyBodyDepthMaterialOrderUsesDoubleSidedFallbackWhenAlphaDetectionFails(t *testing.T) {
	modelData := buildBodyDepthReorderModel()
	modelData.SetPath(filepath.Join(t.TempDir(), "model.pmx"))

	for i := 0; i < 3; i++ {
		texture := model.NewTexture()
		texture.SetName(filepath.Join("tex", "missing_texture.png"))
		texture.SetValid(true)
		modelData.Textures.AppendRaw(texture)
	}

	bodyMaterial, err := modelData.Materials.Get(0)
	if err != nil || bodyMaterial == nil {
		t.Fatalf("body material missing: err=%v", err)
	}
	bodyMaterial.TextureIndex = 0

	farMaterial, err := modelData.Materials.Get(1)
	if err != nil || farMaterial == nil {
		t.Fatalf("far material missing: err=%v", err)
	}
	farMaterial.TextureIndex = 1
	farMaterial.DrawFlag |= model.DRAW_FLAG_DOUBLE_SIDED_DRAWING

	nearMaterial, err := modelData.Materials.Get(2)
	if err != nil || nearMaterial == nil {
		t.Fatalf("near material missing: err=%v", err)
	}
	nearMaterial.TextureIndex = 2
	nearMaterial.DrawFlag |= model.DRAW_FLAG_DOUBLE_SIDED_DRAWING

	applyBodyDepthMaterialOrder(modelData)

	gotNames := materialNames(modelData)
	wantNames := []string{"body", "near", "far"}
	for i := range wantNames {
		if i >= len(gotNames) || gotNames[i] != wantNames[i] {
			t.Fatalf("material order mismatch: got=%v want=%v", gotNames, wantNames)
		}
	}
}

func TestBuildMaterialTransparencyScoresUsesFaceUvRegion(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "out", "model.pmx")
	texDir := filepath.Join(filepath.Dir(modelPath), "tex")
	if err := os.MkdirAll(texDir, 0o755); err != nil {
		t.Fatalf("mkdir tex failed: %v", err)
	}
	texPath := filepath.Join(texDir, "uv_alpha.png")
	if err := writeHalfAlphaTexture(texPath, 10, 255); err != nil {
		t.Fatalf("write texture failed: %v", err)
	}

	modelData := model.NewPmxModel()
	modelData.SetPath(modelPath)
	texture := model.NewTexture()
	texture.SetName(filepath.Join("tex", "uv_alpha.png"))
	texture.SetValid(true)
	modelData.Textures.AppendRaw(texture)

	modelData.Materials.AppendRaw(newMaterial("opaque_uv", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("transparent_uv", 1.0, 3))
	opaqueMaterial, _ := modelData.Materials.Get(0)
	transparentMaterial, _ := modelData.Materials.Get(1)
	opaqueMaterial.TextureIndex = 0
	transparentMaterial.TextureIndex = 0

	appendUvVertex(modelData, vec3(0, 0, 0), mmath.Vec2{X: 1.0, Y: 0.5}, 0, []int{0})
	appendUvVertex(modelData, vec3(1, 0, 0), mmath.Vec2{X: 1.0, Y: 0.5}, 0, []int{0})
	appendUvVertex(modelData, vec3(0, 1, 0), mmath.Vec2{X: 1.0, Y: 0.5}, 0, []int{0})
	appendUvVertex(modelData, vec3(0, 0, 1), mmath.Vec2{X: 0.0, Y: 0.5}, 0, []int{1})
	appendUvVertex(modelData, vec3(1, 0, 1), mmath.Vec2{X: 0.0, Y: 0.5}, 0, []int{1})
	appendUvVertex(modelData, vec3(0, 1, 1), mmath.Vec2{X: 0.0, Y: 0.5}, 0, []int{1})

	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{0, 1, 2}})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{3, 4, 5}})

	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil {
		t.Fatalf("build face ranges failed: %v", err)
	}
	scores := buildMaterialTransparencyScores(
		modelData,
		faceRanges,
		map[int]textureImageCacheEntry{},
		textureAlphaTransparentThreshold,
	)
	if scores[0] != 0 {
		t.Fatalf("opaque uv score should be 0: got=%f", scores[0])
	}
	if scores[1] <= 0 {
		t.Fatalf("transparent uv score should be >0: got=%f", scores[1])
	}
}

func TestAbbreviateMaterialNamesBeforeReorderAppliesPrefixSuffixRuleAndResolvesConflict(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("N00_000_00_Face_00_SKIN (Instance)", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("N00_001_00_Face_00_SKIN (Instance)", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("J_Sec_R_SkirtBack0_01", 1.0, 3))

	if err := abbreviateMaterialNamesBeforeReorder(modelData); err != nil {
		t.Fatalf("abbreviate material names failed: %v", err)
	}

	gotNames := materialNames(modelData)
	wantNames := []string{"Face_00_SKIN", "Face_00_SKIN_2", "RSkBc0_01"}
	for i := range wantNames {
		if i >= len(gotNames) || gotNames[i] != wantNames[i] {
			t.Fatalf("material names mismatch: got=%v want=%v", gotNames, wantNames)
		}
	}

	secondMaterial, err := modelData.Materials.Get(1)
	if err != nil || secondMaterial == nil {
		t.Fatalf("2nd material missing: err=%v", err)
	}
	if secondMaterial.EnglishName != "Face_00_SKIN_2" {
		t.Fatalf("expected EnglishName synced: got=%s", secondMaterial.EnglishName)
	}
	thirdMaterial, err := modelData.Materials.Get(2)
	if err != nil || thirdMaterial == nil {
		t.Fatalf("3rd material missing: err=%v", err)
	}
	if thirdMaterial.EnglishName != "RSkBc0_01" {
		t.Fatalf("expected EnglishName synced: got=%s", thirdMaterial.EnglishName)
	}
}

func TestAbbreviateMaterialNamesBeforeReorderSupportsAsciiSeparators(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("Hair-Back 01", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Face.Mouth", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("UpperBody (Instance)", 1.0, 3))

	if err := abbreviateMaterialNamesBeforeReorder(modelData); err != nil {
		t.Fatalf("abbreviate material names failed: %v", err)
	}

	gotNames := materialNames(modelData)
	wantNames := []string{"HrBc_01", "FcMt", "UpperBody"}
	for i := range wantNames {
		if i >= len(gotNames) || gotNames[i] != wantNames[i] {
			t.Fatalf("material names mismatch: got=%v want=%v", gotNames, wantNames)
		}
	}
}

func TestAbbreviateMaterialNamesBeforeReorderStripsVroidPrefixAndInstanceSuffix(t *testing.T) {
	type materialCase struct {
		name string
		want string
	}

	cases := []materialCase{
		{name: "N00_000_00_FaceMouth_00_FACE (Instance)", want: "FaceMouth_00_FACE"},
		{name: "N00_000_00_EyeIris_00_EYE (Instance)", want: "EyeIris_00_EYE"},
		{name: "N00_000_00_EyeHighlight_00_EYE (Instance)", want: "EyeHighlight_00_EYE"},
		{name: "N00_000_00_Face_00_SKIN (Instance)", want: "Face_00_SKIN"},
		{name: "N00_000_00_EyeWhite_00_EYE (Instance)", want: "EyeWhite_00_EYE"},
		{name: "N00_000_00_FaceBrow_00_FACE (Instance)", want: "FaceBrow_00_FACE"},
		{name: "N00_000_00_FaceEyeline_00_FACE (Instance)", want: "FaceEyeline_00_FACE"},
		{name: "N00_000_00_FaceEyelash_00_FACE (Instance)", want: "FaceEyelash_00_FACE"},
		{name: "N00_000_00_Body_00_SKIN (Instance)", want: "Body_00_SKIN"},
		{name: "N00_004_01_Shoes_01_CLOTH (Instance)", want: "Shoes_01_CLOTH"},
		{name: "N00_000_00_HairBack_00_HAIR (Instance)", want: "HairBack_00_HAIR"},
		{name: "N00_010_01_Onepiece_00_CLOTH (Instance)", want: "Onepiece_00_CLOTH"},
		{name: "N00_002_01_Tops_01_CLOTH_02 (Instance)", want: "Tops_01_CLOTH_02"},
		{name: "N00_002_01_Tops_01_CLOTH_01 (Instance)", want: "Tops_01_CLOTH_01"},
		{name: "N00_007_01_Tops_01_CLOTH (Instance)", want: "Tops_01_CLOTH"},
		{name: "N00_002_01_Tops_01_CLOTH_03 (Instance)", want: "Tops_01_CLOTH_03"},
		{name: "N00_000_Hair_00_HAIR_01 (Instance)", want: "Hair_00_HAIR_01"},
		{name: "N00_000_Hair_00_HAIR_02 (Instance)", want: "Hair_00_HAIR_02"},
		{name: "N00_000_Hair_00_HAIR_03 (Instance)", want: "Hair_00_HAIR_03"},
		{name: "N00_000_00_Body_00_SKIN_(なし) (Instance)", want: "Body_00_SKIN_表面"},
		{name: "N00_000_00_Body_00_SKIN-裏面 (Instance)", want: "Body_00_SKIN_裏面"},
		{name: "N00_000_00_Body_00_SKIN エッジ (Instance)", want: "Body_00_SKIN_エッジ"},
		{name: "N00_010_01_Onepiece_00_CLOTH (Instance)_表面", want: "Onepiece_00_CLOTH_表面"},
		{name: "Face_00_SKIN_裏面", want: "Face_00_SKIN_裏面"},
	}

	modelData := model.NewPmxModel()
	for _, c := range cases {
		modelData.Materials.AppendRaw(newMaterial(c.name, 1.0, 3))
	}

	if err := abbreviateMaterialNamesBeforeReorder(modelData); err != nil {
		t.Fatalf("abbreviate material names failed: %v", err)
	}

	gotNames := materialNames(modelData)
	if len(gotNames) != len(cases) {
		t.Fatalf("material count mismatch: got=%d want=%d names=%v", len(gotNames), len(cases), gotNames)
	}
	for i, c := range cases {
		if gotNames[i] != c.want {
			t.Fatalf("material names mismatch at %d: got=%q want=%q source=%q", i, gotNames[i], c.want, c.name)
		}
	}
}

func TestPrepareVroidMaterialVariantsBeforeReorderNormalizesLegacySuffixes(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("Body_00_SKIN_(なし)", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Hair_00_HAIR-エッジ", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_00_CLOTH 裏面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Face_00_SKIN_表面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("目光なし", 1.0, 3))

	if err := prepareVroidMaterialVariantsBeforeReorder(modelData); err != nil {
		t.Fatalf("prepare vroid material variants failed: %v", err)
	}

	gotNames := materialNames(modelData)
	wantNames := []string{
		"Body_00_SKIN_表面",
		"Hair_00_HAIR_エッジ",
		"Tops_00_CLOTH_裏面",
		"Face_00_SKIN_表面",
		"目光なし",
	}
	for i := range wantNames {
		if i >= len(gotNames) || gotNames[i] != wantNames[i] {
			t.Fatalf("material names mismatch: got=%v want=%v", gotNames, wantNames)
		}
	}
}

func TestPrepareVroidMaterialVariantsBeforeReorderDuplicatesBlendMaterial(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "out", "model.pmx")
	texDir := filepath.Join(filepath.Dir(modelPath), "tex")
	if err := os.MkdirAll(texDir, 0o755); err != nil {
		t.Fatalf("mkdir tex failed: %v", err)
	}
	if err := writeAlphaTexture(filepath.Join(texDir, "cloth.png"), 10); err != nil {
		t.Fatalf("write texture failed: %v", err)
	}

	modelData := model.NewPmxModel()
	modelData.SetPath(modelPath)
	texture := model.NewTexture()
	texture.SetName(filepath.Join("tex", "cloth.png"))
	texture.SetValid(true)
	textureIndex := modelData.Textures.AppendRaw(texture)

	materialData := newMaterial("Tops_01_CLOTH", 1.0, 3)
	materialData.TextureIndex = textureIndex
	materialData.Edge = mmath.Vec4{X: 0.2, Y: 0.3, Z: 0.4, W: 1.0}
	materialData.Specular = mmath.Vec4{X: 0.1, Y: 0.2, Z: 0.3, W: 0.4}
	materialData.EdgeSize = 1.2
	materialData.Memo = "VRM primitive alphaMode=BLEND"
	materialData.DrawFlag = model.DRAW_FLAG_DOUBLE_SIDED_DRAWING | model.DRAW_FLAG_DRAWING_EDGE
	modelData.Materials.AppendRaw(materialData)

	appendUvVertex(modelData, vec3(0, 0, 0), mmath.Vec2{X: 0.1, Y: 0.1}, 0, []int{0})
	appendUvVertex(modelData, vec3(1, 0, 0), mmath.Vec2{X: 0.2, Y: 0.1}, 0, []int{0})
	appendUvVertex(modelData, vec3(0, 1, 0), mmath.Vec2{X: 0.1, Y: 0.2}, 0, []int{0})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{0, 1, 2}})

	if err := prepareVroidMaterialVariantsBeforeReorder(modelData); err != nil {
		t.Fatalf("prepare vroid material variants failed: %v", err)
	}

	gotNames := materialNames(modelData)
	wantNames := []string{"Tops_01_CLOTH_表面", "Tops_01_CLOTH_裏面", "Tops_01_CLOTH_エッジ"}
	if len(gotNames) != len(wantNames) {
		t.Fatalf("material count mismatch: got=%d want=%d names=%v", len(gotNames), len(wantNames), gotNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("material names mismatch: got=%v want=%v", gotNames, wantNames)
		}
	}
	if modelData.Vertices.Len() != 9 {
		t.Fatalf("vertex count mismatch: got=%d want=9", modelData.Vertices.Len())
	}
	if modelData.Faces.Len() != 3 {
		t.Fatalf("face count mismatch: got=%d want=3", modelData.Faces.Len())
	}
	frontFace, err := modelData.Faces.Get(0)
	if err != nil || frontFace == nil {
		t.Fatalf("front face missing: err=%v", err)
	}
	if frontFace.VertexIndexes != [3]int{0, 1, 2} {
		t.Fatalf("front face vertices mismatch: got=%v want=[0 1 2]", frontFace.VertexIndexes)
	}
	backFace, err := modelData.Faces.Get(1)
	if err != nil || backFace == nil {
		t.Fatalf("back face missing: err=%v", err)
	}
	if backFace.VertexIndexes != [3]int{5, 4, 3} {
		t.Fatalf("back face vertices mismatch: got=%v want=[5 4 3]", backFace.VertexIndexes)
	}
	edgeFace, err := modelData.Faces.Get(2)
	if err != nil || edgeFace == nil {
		t.Fatalf("edge face missing: err=%v", err)
	}
	if edgeFace.VertexIndexes != [3]int{8, 7, 6} {
		t.Fatalf("edge face vertices mismatch: got=%v want=[8 7 6]", edgeFace.VertexIndexes)
	}

	frontMaterial, err := modelData.Materials.Get(0)
	if err != nil || frontMaterial == nil {
		t.Fatalf("front material missing: err=%v", err)
	}
	if (frontMaterial.DrawFlag & model.DRAW_FLAG_DOUBLE_SIDED_DRAWING) != 0 {
		t.Fatalf("front material should disable double sided flag: flag=%d", frontMaterial.DrawFlag)
	}
	if (frontMaterial.DrawFlag & model.DRAW_FLAG_DRAWING_EDGE) != 0 {
		t.Fatalf("front material should disable edge flag: flag=%d", frontMaterial.DrawFlag)
	}
	if frontMaterial.VerticesCount != 3 {
		t.Fatalf("front material vertices count mismatch: got=%d want=3", frontMaterial.VerticesCount)
	}
	if !frontMaterial.Specular.NearEquals(materialData.Specular, 1e-9) {
		t.Fatalf("front material specular mismatch: got=%v want=%v", frontMaterial.Specular, materialData.Specular)
	}

	backMaterial, err := modelData.Materials.Get(1)
	if err != nil || backMaterial == nil {
		t.Fatalf("back material missing: err=%v", err)
	}
	if (backMaterial.DrawFlag & model.DRAW_FLAG_DOUBLE_SIDED_DRAWING) != 0 {
		t.Fatalf("back material should disable double sided flag: flag=%d", backMaterial.DrawFlag)
	}
	if (backMaterial.DrawFlag & model.DRAW_FLAG_DRAWING_EDGE) != 0 {
		t.Fatalf("back material should disable edge flag: flag=%d", backMaterial.DrawFlag)
	}
	if !backMaterial.Specular.NearEquals(materialData.Specular, 1e-9) {
		t.Fatalf("back material specular mismatch: got=%v want=%v", backMaterial.Specular, materialData.Specular)
	}

	edgeMaterial, err := modelData.Materials.Get(2)
	if err != nil || edgeMaterial == nil {
		t.Fatalf("edge material missing: err=%v", err)
	}
	if (edgeMaterial.DrawFlag & model.DRAW_FLAG_DOUBLE_SIDED_DRAWING) != 0 {
		t.Fatalf("edge material should disable double sided flag: flag=%d", edgeMaterial.DrawFlag)
	}
	if (edgeMaterial.DrawFlag & model.DRAW_FLAG_DRAWING_EDGE) != 0 {
		t.Fatalf("edge material should disable edge flag: flag=%d", edgeMaterial.DrawFlag)
	}
	if edgeMaterial.VerticesCount != 3 {
		t.Fatalf("edge material vertices count mismatch: got=%d want=3", edgeMaterial.VerticesCount)
	}
	if edgeMaterial.Diffuse.X != 0.2 || edgeMaterial.Diffuse.Y != 0.3 || edgeMaterial.Diffuse.Z != 0.4 {
		t.Fatalf("edge material diffuse mismatch: got=%v", edgeMaterial.Diffuse)
	}
	if !edgeMaterial.Specular.NearEquals(materialData.Specular, 1e-9) {
		t.Fatalf("edge material specular mismatch: got=%v want=%v", edgeMaterial.Specular, materialData.Specular)
	}
}

func TestPrepareVroidMaterialVariantsBeforeReorderAppliesScaleFloorForTinyEdgeSize(t *testing.T) {
	modelData := newBlendClothVariantModelForEdgeTest(
		t,
		0.0001,
		[3]mmath.Vec3{
			vec3(0, 0, 0),
			vec3(1, 0, 0),
			vec3(0, 1, 0),
		},
	)
	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil {
		t.Fatalf("build material face ranges failed: %v", err)
	}
	oldFaces := append([]*model.Face(nil), modelData.Faces.Values()...)
	modelScale := resolveModelBoundingDiagonal(modelData.Vertices.Values())
	materialScale := resolveMaterialBoundingDiagonal(modelData, oldFaces, faceRanges[0])
	edgeSizeOffset := 0.0001 * edgeVariantBaseScaleFactor
	expectedFloor := clampFloat64(
		math.Max(modelScale*edgeVariantModelFloorRatio, materialScale*edgeVariantMaterialFloorRatio),
		edgeVariantFloorAbsMin,
		edgeVariantFloorAbsMax,
	)
	if edgeSizeOffset >= expectedFloor {
		t.Fatalf("expected scale floor to dominate: edgeSizeOffset=%f scaleFloor=%f", edgeSizeOffset, expectedFloor)
	}

	if err := prepareVroidMaterialVariantsBeforeReorder(modelData); err != nil {
		t.Fatalf("prepare vroid material variants failed: %v", err)
	}

	sourceVertex, err := modelData.Vertices.Get(0)
	if err != nil || sourceVertex == nil {
		t.Fatalf("source vertex missing: err=%v", err)
	}
	edgeVertex, err := modelData.Vertices.Get(6)
	if err != nil || edgeVertex == nil {
		t.Fatalf("edge vertex missing: err=%v", err)
	}
	gotOffset := edgeVertex.Position.Subed(sourceVertex.Position).Length()
	if math.Abs(gotOffset-expectedFloor) > 1e-9 {
		t.Fatalf("edge offset mismatch: got=%f want=%f", gotOffset, expectedFloor)
	}
}

func TestNewEdgeVariantDuplicateContextUsesMemoFloorCoefficients(t *testing.T) {
	modelData := newBlendClothVariantModelForEdgeTest(
		t,
		0.0001,
		[3]mmath.Vec3{
			vec3(0, 0, 0),
			vec3(1, 0, 0),
			vec3(0, 1, 0),
		},
	)
	materialData, err := modelData.Materials.Get(0)
	if err != nil || materialData == nil {
		t.Fatalf("material missing: err=%v", err)
	}
	materialData.Memo += " k_model_floor=0.001 k_material_floor=0.002"
	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil {
		t.Fatalf("build material face ranges failed: %v", err)
	}
	oldFaces := append([]*model.Face(nil), modelData.Faces.Values()...)
	modelScale := resolveModelBoundingDiagonal(modelData.Vertices.Values())
	materialScale := resolveMaterialBoundingDiagonal(modelData, oldFaces, faceRanges[0])

	ctx := newEdgeVariantDuplicateContext(modelData, oldFaces, faceRanges[0], 0, materialData, modelScale)
	if ctx == nil {
		t.Fatal("context should not be nil")
	}
	expectedFloor := clampFloat64(
		math.Max(modelScale*0.001, materialScale*0.002),
		edgeVariantFloorAbsMin,
		edgeVariantFloorAbsMax,
	)
	if math.Abs(ctx.scaleFloor-expectedFloor) > 1e-9 {
		t.Fatalf("scale floor mismatch: got=%f want=%f", ctx.scaleFloor, expectedFloor)
	}
	if ctx.stats == nil {
		t.Fatal("stats should not be nil")
	}
	if math.Abs(ctx.stats.kModelFloor-0.001) > 1e-9 || math.Abs(ctx.stats.kMaterialFloor-0.002) > 1e-9 {
		t.Fatalf(
			"coefficient mismatch: got(k_model_floor=%f,k_material_floor=%f)",
			ctx.stats.kModelFloor,
			ctx.stats.kMaterialFloor,
		)
	}
}

func TestNewEdgeVariantDuplicateContextAppliesTwoStageGuard(t *testing.T) {
	modelData := newBlendClothVariantModelForEdgeTest(
		t,
		0.0001,
		[3]mmath.Vec3{
			vec3(0, 0, 0),
			vec3(1, 0, 0),
			vec3(0, 1, 0),
		},
	)
	materialData, err := modelData.Materials.Get(0)
	if err != nil || materialData == nil {
		t.Fatalf("material missing: err=%v", err)
	}
	materialData.Memo += " edge_offset_mode=two_stage target_abs_floor=0.009 target_scale_factor=0 max_guard_delta=0.05"
	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil {
		t.Fatalf("build material face ranges failed: %v", err)
	}
	oldFaces := append([]*model.Face(nil), modelData.Faces.Values()...)
	modelScale := resolveModelBoundingDiagonal(modelData.Vertices.Values())
	materialScale := resolveMaterialBoundingDiagonal(modelData, oldFaces, faceRanges[0])

	ctx := newEdgeVariantDuplicateContext(modelData, oldFaces, faceRanges[0], 0, materialData, modelScale)
	if ctx == nil {
		t.Fatal("context should not be nil")
	}
	expectedScaleFloor := clampFloat64(
		math.Max(modelScale*edgeVariantModelFloorRatio, materialScale*edgeVariantMaterialFloorRatio),
		edgeVariantFloorAbsMin,
		edgeVariantFloorAbsMax,
	)
	expectedBaseOffset := math.Max(materialData.EdgeSize*edgeVariantBaseScaleFactor, expectedScaleFloor)
	expectedGuardDelta := 0.009 - expectedBaseOffset
	if expectedGuardDelta < 0 {
		expectedGuardDelta = 0
	}
	if math.Abs(ctx.baseOffset-expectedBaseOffset) > 1e-9 {
		t.Fatalf("base offset mismatch: got=%f want=%f", ctx.baseOffset, expectedBaseOffset)
	}
	if !ctx.guardTriggeredByP50 || !ctx.guardTriggeredByP95 {
		t.Fatalf(
			"guard should be triggered for both p50/p95: got(p50=%t p95=%t)",
			ctx.guardTriggeredByP50,
			ctx.guardTriggeredByP95,
		)
	}
	if math.Abs(ctx.guardDelta-expectedGuardDelta) > 1e-9 {
		t.Fatalf("guard delta mismatch: got=%f want=%f", ctx.guardDelta, expectedGuardDelta)
	}
	if math.Abs(ctx.finalOffset-0.009) > 1e-9 {
		t.Fatalf("final offset mismatch: got=%f want=0.009", ctx.finalOffset)
	}
}

func TestNewEdgeVariantDuplicateContextKeepsLegacyModeWithoutGuard(t *testing.T) {
	modelData := newBlendClothVariantModelForEdgeTest(
		t,
		0.0001,
		[3]mmath.Vec3{
			vec3(0, 0, 0),
			vec3(1, 0, 0),
			vec3(0, 1, 0),
		},
	)
	materialData, err := modelData.Materials.Get(0)
	if err != nil || materialData == nil {
		t.Fatalf("material missing: err=%v", err)
	}
	materialData.Memo += " edge_offset_mode=legacy target_abs_floor=0.02 target_scale_factor=0 max_guard_delta=0.05"
	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil {
		t.Fatalf("build material face ranges failed: %v", err)
	}
	oldFaces := append([]*model.Face(nil), modelData.Faces.Values()...)
	modelScale := resolveModelBoundingDiagonal(modelData.Vertices.Values())

	ctx := newEdgeVariantDuplicateContext(modelData, oldFaces, faceRanges[0], 0, materialData, modelScale)
	if ctx == nil {
		t.Fatal("context should not be nil")
	}
	if ctx.guardDelta != 0 {
		t.Fatalf("legacy mode should not apply guard delta: got=%f", ctx.guardDelta)
	}
	if math.Abs(ctx.finalOffset-ctx.baseOffset) > 1e-9 {
		t.Fatalf("legacy mode final offset mismatch: got=%f want=%f", ctx.finalOffset, ctx.baseOffset)
	}
	if ctx.stats == nil || ctx.stats.offsetMode != edgeVariantOffsetModeLegacy {
		t.Fatalf("offset mode mismatch: got=%v want=legacy", ctx.stats.offsetMode)
	}
}

func TestNewEdgeVariantDuplicateContextWarnsAndContinuesOnInvalidMemoValues(t *testing.T) {
	logger := mlogging.NewLogger(nil)
	logger.SetLevel(logging.LOG_LEVEL_INFO)
	logger.MessageBuffer().Clear()
	prevLogger := logging.DefaultLogger()
	logging.SetDefaultLogger(logger)
	t.Cleanup(func() {
		logging.SetDefaultLogger(prevLogger)
	})

	modelData := newBlendClothVariantModelForEdgeTest(
		t,
		0.0001,
		[3]mmath.Vec3{
			vec3(0, 0, 0),
			vec3(1, 0, 0),
			vec3(0, 1, 0),
		},
	)
	materialData, err := modelData.Materials.Get(0)
	if err != nil || materialData == nil {
		t.Fatalf("material missing: err=%v", err)
	}
	materialData.Memo += " edge_offset_mode=two_stage k_model_floor=-0.1 k_material_floor=abc target_abs_floor=-1 max_guard_delta=nan"
	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil {
		t.Fatalf("build material face ranges failed: %v", err)
	}
	oldFaces := append([]*model.Face(nil), modelData.Faces.Values()...)
	modelScale := resolveModelBoundingDiagonal(modelData.Vertices.Values())

	ctx := newEdgeVariantDuplicateContext(modelData, oldFaces, faceRanges[0], 0, materialData, modelScale)
	if ctx == nil || ctx.stats == nil {
		t.Fatal("context stats should not be nil")
	}
	if ctx.stats.warningCount < 4 {
		t.Fatalf("warning count mismatch: got=%d want>=4", ctx.stats.warningCount)
	}
	if ctx.finalOffset <= 0 {
		t.Fatalf("final offset should remain positive: got=%f", ctx.finalOffset)
	}

	lines := logger.MessageBuffer().Lines()
	hasWarningLine := false
	for _, line := range lines {
		if strings.Contains(line, "エッジ押し出し設定警告:") {
			hasWarningLine = true
			break
		}
	}
	if !hasWarningLine {
		t.Fatal("warning log should be emitted for invalid memo values")
	}
}

func TestLogEdgeVariantOffsetStatsIncludesGuardAndMaxFields(t *testing.T) {
	logger := mlogging.NewLogger(nil)
	logger.SetLevel(logging.LOG_LEVEL_INFO)
	logger.MessageBuffer().Clear()
	prevLogger := logging.DefaultLogger()
	logging.SetDefaultLogger(logger)
	t.Cleanup(func() {
		logging.SetDefaultLogger(prevLogger)
	})

	stats := &edgeVariantOffsetLogStats{
		materialIndex:           3,
		materialName:            "Tops_01_CLOTH_エッジ",
		offsetMode:              edgeVariantOffsetModeTwoStage,
		modelScale:              24.360183926,
		materialScale:           13.773624070,
		kModelFloor:             2.0e-4,
		kMaterialFloor:          5.0e-4,
		targetAbsFloor:          0.009,
		targetScaleFactor:       6.5e-4,
		maxGuardDelta:           0.02,
		targetVertexCount:       3,
		edgeSizeOffsetSamples:   []float64{0.0002, 0.0002, 0.0002},
		scaleFloorSamples:       []float64{0.0068, 0.0068, 0.0068},
		baseOffsetSamples:       []float64{0.0068, 0.0068, 0.0068},
		targetFloorSamples:      []float64{0.009, 0.009, 0.009},
		guardDeltaSamples:       []float64{0.0022, 0.0022, 0.0022},
		finalOffsetSamples:      []float64{0.009, 0.009, 0.009},
		edgeSizeSelectedCount:   0,
		scaleFloorSelectedCount: 3,
		guardSelectedCount:      3,
		guardP50TriggerCount:    3,
		guardP95TriggerCount:    3,
		coincidentCount:         0,
		normalZeroCount:         0,
		degenerateFaceCount:     0,
		vertexNormalPathCount:   3,
		faceNormalPathCount:     0,
		defaultNormalPathCount:  0,
		outlineWidthMode:        "worldCoordinates",
		outlineWidthFactor:      0.0008,
		hasOutlineWidthFactor:   true,
	}
	logEdgeVariantOffsetStats(stats)

	lines := logger.MessageBuffer().Lines()
	hasExpectedLine := false
	for _, line := range lines {
		if !strings.Contains(line, "エッジ押し出し統計:") {
			continue
		}
		hasExpectedLine = true
		if !strings.Contains(line, "mode=two_stage") {
			t.Fatalf("mode field missing: line=%s", line)
		}
		if !strings.Contains(line, "final_offset[min=") || !strings.Contains(line, "max=") {
			t.Fatalf("max summary field missing: line=%s", line)
		}
		if !strings.Contains(line, "guard_delta[min=") {
			t.Fatalf("guard summary field missing: line=%s", line)
		}
		if !strings.Contains(line, "coef[k_model_floor=") {
			t.Fatalf("coefficient summary field missing: line=%s", line)
		}
		break
	}
	if !hasExpectedLine {
		t.Fatal("edge offset stats log should be emitted")
	}
}

func TestPrepareVroidMaterialVariantsBeforeReorderUsesFaceNormalWhenVertexNormalIsZero(t *testing.T) {
	modelData := newBlendClothVariantModelForEdgeTest(
		t,
		1.0,
		[3]mmath.Vec3{
			vec3(0, 0, 0),
			vec3(1, 0, 0),
			vec3(0, 1, 0),
		},
	)
	for vertexIndex := 0; vertexIndex < 3; vertexIndex++ {
		vertexData, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertexData == nil {
			t.Fatalf("vertex missing: index=%d err=%v", vertexIndex, err)
		}
		vertexData.Normal = mmath.ZERO_VEC3
	}

	if err := prepareVroidMaterialVariantsBeforeReorder(modelData); err != nil {
		t.Fatalf("prepare vroid material variants failed: %v", err)
	}

	sourceVertex, err := modelData.Vertices.Get(0)
	if err != nil || sourceVertex == nil {
		t.Fatalf("source vertex missing: err=%v", err)
	}
	edgeVertex, err := modelData.Vertices.Get(6)
	if err != nil || edgeVertex == nil {
		t.Fatalf("edge vertex missing: err=%v", err)
	}
	offset := edgeVertex.Position.Subed(sourceVertex.Position)
	if math.Abs(offset.Length()-0.02) > 1e-9 {
		t.Fatalf("edge offset length mismatch: got=%f want=0.020000000", offset.Length())
	}
	if math.Abs(offset.Z) <= 1e-9 {
		t.Fatalf("face normal fallback should move along Z: offset=%v", offset)
	}
	if math.Abs(offset.Y) > 1e-9 {
		t.Fatalf("face normal fallback should not move along Y: offset=%v", offset)
	}
}

func TestPrepareVroidMaterialVariantsBeforeReorderUsesDefaultNormalWhenFaceDegenerates(t *testing.T) {
	modelData := newBlendClothVariantModelForEdgeTest(
		t,
		1.0,
		[3]mmath.Vec3{
			vec3(0, 0, 0),
			vec3(1, 0, 0),
			vec3(2, 0, 0),
		},
	)
	for vertexIndex := 0; vertexIndex < 3; vertexIndex++ {
		vertexData, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertexData == nil {
			t.Fatalf("vertex missing: index=%d err=%v", vertexIndex, err)
		}
		vertexData.Normal = mmath.ZERO_VEC3
	}

	if err := prepareVroidMaterialVariantsBeforeReorder(modelData); err != nil {
		t.Fatalf("prepare vroid material variants failed: %v", err)
	}

	sourceVertex, err := modelData.Vertices.Get(0)
	if err != nil || sourceVertex == nil {
		t.Fatalf("source vertex missing: err=%v", err)
	}
	edgeVertex, err := modelData.Vertices.Get(6)
	if err != nil || edgeVertex == nil {
		t.Fatalf("edge vertex missing: err=%v", err)
	}
	offset := edgeVertex.Position.Subed(sourceVertex.Position)
	if math.Abs(offset.Length()-0.02) > 1e-9 {
		t.Fatalf("edge offset length mismatch: got=%f want=0.020000000", offset.Length())
	}
	if math.Abs(offset.Y) <= 1e-9 {
		t.Fatalf("default normal fallback should move along Y: offset=%v", offset)
	}
	if math.Abs(offset.X) > 1e-9 || math.Abs(offset.Z) > 1e-9 {
		t.Fatalf("default normal fallback should only move along Y: offset=%v", offset)
	}
}

func TestPrepareVroidMaterialVariantsBeforeReorderDuplicatesMaskMaterial(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "out", "model.pmx")
	texDir := filepath.Join(filepath.Dir(modelPath), "tex")
	if err := os.MkdirAll(texDir, 0o755); err != nil {
		t.Fatalf("mkdir tex failed: %v", err)
	}
	if err := writeAlphaTexture(filepath.Join(texDir, "cloth.png"), 10); err != nil {
		t.Fatalf("write texture failed: %v", err)
	}

	modelData := model.NewPmxModel()
	modelData.SetPath(modelPath)
	texture := model.NewTexture()
	texture.SetName(filepath.Join("tex", "cloth.png"))
	texture.SetValid(true)
	textureIndex := modelData.Textures.AppendRaw(texture)

	materialData := newMaterial("Tops_01_CLOTH", 1.0, 3)
	materialData.TextureIndex = textureIndex
	materialData.Edge = mmath.Vec4{X: 0.2, Y: 0.3, Z: 0.4, W: 1.0}
	materialData.Specular = mmath.Vec4{X: 0.4, Y: 0.3, Z: 0.2, W: 0.1}
	materialData.EdgeSize = 1.2
	materialData.Memo = "VRM primitive alphaMode=MASK"
	materialData.DrawFlag = model.DRAW_FLAG_DOUBLE_SIDED_DRAWING | model.DRAW_FLAG_DRAWING_EDGE
	modelData.Materials.AppendRaw(materialData)

	appendUvVertex(modelData, vec3(0, 0, 0), mmath.Vec2{X: 0.1, Y: 0.1}, 0, []int{0})
	appendUvVertex(modelData, vec3(1, 0, 0), mmath.Vec2{X: 0.2, Y: 0.1}, 0, []int{0})
	appendUvVertex(modelData, vec3(0, 1, 0), mmath.Vec2{X: 0.1, Y: 0.2}, 0, []int{0})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{0, 1, 2}})

	if err := prepareVroidMaterialVariantsBeforeReorder(modelData); err != nil {
		t.Fatalf("prepare vroid material variants failed: %v", err)
	}

	gotNames := materialNames(modelData)
	wantNames := []string{"Tops_01_CLOTH_表面", "Tops_01_CLOTH_裏面", "Tops_01_CLOTH_エッジ"}
	if len(gotNames) != len(wantNames) {
		t.Fatalf("material count mismatch: got=%d want=%d names=%v", len(gotNames), len(wantNames), gotNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("material names mismatch: got=%v want=%v", gotNames, wantNames)
		}
		materialEntry, err := modelData.Materials.Get(i)
		if err != nil || materialEntry == nil {
			t.Fatalf("material not found: index=%d err=%v", i, err)
		}
		if (materialEntry.DrawFlag & model.DRAW_FLAG_DOUBLE_SIDED_DRAWING) != 0 {
			t.Fatalf("material should disable double sided flag: index=%d flag=%d", i, materialEntry.DrawFlag)
		}
		if (materialEntry.DrawFlag & model.DRAW_FLAG_DRAWING_EDGE) != 0 {
			t.Fatalf("material should disable edge flag: index=%d flag=%d", i, materialEntry.DrawFlag)
		}
		if !materialEntry.Specular.NearEquals(materialData.Specular, 1e-9) {
			t.Fatalf("material specular mismatch: index=%d got=%v want=%v", i, materialEntry.Specular, materialData.Specular)
		}
	}
}

func TestPrepareVroidMaterialVariantsBeforeReorderSkipsOpaqueMaterial(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "out", "model.pmx")
	texDir := filepath.Join(filepath.Dir(modelPath), "tex")
	if err := os.MkdirAll(texDir, 0o755); err != nil {
		t.Fatalf("mkdir tex failed: %v", err)
	}
	if err := writeAlphaTexture(filepath.Join(texDir, "cloth.png"), 10); err != nil {
		t.Fatalf("write texture failed: %v", err)
	}

	modelData := model.NewPmxModel()
	modelData.SetPath(modelPath)
	texture := model.NewTexture()
	texture.SetName(filepath.Join("tex", "cloth.png"))
	texture.SetValid(true)
	textureIndex := modelData.Textures.AppendRaw(texture)

	materialData := newMaterial("Tops_01_CLOTH", 1.0, 3)
	materialData.TextureIndex = textureIndex
	materialData.EdgeSize = 1.2
	materialData.Memo = "VRM primitive alphaMode=OPAQUE"
	materialData.DrawFlag = model.DRAW_FLAG_DOUBLE_SIDED_DRAWING | model.DRAW_FLAG_DRAWING_EDGE
	modelData.Materials.AppendRaw(materialData)

	appendUvVertex(modelData, vec3(0, 0, 0), mmath.Vec2{X: 0.1, Y: 0.1}, 0, []int{0})
	appendUvVertex(modelData, vec3(1, 0, 0), mmath.Vec2{X: 0.2, Y: 0.1}, 0, []int{0})
	appendUvVertex(modelData, vec3(0, 1, 0), mmath.Vec2{X: 0.1, Y: 0.2}, 0, []int{0})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{0, 1, 2}})

	if err := prepareVroidMaterialVariantsBeforeReorder(modelData); err != nil {
		t.Fatalf("prepare vroid material variants failed: %v", err)
	}

	gotNames := materialNames(modelData)
	wantNames := []string{"Tops_01_CLOTH"}
	if len(gotNames) != len(wantNames) {
		t.Fatalf("material count mismatch: got=%d want=%d names=%v", len(gotNames), len(wantNames), gotNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("material names mismatch: got=%v want=%v", gotNames, wantNames)
		}
	}
	if modelData.Vertices.Len() != 3 {
		t.Fatalf("vertex count mismatch: got=%d want=3", modelData.Vertices.Len())
	}
	if modelData.Faces.Len() != 1 {
		t.Fatalf("face count mismatch: got=%d want=1", modelData.Faces.Len())
	}
}

func TestPrepareVroidMaterialVariantsBeforeReorderSkipsBlendMaterialWhenEdgeFlagOff(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "out", "model.pmx")
	texDir := filepath.Join(filepath.Dir(modelPath), "tex")
	if err := os.MkdirAll(texDir, 0o755); err != nil {
		t.Fatalf("mkdir tex failed: %v", err)
	}
	if err := writeAlphaTexture(filepath.Join(texDir, "cloth.png"), 10); err != nil {
		t.Fatalf("write texture failed: %v", err)
	}

	modelData := model.NewPmxModel()
	modelData.SetPath(modelPath)
	texture := model.NewTexture()
	texture.SetName(filepath.Join("tex", "cloth.png"))
	texture.SetValid(true)
	textureIndex := modelData.Textures.AppendRaw(texture)

	materialData := newMaterial("Tops_01_CLOTH", 1.0, 3)
	materialData.TextureIndex = textureIndex
	materialData.EdgeSize = 1.2
	materialData.Memo = "VRM primitive alphaMode=BLEND"
	materialData.DrawFlag = model.DRAW_FLAG_DOUBLE_SIDED_DRAWING
	modelData.Materials.AppendRaw(materialData)

	appendUvVertex(modelData, vec3(0, 0, 0), mmath.Vec2{X: 0.1, Y: 0.1}, 0, []int{0})
	appendUvVertex(modelData, vec3(1, 0, 0), mmath.Vec2{X: 0.2, Y: 0.1}, 0, []int{0})
	appendUvVertex(modelData, vec3(0, 1, 0), mmath.Vec2{X: 0.1, Y: 0.2}, 0, []int{0})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{0, 1, 2}})

	if err := prepareVroidMaterialVariantsBeforeReorder(modelData); err != nil {
		t.Fatalf("prepare vroid material variants failed: %v", err)
	}

	gotNames := materialNames(modelData)
	wantNames := []string{"Tops_01_CLOTH"}
	if len(gotNames) != len(wantNames) {
		t.Fatalf("material count mismatch: got=%d want=%d names=%v", len(gotNames), len(wantNames), gotNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("material names mismatch: got=%v want=%v", gotNames, wantNames)
		}
	}
	if modelData.Vertices.Len() != 3 {
		t.Fatalf("vertex count mismatch: got=%d want=3", modelData.Vertices.Len())
	}
	if modelData.Faces.Len() != 1 {
		t.Fatalf("face count mismatch: got=%d want=1", modelData.Faces.Len())
	}
}

func TestHasMaterialVariantSuffixSupportsSerialSuffix(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{name: "Onepiece_00_CLOTH_表面_2", want: true},
		{name: "Shoes_01_CLOTH_裏面_3", want: true},
		{name: "Tops_01_CLOTH_エッジ_10", want: true},
		{name: "Body_00_SKIN_2", want: false},
	}

	for _, c := range cases {
		got := hasMaterialVariantSuffix(c.name)
		if got != c.want {
			t.Fatalf("hasMaterialVariantSuffix mismatch: name=%q got=%t want=%t", c.name, got, c.want)
		}
	}
}

func TestResolveMaterialVariantBaseNameStripsInstanceAndSerial(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{name: "N00_010_01_Onepiece_00_CLOTH (Instance)_表面", want: "N00_010_01_Onepiece_00_CLOTH"},
		{name: "Shoes_01_CLOTH (Instance)_裏面_2", want: "Shoes_01_CLOTH"},
		{name: "Tops_01_CLOTH_エッジ_3", want: "Tops_01_CLOTH"},
		{name: "Face_00_SKIN", want: "Face_00_SKIN"},
	}

	for _, c := range cases {
		got := resolveMaterialVariantBaseName(c.name)
		if got != c.want {
			t.Fatalf("resolveMaterialVariantBaseName mismatch: name=%q got=%q want=%q", c.name, got, c.want)
		}
	}
}

func TestAbbreviateMaterialNamesBeforeReorderKeepsNonAsciiName(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("髪-後ろ", 1.0, 3))

	if err := abbreviateMaterialNamesBeforeReorder(modelData); err != nil {
		t.Fatalf("abbreviate material names failed: %v", err)
	}

	gotNames := materialNames(modelData)
	wantNames := []string{"髪-後ろ"}
	for i := range wantNames {
		if i >= len(gotNames) || gotNames[i] != wantNames[i] {
			t.Fatalf("material names mismatch: got=%v want=%v", gotNames, wantNames)
		}
	}
}

func TestResolvePairTransparencyScoresForOrderUsesUvForNearFullTransparencyPair(t *testing.T) {
	materialTransparencyScores := map[int]float64{
		16: 0.792871,
		17: 0.935108,
	}
	materialUvTransparencyScores := map[int]float64{
		16: 1.0,
		17: 1.0,
	}

	leftTransparency, rightTransparency := resolvePairTransparencyScoresForOrder(
		16,
		17,
		materialTransparencyScores,
		materialUvTransparencyScores,
	)

	if leftTransparency != 1.0 || rightTransparency != 1.0 {
		t.Fatalf(
			"expected UV transparency override for near-full pair: got=(%f,%f)",
			leftTransparency,
			rightTransparency,
		)
	}
}

func TestShouldPreferHigherTransparencyInStrongOverlap(t *testing.T) {
	cases := []struct {
		name                  string
		absTransparencyDelta  float64
		leftTransparency      float64
		rightTransparency     float64
		wantPreferHigherTrans bool
	}{
		{
			name:                  "large_gap_with_near_full_transparency",
			absTransparencyDelta:  0.40,
			leftTransparency:      0.50,
			rightTransparency:     1.00,
			wantPreferHigherTrans: true,
		},
		{
			name:                  "small_gap",
			absTransparencyDelta:  0.20,
			leftTransparency:      0.50,
			rightTransparency:     1.00,
			wantPreferHigherTrans: false,
		},
		{
			name:                  "large_gap_without_near_full_transparency",
			absTransparencyDelta:  0.40,
			leftTransparency:      0.55,
			rightTransparency:     0.88,
			wantPreferHigherTrans: false,
		},
	}

	for _, c := range cases {
		got := shouldPreferHigherTransparencyInStrongOverlap(
			c.absTransparencyDelta,
			c.leftTransparency,
			c.rightTransparency,
		)
		if got != c.wantPreferHigherTrans {
			t.Fatalf(
				"shouldPreferHigherTransparencyInStrongOverlap mismatch: case=%s got=%t want=%t",
				c.name,
				got,
				c.wantPreferHigherTrans,
			)
		}
	}
}

func TestResolvePairOrderByOverlapUsesFarFirstOnlyWhenDepthGapIsLarge(t *testing.T) {
	spatialInfoMap := map[int]materialSpatialInfo{
		1: buildOverlapSpatialInfoForTest(0.01),
		2: buildOverlapSpatialInfoForTest(0.05),
	}

	leftBeforeRight, _, valid := resolvePairOrderByOverlap(
		1,
		2,
		spatialInfoMap,
		0.10,
		map[int]float64{
			1: 0.70,
			2: 0.80,
		},
		nil,
	)
	if !valid {
		t.Fatalf("expected overlap pair to be resolvable")
	}
	if leftBeforeRight {
		t.Fatalf("expected far-first ordering when depth gap is large")
	}
}

func TestResolvePairOrderByOverlapUsesTransparencyOrderWhenDepthGapIsSmall(t *testing.T) {
	spatialInfoMap := map[int]materialSpatialInfo{
		1: buildOverlapSpatialInfoForTest(0.01),
		2: buildOverlapSpatialInfoForTest(0.02),
	}

	leftBeforeRight, _, valid := resolvePairOrderByOverlap(
		1,
		2,
		spatialInfoMap,
		0.10,
		map[int]float64{
			1: 0.70,
			2: 0.80,
		},
		nil,
	)
	if !valid {
		t.Fatalf("expected overlap pair to be resolvable")
	}
	if !leftBeforeRight {
		t.Fatalf("expected transparency-first ordering when depth gap is small")
	}
}

func TestMergeDirectionalPairDecisionsUsesForwardResultForConflictingTie(t *testing.T) {
	leftBeforeRight, confidence, valid := mergeDirectionalPairDecisions(
		true,
		1.25,
		true,
		true,
		1.25,
		true,
	)
	if !valid {
		t.Fatalf("expected conflict to resolve with forward decision")
	}
	if !leftBeforeRight {
		t.Fatalf("expected forward decision to be selected")
	}
	if confidence != 1.25 {
		t.Fatalf("expected forward confidence to be selected: got=%f", confidence)
	}
}

func TestMergeDirectionalPairDecisionsUsesForwardResultForConflictingDirection(t *testing.T) {
	leftBeforeRight, confidence, valid := mergeDirectionalPairDecisions(
		false,
		0.75,
		true,
		false,
		9.99,
		true,
	)
	if !valid {
		t.Fatalf("expected conflict to resolve with forward decision")
	}
	if leftBeforeRight {
		t.Fatalf("expected forward decision to be selected")
	}
	if confidence != 0.75 {
		t.Fatalf("expected forward confidence to be selected: got=%f", confidence)
	}
}

func TestResolvePairOrderConstraintFallsBackToBodyProximity(t *testing.T) {
	spatialInfoMap := map[int]materialSpatialInfo{
		10: {
			points:       []mmath.Vec3{vec3(0, 0, 0)},
			bodyDistance: []float64{0.60},
			minX:         0,
			maxX:         0,
			minY:         0,
			maxY:         0,
			minZ:         0,
			maxZ:         0,
		},
		11: {
			points:       []mmath.Vec3{vec3(10, 0, 0)},
			bodyDistance: []float64{0.20},
			minX:         10,
			maxX:         10,
			minY:         0,
			maxY:         0,
			minZ:         0,
			maxZ:         0,
		},
	}

	leftBeforeRight, _, valid := resolvePairOrderConstraint(
		10,
		11,
		spatialInfoMap,
		0.05,
		map[int]float64{
			10: 0.50,
			11: 0.50,
		},
		nil,
		map[int]float64{
			10: 0.60,
			11: 0.20,
		},
	)
	if !valid {
		t.Fatalf("expected body proximity fallback to resolve order")
	}
	if leftBeforeRight {
		t.Fatalf("expected right material to be selected first by body proximity fallback")
	}
}

func TestResolvePairOrderConstraintReturnsInvalidWithoutBodyFallback(t *testing.T) {
	spatialInfoMap := map[int]materialSpatialInfo{
		10: {
			points:       []mmath.Vec3{vec3(0, 0, 0)},
			bodyDistance: []float64{0.60},
			minX:         0,
			maxX:         0,
			minY:         0,
			maxY:         0,
			minZ:         0,
			maxZ:         0,
		},
		11: {
			points:       []mmath.Vec3{vec3(10, 0, 0)},
			bodyDistance: []float64{0.20},
			minX:         10,
			maxX:         10,
			minY:         0,
			maxY:         0,
			minZ:         0,
			maxZ:         0,
		},
	}

	_, _, valid := resolvePairOrderConstraint(
		10,
		11,
		spatialInfoMap,
		0.05,
		map[int]float64{
			10: 0.50,
			11: 0.50,
		},
		nil,
		map[int]float64{
			10: math.Inf(1),
		},
	)
	if valid {
		t.Fatalf("expected unresolved pair when body proximity fallback is unavailable")
	}
}

func TestApplyFaceEyeMaterialPriorityReordersAcrossNonFaceMaterials(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("FaceEyelash_00_FACE", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Onepiece_00_CLOTH", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("FaceBrow_00_FACE", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("FaceEyeline_00_FACE", 1.0, 3))

	got := applyFaceEyeMaterialPriority(modelData, []int{0, 1, 2, 3, 4})
	want := []int{0, 3, 2, 4, 1}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("applyFaceEyeMaterialPriority mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestApplyFaceEyeMaterialPriorityPrioritizesFaceSkinBeforeFaceParts(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("FaceBrow_00_FACE", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Face_00_SKIN", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("FaceEyeline_00_FACE", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("FaceEyelash_00_FACE", 1.0, 3))

	got := applyFaceEyeMaterialPriority(modelData, []int{0, 1, 2, 3})
	want := []int{1, 0, 2, 3}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("applyFaceEyeMaterialPriority face-skin mismatch: got=%v want=%v", got, want)
		}
	}
}

func buildOverlapSpatialInfoForTest(bodyDistance float64) materialSpatialInfo {
	points := []mmath.Vec3{
		vec3(0.0, 0.0, 0.0),
		vec3(0.1, 0.0, 0.0),
		vec3(0.0, 0.1, 0.0),
		vec3(0.1, 0.1, 0.0),
	}
	return materialSpatialInfo{
		points:       points,
		bodyDistance: []float64{bodyDistance, bodyDistance, bodyDistance, bodyDistance},
		minX:         0.0,
		maxX:         0.1,
		minY:         0.0,
		maxY:         0.1,
		minZ:         0.0,
		maxZ:         0.0,
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

// newBlendClothVariantModelForEdgeTest はエッジ材質複製検証向けの最小モデルを生成する。
func newBlendClothVariantModelForEdgeTest(
	t *testing.T,
	edgeSize float64,
	positions [3]mmath.Vec3,
) *ModelData {
	t.Helper()
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "out", "model.pmx")
	texDir := filepath.Join(filepath.Dir(modelPath), "tex")
	if err := os.MkdirAll(texDir, 0o755); err != nil {
		t.Fatalf("mkdir tex failed: %v", err)
	}
	if err := writeAlphaTexture(filepath.Join(texDir, "cloth.png"), 10); err != nil {
		t.Fatalf("write texture failed: %v", err)
	}

	modelData := model.NewPmxModel()
	modelData.SetPath(modelPath)
	texture := model.NewTexture()
	texture.SetName(filepath.Join("tex", "cloth.png"))
	texture.SetValid(true)
	textureIndex := modelData.Textures.AppendRaw(texture)

	materialData := newMaterial("Tops_01_CLOTH", 1.0, 3)
	materialData.TextureIndex = textureIndex
	materialData.Edge = mmath.Vec4{X: 0.2, Y: 0.3, Z: 0.4, W: 1.0}
	materialData.EdgeSize = edgeSize
	materialData.Memo = "VRM primitive alphaMode=BLEND outlineWidthMode=worldCoordinates outlineWidthFactor=0.0008"
	materialData.DrawFlag = model.DRAW_FLAG_DOUBLE_SIDED_DRAWING | model.DRAW_FLAG_DRAWING_EDGE
	modelData.Materials.AppendRaw(materialData)

	appendUvVertex(modelData, positions[0], mmath.Vec2{X: 0.1, Y: 0.1}, 0, []int{0})
	appendUvVertex(modelData, positions[1], mmath.Vec2{X: 0.2, Y: 0.1}, 0, []int{0})
	appendUvVertex(modelData, positions[2], mmath.Vec2{X: 0.1, Y: 0.2}, 0, []int{0})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{0, 1, 2}})
	return modelData
}

// appendVertex は並べ替え検証用の頂点を追加する。
func appendVertex(modelData *ModelData, position mmath.Vec3, boneIndex int, materialIndexes []int) {
	appendUvVertex(modelData, position, mmath.ZERO_VEC2, boneIndex, materialIndexes)
}

// appendUvVertex は並べ替え検証用の頂点を追加する。
func appendUvVertex(modelData *ModelData, position mmath.Vec3, uv mmath.Vec2, boneIndex int, materialIndexes []int) {
	vertex := &model.Vertex{
		Position:        position,
		Normal:          vec3(0, 1, 0),
		Uv:              uv,
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

// writeHalfAlphaTexture は左右で異なるアルファ値の2x1テクスチャを書き込む。
func writeHalfAlphaTexture(path string, leftAlpha uint8, rightAlpha uint8) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: leftAlpha})
	img.Set(1, 0, color.NRGBA{R: 255, G: 255, B: 255, A: rightAlpha})
	return png.Encode(file, img)
}

// prepareTransparentTestTextures は透過テクスチャを作成しモデルパスを返す。
func prepareTransparentTestTextures(t *testing.T, names []string) (string, error) {
	t.Helper()
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "out", "model.pmx")
	texDir := filepath.Join(filepath.Dir(modelPath), "tex")
	if err := os.MkdirAll(texDir, 0o755); err != nil {
		return "", err
	}
	for _, name := range names {
		texPath := filepath.Join(texDir, name)
		if err := writeAlphaTexture(texPath, 10); err != nil {
			return "", err
		}
	}
	return modelPath, nil
}

// assignMaterialTextureIndex は材質へテクスチャを割り当てる。
func assignMaterialTextureIndex(modelData *ModelData, materialIndex int, textureName string) {
	if modelData == nil {
		return
	}
	texture := model.NewTexture()
	texture.SetName(filepath.Join("tex", textureName))
	texture.SetValid(true)
	textureIndex := modelData.Textures.AppendRaw(texture)
	materialData, err := modelData.Materials.Get(materialIndex)
	if err != nil || materialData == nil {
		return
	}
	materialData.TextureIndex = textureIndex
}

// vec3 はテスト用のVec3を生成する。
func vec3(x, y, z float64) mmath.Vec3 {
	return mmath.Vec3{Vec: r3.Vec{X: x, Y: y, Z: z}}
}
