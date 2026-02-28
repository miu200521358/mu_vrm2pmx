// 指示: miu200521358
package vrm

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	modelvrm "github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
	warningid "github.com/miu200521358/mu_vrm2pmx/pkg/domain/model"
	"gonum.org/v1/gonum/spatial/r3"
)

func TestAppendSpecialEyeMaterialMorphsFromFallbackRulesGeneratesAugmentedMaterialsAndMorphs(t *testing.T) {
	modelData := model.NewPmxModel()

	appendTexture := func(name string) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}

	irisTextureIndex := appendTexture("eye_iris.png")
	whiteTextureIndex := appendTexture("eye_white.png")
	eyeLineTextureIndex := appendTexture("eye_line.png")
	eyeLashTextureIndex := appendTexture("eye_lash.png")
	appendTexture("effect_eye_star.png")
	appendTexture("effect_eye_heart.png")
	appendTexture("effect_eye_hau.png")
	appendTexture("effect_eye_hachume.png")
	appendTexture("effect_eye_nagomi.png")

	appendBaseMaterial := func(name string, textureIndex int) int {
		materialData := model.NewMaterial()
		materialData.SetName(name)
		materialData.EnglishName = name
		materialData.Diffuse = mmath.Vec4{X: 1.0, Y: 1.0, Z: 1.0, W: 1.0}
		materialData.DrawFlag = model.DRAW_FLAG_DRAWING_EDGE
		materialData.TextureIndex = textureIndex
		return modelData.Materials.AppendRaw(materialData)
	}

	irisMaterialIndex := appendBaseMaterial("Face_EyeIris_00", irisTextureIndex)
	whiteMaterialIndex := appendBaseMaterial("Face_EyeWhite_00", whiteTextureIndex)
	eyeLineMaterialIndex := appendBaseMaterial("Face_EyeLine_00", eyeLineTextureIndex)
	eyeLashMaterialIndex := appendBaseMaterial("Face_EyeLash_00", eyeLashTextureIndex)

	appendFaceByMaterial := func(materialIndex int, xOffset float64) {
		vertexStart := modelData.Vertices.Len()
		modelData.Vertices.AppendRaw(&model.Vertex{
			Position:        mmath.Vec3{Vec: r3.Vec{X: xOffset, Y: 0.0, Z: 0.0}},
			MaterialIndexes: []int{materialIndex},
		})
		modelData.Vertices.AppendRaw(&model.Vertex{
			Position:        mmath.Vec3{Vec: r3.Vec{X: xOffset + 0.1, Y: 0.0, Z: 0.0}},
			MaterialIndexes: []int{materialIndex},
		})
		modelData.Vertices.AppendRaw(&model.Vertex{
			Position:        mmath.Vec3{Vec: r3.Vec{X: xOffset, Y: 0.1, Z: 0.0}},
			MaterialIndexes: []int{materialIndex},
		})
		modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{vertexStart, vertexStart + 1, vertexStart + 2}})
		materialData, err := modelData.Materials.Get(materialIndex)
		if err != nil || materialData == nil {
			t.Fatalf("material not found: index=%d err=%v", materialIndex, err)
		}
		materialData.VerticesCount += 3
	}

	appendFaceByMaterial(irisMaterialIndex, 0.0)
	appendFaceByMaterial(whiteMaterialIndex, 1.0)
	appendFaceByMaterial(eyeLineMaterialIndex, 2.0)
	appendFaceByMaterial(eyeLashMaterialIndex, 3.0)

	appendSpecialEyeMaterialMorphsFromFallbackRules(modelData, nil, newTargetMorphRegistry())

	if modelData.Materials.Len() != 9 {
		t.Fatalf("material count mismatch: got=%d want=9", modelData.Materials.Len())
	}
	if modelData.Faces.Len() != 9 {
		t.Fatalf("face count mismatch: got=%d want=9", modelData.Faces.Len())
	}

	if len(findMaterialIndexesBySuffixToken(modelData, "eye_star")) == 0 {
		t.Fatal("augmented material eye_star should exist")
	}
	if len(findMaterialIndexesBySuffixToken(modelData, "eye_heart")) == 0 {
		t.Fatal("augmented material eye_heart should exist")
	}
	if len(findMaterialIndexesBySuffixToken(modelData, "eye_hau")) == 0 {
		t.Fatal("augmented material eye_hau should exist")
	}
	if len(findMaterialIndexesBySuffixToken(modelData, "eye_hachume")) == 0 {
		t.Fatal("augmented material eye_hachume should exist")
	}
	if len(findMaterialIndexesBySuffixToken(modelData, "eye_nagomi")) == 0 {
		t.Fatal("augmented material eye_nagomi should exist")
	}

	hauMorph, err := modelData.Morphs.GetByName("はぅ材質")
	if err != nil || hauMorph == nil {
		t.Fatalf("はぅ材質 morph not found: err=%v", err)
	}
	if hauMorph.MorphType != model.MORPH_TYPE_MATERIAL {
		t.Fatalf("はぅ材質 morph type mismatch: got=%d want=%d", hauMorph.MorphType, model.MORPH_TYPE_MATERIAL)
	}
	hauOffsets := collectMaterialOffsetByIndex(hauMorph)
	if !hasPositiveMaterialAlphaOffset(hauOffsets) {
		t.Fatal("はぅ材質 should include show alpha offsets")
	}
	if hauOffset, exists := hauOffsets[whiteMaterialIndex]; !exists || hauOffset.Diffuse.W >= 0 {
		t.Fatalf("white material should be hidden by はぅ材質: offset=%+v exists=%t", hauOffset, exists)
	}
	if hauOffset, exists := hauOffsets[eyeLineMaterialIndex]; !exists || hauOffset.Diffuse.W >= 0 {
		t.Fatalf("eyeline material should be hidden by はぅ材質: offset=%+v exists=%t", hauOffset, exists)
	}
	if hauOffset, exists := hauOffsets[eyeLashMaterialIndex]; !exists || hauOffset.Diffuse.W >= 0 {
		t.Fatalf("eyelash material should be hidden by はぅ材質: offset=%+v exists=%t", hauOffset, exists)
	}

	starMorph, err := modelData.Morphs.GetByName("星目材質")
	if err != nil || starMorph == nil {
		t.Fatalf("星目材質 morph not found: err=%v", err)
	}
	if starMorph.MorphType != model.MORPH_TYPE_MATERIAL {
		t.Fatalf("星目材質 morph type mismatch: got=%d want=%d", starMorph.MorphType, model.MORPH_TYPE_MATERIAL)
	}
	starOffsets := collectMaterialOffsetByIndex(starMorph)
	if !hasPositiveMaterialAlphaOffset(starOffsets) {
		t.Fatal("星目材質 should include show alpha offsets")
	}
	if _, exists := starOffsets[whiteMaterialIndex]; exists {
		t.Fatal("星目材質 should not hide white material")
	}
	if _, exists := starOffsets[eyeLineMaterialIndex]; exists {
		t.Fatal("星目材質 should not hide eyeline material")
	}
	if _, exists := starOffsets[eyeLashMaterialIndex]; exists {
		t.Fatal("星目材質 should not hide eyelash material")
	}
}

func TestAppendSpecialEyeMaterialMorphsFromFallbackRulesGeneratesCheekDyeMorphFromExistingMaterial(t *testing.T) {
	modelData := model.NewPmxModel()

	appendTexture := func(name string) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}

	faceTextureIndex := appendTexture("effect_cheek_dye.png")

	faceMaterial := model.NewMaterial()
	faceMaterial.SetName("Face_00_SKIN_cheek_dye")
	faceMaterial.EnglishName = "Face_00_SKIN_cheek_dye"
	faceMaterial.TextureIndex = faceTextureIndex
	faceMaterial.Diffuse = mmath.Vec4{X: 1.0, Y: 1.0, Z: 1.0, W: 0.0}
	faceMaterial.DrawFlag = model.DRAW_FLAG_DRAWING_EDGE
	faceMaterialIndex := modelData.Materials.AppendRaw(faceMaterial)

	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 0.0, Z: 0.0}},
		MaterialIndexes: []int{faceMaterialIndex},
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        mmath.Vec3{Vec: r3.Vec{X: 0.1, Y: 0.0, Z: 0.0}},
		MaterialIndexes: []int{faceMaterialIndex},
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 0.1, Z: 0.0}},
		MaterialIndexes: []int{faceMaterialIndex},
	})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{0, 1, 2}})
	faceMaterial.VerticesCount = 3

	appendSpecialEyeMaterialMorphsFromFallbackRules(modelData, nil, newTargetMorphRegistry())

	cheekMorph, err := modelData.Morphs.GetByName("照れ")
	if err != nil || cheekMorph == nil {
		t.Fatalf("照れ morph not found: err=%v", err)
	}
	if cheekMorph.Panel != model.MORPH_PANEL_OTHER_LOWER_RIGHT {
		t.Fatalf("照れ panel mismatch: got=%d want=%d", cheekMorph.Panel, model.MORPH_PANEL_OTHER_LOWER_RIGHT)
	}
	if cheekMorph.MorphType != model.MORPH_TYPE_MATERIAL {
		t.Fatalf("照れ morph type mismatch: got=%d want=%d", cheekMorph.MorphType, model.MORPH_TYPE_MATERIAL)
	}
	cheekOffsets := collectMaterialOffsetByIndex(cheekMorph)
	if !hasPositiveMaterialAlphaOffset(cheekOffsets) {
		t.Fatal("照れ should include show alpha offsets")
	}
	offsetData, exists := cheekOffsets[faceMaterialIndex]
	if !exists || offsetData == nil {
		t.Fatal("照れ should target cheek_dye material")
	}
	if math.Abs(offsetData.Diffuse.W-1.0) > 1e-9 {
		t.Fatalf("照れ alpha delta mismatch: got=%.8f want=1.0", offsetData.Diffuse.W)
	}
}

func TestAppendSpecialEyeMaterialMorphsFromFallbackRulesGeneratesCheekDyeAugmentedMaterialFromFaceEnglishName(t *testing.T) {
	modelData := model.NewPmxModel()

	appendTexture := func(name string) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}

	faceTextureIndex := appendTexture("face_skin_base.png")
	appendTexture("effect_cheek_dye.png")

	faceMaterial := model.NewMaterial()
	faceMaterial.SetName("N00_000_FACE_00")
	faceMaterial.EnglishName = "N00_000_Face_00_FACE"
	faceMaterial.TextureIndex = faceTextureIndex
	faceMaterial.Diffuse = mmath.Vec4{X: 1.0, Y: 1.0, Z: 1.0, W: 1.0}
	faceMaterial.Specular = mmath.Vec4{X: 0.2, Y: 0.3, Z: 0.4, W: 0.5}
	faceMaterial.EdgeSize = 1.25
	faceMaterial.DrawFlag = model.DRAW_FLAG_DRAWING_EDGE
	faceMaterialIndex := modelData.Materials.AppendRaw(faceMaterial)

	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 0.0, Z: 0.0}},
		MaterialIndexes: []int{faceMaterialIndex},
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        mmath.Vec3{Vec: r3.Vec{X: 0.1, Y: 0.0, Z: 0.0}},
		MaterialIndexes: []int{faceMaterialIndex},
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 0.1, Z: 0.0}},
		MaterialIndexes: []int{faceMaterialIndex},
	})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{0, 1, 2}})
	faceMaterial.VerticesCount = 3

	appendSpecialEyeMaterialMorphsFromFallbackRules(modelData, nil, newTargetMorphRegistry())

	cheekMaterialIndexes := findMaterialIndexesBySuffixToken(modelData, "cheek_dye")
	if len(cheekMaterialIndexes) == 0 {
		t.Fatal("cheek_dye augmented material should be generated from _Face_ englishName")
	}
	cheekMaterialIndex := cheekMaterialIndexes[0]

	cheekMaterial, err := modelData.Materials.Get(cheekMaterialIndex)
	if err != nil || cheekMaterial == nil {
		t.Fatalf("cheek_dye augmented material not found: err=%v", err)
	}
	if math.Abs(cheekMaterial.Diffuse.W) > 1e-9 {
		t.Fatalf("cheek_dye augmented material should start hidden: alpha=%f", cheekMaterial.Diffuse.W)
	}
	if (cheekMaterial.DrawFlag & model.DRAW_FLAG_DRAWING_EDGE) != 0 {
		t.Fatal("cheek_dye augmented material should disable edge drawing")
	}
	if math.Abs(cheekMaterial.EdgeSize-faceMaterial.EdgeSize) > 1e-9 {
		t.Fatalf("cheek_dye augmented material should keep edge size: got=%f want=%f", cheekMaterial.EdgeSize, faceMaterial.EdgeSize)
	}
	if !cheekMaterial.Specular.NearEquals(faceMaterial.Specular, 1e-9) {
		t.Fatalf("cheek_dye augmented material should keep specular: got=%v want=%v", cheekMaterial.Specular, faceMaterial.Specular)
	}

	cheekMorph, err := modelData.Morphs.GetByName("照れ")
	if err != nil || cheekMorph == nil {
		t.Fatalf("照れ morph not found: err=%v", err)
	}
	cheekOffsets := collectMaterialOffsetByIndex(cheekMorph)
	offsetData, exists := cheekOffsets[cheekMaterialIndex]
	if !exists || offsetData == nil {
		t.Fatal("照れ should target generated cheek_dye material")
	}
	if math.Abs(offsetData.Diffuse.W-1.0) > 1e-9 {
		t.Fatalf("照れ alpha delta mismatch: got=%.8f want=1.0", offsetData.Diffuse.W)
	}
	if _, exists := cheekOffsets[faceMaterialIndex]; exists {
		t.Fatal("照れ should not fallback to base face material")
	}
}

func TestAppendSpecialEyeMaterialMorphsFromFallbackRulesSkipsCheekMorphWithoutCheekDyeMaterial(t *testing.T) {
	modelData := model.NewPmxModel()

	appendTexture := func(name string) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}

	faceTextureIndex := appendTexture("face_skin_base.png")

	faceMaterial := model.NewMaterial()
	faceMaterial.SetName("N00_000_FACE_00")
	faceMaterial.EnglishName = "N00_000_Face_00_FACE"
	faceMaterial.TextureIndex = faceTextureIndex
	faceMaterial.Diffuse = mmath.Vec4{X: 1.0, Y: 1.0, Z: 1.0, W: 1.0}
	faceMaterial.DrawFlag = model.DRAW_FLAG_DRAWING_EDGE
	faceMaterialIndex := modelData.Materials.AppendRaw(faceMaterial)

	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 0.0, Z: 0.0}},
		MaterialIndexes: []int{faceMaterialIndex},
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        mmath.Vec3{Vec: r3.Vec{X: 0.1, Y: 0.0, Z: 0.0}},
		MaterialIndexes: []int{faceMaterialIndex},
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 0.1, Z: 0.0}},
		MaterialIndexes: []int{faceMaterialIndex},
	})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{0, 1, 2}})
	faceMaterial.VerticesCount = 3

	appendSpecialEyeMaterialMorphsFromFallbackRules(modelData, nil, newTargetMorphRegistry())

	if _, err := modelData.Morphs.GetByName("照れ"); err == nil {
		t.Fatal("照れ should not be generated when cheek_dye material is unavailable")
	}
}

func TestResolveSpecialEyeTokenMatchLevelPriority(t *testing.T) {
	texturePreferred := specialEyeMaterialInfo{
		NormalizedTextureMatch: normalizeSpecialEyeToken("asset/effect_eye_star.png"),
		NormalizedEnglishName:  normalizeSpecialEyeToken("effect_eye_star"),
		NormalizedName:         normalizeSpecialEyeToken("face_eye_star"),
	}
	if got := resolveSpecialEyeTokenMatchLevel(texturePreferred, "eye_star"); got != 1 {
		t.Fatalf("texture priority mismatch: got=%d want=1", got)
	}

	englishPreferred := specialEyeMaterialInfo{
		NormalizedTextureMatch: normalizeSpecialEyeToken("asset/eye_other.png"),
		NormalizedEnglishName:  normalizeSpecialEyeToken("effect_eye_hau"),
		NormalizedName:         normalizeSpecialEyeToken("face_overlay"),
	}
	if got := resolveSpecialEyeTokenMatchLevel(englishPreferred, "eye_hau"); got != 2 {
		t.Fatalf("english priority mismatch: got=%d want=2", got)
	}

	nameSuffixFallback := specialEyeMaterialInfo{
		NormalizedTextureMatch: normalizeSpecialEyeToken("asset/eye_other.png"),
		NormalizedEnglishName:  normalizeSpecialEyeToken("effect_overlay"),
		NormalizedName:         normalizeSpecialEyeToken("face_overlay_eye_hachume"),
	}
	if got := resolveSpecialEyeTokenMatchLevel(nameSuffixFallback, "eye_hachume"); got != 3 {
		t.Fatalf("name suffix priority mismatch: got=%d want=3", got)
	}

	noMatch := specialEyeMaterialInfo{
		NormalizedTextureMatch: normalizeSpecialEyeToken("asset/eye_other.png"),
		NormalizedEnglishName:  normalizeSpecialEyeToken("effect_overlay"),
		NormalizedName:         normalizeSpecialEyeToken("face_overlay"),
	}
	if got := resolveSpecialEyeTokenMatchLevel(noMatch, "eye_nagomi"); got != 0 {
		t.Fatalf("no match mismatch: got=%d want=0", got)
	}
}

func TestResolveSpecialEyeAugmentedMaterialNameRemovesInstanceSuffix(t *testing.T) {
	testCases := []struct {
		name     string
		baseName string
		token    string
		expected string
	}{
		{
			name:     "with_space_instance",
			baseName: "EyeIris_00_EYE (Instance)",
			token:    "eye_star",
			expected: "EyeIris_00_EYE_eye_star",
		},
		{
			name:     "without_space_instance",
			baseName: "EyeIris_00_EYE(Instance)",
			token:    "eye_star",
			expected: "EyeIris_00_EYE_eye_star",
		},
		{
			name:     "no_instance_suffix",
			baseName: "EyeIris_00_EYE",
			token:    "eye_star",
			expected: "EyeIris_00_EYE_eye_star",
		},
	}
	for _, testCase := range testCases {
		got := resolveSpecialEyeAugmentedMaterialName(testCase.baseName, testCase.token)
		if got != testCase.expected {
			t.Fatalf("augmented material name mismatch: case=%s got=%s want=%s", testCase.name, got, testCase.expected)
		}
	}
}

func TestAppendPrimitiveMaterialAppliesSpecularAndEdgeRule(t *testing.T) {
	type testCase struct {
		name        string
		material    string
		alphaMode   string
		withTexture bool
		wantEdge    bool
	}
	cases := []testCase{
		{
			name:        "body_opaque",
			material:    "Body_00_SKIN",
			alphaMode:   "OPAQUE",
			withTexture: true,
			wantEdge:    true,
		},
		{
			name:        "cloth_blend",
			material:    "Tops_01_CLOTH",
			alphaMode:   "BLEND",
			withTexture: true,
			wantEdge:    true,
		},
		{
			name:        "cloth_mask",
			material:    "Tops_01_CLOTH",
			alphaMode:   "MASK",
			withTexture: true,
			wantEdge:    true,
		},
		{
			name:        "cloth_opaque",
			material:    "Tops_01_CLOTH",
			alphaMode:   "OPAQUE",
			withTexture: true,
			wantEdge:    false,
		},
		{
			name:        "cloth_blend_without_texture",
			material:    "Tops_01_CLOTH",
			alphaMode:   "BLEND",
			withTexture: false,
			wantEdge:    false,
		},
		{
			name:        "cheek_dye_overlay",
			material:    "Face_00_SKIN_cheek_dye",
			alphaMode:   "BLEND",
			withTexture: true,
			wantEdge:    false,
		},
	}
	for _, tc := range cases {
		modelData := model.NewPmxModel()
		doc := &gltfDocument{
			Materials: []gltfMaterial{
				{
					Name:      tc.material,
					AlphaMode: tc.alphaMode,
					PbrMetallicRoughness: gltfPbrMetallicRoughness{
						BaseColorFactor: []float64{1, 1, 1, 1},
					},
				},
			},
		}
		textureIndexesByImage := []int{}
		if tc.withTexture {
			source := 0
			doc.Materials[0].PbrMetallicRoughness.BaseColorTexture = &gltfTextureRef{Index: 0}
			doc.Textures = []gltfTexture{
				{Source: &source},
			}
			doc.Images = []gltfImage{
				{Name: "base.png"},
			}
			textureIndexesByImage = []int{0}
		}
		materialIndex := 0
		primitive := gltfPrimitive{Material: &materialIndex}

		appendedIndex := appendPrimitiveMaterial(
			modelData,
			doc,
			primitive,
			tc.material,
			textureIndexesByImage,
			3,
			newTargetMorphRegistry(),
		)
		materialData, err := modelData.Materials.Get(appendedIndex)
		if err != nil || materialData == nil {
			t.Fatalf("appendPrimitiveMaterial failed: case=%s err=%v", tc.name, err)
		}
		if math.Abs(materialData.Specular.W) > 1e-9 {
			t.Fatalf("specular W should be zero: case=%s got=%f", tc.name, materialData.Specular.W)
		}
		gotEdge := (materialData.DrawFlag & model.DRAW_FLAG_DRAWING_EDGE) != 0
		if gotEdge != tc.wantEdge {
			t.Fatalf("edge flag mismatch: case=%s got=%t want=%t flag=%d", tc.name, gotEdge, tc.wantEdge, materialData.DrawFlag)
		}
	}
}

func TestAppendPrimitiveMaterialAppliesVrm0OutlineProperties(t *testing.T) {
	type testCase struct {
		name             string
		outlineWidth     float64
		outlineWidthMode float64
		wantEdgeSize     float64
		wantDrawEdge     bool
	}
	cases := []testCase{
		{
			name:             "world_coordinates",
			outlineWidth:     0.24,
			outlineWidthMode: 1.0,
			wantEdgeSize:     3.0,
			wantDrawEdge:     true,
		},
		{
			name:             "none_mode",
			outlineWidth:     0.24,
			outlineWidthMode: 0.0,
			wantEdgeSize:     0.0,
			wantDrawEdge:     false,
		},
	}

	for _, tc := range cases {
		modelData := model.NewPmxModel()
		materialName := "Body_00_SKIN"
		vrmExtensionRaw, err := json.Marshal(map[string]any{
			"materialProperties": []any{
				map[string]any{
					"name": materialName,
					"floatProperties": map[string]any{
						"_OutlineWidth":     tc.outlineWidth,
						"_OutlineWidthMode": tc.outlineWidthMode,
					},
					"vectorProperties": map[string]any{
						"_OutlineColor": []any{0.2, 0.4, 0.6, 0.8},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to marshal VRM extension: case=%s err=%v", tc.name, err)
		}

		doc := &gltfDocument{
			Materials: []gltfMaterial{
				{
					Name:      materialName,
					AlphaMode: "OPAQUE",
					PbrMetallicRoughness: gltfPbrMetallicRoughness{
						BaseColorFactor: []float64{1, 1, 1, 1},
					},
				},
			},
			Extensions: map[string]json.RawMessage{
				"VRM": vrmExtensionRaw,
			},
		}
		materialIndex := 0
		primitive := gltfPrimitive{Material: &materialIndex}

		appendedIndex := appendPrimitiveMaterial(
			modelData,
			doc,
			primitive,
			materialName,
			nil,
			3,
			newTargetMorphRegistry(),
		)
		materialData, err := modelData.Materials.Get(appendedIndex)
		if err != nil || materialData == nil {
			t.Fatalf("appendPrimitiveMaterial failed: case=%s err=%v", tc.name, err)
		}

		wantEdgeColor := mmath.Vec4{X: 0.2, Y: 0.4, Z: 0.6, W: 0.8}
		if !materialData.Edge.NearEquals(wantEdgeColor, 1e-9) {
			t.Fatalf("edge color mismatch: case=%s got=%v want=%v", tc.name, materialData.Edge, wantEdgeColor)
		}
		if math.Abs(materialData.EdgeSize-tc.wantEdgeSize) > 1e-9 {
			t.Fatalf("edge size mismatch: case=%s got=%f want=%f", tc.name, materialData.EdgeSize, tc.wantEdgeSize)
		}
		gotDrawEdge := (materialData.DrawFlag & model.DRAW_FLAG_DRAWING_EDGE) != 0
		if gotDrawEdge != tc.wantDrawEdge {
			t.Fatalf("edge flag mismatch: case=%s got=%t want=%t flag=%d", tc.name, gotDrawEdge, tc.wantDrawEdge, materialData.DrawFlag)
		}
	}
}

func TestAppendPrimitiveMaterialResolvesLegacyToonTextureForVroidProfile(t *testing.T) {
	modelData := newVroidProfileTestModelData()
	materialName := "Body_00_SKIN"
	vrmExtensionRaw, err := json.Marshal(map[string]any{
		"materialProperties": []any{
			map[string]any{
				"name": materialName,
				"vectorProperties": map[string]any{
					"_ShadeColor": []any{1.0, 0.5, 0.25, 1.0},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal VRM extension: %v", err)
	}
	doc := &gltfDocument{
		Materials: []gltfMaterial{
			{
				Name:      materialName,
				AlphaMode: "OPAQUE",
				PbrMetallicRoughness: gltfPbrMetallicRoughness{
					BaseColorFactor: []float64{1, 1, 1, 1},
				},
			},
		},
		Extensions: map[string]json.RawMessage{
			"VRM": vrmExtensionRaw,
		},
	}
	materialIndex := 0
	primitive := gltfPrimitive{Material: &materialIndex}

	appendedIndex := appendPrimitiveMaterial(
		modelData,
		doc,
		primitive,
		materialName,
		nil,
		3,
		newTargetMorphRegistry(),
	)
	materialData, getErr := modelData.Materials.Get(appendedIndex)
	if getErr != nil || materialData == nil {
		t.Fatalf("appendPrimitiveMaterial failed: err=%v", getErr)
	}
	if materialData.ToonSharingFlag != model.TOON_SHARING_INDIVIDUAL {
		t.Fatalf("toon sharing flag mismatch: got=%d want=%d", materialData.ToonSharingFlag, model.TOON_SHARING_INDIVIDUAL)
	}
	if materialData.ToonTextureIndex < 0 {
		t.Fatalf("toon texture index should be generated: got=%d", materialData.ToonTextureIndex)
	}
	toonTexture, getTextureErr := modelData.Textures.Get(materialData.ToonTextureIndex)
	if getTextureErr != nil || toonTexture == nil {
		t.Fatalf("toon texture not found: index=%d err=%v", materialData.ToonTextureIndex, getTextureErr)
	}
	if filepath.ToSlash(toonTexture.Name()) != "tex/toon/toon_000_ff8040.bmp" {
		t.Fatalf("toon texture name mismatch: got=%s want=%s", filepath.ToSlash(toonTexture.Name()), "tex/toon/toon_000_ff8040.bmp")
	}
	if toonTexture.TextureType != model.TEXTURE_TYPE_TOON {
		t.Fatalf("toon texture type mismatch: got=%d want=%d", toonTexture.TextureType, model.TEXTURE_TYPE_TOON)
	}
	if hasWarningID(modelData, warningid.VrmWarningToonTextureGenerationFailed) {
		t.Fatalf("unexpected warning id: %s", warningid.VrmWarningToonTextureGenerationFailed)
	}
}

func TestAppendPrimitiveMaterialFallsBackToSharedToonWhenShadeMissing(t *testing.T) {
	modelData := newVroidProfileTestModelData()
	doc := &gltfDocument{
		Materials: []gltfMaterial{
			{
				Name:      "Body_00_SKIN",
				AlphaMode: "OPAQUE",
				PbrMetallicRoughness: gltfPbrMetallicRoughness{
					BaseColorFactor: []float64{1, 1, 1, 1},
				},
			},
		},
	}
	materialIndex := 0
	primitive := gltfPrimitive{Material: &materialIndex}

	appendedIndex := appendPrimitiveMaterial(
		modelData,
		doc,
		primitive,
		"Body_00_SKIN",
		nil,
		3,
		newTargetMorphRegistry(),
	)
	materialData, getErr := modelData.Materials.Get(appendedIndex)
	if getErr != nil || materialData == nil {
		t.Fatalf("appendPrimitiveMaterial failed: err=%v", getErr)
	}
	if materialData.ToonSharingFlag != model.TOON_SHARING_SHARING {
		t.Fatalf("toon sharing fallback mismatch: got=%d want=%d", materialData.ToonSharingFlag, model.TOON_SHARING_SHARING)
	}
	if materialData.ToonTextureIndex != 1 {
		t.Fatalf("toon fallback index mismatch: got=%d want=1", materialData.ToonTextureIndex)
	}
	if !hasWarningID(modelData, warningid.VrmWarningToonTextureGenerationFailed) {
		t.Fatalf("warning id should be recorded: %s", warningid.VrmWarningToonTextureGenerationFailed)
	}
}

func TestAppendPrimitiveMaterialSpherePriorityUsesSphereAddAndWarnsEmissiveIgnored(t *testing.T) {
	modelData := newVroidProfileTestModelData()
	appendTexture := func(name string) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}
	baseTextureIndex := appendTexture("base.png")
	sphereAddTextureIndex := appendTexture("sphere_add.png")
	appendTexture("matcap.png")
	appendTexture("emissive.png")
	textureIndexesByImage := []int{baseTextureIndex, sphereAddTextureIndex, 2, 3}

	vrmExtensionRaw, err := json.Marshal(map[string]any{
		"materialProperties": []any{
			map[string]any{
				"name": "Body_00_SKIN",
				"textureProperties": map[string]any{
					"_SphereAdd": 1,
				},
				"vectorProperties": map[string]any{
					"_ShadeColor": []any{0.2, 0.4, 0.6},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal VRM extension: %v", err)
	}
	mtoonRaw, err := json.Marshal(map[string]any{
		"matcapTexture": map[string]any{
			"index": 2,
		},
		"matcapFactor": []any{1.0, 1.0, 1.0},
	})
	if err != nil {
		t.Fatalf("failed to marshal mtoon extension: %v", err)
	}
	source0, source1, source2, source3 := 0, 1, 2, 3
	doc := &gltfDocument{
		Materials: []gltfMaterial{
			{
				Name:      "Body_00_SKIN",
				AlphaMode: "OPAQUE",
				PbrMetallicRoughness: gltfPbrMetallicRoughness{
					BaseColorFactor:  []float64{1, 1, 1, 1},
					BaseColorTexture: &gltfTextureRef{Index: 0},
				},
				EmissiveFactor:  []float64{0.6, 0.6, 0.6},
				EmissiveTexture: &gltfTextureRef{Index: 3},
				Extensions: map[string]json.RawMessage{
					"VRMC_materials_mtoon": mtoonRaw,
				},
			},
		},
		Textures: []gltfTexture{
			{Source: &source0},
			{Source: &source1},
			{Source: &source2},
			{Source: &source3},
		},
		Extensions: map[string]json.RawMessage{
			"VRM": vrmExtensionRaw,
		},
	}
	materialIndex := 0
	primitive := gltfPrimitive{Material: &materialIndex}

	appendedIndex := appendPrimitiveMaterial(
		modelData,
		doc,
		primitive,
		"Body_00_SKIN",
		textureIndexesByImage,
		3,
		newTargetMorphRegistry(),
	)
	materialData, getErr := modelData.Materials.Get(appendedIndex)
	if getErr != nil || materialData == nil {
		t.Fatalf("appendPrimitiveMaterial failed: err=%v", getErr)
	}
	if materialData.SphereTextureIndex != sphereAddTextureIndex {
		t.Fatalf("sphere texture index mismatch: got=%d want=%d", materialData.SphereTextureIndex, sphereAddTextureIndex)
	}
	if materialData.SphereMode != model.SPHERE_MODE_ADDITION {
		t.Fatalf("sphere mode mismatch: got=%d want=%d", materialData.SphereMode, model.SPHERE_MODE_ADDITION)
	}
	if !hasWarningID(modelData, warningid.VrmWarningEmissiveIgnoredBySpherePriority) {
		t.Fatalf("warning id should be recorded: %s", warningid.VrmWarningEmissiveIgnoredBySpherePriority)
	}
}

func TestAppendPrimitiveMaterialSpherePriorityUsesHairSphereBeforeMatcap(t *testing.T) {
	modelData := newVroidProfileTestModelData()
	appendTexture := func(name string) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}
	baseTextureIndex := appendTexture("hair_base.png")
	appendTexture("matcap.png")
	appendTexture("emissive.png")
	textureIndexesByImage := []int{baseTextureIndex, 1, 2}

	vrmExtensionRaw, err := json.Marshal(map[string]any{
		"materialProperties": []any{
			map[string]any{
				"name": "N00_Hair_00_HAIR",
				"vectorProperties": map[string]any{
					"_ShadeColor": []any{0.1, 0.2, 0.3},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal VRM extension: %v", err)
	}
	mtoonRaw, err := json.Marshal(map[string]any{
		"matcapTexture": map[string]any{
			"index": 1,
		},
		"matcapFactor": []any{1.0, 1.0, 1.0},
	})
	if err != nil {
		t.Fatalf("failed to marshal mtoon extension: %v", err)
	}
	source0, source1, source2 := 0, 1, 2
	doc := &gltfDocument{
		Materials: []gltfMaterial{
			{
				Name:      "N00_Hair_00_HAIR",
				AlphaMode: "OPAQUE",
				PbrMetallicRoughness: gltfPbrMetallicRoughness{
					BaseColorFactor:  []float64{1, 1, 1, 1},
					BaseColorTexture: &gltfTextureRef{Index: 0},
				},
				EmissiveFactor:  []float64{0.5, 0.5, 0.5},
				EmissiveTexture: &gltfTextureRef{Index: 2},
				Extensions: map[string]json.RawMessage{
					"VRMC_materials_mtoon": mtoonRaw,
				},
			},
		},
		Textures: []gltfTexture{
			{Source: &source0},
			{Source: &source1},
			{Source: &source2},
		},
		Extensions: map[string]json.RawMessage{
			"VRM": vrmExtensionRaw,
		},
	}
	materialIndex := 0
	primitive := gltfPrimitive{Material: &materialIndex}

	appendedIndex := appendPrimitiveMaterial(
		modelData,
		doc,
		primitive,
		"N00_Hair_00_HAIR",
		textureIndexesByImage,
		3,
		newTargetMorphRegistry(),
	)
	materialData, getErr := modelData.Materials.Get(appendedIndex)
	if getErr != nil || materialData == nil {
		t.Fatalf("appendPrimitiveMaterial failed: err=%v", getErr)
	}
	if materialData.SphereMode != model.SPHERE_MODE_ADDITION {
		t.Fatalf("sphere mode mismatch: got=%d want=%d", materialData.SphereMode, model.SPHERE_MODE_ADDITION)
	}
	if materialData.SphereTextureIndex < 0 {
		t.Fatalf("sphere texture index should be generated: got=%d", materialData.SphereTextureIndex)
	}
	sphereTexture, getTextureErr := modelData.Textures.Get(materialData.SphereTextureIndex)
	if getTextureErr != nil || sphereTexture == nil {
		t.Fatalf("sphere texture not found: index=%d err=%v", materialData.SphereTextureIndex, getTextureErr)
	}
	if filepath.ToSlash(sphereTexture.Name()) != "tex/sphere/hair_sphere_000.png" {
		t.Fatalf("hair sphere texture mismatch: got=%s want=%s", filepath.ToSlash(sphereTexture.Name()), "tex/sphere/hair_sphere_000.png")
	}
	if !hasWarningID(modelData, warningid.VrmWarningEmissiveIgnoredBySpherePriority) {
		t.Fatalf("warning id should be recorded: %s", warningid.VrmWarningEmissiveIgnoredBySpherePriority)
	}
}

func TestAppendPrimitiveMaterialSphereFallbackRecordsSourceMissingWarning(t *testing.T) {
	modelData := newVroidProfileTestModelData()
	doc := &gltfDocument{
		Materials: []gltfMaterial{
			{
				Name:      "Body_00_SKIN",
				AlphaMode: "OPAQUE",
				PbrMetallicRoughness: gltfPbrMetallicRoughness{
					BaseColorFactor: []float64{1, 1, 1, 1},
				},
			},
		},
	}
	materialIndex := 0
	primitive := gltfPrimitive{Material: &materialIndex}

	appendedIndex := appendPrimitiveMaterial(
		modelData,
		doc,
		primitive,
		"Body_00_SKIN",
		nil,
		3,
		newTargetMorphRegistry(),
	)
	materialData, getErr := modelData.Materials.Get(appendedIndex)
	if getErr != nil || materialData == nil {
		t.Fatalf("appendPrimitiveMaterial failed: err=%v", getErr)
	}
	if materialData.SphereTextureIndex != 0 {
		t.Fatalf("sphere fallback texture index mismatch: got=%d want=0", materialData.SphereTextureIndex)
	}
	if materialData.SphereMode != model.SPHERE_MODE_INVALID {
		t.Fatalf("sphere fallback mode mismatch: got=%d want=%d", materialData.SphereMode, model.SPHERE_MODE_INVALID)
	}
	if !hasWarningID(modelData, warningid.VrmWarningSphereTextureSourceMissing) {
		t.Fatalf("warning id should be recorded: %s", warningid.VrmWarningSphereTextureSourceMissing)
	}
}

func TestAppendPrimitiveMaterialSphereGenerationFailureRecordsWarning(t *testing.T) {
	modelData := newVroidProfileTestModelData()
	modelData.Textures = nil
	vrmExtensionRaw, err := json.Marshal(map[string]any{
		"materialProperties": []any{
			map[string]any{
				"name": "N00_Hair_00_HAIR",
				"vectorProperties": map[string]any{
					"_ShadeColor": []any{0.1, 0.2, 0.3},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal VRM extension: %v", err)
	}
	source0 := 0
	doc := &gltfDocument{
		Materials: []gltfMaterial{
			{
				Name:      "N00_Hair_00_HAIR",
				AlphaMode: "OPAQUE",
				PbrMetallicRoughness: gltfPbrMetallicRoughness{
					BaseColorFactor:  []float64{1, 1, 1, 1},
					BaseColorTexture: &gltfTextureRef{Index: 0},
				},
			},
		},
		Textures: []gltfTexture{
			{Source: &source0},
		},
		Extensions: map[string]json.RawMessage{
			"VRM": vrmExtensionRaw,
		},
	}
	materialIndex := 0
	primitive := gltfPrimitive{Material: &materialIndex}

	_ = appendPrimitiveMaterial(
		modelData,
		doc,
		primitive,
		"N00_Hair_00_HAIR",
		[]int{0},
		3,
		newTargetMorphRegistry(),
	)
	if !hasWarningID(modelData, warningid.VrmWarningSphereTextureGenerationFailed) {
		t.Fatalf("warning id should be recorded: %s", warningid.VrmWarningSphereTextureGenerationFailed)
	}
}

func TestCloneSpecialEyeMaterialKeepsEdgeSizeAndDisablesEdgeFlag(t *testing.T) {
	baseMaterial := model.NewMaterial()
	baseMaterial.SetName("Face_00_SKIN")
	baseMaterial.EnglishName = "Face_00_SKIN"
	baseMaterial.Diffuse = mmath.Vec4{X: 1.0, Y: 1.0, Z: 1.0, W: 1.0}
	baseMaterial.Specular = mmath.Vec4{X: 0.3, Y: 0.2, Z: 0.1, W: 0.7}
	baseMaterial.DrawFlag = model.DRAW_FLAG_DRAWING_EDGE | model.DRAW_FLAG_DOUBLE_SIDED_DRAWING
	baseMaterial.EdgeSize = 1.3

	clonedMaterial := cloneSpecialEyeMaterial(baseMaterial, "Face_00_SKIN_cheek_dye", 4)
	if clonedMaterial == nil {
		t.Fatal("cloneSpecialEyeMaterial returned nil")
	}
	if math.Abs(clonedMaterial.EdgeSize-baseMaterial.EdgeSize) > 1e-9 {
		t.Fatalf("edge size mismatch: got=%f want=%f", clonedMaterial.EdgeSize, baseMaterial.EdgeSize)
	}
	if !clonedMaterial.Specular.NearEquals(baseMaterial.Specular, 1e-9) {
		t.Fatalf("specular mismatch: got=%v want=%v", clonedMaterial.Specular, baseMaterial.Specular)
	}
	if (clonedMaterial.DrawFlag & model.DRAW_FLAG_DRAWING_EDGE) != 0 {
		t.Fatalf("edge flag should be disabled: flag=%d", clonedMaterial.DrawFlag)
	}
	if math.Abs(clonedMaterial.Diffuse.W) > 1e-9 {
		t.Fatalf("cloned material should start hidden: alpha=%f", clonedMaterial.Diffuse.W)
	}
}

func TestAppendSpecialEyeMaterialMorphsFromFallbackRulesUsesBaseTextureWhenOverlayTextureMissing(t *testing.T) {
	modelData := model.NewPmxModel()

	appendTexture := func(name string) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}

	irisTextureIndex := appendTexture("eye_iris_base.png")
	whiteTextureIndex := appendTexture("eye_white_base.png")
	eyeLineTextureIndex := appendTexture("eye_line_base.png")
	eyeLashTextureIndex := appendTexture("eye_lash_base.png")

	appendBaseMaterial := func(name string, textureIndex int) int {
		materialData := model.NewMaterial()
		materialData.SetName(name)
		materialData.EnglishName = name
		materialData.Diffuse = mmath.Vec4{X: 1.0, Y: 1.0, Z: 1.0, W: 1.0}
		materialData.DrawFlag = model.DRAW_FLAG_DRAWING_EDGE
		materialData.TextureIndex = textureIndex
		return modelData.Materials.AppendRaw(materialData)
	}

	irisMaterialIndex := appendBaseMaterial("Face_EyeIris_00", irisTextureIndex)
	whiteMaterialIndex := appendBaseMaterial("Face_EyeWhite_00", whiteTextureIndex)
	appendBaseMaterial("Face_EyeLine_00", eyeLineTextureIndex)
	appendBaseMaterial("Face_EyeLash_00", eyeLashTextureIndex)

	appendFaceByMaterial := func(materialIndex int, xOffset float64) {
		vertexStart := modelData.Vertices.Len()
		modelData.Vertices.AppendRaw(&model.Vertex{
			Position:        mmath.Vec3{Vec: r3.Vec{X: xOffset, Y: 0.0, Z: 0.0}},
			MaterialIndexes: []int{materialIndex},
		})
		modelData.Vertices.AppendRaw(&model.Vertex{
			Position:        mmath.Vec3{Vec: r3.Vec{X: xOffset + 0.1, Y: 0.0, Z: 0.0}},
			MaterialIndexes: []int{materialIndex},
		})
		modelData.Vertices.AppendRaw(&model.Vertex{
			Position:        mmath.Vec3{Vec: r3.Vec{X: xOffset, Y: 0.1, Z: 0.0}},
			MaterialIndexes: []int{materialIndex},
		})
		modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{vertexStart, vertexStart + 1, vertexStart + 2}})
		materialData, err := modelData.Materials.Get(materialIndex)
		if err != nil || materialData == nil {
			t.Fatalf("material not found: index=%d err=%v", materialIndex, err)
		}
		materialData.VerticesCount += 3
	}

	appendFaceByMaterial(irisMaterialIndex, 0.0)
	appendFaceByMaterial(whiteMaterialIndex, 1.0)

	appendSpecialEyeMaterialMorphsFromFallbackRules(modelData, nil, newTargetMorphRegistry())

	if len(findMaterialIndexesBySuffixToken(modelData, "eye_hau")) != 0 {
		t.Fatal("eye_hau material should not be generated without overlay texture")
	}
	if len(findMaterialIndexesBySuffixToken(modelData, "eye_star")) != 0 {
		t.Fatal("eye_star material should not be generated without overlay texture")
	}
	if hauMorph, err := modelData.Morphs.GetByName("はぅ材質"); err != nil || hauMorph == nil {
		t.Fatalf("はぅ材質 morph should exist as hide-only fallback: err=%v", err)
	}
	if _, err := modelData.Morphs.GetByName("星目材質"); err == nil {
		t.Fatal("星目材質 should not exist without overlay texture")
	}
}

func TestAppendSpecialEyeFacesForMaterialDuplicatesVertices(t *testing.T) {
	modelData := model.NewPmxModel()

	baseMaterial := model.NewMaterial()
	baseMaterial.SetName("EyeWhite_00_EYE")
	modelData.Materials.AppendRaw(baseMaterial)

	overlayMaterial := model.NewMaterial()
	overlayMaterial.SetName("EyeWhite_00_EYE_eye_hau")
	overlayMaterialIndex := modelData.Materials.AppendRaw(overlayMaterial)

	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 0.0, Z: 0.0}},
		MaterialIndexes: []int{0},
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        mmath.Vec3{Vec: r3.Vec{X: 0.1, Y: 0.0, Z: 0.0}},
		MaterialIndexes: []int{0},
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 0.1, Z: 0.0}},
		MaterialIndexes: []int{0},
	})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{0, 1, 2}})

	beforeVertexCount := modelData.Vertices.Len()
	beforeFaceCount := modelData.Faces.Len()
	copiedFaceCount := appendSpecialEyeFacesForMaterial(modelData, 0, 1, overlayMaterialIndex)
	if copiedFaceCount != 1 {
		t.Fatalf("copied face count mismatch: got=%d want=1", copiedFaceCount)
	}
	if modelData.Vertices.Len() <= beforeVertexCount {
		t.Fatalf("overlay vertices should be duplicated: before=%d after=%d", beforeVertexCount, modelData.Vertices.Len())
	}
	if modelData.Faces.Len() != beforeFaceCount+1 {
		t.Fatalf("overlay face should be added: before=%d after=%d", beforeFaceCount, modelData.Faces.Len())
	}

	for sourceVertexIndex := 0; sourceVertexIndex < beforeVertexCount; sourceVertexIndex++ {
		sourceVertex, err := modelData.Vertices.Get(sourceVertexIndex)
		if err != nil || sourceVertex == nil {
			t.Fatalf("source vertex not found: index=%d err=%v", sourceVertexIndex, err)
		}
		if len(sourceVertex.MaterialIndexes) != 1 || sourceVertex.MaterialIndexes[0] != 0 {
			t.Fatalf("source vertex material indexes should stay unchanged: index=%d materials=%v", sourceVertexIndex, sourceVertex.MaterialIndexes)
		}
	}
}

func TestBuildCreateTargetVertexSetForEyeHideExcludesIrisOverlap(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Vertices.AppendRaw(&model.Vertex{Position: mmath.Vec3{Vec: r3.Vec{X: 0.1}}})
	modelData.Vertices.AppendRaw(&model.Vertex{Position: mmath.Vec3{Vec: r3.Vec{X: -0.1}}})

	rule := createMorphRule{
		Name:    "目隠し頂点",
		Type:    createMorphRuleTypeEyeHideVertex,
		Creates: []string{"EyeWhite"},
	}
	morphSemanticVertexSets := map[string]map[int]struct{}{
		createSemanticEyeWhite: {0: {}, 1: {}},
		createSemanticIris:     {1: {}},
	}
	materialSemanticVertexSets := map[string]map[int]struct{}{}

	targetVertices := buildCreateTargetVertexSet(rule, modelData, morphSemanticVertexSets, materialSemanticVertexSets)
	if _, exists := targetVertices[0]; !exists {
		t.Fatal("white-only vertex should remain in eye hide targets")
	}
	if _, exists := targetVertices[1]; exists {
		t.Fatal("iris-overlap vertex should be excluded from eye hide targets")
	}
}

func TestBuildCreateTargetVertexSetForEyeHideFallsBackWhenIrisFilterWouldEmpty(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Vertices.AppendRaw(&model.Vertex{Position: mmath.Vec3{Vec: r3.Vec{X: 0.1}}})
	modelData.Vertices.AppendRaw(&model.Vertex{Position: mmath.Vec3{Vec: r3.Vec{X: -0.1}}})

	rule := createMorphRule{
		Name:    "目隠し頂点",
		Type:    createMorphRuleTypeEyeHideVertex,
		Creates: []string{"EyeWhite"},
	}
	morphSemanticVertexSets := map[string]map[int]struct{}{
		createSemanticEyeWhite: {0: {}, 1: {}},
		createSemanticIris:     {0: {}, 1: {}},
	}
	materialSemanticVertexSets := map[string]map[int]struct{}{}

	targetVertices := buildCreateTargetVertexSet(rule, modelData, morphSemanticVertexSets, materialSemanticVertexSets)
	if len(targetVertices) == 0 {
		t.Fatal("eye hide targets should fallback to non-empty when iris filtering empties all")
	}
}

func TestBuildCreateEyeHideOffsetsProjectsToFacePlusOffsetAfterBlink(t *testing.T) {
	modelData := model.NewPmxModel()
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position: mmath.Vec3{Vec: r3.Vec{X: 0.2, Y: 0.0, Z: 0.0}},
	})

	targetVertices := map[int]struct{}{
		0: {},
	}
	hideVertices := map[int]struct{}{}
	closeOffsets := map[int]mmath.Vec3{
		0: {Vec: r3.Vec{X: 0.0, Y: -0.3, Z: 0.2}},
	}

	openFaceTriangles := []createFaceTriangle{
		newCreateFaceTriangle(
			mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 0.0, Z: 1.0}},
			mmath.Vec3{Vec: r3.Vec{X: 1.0, Y: 0.0, Z: 1.0}},
			mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 1.0, Z: 1.0}},
		),
	}
	leftClosedFaceTriangles := []createFaceTriangle{
		newCreateFaceTriangle(
			mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 0.0, Z: 1.0}},
			mmath.Vec3{Vec: r3.Vec{X: 1.0, Y: 0.0, Z: 1.0}},
			mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 1.0, Z: 1.0}},
		),
	}

	offsets := buildCreateEyeHideOffsets(
		modelData,
		targetVertices,
		hideVertices,
		closeOffsets,
		openFaceTriangles,
		leftClosedFaceTriangles,
		nil,
	)
	offsetsByVertex := collectVertexOffsetByIndex(offsets)
	offsetData, exists := offsetsByVertex[0]
	if !exists || offsetData == nil {
		t.Fatalf("vertex offset not found: exists=%t", exists)
	}

	// まばたき(Z=+0.2)を適用済みでも、最終到達位置は Face(Z=1.0)-0.1 になることを確認する。
	vertexData, err := modelData.Vertices.Get(0)
	if err != nil || vertexData == nil {
		t.Fatalf("vertex not found: err=%v", err)
	}
	finalZ := vertexData.Position.Z + offsetData.Position.Z
	if math.Abs(finalZ-0.9) > 1e-6 {
		t.Fatalf("final Z mismatch: got=%f want=%f", finalZ, 0.9)
	}
}

func findMaterialIndexesBySuffixToken(modelData *model.PmxModel, token string) []int {
	if modelData == nil || modelData.Materials == nil {
		return nil
	}
	normalizedToken := normalizeSpecialEyeToken(token)
	materialIndexes := []int{}
	for materialIndex := 0; materialIndex < modelData.Materials.Len(); materialIndex++ {
		materialData, err := modelData.Materials.Get(materialIndex)
		if err != nil || materialData == nil {
			continue
		}
		if strings.HasSuffix(normalizeSpecialEyeToken(materialData.Name()), normalizedToken) {
			materialIndexes = append(materialIndexes, materialIndex)
		}
	}
	return materialIndexes
}

func collectMaterialOffsetByIndex(morphData *model.Morph) map[int]*model.MaterialMorphOffset {
	result := map[int]*model.MaterialMorphOffset{}
	if morphData == nil {
		return result
	}
	for _, rawOffset := range morphData.Offsets {
		offsetData, ok := rawOffset.(*model.MaterialMorphOffset)
		if !ok || offsetData == nil {
			continue
		}
		result[offsetData.MaterialIndex] = offsetData
	}
	return result
}

func hasPositiveMaterialAlphaOffset(offsets map[int]*model.MaterialMorphOffset) bool {
	for _, offsetData := range offsets {
		if offsetData != nil && offsetData.Diffuse.W > 0 {
			return true
		}
	}
	return false
}

// collectVertexOffsetByIndex は頂点モーフオフセットを頂点indexで引ける形に変換する。
func collectVertexOffsetByIndex(offsets []model.IMorphOffset) map[int]*model.VertexMorphOffset {
	result := map[int]*model.VertexMorphOffset{}
	for _, rawOffset := range offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil {
			continue
		}
		result[offsetData.VertexIndex] = offsetData
	}
	return result
}

func newVroidProfileTestModelData() *model.PmxModel {
	modelData := model.NewPmxModel()
	modelData.VrmData = &modelvrm.VrmData{
		Profile:       modelvrm.VRM_PROFILE_VROID,
		RawExtensions: map[string]json.RawMessage{},
	}
	return modelData
}

func hasWarningID(modelData *model.PmxModel, targetWarningID string) bool {
	if modelData == nil || modelData.VrmData == nil {
		return false
	}
	rawWarnings, exists := modelData.VrmData.RawExtensions[warningid.VrmWarningRawExtensionKey]
	if !exists || len(rawWarnings) == 0 {
		return false
	}
	warningIDs := []string{}
	if err := json.Unmarshal(rawWarnings, &warningIDs); err != nil {
		return false
	}
	for _, warningID := range warningIDs {
		if strings.TrimSpace(warningID) == targetWarningID {
			return true
		}
	}
	return false
}
