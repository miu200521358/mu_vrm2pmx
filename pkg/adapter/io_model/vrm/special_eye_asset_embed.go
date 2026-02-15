// 指示: miu200521358
package vrm

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const specialEyeEmbeddedAssetsDir = "assets"

var specialEyeEmbeddedTextureAssetFileNames = []string{
	"eye_star.png",
	"eye_heart.png",
	"eye_hau.png",
	"eye_hachume.png",
	"eye_nagomi.png",
}

// specialEyeEmbeddedTextureFiles は特殊目テクスチャの組み込みリソースを保持する。
//
//go:embed assets/eye_*.png
var specialEyeEmbeddedTextureFiles embed.FS

// ExportEmbeddedSpecialEyeTextures は組み込み特殊目テクスチャを出力先 tex ディレクトリへ展開する。
func ExportEmbeddedSpecialEyeTextures(textureDir string) ([]string, error) {
	trimmedTextureDir := strings.TrimSpace(textureDir)
	if trimmedTextureDir == "" {
		return nil, fmt.Errorf("特殊目テクスチャ出力先ディレクトリが未指定です")
	}
	if err := os.MkdirAll(trimmedTextureDir, exportDirMode); err != nil {
		return nil, fmt.Errorf("特殊目テクスチャ出力先ディレクトリの作成に失敗しました: %w", err)
	}
	writtenFileNames := make([]string, 0, len(specialEyeEmbeddedTextureAssetFileNames))
	for _, fileName := range specialEyeEmbeddedTextureAssetFileNames {
		assetPath := filepath.ToSlash(filepath.Join(specialEyeEmbeddedAssetsDir, fileName))
		textureBytes, err := specialEyeEmbeddedTextureFiles.ReadFile(assetPath)
		if err != nil {
			return nil, fmt.Errorf("特殊目テクスチャの読込に失敗しました: %s: %w", fileName, err)
		}
		outputPath := filepath.Join(trimmedTextureDir, fileName)
		if _, err := os.Stat(outputPath); err == nil {
			writtenFileNames = append(writtenFileNames, fileName)
			continue
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("特殊目テクスチャ出力先の確認に失敗しました: %s: %w", fileName, err)
		}
		if err := os.WriteFile(outputPath, textureBytes, exportFileMode); err != nil {
			return nil, fmt.Errorf("特殊目テクスチャの保存に失敗しました: %s: %w", fileName, err)
		}
		writtenFileNames = append(writtenFileNames, fileName)
	}
	return writtenFileNames, nil
}
