// 指示: miu200521358
package minteractor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

	legacyGeneratedSphereMetaKey = "MU_VRM2PMX_legacy_generated_sphere_metadata"
)

var (
	nowFunc                  = time.Now
	generatedToonNamePattern = regexp.MustCompile(`^toon[0-9]+\.bmp$`)
	generatedHairSphereName  = regexp.MustCompile(`^hair_sphere_[0-9]{2}\.png$`)
	generatedMatcapSphere    = regexp.MustCompile(`^sphere/matcap_sphere_[0-9]{3}\.png$`)
	generatedEmissiveSphere  = regexp.MustCompile(`^sphere/emissive_sphere_[0-9]{3}\.png$`)
	generatedTextureNumber   = regexp.MustCompile(`_(\d+)`)
)

var (
	errGeneratedSphereSourceMissing = errors.New("generated sphere source missing")
)

type generatedSphereKind int

const (
	generatedSphereKindUnknown generatedSphereKind = iota
	generatedSphereKindHair
	generatedSphereKindMatcap
	generatedSphereKindEmissive
)

type generatedSphereMetadata struct {
	SourceTextureIndex int        `json:"source_texture_index"`
	MaterialIndex      int        `json:"material_index"`
	SphereKind         string     `json:"sphere_kind"`
	EmissiveFactor     [3]float64 `json:"emissive_factor"`
	DiffuseFactor      [4]float64 `json:"diffuse_factor"`
	HighlightTexture   string     `json:"highlight_texture_name"`
	BlendTexture       string     `json:"blend_texture_name"`
}

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
	sphereMetadataMap := resolveGeneratedSphereMetadataMap(modelData)

	for textureIndex, textureData := range modelData.Textures.Values() {
		if textureData == nil || textureData.TextureType != model.TEXTURE_TYPE_SPHERE {
			continue
		}
		relativePath, sphereKind, ok := resolveGeneratedSphereRelativePath(textureData.Name())
		if !ok {
			continue
		}
		normalizedTextureName := normalizeGeneratedSphereTextureName(textureData.Name())
		sphereMetadata, hasMetadata := resolveGeneratedSphereMetadata(sphereMetadataMap, normalizedTextureName)

		sphereBytes, err := buildGeneratedSphereTextureBytes(
			trimmedTextureDir,
			modelData,
			sphereKind,
			sphereMetadata,
			hasMetadata,
		)
		if err != nil {
			if errors.Is(err, errGeneratedSphereSourceMissing) {
				appendGeneratedSphereWarningID(modelData, warningid.VrmWarningSphereTextureSourceMissing)
			} else {
				appendGeneratedSphereWarningID(modelData, warningid.VrmWarningSphereTextureGenerationFailed)
			}
			disableSphereMaterialsByTextureIndex(modelData, textureIndex)
			continue
		}
		outputPath := filepath.Join(trimmedTextureDir, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(outputPath), outputDirFileMode); err != nil {
			appendGeneratedSphereWarningID(modelData, warningid.VrmWarningSphereTextureGenerationFailed)
			disableSphereMaterialsByTextureIndex(modelData, textureIndex)
			continue
		}
		if err := os.WriteFile(outputPath, sphereBytes, outputFileMode); err != nil {
			appendGeneratedSphereWarningID(modelData, warningid.VrmWarningSphereTextureGenerationFailed)
			disableSphereMaterialsByTextureIndex(modelData, textureIndex)
			continue
		}
		if sphereKind == generatedSphereKindHair && hasMetadata {
			if blendErr := exportGeneratedHairBlendPng(trimmedTextureDir, modelData, sphereMetadata); blendErr != nil {
				if errors.Is(blendErr, errGeneratedSphereSourceMissing) {
					appendGeneratedSphereWarningID(modelData, warningid.VrmWarningSphereTextureSourceMissing)
				} else {
					appendGeneratedSphereWarningID(modelData, warningid.VrmWarningSphereTextureGenerationFailed)
				}
			}
		}
	}
}

// buildGeneratedSphereTextureBytes は sphere 種別ごとの生成処理を選択して PNG バイト列を返す。
func buildGeneratedSphereTextureBytes(
	textureDir string,
	modelData *ModelData,
	sphereKind generatedSphereKind,
	sphereMetadata generatedSphereMetadata,
	hasMetadata bool,
) ([]byte, error) {
	if !hasMetadata {
		return nil, errGeneratedSphereSourceMissing
	}
	switch sphereKind {
	case generatedSphereKindHair:
		return buildGeneratedHairSpherePng(textureDir, modelData, sphereMetadata)
	case generatedSphereKindMatcap, generatedSphereKindEmissive:
		return buildGeneratedSourceSpherePng(textureDir, modelData, sphereKind, sphereMetadata)
	default:
		return nil, errGeneratedSphereSourceMissing
	}
}

func resolveGeneratedSphereMetadataMap(modelData *ModelData) map[string]generatedSphereMetadata {
	metadataMap := map[string]generatedSphereMetadata{}
	if modelData == nil || modelData.VrmData == nil || modelData.VrmData.RawExtensions == nil {
		return metadataMap
	}

	rawSphereMetadata, exists := modelData.VrmData.RawExtensions[legacyGeneratedSphereMetaKey]
	if !exists || len(rawSphereMetadata) == 0 {
		return metadataMap
	}

	decodedMetadata := map[string]generatedSphereMetadata{}
	if err := json.Unmarshal(rawSphereMetadata, &decodedMetadata); err != nil {
		return metadataMap
	}
	for textureName, metadata := range decodedMetadata {
		normalizedTextureName := normalizeGeneratedSphereTextureName(textureName)
		if normalizedTextureName == "" {
			continue
		}
		metadataMap[normalizedTextureName] = metadata
		if strings.HasPrefix(normalizedTextureName, defaultTextureDirName+"/") {
			metadataMap[strings.TrimPrefix(normalizedTextureName, defaultTextureDirName+"/")] = metadata
		} else {
			metadataMap[defaultTextureDirName+"/"+normalizedTextureName] = metadata
		}
	}
	return metadataMap
}

func normalizeGeneratedSphereTextureName(textureName string) string {
	return strings.ToLower(filepath.ToSlash(strings.TrimSpace(textureName)))
}

func resolveGeneratedSphereMetadata(
	sphereMetadataMap map[string]generatedSphereMetadata,
	textureName string,
) (generatedSphereMetadata, bool) {
	normalizedTextureName := normalizeGeneratedSphereTextureName(textureName)
	if normalizedTextureName == "" {
		return generatedSphereMetadata{}, false
	}
	if metadata, exists := sphereMetadataMap[normalizedTextureName]; exists {
		return metadata, true
	}
	if strings.HasPrefix(normalizedTextureName, defaultTextureDirName+"/") {
		trimmedTextureName := strings.TrimPrefix(normalizedTextureName, defaultTextureDirName+"/")
		metadata, exists := sphereMetadataMap[trimmedTextureName]
		return metadata, exists
	}
	metadata, exists := sphereMetadataMap[defaultTextureDirName+"/"+normalizedTextureName]
	return metadata, exists
}

func buildGeneratedHairSpherePng(
	textureDir string,
	modelData *ModelData,
	sphereMetadata generatedSphereMetadata,
) ([]byte, error) {
	sourceImage, err := loadGeneratedSphereSourceImage(textureDir, modelData, sphereMetadata.SourceTextureIndex)
	if err != nil {
		return nil, err
	}
	sourceBounds := sourceImage.Bounds()
	if sourceBounds.Dx() <= 0 || sourceBounds.Dy() <= 0 {
		return nil, fmt.Errorf("%w: invalid source dimensions", errGeneratedSphereSourceMissing)
	}
	templateImage, templateErr := loadGeneratedHairSphereTemplateImage()
	if templateErr != nil {
		return nil, templateErr
	}
	emissiveFactor := resolveGeneratedHairEmissiveFactor(sphereMetadata.EmissiveFactor)

	hairSphereImage := image.NewRGBA(image.Rect(0, 0, sourceBounds.Dx(), sourceBounds.Dy()))
	for y := 0; y < sourceBounds.Dy(); y++ {
		for x := 0; x < sourceBounds.Dx(); x++ {
			templateColor := sampleScaledColor(templateImage, x, y, sourceBounds.Dx(), sourceBounds.Dy())
			hairSphereImage.SetRGBA(
				x,
				y,
				multiplyColorByFactor(templateColor, [4]float64{
					emissiveFactor[0],
					emissiveFactor[1],
					emissiveFactor[2],
					1.0,
				}),
			)
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, hairSphereImage); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func exportGeneratedHairBlendPng(textureDir string, modelData *ModelData, sphereMetadata generatedSphereMetadata) error {
	highlightTextureName, blendTextureName, shouldGenerate := resolveGeneratedHairBlendTextureNames(modelData, sphereMetadata)
	if !shouldGenerate {
		return nil
	}

	sourceImage, err := loadGeneratedSphereSourceImage(textureDir, modelData, sphereMetadata.SourceTextureIndex)
	if err != nil {
		return err
	}
	highlightImage, err := loadGeneratedSphereImageByTextureName(textureDir, highlightTextureName)
	if err != nil {
		return err
	}

	sourceBounds := sourceImage.Bounds()
	if sourceBounds.Dx() <= 0 || sourceBounds.Dy() <= 0 {
		return fmt.Errorf("%w: invalid source dimensions", errGeneratedSphereSourceMissing)
	}
	blendOutputPath, ok := resolveOutputTexturePath(textureDir, blendTextureName)
	if !ok {
		return errGeneratedSphereSourceMissing
	}

	diffuseFactor := resolveGeneratedHairDiffuseFactor(sphereMetadata.DiffuseFactor)
	emissiveFactor := resolveGeneratedHairEmissiveFactor(sphereMetadata.EmissiveFactor)
	blendImage := image.NewRGBA(image.Rect(0, 0, sourceBounds.Dx(), sourceBounds.Dy()))
	for y := 0; y < sourceBounds.Dy(); y++ {
		for x := 0; x < sourceBounds.Dx(); x++ {
			hairColor := sampleScaledColor(sourceImage, x, y, sourceBounds.Dx(), sourceBounds.Dy())
			highlightColor := sampleScaledColor(highlightImage, x, y, sourceBounds.Dx(), sourceBounds.Dy())
			hairDiffuseColor := multiplyColorByFactor(hairColor, diffuseFactor)
			hairEmissiveColor := multiplyColorByFactor(highlightColor, [4]float64{
				emissiveFactor[0],
				emissiveFactor[1],
				emissiveFactor[2],
				1.0,
			})
			blendImage.SetRGBA(x, y, screenBlendColor(hairDiffuseColor, hairEmissiveColor))
		}
	}

	if err := os.MkdirAll(filepath.Dir(blendOutputPath), outputDirFileMode); err != nil {
		return err
	}
	blendFile, createErr := os.Create(blendOutputPath)
	if createErr != nil {
		return createErr
	}
	defer blendFile.Close()
	if encodeErr := png.Encode(blendFile, blendImage); encodeErr != nil {
		return encodeErr
	}
	return nil
}

func loadGeneratedHairSphereTemplateImage() (image.Image, error) {
	templateBytes, err := vrm.LoadEmbeddedHairSphereTemplatePNG()
	if err != nil {
		return nil, err
	}
	templateImage, decodeErr := png.Decode(bytes.NewReader(templateBytes))
	if decodeErr != nil {
		return nil, decodeErr
	}
	return templateImage, nil
}

func loadGeneratedSphereImageByTextureName(textureDir string, textureName string) (image.Image, error) {
	sourceTexturePath, ok := resolveOutputTexturePath(textureDir, textureName)
	if !ok {
		return nil, errGeneratedSphereSourceMissing
	}
	sourceFile, err := os.Open(sourceTexturePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errGeneratedSphereSourceMissing
		}
		return nil, err
	}
	defer sourceFile.Close()

	sourceImage, decodeErr := decodeGeneratedSphereSourceImage(sourceFile, sourceTexturePath)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return sourceImage, nil
}

func resolveGeneratedHairBlendTextureNames(
	modelData *ModelData,
	sphereMetadata generatedSphereMetadata,
) (string, string, bool) {
	highlightTextureName := strings.TrimSpace(sphereMetadata.HighlightTexture)
	blendTextureName := strings.TrimSpace(sphereMetadata.BlendTexture)
	if highlightTextureName != "" && blendTextureName != "" {
		return highlightTextureName, blendTextureName, true
	}

	if modelData == nil || modelData.Textures == nil || sphereMetadata.SourceTextureIndex < 0 {
		return "", "", false
	}
	sourceTexture, err := modelData.Textures.Get(sphereMetadata.SourceTextureIndex)
	if err != nil || sourceTexture == nil {
		return "", "", false
	}
	textureNumber, ok := resolveGeneratedTextureNumber(strings.TrimSpace(sourceTexture.Name()))
	if !ok {
		return "", "", false
	}
	return fmt.Sprintf("_%02d.png", textureNumber+1), fmt.Sprintf("_%02d_blend.png", textureNumber), true
}

func resolveGeneratedTextureNumber(textureName string) (int, bool) {
	matches := generatedTextureNumber.FindStringSubmatch(strings.TrimSpace(filepath.Base(filepath.ToSlash(textureName))))
	if len(matches) < 2 {
		return 0, false
	}
	textureNumber, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}
	return textureNumber, true
}

func resolveGeneratedHairEmissiveFactor(emissiveFactor [3]float64) [3]float64 {
	if math.Abs(emissiveFactor[0])+math.Abs(emissiveFactor[1])+math.Abs(emissiveFactor[2]) < 1e-9 {
		return [3]float64{0.9, 0.9, 0.9}
	}
	return emissiveFactor
}

func resolveGeneratedHairDiffuseFactor(diffuseFactor [4]float64) [4]float64 {
	if math.Abs(diffuseFactor[0])+math.Abs(diffuseFactor[1])+math.Abs(diffuseFactor[2])+math.Abs(diffuseFactor[3]) < 1e-9 {
		return [4]float64{1.0, 1.0, 1.0, 1.0}
	}
	return diffuseFactor
}

func sampleScaledColor(sourceImage image.Image, x int, y int, width int, height int) color.RGBA {
	if sourceImage == nil || width <= 0 || height <= 0 {
		return color.RGBA{}
	}
	bounds := sourceImage.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return color.RGBA{}
	}

	srcX := 0
	if bounds.Dx() > 1 && width > 1 {
		srcX = int(math.Round(float64(x) * float64(bounds.Dx()-1) / float64(width-1)))
	}
	srcY := 0
	if bounds.Dy() > 1 && height > 1 {
		srcY = int(math.Round(float64(y) * float64(bounds.Dy()-1) / float64(height-1)))
	}
	return color.RGBAModel.Convert(sourceImage.At(bounds.Min.X+srcX, bounds.Min.Y+srcY)).(color.RGBA)
}

func multiplyColorByFactor(sourceColor color.RGBA, factor [4]float64) color.RGBA {
	return color.RGBA{
		R: multiplyColorChannel(sourceColor.R, factor[0]),
		G: multiplyColorChannel(sourceColor.G, factor[1]),
		B: multiplyColorChannel(sourceColor.B, factor[2]),
		A: multiplyColorChannel(sourceColor.A, factor[3]),
	}
}

func multiplyColorChannel(value uint8, factor float64) uint8 {
	return clampColor(float64(value) * factor)
}

func screenBlendColor(baseColor color.RGBA, overlayColor color.RGBA) color.RGBA {
	return color.RGBA{
		R: screenBlendChannel(baseColor.R, overlayColor.R),
		G: screenBlendChannel(baseColor.G, overlayColor.G),
		B: screenBlendChannel(baseColor.B, overlayColor.B),
		A: screenBlendChannel(baseColor.A, overlayColor.A),
	}
}

func screenBlendChannel(base uint8, overlay uint8) uint8 {
	baseValue := float64(base)
	overlayValue := float64(overlay)
	return clampColor(255.0 - ((255.0 - baseValue) * (255.0 - overlayValue) / 255.0))
}

func clampColor(value float64) uint8 {
	if value <= 0 {
		return 0
	}
	if value >= 255 {
		return 255
	}
	return uint8(math.Round(value))
}

func loadGeneratedSphereSourceImage(
	textureDir string,
	modelData *ModelData,
	sourceTextureIndex int,
) (image.Image, error) {
	if sourceTextureIndex < 0 || modelData == nil || modelData.Textures == nil {
		return nil, errGeneratedSphereSourceMissing
	}

	sourceTexture, getTextureErr := modelData.Textures.Get(sourceTextureIndex)
	if getTextureErr != nil || sourceTexture == nil {
		return nil, errGeneratedSphereSourceMissing
	}

	sourceTexturePath, ok := resolveOutputTexturePath(textureDir, sourceTexture.Name())
	if !ok {
		return nil, errGeneratedSphereSourceMissing
	}
	sourceFile, err := os.Open(sourceTexturePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errGeneratedSphereSourceMissing
		}
		return nil, err
	}
	defer sourceFile.Close()

	sourceImage, decodeErr := decodeGeneratedSphereSourceImage(sourceFile, sourceTexturePath)
	if decodeErr != nil {
		return nil, decodeErr
	}
	return sourceImage, nil
}

func decodeGeneratedSphereSourceImage(sourceFile *os.File, sourceTexturePath string) (image.Image, error) {
	if sourceFile == nil {
		return nil, errGeneratedSphereSourceMissing
	}
	switch strings.ToLower(filepath.Ext(sourceTexturePath)) {
	case ".png":
		return png.Decode(sourceFile)
	case ".bmp":
		return bmp.Decode(sourceFile)
	case ".jpg", ".jpeg":
		return jpeg.Decode(sourceFile)
	case ".gif":
		return gif.Decode(sourceFile)
	default:
		return nil, fmt.Errorf("unsupported generated sphere source format: %s", filepath.Ext(sourceTexturePath))
	}
}

func resolveOutputTexturePath(textureDir string, textureName string) (string, bool) {
	normalizedTextureName := filepath.ToSlash(strings.TrimSpace(textureName))
	if normalizedTextureName == "" {
		return "", false
	}
	if strings.HasPrefix(strings.ToLower(normalizedTextureName), defaultTextureDirName+"/") {
		normalizedTextureName = normalizedTextureName[len(defaultTextureDirName)+1:]
	}
	normalizedTextureName = strings.TrimPrefix(normalizedTextureName, "./")
	normalizedTextureName = filepath.ToSlash(normalizedTextureName)
	if normalizedTextureName == "" || strings.HasPrefix(normalizedTextureName, "/") || strings.Contains(normalizedTextureName, "..") {
		return "", false
	}
	return filepath.Join(textureDir, filepath.FromSlash(normalizedTextureName)), true
}

func appendGeneratedSphereWarningID(modelData *ModelData, warningID string) {
	if modelData == nil || modelData.VrmData == nil {
		return
	}
	if modelData.VrmData.RawExtensions == nil {
		modelData.VrmData.RawExtensions = map[string]json.RawMessage{}
	}

	warningIDs := []string{}
	if rawWarnings, exists := modelData.VrmData.RawExtensions[warningid.VrmWarningRawExtensionKey]; exists && len(rawWarnings) > 0 {
		if err := json.Unmarshal(rawWarnings, &warningIDs); err != nil {
			warningIDs = []string{}
		}
	}

	normalizedWarningID := strings.TrimSpace(warningID)
	if normalizedWarningID == "" {
		return
	}
	for _, existingWarningID := range warningIDs {
		if strings.TrimSpace(existingWarningID) == normalizedWarningID {
			return
		}
	}

	warningIDs = append(warningIDs, normalizedWarningID)
	encodedWarnings, err := json.Marshal(warningIDs)
	if err != nil {
		return
	}
	modelData.VrmData.RawExtensions[warningid.VrmWarningRawExtensionKey] = encodedWarnings
}

func disableSphereMaterialsByTextureIndex(modelData *ModelData, sphereTextureIndex int) {
	if modelData == nil || modelData.Materials == nil || sphereTextureIndex < 0 {
		return
	}
	for _, materialData := range modelData.Materials.Values() {
		if materialData == nil || materialData.SphereTextureIndex != sphereTextureIndex {
			continue
		}
		materialData.SphereTextureIndex = 0
		materialData.SphereMode = model.SPHERE_MODE_INVALID
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

// buildGeneratedSourceSpherePng は source テクスチャ由来の sphere PNG を生成する。
func buildGeneratedSourceSpherePng(
	textureDir string,
	modelData *ModelData,
	sphereKind generatedSphereKind,
	sphereMetadata generatedSphereMetadata,
) ([]byte, error) {
	sourceImage, err := loadGeneratedSphereSourceImage(textureDir, modelData, sphereMetadata.SourceTextureIndex)
	if err != nil {
		return nil, err
	}
	sourceBounds := sourceImage.Bounds()
	if sourceBounds.Dx() <= 0 || sourceBounds.Dy() <= 0 {
		return nil, fmt.Errorf("%w: invalid source dimensions", errGeneratedSphereSourceMissing)
	}

	colorFactor := resolveGeneratedSphereColorFactor(sphereKind, sphereMetadata)
	sphereImage := image.NewRGBA(image.Rect(0, 0, sourceBounds.Dx(), sourceBounds.Dy()))
	for y := 0; y < sourceBounds.Dy(); y++ {
		for x := 0; x < sourceBounds.Dx(); x++ {
			sourceColor := color.RGBAModel.Convert(sourceImage.At(sourceBounds.Min.X+x, sourceBounds.Min.Y+y)).(color.RGBA)
			sphereImage.SetRGBA(x, y, multiplyColorByFactor(sourceColor, colorFactor))
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, sphereImage); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// resolveGeneratedSphereColorFactor は sphere 種別に応じた色係数を返す。
func resolveGeneratedSphereColorFactor(
	sphereKind generatedSphereKind,
	sphereMetadata generatedSphereMetadata,
) [4]float64 {
	switch sphereKind {
	case generatedSphereKindEmissive:
		return [4]float64{
			sphereMetadata.EmissiveFactor[0],
			sphereMetadata.EmissiveFactor[1],
			sphereMetadata.EmissiveFactor[2],
			1.0,
		}
	default:
		return [4]float64{1.0, 1.0, 1.0, 1.0}
	}
}
