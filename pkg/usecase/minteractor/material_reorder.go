// 指示: miu200521358
package minteractor

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/ftrvxmtrx/tga"
	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/collection"
	"github.com/miu200521358/mlib_go/pkg/shared/base/logging"
	"golang.org/x/image/bmp"
	"golang.org/x/image/webp"
)

const (
	textureAlphaTransparentThreshold    = 0.05
	textureAlphaFallbackThreshold       = 0.995
	bodyWeightThreshold                 = 0.35
	fallbackOpaqueMaterialCount         = 3
	overlapPointScaleRatio              = 0.03
	overlapPointDistanceMin             = 0.01
	minimumOverlapSampleCount           = 4
	minimumOverlapCoverageRatio         = 0.05
	dynamicSampleScale                  = 2.3
	dynamicSampleBlockExponent          = 1.0 / 4.0
	minimumBodyPointSampleCount         = minimumOverlapSampleCount * 24
	minimumMaterialSampleCount          = minimumOverlapSampleCount * 6
	minimumOverlapPointSampleCount      = minimumOverlapSampleCount * 4
	minimumMaterialFaceSampleCount      = minimumOverlapSampleCount * 8
	minimumMaterialOrderDelta           = 0.001
	materialOrderScoreEpsilon           = 1e-6
	materialRelativeNearDelta           = 0.05
	materialTransparencyOrderDelta      = 0.005
	materialDepthSwitchDelta            = 0.085
	nonOverlapSwapMinimumDelta          = 0.5
	strongOverlapCoverageThreshold      = 0.50
	overlapAsymmetricCoverageGapMin     = 0.30
	overlapAsymmetricMinCoverageMax     = 0.50
	tinyDepthDeltaThreshold             = 0.02
	tinyDepthFarFirstCoverageThreshold  = 0.20
	exactTransparencyDeltaThreshold     = 1e-6
	veryLowCoverageTransparencyMax      = 0.10
	asymHighAlphaThreshold              = 0.90
	asymHighAlphaGapSwitchDelta         = 0.08
	balancedOverlapGapMax               = 0.10
	balancedOverlapTransparencyMinDelta = 0.05
	midCoverageDepthConfidencePenalty   = 1.0
	exactOrderDPMaxNodes                = 18
)

// materialFaceRange は材質ごとの面範囲を表す。
type materialFaceRange struct {
	start int
	count int
}

// materialSortMetric は並べ替え判定用の材質指標を表す。
type materialSortMetric struct {
	index int
	score float64
}

// textureAlphaCacheEntry はテクスチャアルファ判定のキャッシュを表す。
type textureAlphaCacheEntry struct {
	checked          bool
	transparent      bool
	transparentRatio float64
	failed           bool
}

// textureImageCacheEntry はテクスチャ画像読み込みキャッシュを表す。
type textureImageCacheEntry struct {
	checked bool
	img     image.Image
	bounds  image.Rectangle
	path    string
	format  string
}

// textureJudgeStats はテクスチャ判定の集計結果を表す。
type textureJudgeStats struct {
	checked   int
	succeeded int
	failed    int
}

// materialSpatialInfo は材質比較用の幾何情報を表す。
type materialSpatialInfo struct {
	points       []mmath.Vec3
	bodyDistance []float64
	minX         float64
	maxX         float64
	minY         float64
	maxY         float64
	minZ         float64
	maxZ         float64
}

// materialOrderConstraint は材質順序制約グラフの有向制約を表す。
type materialOrderConstraint struct {
	from       int
	to         int
	confidence float64
}

// indexedMaterialRename はindex指定の材質名変更情報を表す。
type indexedMaterialRename struct {
	Index   int
	NewName string
}

const (
	materialRenameTempPrefix   = "__mu_vrm2pmx_material_tmp_"
	materialNameInstanceSuffix = " (Instance)"
)

// abbreviateMaterialNamesBeforeReorder は材質並べ替え直前に材質名を略称へ正規化する。
func abbreviateMaterialNamesBeforeReorder(modelData *ModelData) error {
	if modelData == nil || modelData.Materials == nil {
		return nil
	}
	renames := collectMaterialAbbreviationRenames(modelData.Materials)
	if len(renames) == 0 {
		return nil
	}
	assignUniqueMaterialRenameNames(modelData.Materials, renames)
	return applyIndexedMaterialRenames(modelData.Materials, renames)
}

// collectMaterialAbbreviationRenames は材質略称化の変更候補を収集する。
func collectMaterialAbbreviationRenames(materials *collection.NamedCollection[*model.Material]) []indexedMaterialRename {
	if materials == nil {
		return []indexedMaterialRename{}
	}
	renames := make([]indexedMaterialRename, 0, materials.Len())
	for index := 0; index < materials.Len(); index++ {
		materialData, err := materials.Get(index)
		if err != nil || materialData == nil {
			continue
		}
		currentName := strings.TrimSpace(materialData.Name())
		abbreviatedName := abbreviateMaterialName(currentName)
		if abbreviatedName == "" {
			abbreviatedName = fmt.Sprintf("material_%d", index)
		}
		if currentName == abbreviatedName {
			continue
		}
		renames = append(renames, indexedMaterialRename{
			Index:   index,
			NewName: abbreviatedName,
		})
	}
	return renames
}

// abbreviateMaterialName は材質名を決定的に短縮正規化する。
func abbreviateMaterialName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	if normalized, changed := normalizeMaterialNameByPrefixAndSuffix(trimmed); changed {
		return normalized
	}
	if removedPrefix, ok := trimJSecPrefix(trimmed); ok {
		trimmed = removedPrefix
	}
	if isASCIIString(trimmed) {
		return abbreviateNameByNonAlphaNumericTokens(trimmed)
	}
	return abbreviateNameByUnderscoreTokens(trimmed)
}

// normalizeMaterialNameByPrefixAndSuffix はVRM材質名の接頭辞/接尾辞を除去する。
func normalizeMaterialNameByPrefixAndSuffix(name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", false
	}
	normalized := trimmed
	changed := false
	if strings.HasSuffix(normalized, materialNameInstanceSuffix) {
		normalized = strings.TrimSpace(strings.TrimSuffix(normalized, materialNameInstanceSuffix))
		changed = true
	}
	if removedPrefix, ok := trimVroidMaterialPrefix(normalized); ok {
		normalized = removedPrefix
		changed = true
	}
	return normalized, changed
}

// trimVroidMaterialPrefix はVRoid系材質名プレフィックスを除去する。
func trimVroidMaterialPrefix(name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", false
	}
	parts := strings.Split(trimmed, "_")
	if len(parts) < 3 {
		return "", false
	}
	if !isVroidMaterialPrefixHeadToken(parts[0]) {
		return "", false
	}
	nextIndex := 1
	for nextIndex < len(parts) && isASCIIOnlyDigits(parts[nextIndex]) {
		nextIndex++
	}
	if nextIndex <= 1 || nextIndex >= len(parts) {
		return "", false
	}
	removed := strings.TrimSpace(strings.Join(parts[nextIndex:], "_"))
	if removed == "" {
		return "", false
	}
	return removed, true
}

// isVroidMaterialPrefixHeadToken はVRoid材質プレフィックス先頭トークンかを判定する。
func isVroidMaterialPrefixHeadToken(token string) bool {
	if len(token) != 3 {
		return false
	}
	first := token[0]
	if !((first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')) {
		return false
	}
	return isASCIIOnlyDigits(token[1:])
}

// isASCIIOnlyDigits はASCII数字のみで構成されるかを判定する。
func isASCIIOnlyDigits(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isASCIIString はASCII文字のみで構成されるかを判定する。
func isASCIIString(value string) bool {
	for _, r := range value {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// abbreviateNameByNonAlphaNumericTokens は非英数字区切り名を短縮正規化する。
func abbreviateNameByNonAlphaNumericTokens(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	tokens := splitASCIIAlphaNumericTokens(trimmed)
	if len(tokens) == 0 {
		return abbreviateNameByUnderscoreTokens(trimmed)
	}
	type tokenPart struct {
		Text      string
		IsNumeric bool
	}
	shortParts := make([]tokenPart, 0, len(tokens))
	for _, token := range tokens {
		isNumeric := true
		for _, r := range token {
			if !unicode.IsDigit(r) {
				isNumeric = false
				break
			}
		}
		if isNumeric {
			shortParts = append(shortParts, tokenPart{
				Text:      token,
				IsNumeric: true,
			})
			continue
		}
		short := abbreviateModelSpecificToken(token)
		if short == "" {
			short = token
		}
		shortParts = append(shortParts, tokenPart{
			Text:      short,
			IsNumeric: false,
		})
	}
	if len(shortParts) == 0 {
		return abbreviateNameByUnderscoreTokens(trimmed)
	}
	builder := strings.Builder{}
	for i, part := range shortParts {
		if i > 0 && part.IsNumeric {
			builder.WriteString("_")
		}
		builder.WriteString(part.Text)
	}
	result := builder.String()
	if result == "" {
		return abbreviateNameByUnderscoreTokens(trimmed)
	}
	return result
}

// splitASCIIAlphaNumericTokens はASCII英数字トークン列を抽出する。
func splitASCIIAlphaNumericTokens(value string) []string {
	if value == "" {
		return []string{}
	}
	tokens := make([]string, 0, 8)
	builder := strings.Builder{}
	flush := func() {
		if builder.Len() == 0 {
			return
		}
		tokens = append(tokens, builder.String())
		builder.Reset()
	}
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

// abbreviateNameByUnderscoreTokens はアンダースコア区切り名を短縮正規化する。
func abbreviateNameByUnderscoreTokens(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "_")
	if len(parts) == 0 {
		return trimmed
	}
	type tokenPart struct {
		Text      string
		IsNumeric bool
	}
	shortParts := make([]tokenPart, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		isNumeric := true
		for _, r := range part {
			if !unicode.IsDigit(r) {
				isNumeric = false
				break
			}
		}
		if isNumeric {
			shortParts = append(shortParts, tokenPart{
				Text:      part,
				IsNumeric: true,
			})
			continue
		}
		short := abbreviateModelSpecificToken(part)
		if short == "" {
			short = part
		}
		shortParts = append(shortParts, tokenPart{
			Text:      short,
			IsNumeric: false,
		})
	}
	if len(shortParts) == 0 {
		return trimmed
	}
	builder := strings.Builder{}
	for i, part := range shortParts {
		if i > 0 && part.IsNumeric {
			builder.WriteString("_")
		}
		builder.WriteString(part.Text)
	}
	result := builder.String()
	if result == "" {
		return trimmed
	}
	return result
}

// abbreviateModelSpecificToken は材質/非標準名トークンを略称へ変換する。
func abbreviateModelSpecificToken(token string) string {
	if token == "" {
		return ""
	}
	if !isAsciiAlphaNumericToken(token) {
		return token
	}
	if isLikelyAbbreviatedToken(token) {
		return token
	}
	return abbreviateJSecToken(token)
}

// isAsciiAlphaNumericToken はASCII英数とアンダースコアのみで構成されるかを判定する。
func isAsciiAlphaNumericToken(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r > unicode.MaxASCII {
			return false
		}
		if (r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// isLikelyAbbreviatedToken は既に略称済みトークンかを推定する。
func isLikelyAbbreviatedToken(token string) bool {
	if token == "" {
		return false
	}
	upperCount := 0
	hasLowerVowel := false
	hasLowerConsonant := false
	for _, r := range token {
		if r >= 'A' && r <= 'Z' {
			upperCount++
			continue
		}
		if r >= 'a' && r <= 'z' {
			if isAsciiLowerVowel(r) {
				hasLowerVowel = true
			} else {
				hasLowerConsonant = true
			}
		}
	}
	if hasLowerVowel {
		return false
	}
	if upperCount >= 2 && hasLowerConsonant {
		return true
	}
	if upperCount == 1 && hasLowerConsonant && len(token) <= 4 {
		return true
	}
	if upperCount == 1 && !hasLowerConsonant && len(token) <= 3 {
		return true
	}
	return false
}

// isAsciiLowerVowel はASCII小文字母音かを判定する。
func isAsciiLowerVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	default:
		return false
	}
}

// assignUniqueMaterialRenameNames は候補名の重複を連番で解決する。
func assignUniqueMaterialRenameNames(
	materials *collection.NamedCollection[*model.Material],
	renames []indexedMaterialRename,
) {
	if materials == nil || len(renames) == 0 {
		return
	}
	targetIndexes := map[int]struct{}{}
	for _, rename := range renames {
		targetIndexes[rename.Index] = struct{}{}
	}
	usedNames := map[string]struct{}{}
	for index := 0; index < materials.Len(); index++ {
		if _, isRenameTarget := targetIndexes[index]; isRenameTarget {
			continue
		}
		materialData, err := materials.Get(index)
		if err != nil || materialData == nil {
			continue
		}
		usedNames[materialData.Name()] = struct{}{}
	}
	for i := range renames {
		base := strings.TrimSpace(renames[i].NewName)
		if base == "" {
			base = fmt.Sprintf("material_%d", renames[i].Index)
		}
		candidate := base
		serial := 2
		for {
			if _, exists := usedNames[candidate]; !exists {
				break
			}
			candidate = fmt.Sprintf("%s_%d", base, serial)
			serial++
		}
		renames[i].NewName = candidate
		usedNames[candidate] = struct{}{}
	}
}

// applyIndexedMaterialRenames はindex指定の材質名変更を安全に適用する。
func applyIndexedMaterialRenames(
	materials *collection.NamedCollection[*model.Material],
	renames []indexedMaterialRename,
) error {
	if materials == nil || len(renames) == 0 {
		return nil
	}
	tempSerial := 0
	applied := make([]indexedMaterialRename, 0, len(renames))
	for _, rename := range renames {
		materialData, err := materials.Get(rename.Index)
		if err != nil || materialData == nil {
			continue
		}
		if materialData.Name() == rename.NewName {
			continue
		}
		tempName := nextTemporaryMaterialName(materials, &tempSerial)
		if _, err := materials.Rename(rename.Index, tempName); err != nil {
			return err
		}
		applied = append(applied, rename)
	}
	for _, rename := range applied {
		if _, err := materials.Rename(rename.Index, rename.NewName); err != nil {
			return err
		}
		materialData, err := materials.Get(rename.Index)
		if err != nil || materialData == nil {
			continue
		}
		materialData.EnglishName = rename.NewName
	}
	return nil
}

// nextTemporaryMaterialName は重複しない一時材質名を生成する。
func nextTemporaryMaterialName(materials *collection.NamedCollection[*model.Material], serial *int) string {
	if serial == nil {
		return materialRenameTempPrefix + "000"
	}
	for {
		candidate := fmt.Sprintf("%s%03d", materialRenameTempPrefix, *serial)
		*serial = *serial + 1
		if _, err := materials.GetByName(candidate); err != nil {
			return candidate
		}
	}
}

// applyBodyDepthMaterialOrder は半透明材質をボディ近傍順へ並べ替える。
func applyBodyDepthMaterialOrder(modelData *ModelData) {
	applyBodyDepthMaterialOrderWithProgress(modelData, nil)
}

// applyBodyDepthMaterialOrderWithProgress は進捗通知付きで半透明材質をボディ近傍順へ並べ替える。
func applyBodyDepthMaterialOrderWithProgress(modelData *ModelData, progressReporter IPrepareProgressReporter) {
	if modelData == nil || modelData.Materials == nil || modelData.Faces == nil {
		logMaterialReorderViewerVerbose("材質並べ替えスキップ: モデル情報が不足しています")
		reportPrepareProgress(progressReporter, PrepareProgressEvent{
			Type: PrepareProgressEventTypeReorderCompleted,
		})
		return
	}
	logMaterialReorderViewerVerbose(
		"材質並べ替え開始: materials=%d faces=%d",
		modelData.Materials.Len(),
		modelData.Faces.Len(),
	)
	logMaterialReorderInfo(
		"材質並べ替え開始(Info): materials=%d faces=%d",
		modelData.Materials.Len(),
		modelData.Faces.Len(),
	)

	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil {
		logMaterialReorderViewerVerbose("材質並べ替えスキップ: 面範囲構築に失敗しました: %v", err)
		reportPrepareProgress(progressReporter, PrepareProgressEvent{
			Type: PrepareProgressEventTypeReorderCompleted,
		})
		return
	}
	if len(faceRanges) < 2 {
		logMaterialReorderViewerVerbose("材質並べ替えスキップ: 面範囲が不足しています count=%d", len(faceRanges))
		reportPrepareProgress(progressReporter, PrepareProgressEvent{
			Type: PrepareProgressEventTypeReorderCompleted,
		})
		return
	}

	textureAlphaThreshold := textureAlphaTransparentThreshold
	textureImageCache := map[int]textureImageCacheEntry{}
	logMaterialReorderInfo(
		"材質並べ替え: UV画像取得開始 materials=%d threshold=%.3f",
		modelData.Materials.Len(),
		textureAlphaThreshold,
	)
	materialUvTransparencyScores := buildMaterialTransparencyScores(
		modelData,
		faceRanges,
		textureImageCache,
		textureAlphaThreshold,
	)
	transparentMaterialIndexes := collectTransparentMaterialIndexesFromScores(
		modelData,
		materialUvTransparencyScores,
	)
	reportPrepareProgress(progressReporter, PrepareProgressEvent{
		Type: PrepareProgressEventTypeReorderUvScanned,
	})
	logMaterialReorderViewerVerbose(
		"材質並べ替え: テクスチャ判定開始 materials=%d threshold=%.3f",
		modelData.Materials.Len(),
		textureAlphaThreshold,
	)
	materialTransparencyScores, textureStats := buildTextureTransparencyScores(modelData, textureAlphaThreshold)
	reportPrepareProgress(progressReporter, PrepareProgressEvent{
		Type:         PrepareProgressEventTypeReorderTextureScanned,
		TextureCount: textureStats.checked,
	})
	logMaterialReorderInfo(
		"材質並べ替え: テクスチャ判定完了 textures=%d succeeded=%d failed=%d threshold=%.3f",
		textureStats.checked,
		textureStats.succeeded,
		textureStats.failed,
		textureAlphaThreshold,
	)
	if len(transparentMaterialIndexes) < 2 {
		logMaterialReorderInfo(
			"材質並べ替え: UV画像取得開始 materials=%d threshold=%.3f",
			modelData.Materials.Len(),
			textureAlphaFallbackThreshold,
		)
		fallbackMaterialUvTransparencyScores := buildMaterialTransparencyScores(
			modelData,
			faceRanges,
			textureImageCache,
			textureAlphaFallbackThreshold,
		)
		fallbackTransparentMaterialIndexes := collectTransparentMaterialIndexesFromScores(
			modelData,
			fallbackMaterialUvTransparencyScores,
		)
		if len(fallbackTransparentMaterialIndexes) >= 2 {
			textureAlphaThreshold = textureAlphaFallbackThreshold
			materialUvTransparencyScores = fallbackMaterialUvTransparencyScores
			transparentMaterialIndexes = fallbackTransparentMaterialIndexes
			reportPrepareProgress(progressReporter, PrepareProgressEvent{
				Type: PrepareProgressEventTypeReorderUvScanned,
			})
			logMaterialReorderViewerVerbose(
				"材質並べ替え: テクスチャ判定開始 materials=%d threshold=%.3f",
				modelData.Materials.Len(),
				textureAlphaThreshold,
			)
			materialTransparencyScores, textureStats = buildTextureTransparencyScores(modelData, textureAlphaThreshold)
			reportPrepareProgress(progressReporter, PrepareProgressEvent{
				Type:         PrepareProgressEventTypeReorderTextureScanned,
				TextureCount: textureStats.checked,
			})
			logMaterialReorderInfo(
				"材質並べ替え: テクスチャ判定完了 textures=%d succeeded=%d failed=%d threshold=%.3f",
				textureStats.checked,
				textureStats.succeeded,
				textureStats.failed,
				textureAlphaThreshold,
			)
			logMaterialReorderViewerVerbose(
				"材質並べ替え: 半透明候補の再判定を適用 threshold<=%.3f count=%d",
				textureAlphaThreshold,
				len(transparentMaterialIndexes),
			)
		}
	}
	if len(transparentMaterialIndexes) < 2 {
		fallbackTransparentMaterialIndexes := collectDoubleSidedTextureMaterialIndexes(modelData)
		if len(fallbackTransparentMaterialIndexes) >= 2 {
			transparentMaterialIndexes = fallbackTransparentMaterialIndexes
			logMaterialReorderViewerVerbose(
				"材質並べ替え: 半透明候補不足のため両面描画材質を代替採用 count=%d",
				len(transparentMaterialIndexes),
			)
			for _, materialIndex := range transparentMaterialIndexes {
				if _, ok := materialTransparencyScores[materialIndex]; !ok {
					materialTransparencyScores[materialIndex] = 0
				}
			}
		}
	}
	logMaterialReorderViewerVerbose(
		"材質並べ替え: 半透明候補=%d [%s]",
		len(transparentMaterialIndexes),
		formatMaterialIndexesForViewerLog(modelData, transparentMaterialIndexes),
	)
	logMaterialReorderInfo(
		"材質並べ替え: UV透明率取得完了 materials=%d transparentCandidates=%d threshold=%.3f",
		modelData.Materials.Len(),
		len(transparentMaterialIndexes),
		textureAlphaThreshold,
	)
	bodyBoneIndexes := collectBodyBoneIndexesFromHumanoid(modelData)
	bodyMaterialIndex := detectBodyMaterialIndex(modelData, bodyBoneIndexes)
	logMaterialReorderViewerVerbose(
		"材質並べ替え: bodyBoneIndexes=%d bodyMaterial=%s",
		len(bodyBoneIndexes),
		formatMaterialLabelForViewerLog(modelData, bodyMaterialIndex),
	)
	if bodyMaterialIndex >= 0 {
		filteredTransparentMaterialIndexes := make([]int, 0, len(transparentMaterialIndexes))
		for _, materialIndex := range transparentMaterialIndexes {
			if materialIndex == bodyMaterialIndex {
				continue
			}
			filteredTransparentMaterialIndexes = append(filteredTransparentMaterialIndexes, materialIndex)
		}
		transparentMaterialIndexes = filteredTransparentMaterialIndexes
		logMaterialReorderViewerVerbose(
			"材質並べ替え: ボディ材質を除外後=%d [%s]",
			len(transparentMaterialIndexes),
			formatMaterialIndexesForViewerLog(modelData, transparentMaterialIndexes),
		)
	}
	if len(transparentMaterialIndexes) < 2 {
		logMaterialReorderViewerVerbose("材質並べ替えスキップ: 半透明材質が2件未満です count=%d", len(transparentMaterialIndexes))
		reportPrepareProgress(progressReporter, PrepareProgressEvent{
			Type:       PrepareProgressEventTypeReorderBlocksPlanned,
			PairCount:  0,
			BlockCount: 0,
		})
		logMaterialReorderInfo(
			"材質並べ替え完了: changed=%t transparent=%d blocks=%d",
			false,
			len(transparentMaterialIndexes),
			0,
		)
		reportPrepareProgress(progressReporter, PrepareProgressEvent{
			Type: PrepareProgressEventTypeReorderCompleted,
		})
		return
	}

	newOrder := make([]int, modelData.Materials.Len())
	for i := range newOrder {
		newOrder[i] = i
	}
	transparentMaterialIndexSet := map[int]struct{}{}
	for _, materialIndex := range transparentMaterialIndexes {
		transparentMaterialIndexSet[materialIndex] = struct{}{}
	}
	transparentBlocks := splitContinuousMaterialIndexBlocks(transparentMaterialIndexes)
	targetBlockCount := countProcessableMaterialBlocks(transparentBlocks)
	targetPairCount := countProcessableMaterialPairs(transparentBlocks)
	reportPrepareProgress(progressReporter, PrepareProgressEvent{
		Type:       PrepareProgressEventTypeReorderBlocksPlanned,
		PairCount:  targetPairCount,
		BlockCount: targetBlockCount,
	})
	logMaterialReorderViewerVerbose("材質並べ替え: 連続ブロック数=%d", len(transparentBlocks))
	transparentSampleBlockSize := len(transparentMaterialIndexes)
	if transparentSampleBlockSize < 1 {
		transparentSampleBlockSize = 1
	}
	for _, block := range transparentBlocks {
		if len(block) < 2 {
			logMaterialReorderViewerVerbose("材質並べ替え: ブロックスキップ size=%d block=[%s]", len(block), formatMaterialIndexesForViewerLog(modelData, block))
			continue
		}
		blockPairCount := materialBlockPairCount(block)
		bodyPoints := collectBodyPointsForSorting(
			modelData,
			faceRanges,
			transparentMaterialIndexSet,
			transparentSampleBlockSize,
		)
		if len(bodyPoints) == 0 {
			logMaterialReorderViewerVerbose("材質並べ替え: ボディ点が取得できないためスキップ block=[%s]", formatMaterialIndexesForViewerLog(modelData, block))
			reportPrepareProgress(progressReporter, PrepareProgressEvent{
				Type:       PrepareProgressEventTypeReorderBlockProcessed,
				PairCount:  blockPairCount,
				BlockCount: 1,
			})
			continue
		}
		logMaterialReorderViewerVerbose(
			"材質並べ替え: ブロック評価開始 block=[%s] bodyPoints=%d sampleBlock=%d",
			formatMaterialIndexesForViewerLog(modelData, block),
			len(bodyPoints),
			transparentSampleBlockSize,
		)
		sortedBlock := sortTransparentMaterialsByOverlapDepth(
			modelData,
			faceRanges,
			block,
			bodyPoints,
			materialTransparencyScores,
			transparentSampleBlockSize,
		)
		if len(sortedBlock) != len(block) {
			logMaterialReorderViewerVerbose(
				"材質並べ替え: ソート結果サイズ不一致でスキップ block=%d sorted=%d",
				len(block),
				len(sortedBlock),
			)
			reportPrepareProgress(progressReporter, PrepareProgressEvent{
				Type:       PrepareProgressEventTypeReorderBlockProcessed,
				PairCount:  blockPairCount,
				BlockCount: 1,
			})
			continue
		}
		logMaterialReorderViewerVerbose(
			"材質並べ替え: ブロック並べ替え [%s] -> [%s]",
			formatMaterialIndexesForViewerLog(modelData, block),
			formatMaterialIndexesForViewerLog(modelData, sortedBlock),
		)
		logMaterialReorderViewerVerbose(
			"材質並べ替え: 制約解決完了 block=[%s] changed=%t",
			formatMaterialIndexesForViewerLog(modelData, block),
			!areEqualMaterialOrders(block, sortedBlock),
		)
		for i, position := range block {
			newOrder[position] = sortedBlock[i]
		}
		reportPrepareProgress(progressReporter, PrepareProgressEvent{
			Type:       PrepareProgressEventTypeReorderBlockProcessed,
			PairCount:  blockPairCount,
			BlockCount: 1,
		})
	}
	if isIdentityOrder(newOrder) {
		logMaterialReorderViewerVerbose("材質並べ替えスキップ: 並び順の変更なし")
		logMaterialReorderInfo(
			"材質並べ替え完了: changed=%t transparent=%d blocks=%d",
			false,
			len(transparentMaterialIndexes),
			len(transparentBlocks),
		)
		reportPrepareProgress(progressReporter, PrepareProgressEvent{
			Type: PrepareProgressEventTypeReorderCompleted,
		})
		return
	}

	beforeOrder := formatMaterialIndexesForViewerLog(modelData, newOrder)
	if err := rebuildMaterialAndFaceOrder(modelData, faceRanges, newOrder); err != nil {
		logMaterialReorderViewerVerbose("材質並べ替え失敗: 再構築に失敗しました: %v", err)
		reportPrepareProgress(progressReporter, PrepareProgressEvent{
			Type: PrepareProgressEventTypeReorderCompleted,
		})
		return
	}
	logMaterialReorderViewerVerbose(
		"材質並べ替え完了: order=[%s]",
		beforeOrder,
	)
	logMaterialReorderInfo(
		"材質並べ替え完了: changed=%t transparent=%d blocks=%d",
		true,
		len(transparentMaterialIndexes),
		len(transparentBlocks),
	)
	reportPrepareProgress(progressReporter, PrepareProgressEvent{
		Type: PrepareProgressEventTypeReorderCompleted,
	})
}

// logMaterialReorderViewerVerbose は材質並べ替え専用のデバッグ/ビューワー冗長ログを出力する。
func logMaterialReorderViewerVerbose(format string, params ...any) {
	logger := logging.DefaultLogger()
	if logger == nil {
		return
	}
	logger.Debug(format, params...)
	if logger.IsVerboseEnabled(logging.VERBOSE_INDEX_VIEWER) {
		logger.Verbose(logging.VERBOSE_INDEX_VIEWER, "[DEBUG] "+format, params...)
	}
}

// logMaterialReorderInfo は材質並べ替えのINFOログを出力し、viewer冗長ログにも転送する。
func logMaterialReorderInfo(format string, params ...any) {
	logger := logging.DefaultLogger()
	if logger == nil {
		return
	}
	logger.Info(format, params...)
	if logger.IsVerboseEnabled(logging.VERBOSE_INDEX_VIEWER) {
		logger.Verbose(logging.VERBOSE_INDEX_VIEWER, "[INFO] "+format, params...)
	}
}

// formatMaterialLabelForViewerLog は材質インデックスを冗長ログ向けに整形する。
func formatMaterialLabelForViewerLog(modelData *ModelData, materialIndex int) string {
	if materialIndex < 0 {
		return "none"
	}
	return fmt.Sprintf("%d:%s", materialIndex, resolveMaterialNameForViewerLog(modelData, materialIndex))
}

// formatMaterialIndexesForViewerLog は材質インデックス配列を冗長ログ向けに整形する。
func formatMaterialIndexesForViewerLog(modelData *ModelData, materialIndexes []int) string {
	if len(materialIndexes) == 0 {
		return ""
	}
	labels := make([]string, 0, len(materialIndexes))
	for _, materialIndex := range materialIndexes {
		labels = append(labels, formatMaterialLabelForViewerLog(modelData, materialIndex))
	}
	return strings.Join(labels, ", ")
}

// resolveMaterialNameForViewerLog は材質名の表示文字列を返す。
func resolveMaterialNameForViewerLog(modelData *ModelData, materialIndex int) string {
	if modelData == nil || modelData.Materials == nil || materialIndex < 0 || materialIndex >= modelData.Materials.Len() {
		return "unknown"
	}
	materialData, err := modelData.Materials.Get(materialIndex)
	if err != nil || materialData == nil {
		return "nil"
	}
	name := strings.TrimSpace(materialData.Name())
	if name != "" {
		return name
	}
	return fmt.Sprintf("index_%d", materialIndex)
}

// splitContinuousMaterialIndexBlocks は連続する材質indexのブロックへ分割する。
func splitContinuousMaterialIndexBlocks(materialIndexes []int) [][]int {
	if len(materialIndexes) == 0 {
		return [][]int{}
	}
	blocks := make([][]int, 0)
	current := []int{materialIndexes[0]}
	for i := 1; i < len(materialIndexes); i++ {
		if materialIndexes[i] == materialIndexes[i-1]+1 {
			current = append(current, materialIndexes[i])
			continue
		}
		blocks = append(blocks, current)
		current = []int{materialIndexes[i]}
	}
	blocks = append(blocks, current)
	return blocks
}

// countProcessableMaterialBlocks は並べ替え対象ブロック件数を返す。
func countProcessableMaterialBlocks(blocks [][]int) int {
	count := 0
	for _, block := range blocks {
		if len(block) < 2 {
			continue
		}
		count++
	}
	return count
}

// countProcessableMaterialPairs は並べ替え対象ブロックの総ペア数を返す。
func countProcessableMaterialPairs(blocks [][]int) int {
	total := 0
	for _, block := range blocks {
		total += materialBlockPairCount(block)
	}
	return total
}

// materialBlockPairCount はブロック内の総ペア数を返す。
func materialBlockPairCount(block []int) int {
	if len(block) < 2 {
		return 0
	}
	return len(block) * (len(block) - 1) / 2
}

// areEqualMaterialOrders は材質index配列が同一か判定する。
func areEqualMaterialOrders(left []int, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// collectTransparentMaterialIndexesFromScores は透明スコアから半透明材質indexを抽出する。
func collectTransparentMaterialIndexesFromScores(
	modelData *ModelData,
	materialTransparencyScores map[int]float64,
) []int {
	transparentMaterialIndexes := make([]int, 0)
	if modelData == nil || modelData.Materials == nil {
		return transparentMaterialIndexes
	}
	for materialIndex := range modelData.Materials.Values() {
		score := materialTransparencyScores[materialIndex]
		if score <= 0 {
			continue
		}
		transparentMaterialIndexes = append(transparentMaterialIndexes, materialIndex)
	}
	return transparentMaterialIndexes
}

// collectDoubleSidedTextureMaterialIndexes は両面描画かつテクスチャ参照ありの材質indexを返す。
func collectDoubleSidedTextureMaterialIndexes(modelData *ModelData) []int {
	transparentMaterialIndexes := make([]int, 0)
	if modelData == nil || modelData.Materials == nil {
		return transparentMaterialIndexes
	}
	for materialIndex, materialData := range modelData.Materials.Values() {
		if materialData == nil {
			continue
		}
		if materialData.TextureIndex < 0 {
			continue
		}
		if materialData.DrawFlag&model.DRAW_FLAG_DOUBLE_SIDED_DRAWING == 0 {
			continue
		}
		transparentMaterialIndexes = append(transparentMaterialIndexes, materialIndex)
	}
	return transparentMaterialIndexes
}

// buildMaterialTransparencyScores は材質ごとの透明画素率スコアを返す。
func buildMaterialTransparencyScores(
	modelData *ModelData,
	faceRanges []materialFaceRange,
	textureImageCache map[int]textureImageCacheEntry,
	textureAlphaThreshold float64,
) map[int]float64 {
	scores := make(map[int]float64)
	if modelData == nil || modelData.Materials == nil || len(faceRanges) != modelData.Materials.Len() {
		return scores
	}
	for materialIndex := range modelData.Materials.Values() {
		scores[materialIndex] = calculateMaterialUVTransparencyRatio(
			modelData,
			faceRanges,
			materialIndex,
			textureImageCache,
			textureAlphaThreshold,
		)
	}
	return scores
}

// buildTextureTransparencyScores は材質ごとのテクスチャ全体透明率スコアを返す。
func buildTextureTransparencyScores(
	modelData *ModelData,
	textureAlphaThreshold float64,
) (map[int]float64, textureJudgeStats) {
	scores := make(map[int]float64)
	stats := textureJudgeStats{}
	if modelData == nil || modelData.Materials == nil {
		return scores, stats
	}
	textureAlphaCache := map[int]textureAlphaCacheEntry{}
	for materialIndex, materialData := range modelData.Materials.Values() {
		if materialData == nil {
			continue
		}
		score := 0.0
		if hasTransparentTextureAlphaWithThreshold(
			modelData,
			materialData.TextureIndex,
			textureAlphaCache,
			textureAlphaThreshold,
		) {
			score = textureAlphaCache[materialData.TextureIndex].transparentRatio
		}
		scores[materialIndex] = score
	}
	for _, entry := range textureAlphaCache {
		if !entry.checked {
			continue
		}
		stats.checked++
		if entry.failed {
			stats.failed++
			continue
		}
		stats.succeeded++
	}
	return scores, stats
}

// calculateMaterialUVTransparencyRatio は材質が参照するUV面サンプルの透明率を返す。
func calculateMaterialUVTransparencyRatio(
	modelData *ModelData,
	faceRanges []materialFaceRange,
	materialIndex int,
	textureImageCache map[int]textureImageCacheEntry,
	textureAlphaThreshold float64,
) float64 {
	if modelData == nil || modelData.Materials == nil || modelData.Faces == nil || modelData.Vertices == nil {
		return 0
	}
	if materialIndex < 0 || materialIndex >= modelData.Materials.Len() || materialIndex >= len(faceRanges) {
		return 0
	}

	materialData, err := modelData.Materials.Get(materialIndex)
	if err != nil || materialData == nil || materialData.TextureIndex < 0 {
		return 0
	}
	textureEntry, ok := resolveTextureImageCacheEntry(modelData, materialData.TextureIndex, textureImageCache)
	if !ok {
		return 0
	}

	faceRange := faceRanges[materialIndex]
	if faceRange.count <= 0 {
		return 0
	}

	sampleFaceLimit := resolveDynamicSampleLimit(faceRange.count, 1, minimumMaterialFaceSampleCount)
	if sampleFaceLimit <= 0 {
		sampleFaceLimit = 1
	}
	step := 1
	if faceRange.count > sampleFaceLimit {
		step = faceRange.count/sampleFaceLimit + 1
	}

	totalSamples := 0
	transparentSamples := 0
	for i := 0; i < faceRange.count; i += step {
		face, faceErr := modelData.Faces.Get(faceRange.start + i)
		if faceErr != nil || face == nil {
			continue
		}
		uvSamples, sampleOK := collectFaceUvSamplePoints(modelData, face)
		if !sampleOK {
			continue
		}
		for _, uv := range uvSamples {
			alpha, alphaOK := sampleTextureAlphaAtUV(textureEntry, uv)
			if !alphaOK {
				continue
			}
			totalSamples++
			if alpha <= textureAlphaThreshold {
				transparentSamples++
			}
		}
	}
	if totalSamples == 0 {
		return 0
	}
	ratio := float64(transparentSamples) / float64(totalSamples)
	logMaterialReorderViewerVerbose(
		"材質並べ替え: UV透明率 index=%d threshold=%.3f ratio=%.6f samples=%d",
		materialIndex,
		textureAlphaThreshold,
		ratio,
		totalSamples,
	)
	return ratio
}

// collectFaceUvSamplePoints は面のUVサンプル点を返す。
func collectFaceUvSamplePoints(modelData *ModelData, face *model.Face) ([]mmath.Vec2, bool) {
	if modelData == nil || modelData.Vertices == nil || face == nil {
		return nil, false
	}
	v0, err0 := modelData.Vertices.Get(face.VertexIndexes[0])
	v1, err1 := modelData.Vertices.Get(face.VertexIndexes[1])
	v2, err2 := modelData.Vertices.Get(face.VertexIndexes[2])
	if err0 != nil || err1 != nil || err2 != nil || v0 == nil || v1 == nil || v2 == nil {
		return nil, false
	}
	center := mmath.Vec2{
		X: (v0.Uv.X + v1.Uv.X + v2.Uv.X) / 3.0,
		Y: (v0.Uv.Y + v1.Uv.Y + v2.Uv.Y) / 3.0,
	}
	return []mmath.Vec2{v0.Uv, v1.Uv, v2.Uv, center}, true
}

// resolveTextureImageCacheEntry はテクスチャ画像キャッシュを解決する。
func resolveTextureImageCacheEntry(
	modelData *ModelData,
	textureIndex int,
	textureImageCache map[int]textureImageCacheEntry,
) (textureImageCacheEntry, bool) {
	if textureIndex < 0 || modelData == nil || modelData.Textures == nil {
		return textureImageCacheEntry{}, false
	}
	if cached, ok := textureImageCache[textureIndex]; ok {
		return cached, cached.checked && cached.img != nil && !cached.bounds.Empty()
	}

	textureData, err := modelData.Textures.Get(textureIndex)
	if err != nil || textureData == nil || !textureData.IsValid() {
		logMaterialReorderViewerVerbose(
			"材質並べ替え: UV画像取得スキップ index=%d reason=invalidTexture err=%v",
			textureIndex,
			err,
		)
		entry := textureImageCacheEntry{checked: true}
		textureImageCache[textureIndex] = entry
		return entry, false
	}
	modelPath := strings.TrimSpace(modelData.Path())
	textureName := strings.TrimSpace(textureData.Name())
	if modelPath == "" || textureName == "" {
		logMaterialReorderViewerVerbose(
			"材質並べ替え: UV画像取得スキップ index=%d reason=pathOrNameEmpty modelPath=%q texture=%q",
			textureIndex,
			modelPath,
			textureName,
		)
		entry := textureImageCacheEntry{checked: true}
		textureImageCache[textureIndex] = entry
		return entry, false
	}

	texturePath := filepath.Join(filepath.Dir(modelPath), normalizeTextureRelativePath(textureName))
	img, decodeFormat, decodeErr := decodeTextureImageFile(texturePath)
	if decodeErr != nil {
		logMaterialReorderViewerVerbose(
			"材質並べ替え: UV画像デコード失敗 index=%d path=%q format=%q err=%v",
			textureIndex,
			texturePath,
			decodeFormat,
			decodeErr,
		)
		entry := textureImageCacheEntry{checked: true, path: texturePath, format: decodeFormat}
		textureImageCache[textureIndex] = entry
		return entry, false
	}
	entry := textureImageCacheEntry{
		checked: true,
		img:     img,
		bounds:  img.Bounds(),
		path:    texturePath,
		format:  decodeFormat,
	}
	textureImageCache[textureIndex] = entry
	logMaterialReorderViewerVerbose(
		"材質並べ替え: UV画像取得 index=%d path=%q format=%q size=%dx%d",
		textureIndex,
		texturePath,
		decodeFormat,
		entry.bounds.Dx(),
		entry.bounds.Dy(),
	)
	return entry, !entry.bounds.Empty()
}

// sampleTextureAlphaAtUV はUV座標に対応するテクスチャアルファを返す。
func sampleTextureAlphaAtUV(textureEntry textureImageCacheEntry, uv mmath.Vec2) (float64, bool) {
	if !textureEntry.checked || textureEntry.img == nil || textureEntry.bounds.Empty() {
		return 0, false
	}
	width := textureEntry.bounds.Dx()
	height := textureEntry.bounds.Dy()
	if width <= 0 || height <= 0 {
		return 0, false
	}
	u := clampUv(uv.X)
	v := clampUv(uv.Y)
	x := int(math.Round(u * float64(width-1)))
	y := int(math.Round((1.0 - v) * float64(height-1)))
	if x < 0 {
		x = 0
	} else if x >= width {
		x = width - 1
	}
	if y < 0 {
		y = 0
	} else if y >= height {
		y = height - 1
	}
	return extractAlpha(textureEntry.img.At(textureEntry.bounds.Min.X+x, textureEntry.bounds.Min.Y+y)), true
}

// clampUv はUV座標を0-1へ丸める。
func clampUv(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// sortTransparentMaterialsByOverlapDepth は重なり領域のボディ近傍度から透明材質順を決定する。
func sortTransparentMaterialsByOverlapDepth(
	modelData *ModelData,
	faceRanges []materialFaceRange,
	transparentMaterialIndexes []int,
	bodyPoints []mmath.Vec3,
	materialTransparencyScores map[int]float64,
	sampleBlockSize int,
) []int {
	if len(transparentMaterialIndexes) < 2 {
		return append([]int(nil), transparentMaterialIndexes...)
	}

	// 元順序を起点に、重なり判定で前後が確定できる材質ペアから順序制約を組み立てる。
	sortedMaterialIndexes := append([]int(nil), transparentMaterialIndexes...)
	blockSize := sampleBlockSize
	if blockSize < 1 {
		blockSize = len(sortedMaterialIndexes)
	}
	bodyProximityScores := make(map[int]float64, len(sortedMaterialIndexes))
	for _, materialIndex := range sortedMaterialIndexes {
		score, ok := calculateBodyProximityScore(modelData, faceRanges[materialIndex], bodyPoints, blockSize)
		if !ok {
			score = math.MaxFloat64
		}
		bodyProximityScores[materialIndex] = score
		logMaterialReorderViewerVerbose(
			"材質並べ替え: 指標 material=%s bodyProximity=%.6f transparency=%.6f",
			formatMaterialLabelForViewerLog(modelData, materialIndex),
			score,
			materialTransparencyScores[materialIndex],
		)
	}

	spatialInfoMap := collectMaterialSpatialInfos(
		modelData,
		faceRanges,
		transparentMaterialIndexes,
		bodyPoints,
		blockSize,
	)
	modelScale := estimatePointCloudScale(bodyPoints)
	if modelScale <= 0 {
		modelScale = 1.0
	}
	overlapThreshold := math.Max(modelScale*overlapPointScaleRatio, overlapPointDistanceMin)

	nodeCount := len(sortedMaterialIndexes)
	nodeByMaterialIndex := make(map[int]int, nodeCount)
	nodePriorities := make([]int, nodeCount)
	for nodeIndex, materialIndex := range sortedMaterialIndexes {
		nodeByMaterialIndex[materialIndex] = nodeIndex
		nodePriorities[nodeIndex] = nodeIndex
	}
	constraints := make([]materialOrderConstraint, 0, nodeCount*2)
	constraintIndexByEdge := make(map[[2]int]int)
	pairResolvedCount := 0

	for i := 0; i < nodeCount-1; i++ {
		leftMaterialIndex := sortedMaterialIndexes[i]
		for j := i + 1; j < nodeCount; j++ {
			rightMaterialIndex := sortedMaterialIndexes[j]
			leftBeforeRight, confidence, valid := resolvePairOrderConstraint(
				leftMaterialIndex,
				rightMaterialIndex,
				spatialInfoMap,
				overlapThreshold,
				materialTransparencyScores,
				bodyProximityScores,
			)
			if !valid {
				continue
			}
			pairResolvedCount++
			beforeMaterialIndex := leftMaterialIndex
			afterMaterialIndex := rightMaterialIndex
			if !leftBeforeRight {
				beforeMaterialIndex = rightMaterialIndex
				afterMaterialIndex = leftMaterialIndex
			}
			logMaterialReorderViewerVerbose(
				"材質並べ替え: ペア判定 left=%s right=%s decided=%s->%s conf=%.6f prox=(%.6f,%.6f) transparency=(%.6f,%.6f)",
				formatMaterialLabelForViewerLog(modelData, leftMaterialIndex),
				formatMaterialLabelForViewerLog(modelData, rightMaterialIndex),
				formatMaterialLabelForViewerLog(modelData, beforeMaterialIndex),
				formatMaterialLabelForViewerLog(modelData, afterMaterialIndex),
				confidence,
				bodyProximityScores[leftMaterialIndex],
				bodyProximityScores[rightMaterialIndex],
				materialTransparencyScores[leftMaterialIndex],
				materialTransparencyScores[rightMaterialIndex],
			)
			beforeNode := nodeByMaterialIndex[beforeMaterialIndex]
			afterNode := nodeByMaterialIndex[afterMaterialIndex]
			edge := [2]int{beforeNode, afterNode}
			if currentIndex, exists := constraintIndexByEdge[edge]; exists {
				if confidence > constraints[currentIndex].confidence {
					logMaterialReorderViewerVerbose(
						"材質並べ替え: 制約更新 from=%s to=%s old=%.6f new=%.6f",
						formatMaterialLabelForViewerLog(modelData, sortedMaterialIndexes[constraints[currentIndex].from]),
						formatMaterialLabelForViewerLog(modelData, sortedMaterialIndexes[constraints[currentIndex].to]),
						constraints[currentIndex].confidence,
						confidence,
					)
					constraints[currentIndex].confidence = confidence
				}
				continue
			}
			constraintIndexByEdge[edge] = len(constraints)
			constraints = append(constraints, materialOrderConstraint{
				from:       beforeNode,
				to:         afterNode,
				confidence: confidence,
			})
		}
	}
	logMaterialReorderViewerVerbose(
		"材質並べ替え: ペア判定解決 block=[%s] pairs=%d constraints=%d",
		formatMaterialIndexesForViewerLog(modelData, transparentMaterialIndexes),
		pairResolvedCount,
		len(constraints),
	)
	logMaterialReorderViewerVerbose("材質並べ替え: 制約数=%d", len(constraints))
	for _, constraint := range constraints {
		logMaterialReorderViewerVerbose(
			"材質並べ替え: 制約 from=%s to=%s conf=%.6f",
			formatMaterialLabelForViewerLog(modelData, sortedMaterialIndexes[constraint.from]),
			formatMaterialLabelForViewerLog(modelData, sortedMaterialIndexes[constraint.to]),
			constraint.confidence,
		)
	}

	if len(constraints) == 0 {
		if nodeCount == 2 {
			left := sortedMaterialIndexes[0]
			right := sortedMaterialIndexes[1]
			if bodyProximityScores[left]-bodyProximityScores[right] > nonOverlapSwapMinimumDelta {
				logMaterialReorderViewerVerbose(
					"材質並べ替え: 制約なし2材質フォールバック swap %s <-> %s",
					formatMaterialLabelForViewerLog(modelData, left),
					formatMaterialLabelForViewerLog(modelData, right),
				)
				return []int{right, left}
			}
		}
		logMaterialReorderViewerVerbose("材質並べ替え: 制約なしのため元順を維持")
		return sortedMaterialIndexes
	}

	sortedNodes := resolveMaterialOrderNodes(nodeCount, constraints, nodePriorities)
	if len(sortedNodes) != nodeCount {
		logMaterialReorderViewerVerbose(
			"材質並べ替え: ノード解決失敗 nodeCount=%d resolved=%d",
			nodeCount,
			len(sortedNodes),
		)
		return sortedMaterialIndexes
	}
	result := make([]int, 0, nodeCount)
	for _, nodeIndex := range sortedNodes {
		result = append(result, sortedMaterialIndexes[nodeIndex])
	}
	logMaterialReorderViewerVerbose(
		"材質並べ替え: ブロック解決順 [%s]",
		formatMaterialIndexesForViewerLog(modelData, result),
	)
	return result
}

// resolvePairOrderConstraint は前後両方向の判定を突き合わせて材質ペア順序を決定する。
func resolvePairOrderConstraint(
	leftMaterialIndex int,
	rightMaterialIndex int,
	spatialInfoMap map[int]materialSpatialInfo,
	overlapThreshold float64,
	materialTransparencyScores map[int]float64,
	bodyProximityScores map[int]float64,
) (bool, float64, bool) {
	forwardBefore, forwardConfidence, forwardValid := resolvePairOrderByOverlap(
		leftMaterialIndex,
		rightMaterialIndex,
		spatialInfoMap,
		overlapThreshold,
		materialTransparencyScores,
	)
	reverseBefore, reverseConfidence, reverseValid := resolvePairOrderByOverlap(
		rightMaterialIndex,
		leftMaterialIndex,
		spatialInfoMap,
		overlapThreshold,
		materialTransparencyScores,
	)
	mergedBefore, mergedConfidence, mergedValid := mergeDirectionalPairDecisions(
		forwardBefore,
		forwardConfidence,
		forwardValid,
		reverseBefore,
		reverseConfidence,
		reverseValid,
	)
	if mergedValid {
		return mergedBefore, mergedConfidence, true
	}
	return false, 0, false
}

// mergeDirectionalPairDecisions は順方向/逆方向の判定結果を順方向基準へ統合する。
func mergeDirectionalPairDecisions(
	forwardBefore bool,
	forwardConfidence float64,
	forwardValid bool,
	reverseBefore bool,
	reverseConfidence float64,
	reverseValid bool,
) (bool, float64, bool) {
	if !forwardValid && !reverseValid {
		return false, 0, false
	}

	reverseForwardBefore := !reverseBefore
	if forwardValid && reverseValid {
		if forwardBefore == reverseForwardBefore {
			if reverseConfidence > forwardConfidence {
				return reverseForwardBefore, reverseConfidence, true
			}
			return forwardBefore, forwardConfidence, true
		}
		if forwardConfidence > reverseConfidence+materialOrderScoreEpsilon {
			return forwardBefore, forwardConfidence, true
		}
		if reverseConfidence > forwardConfidence+materialOrderScoreEpsilon {
			return reverseForwardBefore, reverseConfidence, true
		}
		// 信頼度が拮抗した場合は制約欠落を防ぐため順方向判定を採用する。
		return forwardBefore, forwardConfidence, true
	}
	if forwardValid {
		return forwardBefore, forwardConfidence, true
	}
	return reverseForwardBefore, reverseConfidence, true
}

// resolvePairOrderByOverlap は材質ペアの順序制約を返す。
func resolvePairOrderByOverlap(
	leftMaterialIndex int,
	rightMaterialIndex int,
	spatialInfoMap map[int]materialSpatialInfo,
	overlapThreshold float64,
	materialTransparencyScores map[int]float64,
) (bool, float64, bool) {
	leftInfo, leftOK := spatialInfoMap[leftMaterialIndex]
	rightInfo, rightOK := spatialInfoMap[rightMaterialIndex]
	if !leftOK || !rightOK {
		return false, 0, false
	}

	leftScore, rightScore, leftCoverage, rightCoverage, valid := calculateOverlapBodyMetrics(
		leftInfo,
		rightInfo,
		overlapThreshold,
	)
	if !valid {
		return false, 0, false
	}

	leftTransparency := materialTransparencyScores[leftMaterialIndex]
	rightTransparency := materialTransparencyScores[rightMaterialIndex]
	transparencyDelta := leftTransparency - rightTransparency
	absTransparencyDelta := math.Abs(transparencyDelta)
	scoreDelta := math.Abs(leftScore - rightScore)
	coverageGap := math.Abs(leftCoverage - rightCoverage)
	minCoverage := math.Min(leftCoverage, rightCoverage)
	baseConfidence := calculatePairOrderConfidence(scoreDelta, absTransparencyDelta, minCoverage, coverageGap)

	// 片側だけが重なる材質ペアは近い方を先に描画して剥離を抑える。
	if coverageGap >= overlapAsymmetricCoverageGapMin && minCoverage < overlapAsymmetricMinCoverageMax {
		if absTransparencyDelta >= materialTransparencyOrderDelta {
			// 非対称重なりでは低透明率優先を基本としつつ、低透明側が遠い場合は高透明率側を優先する。
			lowIsLeft := leftTransparency < rightTransparency
			lowScore := leftScore
			highScore := rightScore
			lowTransparency := leftTransparency
			highTransparency := rightTransparency
			if !lowIsLeft {
				lowScore = rightScore
				highScore = leftScore
				lowTransparency = rightTransparency
				highTransparency = leftTransparency
			}
			if math.Abs(lowScore-highScore) <= materialOrderScoreEpsilon || scoreDelta < minimumMaterialOrderDelta {
				return false, 0, false
			}
			lowFartherDelta := lowScore - highScore
			chooseLow := lowFartherDelta <= materialOrderScoreEpsilon
			if lowTransparency >= asymHighAlphaThreshold &&
				highTransparency >= asymHighAlphaThreshold &&
				(highTransparency-lowTransparency) >= asymHighAlphaGapSwitchDelta &&
				lowFartherDelta > 0 {
				chooseLow = false
			}
			if chooseLow {
				return lowIsLeft, baseConfidence + 1.2, true
			}
			return !lowIsLeft, baseConfidence + 1.2, true
		}
		if scoreDelta <= materialOrderScoreEpsilon || scoreDelta < minimumMaterialOrderDelta {
			return false, 0, false
		}
		return leftScore < rightScore, baseConfidence + 0.9, true
	}

	// 重なりが極小なペアでは透明率を優先する。
	if minCoverage < veryLowCoverageTransparencyMax && absTransparencyDelta >= materialTransparencyOrderDelta {
		return leftTransparency < rightTransparency, baseConfidence + 1.0, true
	}

	// カバレッジが近い重なりは透明率差を優先する。
	if minCoverage >= tinyDepthFarFirstCoverageThreshold &&
		minCoverage < strongOverlapCoverageThreshold &&
		coverageGap <= balancedOverlapGapMax &&
		absTransparencyDelta >= balancedOverlapTransparencyMinDelta {
		return leftTransparency < rightTransparency, baseConfidence + 1.1, true
	}

	// 透明率が実質同一で密接に重なる場合は近い方を先に描画する。
	if absTransparencyDelta <= exactTransparencyDeltaThreshold && minCoverage >= strongOverlapCoverageThreshold {
		if scoreDelta <= materialOrderScoreEpsilon || scoreDelta < minimumMaterialOrderDelta {
			return false, 0, false
		}
		return leftScore < rightScore, baseConfidence + 2.0, true
	}

	// 強重なりかつ透明率差が十分大きい場合は低透明率を優先する。
	if minCoverage >= strongOverlapCoverageThreshold &&
		absTransparencyDelta >= balancedOverlapTransparencyMinDelta &&
		math.Min(leftTransparency, rightTransparency) <= 0.5 {
		return leftTransparency < rightTransparency, baseConfidence + 1.0, true
	}

	// 深度差が十分ある場合は遠い方を先に描画する。
	if scoreDelta >= materialDepthSwitchDelta {
		confidence := baseConfidence + 0.7
		if minCoverage < strongOverlapCoverageThreshold &&
			absTransparencyDelta < balancedOverlapTransparencyMinDelta {
			confidence -= midCoverageDepthConfidencePenalty
			if confidence < 0.1 {
				confidence = 0.1
			}
		}
		return leftScore > rightScore, confidence, true
	}

	// 強重なりで深度差が小さい場合は低透明率を先に描画する。
	if minCoverage >= strongOverlapCoverageThreshold && absTransparencyDelta >= materialTransparencyOrderDelta {
		return leftTransparency < rightTransparency, baseConfidence + 0.9, true
	}

	// 深度差が極小の場合は重なり量で遠方先行/近傍先行を切り替える。
	if scoreDelta < tinyDepthDeltaThreshold {
		if minCoverage >= tinyDepthFarFirstCoverageThreshold {
			return leftScore > rightScore, baseConfidence + 0.4, true
		}
		return leftScore < rightScore, baseConfidence + 0.4, true
	}

	if scoreDelta <= materialOrderScoreEpsilon || scoreDelta < minimumMaterialOrderDelta {
		return false, 0, false
	}
	denominator := math.Max(math.Max(math.Abs(leftScore), math.Abs(rightScore)), materialOrderScoreEpsilon)
	relativeDelta := scoreDelta / denominator
	if relativeDelta < materialRelativeNearDelta {
		return leftScore < rightScore, baseConfidence + 0.5, true
	}
	return leftScore > rightScore, baseConfidence + 0.5, true
}

// resolvePairOrderByBodyProximity は重なり判定不能ペアをボディ近傍スコアで補完判定する。
func resolvePairOrderByBodyProximity(
	leftMaterialIndex int,
	rightMaterialIndex int,
	bodyProximityScores map[int]float64,
	materialTransparencyScores map[int]float64,
) (bool, float64, bool) {
	leftScore, leftOK := bodyProximityScores[leftMaterialIndex]
	rightScore, rightOK := bodyProximityScores[rightMaterialIndex]
	if !leftOK || !rightOK {
		return false, 0, false
	}
	if math.IsInf(leftScore, 0) || math.IsInf(rightScore, 0) {
		return false, 0, false
	}
	if leftScore >= math.MaxFloat64/4 || rightScore >= math.MaxFloat64/4 {
		return false, 0, false
	}

	scoreDelta := math.Abs(leftScore - rightScore)
	leftTransparency := materialTransparencyScores[leftMaterialIndex]
	rightTransparency := materialTransparencyScores[rightMaterialIndex]
	transparencyDelta := math.Abs(leftTransparency - rightTransparency)
	if scoreDelta < minimumMaterialOrderDelta && transparencyDelta < materialTransparencyOrderDelta {
		return false, 0, false
	}

	if scoreDelta >= minimumMaterialOrderDelta {
		confidence := 0.4 + math.Min(scoreDelta/math.Max(nonOverlapSwapMinimumDelta, materialOrderScoreEpsilon), 1.0)
		if transparencyDelta >= materialTransparencyOrderDelta {
			confidence += 0.2
		}
		return leftScore < rightScore, confidence, true
	}

	confidence := 0.35 + math.Min(
		transparencyDelta/math.Max(materialTransparencyOrderDelta, materialOrderScoreEpsilon),
		1.0,
	)*0.4
	return leftTransparency < rightTransparency, confidence, true
}

// calculatePairOrderConfidence は材質ペア順序制約の基本信頼度を算出する。
func calculatePairOrderConfidence(
	scoreDelta float64,
	absTransparencyDelta float64,
	minCoverage float64,
	coverageGap float64,
) float64 {
	depthComponent := scoreDelta / math.Max(materialDepthSwitchDelta, materialOrderScoreEpsilon)
	if depthComponent > 2.0 {
		depthComponent = 2.0
	}

	transparencyComponent := absTransparencyDelta / math.Max(materialTransparencyOrderDelta, materialOrderScoreEpsilon)
	if transparencyComponent > 2.0 {
		transparencyComponent = 2.0
	}

	coverageComponent := minCoverage / math.Max(strongOverlapCoverageThreshold, materialOrderScoreEpsilon)
	if coverageComponent > 1.5 {
		coverageComponent = 1.5
	}

	asymmetryComponent := coverageGap / math.Max(overlapAsymmetricCoverageGapMin, materialOrderScoreEpsilon)
	if asymmetryComponent > 1.5 {
		asymmetryComponent = 1.5
	}

	return 0.1 + depthComponent + transparencyComponent + coverageComponent + asymmetryComponent
}

// collectMaterialSpatialInfos は材質ごとの点群とボディ距離情報を収集する。
func collectMaterialSpatialInfos(
	modelData *ModelData,
	faceRanges []materialFaceRange,
	materialIndexes []int,
	bodyPoints []mmath.Vec3,
	blockSize int,
) map[int]materialSpatialInfo {
	out := make(map[int]materialSpatialInfo, len(materialIndexes))
	if modelData == nil || len(bodyPoints) == 0 {
		return out
	}
	for _, materialIndex := range materialIndexes {
		if materialIndex < 0 || materialIndex >= len(faceRanges) {
			continue
		}
		sampleLimit := resolveOverlapSampleLimit(faceRanges[materialIndex], blockSize)
		if sampleLimit <= 0 {
			continue
		}
		sampledPoints := appendSampledMaterialVertices(
			modelData,
			faceRanges[materialIndex],
			make([]mmath.Vec3, 0, sampleLimit),
			sampleLimit,
			blockSize,
		)
		if len(sampledPoints) == 0 {
			continue
		}
		bodyDistances := make([]float64, len(sampledPoints))
		minX := math.MaxFloat64
		minY := math.MaxFloat64
		minZ := math.MaxFloat64
		maxX := -math.MaxFloat64
		maxY := -math.MaxFloat64
		maxZ := -math.MaxFloat64
		for i, point := range sampledPoints {
			bodyDistances[i] = nearestDistance(point, bodyPoints)
			if point.X < minX {
				minX = point.X
			}
			if point.Y < minY {
				minY = point.Y
			}
			if point.Z < minZ {
				minZ = point.Z
			}
			if point.X > maxX {
				maxX = point.X
			}
			if point.Y > maxY {
				maxY = point.Y
			}
			if point.Z > maxZ {
				maxZ = point.Z
			}
		}
		out[materialIndex] = materialSpatialInfo{
			points:       sampledPoints,
			bodyDistance: bodyDistances,
			minX:         minX,
			maxX:         maxX,
			minY:         minY,
			maxY:         maxY,
			minZ:         minZ,
			maxZ:         maxZ,
		}
	}
	return out
}

// estimatePointCloudScale は点群の対角長を返す。
func estimatePointCloudScale(points []mmath.Vec3) float64 {
	if len(points) == 0 {
		return 0
	}
	minX := math.MaxFloat64
	minY := math.MaxFloat64
	minZ := math.MaxFloat64
	maxX := -math.MaxFloat64
	maxY := -math.MaxFloat64
	maxZ := -math.MaxFloat64
	for _, point := range points {
		if point.X < minX {
			minX = point.X
		}
		if point.Y < minY {
			minY = point.Y
		}
		if point.Z < minZ {
			minZ = point.Z
		}
		if point.X > maxX {
			maxX = point.X
		}
		if point.Y > maxY {
			maxY = point.Y
		}
		if point.Z > maxZ {
			maxZ = point.Z
		}
	}
	dx := maxX - minX
	dy := maxY - minY
	dz := maxZ - minZ
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// calculateOverlapBodyMetrics は重なり領域のボディ近傍スコアとカバレッジを返す。
func calculateOverlapBodyMetrics(
	left materialSpatialInfo,
	right materialSpatialInfo,
	overlapThreshold float64,
) (float64, float64, float64, float64, bool) {
	if len(left.points) == 0 || len(right.points) == 0 {
		return 0, 0, 0, 0, false
	}
	// AABBが離れている場合は近接判定を行わず不重なりとみなす。
	interMinX := math.Max(left.minX, right.minX)
	interMaxX := math.Min(left.maxX, right.maxX)
	interMinY := math.Max(left.minY, right.minY)
	interMaxY := math.Min(left.maxY, right.maxY)
	interMinZ := math.Max(left.minZ, right.minZ)
	interMaxZ := math.Min(left.maxZ, right.maxZ)
	if interMinX > interMaxX || interMinY > interMaxY || interMinZ > interMaxZ {
		return 0, 0, 0, 0, false
	}

	leftLocalDistances := make([]float64, 0, len(left.points))
	rightLocalDistances := make([]float64, 0, len(right.points))

	for i, point := range left.points {
		if nearestDistance(point, right.points) > overlapThreshold {
			continue
		}
		leftLocalDistances = append(leftLocalDistances, left.bodyDistance[i])
	}
	for i, point := range right.points {
		if nearestDistance(point, left.points) > overlapThreshold {
			continue
		}
		rightLocalDistances = append(rightLocalDistances, right.bodyDistance[i])
	}
	if len(leftLocalDistances) < minimumOverlapSampleCount || len(rightLocalDistances) < minimumOverlapSampleCount {
		return 0, 0, 0, 0, false
	}
	leftCoverage := float64(len(leftLocalDistances)) / float64(len(left.points))
	rightCoverage := float64(len(rightLocalDistances)) / float64(len(right.points))
	if leftCoverage < minimumOverlapCoverageRatio || rightCoverage < minimumOverlapCoverageRatio {
		return 0, 0, leftCoverage, rightCoverage, false
	}

	return median(leftLocalDistances), median(rightLocalDistances), leftCoverage, rightCoverage, true
}

// calculateOverlapBodyScores は重なり領域のボディ近傍スコアを返す。
func calculateOverlapBodyScores(
	left materialSpatialInfo,
	right materialSpatialInfo,
	overlapThreshold float64,
) (float64, float64, bool) {
	leftScore, rightScore, _, _, valid := calculateOverlapBodyMetrics(left, right, overlapThreshold)
	return leftScore, rightScore, valid
}

// rankNodesByConstraintScore は制約信頼度の勝敗スコアでノード順を決定する。
func rankNodesByConstraintScore(
	nodeCount int,
	constraints []materialOrderConstraint,
	priorities []int,
) []int {
	if nodeCount <= 0 || len(priorities) != nodeCount {
		return []int{}
	}
	scores := make([]float64, nodeCount)
	for _, constraint := range constraints {
		if constraint.from < 0 || constraint.from >= nodeCount || constraint.to < 0 || constraint.to >= nodeCount {
			continue
		}
		scores[constraint.from] += constraint.confidence
		scores[constraint.to] -= constraint.confidence
	}

	nodes := make([]int, nodeCount)
	for nodeIndex := 0; nodeIndex < nodeCount; nodeIndex++ {
		nodes[nodeIndex] = nodeIndex
	}
	sort.SliceStable(nodes, func(i int, j int) bool {
		leftNode := nodes[i]
		rightNode := nodes[j]
		leftScore := scores[leftNode]
		rightScore := scores[rightNode]
		if math.Abs(leftScore-rightScore) > materialOrderScoreEpsilon {
			return leftScore > rightScore
		}
		if priorities[leftNode] != priorities[rightNode] {
			return priorities[leftNode] < priorities[rightNode]
		}
		return leftNode < rightNode
	})
	return nodes
}

// resolveMaterialOrderNodes は制約集合から材質順ノード列を解決する。
func resolveMaterialOrderNodes(
	nodeCount int,
	constraints []materialOrderConstraint,
	priorities []int,
) []int {
	if nodeCount <= 0 || len(priorities) != nodeCount {
		return []int{}
	}
	if len(constraints) == 0 {
		nodes := make([]int, 0, nodeCount)
		for nodeIndex := 0; nodeIndex < nodeCount; nodeIndex++ {
			nodes = append(nodes, nodeIndex)
		}
		return nodes
	}

	if nodeCount <= exactOrderDPMaxNodes {
		optimalNodes, ok := solveOptimalConstraintOrderByDP(nodeCount, constraints, priorities)
		if ok && len(optimalNodes) == nodeCount {
			return optimalNodes
		}
	}

	sortedNodes := stableTopologicalSortByConstraintConfidence(nodeCount, constraints, priorities)
	if len(sortedNodes) == nodeCount {
		return refineOrderByConstraintObjective(sortedNodes, constraints)
	}
	return refineOrderByConstraintObjective(priorities, constraints)
}

// solveOptimalConstraintOrderByDP は制約重み目的関数をビットDPで最大化した順序を返す。
func solveOptimalConstraintOrderByDP(
	nodeCount int,
	constraints []materialOrderConstraint,
	priorities []int,
) ([]int, bool) {
	if nodeCount <= 0 || nodeCount > exactOrderDPMaxNodes || len(priorities) != nodeCount {
		return []int{}, false
	}

	weights := make([][]float64, nodeCount)
	for i := 0; i < nodeCount; i++ {
		weights[i] = make([]float64, nodeCount)
	}
	for _, constraint := range constraints {
		if constraint.from < 0 || constraint.from >= nodeCount || constraint.to < 0 || constraint.to >= nodeCount {
			continue
		}
		weights[constraint.from][constraint.to] += constraint.confidence
		weights[constraint.to][constraint.from] -= constraint.confidence
	}

	subsetCount := 1 << uint(nodeCount)
	dp := make([]float64, subsetCount)
	lastNode := make([]int, subsetCount)
	for subset := 1; subset < subsetCount; subset++ {
		dp[subset] = math.Inf(-1)
		lastNode[subset] = -1
	}
	dp[0] = 0
	lastNode[0] = -1

	for subset := 1; subset < subsetCount; subset++ {
		bestScore := math.Inf(-1)
		bestNode := -1
		for node := 0; node < nodeCount; node++ {
			bit := 1 << uint(node)
			if subset&bit == 0 {
				continue
			}
			prevSubset := subset &^ bit
			if math.IsInf(dp[prevSubset], -1) {
				continue
			}

			score := dp[prevSubset]
			for prevNode := 0; prevNode < nodeCount; prevNode++ {
				prevBit := 1 << uint(prevNode)
				if prevSubset&prevBit == 0 {
					continue
				}
				score += weights[prevNode][node]
			}

			if score > bestScore+materialOrderScoreEpsilon {
				bestScore = score
				bestNode = node
				continue
			}
			if math.Abs(score-bestScore) > materialOrderScoreEpsilon || bestNode < 0 {
				continue
			}
			if priorities[node] > priorities[bestNode] ||
				(priorities[node] == priorities[bestNode] && node > bestNode) {
				bestNode = node
			}
		}
		dp[subset] = bestScore
		lastNode[subset] = bestNode
		if bestNode < 0 {
			return []int{}, false
		}
	}

	reversed := make([]int, 0, nodeCount)
	subset := subsetCount - 1
	for subset > 0 {
		node := lastNode[subset]
		if node < 0 {
			return []int{}, false
		}
		reversed = append(reversed, node)
		subset &^= 1 << uint(node)
	}

	order := make([]int, nodeCount)
	for i := 0; i < nodeCount; i++ {
		order[i] = reversed[nodeCount-1-i]
	}
	return order, true
}

// refineOrderByConstraintObjective は制約満足度が上がる隣接swapを反復して順序を改善する。
func refineOrderByConstraintObjective(
	initialOrder []int,
	constraints []materialOrderConstraint,
) []int {
	order := append([]int(nil), initialOrder...)
	if len(order) < 2 || len(constraints) == 0 {
		return order
	}

	currentScore := calculateConstraintOrderObjective(order, constraints)
	maxPassCount := len(order) * 6
	for pass := 0; pass < maxPassCount; pass++ {
		bestScore := currentScore
		bestOrder := append([]int(nil), order...)
		for from := 0; from < len(order); from++ {
			for to := 0; to < len(order); to++ {
				if from == to {
					continue
				}
				candidate := moveNodeInOrder(order, from, to)
				candidateScore := calculateConstraintOrderObjective(candidate, constraints)
				if candidateScore > bestScore+materialOrderScoreEpsilon {
					bestScore = candidateScore
					bestOrder = candidate
				}
			}
		}
		if bestScore <= currentScore+materialOrderScoreEpsilon {
			break
		}
		order = bestOrder
		currentScore = bestScore
	}
	return order
}

// moveNodeInOrder は順序配列から1要素を取り出して指定位置へ挿入した配列を返す。
func moveNodeInOrder(order []int, from int, to int) []int {
	if len(order) == 0 || from < 0 || from >= len(order) || to < 0 || to >= len(order) {
		return append([]int(nil), order...)
	}
	moved := append([]int(nil), order...)
	value := moved[from]
	if from < to {
		copy(moved[from:to], moved[from+1:to+1])
		moved[to] = value
		return moved
	}
	copy(moved[to+1:from+1], moved[to:from])
	moved[to] = value
	return moved
}

// calculateConstraintOrderObjective は現在順で満たしている制約の重み合計を返す。
func calculateConstraintOrderObjective(
	order []int,
	constraints []materialOrderConstraint,
) float64 {
	if len(order) == 0 || len(constraints) == 0 {
		return 0
	}
	positions := make([]int, len(order))
	for i := range positions {
		positions[i] = -1
	}
	for i, node := range order {
		if node < 0 || node >= len(positions) {
			continue
		}
		positions[node] = i
	}

	score := 0.0
	for _, constraint := range constraints {
		if constraint.from < 0 || constraint.from >= len(positions) || constraint.to < 0 || constraint.to >= len(positions) {
			continue
		}
		fromPos := positions[constraint.from]
		toPos := positions[constraint.to]
		if fromPos < 0 || toPos < 0 {
			continue
		}
		if fromPos < toPos {
			score += constraint.confidence
			continue
		}
		score -= constraint.confidence
	}
	return score
}

// stableTopologicalSortByConstraintConfidence は信頼度付き制約で安定トポロジカルソートを行う。
func stableTopologicalSortByConstraintConfidence(
	nodeCount int,
	constraints []materialOrderConstraint,
	priorities []int,
) []int {
	if nodeCount <= 0 || len(priorities) != nodeCount {
		return []int{}
	}
	if len(constraints) == 0 {
		result := make([]int, 0, nodeCount)
		for nodeIndex := 0; nodeIndex < nodeCount; nodeIndex++ {
			result = append(result, nodeIndex)
		}
		return result
	}

	active := make([]bool, len(constraints))
	for i := range active {
		active[i] = true
	}

	for removedCount := 0; removedCount <= len(constraints); removedCount++ {
		sortedNodes, unprocessedNodes, ok := stableTopologicalSortWithActiveConstraints(
			nodeCount,
			constraints,
			active,
			priorities,
		)
		if ok {
			return sortedNodes
		}
		if len(unprocessedNodes) == 0 {
			break
		}

		cyclicNodes := collectCyclicNodesByTarjan(nodeCount, constraints, active, unprocessedNodes)
		if len(cyclicNodes) == 0 {
			cyclicNodes = unprocessedNodes
		}
		weakestIndex := pickWeakestConstraintFromCycle(constraints, active, cyclicNodes, priorities)
		if weakestIndex < 0 {
			break
		}
		active[weakestIndex] = false
	}

	return []int{}
}

// stableTopologicalSortWithActiveConstraints は有効制約のみで安定トポロジカルソートを試行する。
func stableTopologicalSortWithActiveConstraints(
	nodeCount int,
	constraints []materialOrderConstraint,
	active []bool,
	priorities []int,
) ([]int, map[int]struct{}, bool) {
	if nodeCount <= 0 || len(priorities) != nodeCount || len(active) != len(constraints) {
		return []int{}, map[int]struct{}{}, false
	}

	adjacency := make([][]int, nodeCount)
	inDegree := make([]int, nodeCount)
	for i, constraint := range constraints {
		if !active[i] {
			continue
		}
		if constraint.from < 0 || constraint.from >= nodeCount || constraint.to < 0 || constraint.to >= nodeCount {
			continue
		}
		adjacency[constraint.from] = append(adjacency[constraint.from], constraint.to)
		inDegree[constraint.to]++
	}

	available := make([]int, 0, nodeCount)
	processed := make([]bool, nodeCount)
	for nodeIndex := 0; nodeIndex < nodeCount; nodeIndex++ {
		if inDegree[nodeIndex] == 0 {
			available = appendPriorityNode(available, nodeIndex, priorities)
		}
	}

	result := make([]int, 0, nodeCount)
	for len(available) > 0 {
		nodeIndex := available[0]
		available = available[1:]
		if processed[nodeIndex] {
			continue
		}
		processed[nodeIndex] = true
		result = append(result, nodeIndex)

		for _, next := range adjacency[nodeIndex] {
			inDegree[next]--
			if inDegree[next] == 0 && !processed[next] {
				available = appendPriorityNode(available, next, priorities)
			}
		}
	}

	if len(result) == nodeCount {
		return result, map[int]struct{}{}, true
	}

	unprocessedNodes := make(map[int]struct{})
	for nodeIndex := 0; nodeIndex < nodeCount; nodeIndex++ {
		if !processed[nodeIndex] {
			unprocessedNodes[nodeIndex] = struct{}{}
		}
	}
	return result, unprocessedNodes, false
}

// pickWeakestConstraintFromCycle は循環ノード内で最も弱い制約を選んで返す。
func pickWeakestConstraintFromCycle(
	constraints []materialOrderConstraint,
	active []bool,
	cyclicNodes map[int]struct{},
	priorities []int,
) int {
	if len(active) != len(constraints) || len(cyclicNodes) == 0 {
		return -1
	}

	weakestIndex := -1
	weakestConfidence := math.MaxFloat64
	weakestSpan := -1

	for i, constraint := range constraints {
		if !active[i] {
			continue
		}
		if _, ok := cyclicNodes[constraint.from]; !ok {
			continue
		}
		if _, ok := cyclicNodes[constraint.to]; !ok {
			continue
		}

		currentSpan := int(math.Abs(float64(constraint.from - constraint.to)))
		if weakestIndex < 0 || constraint.confidence < weakestConfidence-materialOrderScoreEpsilon {
			weakestIndex = i
			weakestConfidence = constraint.confidence
			weakestSpan = currentSpan
			continue
		}

		if math.Abs(constraint.confidence-weakestConfidence) > materialOrderScoreEpsilon {
			continue
		}
		if currentSpan > weakestSpan {
			weakestIndex = i
			weakestSpan = currentSpan
			continue
		}
		if currentSpan < weakestSpan || weakestIndex < 0 {
			continue
		}

		currentFrom := priorities[constraint.from]
		currentTo := priorities[constraint.to]
		weakestFrom := priorities[constraints[weakestIndex].from]
		weakestTo := priorities[constraints[weakestIndex].to]
		if currentFrom > weakestFrom || (currentFrom == weakestFrom && currentTo > weakestTo) {
			weakestIndex = i
			weakestSpan = currentSpan
		}
	}

	return weakestIndex
}

// collectCyclicNodesByTarjan は有効制約のうち循環に含まれるノード集合を返す。
func collectCyclicNodesByTarjan(
	nodeCount int,
	constraints []materialOrderConstraint,
	active []bool,
	targetNodes map[int]struct{},
) map[int]struct{} {
	cyclicNodes := make(map[int]struct{})
	if nodeCount <= 0 || len(active) != len(constraints) || len(targetNodes) == 0 {
		return cyclicNodes
	}

	adjacency := make([][]int, nodeCount)
	selfLoop := make([]bool, nodeCount)
	for i, constraint := range constraints {
		if !active[i] {
			continue
		}
		if _, ok := targetNodes[constraint.from]; !ok {
			continue
		}
		if _, ok := targetNodes[constraint.to]; !ok {
			continue
		}
		adjacency[constraint.from] = append(adjacency[constraint.from], constraint.to)
		if constraint.from == constraint.to {
			selfLoop[constraint.from] = true
		}
	}

	indexes := make([]int, nodeCount)
	lowlinks := make([]int, nodeCount)
	onStack := make([]bool, nodeCount)
	for i := 0; i < nodeCount; i++ {
		indexes[i] = -1
		lowlinks[i] = -1
	}
	stack := make([]int, 0, len(targetNodes))
	currentIndex := 0

	var strongConnect func(node int)
	strongConnect = func(node int) {
		indexes[node] = currentIndex
		lowlinks[node] = currentIndex
		currentIndex++
		stack = append(stack, node)
		onStack[node] = true

		for _, next := range adjacency[node] {
			if indexes[next] < 0 {
				strongConnect(next)
				if lowlinks[next] < lowlinks[node] {
					lowlinks[node] = lowlinks[next]
				}
				continue
			}
			if onStack[next] && indexes[next] < lowlinks[node] {
				lowlinks[node] = indexes[next]
			}
		}

		if lowlinks[node] != indexes[node] {
			return
		}

		component := make([]int, 0, 1)
		for {
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[last] = false
			component = append(component, last)
			if last == node {
				break
			}
		}
		if len(component) > 1 {
			for _, componentNode := range component {
				cyclicNodes[componentNode] = struct{}{}
			}
			return
		}
		if selfLoop[node] {
			cyclicNodes[node] = struct{}{}
		}
	}

	for node := range targetNodes {
		if node < 0 || node >= nodeCount {
			continue
		}
		if indexes[node] >= 0 {
			continue
		}
		strongConnect(node)
	}

	return cyclicNodes
}

// appendPriorityNode は優先順位配列に従ってノードを挿入する。
func appendPriorityNode(sorted []int, index int, priorities []int) []int {
	insertAt := len(sorted)
	for i := range sorted {
		left := sorted[i]
		if priorities[left] > priorities[index] || (priorities[left] == priorities[index] && left > index) {
			insertAt = i
			break
		}
	}
	sorted = append(sorted, 0)
	copy(sorted[insertAt+1:], sorted[insertAt:])
	sorted[insertAt] = index
	return sorted
}

// median は値列の中央値を返す。
func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2.0
}

// buildMaterialFaceRanges は材質ごとの面範囲を算出する。
func buildMaterialFaceRanges(modelData *ModelData) ([]materialFaceRange, error) {
	if modelData == nil || modelData.Materials == nil || modelData.Faces == nil {
		return nil, fmt.Errorf("材質または面データが未設定です")
	}

	faceRanges := make([]materialFaceRange, modelData.Materials.Len())
	faceOffset := 0
	for materialIndex, materialData := range modelData.Materials.Values() {
		if materialData == nil {
			return nil, fmt.Errorf("材質が未設定です: index=%d", materialIndex)
		}
		if materialData.VerticesCount < 0 || materialData.VerticesCount%3 != 0 {
			return nil, fmt.Errorf("材質頂点数が不正です: index=%d verticesCount=%d", materialIndex, materialData.VerticesCount)
		}
		faceCount := materialData.VerticesCount / 3
		if faceOffset+faceCount > modelData.Faces.Len() {
			return nil, fmt.Errorf("面範囲が不正です: index=%d start=%d count=%d faces=%d", materialIndex, faceOffset, faceCount, modelData.Faces.Len())
		}
		faceRanges[materialIndex] = materialFaceRange{
			start: faceOffset,
			count: faceCount,
		}
		faceOffset += faceCount
	}
	if faceOffset != modelData.Faces.Len() {
		return nil, fmt.Errorf("材質頂点数と面数が一致しません: mappedFaces=%d totalFaces=%d", faceOffset, modelData.Faces.Len())
	}
	return faceRanges, nil
}

// isTransparentMaterial は材質を半透明扱いするか判定する。
func isTransparentMaterial(
	modelData *ModelData,
	materialData *model.Material,
	textureAlphaCache map[int]textureAlphaCacheEntry,
) bool {
	return isTransparentMaterialWithTextureThreshold(
		modelData,
		materialData,
		textureAlphaCache,
		textureAlphaTransparentThreshold,
	)
}

// isTransparentMaterialWithTextureThreshold は閾値付きで材質を半透明扱いするか判定する。
func isTransparentMaterialWithTextureThreshold(
	modelData *ModelData,
	materialData *model.Material,
	textureAlphaCache map[int]textureAlphaCacheEntry,
	textureAlphaThreshold float64,
) bool {
	if materialData == nil {
		return false
	}
	return hasTransparentTextureAlphaWithThreshold(
		modelData,
		materialData.TextureIndex,
		textureAlphaCache,
		textureAlphaThreshold,
	)
}

// hasTransparentTextureAlpha はテクスチャに閾値以下のアルファがあるか判定する。
func hasTransparentTextureAlpha(
	modelData *ModelData,
	textureIndex int,
	textureAlphaCache map[int]textureAlphaCacheEntry,
) bool {
	return hasTransparentTextureAlphaWithThreshold(
		modelData,
		textureIndex,
		textureAlphaCache,
		textureAlphaTransparentThreshold,
	)
}

// hasTransparentTextureAlphaWithThreshold は閾値付きでテクスチャアルファ透明判定を返す。
func hasTransparentTextureAlphaWithThreshold(
	modelData *ModelData,
	textureIndex int,
	textureAlphaCache map[int]textureAlphaCacheEntry,
	textureAlphaThreshold float64,
) bool {
	if textureIndex < 0 || modelData == nil || modelData.Textures == nil {
		return false
	}
	cached := textureAlphaCache[textureIndex]
	if cached.checked {
		return cached.transparent
	}

	textureData, err := modelData.Textures.Get(textureIndex)
	if err != nil || textureData == nil || !textureData.IsValid() {
		textureAlphaCache[textureIndex] = textureAlphaCacheEntry{checked: true, transparent: false, transparentRatio: 0, failed: true}
		logMaterialReorderViewerVerbose(
			"材質並べ替え: テクスチャ判定スキップ index=%d reason=invalidTexture err=%v",
			textureIndex,
			err,
		)
		return false
	}

	modelPath := strings.TrimSpace(modelData.Path())
	textureName := strings.TrimSpace(textureData.Name())
	if modelPath == "" || textureName == "" {
		textureAlphaCache[textureIndex] = textureAlphaCacheEntry{checked: true, transparent: false, transparentRatio: 0, failed: true}
		logMaterialReorderViewerVerbose(
			"材質並べ替え: テクスチャ判定スキップ index=%d reason=pathOrNameEmpty modelPath=%q texture=%q",
			textureIndex,
			modelPath,
			textureName,
		)
		return false
	}
	texturePath := filepath.Join(filepath.Dir(modelPath), normalizeTextureRelativePath(textureName))
	transparent, ratio, decodeFormat, err := detectTextureTransparency(texturePath, textureAlphaThreshold)
	if err != nil {
		textureAlphaCache[textureIndex] = textureAlphaCacheEntry{checked: true, transparent: false, transparentRatio: 0, failed: true}
		logMaterialReorderViewerVerbose(
			"材質並べ替え: テクスチャ判定失敗 index=%d threshold=%.3f path=%q format=%q err=%v",
			textureIndex,
			textureAlphaThreshold,
			texturePath,
			decodeFormat,
			err,
		)
		return false
	}
	textureAlphaCache[textureIndex] = textureAlphaCacheEntry{checked: true, transparent: transparent, transparentRatio: ratio, failed: false}
	logMaterialReorderViewerVerbose(
		"材質並べ替え: テクスチャ判定 index=%d threshold=%.3f transparent=%t ratio=%.6f path=%q format=%q",
		textureIndex,
		textureAlphaThreshold,
		transparent,
		ratio,
		texturePath,
		decodeFormat,
	)
	return transparent
}

// normalizeTextureRelativePath は相対パス区切りを現在OS向けに正規化する。
func normalizeTextureRelativePath(path string) string {
	replaced := strings.ReplaceAll(path, "\\", string(os.PathSeparator))
	replaced = strings.ReplaceAll(replaced, "/", string(os.PathSeparator))
	return filepath.Clean(replaced)
}

// decodeTextureImageFile は拡張子優先で画像デコードを行いフォーマット名を返す。
func decodeTextureImageFile(texturePath string) (image.Image, string, error) {
	sourceBytes, err := os.ReadFile(texturePath)
	if err != nil {
		return nil, "", err
	}

	extension := strings.ToLower(strings.TrimSpace(filepath.Ext(texturePath)))
	if extension != "" {
		img, decodeErr := decodeTextureBytesByExtension(sourceBytes, extension)
		if decodeErr == nil {
			return img, normalizeImageFormat(extension), nil
		}
		detectedExtension := detectTextureDataExtension(sourceBytes)
		if detectedExtension != "" && detectedExtension != extension {
			fallbackImage, fallbackErr := decodeTextureBytesByExtension(sourceBytes, detectedExtension)
			if fallbackErr == nil {
				return fallbackImage, normalizeImageFormat(detectedExtension), nil
			}
			return nil, normalizeImageFormat(extension), fmt.Errorf(
				"拡張子=%s と実データ=%s の両方でデコードに失敗しました: extErr=%w fallbackErr=%v",
				extension,
				detectedExtension,
				decodeErr,
				fallbackErr,
			)
		}
		return nil, normalizeImageFormat(extension), decodeErr
	}

	detectedExtension := detectTextureDataExtension(sourceBytes)
	if detectedExtension != "" {
		img, decodeErr := decodeTextureBytesByExtension(sourceBytes, detectedExtension)
		if decodeErr == nil {
			return img, normalizeImageFormat(detectedExtension), nil
		}
		return nil, normalizeImageFormat(detectedExtension), decodeErr
	}
	return nil, "", fmt.Errorf("画像形式を判定できませんでした")
}

// decodeTextureBytesByExtension は拡張子指定で画像バイト列をデコードする。
func decodeTextureBytesByExtension(sourceBytes []byte, extension string) (image.Image, error) {
	reader := bytes.NewReader(sourceBytes)
	switch strings.ToLower(strings.TrimSpace(extension)) {
	case ".png":
		return png.Decode(reader)
	case ".jpg", ".jpeg":
		return jpeg.Decode(reader)
	case ".gif":
		return gif.Decode(reader)
	case ".bmp":
		return bmp.Decode(reader)
	case ".webp":
		return webp.Decode(reader)
	case ".tga":
		return tga.Decode(reader)
	default:
		return nil, fmt.Errorf("未対応画像拡張子です: %s", extension)
	}
}

// detectTextureDataExtension は画像バイト列のシグネチャから拡張子を推定する。
func detectTextureDataExtension(sourceBytes []byte) string {
	if len(sourceBytes) >= 8 &&
		sourceBytes[0] == 0x89 && sourceBytes[1] == 0x50 &&
		sourceBytes[2] == 0x4E && sourceBytes[3] == 0x47 &&
		sourceBytes[4] == 0x0D && sourceBytes[5] == 0x0A &&
		sourceBytes[6] == 0x1A && sourceBytes[7] == 0x0A {
		return ".png"
	}
	if len(sourceBytes) >= 3 &&
		sourceBytes[0] == 0xFF && sourceBytes[1] == 0xD8 && sourceBytes[2] == 0xFF {
		return ".jpg"
	}
	if len(sourceBytes) >= 6 &&
		(string(sourceBytes[0:6]) == "GIF87a" || string(sourceBytes[0:6]) == "GIF89a") {
		return ".gif"
	}
	if len(sourceBytes) >= 2 && sourceBytes[0] == 'B' && sourceBytes[1] == 'M' {
		return ".bmp"
	}
	if len(sourceBytes) >= 12 &&
		string(sourceBytes[0:4]) == "RIFF" &&
		string(sourceBytes[8:12]) == "WEBP" {
		return ".webp"
	}
	return ""
}

// normalizeImageFormat は拡張子文字列をログ出力用のフォーマット名へ変換する。
func normalizeImageFormat(extension string) string {
	trimmed := strings.ToLower(strings.TrimSpace(extension))
	return strings.TrimPrefix(trimmed, ".")
}

// detectTextureTransparency はテクスチャ画像のアルファを走査して透明領域の有無と割合を返す。
func detectTextureTransparency(texturePath string, threshold float64) (bool, float64, string, error) {
	img, decodeFormat, err := decodeTextureImageFile(texturePath)
	if err != nil {
		return false, 0, decodeFormat, err
	}
	bounds := img.Bounds()
	if bounds.Empty() {
		return false, 0, decodeFormat, nil
	}

	totalPixels := 0
	transparentPixels := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			totalPixels++
			alpha := extractAlpha(img.At(x, y))
			if alpha <= threshold {
				transparentPixels++
			}
		}
	}
	if totalPixels == 0 {
		return false, 0, decodeFormat, nil
	}
	ratio := float64(transparentPixels) / float64(totalPixels)
	return transparentPixels > 0, ratio, decodeFormat, nil
}

// extractAlpha は色から0.0-1.0のアルファ値を抽出する。
func extractAlpha(c color.Color) float64 {
	normalized := color.NRGBAModel.Convert(c).(color.NRGBA)
	return float64(normalized.A) / 255.0
}

// resolveBodyPointSampleLimit はボディ基準点の動的サンプル上限を返す。
func resolveBodyPointSampleLimit(
	modelData *ModelData,
	faceRanges []materialFaceRange,
	bodyMaterialIndex int,
	blockSize int,
) int {
	vertexCount := 0
	if bodyMaterialIndex >= 0 && bodyMaterialIndex < len(faceRanges) {
		vertexCount = faceRanges[bodyMaterialIndex].count * 3
	}
	if vertexCount <= 0 && modelData != nil && modelData.Vertices != nil {
		vertexCount = modelData.Vertices.Len()
	}
	return resolveDynamicSampleLimit(vertexCount, blockSize, minimumBodyPointSampleCount)
}

// resolveMaterialSampleLimit は材質近傍スコア計算用の動的サンプル上限を返す。
func resolveMaterialSampleLimit(faceRange materialFaceRange, blockSize int) int {
	vertexCount := faceRange.count * 3
	return resolveDynamicSampleLimit(vertexCount, blockSize, minimumMaterialSampleCount)
}

// resolveOverlapSampleLimit は材質重なり判定用の動的サンプル上限を返す。
func resolveOverlapSampleLimit(faceRange materialFaceRange, blockSize int) int {
	vertexCount := faceRange.count * 3
	return resolveDynamicSampleLimit(vertexCount, blockSize, minimumOverlapPointSampleCount)
}

// resolveDynamicSampleLimit は入力サイズに応じたサンプル上限を返す。
func resolveDynamicSampleLimit(
	elementCount int,
	blockSize int,
	minimumCount int,
) int {
	if elementCount <= 0 {
		return 0
	}
	if blockSize < 1 {
		blockSize = 1
	}
	if minimumCount < 1 {
		minimumCount = 1
	}
	dynamicCount := math.Sqrt(float64(elementCount)) * math.Log2(float64(elementCount)+1.0)
	dynamicCount /= math.Pow(float64(blockSize), dynamicSampleBlockExponent)
	dynamicCount *= dynamicSampleScale
	limit := int(math.Ceil(dynamicCount))
	if limit < minimumCount {
		limit = minimumCount
	}
	if limit > elementCount {
		limit = elementCount
	}
	return limit
}

// collectBodyPointsForSorting は並べ替えに使うボディ基準点を収集する。
func collectBodyPointsForSorting(
	modelData *ModelData,
	faceRanges []materialFaceRange,
	transparentMaterialIndexSet map[int]struct{},
	blockSize int,
) []mmath.Vec3 {
	bodyBoneIndexes := collectBodyBoneIndexesFromHumanoid(modelData)
	bodyMaterialIndex := detectBodyMaterialIndex(modelData, bodyBoneIndexes)
	sampleLimit := resolveBodyPointSampleLimit(modelData, faceRanges, bodyMaterialIndex, blockSize)
	if sampleLimit <= 0 {
		return []mmath.Vec3{}
	}
	if bodyMaterialIndex >= 0 && bodyMaterialIndex < len(faceRanges) {
		points := appendSampledMaterialVertices(
			modelData,
			faceRanges[bodyMaterialIndex],
			make([]mmath.Vec3, 0, sampleLimit),
			sampleLimit,
			blockSize,
		)
		if len(points) > 0 {
			return points
		}
	}
	points := collectBodyWeightedPoints(modelData, bodyBoneIndexes, sampleLimit)
	if len(points) > 0 {
		return points
	}
	return collectBodyPointsFromOpaqueMaterials(modelData, faceRanges, transparentMaterialIndexSet, sampleLimit, blockSize)
}

// collectBodyBoneIndexesFromHumanoid はVRM humanoidからボディ基準ボーンindex集合を収集する。
func collectBodyBoneIndexesFromHumanoid(modelData *ModelData) map[int]struct{} {
	out := map[int]struct{}{}
	if modelData == nil || modelData.VrmData == nil {
		return out
	}
	// VRM->PMX 変換では node を順番に AppendRaw しており、nodeIndex と PMX bone index は同一値で対応する。
	maxBoneIndex := -1
	if modelData.Bones != nil {
		maxBoneIndex = modelData.Bones.Len() - 1
	}

	bodyNames := map[string]struct{}{
		"hips":       {},
		"spine":      {},
		"chest":      {},
		"upperchest": {},
		"neck":       {},
	}

	vrmData := modelData.VrmData
	if vrmData.Vrm1 != nil && vrmData.Vrm1.Humanoid != nil {
		for boneName, humanBone := range vrmData.Vrm1.Humanoid.HumanBones {
			if _, ok := bodyNames[strings.ToLower(strings.TrimSpace(boneName))]; !ok {
				continue
			}
			if humanBone.Node < 0 {
				continue
			}
			if maxBoneIndex >= 0 && humanBone.Node > maxBoneIndex {
				continue
			}
			out[humanBone.Node] = struct{}{}
		}
	}
	if vrmData.Vrm0 != nil && vrmData.Vrm0.Humanoid != nil {
		for _, humanBone := range vrmData.Vrm0.Humanoid.HumanBones {
			if _, ok := bodyNames[strings.ToLower(strings.TrimSpace(humanBone.Bone))]; !ok {
				continue
			}
			if humanBone.Node < 0 {
				continue
			}
			if maxBoneIndex >= 0 && humanBone.Node > maxBoneIndex {
				continue
			}
			out[humanBone.Node] = struct{}{}
		}
	}
	return out
}

// collectBodyWeightedPoints はボディ基準ボーンへのウェイトが高い頂点位置を収集する。
func collectBodyWeightedPoints(modelData *ModelData, bodyBoneIndexes map[int]struct{}, sampleLimit int) []mmath.Vec3 {
	points := make([]mmath.Vec3, 0, sampleLimit)
	if modelData == nil || modelData.Vertices == nil || len(bodyBoneIndexes) == 0 || sampleLimit <= 0 {
		return points
	}

	vertices := modelData.Vertices.Values()
	if len(vertices) == 0 {
		return points
	}
	step := 1
	if len(vertices) > sampleLimit {
		step = len(vertices)/sampleLimit + 1
	}

	for vertexIndex := 0; vertexIndex < len(vertices); vertexIndex += step {
		vertex := vertices[vertexIndex]
		if vertex == nil || vertex.Deform == nil {
			continue
		}
		indexes := vertex.Deform.Indexes()
		weights := vertex.Deform.Weights()
		maxCount := len(indexes)
		if len(weights) < maxCount {
			maxCount = len(weights)
		}
		if maxCount == 0 {
			continue
		}

		bodyWeight := 0.0
		for i := 0; i < maxCount; i++ {
			if _, ok := bodyBoneIndexes[indexes[i]]; ok {
				bodyWeight += weights[i]
			}
		}
		if bodyWeight < bodyWeightThreshold {
			continue
		}

		points = append(points, vertex.Position)
		if len(points) >= sampleLimit {
			break
		}
	}
	return points
}

// detectBodyMaterialIndex はボディ寄与の高い頂点からボディ材質の基準indexを推定する。
func detectBodyMaterialIndex(modelData *ModelData, bodyBoneIndexes map[int]struct{}) int {
	if modelData == nil || modelData.Materials == nil || modelData.Vertices == nil || len(bodyBoneIndexes) == 0 {
		return -1
	}

	materialScores := make([]float64, modelData.Materials.Len())
	for _, vertex := range modelData.Vertices.Values() {
		if vertex == nil || vertex.Deform == nil || len(vertex.MaterialIndexes) == 0 {
			continue
		}
		indexes := vertex.Deform.Indexes()
		weights := vertex.Deform.Weights()
		maxCount := len(indexes)
		if len(weights) < maxCount {
			maxCount = len(weights)
		}
		if maxCount == 0 {
			continue
		}

		bodyWeight := 0.0
		for i := 0; i < maxCount; i++ {
			if _, ok := bodyBoneIndexes[indexes[i]]; ok {
				bodyWeight += weights[i]
			}
		}
		if bodyWeight < bodyWeightThreshold {
			continue
		}

		for _, materialIndex := range vertex.MaterialIndexes {
			if materialIndex < 0 || materialIndex >= len(materialScores) {
				continue
			}
			materialScores[materialIndex] += bodyWeight
		}
	}

	bestIndex := -1
	bestScore := 0.0
	for materialIndex, score := range materialScores {
		if score <= 0 {
			continue
		}
		if bestIndex < 0 || score > bestScore || (score == bestScore && materialIndex < bestIndex) {
			bestIndex = materialIndex
			bestScore = score
		}
	}
	return bestIndex
}

// collectBodyPointsFromOpaqueMaterials は不透明材質の頂点からボディ基準点を収集する。
func collectBodyPointsFromOpaqueMaterials(
	modelData *ModelData,
	faceRanges []materialFaceRange,
	transparentMaterialIndexSet map[int]struct{},
	sampleLimit int,
	blockSize int,
) []mmath.Vec3 {
	points := make([]mmath.Vec3, 0, sampleLimit)
	if modelData == nil || modelData.Materials == nil || sampleLimit <= 0 {
		return points
	}

	opaqueCandidates := make([]int, 0)
	for materialIndex, materialData := range modelData.Materials.Values() {
		if materialData == nil || materialData.VerticesCount <= 0 {
			continue
		}
		if _, exists := transparentMaterialIndexSet[materialIndex]; exists {
			continue
		}
		opaqueCandidates = append(opaqueCandidates, materialIndex)
	}
	if len(opaqueCandidates) == 0 {
		return points
	}

	sort.SliceStable(opaqueCandidates, func(i int, j int) bool {
		left := modelData.Materials.Values()[opaqueCandidates[i]]
		right := modelData.Materials.Values()[opaqueCandidates[j]]
		if left.VerticesCount == right.VerticesCount {
			return opaqueCandidates[i] < opaqueCandidates[j]
		}
		return left.VerticesCount > right.VerticesCount
	})

	for i, materialIndex := range opaqueCandidates {
		if i >= fallbackOpaqueMaterialCount {
			break
		}
		points = appendSampledMaterialVertices(modelData, faceRanges[materialIndex], points, sampleLimit, blockSize)
		if len(points) >= sampleLimit {
			break
		}
	}
	return points
}

// appendSampledMaterialVertices は材質の面範囲から代表頂点をサンプル追加する。
func appendSampledMaterialVertices(
	modelData *ModelData,
	faceRange materialFaceRange,
	current []mmath.Vec3,
	limit int,
	blockSize int,
) []mmath.Vec3 {
	if modelData == nil || modelData.Faces == nil || modelData.Vertices == nil || faceRange.count <= 0 {
		return current
	}
	if limit <= 0 {
		return current
	}

	sampleFaceLimit := resolveDynamicSampleLimit(faceRange.count, blockSize, minimumMaterialFaceSampleCount)
	if sampleFaceLimit <= 0 {
		sampleFaceLimit = 1
	}
	step := 1
	if faceRange.count > sampleFaceLimit {
		step = faceRange.count/sampleFaceLimit + 1
	}

	for i := 0; i < faceRange.count && len(current) < limit; i += step {
		face, err := modelData.Faces.Get(faceRange.start + i)
		if err != nil || face == nil {
			continue
		}
		for _, vertexIndex := range face.VertexIndexes {
			vertex, vErr := modelData.Vertices.Get(vertexIndex)
			if vErr != nil || vertex == nil {
				continue
			}
			current = append(current, vertex.Position)
			if len(current) >= limit {
				break
			}
		}
	}
	return current
}

// calculateBodyProximityScore は材質とボディ基準点群の近さスコアを算出する。
func calculateBodyProximityScore(
	modelData *ModelData,
	faceRange materialFaceRange,
	bodyPoints []mmath.Vec3,
	blockSize int,
) (float64, bool) {
	if modelData == nil || len(bodyPoints) == 0 {
		return 0, false
	}
	sampleLimit := resolveMaterialSampleLimit(faceRange, blockSize)
	if sampleLimit <= 0 {
		return 0, false
	}
	sampledVertices := appendSampledMaterialVertices(
		modelData,
		faceRange,
		make([]mmath.Vec3, 0, sampleLimit),
		sampleLimit,
		blockSize,
	)
	if len(sampledVertices) == 0 {
		return 0, false
	}

	distances := make([]float64, 0, len(sampledVertices))
	for _, vertexPosition := range sampledVertices {
		distances = append(distances, nearestDistance(vertexPosition, bodyPoints))
	}
	if len(distances) == 0 {
		return 0, false
	}
	sort.Float64s(distances)
	mid := len(distances) / 2
	if len(distances)%2 == 1 {
		return distances[mid], true
	}
	return (distances[mid-1] + distances[mid]) / 2.0, true
}

// nearestDistance は点群への最短距離を返す。
func nearestDistance(position mmath.Vec3, points []mmath.Vec3) float64 {
	best := math.MaxFloat64
	for _, p := range points {
		d := position.Distance(p)
		if d < best {
			best = d
		}
	}
	return best
}

// isIdentityOrder は順序変更が発生していないか判定する。
func isIdentityOrder(order []int) bool {
	for i := range order {
		if order[i] != i {
			return false
		}
	}
	return true
}

// rebuildMaterialAndFaceOrder は材質順に合わせて面列と参照インデックスを更新する。
func rebuildMaterialAndFaceOrder(modelData *ModelData, faceRanges []materialFaceRange, newOrder []int) error {
	if modelData == nil || modelData.Materials == nil || modelData.Faces == nil {
		return fmt.Errorf("材質または面データが未設定です")
	}

	oldMaterials := append([]*model.Material(nil), modelData.Materials.Values()...)
	oldFaces := append([]*model.Face(nil), modelData.Faces.Values()...)
	if len(oldMaterials) != len(newOrder) || len(faceRanges) != len(newOrder) {
		return fmt.Errorf("材質順序情報が不正です")
	}

	newMaterials := collection.NewNamedCollection[*model.Material](len(oldMaterials))
	newFaces := collection.NewIndexedCollection[*model.Face](len(oldFaces))
	oldToNew := make([]int, len(oldMaterials))
	for i := range oldToNew {
		oldToNew[i] = -1
	}

	for newIndex, oldIndex := range newOrder {
		if oldIndex < 0 || oldIndex >= len(oldMaterials) {
			return fmt.Errorf("材質indexが不正です: %d", oldIndex)
		}
		materialData := oldMaterials[oldIndex]
		if materialData == nil {
			return fmt.Errorf("材質データが未設定です: %d", oldIndex)
		}
		oldToNew[oldIndex] = newIndex
		newMaterials.AppendRaw(materialData)

		faceRange := faceRanges[oldIndex]
		for i := 0; i < faceRange.count; i++ {
			face := oldFaces[faceRange.start+i]
			if face == nil {
				continue
			}
			newFaces.AppendRaw(face)
		}
	}

	modelData.Materials = newMaterials
	modelData.Faces = newFaces
	remapVertexMaterialIndexes(modelData, oldToNew)
	remapMaterialMorphOffsets(modelData, oldToNew)
	return nil
}

// remapVertexMaterialIndexes は頂点が参照する材質indexを新順序へ変換する。
func remapVertexMaterialIndexes(modelData *ModelData, oldToNew []int) {
	if modelData == nil || modelData.Vertices == nil {
		return
	}
	for _, vertex := range modelData.Vertices.Values() {
		if vertex == nil || len(vertex.MaterialIndexes) == 0 {
			continue
		}
		for i, materialIndex := range vertex.MaterialIndexes {
			if materialIndex < 0 || materialIndex >= len(oldToNew) {
				continue
			}
			newIndex := oldToNew[materialIndex]
			if newIndex < 0 {
				continue
			}
			vertex.MaterialIndexes[i] = newIndex
		}
		sort.Ints(vertex.MaterialIndexes)
	}
}

// remapMaterialMorphOffsets は材質モーフの材質indexを新順序へ変換する。
func remapMaterialMorphOffsets(modelData *ModelData, oldToNew []int) {
	if modelData == nil || modelData.Morphs == nil {
		return
	}
	for _, morph := range modelData.Morphs.Values() {
		if morph == nil || morph.MorphType != model.MORPH_TYPE_MATERIAL {
			continue
		}
		for _, offset := range morph.Offsets {
			materialOffset, ok := offset.(*model.MaterialMorphOffset)
			if !ok || materialOffset == nil {
				continue
			}
			if materialOffset.MaterialIndex < 0 || materialOffset.MaterialIndex >= len(oldToNew) {
				continue
			}
			newIndex := oldToNew[materialOffset.MaterialIndex]
			if newIndex < 0 {
				continue
			}
			materialOffset.MaterialIndex = newIndex
		}
	}
}
