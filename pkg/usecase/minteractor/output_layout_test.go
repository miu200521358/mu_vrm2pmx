// 指示: miu200521358
package minteractor

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/domain/model"
	modelvrm "github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
	warningid "github.com/miu200521358/mu_vrm2pmx/pkg/domain/model"
	"golang.org/x/image/bmp"
)

func TestExportGeneratedToonTexturesSkipsWhenShadeColorMapUnavailable(t *testing.T) {
	texDir := t.TempDir()
	modelData := model.NewPmxModel()

	appendTexture := func(name string, textureType model.TextureType) {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.TextureType = textureType
		texture.SetValid(true)
		modelData.Textures.AppendRaw(texture)
	}
	appendTexture(filepath.Join(defaultTextureDirName, "toon01.bmp"), model.TEXTURE_TYPE_TOON)
	appendTexture(filepath.Join(defaultTextureDirName, "toon", "toon_000_ff8040.bmp"), model.TEXTURE_TYPE_TOON)
	appendTexture(filepath.Join(defaultTextureDirName, "sphere00.png"), model.TEXTURE_TYPE_SPHERE)

	exportGeneratedToonTextures(texDir, modelData)

	outputPath := filepath.Join(texDir, "toon01.bmp")
	if _, err := os.Stat(outputPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("toon texture should be skipped when shade map is unavailable: %v", err)
	}

	legacySubdirPath := filepath.Join(texDir, "toon_000_ff8040.bmp")
	if _, err := os.Stat(legacySubdirPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy toon subdir output should not be generated: %v", err)
	}
	spherePath := filepath.Join(texDir, "sphere00.png")
	if _, err := os.Stat(spherePath); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("non-toon texture should not be generated: %v", err)
	}
}

func TestExportGeneratedToonTexturesUsesShadeColorMapPerTexture(t *testing.T) {
	texDir := t.TempDir()
	modelData := model.NewPmxModel()
	modelData.VrmData = &modelvrm.VrmData{
		RawExtensions: map[string]json.RawMessage{},
	}

	appendTexture := func(name string, textureType model.TextureType) {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.TextureType = textureType
		texture.SetValid(true)
		modelData.Textures.AppendRaw(texture)
	}
	appendTexture("tex/toon01.bmp", model.TEXTURE_TYPE_TOON)
	appendTexture("tex/toon02.bmp", model.TEXTURE_TYPE_TOON)
	appendTexture("tex/toon03.bmp", model.TEXTURE_TYPE_TOON)

	shadeColorMapRaw, err := json.Marshal(map[string][3]uint8{
		"toon01.bmp": {0x11, 0x22, 0x33},
		"toon02.bmp": {0xaa, 0xbb, 0xcc},
	})
	if err != nil {
		t.Fatalf("failed to marshal shade color map: %v", err)
	}
	modelData.VrmData.RawExtensions[warningid.VrmLegacyGeneratedToonShadeMapRawExtensionKey] = shadeColorMapRaw

	exportGeneratedToonTextures(texDir, modelData)

	assertLowerColor := func(fileName string, want color.RGBA) {
		t.Helper()
		data, readErr := os.ReadFile(filepath.Join(texDir, fileName))
		if readErr != nil {
			t.Fatalf("generated toon texture not found: file=%s err=%v", fileName, readErr)
		}
		got, decodeErr := readToonLowerPixelColor(data)
		if decodeErr != nil {
			t.Fatalf("failed to decode generated toon texture: file=%s err=%v", fileName, decodeErr)
		}
		if got != want {
			t.Fatalf("lower shade color mismatch: file=%s got=%v want=%v", fileName, got, want)
		}
	}
	assertLowerColor("toon01.bmp", color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})
	assertLowerColor("toon02.bmp", color.RGBA{R: 0xaa, G: 0xbb, B: 0xcc, A: 0xff})
	if _, err := os.Stat(filepath.Join(texDir, "toon03.bmp")); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("toon texture without shade color should be skipped: %v", err)
	}
}

func TestResolveGeneratedToonFileName(t *testing.T) {
	testCases := []struct {
		name      string
		texture   string
		wantName  string
		shouldHit bool
	}{
		{name: "tex root toon", texture: "tex/toon01.bmp", wantName: "toon01.bmp", shouldHit: true},
		{name: "case insensitive", texture: "TeX/ToOn12.BmP", wantName: "toon12.bmp", shouldHit: true},
		{name: "no prefix", texture: "toon99.bmp", wantName: "toon99.bmp", shouldHit: true},
		{name: "legacy subdir", texture: "tex/toon/toon_000_ff8040.bmp", wantName: "", shouldHit: false},
		{name: "different extension", texture: "tex/toon00.png", wantName: "", shouldHit: false},
		{name: "different texture", texture: "tex/body.png", wantName: "", shouldHit: false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotName, ok := resolveGeneratedToonFileName(tc.texture)
			if ok != tc.shouldHit {
				t.Fatalf("match result mismatch: got=%t want=%t texture=%s", ok, tc.shouldHit, tc.texture)
			}
			if gotName != tc.wantName {
				t.Fatalf("resolved file name mismatch: got=%s want=%s texture=%s", gotName, tc.wantName, tc.texture)
			}
		})
	}
}

func TestExportGeneratedSphereTexturesWritesGeneratedFiles(t *testing.T) {
	texDir := t.TempDir()
	modelData := model.NewPmxModel()
	modelData.VrmData = &modelvrm.VrmData{
		RawExtensions: map[string]json.RawMessage{},
	}

	appendTexture := func(name string, textureType model.TextureType) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.TextureType = textureType
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}
	hairSourceTextureIndex := appendTexture("tex/hair_base.png", model.TEXTURE_TYPE_TEXTURE)
	if err := writePatternPNG(filepath.Join(texDir, "hair_base.png"), 64, 48); err != nil {
		t.Fatalf("failed to write source hair texture: %v", err)
	}
	appendGeneratedSphereMetadata(
		t,
		modelData,
		map[string]generatedSphereMetadata{
			"tex/hair_sphere_00.png": {
				SourceTextureIndex: hairSourceTextureIndex,
				MaterialIndex:      0,
				SphereKind:         "hair",
			},
		},
	)
	appendTexture("tex/hair_sphere_00.png", model.TEXTURE_TYPE_SPHERE)
	appendTexture("tex/sphere/matcap_sphere_001.png", model.TEXTURE_TYPE_SPHERE)
	appendTexture("tex/sphere/emissive_sphere_002.png", model.TEXTURE_TYPE_SPHERE)
	appendTexture("tex/sphere00.png", model.TEXTURE_TYPE_SPHERE)
	appendTexture("tex/hair_sphere_03.png", model.TEXTURE_TYPE_TOON)

	sphereMetadataMap := resolveGeneratedSphereMetadataMap(modelData)
	sphereMetadata, hasSphereMetadata := resolveGeneratedSphereMetadata(sphereMetadataMap, "tex/hair_sphere_00.png")
	if !hasSphereMetadata {
		t.Fatal("sphere metadata should be resolved before export")
	}
	if sphereMetadata.SourceTextureIndex != hairSourceTextureIndex {
		t.Fatalf(
			"sphere metadata source texture index mismatch: got=%d want=%d",
			sphereMetadata.SourceTextureIndex,
			hairSourceTextureIndex,
		)
	}
	if _, err := buildGeneratedHairSpherePng(texDir, modelData, sphereMetadata); err != nil {
		t.Fatalf("hair sphere should be buildable before export: %v", err)
	}

	exportGeneratedSphereTextures(texDir, modelData)

	hairData, hairReadErr := os.ReadFile(filepath.Join(texDir, "hair_sphere_00.png"))
	if hairReadErr != nil {
		t.Fatalf("generated hair sphere texture not found: %v", hairReadErr)
	}
	hairWidth, hairHeight, hairDimensionErr := readSphereDimensions(hairData)
	if hairDimensionErr != nil {
		t.Fatalf("failed to decode generated hair sphere texture: %v", hairDimensionErr)
	}
	if hairWidth != 64 || hairHeight != 48 {
		t.Fatalf("generated hair sphere dimensions mismatch: got=%dx%d want=64x48", hairWidth, hairHeight)
	}
	sourceHairData, sourceReadErr := os.ReadFile(filepath.Join(texDir, "hair_base.png"))
	if sourceReadErr != nil {
		t.Fatalf("failed to read source hair texture: %v", sourceReadErr)
	}
	isSameAsSource, compareErr := arePngImagesEqual(hairData, sourceHairData)
	if compareErr != nil {
		t.Fatalf("failed to compare generated hair sphere against source: %v", compareErr)
	}
	if isSameAsSource {
		t.Fatal("generated hair sphere should not be an exact copy of source hair texture")
	}
	isUniform, uniformErr := isSphereSingleColor(hairData)
	if uniformErr != nil {
		t.Fatalf("failed to inspect generated hair sphere texture: %v", uniformErr)
	}
	if isUniform {
		t.Fatal("generated hair sphere should not fallback to single-color dummy")
	}

	assertGenerated := func(relPath string, want color.RGBA) {
		t.Helper()
		outputPath := filepath.Join(texDir, filepath.FromSlash(relPath))
		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("generated sphere texture not found: path=%s err=%v", relPath, err)
		}
		got, decodeErr := readSpherePixelColor(data)
		if decodeErr != nil {
			t.Fatalf("failed to decode generated sphere texture: path=%s err=%v", relPath, decodeErr)
		}
		if got != want {
			t.Fatalf("generated sphere color mismatch: path=%s got=%v want=%v", relPath, got, want)
		}
	}
	assertGenerated("sphere/matcap_sphere_001.png", color.RGBA{R: 0xd8, G: 0xd8, B: 0xd8, A: 0xff})
	assertGenerated("sphere/emissive_sphere_002.png", color.RGBA{R: 0xf0, G: 0xf0, B: 0xf0, A: 0xff})

	if _, err := os.Stat(filepath.Join(texDir, "sphere00.png")); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("non-generated sphere texture should be skipped: %v", err)
	}
	if _, err := os.Stat(filepath.Join(texDir, "hair_sphere_03.png")); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("non-sphere texture type should be skipped: %v", err)
	}
}

func TestExportGeneratedSphereTexturesKeepsHairSphereMaterialAssignmentConsistent(t *testing.T) {
	texDir := t.TempDir()
	modelData := model.NewPmxModel()
	modelData.VrmData = &modelvrm.VrmData{
		RawExtensions: map[string]json.RawMessage{},
	}

	appendTexture := func(name string, textureType model.TextureType) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.TextureType = textureType
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}
	hairSourceTextureIndex := appendTexture("tex/hair_base.png", model.TEXTURE_TYPE_TEXTURE)
	if err := writePatternPNG(filepath.Join(texDir, "hair_base.png"), 96, 40); err != nil {
		t.Fatalf("failed to write source hair texture: %v", err)
	}
	hairSphereTextureIndex := appendTexture("tex/hair_sphere_00.png", model.TEXTURE_TYPE_SPHERE)
	appendTexture("tex/sphere/matcap_sphere_001.png", model.TEXTURE_TYPE_SPHERE)
	appendGeneratedSphereMetadata(
		t,
		modelData,
		map[string]generatedSphereMetadata{
			"tex/hair_sphere_00.png": {
				SourceTextureIndex: hairSourceTextureIndex,
				MaterialIndex:      0,
				SphereKind:         "hair",
			},
		},
	)
	sphereMetadataMap := resolveGeneratedSphereMetadataMap(modelData)
	sphereMetadata, hasSphereMetadata := resolveGeneratedSphereMetadata(sphereMetadataMap, "tex/hair_sphere_00.png")
	if !hasSphereMetadata {
		t.Fatal("sphere metadata should be resolved before export")
	}
	if sphereMetadata.SourceTextureIndex != hairSourceTextureIndex {
		t.Fatalf(
			"sphere metadata source texture index mismatch: got=%d want=%d",
			sphereMetadata.SourceTextureIndex,
			hairSourceTextureIndex,
		)
	}
	if _, err := buildGeneratedHairSpherePng(texDir, modelData, sphereMetadata); err != nil {
		t.Fatalf("hair sphere should be buildable before export: %v", err)
	}

	hairMaterial := model.NewMaterial()
	hairMaterial.SetName("N00_000_Hair_00_HAIR_01")
	hairMaterial.EnglishName = "N00_000_Hair_00_HAIR_01"
	hairMaterial.SphereMode = model.SPHERE_MODE_ADDITION
	hairMaterial.SphereTextureIndex = hairSphereTextureIndex
	modelData.Materials.AppendRaw(hairMaterial)

	exportGeneratedSphereTextures(texDir, modelData)

	generatedHairSphereData, readErr := os.ReadFile(filepath.Join(texDir, "hair_sphere_00.png"))
	if readErr != nil {
		t.Fatalf("generated hair sphere texture not found: %v", readErr)
	}
	generatedWidth, generatedHeight, dimensionErr := readSphereDimensions(generatedHairSphereData)
	if dimensionErr != nil {
		t.Fatalf("failed to decode generated hair sphere texture: %v", dimensionErr)
	}
	if generatedWidth != 96 || generatedHeight != 40 {
		t.Fatalf("generated hair sphere dimensions mismatch: got=%dx%d want=96x40", generatedWidth, generatedHeight)
	}
	resolvedMaterial, getMaterialErr := modelData.Materials.Get(0)
	if getMaterialErr != nil || resolvedMaterial == nil {
		t.Fatalf("material not found: err=%v", getMaterialErr)
	}
	if resolvedMaterial.SphereTextureIndex != hairSphereTextureIndex {
		t.Fatalf(
			"sphere texture index mismatch: got=%d want=%d",
			resolvedMaterial.SphereTextureIndex,
			hairSphereTextureIndex,
		)
	}
	resolvedTexture, getTextureErr := modelData.Textures.Get(resolvedMaterial.SphereTextureIndex)
	if getTextureErr != nil || resolvedTexture == nil {
		t.Fatalf("sphere texture not found: index=%d err=%v", resolvedMaterial.SphereTextureIndex, getTextureErr)
	}
	if filepath.ToSlash(resolvedTexture.Name()) != "tex/hair_sphere_00.png" {
		t.Fatalf(
			"sphere texture path mismatch: got=%s want=%s",
			filepath.ToSlash(resolvedTexture.Name()),
			"tex/hair_sphere_00.png",
		)
	}
	if hasWarningID(modelData, warningid.VrmWarningSphereTextureSourceMissing) {
		t.Fatalf("unexpected warning id: %s", warningid.VrmWarningSphereTextureSourceMissing)
	}
	if hasWarningID(modelData, warningid.VrmWarningSphereTextureGenerationFailed) {
		t.Fatalf("unexpected warning id: %s", warningid.VrmWarningSphereTextureGenerationFailed)
	}
}

func TestExportGeneratedSphereTexturesGeneratesHairBlendTexture(t *testing.T) {
	texDir := t.TempDir()
	modelData := model.NewPmxModel()
	modelData.VrmData = &modelvrm.VrmData{
		RawExtensions: map[string]json.RawMessage{},
	}

	appendTexture := func(name string, textureType model.TextureType) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.TextureType = textureType
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}
	hairSourceTextureIndex := appendTexture("tex/_00.png", model.TEXTURE_TYPE_TEXTURE)
	appendTexture("tex/hair_sphere_00.png", model.TEXTURE_TYPE_SPHERE)
	if err := writeSolidPNG(filepath.Join(texDir, "_00.png"), 16, 16, color.RGBA{R: 100, G: 150, B: 200, A: 0xff}); err != nil {
		t.Fatalf("failed to write source hair texture: %v", err)
	}
	if err := writeSolidPNG(filepath.Join(texDir, "_01.png"), 16, 16, color.RGBA{R: 80, G: 40, B: 20, A: 0xff}); err != nil {
		t.Fatalf("failed to write highlight texture: %v", err)
	}
	appendGeneratedSphereMetadata(
		t,
		modelData,
		map[string]generatedSphereMetadata{
			"tex/hair_sphere_00.png": {
				SourceTextureIndex: hairSourceTextureIndex,
				MaterialIndex:      0,
				SphereKind:         "hair",
				EmissiveFactor:     [3]float64{0.5, 0.5, 0.5},
				DiffuseFactor:      [4]float64{1.0, 1.0, 1.0, 1.0},
				HighlightTexture:   "_01.png",
				BlendTexture:       "_00_blend.png",
			},
		},
	)

	exportGeneratedSphereTextures(texDir, modelData)

	blendData, readErr := os.ReadFile(filepath.Join(texDir, "_00_blend.png"))
	if readErr != nil {
		t.Fatalf("generated blend texture not found: %v", readErr)
	}
	blendColor, decodeErr := readSpherePixelColor(blendData)
	if decodeErr != nil {
		t.Fatalf("failed to decode generated blend texture: %v", decodeErr)
	}
	wantBlendColor := color.RGBA{R: 124, G: 158, B: 202, A: 0xff}
	if blendColor != wantBlendColor {
		t.Fatalf("generated blend texture color mismatch: got=%v want=%v", blendColor, wantBlendColor)
	}
	if hasWarningID(modelData, warningid.VrmWarningSphereTextureSourceMissing) {
		t.Fatalf("unexpected warning id: %s", warningid.VrmWarningSphereTextureSourceMissing)
	}
	if hasWarningID(modelData, warningid.VrmWarningSphereTextureGenerationFailed) {
		t.Fatalf("unexpected warning id: %s", warningid.VrmWarningSphereTextureGenerationFailed)
	}
}

func TestExportGeneratedSphereTexturesSkipsHairBlendWhenHighlightMissing(t *testing.T) {
	texDir := t.TempDir()
	modelData := model.NewPmxModel()
	modelData.VrmData = &modelvrm.VrmData{
		RawExtensions: map[string]json.RawMessage{},
	}

	appendTexture := func(name string, textureType model.TextureType) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.TextureType = textureType
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}
	hairSourceTextureIndex := appendTexture("tex/_00.png", model.TEXTURE_TYPE_TEXTURE)
	hairSphereTextureIndex := appendTexture("tex/hair_sphere_00.png", model.TEXTURE_TYPE_SPHERE)
	if err := writePatternPNG(filepath.Join(texDir, "_00.png"), 24, 24); err != nil {
		t.Fatalf("failed to write source hair texture: %v", err)
	}
	appendGeneratedSphereMetadata(
		t,
		modelData,
		map[string]generatedSphereMetadata{
			"tex/hair_sphere_00.png": {
				SourceTextureIndex: hairSourceTextureIndex,
				MaterialIndex:      0,
				SphereKind:         "hair",
				EmissiveFactor:     [3]float64{0.5, 0.5, 0.5},
				DiffuseFactor:      [4]float64{1.0, 1.0, 1.0, 1.0},
				HighlightTexture:   "_01.png",
				BlendTexture:       "_00_blend.png",
			},
		},
	)

	hairMaterial := model.NewMaterial()
	hairMaterial.SetName("N00_000_Hair_00_HAIR_01")
	hairMaterial.EnglishName = "N00_000_Hair_00_HAIR_01"
	hairMaterial.SphereMode = model.SPHERE_MODE_ADDITION
	hairMaterial.SphereTextureIndex = hairSphereTextureIndex
	modelData.Materials.AppendRaw(hairMaterial)

	exportGeneratedSphereTextures(texDir, modelData)

	if _, err := os.Stat(filepath.Join(texDir, "hair_sphere_00.png")); err != nil {
		t.Fatalf("hair sphere texture should still be generated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(texDir, "_00_blend.png")); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("blend texture should be skipped when highlight source is missing: %v", err)
	}
	if !hasWarningID(modelData, warningid.VrmWarningSphereTextureSourceMissing) {
		t.Fatalf("warning id should be recorded: %s", warningid.VrmWarningSphereTextureSourceMissing)
	}
	resolvedMaterial, getMaterialErr := modelData.Materials.Get(0)
	if getMaterialErr != nil || resolvedMaterial == nil {
		t.Fatalf("material not found: err=%v", getMaterialErr)
	}
	if resolvedMaterial.SphereTextureIndex != hairSphereTextureIndex {
		t.Fatalf(
			"sphere texture index should be preserved when only blend generation fails: got=%d want=%d",
			resolvedMaterial.SphereTextureIndex,
			hairSphereTextureIndex,
		)
	}
	if resolvedMaterial.SphereMode != model.SPHERE_MODE_ADDITION {
		t.Fatalf(
			"sphere mode should be preserved when only blend generation fails: got=%d want=%d",
			resolvedMaterial.SphereMode,
			model.SPHERE_MODE_ADDITION,
		)
	}
}

func TestExportGeneratedSphereTexturesDisablesHairSphereWhenMetadataMissing(t *testing.T) {
	texDir := t.TempDir()
	modelData := model.NewPmxModel()
	modelData.VrmData = &modelvrm.VrmData{
		RawExtensions: map[string]json.RawMessage{},
	}

	appendTexture := func(name string, textureType model.TextureType) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.TextureType = textureType
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}
	hairSphereTextureIndex := appendTexture("tex/hair_sphere_00.png", model.TEXTURE_TYPE_SPHERE)

	hairMaterial := model.NewMaterial()
	hairMaterial.SetName("N00_000_Hair_00_HAIR_01")
	hairMaterial.EnglishName = "N00_000_Hair_00_HAIR_01"
	hairMaterial.SphereMode = model.SPHERE_MODE_ADDITION
	hairMaterial.SphereTextureIndex = hairSphereTextureIndex
	modelData.Materials.AppendRaw(hairMaterial)

	exportGeneratedSphereTextures(texDir, modelData)

	if _, err := os.Stat(filepath.Join(texDir, "hair_sphere_00.png")); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("hair sphere texture should not be generated without metadata: %v", err)
	}
	resolvedMaterial, getMaterialErr := modelData.Materials.Get(0)
	if getMaterialErr != nil || resolvedMaterial == nil {
		t.Fatalf("material not found: err=%v", getMaterialErr)
	}
	if resolvedMaterial.SphereTextureIndex != 0 {
		t.Fatalf("sphere texture index should be cleared on metadata missing: got=%d want=0", resolvedMaterial.SphereTextureIndex)
	}
	if resolvedMaterial.SphereMode != model.SPHERE_MODE_INVALID {
		t.Fatalf("sphere mode should be invalid on metadata missing: got=%d want=%d", resolvedMaterial.SphereMode, model.SPHERE_MODE_INVALID)
	}
	if !hasWarningID(modelData, warningid.VrmWarningSphereTextureSourceMissing) {
		t.Fatalf("warning id should be recorded: %s", warningid.VrmWarningSphereTextureSourceMissing)
	}
}

func TestResolveGeneratedSphereRelativePath(t *testing.T) {
	testCases := []struct {
		name      string
		texture   string
		wantPath  string
		wantKind  generatedSphereKind
		shouldHit bool
	}{
		{
			name:      "hair sphere",
			texture:   "tex/hair_sphere_00.png",
			wantPath:  "hair_sphere_00.png",
			wantKind:  generatedSphereKindHair,
			shouldHit: true,
		},
		{
			name:      "matcap sphere",
			texture:   "tex/sphere/matcap_sphere_001.png",
			wantPath:  "sphere/matcap_sphere_001.png",
			wantKind:  generatedSphereKindMatcap,
			shouldHit: true,
		},
		{
			name:      "emissive sphere",
			texture:   "tex/sphere/emissive_sphere_001.png",
			wantPath:  "sphere/emissive_sphere_001.png",
			wantKind:  generatedSphereKindEmissive,
			shouldHit: true,
		},
		{
			name:      "case insensitive",
			texture:   "TeX/HaIr_SpHeRe_12.PnG",
			wantPath:  "hair_sphere_12.png",
			wantKind:  generatedSphereKindHair,
			shouldHit: true,
		},
		{
			name:      "invalid path traversal",
			texture:   "tex/../hair_sphere_00.png",
			wantPath:  "",
			wantKind:  generatedSphereKindUnknown,
			shouldHit: false,
		},
		{
			name:      "invalid name",
			texture:   "tex/sphere00.png",
			wantPath:  "",
			wantKind:  generatedSphereKindUnknown,
			shouldHit: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotPath, gotKind, ok := resolveGeneratedSphereRelativePath(tc.texture)
			if ok != tc.shouldHit {
				t.Fatalf("match result mismatch: got=%t want=%t texture=%s", ok, tc.shouldHit, tc.texture)
			}
			if gotPath != tc.wantPath {
				t.Fatalf("resolved path mismatch: got=%s want=%s texture=%s", gotPath, tc.wantPath, tc.texture)
			}
			if gotKind != tc.wantKind {
				t.Fatalf("resolved kind mismatch: got=%d want=%d texture=%s", gotKind, tc.wantKind, tc.texture)
			}
		})
	}
}

func readToonLowerPixelColor(bmpData []byte) (color.RGBA, error) {
	img, err := bmp.Decode(bytes.NewReader(bmpData))
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBAModel.Convert(img.At(0, 24)).(color.RGBA), nil
}

func readSpherePixelColor(pngData []byte) (color.RGBA, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA), nil
}

func readSphereDimensions(pngData []byte) (int, int, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return 0, 0, err
	}
	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy(), nil
}

func isSphereSingleColor(pngData []byte) (bool, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return false, err
	}
	bounds := img.Bounds()
	firstColor := color.RGBAModel.Convert(img.At(bounds.Min.X, bounds.Min.Y)).(color.RGBA)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if color.RGBAModel.Convert(img.At(x, y)).(color.RGBA) != firstColor {
				return false, nil
			}
		}
	}
	return true, nil
}

func arePngImagesEqual(leftPNG []byte, rightPNG []byte) (bool, error) {
	leftImage, leftErr := png.Decode(bytes.NewReader(leftPNG))
	if leftErr != nil {
		return false, leftErr
	}
	rightImage, rightErr := png.Decode(bytes.NewReader(rightPNG))
	if rightErr != nil {
		return false, rightErr
	}
	leftBounds := leftImage.Bounds()
	rightBounds := rightImage.Bounds()
	if leftBounds.Dx() != rightBounds.Dx() || leftBounds.Dy() != rightBounds.Dy() {
		return false, nil
	}
	for y := 0; y < leftBounds.Dy(); y++ {
		for x := 0; x < leftBounds.Dx(); x++ {
			leftColor := color.RGBAModel.Convert(leftImage.At(leftBounds.Min.X+x, leftBounds.Min.Y+y)).(color.RGBA)
			rightColor := color.RGBAModel.Convert(rightImage.At(rightBounds.Min.X+x, rightBounds.Min.Y+y)).(color.RGBA)
			if leftColor != rightColor {
				return false, nil
			}
		}
	}
	return true, nil
}

func writePatternPNG(outputPath string, width int, height int) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), outputDirFileMode); err != nil {
		return err
	}
	patternImage := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			patternImage.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 17) % 255),
				G: uint8((y * 29) % 255),
				B: uint8((x + y*3) % 255),
				A: 0xff,
			})
		}
	}
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()
	return png.Encode(outputFile, patternImage)
}

func writeSolidPNG(outputPath string, width int, height int, fillColor color.RGBA) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), outputDirFileMode); err != nil {
		return err
	}
	solidImage := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			solidImage.SetRGBA(x, y, fillColor)
		}
	}
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()
	return png.Encode(outputFile, solidImage)
}

func appendGeneratedSphereMetadata(
	t *testing.T,
	modelData *model.PmxModel,
	metadataByTexture map[string]generatedSphereMetadata,
) {
	t.Helper()
	if modelData == nil || modelData.VrmData == nil || modelData.VrmData.RawExtensions == nil {
		t.Fatal("raw extensions should be available")
	}
	encodedMetadata, err := json.Marshal(metadataByTexture)
	if err != nil {
		t.Fatalf("failed to marshal generated sphere metadata: %v", err)
	}
	modelData.VrmData.RawExtensions[legacyGeneratedSphereMetaKey] = encodedMetadata
}

func hasWarningID(modelData *model.PmxModel, targetWarningID string) bool {
	if modelData == nil || modelData.VrmData == nil || modelData.VrmData.RawExtensions == nil {
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
		if warningID == targetWarningID {
			return true
		}
	}
	return false
}
