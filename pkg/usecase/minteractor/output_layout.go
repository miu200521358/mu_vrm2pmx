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

	legacyGeneratedSphereMetaKey = "MU_VRM2PMX_legacy_generated_sphere_metadata"
)

var (
	nowFunc                  = time.Now
	generatedToonNamePattern = regexp.MustCompile(`^toon[0-9]+\.bmp$`)
	generatedHairSphereName  = regexp.MustCompile(`^hair_sphere_[0-9]{2}\.png$`)
	generatedMatcapSphere    = regexp.MustCompile(`^sphere/matcap_sphere_[0-9]{3}\.png$`)
	generatedEmissiveSphere  = regexp.MustCompile(`^sphere/emissive_sphere_[0-9]{3}\.png$`)
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

		var sphereBytes []byte
		var err error
		switch sphereKind {
		case generatedSphereKindHair:
			sphereMetadata, hasMetadata := resolveGeneratedSphereMetadata(sphereMetadataMap, normalizedTextureName)
			if !hasMetadata {
				appendGeneratedSphereWarningID(modelData, warningid.VrmWarningSphereTextureSourceMissing)
				disableSphereMaterialsByTextureIndex(modelData, textureIndex)
				continue
			}
			sphereBytes, err = buildGeneratedHairSpherePng(trimmedTextureDir, modelData, sphereMetadata)
		default:
			sphereBytes, err = buildGeneratedSpherePng32(sphereKind)
		}
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

	hairSphereImage := image.NewRGBA(image.Rect(0, 0, sourceBounds.Dx(), sourceBounds.Dy()))
	for y := 0; y < sourceBounds.Dy(); y++ {
		for x := 0; x < sourceBounds.Dx(); x++ {
			resolvedColor := color.NRGBAModel.Convert(
				sourceImage.At(sourceBounds.Min.X+x, sourceBounds.Min.Y+y),
			).(color.NRGBA)
			hairSphereImage.SetRGBA(x, y, color.RGBA{
				R: resolvedColor.R,
				G: resolvedColor.G,
				B: resolvedColor.B,
				A: resolvedColor.A,
			})
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, hairSphereImage); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
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
