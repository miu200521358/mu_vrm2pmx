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
	"github.com/miu200521358/mu_vrm2pmx/pkg/adapter/mpresenter/messages"
	"gonum.org/v1/gonum/spatial/r3"
)

func TestApplyBodyDepthMaterialOrderMaintainsOriginalOrderWhenTransparentMaterialsDoNotOverlap(t *testing.T) {
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
	wantNames := []string{"body", "far", "near"}
	for i := range wantNames {
		if i >= len(gotNames) || gotNames[i] != wantNames[i] {
			t.Fatalf("material order mismatch: got=%v want=%v", gotNames, wantNames)
		}
	}

	faceFar, err := modelData.Faces.Get(1)
	if err != nil || faceFar == nil {
		t.Fatalf("far face missing: err=%v", err)
	}
	if faceFar.VertexIndexes != [3]int{3, 4, 5} {
		t.Fatalf("far face vertices mismatch: got=%v", faceFar.VertexIndexes)
	}

	faceNear, err := modelData.Faces.Get(2)
	if err != nil || faceNear == nil {
		t.Fatalf("near face missing: err=%v", err)
	}
	if faceNear.VertexIndexes != [3]int{6, 7, 8} {
		t.Fatalf("near face vertices mismatch: got=%v", faceNear.VertexIndexes)
	}

	for _, idx := range []int{3, 4, 5} {
		vertex, vErr := modelData.Vertices.Get(idx)
		if vErr != nil || vertex == nil {
			t.Fatalf("far vertex missing: idx=%d err=%v", idx, vErr)
		}
		if len(vertex.MaterialIndexes) == 0 || vertex.MaterialIndexes[0] != 1 {
			t.Fatalf("far vertex material index mismatch: idx=%d got=%v", idx, vertex.MaterialIndexes)
		}
	}
	for _, idx := range []int{6, 7, 8} {
		vertex, vErr := modelData.Vertices.Get(idx)
		if vErr != nil || vertex == nil {
			t.Fatalf("near vertex missing: idx=%d err=%v", idx, vErr)
		}
		if len(vertex.MaterialIndexes) == 0 || vertex.MaterialIndexes[0] != 2 {
			t.Fatalf("near vertex material index mismatch: idx=%d got=%v", idx, vertex.MaterialIndexes)
		}
	}
}

func TestHasTransparentTextureAlphaUsesNonOpaqueThreshold(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "out", "model.pmx")
	texDir := filepath.Join(filepath.Dir(modelPath), "tex")
	if err := os.MkdirAll(texDir, 0o755); err != nil {
		t.Fatalf("mkdir tex failed: %v", err)
	}

	alphaNonOpaquePath := filepath.Join(texDir, "non_opaque.png")
	if err := writeAlphaTexture(alphaNonOpaquePath, 254); err != nil {
		t.Fatalf("write texture failed: %v", err)
	}
	alphaOpaquePath := filepath.Join(texDir, "opaque.png")
	if err := writeAlphaTexture(alphaOpaquePath, 255); err != nil {
		t.Fatalf("write texture failed: %v", err)
	}

	modelData := model.NewPmxModel()
	modelData.SetPath(modelPath)
	textureBelow := model.NewTexture()
	textureBelow.SetName(filepath.Join("tex", "non_opaque.png"))
	textureBelow.SetValid(true)
	textureAbove := model.NewTexture()
	textureAbove.SetName(filepath.Join("tex", "opaque.png"))
	textureAbove.SetValid(true)
	modelData.Textures.AppendRaw(textureBelow)
	modelData.Textures.AppendRaw(textureAbove)

	cache := map[int]textureAlphaCacheEntry{}
	if !hasTransparentTextureAlpha(modelData, 0, cache) {
		t.Fatalf("expected non-opaque texture alpha to be transparent")
	}
	if hasTransparentTextureAlpha(modelData, 1, cache) {
		t.Fatalf("expected fully opaque texture alpha to stay opaque")
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

func TestBuildTransparentCandidateScoresUsesExplicitAlphaMode(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "out", "model.pmx")
	texDir := filepath.Join(filepath.Dir(modelPath), "tex")
	if err := os.MkdirAll(texDir, 0o755); err != nil {
		t.Fatalf("mkdir tex failed: %v", err)
	}
	if err := writeAlphaTexture(filepath.Join(texDir, "opaque.png"), 255); err != nil {
		t.Fatalf("write texture failed: %v", err)
	}

	modelData := model.NewPmxModel()
	modelData.SetPath(modelPath)
	texture := model.NewTexture()
	texture.SetName(filepath.Join("tex", "opaque.png"))
	texture.SetValid(true)
	textureIndex := modelData.Textures.AppendRaw(texture)

	blend := newMaterial("Tops_01_CLOTH_表面", 1.0, 3)
	blend.Memo = "VRM primitive alphaMode=BLEND"
	blend.TextureIndex = textureIndex
	modelData.Materials.AppendRaw(blend)
	opaque := newMaterial("Cape_01_CLOTH", 1.0, 3)
	opaque.TextureIndex = textureIndex
	modelData.Materials.AppendRaw(opaque)

	got := buildTransparentCandidateScores(
		modelData,
		[]materialFaceRange{{}, {}},
		map[int]textureImageCacheEntry{},
		resolveTransparentCandidateAlphaThreshold(),
	)

	if got[0] <= materialOrderScoreEpsilon {
		t.Fatalf("blend material should become candidate from explicit alpha semantics: got=%v", got)
	}
	if got[1] != 0 {
		t.Fatalf("opaque material without transparency evidence should stay 0: got=%v", got)
	}
}

func TestCollectTransparentMaterialIndexesFromScoresFollowsVariantFamily(t *testing.T) {
	modelData := model.NewPmxModel()
	front := newMaterial("Tops_01_CLOTH_表面", 1.0, 3)
	modelData.Materials.AppendRaw(front)
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_裏面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_エッジ", 1.0, 3))
	specialEye := newMaterial("eye_star", 1.0, 3)
	specialEye.Memo = "VRM primitive alphaMode=BLEND"
	modelData.Materials.AppendRaw(specialEye)

	got := collectTransparentMaterialIndexesFromScores(modelData, map[int]float64{
		0: 0.25,
		3: 1.0,
	})

	want := []int{0, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("transparent candidates count mismatch: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("transparent candidates mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestFilterTransparentMaterialIndexesByActualTransparencyDropsUvOnlyOpaqueMaterials(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("Face_00_SKIN", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Overlay_01", 0.75, 3))

	got := filterTransparentMaterialIndexesByActualTransparency(
		modelData,
		[]int{0, 1, 2},
		map[int]float64{
			1: 0.32,
		},
	)

	want := []int{1, 2}
	if len(got) != len(want) {
		t.Fatalf("actual transparency filtered count mismatch: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("actual transparency filtered indexes mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestFilterTransparentMaterialIndexesByActualTransparencyKeepsVariantFamilyMembers(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_表面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_裏面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_エッジ", 1.0, 3))

	got := filterTransparentMaterialIndexesByActualTransparency(
		modelData,
		[]int{0, 1, 2},
		map[int]float64{
			0: 0.42,
		},
	)

	want := []int{0, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("variant family filtered count mismatch: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("variant family filtered indexes mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestCollectBodyWeightedPointsIncludesPositiveWeightsBelowLegacyThreshold(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position: vec3(1, 0, 0),
		Normal:   vec3(0, 1, 0),
		Deform:   model.NewBdef2(0, 1, 0.2),
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position: vec3(2, 0, 0),
		Normal:   vec3(0, 1, 0),
		Deform:   model.NewBdef2(1, 0, 0.8),
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position: vec3(3, 0, 0),
		Normal:   vec3(0, 1, 0),
		Deform:   model.NewBdef2(1, 2, 0.7),
	})

	got := collectBodyWeightedPoints(modelData, map[int]struct{}{0: {}}, 8)
	if len(got) != 2 {
		t.Fatalf("body weighted points count mismatch: got=%d want=2 points=%v", len(got), got)
	}
	if !got[0].NearEquals(vec3(1, 0, 0), 1e-9) {
		t.Fatalf("1st body point mismatch: got=%v", got[0])
	}
	if !got[1].NearEquals(vec3(2, 0, 0), 1e-9) {
		t.Fatalf("2nd body point mismatch: got=%v", got[1])
	}
}

func TestCollectBodyPointsFromOpaqueMaterialsUsesAllOpaqueCandidates(t *testing.T) {
	modelData := model.NewPmxModel()
	for i := 0; i < 4; i++ {
		modelData.Materials.AppendRaw(newMaterial("opaque", 1.0, 3))
	}
	for i := 0; i < 4; i++ {
		baseX := float64(i * 10)
		appendVertex(modelData, vec3(baseX+0, 0, 0), 0, []int{i})
		appendVertex(modelData, vec3(baseX+1, 0, 0), 0, []int{i})
		appendVertex(modelData, vec3(baseX+2, 0, 0), 0, []int{i})
		modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{i * 3, i*3 + 1, i*3 + 2}})
	}

	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil {
		t.Fatalf("build face ranges failed: %v", err)
	}

	got := collectBodyPointsFromOpaqueMaterials(modelData, faceRanges, map[int]struct{}{}, 16, 1)
	if len(got) != 12 {
		t.Fatalf("opaque fallback points count mismatch: got=%d want=12", len(got))
	}
	foundLastMaterial := false
	for _, point := range got {
		if point.NearEquals(vec3(30, 0, 0), 1e-9) ||
			point.NearEquals(vec3(31, 0, 0), 1e-9) ||
			point.NearEquals(vec3(32, 0, 0), 1e-9) {
			foundLastMaterial = true
			break
		}
	}
	if !foundLastMaterial {
		t.Fatalf("expected opaque fallback to include the 4th material: points=%v", got)
	}
}

func TestCollectBodyPointsForSortingUsesBodyMaterialFacesBeforeWeightFallback(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("body", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("cloth", 1.0, 0))
	modelData.Bones.AppendRaw(newBone("hips"))

	vrmData := vrm.NewVrmData()
	vrmData.Vrm1 = vrm.NewVrm1Data()
	vrmData.Vrm1.Humanoid.HumanBones["hips"] = vrm.Vrm1HumanBone{Node: 0}
	modelData.VrmData = vrmData

	appendVertex(modelData, vec3(0, 0, 0), 1, []int{0})
	appendVertex(modelData, vec3(1, 0, 0), 1, []int{0})
	appendVertex(modelData, vec3(0, 1, 0), 1, []int{0})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{0, 1, 2}})

	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        vec3(10, 10, 10),
		Normal:          vec3(0, 1, 0),
		Deform:          model.NewBdef1(0),
		MaterialIndexes: []int{0},
	})

	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil {
		t.Fatalf("build face ranges failed: %v", err)
	}

	got := collectBodyPointsForSorting(modelData, faceRanges, map[int]struct{}{1: {}}, 1)
	if len(got) == 0 {
		t.Fatalf("body points should not be empty")
	}
	if !got[0].NearEquals(vec3(0, 0, 0), 1e-9) {
		t.Fatalf("expected body material face samples to take precedence: got=%v", got)
	}
}

func TestDetectBodyMaterialIndexPrefersBodySkinSemanticName(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("Onepiece_00_CLOTH", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Body_00_SKIN", 1.0, 3))

	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        vec3(0, 0, 0),
		Normal:          vec3(0, 1, 0),
		Deform:          model.NewBdef1(0),
		MaterialIndexes: []int{0},
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        vec3(1, 0, 0),
		Normal:          vec3(0, 1, 0),
		Deform:          model.NewBdef1(0),
		MaterialIndexes: []int{0},
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        vec3(2, 0, 0),
		Normal:          vec3(0, 1, 0),
		Deform:          model.NewBdef1(0),
		MaterialIndexes: []int{1},
	})

	got := detectBodyMaterialIndex(modelData, map[int]struct{}{0: {}})
	if got != 1 {
		t.Fatalf("body material index mismatch: got=%d want=1", got)
	}
}

func TestApplyBodyDepthMaterialOrderUsesAlphaModeCandidateWhenTextureDetectionFails(t *testing.T) {
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
	farMaterial.Memo = "VRM primitive alphaMode=BLEND"

	nearMaterial, err := modelData.Materials.Get(2)
	if err != nil || nearMaterial == nil {
		t.Fatalf("near material missing: err=%v", err)
	}
	nearMaterial.TextureIndex = 2
	nearMaterial.Memo = "VRM primitive alphaMode=BLEND"

	applyBodyDepthMaterialOrder(modelData)

	gotNames := materialNames(modelData)
	wantNames := []string{"body", "far", "near"}
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
		resolveTransparentCandidateAlphaThreshold(),
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
	warningPrefix := strings.SplitN(messages.LogMaterialReorderWarnEdgeOffsetTuning, " material=", 2)[0]
	hasWarningLine := false
	for _, line := range lines {
		if strings.Contains(line, warningPrefix) {
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
	statsPrefix := strings.SplitN(messages.LogMaterialReorderInfoEdgeOffsetStats, " material=", 2)[0]
	hasExpectedLine := false
	for _, line := range lines {
		if !strings.Contains(line, statsPrefix) {
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
		if !strings.Contains(line, "route_count[edge_size=") || !strings.Contains(line, "guard_count[p50=") {
			t.Fatalf("route/guard count fields missing: line=%s", line)
		}
		if !strings.Contains(line, "coef[k_model_floor=") {
			t.Fatalf("coefficient summary field missing: line=%s", line)
		}
		if !strings.Contains(line, "judge_primary="+messages.EdgeAcceptanceJudgePass) || !strings.Contains(line, "reason_codes="+messages.EdgeAcceptanceReasonNone) {
			t.Fatalf("judge result fields missing: line=%s", line)
		}
		break
	}
	if !hasExpectedLine {
		t.Fatal("edge offset stats log should be emitted")
	}
}

func TestEvaluatePrimaryEdgeAcceptance(t *testing.T) {
	passResult := evaluatePrimaryEdgeAcceptance(0.009, 0.010, 0)
	if !passResult.passed {
		t.Fatalf("expected pass result: %+v", passResult)
	}
	if len(passResult.reasonCodes) != 0 {
		t.Fatalf("pass result should not have reasons: %+v", passResult.reasonCodes)
	}

	failResult := evaluatePrimaryEdgeAcceptance(0.008, 0.0085, 2)
	if failResult.passed {
		t.Fatalf("expected fail result: %+v", failResult)
	}
	expectedReasons := []string{
		messages.EdgeAcceptanceReasonPrimaryP50Below,
		messages.EdgeAcceptanceReasonPrimaryP95Below,
		messages.EdgeAcceptanceReasonPrimaryCoincident,
	}
	for _, expectedReason := range expectedReasons {
		if !containsString(failResult.reasonCodes, expectedReason) {
			t.Fatalf("missing reason code: expected=%s got=%v", expectedReason, failResult.reasonCodes)
		}
	}
}

func TestEvaluateComparisonEdgeRegressionBoundaries(t *testing.T) {
	passResult := evaluateComparisonEdgeRegression(0.0085, 0, 0.01, 0)
	if !passResult.passed {
		t.Fatalf("expected pass within 15%% drop: %+v", passResult)
	}
	if math.Abs(passResult.p50DropRatio-0.15) > 1e-9 {
		t.Fatalf("drop ratio mismatch: got=%f want=0.15", passResult.p50DropRatio)
	}

	failResult := evaluateComparisonEdgeRegression(0.008499, 1, 0.01, 0)
	if failResult.passed {
		t.Fatalf("expected fail over 15%% drop with coincident: %+v", failResult)
	}
	if !containsString(failResult.reasonCodes, messages.EdgeAcceptanceReasonComparisonCurrent) {
		t.Fatalf("missing coincident reason: got=%v", failResult.reasonCodes)
	}
	if !containsString(failResult.reasonCodes, messages.EdgeAcceptanceReasonComparisonP50Drop) {
		t.Fatalf("missing regression reason: got=%v", failResult.reasonCodes)
	}
}

func TestEvaluateComparisonEdgeRegressionRejectsInvalidBaseline(t *testing.T) {
	result := evaluateComparisonEdgeRegression(0.009, 0, 0, 0)
	if result.passed {
		t.Fatalf("expected fail with non-positive baseline: %+v", result)
	}
	if !containsString(result.reasonCodes, messages.EdgeAcceptanceReasonBaselineP50NonPos) {
		t.Fatalf("missing baseline reason: got=%v", result.reasonCodes)
	}
}

func TestSelectEdgeAcceptanceTargetsRequiresVrm0AndVrm1(t *testing.T) {
	_, err := selectEdgeAcceptanceTargets([]edgeVariantAcceptanceModelCandidate{
		{modelID: "model_a", profile: "vrm1", finalP50: 0.01, finalP95: 0.01},
	})
	if err == nil {
		t.Fatal("expected error when vrm0 candidate is missing")
	}
	if !strings.Contains(err.Error(), "比較対象選定規則違反") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSelectEdgeAcceptanceTargetsSelectsPrimaryAndComparisons(t *testing.T) {
	selection, err := selectEdgeAcceptanceTargets([]edgeVariantAcceptanceModelCandidate{
		{modelID: "vrm1_good", profile: "vrm1", finalP50: 0.02, finalP95: 0.02, coincidentCount: 0},
		{modelID: "vrm1_worst", profile: "VRM1.0", finalP50: 0.008, finalP95: 0.0085, coincidentCount: 1},
		{modelID: "vrm0_ref", profile: "vrm0", finalP50: 0.011, finalP95: 0.012, coincidentCount: 0},
	})
	if err != nil {
		t.Fatalf("select target failed: %v", err)
	}
	if selection.primary.modelID != "vrm1_worst" {
		t.Fatalf("primary selection mismatch: got=%s want=vrm1_worst", selection.primary.modelID)
	}
	if len(selection.comparisons) != 2 {
		t.Fatalf("comparison count mismatch: got=%d want=2", len(selection.comparisons))
	}
	hasVrm0 := false
	hasVrm1 := false
	for _, comparison := range selection.comparisons {
		switch comparison.profile {
		case "vrm0":
			hasVrm0 = true
		case "vrm1":
			hasVrm1 = true
		}
	}
	if !hasVrm0 || !hasVrm1 {
		t.Fatalf("comparison profiles should include vrm0 and vrm1: %+v", selection.comparisons)
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

func TestResolveMaterialTransparencyForObservationPrefersUvScore(t *testing.T) {
	materialTransparencyScores := map[int]float64{
		16: 0.792871,
		17: 0.935108,
	}
	materialUvTransparencyScores := map[int]float64{
		16: 1.0,
		17: 1.0,
	}

	leftTransparency := resolveMaterialTransparencyForObservation(
		16,
		materialTransparencyScores,
		materialUvTransparencyScores,
	)
	rightTransparency := resolveMaterialTransparencyForObservation(
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

func TestAnalyzeMaterialPairObservationIsSymmetric(t *testing.T) {
	spatialInfoMap := map[int]materialSpatialInfo{
		1: buildOverlapSpatialInfoForTest(0.40),
		2: buildOverlapSpatialInfoForTest(0.10),
	}

	forward, valid := analyzeMaterialPairObservation(
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
		t.Fatalf("expected forward observation to be valid")
	}

	reverse, valid := analyzeMaterialPairObservation(
		2,
		1,
		spatialInfoMap,
		0.10,
		map[int]float64{
			1: 0.70,
			2: 0.80,
		},
		nil,
	)
	if !valid {
		t.Fatalf("expected reverse observation to be valid")
	}
	if forward.sharedEvidenceCount != reverse.sharedEvidenceCount {
		t.Fatalf("shared evidence mismatch: forward=%d reverse=%d", forward.sharedEvidenceCount, reverse.sharedEvidenceCount)
	}
	if math.Abs(forward.bodyDominance+reverse.bodyDominance) > 1e-9 {
		t.Fatalf("body dominance should be symmetric: forward=%f reverse=%f", forward.bodyDominance, reverse.bodyDominance)
	}
	if math.Abs(forward.containmentDominance+reverse.containmentDominance) > 1e-9 {
		t.Fatalf("containment dominance should be symmetric: forward=%f reverse=%f", forward.containmentDominance, reverse.containmentDominance)
	}
}

func TestAnalyzeMaterialPairObservationUsesMatchedBodyDominanceSumAndOpacityGap(t *testing.T) {
	spatialInfoMap := map[int]materialSpatialInfo{
		1: {
			points: []mmath.Vec3{
				vec3(0, 0, 0),
				vec3(1, 0, 0),
				vec3(2, 0, 0),
				vec3(3, 0, 0),
			},
			bodyDistance: []float64{1, 1, 1, 1},
		},
		2: {
			points: []mmath.Vec3{
				vec3(0, 0, 0),
				vec3(1, 0, 0),
				vec3(2, 0, 0),
				vec3(3, 0, 0),
			},
			bodyDistance: []float64{2, 2, 2, 2},
		},
	}

	observation, valid := analyzeMaterialPairObservation(
		1,
		2,
		spatialInfoMap,
		0.01,
		map[int]float64{
			1: 0.20,
			2: 0.80,
		},
		nil,
	)
	if !valid {
		t.Fatalf("expected observation to be valid")
	}
	if math.Abs(observation.bodyDominance-4.0) > 1e-9 {
		t.Fatalf("body dominance should use matched body distance sum: got=%f want=4.0", observation.bodyDominance)
	}
	if math.Abs(observation.opacityDominance-0.60) > 1e-9 {
		t.Fatalf("opacity dominance should be positive when left is more opaque: got=%f want=0.60", observation.opacityDominance)
	}
}

func TestCollectOverlapLocalBodyDistancesUsesOnlyNearbyPoints(t *testing.T) {
	got := collectOverlapLocalBodyDistances(
		materialSpatialInfo{
			points: []mmath.Vec3{
				vec3(0, 0, 0),
				vec3(1, 0, 0),
				vec3(5, 0, 0),
			},
			bodyDistance: []float64{0.20, 0.40, 0.80},
		},
		materialSpatialInfo{
			points: []mmath.Vec3{
				vec3(0.05, 0, 0),
				vec3(1.05, 0, 0),
			},
			bodyDistance: []float64{0.10, 0.10},
		},
		0.10,
	)
	want := []float64{0.20, 0.40}
	if len(got) != len(want) {
		t.Fatalf("nearby body distance count mismatch: got=%v want=%v", got, want)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("nearby body distance mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestResolvePairOrderByOverlapPrefersCloserBodySide(t *testing.T) {
	spatialInfoMap := map[int]materialSpatialInfo{
		1: buildOverlapSpatialInfoForTest(0.40),
		2: buildOverlapSpatialInfoForTest(0.10),
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
		t.Fatalf("expected body-closer right material to be selected first")
	}
}

func TestResolvePairOrderFromObservationPrioritizesBodyDominanceBeforeOpacity(t *testing.T) {
	leftBeforeRight, _, valid := resolvePairOrderFromObservation(
		1,
		2,
		materialPairObservation{
			sharedEvidenceCount:  8,
			leftCoverage:         0.60,
			rightCoverage:        0.60,
			bodyDominance:        0.25,
			containmentDominance: 0,
			opacityDominance:     -0.80,
			depthDominance:       -0.20,
		},
		nil,
		nil,
	)
	if !valid {
		t.Fatalf("expected observation to be resolvable")
	}
	if !leftBeforeRight {
		t.Fatalf("expected body dominance to win before opacity dominance")
	}
}

func TestResolvePairOrderFromObservationPrefersContainmentWhenBodyDominanceTies(t *testing.T) {
	leftBeforeRight, _, valid := resolvePairOrderFromObservation(
		1,
		2,
		materialPairObservation{
			sharedEvidenceCount:  8,
			leftCoverage:         0.25,
			rightCoverage:        0.85,
			bodyDominance:        0,
			containmentDominance: -0.60,
			opacityDominance:     0.80,
			depthDominance:       0.05,
		},
		nil,
		nil,
	)
	if !valid {
		t.Fatalf("expected containment-dominant observation to be resolvable")
	}
	if leftBeforeRight {
		t.Fatalf("expected containment dominance to decide before opacity/depth when body ties")
	}
}

func TestResolvePairOrderFromObservationKeepsBodyPriorityBeforeContainment(t *testing.T) {
	leftBeforeRight, _, valid := resolvePairOrderFromObservation(
		1,
		2,
		materialPairObservation{
			sharedEvidenceCount:  8,
			leftCoverage:         0.30,
			rightCoverage:        0.80,
			bodyDominance:        0.08,
			containmentDominance: -0.50,
			opacityDominance:     -0.30,
			depthDominance:       0.03,
		},
		nil,
		nil,
	)
	if !valid {
		t.Fatalf("expected observation to be resolvable")
	}
	if !leftBeforeRight {
		t.Fatalf("expected body dominance to stay ahead of containment/opacity/depth")
	}
}

func TestResolvePairOrderFromObservationKeepsBodyPriorityWhenOtherSignalsDisagree(t *testing.T) {
	leftBeforeRight, _, valid := resolvePairOrderFromObservation(
		1,
		2,
		materialPairObservation{
			sharedEvidenceCount:  8,
			leftCoverage:         0.30,
			rightCoverage:        0.80,
			bodyDominance:        -0.40,
			containmentDominance: 0.10,
			opacityDominance:     0.30,
			depthDominance:       0.20,
		},
		nil,
		nil,
	)
	if !valid {
		t.Fatalf("expected observation to be resolvable")
	}
	if leftBeforeRight {
		t.Fatalf("expected body dominance to stay first even when containment/opacity/depth disagree")
	}
}

func TestResolvePairOrderFromObservationSkipsBodyWhenContainmentAndDepthDisagreeAndOpacityIsNeutral(t *testing.T) {
	leftBeforeRight, _, valid := resolvePairOrderFromObservation(
		1,
		2,
		materialPairObservation{
			sharedEvidenceCount:  8,
			leftCoverage:         0.30,
			rightCoverage:        0.80,
			bodyDominance:        -0.40,
			containmentDominance: 0.10,
			opacityDominance:     0,
			depthDominance:       0.20,
		},
		nil,
		nil,
	)
	if !valid {
		t.Fatalf("expected observation to be resolvable")
	}
	if !leftBeforeRight {
		t.Fatalf("expected containment to decide when body conflicts with containment/depth and opacity is neutral")
	}
}

func TestResolvePairOrderFromObservationPrefersOpacityWhenBodyAndContainmentTie(t *testing.T) {
	leftBeforeRight, _, valid := resolvePairOrderFromObservation(
		1,
		2,
		materialPairObservation{
			sharedEvidenceCount:  8,
			leftCoverage:         0.33,
			rightCoverage:        0.30,
			bodyDominance:        0,
			containmentDominance: 0,
			opacityDominance:     -0.30,
			depthDominance:       0.30,
		},
		nil,
		nil,
	)
	if !valid {
		t.Fatalf("expected observation to be resolvable")
	}
	if leftBeforeRight {
		t.Fatalf("expected opacity dominance to decide before depth when earlier priorities tie")
	}
}

func TestResolvePairOrderFromObservationPrefersDepthWhenEarlierPrioritiesTie(t *testing.T) {
	leftBeforeRight, _, valid := resolvePairOrderFromObservation(
		1,
		2,
		materialPairObservation{
			sharedEvidenceCount:  8,
			leftCoverage:         0.90,
			rightCoverage:        0.56,
			bodyDominance:        0,
			containmentDominance: 0,
			opacityDominance:     0,
			depthDominance:       -0.05,
		},
		nil,
		nil,
	)
	if !valid {
		t.Fatalf("expected observation to be resolvable")
	}
	if leftBeforeRight {
		t.Fatalf("expected depth dominance to decide after body/containment/opacity tie")
	}
}

func TestResolvePairOrderFromObservationUsesBodyProximityWhenDominancesTie(t *testing.T) {
	leftBeforeRight, _, valid := resolvePairOrderFromObservation(
		1,
		2,
		materialPairObservation{
			sharedEvidenceCount:  8,
			leftCoverage:         0.78,
			rightCoverage:        0.54,
			bodyDominance:        0,
			containmentDominance: 0,
			opacityDominance:     0,
			depthDominance:       0,
		},
		map[int]float64{
			1: 0.80,
			2: 0.30,
		},
		nil,
	)
	if !valid {
		t.Fatalf("expected observation to be resolvable")
	}
	if leftBeforeRight {
		t.Fatalf("expected lower body proximity score right material to be selected first")
	}
}

func TestResolvePairOrderFromObservationUsesBodyOrderKeyWhenDominancesAndScoresTie(t *testing.T) {
	leftBeforeRight, _, valid := resolvePairOrderFromObservation(
		1,
		2,
		materialPairObservation{
			sharedEvidenceCount:  8,
			leftCoverage:         0.93,
			rightCoverage:        0.90,
			bodyDominance:        0,
			containmentDominance: 0,
			opacityDominance:     0,
			depthDominance:       0,
		},
		map[int]float64{
			1: 0.40,
			2: 0.40,
		},
		map[int]int{
			1: 1,
			2: 0,
		},
	)
	if !valid {
		t.Fatalf("expected observation to be resolvable")
	}
	if leftBeforeRight {
		t.Fatalf("expected body order key to select right material first when observation ties")
	}
}

func TestResolvePairOrderFromObservationFallsBackToOriginalIndexWhenAllSignalsTie(t *testing.T) {
	leftBeforeRight, _, valid := resolvePairOrderFromObservation(
		1,
		2,
		materialPairObservation{
			sharedEvidenceCount:  8,
			leftCoverage:         0.93,
			rightCoverage:        0.90,
			bodyDominance:        0,
			containmentDominance: 0,
			opacityDominance:     0,
			depthDominance:       0,
		},
		nil,
		nil,
	)
	if !valid {
		t.Fatalf("expected observation to be resolvable")
	}
	if !leftBeforeRight {
		t.Fatalf("expected original material index to be the final fallback")
	}
}

func TestResolvePairOrderConstraintUsesBodyOrderKeyForTiedObservation(t *testing.T) {
	spatialInfoMap := map[int]materialSpatialInfo{
		1: buildOverlapSpatialInfoForTest(0.20),
		2: buildOverlapSpatialInfoForTest(0.20),
	}

	leftBeforeRight, _, valid := resolvePairOrderConstraint(
		1,
		2,
		spatialInfoMap,
		0.10,
		map[int]float64{
			1: 0.30,
			2: 0.30,
		},
		nil,
		map[int]float64{
			1: 0.40,
			2: 0.20,
		},
	)
	if !valid {
		t.Fatalf("expected tied observation to be resolved by body order key")
	}
	if leftBeforeRight {
		t.Fatalf("expected right material to be selected first by body order key")
	}
}

func TestResolveMaterialOrderByConnectedComponentsKeepsComponentOrder(t *testing.T) {
	got := resolveMaterialOrderByConnectedComponents(
		4,
		[]materialOrderConstraint{
			{from: 0, to: 1, confidence: 2.0},
			{from: 3, to: 2, confidence: 1.0},
		},
		[]int{0, 1, 2, 3},
		[]int{0, 1, 2, 3},
	)
	want := []int{0, 1, 3, 2}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("connected component order mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestResolveMaterialOrderByConnectedComponentsKeepsOriginalOrderWithoutConstraints(t *testing.T) {
	got := resolveMaterialOrderByConnectedComponents(
		3,
		nil,
		[]int{0, 1, 2},
		[]int{2, 0, 1},
	)
	want := []int{0, 1, 2}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("no-constraint order mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestResolveMaterialOrderByConnectedComponentsUsesBodyOrderKeyWithinCycle(t *testing.T) {
	got := resolveMaterialOrderByConnectedComponents(
		3,
		[]materialOrderConstraint{
			{from: 0, to: 1, confidence: 1.0},
			{from: 1, to: 2, confidence: 1.0},
			{from: 2, to: 0, confidence: 1.0},
		},
		[]int{0, 1, 2},
		[]int{1, 2, 0},
	)
	want := []int{2, 0, 1}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("cycle order mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestResolveMaterialOrderByConnectedComponentsUsesOriginalOrderWhenCycleBodyKeysTie(t *testing.T) {
	got := resolveMaterialOrderByConnectedComponents(
		3,
		[]materialOrderConstraint{
			{from: 0, to: 1, confidence: 1.0},
			{from: 1, to: 2, confidence: 1.0},
			{from: 2, to: 0, confidence: 1.0},
		},
		[]int{0, 1, 2},
		[]int{0, 0, 0},
	)
	want := []int{0, 1, 2}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("cycle original-order fallback mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestRebuildTransparentMaterialOrderGroupsByMemberOrderUsesAverageRanking(t *testing.T) {
	got := rebuildTransparentMaterialOrderGroupsByMemberOrder(
		[]transparentMaterialOrderGroup{
			{key: "g1", members: []int{0, 1, 2}},
			{key: "g2", members: []int{3, 4, 5}},
		},
		[]int{3, 0, 1, 4, 2, 5},
	)
	want := []int{0, 1, 2, 3, 4, 5}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("rebuilt group order mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestAggregateTransparentMaterialGroupPairConstraintAccumulatesDirectionConfidence(t *testing.T) {
	spatialInfoMap := map[int]materialSpatialInfo{
		1: buildOverlapSpatialInfoForTest(0.40),
		2: buildOverlapSpatialInfoForTest(0.20),
		3: buildOverlapSpatialInfoForTest(0.10),
	}
	want13, ok := analyzeMaterialPairObservation(
		1,
		3,
		spatialInfoMap,
		0.10,
		map[int]float64{1: 0.80, 2: 0.60, 3: 0.20},
		nil,
	)
	if !ok {
		t.Fatalf("expected pair 1-3 to be observable")
	}
	want23, ok := analyzeMaterialPairObservation(
		2,
		3,
		spatialInfoMap,
		0.10,
		map[int]float64{1: 0.80, 2: 0.60, 3: 0.20},
		nil,
	)
	if !ok {
		t.Fatalf("expected pair 2-3 to be observable")
	}
	want13LeftBefore, want13Confidence, valid := resolvePairOrderByOverlap(
		1,
		3,
		spatialInfoMap,
		0.10,
		map[int]float64{1: 0.80, 2: 0.60, 3: 0.20},
		nil,
	)
	if !valid {
		t.Fatalf("expected pair 1-3 to be resolvable")
	}
	want23LeftBefore, want23Confidence, valid := resolvePairOrderByOverlap(
		2,
		3,
		spatialInfoMap,
		0.10,
		map[int]float64{1: 0.80, 2: 0.60, 3: 0.20},
		nil,
	)
	if !valid {
		t.Fatalf("expected pair 2-3 to be resolvable")
	}

	got, ok := aggregateTransparentMaterialGroupPairConstraint(
		transparentMaterialOrderGroup{members: []int{1, 2}},
		transparentMaterialOrderGroup{members: []int{3}},
		spatialInfoMap,
		0.10,
		map[int]float64{1: 0.80, 2: 0.60, 3: 0.20},
		nil,
		nil,
		nil,
	)
	if !ok {
		t.Fatalf("expected aggregated group constraint to be valid")
	}

	if got.observation.sharedEvidenceCount != want13.sharedEvidenceCount+want23.sharedEvidenceCount {
		t.Fatalf("shared evidence mismatch: got=%d want=%d", got.observation.sharedEvidenceCount, want13.sharedEvidenceCount+want23.sharedEvidenceCount)
	}
	wantBodyDominance := (want13.bodyDominance + want23.bodyDominance) / 2
	if math.Abs(got.observation.bodyDominance-wantBodyDominance) > 1e-9 {
		t.Fatalf("body dominance mismatch: got=%f want=%f", got.observation.bodyDominance, wantBodyDominance)
	}
	wantOpacityDominance := (want13.opacityDominance + want23.opacityDominance) / 2
	if math.Abs(got.observation.opacityDominance-wantOpacityDominance) > 1e-9 {
		t.Fatalf("opacity dominance mismatch: got=%f want=%f", got.observation.opacityDominance, wantOpacityDominance)
	}
	wantLeftCoverage := (want13.leftCoverage + want23.leftCoverage) / 2
	if math.Abs(got.observation.leftCoverage-wantLeftCoverage) > 1e-9 {
		t.Fatalf("left coverage mismatch: got=%f want=%f", got.observation.leftCoverage, wantLeftCoverage)
	}

	wantLeftBeforeConfidence := 0.0
	wantRightBeforeConfidence := 0.0
	if want13LeftBefore {
		wantLeftBeforeConfidence += want13Confidence
	} else {
		wantRightBeforeConfidence += want13Confidence
	}
	if want23LeftBefore {
		wantLeftBeforeConfidence += want23Confidence
	} else {
		wantRightBeforeConfidence += want23Confidence
	}
	if math.Abs(got.leftBeforeConfidence-wantLeftBeforeConfidence) > 1e-9 {
		t.Fatalf("left-before confidence mismatch: got=%f want=%f", got.leftBeforeConfidence, wantLeftBeforeConfidence)
	}
	if math.Abs(got.rightBeforeConfidence-wantRightBeforeConfidence) > 1e-9 {
		t.Fatalf("right-before confidence mismatch: got=%f want=%f", got.rightBeforeConfidence, wantRightBeforeConfidence)
	}
}

func TestCollectMatchedBodyDistanceDominancesUsesMatchedPairs(t *testing.T) {
	got := collectMatchedBodyDistanceDominances(
		materialSpatialInfo{
			bodyDistance: []float64{0.20, 0.90},
		},
		materialSpatialInfo{
			bodyDistance: []float64{0.80, 0.10},
		},
		[]materialPointPairMatch{
			{leftIndex: 0, rightIndex: 0},
			{leftIndex: 1, rightIndex: 1},
		},
	)
	want := []float64{0.60, -0.80}
	if len(got) != len(want) {
		t.Fatalf("matched body dominances count mismatch: got=%v want=%v", got, want)
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("matched body dominance mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestBuildTransparentMaterialGroupBodyProximityScoresUsesMedian(t *testing.T) {
	got := buildTransparentMaterialGroupBodyProximityScores(
		[]transparentMaterialOrderGroup{
			{members: []int{0, 1, 2}},
			{members: []int{3}},
		},
		map[int]float64{
			0: 0.60,
			1: 0.20,
			2: 0.40,
			3: 0.10,
		},
	)
	if math.Abs(got[0]-0.40) > 1e-9 {
		t.Fatalf("group 0 score mismatch: got=%f want=%f", got[0], 0.40)
	}
	if math.Abs(got[1]-0.10) > 1e-9 {
		t.Fatalf("group 1 score mismatch: got=%f want=%f", got[1], 0.10)
	}
}

func TestResolvePairOrderConstraintReturnsInvalidWithoutSharedObservation(t *testing.T) {
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
	if valid {
		t.Fatalf("expected unresolved pair without shared observation: leftBeforeRight=%t", leftBeforeRight)
	}
}

func TestSortTransparentMaterialGroupMembersOutputsBackFrontEdge(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_表面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_裏面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_エッジ", 1.0, 3))

	got := sortTransparentMaterialGroupMembers(modelData, []int{0, 1, 2})
	want := []int{1, 0, 2}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("group member order mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestBuildTransparentMaterialOrderGroupsKeepsVariantFamiliesContinuous(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_表面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Skirt_01_CLOTH_表面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_裏面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_エッジ", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Skirt_01_CLOTH_裏面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Skirt_01_CLOTH_エッジ", 1.0, 3))

	got := buildTransparentMaterialOrderGroups(modelData, []int{0, 1, 2, 3, 4, 5})
	if len(got) != 2 {
		t.Fatalf("group count mismatch: got=%d want=2 groups=%+v", len(got), got)
	}

	if got[0].representative != 0 {
		t.Fatalf("1st representative mismatch: got=%d want=0", got[0].representative)
	}
	wantFirstMembers := []int{2, 0, 3}
	for i := range wantFirstMembers {
		if i >= len(got[0].members) || got[0].members[i] != wantFirstMembers[i] {
			t.Fatalf("1st group members mismatch: got=%v want=%v", got[0].members, wantFirstMembers)
		}
	}

	if got[1].representative != 1 {
		t.Fatalf("2nd representative mismatch: got=%d want=1", got[1].representative)
	}
	wantSecondMembers := []int{4, 1, 5}
	for i := range wantSecondMembers {
		if i >= len(got[1].members) || got[1].members[i] != wantSecondMembers[i] {
			t.Fatalf("2nd group members mismatch: got=%v want=%v", got[1].members, wantSecondMembers)
		}
	}
}

func TestBuildTransparentMaterialOrderGroupsSeparatesRepresentativeAndOutputOrder(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_表面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_裏", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_エッジ", 1.0, 3))

	got := buildTransparentMaterialOrderGroups(modelData, []int{2, 1, 0})
	if len(got) != 1 {
		t.Fatalf("group count mismatch: got=%d want=1 groups=%+v", len(got), got)
	}
	if got[0].representative != 0 {
		t.Fatalf("representative should stay on front surface: got=%d want=0", got[0].representative)
	}

	wantMembers := []int{1, 0, 2}
	for i := range wantMembers {
		if i >= len(got[0].members) || got[0].members[i] != wantMembers[i] {
			t.Fatalf("group members mismatch: got=%v want=%v", got[0].members, wantMembers)
		}
	}
}

func TestSelectMaterialVariantRepresentativeKeepsFrontSurface(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_表面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_裏面", 1.0, 3))
	modelData.Materials.AppendRaw(newMaterial("Tops_01_CLOTH_エッジ", 1.0, 3))

	got := selectMaterialVariantRepresentative(modelData, 1, 0)
	if got != 0 {
		t.Fatalf("representative should stay on front surface: got=%d", got)
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
