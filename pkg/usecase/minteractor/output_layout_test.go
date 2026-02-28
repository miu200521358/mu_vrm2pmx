// 指示: miu200521358
package minteractor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/domain/model"
)

func TestExportGeneratedToonTexturesWritesTexRootBmp(t *testing.T) {
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
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("generated toon texture not found: %v", err)
	}
	if len(data) < 2 || data[0] != 'B' || data[1] != 'M' {
		t.Fatalf("generated toon texture should be BMP: size=%d", len(data))
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
