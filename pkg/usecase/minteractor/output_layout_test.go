// 指示: miu200521358
package minteractor

import (
	"bytes"
	"encoding/json"
	"errors"
	"image/color"
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

func readToonLowerPixelColor(bmpData []byte) (color.RGBA, error) {
	img, err := bmp.Decode(bytes.NewReader(bmpData))
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBAModel.Convert(img.At(0, 24)).(color.RGBA), nil
}
