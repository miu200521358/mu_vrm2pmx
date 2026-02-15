// 指示: miu200521358
package vrm

import (
	"path/filepath"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/domain/model"
)

func TestAppendEmbeddedSpecialEyeTexturesRegistersRequiredTextures(t *testing.T) {
	modelData := model.NewPmxModel()

	existingTexture := model.NewTexture()
	existingTexture.SetName(filepath.Join("tex", "eye_star.png"))
	existingTexture.EnglishName = filepath.Join("tex", "eye_star.png")
	existingTexture.SetValid(true)
	modelData.Textures.AppendRaw(existingTexture)

	beforeCount := modelData.Textures.Len()
	appendEmbeddedSpecialEyeTextures(modelData)
	afterCount := modelData.Textures.Len()
	if afterCount <= beforeCount {
		t.Fatalf("embedded special eye textures should be added: before=%d after=%d", beforeCount, afterCount)
	}

	for _, fileName := range specialEyeEmbeddedTextureAssetFileNames {
		normalizedToken := normalizeSpecialEyeToken(fileName)
		if textureIndex := findSpecialEyeTextureIndexByToken(modelData, normalizedToken); textureIndex < 0 {
			t.Fatalf("embedded special eye texture not registered: file=%s token=%s", fileName, normalizedToken)
		}
	}

	idempotentBefore := modelData.Textures.Len()
	appendEmbeddedSpecialEyeTextures(modelData)
	idempotentAfter := modelData.Textures.Len()
	if idempotentAfter != idempotentBefore {
		t.Fatalf("appendEmbeddedSpecialEyeTextures should be idempotent: before=%d after=%d", idempotentBefore, idempotentAfter)
	}
}
