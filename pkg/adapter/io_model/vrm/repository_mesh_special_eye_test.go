// 指示: miu200521358
package vrm

import (
	"strings"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
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
