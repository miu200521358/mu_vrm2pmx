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
	maxOverlapSampleVertices            = 192
	fallbackOpaqueMaterialCount         = 3
	overlapPointScaleRatio              = 0.03
	overlapPointDistanceMin             = 0.01
	minimumOverlapSampleCount           = 4
	minimumOverlapCoverageRatio         = 0.05
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
	asymLowTransFartherSwitchDelta      = 0.09
	asymHighAlphaThreshold              = 0.90
	asymHighAlphaGapSwitchDelta         = 0.08
	balancedOverlapGapMax               = 0.03
	balancedOverlapTransparencyMinDelta = 0.06
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
	bodyBoneIndexes := collectBodyBoneIndexesFromHumanoid(modelData)
	bodyMaterialIndex := detectBodyMaterialIndex(modelData, bodyBoneIndexes)
	if bodyMaterialIndex >= 0 {
		filteredTransparentMaterialIndexes := make([]int, 0, len(transparentMaterialIndexes))
		for _, materialIndex := range transparentMaterialIndexes {
			if materialIndex == bodyMaterialIndex {
				continue
			}
			filteredTransparentMaterialIndexes = append(filteredTransparentMaterialIndexes, materialIndex)
		}
		transparentMaterialIndexes = filteredTransparentMaterialIndexes
	}
	if len(transparentMaterialIndexes) < 2 {
		return
	}

	bodyPoints := collectBodyPointsForSorting(modelData, faceRanges, textureAlphaCache)
	if len(bodyPoints) == 0 {
		return
	}
	materialTransparencyScores := buildMaterialTransparencyScores(modelData, textureAlphaCache)
	newOrder := make([]int, modelData.Materials.Len())
	for i := range newOrder {
		newOrder[i] = i
	}
	transparentBlocks := splitContinuousMaterialIndexBlocks(transparentMaterialIndexes)
	for _, block := range transparentBlocks {
		if len(block) < 2 {
			continue
		}
		sortedBlock := sortTransparentMaterialsByOverlapDepth(
			modelData,
			faceRanges,
			block,
			bodyPoints,
			materialTransparencyScores,
		)
		if len(sortedBlock) != len(block) {
			continue
		}
		for i, position := range block {
			newOrder[position] = sortedBlock[i]
		}
	}
	if isIdentityOrder(newOrder) {
		return
	}

	_ = rebuildMaterialAndFaceOrder(modelData, faceRanges, newOrder)
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

// buildMaterialTransparencyScores は材質ごとの透明画素率スコアを返す。
func buildMaterialTransparencyScores(
	modelData *ModelData,
	textureAlphaCache map[int]textureAlphaCacheEntry,
) map[int]float64 {
	scores := make(map[int]float64)
	if modelData == nil || modelData.Materials == nil {
		return scores
	}
	for materialIndex, materialData := range modelData.Materials.Values() {
		if materialData == nil {
			continue
		}
		score := 0.0
		if materialData.Diffuse.W < materialDiffuseTransparentThreshold {
			score = math.Max(score, 1.0-materialData.Diffuse.W)
		}
		if hasTransparentTextureAlpha(modelData, materialData.TextureIndex, textureAlphaCache) {
			cacheEntry := textureAlphaCache[materialData.TextureIndex]
			if cacheEntry.transparentRatio > score {
				score = cacheEntry.transparentRatio
			}
		}
		scores[materialIndex] = score
	}
	return scores
}

// sortTransparentMaterialsByOverlapDepth は重なり領域のボディ近傍度から透明材質順を決定する。
func sortTransparentMaterialsByOverlapDepth(
	modelData *ModelData,
	faceRanges []materialFaceRange,
	transparentMaterialIndexes []int,
	bodyPoints []mmath.Vec3,
	materialTransparencyScores map[int]float64,
) []int {
	if len(transparentMaterialIndexes) < 2 {
		return append([]int(nil), transparentMaterialIndexes...)
	}

	// 元順序を起点に、重なり判定で前後が確定できる材質ペアから順序制約を組み立てる。
	sortedMaterialIndexes := append([]int(nil), transparentMaterialIndexes...)
	bodyProximityScores := make(map[int]float64, len(sortedMaterialIndexes))
	for _, materialIndex := range sortedMaterialIndexes {
		score, ok := calculateBodyProximityScore(modelData, faceRanges[materialIndex], bodyPoints)
		if !ok {
			score = math.MaxFloat64
		}
		bodyProximityScores[materialIndex] = score
	}

	spatialInfoMap := collectMaterialSpatialInfos(modelData, faceRanges, transparentMaterialIndexes, bodyPoints)
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
	adjacency := make([][]int, nodeCount)
	inDegree := make([]int, nodeCount)
	addedEdges := make(map[[2]int]struct{})
	edgeCount := 0

	for i := 0; i < nodeCount-1; i++ {
		leftMaterialIndex := sortedMaterialIndexes[i]
		for j := i + 1; j < nodeCount; j++ {
			rightMaterialIndex := sortedMaterialIndexes[j]
			leftBeforeRight, valid := resolvePairOrderByOverlap(
				leftMaterialIndex,
				rightMaterialIndex,
				spatialInfoMap,
				overlapThreshold,
				materialTransparencyScores,
			)
			if !valid {
				continue
			}
			beforeMaterialIndex := leftMaterialIndex
			afterMaterialIndex := rightMaterialIndex
			if !leftBeforeRight {
				beforeMaterialIndex = rightMaterialIndex
				afterMaterialIndex = leftMaterialIndex
			}
			beforeNode := nodeByMaterialIndex[beforeMaterialIndex]
			afterNode := nodeByMaterialIndex[afterMaterialIndex]
			edge := [2]int{beforeNode, afterNode}
			if _, exists := addedEdges[edge]; exists {
				continue
			}
			addedEdges[edge] = struct{}{}
			adjacency[beforeNode] = append(adjacency[beforeNode], afterNode)
			inDegree[afterNode]++
			edgeCount++
		}
	}

	if edgeCount == 0 {
		if nodeCount == 2 {
			left := sortedMaterialIndexes[0]
			right := sortedMaterialIndexes[1]
			if bodyProximityScores[left]-bodyProximityScores[right] > nonOverlapSwapMinimumDelta {
				return []int{right, left}
			}
		}
		return sortedMaterialIndexes
	}

	sortedNodes := stableTopologicalSortByPriority(adjacency, inDegree, nodePriorities)
	if len(sortedNodes) != nodeCount {
		return sortedMaterialIndexes
	}
	result := make([]int, 0, nodeCount)
	for _, nodeIndex := range sortedNodes {
		result = append(result, sortedMaterialIndexes[nodeIndex])
	}
	return result
}

// resolvePairOrderByOverlap は材質ペアの順序制約を返す。
func resolvePairOrderByOverlap(
	leftMaterialIndex int,
	rightMaterialIndex int,
	spatialInfoMap map[int]materialSpatialInfo,
	overlapThreshold float64,
	materialTransparencyScores map[int]float64,
) (bool, bool) {
	leftInfo, leftOK := spatialInfoMap[leftMaterialIndex]
	rightInfo, rightOK := spatialInfoMap[rightMaterialIndex]
	if !leftOK || !rightOK {
		return false, false
	}

	leftScore, rightScore, leftCoverage, rightCoverage, valid := calculateOverlapBodyMetrics(
		leftInfo,
		rightInfo,
		overlapThreshold,
	)
	if !valid {
		return false, false
	}

	leftTransparency := materialTransparencyScores[leftMaterialIndex]
	rightTransparency := materialTransparencyScores[rightMaterialIndex]
	transparencyDelta := leftTransparency - rightTransparency
	absTransparencyDelta := math.Abs(transparencyDelta)
	scoreDelta := math.Abs(leftScore - rightScore)
	coverageGap := math.Abs(leftCoverage - rightCoverage)
	minCoverage := math.Min(leftCoverage, rightCoverage)

	// 片側だけが重なる材質ペアは近い方を先に描画して剥離を抑える。
	if coverageGap >= overlapAsymmetricCoverageGapMin && minCoverage < overlapAsymmetricMinCoverageMax {
		if absTransparencyDelta >= materialTransparencyOrderDelta {
			// 非対称重なりでは低透明率を優先しつつ、低透明側が大きく遠い場合のみ近傍側を優先する。
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
				return false, false
			}
			lowFartherDelta := lowScore - highScore
			chooseLow := lowFartherDelta <= asymLowTransFartherSwitchDelta
			if lowTransparency >= asymHighAlphaThreshold &&
				highTransparency >= asymHighAlphaThreshold &&
				(highTransparency-lowTransparency) >= asymHighAlphaGapSwitchDelta &&
				lowFartherDelta > 0 {
				chooseLow = false
			}
			if chooseLow {
				return lowIsLeft, true
			}
			return !lowIsLeft, true
		}
		if scoreDelta <= materialOrderScoreEpsilon || scoreDelta < minimumMaterialOrderDelta {
			return false, false
		}
		return leftScore < rightScore, true
	}

	// 重なりが極小なペアでは透明率を優先する。
	if minCoverage < veryLowCoverageTransparencyMax && absTransparencyDelta >= materialTransparencyOrderDelta {
		return leftTransparency < rightTransparency, true
	}

	// カバレッジが近い重なりは透明率差を優先する。
	if minCoverage >= tinyDepthFarFirstCoverageThreshold &&
		minCoverage < strongOverlapCoverageThreshold &&
		coverageGap <= balancedOverlapGapMax &&
		absTransparencyDelta >= balancedOverlapTransparencyMinDelta {
		return leftTransparency < rightTransparency, true
	}

	// 透明率が実質同一で密接に重なる場合は近い方を先に描画する。
	if absTransparencyDelta <= exactTransparencyDeltaThreshold && minCoverage >= strongOverlapCoverageThreshold {
		if scoreDelta <= materialOrderScoreEpsilon || scoreDelta < minimumMaterialOrderDelta {
			return false, false
		}
		return leftScore < rightScore, true
	}

	// 深度差が十分ある場合は遠い方を先に描画する。
	if scoreDelta >= materialDepthSwitchDelta {
		return leftScore > rightScore, true
	}

	// 強重なりで深度差が小さい場合は低透明率を先に描画する。
	if minCoverage >= strongOverlapCoverageThreshold && absTransparencyDelta >= materialTransparencyOrderDelta {
		return leftTransparency < rightTransparency, true
	}

	// 深度差が極小の場合は重なり量で遠方先行/近傍先行を切り替える。
	if scoreDelta < tinyDepthDeltaThreshold {
		if minCoverage >= tinyDepthFarFirstCoverageThreshold {
			return leftScore > rightScore, true
		}
		return leftScore < rightScore, true
	}

	if scoreDelta <= materialOrderScoreEpsilon || scoreDelta < minimumMaterialOrderDelta {
		return false, false
	}
	denominator := math.Max(math.Max(math.Abs(leftScore), math.Abs(rightScore)), materialOrderScoreEpsilon)
	relativeDelta := scoreDelta / denominator
	if relativeDelta < materialRelativeNearDelta {
		return leftScore < rightScore, true
	}
	return leftScore > rightScore, true
}

// collectMaterialSpatialInfos は材質ごとの点群とボディ距離情報を収集する。
func collectMaterialSpatialInfos(
	modelData *ModelData,
	faceRanges []materialFaceRange,
	materialIndexes []int,
	bodyPoints []mmath.Vec3,
) map[int]materialSpatialInfo {
	out := make(map[int]materialSpatialInfo, len(materialIndexes))
	if modelData == nil || len(bodyPoints) == 0 {
		return out
	}
	for _, materialIndex := range materialIndexes {
		if materialIndex < 0 || materialIndex >= len(faceRanges) {
			continue
		}
		sampledPoints := appendSampledMaterialVertices(
			modelData,
			faceRanges[materialIndex],
			make([]mmath.Vec3, 0, maxOverlapSampleVertices),
			maxOverlapSampleVertices,
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

// stableTopologicalSortByPriority は優先順位に従う安定トポロジカルソートを行う。
func stableTopologicalSortByPriority(adjacency [][]int, inDegree []int, priorities []int) []int {
	nodeCount := len(inDegree)
	if len(adjacency) != nodeCount || len(priorities) != nodeCount {
		return []int{}
	}

	available := make([]int, 0, nodeCount)
	processed := make([]bool, nodeCount)
	for nodeIndex := 0; nodeIndex < nodeCount; nodeIndex++ {
		if inDegree[nodeIndex] == 0 {
			available = appendPriorityNode(available, nodeIndex, priorities)
		}
	}

	result := make([]int, 0, nodeCount)
	for len(result) < nodeCount {
		if len(available) == 0 {
			// サイクルがある場合は、未処理ノードのうち入次数が最小のノードから順に崩して進める。
			candidate := -1
			candidateInDegree := math.MaxInt
			for nodeIndex := 0; nodeIndex < nodeCount; nodeIndex++ {
				if processed[nodeIndex] {
					continue
				}
				if inDegree[nodeIndex] < candidateInDegree {
					candidate = nodeIndex
					candidateInDegree = inDegree[nodeIndex]
					continue
				}
				if inDegree[nodeIndex] == candidateInDegree {
					if candidate < 0 || priorities[nodeIndex] < priorities[candidate] ||
						(priorities[nodeIndex] == priorities[candidate] && nodeIndex < candidate) {
						candidate = nodeIndex
					}
				}
			}
			if candidate < 0 {
				break
			}
			available = appendPriorityNode(available, candidate, priorities)
		}

		nodeIndex := available[0]
		available = available[1:]
		if processed[nodeIndex] {
			continue
		}
		processed[nodeIndex] = true
		result = append(result, nodeIndex)

		for _, next := range adjacency[nodeIndex] {
			inDegree[next]--
			if inDegree[next] != 0 || processed[next] {
				continue
			}
			available = appendPriorityNode(available, next, priorities)
		}
	}
	if len(result) != nodeCount {
		return []int{}
	}
	return result
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
		textureAlphaCache[textureIndex] = textureAlphaCacheEntry{checked: true, transparent: false, transparentRatio: 0}
		return false
	}

	modelPath := strings.TrimSpace(modelData.Path())
	textureName := strings.TrimSpace(textureData.Name())
	if modelPath == "" || textureName == "" {
		textureAlphaCache[textureIndex] = textureAlphaCacheEntry{checked: true, transparent: false, transparentRatio: 0}
		return false
	}
	texturePath := filepath.Join(filepath.Dir(modelPath), normalizeTextureRelativePath(textureName))
	transparent, ratio, err := detectTextureTransparency(texturePath, textureAlphaTransparentThreshold)
	if err != nil {
		textureAlphaCache[textureIndex] = textureAlphaCacheEntry{checked: true, transparent: false, transparentRatio: 0}
		return false
	}
	textureAlphaCache[textureIndex] = textureAlphaCacheEntry{checked: true, transparent: transparent, transparentRatio: ratio}
	return transparent
}

// normalizeTextureRelativePath は相対パス区切りを現在OS向けに正規化する。
func normalizeTextureRelativePath(path string) string {
	replaced := strings.ReplaceAll(path, "\\", string(os.PathSeparator))
	replaced = strings.ReplaceAll(replaced, "/", string(os.PathSeparator))
	return filepath.Clean(replaced)
}

// detectTextureTransparency はテクスチャ画像のアルファを走査して透明領域の有無と割合を返す。
func detectTextureTransparency(texturePath string, threshold float64) (bool, float64, error) {
	file, err := os.Open(texturePath)
	if err != nil {
		return false, 0, err
	}
	defer func() {
		_ = file.Close()
	}()

	img, _, err := image.Decode(file)
	if err != nil {
		return false, 0, err
	}
	bounds := img.Bounds()
	if bounds.Empty() {
		return false, 0, nil
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
		return false, 0, nil
	}
	ratio := float64(transparentPixels) / float64(totalPixels)
	return transparentPixels > 0, ratio, nil
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
	bodyMaterialIndex := detectBodyMaterialIndex(modelData, bodyBoneIndexes)
	if bodyMaterialIndex >= 0 && bodyMaterialIndex < len(faceRanges) {
		points := appendSampledMaterialVertices(
			modelData,
			faceRanges[bodyMaterialIndex],
			make([]mmath.Vec3, 0, maxBodyPointCount),
			maxBodyPointCount,
		)
		if len(points) > 0 {
			return points
		}
	}
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
