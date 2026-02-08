// 指示: miu200521358
package minteractor

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/collection"
)

const (
	materialDiffuseTransparentThreshold = 0.995
	textureAlphaTransparentThreshold    = 0.05
	bodyWeightThreshold                 = 0.35
	maxBodyPointCount                   = 3072
	maxMaterialSampleVertices           = 384
	fallbackOpaqueMaterialCount         = 3
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
	checked     bool
	transparent bool
}

// applyBodyDepthMaterialOrder は半透明材質をボディ近傍順へ並べ替える。
func applyBodyDepthMaterialOrder(modelData *ModelData) {
	if modelData == nil || modelData.Materials == nil || modelData.Faces == nil {
		return
	}

	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil {
		return
	}
	if len(faceRanges) < 2 {
		return
	}

	textureAlphaCache := map[int]textureAlphaCacheEntry{}
	transparentMaterialIndexes := make([]int, 0)
	for materialIndex, materialData := range modelData.Materials.Values() {
		if isTransparentMaterial(modelData, materialData, textureAlphaCache) {
			transparentMaterialIndexes = append(transparentMaterialIndexes, materialIndex)
		}
	}
	if len(transparentMaterialIndexes) < 2 {
		return
	}

	bodyPoints := collectBodyPointsForSorting(modelData, faceRanges, textureAlphaCache)
	if len(bodyPoints) == 0 {
		return
	}

	metrics := make([]materialSortMetric, 0, len(transparentMaterialIndexes))
	for _, materialIndex := range transparentMaterialIndexes {
		score, ok := calculateBodyProximityScore(modelData, faceRanges[materialIndex], bodyPoints)
		if !ok {
			score = math.MaxFloat64
		}
		metrics = append(metrics, materialSortMetric{
			index: materialIndex,
			score: score,
		})
	}
	if len(metrics) < 2 {
		return
	}
	sort.SliceStable(metrics, func(i int, j int) bool {
		if metrics[i].score == metrics[j].score {
			return metrics[i].index < metrics[j].index
		}
		// Index が小さい方が先に描画されるため、身体に近いほど小さい index へ寄せる。
		return metrics[i].score < metrics[j].score
	})

	newOrder := make([]int, modelData.Materials.Len())
	for i := range newOrder {
		newOrder[i] = i
	}
	for i, position := range transparentMaterialIndexes {
		newOrder[position] = metrics[i].index
	}
	if isIdentityOrder(newOrder) {
		return
	}

	_ = rebuildMaterialAndFaceOrder(modelData, faceRanges, newOrder)
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
	if materialData == nil {
		return false
	}
	if materialData.Diffuse.W < materialDiffuseTransparentThreshold {
		return true
	}
	return hasTransparentTextureAlpha(modelData, materialData.TextureIndex, textureAlphaCache)
}

// hasTransparentTextureAlpha はテクスチャに閾値以下のアルファがあるか判定する。
func hasTransparentTextureAlpha(
	modelData *ModelData,
	textureIndex int,
	textureAlphaCache map[int]textureAlphaCacheEntry,
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
		textureAlphaCache[textureIndex] = textureAlphaCacheEntry{checked: true, transparent: false}
		return false
	}

	modelPath := strings.TrimSpace(modelData.Path())
	textureName := strings.TrimSpace(textureData.Name())
	if modelPath == "" || textureName == "" {
		textureAlphaCache[textureIndex] = textureAlphaCacheEntry{checked: true, transparent: false}
		return false
	}
	texturePath := filepath.Join(filepath.Dir(modelPath), normalizeTextureRelativePath(textureName))
	transparent, err := detectTextureTransparency(texturePath, textureAlphaTransparentThreshold)
	if err != nil {
		textureAlphaCache[textureIndex] = textureAlphaCacheEntry{checked: true, transparent: false}
		return false
	}
	textureAlphaCache[textureIndex] = textureAlphaCacheEntry{checked: true, transparent: transparent}
	return transparent
}

// normalizeTextureRelativePath は相対パス区切りを現在OS向けに正規化する。
func normalizeTextureRelativePath(path string) string {
	replaced := strings.ReplaceAll(path, "\\", string(os.PathSeparator))
	replaced = strings.ReplaceAll(replaced, "/", string(os.PathSeparator))
	return filepath.Clean(replaced)
}

// detectTextureTransparency はテクスチャ画像のアルファを走査して透明領域の有無を返す。
func detectTextureTransparency(texturePath string, threshold float64) (bool, error) {
	file, err := os.Open(texturePath)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = file.Close()
	}()

	img, _, err := image.Decode(file)
	if err != nil {
		return false, err
	}
	bounds := img.Bounds()
	if bounds.Empty() {
		return false, nil
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			alpha := extractAlpha(img.At(x, y))
			if alpha <= threshold {
				return true, nil
			}
		}
	}
	return false, nil
}

// extractAlpha は色から0.0-1.0のアルファ値を抽出する。
func extractAlpha(c color.Color) float64 {
	normalized := color.NRGBAModel.Convert(c).(color.NRGBA)
	return float64(normalized.A) / 255.0
}

// collectBodyPointsForSorting は並べ替えに使うボディ基準点を収集する。
func collectBodyPointsForSorting(
	modelData *ModelData,
	faceRanges []materialFaceRange,
	textureAlphaCache map[int]textureAlphaCacheEntry,
) []mmath.Vec3 {
	bodyBoneIndexes := collectBodyBoneIndexesFromHumanoid(modelData)
	points := collectBodyWeightedPoints(modelData, bodyBoneIndexes)
	if len(points) > 0 {
		return points
	}
	return collectBodyPointsFromOpaqueMaterials(modelData, faceRanges, textureAlphaCache)
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
func collectBodyWeightedPoints(modelData *ModelData, bodyBoneIndexes map[int]struct{}) []mmath.Vec3 {
	points := make([]mmath.Vec3, 0, maxBodyPointCount)
	if modelData == nil || modelData.Vertices == nil || len(bodyBoneIndexes) == 0 {
		return points
	}

	vertices := modelData.Vertices.Values()
	if len(vertices) == 0 {
		return points
	}
	step := 1
	if len(vertices) > maxBodyPointCount {
		step = len(vertices)/maxBodyPointCount + 1
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
		if len(points) >= maxBodyPointCount {
			break
		}
	}
	return points
}

// collectBodyPointsFromOpaqueMaterials は不透明材質の頂点からボディ基準点を収集する。
func collectBodyPointsFromOpaqueMaterials(
	modelData *ModelData,
	faceRanges []materialFaceRange,
	textureAlphaCache map[int]textureAlphaCacheEntry,
) []mmath.Vec3 {
	points := make([]mmath.Vec3, 0, maxBodyPointCount)
	if modelData == nil || modelData.Materials == nil {
		return points
	}

	opaqueCandidates := make([]int, 0)
	for materialIndex, materialData := range modelData.Materials.Values() {
		if materialData == nil || materialData.VerticesCount <= 0 {
			continue
		}
		if isTransparentMaterial(modelData, materialData, textureAlphaCache) {
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
		points = appendSampledMaterialVertices(modelData, faceRanges[materialIndex], points, maxBodyPointCount)
		if len(points) >= maxBodyPointCount {
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
) []mmath.Vec3 {
	if modelData == nil || modelData.Faces == nil || modelData.Vertices == nil || faceRange.count <= 0 {
		return current
	}

	sampleFaceLimit := maxMaterialSampleVertices / 3
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
) (float64, bool) {
	if modelData == nil || len(bodyPoints) == 0 {
		return 0, false
	}
	sampledVertices := appendSampledMaterialVertices(modelData, faceRange, make([]mmath.Vec3, 0, maxMaterialSampleVertices), maxMaterialSampleVertices)
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
