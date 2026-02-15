// 指示: miu200521358
package minteractor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/miu200521358/mu_vrm2pmx/pkg/adapter/io_model/vrm"
)

const (
	defaultTextureDirName = "tex"
	defaultGltfDirName    = "glTF"
	outputDirFileMode     = 0o755
)

var nowFunc = time.Now

// BuildDefaultOutputPath は入力VRMパスから既定のPMX出力パスを生成する。
func BuildDefaultOutputPath(inputPath string) string {
	return buildDefaultOutputPathAt(inputPath, nowFunc())
}

// buildDefaultOutputPathAt は指定時刻で既定のPMX出力パスを生成する。
func buildDefaultOutputPathAt(inputPath string, now time.Time) string {
	dir := filepath.Dir(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	stamp := now.Format("20060102150405")
	outDir := filepath.Join(dir, fmt.Sprintf("%s_%s", base, stamp))
	return filepath.Join(outDir, base+".pmx")
}

// prepareOutputLayout は出力先レイアウトを準備し、補助出力を生成する。
func prepareOutputLayout(inputPath string, outputPath string, modelData *ModelData) error {
	texDir, gltfDir, err := createOutputDirs(outputPath)
	if err != nil {
		return err
	}

	artifacts, err := vrm.ExportArtifacts(inputPath, gltfDir, texDir)
	if err != nil {
		return err
	}
	if _, err := vrm.ExportEmbeddedSpecialEyeTextures(texDir); err != nil {
		return err
	}
	if artifacts == nil {
		return nil
	}
	applyTextureOutputPaths(modelData, artifacts.TextureNames)
	return nil
}

// createOutputDirs は PMX/tex/glTF の出力ディレクトリを作成する。
func createOutputDirs(outputPath string) (string, string, error) {
	outputDir := filepath.Dir(outputPath)
	if outputDir == "" {
		return "", "", fmt.Errorf("保存先ディレクトリの解決に失敗しました")
	}
	if err := os.MkdirAll(outputDir, outputDirFileMode); err != nil {
		return "", "", fmt.Errorf("保存先ディレクトリの作成に失敗しました: %w", err)
	}
	texDir := filepath.Join(outputDir, defaultTextureDirName)
	if err := os.MkdirAll(texDir, outputDirFileMode); err != nil {
		return "", "", fmt.Errorf("tex ディレクトリの作成に失敗しました: %w", err)
	}
	gltfDir := filepath.Join(outputDir, defaultGltfDirName)
	if err := os.MkdirAll(gltfDir, outputDirFileMode); err != nil {
		return "", "", fmt.Errorf("glTF ディレクトリの作成に失敗しました: %w", err)
	}
	return texDir, gltfDir, nil
}

// applyTextureOutputPaths は抽出済みテクスチャ名をモデルへ反映する。
func applyTextureOutputPaths(modelData *ModelData, imageNames []string) {
	if modelData == nil || modelData.Textures == nil || len(imageNames) == 0 {
		return
	}
	for index, texture := range modelData.Textures.Values() {
		if texture == nil {
			continue
		}
		if index < 0 || index >= len(imageNames) {
			continue
		}
		imageName := strings.TrimSpace(imageNames[index])
		if imageName == "" {
			continue
		}
		texture.SetName(filepath.Join(defaultTextureDirName, imageName))
		texture.SetValid(true)
	}
}
