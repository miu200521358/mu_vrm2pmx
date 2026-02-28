// 指示: miu200521358
package minteractor

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mu_vrm2pmx/pkg/adapter/io_model/vrm"
	"golang.org/x/image/bmp"
)

const (
	defaultTextureDirName = "tex"
	defaultGltfDirName    = "glTF"
	outputDirFileMode     = 0o755
	outputFileMode        = 0o644
)

var (
	nowFunc                    = time.Now
	generatedToonNamePattern   = regexp.MustCompile(`^toon[0-9]+\.bmp$`)
	generatedToonBaseShadeRGBA = color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff}
)

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
	exportGeneratedToonTextures(texDir, modelData)
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

// exportGeneratedToonTextures は変換時に生成した toon テクスチャを tex 直下へ出力する。
func exportGeneratedToonTextures(textureDir string, modelData *ModelData) {
	if modelData == nil || modelData.Textures == nil {
		return
	}
	trimmedTextureDir := strings.TrimSpace(textureDir)
	if trimmedTextureDir == "" {
		return
	}

	toonBytes, err := buildGeneratedToonBmp32()
	if err != nil {
		return
	}
	for _, textureData := range modelData.Textures.Values() {
		if textureData == nil || textureData.TextureType != model.TEXTURE_TYPE_TOON {
			continue
		}
		fileName, ok := resolveGeneratedToonFileName(textureData.Name())
		if !ok {
			continue
		}
		outputPath := filepath.Join(trimmedTextureDir, fileName)
		if err := os.WriteFile(outputPath, toonBytes, outputFileMode); err != nil {
			continue
		}
	}
}

// resolveGeneratedToonFileName は生成toonの出力対象ファイル名を解決する。
func resolveGeneratedToonFileName(textureName string) (string, bool) {
	normalizedTextureName := strings.ToLower(filepath.ToSlash(strings.TrimSpace(textureName)))
	if strings.HasPrefix(normalizedTextureName, defaultTextureDirName+"/") {
		normalizedTextureName = strings.TrimPrefix(normalizedTextureName, defaultTextureDirName+"/")
	}
	if strings.Contains(normalizedTextureName, "/") {
		return "", false
	}
	if !generatedToonNamePattern.MatchString(normalizedTextureName) {
		return "", false
	}
	return normalizedTextureName, true
}

// buildGeneratedToonBmp32 は旧仕様互換の 32x32 toon BMP を生成する。
func buildGeneratedToonBmp32() ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	upperColor := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	for y := 0; y < 32; y++ {
		lineColor := upperColor
		if y >= 24 {
			lineColor = generatedToonBaseShadeRGBA
		}
		for x := 0; x < 32; x++ {
			img.SetRGBA(x, y, lineColor)
		}
	}

	var out bytes.Buffer
	if err := bmp.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
