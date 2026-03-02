// 指示: miu200521358
package minteractor

import (
	"bytes"
	"encoding/json"
	"errors"
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

	appendTexture := func(name string, textureType model.TextureType) {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.TextureType = textureType
		texture.SetValid(true)
		modelData.Textures.AppendRaw(texture)
	}
	appendTexture("tex/hair_sphere_00.png", model.TEXTURE_TYPE_SPHERE)
	appendTexture("tex/sphere/matcap_sphere_001.png", model.TEXTURE_TYPE_SPHERE)
	appendTexture("tex/sphere/emissive_sphere_002.png", model.TEXTURE_TYPE_SPHERE)
	appendTexture("tex/sphere00.png", model.TEXTURE_TYPE_SPHERE)
	appendTexture("tex/hair_sphere_03.png", model.TEXTURE_TYPE_TOON)

	exportGeneratedSphereTextures(texDir, modelData)

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
	assertGenerated("hair_sphere_00.png", color.RGBA{R: 0xb4, G: 0xb4, B: 0xb4, A: 0xff})
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

	appendTexture := func(name string, textureType model.TextureType) int {
		texture := model.NewTexture()
		texture.SetName(name)
		texture.EnglishName = name
		texture.TextureType = textureType
		texture.SetValid(true)
		return modelData.Textures.AppendRaw(texture)
	}
	hairSphereTextureIndex := appendTexture("tex/hair_sphere_00.png", model.TEXTURE_TYPE_SPHERE)
	appendTexture("tex/sphere/matcap_sphere_001.png", model.TEXTURE_TYPE_SPHERE)

	hairMaterial := model.NewMaterial()
	hairMaterial.SetName("N00_000_Hair_00_HAIR_01")
	hairMaterial.EnglishName = "N00_000_Hair_00_HAIR_01"
	hairMaterial.SphereMode = model.SPHERE_MODE_ADDITION
	hairMaterial.SphereTextureIndex = hairSphereTextureIndex
	modelData.Materials.AppendRaw(hairMaterial)

	exportGeneratedSphereTextures(texDir, modelData)

	if _, err := os.Stat(filepath.Join(texDir, "hair_sphere_00.png")); err != nil {
		t.Fatalf("generated hair sphere texture not found: %v", err)
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
