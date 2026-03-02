// 指示: miu200521358
package minteractor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mu_vrm2pmx/pkg/adapter/io_model/vrm"
	warningid "github.com/miu200521358/mu_vrm2pmx/pkg/domain/model"
	"golang.org/x/image/bmp"
)

const (
	defaultTextureDirName = "tex"
	defaultGltfDirName    = "glTF"
	outputDirFileMode     = 0o755
	outputFileMode        = 0o644
)

var (
	nowFunc                  = time.Now
	generatedToonNamePattern = regexp.MustCompile(`^toon[0-9]+\.bmp$`)
	generatedHairSphereName  = regexp.MustCompile(`^hair_sphere_[0-9]{2}\.png$`)
	generatedMatcapSphere    = regexp.MustCompile(`^sphere/matcap_sphere_[0-9]{3}\.png$`)
	generatedEmissiveSphere  = regexp.MustCompile(`^sphere/emissive_sphere_[0-9]{3}\.png$`)
)

type generatedSphereKind int

const (
	generatedSphereKindUnknown generatedSphereKind = iota
	generatedSphereKindHair
	generatedSphereKindMatcap
	generatedSphereKindEmissive
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
	exportGeneratedSphereTextures(texDir, modelData)
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

	toonShadeColorMap := resolveGeneratedToonShadeColorMap(modelData)
	for _, textureData := range modelData.Textures.Values() {
		if textureData == nil || textureData.TextureType != model.TEXTURE_TYPE_TOON {
			continue
		}
		fileName, ok := resolveGeneratedToonFileName(textureData.Name())
		if !ok {
			continue
		}
		mappedShadeColor, exists := toonShadeColorMap[fileName]
		if !exists {
			continue
		}
		shadeColor := color.RGBA{
			R: mappedShadeColor[0],
			G: mappedShadeColor[1],
			B: mappedShadeColor[2],
			A: 0xff,
		}
		toonBytes, err := buildGeneratedToonBmp32(shadeColor)
		if err != nil {
			continue
		}
		outputPath := filepath.Join(trimmedTextureDir, fileName)
		if err := os.WriteFile(outputPath, toonBytes, outputFileMode); err != nil {
			continue
		}
	}
}

// exportGeneratedSphereTextures は変換時に生成した sphere テクスチャを tex 配下へ出力する。
func exportGeneratedSphereTextures(textureDir string, modelData *ModelData) {
	if modelData == nil || modelData.Textures == nil {
		return
	}
	trimmedTextureDir := strings.TrimSpace(textureDir)
	if trimmedTextureDir == "" {
		return
	}

	for _, textureData := range modelData.Textures.Values() {
		if textureData == nil || textureData.TextureType != model.TEXTURE_TYPE_SPHERE {
			continue
		}
		relativePath, sphereKind, ok := resolveGeneratedSphereRelativePath(textureData.Name())
		if !ok {
			continue
		}
		sphereBytes, err := buildGeneratedSpherePng32(sphereKind)
		if err != nil {
			continue
		}
		outputPath := filepath.Join(trimmedTextureDir, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(outputPath), outputDirFileMode); err != nil {
			continue
		}
		if err := os.WriteFile(outputPath, sphereBytes, outputFileMode); err != nil {
			continue
		}
	}
}

// resolveGeneratedToonShadeColorMap は生成toonの shade 色マップを RawExtensions から復元する。
func resolveGeneratedToonShadeColorMap(modelData *ModelData) map[string][3]uint8 {
	toonShadeColorMap := map[string][3]uint8{}
	if modelData == nil || modelData.VrmData == nil || modelData.VrmData.RawExtensions == nil {
		return toonShadeColorMap
	}

	rawShadeColorMap, exists := modelData.VrmData.RawExtensions[warningid.VrmLegacyGeneratedToonShadeMapRawExtensionKey]
	if !exists || len(rawShadeColorMap) == 0 {
		return toonShadeColorMap
	}

	decodedShadeColorMap := map[string][3]uint8{}
	if err := json.Unmarshal(rawShadeColorMap, &decodedShadeColorMap); err != nil {
		return toonShadeColorMap
	}
	for rawFileName, shadeColor := range decodedShadeColorMap {
		fileName, ok := resolveGeneratedToonFileName(rawFileName)
		if !ok {
			continue
		}
		toonShadeColorMap[fileName] = shadeColor
	}
	return toonShadeColorMap
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

// resolveGeneratedSphereRelativePath は生成 sphere の出力相対パスを解決する。
func resolveGeneratedSphereRelativePath(textureName string) (string, generatedSphereKind, bool) {
	normalizedTextureName := strings.ToLower(filepath.ToSlash(strings.TrimSpace(textureName)))
	if strings.HasPrefix(normalizedTextureName, defaultTextureDirName+"/") {
		normalizedTextureName = strings.TrimPrefix(normalizedTextureName, defaultTextureDirName+"/")
	}
	if strings.Contains(normalizedTextureName, "..") {
		return "", generatedSphereKindUnknown, false
	}

	switch {
	case generatedHairSphereName.MatchString(normalizedTextureName):
		return normalizedTextureName, generatedSphereKindHair, true
	case generatedMatcapSphere.MatchString(normalizedTextureName):
		return normalizedTextureName, generatedSphereKindMatcap, true
	case generatedEmissiveSphere.MatchString(normalizedTextureName):
		return normalizedTextureName, generatedSphereKindEmissive, true
	default:
		return "", generatedSphereKindUnknown, false
	}
}

// buildGeneratedToonBmp32 は旧仕様互換の 32x32 toon BMP を生成する。
func buildGeneratedToonBmp32(lowerColor color.RGBA) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	upperColor := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	for y := 0; y < 32; y++ {
		lineColor := upperColor
		if y >= 24 {
			lineColor = lowerColor
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

// buildGeneratedSpherePng32 は生成 sphere のダミー PNG を組み立てる。
func buildGeneratedSpherePng32(kind generatedSphereKind) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	fillColor := generatedSphereColor(kind)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.SetRGBA(x, y, fillColor)
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func generatedSphereColor(kind generatedSphereKind) color.RGBA {
	switch kind {
	case generatedSphereKindHair:
		return color.RGBA{R: 0xb4, G: 0xb4, B: 0xb4, A: 0xff}
	case generatedSphereKindMatcap:
		return color.RGBA{R: 0xd8, G: 0xd8, B: 0xd8, A: 0xff}
	case generatedSphereKindEmissive:
		return color.RGBA{R: 0xf0, G: 0xf0, B: 0xf0, A: 0xff}
	default:
		return color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}
}
