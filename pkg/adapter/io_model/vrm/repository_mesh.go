// 指示: miu200521358
package vrm

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/miu200521358/mlib_go/pkg/adapter/io_common"
	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
	"gonum.org/v1/gonum/spatial/r3"
)

const (
	gltfComponentTypeByte          = 5120
	gltfComponentTypeUnsignedByte  = 5121
	gltfComponentTypeShort         = 5122
	gltfComponentTypeUnsignedShort = 5123
	gltfComponentTypeUnsignedInt   = 5125
	gltfComponentTypeFloat         = 5126

	gltfPrimitiveModePoints        = 0
	gltfPrimitiveModeLines         = 1
	gltfPrimitiveModeLineLoop      = 2
	gltfPrimitiveModeLineStrip     = 3
	gltfPrimitiveModeTriangles     = 4
	gltfPrimitiveModeTriangleStrip = 5
	gltfPrimitiveModeTriangleFan   = 6

	vroidMeterScale = 12.5
)

// vrmConversion はVRM->PMX変換時の座標設定を表す。
type vrmConversion struct {
	Scale          float64
	Axis           mmath.Vec3
	ReverseWinding bool
}

// accessorReadPlan はaccessorの読み取り計画を表す。
type accessorReadPlan struct {
	Accessor      gltfAccessor
	ComponentSize int
	ComponentNum  int
	Stride        int
	BaseOffset    int
	ViewStart     int
	ViewEnd       int
}

// accessorValueCache はaccessor値の再読込みを抑止するキャッシュ。
type accessorValueCache struct {
	floatValues  map[int][][]float64
	intValues    map[int][][]int
	vertexRanges map[string]primitiveVertexRange
}

// primitiveVertexRange はprimitiveで生成した頂点範囲を表す。
type primitiveVertexRange struct {
	Start int
	Count int
}

// weightedBone はボーンウェイト計算用の一時構造体。
type weightedBone struct {
	BoneIndex int
	Weight    float64
}

// nodeTargetKey は VRM1 expression bind の node/index を識別する。
type nodeTargetKey struct {
	NodeIndex   int
	TargetIndex int
}

// meshTargetKey は VRM0 blendShape bind の mesh/index を識別する。
type meshTargetKey struct {
	MeshIndex   int
	TargetIndex int
}

// targetMorphRegistry は morph target 由来モーフの参照表を表す。
type targetMorphRegistry struct {
	ByNodeAndTarget map[nodeTargetKey]int
	ByMeshAndTarget map[meshTargetKey]int
	ByGltfMaterial  map[int][]int
	ByMaterialName  map[string][]int
}

// buildVrmConversion はVRMプロファイルに応じた座標変換設定を返す。
func buildVrmConversion(vrmData *vrm.VrmData) vrmConversion {
	conversion := vrmConversion{
		Scale: vroidMeterScale,
		Axis: mmath.Vec3{
			Vec: r3.Vec{X: -1.0, Y: 1.0, Z: 1.0},
		},
	}
	if vrmData != nil && vrmData.Profile == vrm.VRM_PROFILE_VROID {
		// VRoidはVRM0/VRM1で座標配置が異なるため、バージョンごとに軸変換を切り替える。
		// VRM0: 既存規約どおり X 反転（-1, 1, 1）
		// VRM1: VRoid Studio 1.x 出力の前向きを保つため標準軸（1, 1, -1）
		if vrmData.Version == vrm.VRM_VERSION_1 {
			conversion.Axis = mmath.Vec3{
				Vec: r3.Vec{X: 1.0, Y: 1.0, Z: -1.0},
			}
		}
	}
	conversion.ReverseWinding = conversion.Axis.X*conversion.Axis.Y*conversion.Axis.Z < 0
	return conversion
}

// convertVrmNormalToPmx は法線をVRM座標系からPMX座標系へ変換する。
func convertVrmNormalToPmx(v mmath.Vec3, conversion vrmConversion) mmath.Vec3 {
	converted := mmath.Vec3{
		Vec: r3.Vec{
			X: v.X * conversion.Axis.X,
			Y: v.Y * conversion.Axis.Y,
			Z: v.Z * conversion.Axis.Z,
		},
	}
	return converted.Normalized()
}

// appendMeshData はglTF mesh/primitives からPMXの頂点・面・材質を生成する。
func appendMeshData(
	modelData *model.PmxModel,
	doc *gltfDocument,
	binChunk []byte,
	nodeToBoneIndex map[int]int,
	conversion vrmConversion,
	progressReporter func(LoadProgressEvent),
) (*targetMorphRegistry, error) {
	if modelData == nil || doc == nil || len(doc.Meshes) == 0 {
		return newTargetMorphRegistry(), nil
	}

	textureIndexesByImage := appendImageTextures(modelData, doc.Images)
	totalPrimitives := countGltfPrimitives(doc.Meshes)
	cache := newAccessorValueCache()
	targetMorphRegistry := newTargetMorphRegistry()
	primitiveStep := 0
	meshUniquePrimitiveIndex := map[int]map[string]int{}
	logVrmInfo(
		"VRMメッシュ変換開始: nodes=%d meshes=%d primitives=%d textures=%d",
		len(doc.Nodes),
		len(doc.Meshes),
		totalPrimitives,
		len(textureIndexesByImage),
	)

	for nodeIndex, node := range doc.Nodes {
		if node.Mesh == nil {
			continue
		}
		meshIndex := *node.Mesh
		if meshIndex < 0 || meshIndex >= len(doc.Meshes) {
			return targetMorphRegistry, io_common.NewIoParseFailed("node.mesh のindexが不正です: %d", nil, meshIndex)
		}

		mesh := doc.Meshes[meshIndex]
		logVrmInfo(
			"VRMメッシュ変換ステップ: node=%d mesh=%d primitives=%d",
			nodeIndex,
			meshIndex,
			len(mesh.Primitives),
		)
		seenByKey := meshUniquePrimitiveIndex[meshIndex]
		if seenByKey == nil {
			seenByKey = map[string]int{}
			meshUniquePrimitiveIndex[meshIndex] = seenByKey
		}
		for primitiveIndex, primitive := range mesh.Primitives {
			primitiveStep++
			primitiveName := resolvePrimitiveName(mesh, meshIndex, primitiveIndex)
			if shouldSkipPrimitiveForUnsupportedTargets(primitive, primitiveIndex, seenByKey) {
				logVrmDebug(
					"VRMプリミティブ変換スキップ: step=%d/%d node=%d mesh=%d primitive=%d name=%s reason=%s",
					primitiveStep,
					totalPrimitives,
					nodeIndex,
					meshIndex,
					primitiveIndex,
					primitiveName,
					"morph targets重複base primitiveを省略",
				)
				if progressReporter != nil {
					progressReporter(LoadProgressEvent{
						Type:           LoadProgressEventTypePrimitiveProcessed,
						PrimitiveTotal: totalPrimitives,
						PrimitiveDone:  primitiveStep,
					})
				}
				continue
			}
			beforeVertexCount := modelData.Vertices.Len()
			beforeFaceCount := modelData.Faces.Len()
			beforeMaterialCount := modelData.Materials.Len()
			logVrmDebug(
				"VRMプリミティブ変換開始: step=%d/%d node=%d mesh=%d primitive=%d name=%s",
				primitiveStep,
				totalPrimitives,
				nodeIndex,
				meshIndex,
				primitiveIndex,
				primitiveName,
			)
			if err := appendPrimitiveMeshData(
				modelData,
				doc,
				binChunk,
				nodeIndex,
				meshIndex,
				node,
				primitive,
				primitiveName,
				nodeToBoneIndex,
				textureIndexesByImage,
				conversion,
				cache,
				targetMorphRegistry,
			); err != nil {
				return targetMorphRegistry, err
			}
			logVrmDebug(
				"VRMプリミティブ変換完了: step=%d/%d node=%d mesh=%d primitive=%d name=%s addVertices=%d addFaces=%d addMaterials=%d",
				primitiveStep,
				totalPrimitives,
				nodeIndex,
				meshIndex,
				primitiveIndex,
				primitiveName,
				modelData.Vertices.Len()-beforeVertexCount,
				modelData.Faces.Len()-beforeFaceCount,
				modelData.Materials.Len()-beforeMaterialCount,
			)
			if progressReporter != nil {
				progressReporter(LoadProgressEvent{
					Type:           LoadProgressEventTypePrimitiveProcessed,
					PrimitiveTotal: totalPrimitives,
					PrimitiveDone:  primitiveStep,
				})
			}
		}
	}
	logVrmInfo(
		"VRMメッシュ変換完了: vertices=%d faces=%d materials=%d textures=%d",
		modelData.Vertices.Len(),
		modelData.Faces.Len(),
		modelData.Materials.Len(),
		modelData.Textures.Len(),
	)
	return targetMorphRegistry, nil
}

// appendImageTextures はglTF image配列に対応するPMX Textureを作成する。
func appendImageTextures(modelData *model.PmxModel, images []gltfImage) []int {
	textureIndexes := make([]int, len(images))
	for imageIndex, image := range images {
		texture := model.NewTexture()
		texture.SetName(resolveImageTextureName(image, imageIndex))
		texture.SetValid(true)
		textureIndexes[imageIndex] = modelData.Textures.AppendRaw(texture)
	}
	return textureIndexes
}

// resolveImageTextureName はimage要素からPMX用テクスチャ名を決定する。
func resolveImageTextureName(image gltfImage, imageIndex int) string {
	uri := strings.TrimSpace(image.URI)
	if uri != "" && !strings.HasPrefix(uri, "data:") {
		return filepath.Base(filepath.FromSlash(uri))
	}
	name := strings.TrimSpace(image.Name)
	if name != "" {
		return name
	}
	return fmt.Sprintf("image_%03d", imageIndex)
}

// resolvePrimitiveName はメッシュ名とindexから材質名の基本名を生成する。
func resolvePrimitiveName(mesh gltfMesh, meshIndex int, primitiveIndex int) string {
	meshName := strings.TrimSpace(mesh.Name)
	if meshName == "" {
		meshName = fmt.Sprintf("mesh_%03d", meshIndex)
	}
	return fmt.Sprintf("%s_%03d", meshName, primitiveIndex)
}

// shouldSkipPrimitiveForUnsupportedTargets は同一base primitive の重複展開要否を判定する。
func shouldSkipPrimitiveForUnsupportedTargets(
	primitive gltfPrimitive,
	primitiveIndex int,
	seenByKey map[string]int,
) bool {
	if seenByKey == nil {
		return false
	}
	key := primitiveBaseKey(primitive)
	if key == "" {
		return false
	}
	if firstIndex, exists := seenByKey[key]; exists {
		// 同一base primitiveの重複展開を抑止する。
		return len(primitive.Targets) > 0 && firstIndex != primitiveIndex
	}
	seenByKey[key] = primitiveIndex
	return false
}

// primitiveBaseKey はtargetsを除くprimitiveの識別キーを返す。
func primitiveBaseKey(primitive gltfPrimitive) string {
	attributesKey := primitiveAttributesKey(primitive.Attributes)
	if attributesKey == "" {
		return ""
	}
	indices := -1
	if primitive.Indices != nil {
		indices = *primitive.Indices
	}
	material := -1
	if primitive.Material != nil {
		material = *primitive.Material
	}
	mode := gltfPrimitiveModeTriangles
	if primitive.Mode != nil {
		mode = *primitive.Mode
	}
	return fmt.Sprintf("attr=%s|idx=%d|mat=%d|mode=%d", attributesKey, indices, material, mode)
}

// primitiveAttributesKey はattribute mapの安定キーを生成する。
func primitiveAttributesKey(attributes map[string]int) string {
	if len(attributes) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", key, attributes[key]))
	}
	return strings.Join(parts, ",")
}

// primitiveVertexKey は頂点配列を再利用するためのキーを返す。
func primitiveVertexKey(nodeIndex int, primitive gltfPrimitive) string {
	return fmt.Sprintf("node=%d|attrs=%s", nodeIndex, primitiveAttributesKey(primitive.Attributes))
}

// newAccessorValueCache はaccessorキャッシュを初期化する。
func newAccessorValueCache() *accessorValueCache {
	return &accessorValueCache{
		floatValues:  map[int][][]float64{},
		intValues:    map[int][][]int{},
		vertexRanges: map[string]primitiveVertexRange{},
	}
}

// readFloatValues はfloat accessor値をキャッシュ付きで返す。
func (c *accessorValueCache) readFloatValues(doc *gltfDocument, accessorIndex int, binChunk []byte) ([][]float64, error) {
	if c == nil {
		return readAccessorFloatValues(doc, accessorIndex, binChunk)
	}
	if values, ok := c.floatValues[accessorIndex]; ok {
		return values, nil
	}
	values, err := readAccessorFloatValues(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, err
	}
	c.floatValues[accessorIndex] = values
	return values, nil
}

// readIntValues はint accessor値をキャッシュ付きで返す。
func (c *accessorValueCache) readIntValues(doc *gltfDocument, accessorIndex int, binChunk []byte) ([][]int, error) {
	if c == nil {
		return readAccessorIntValues(doc, accessorIndex, binChunk)
	}
	if values, ok := c.intValues[accessorIndex]; ok {
		return values, nil
	}
	values, err := readAccessorIntValues(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, err
	}
	c.intValues[accessorIndex] = values
	return values, nil
}

// getVertexRange は頂点範囲キャッシュから開始indexを取得する。
func (c *accessorValueCache) getVertexRange(key string, expectedCount int) (int, bool) {
	if c == nil || strings.TrimSpace(key) == "" {
		return 0, false
	}
	r, ok := c.vertexRanges[key]
	if !ok {
		return 0, false
	}
	if r.Count != expectedCount {
		return 0, false
	}
	return r.Start, true
}

// setVertexRange は頂点範囲キャッシュを登録する。
func (c *accessorValueCache) setVertexRange(key string, start int, count int) {
	if c == nil || strings.TrimSpace(key) == "" || start < 0 || count <= 0 {
		return
	}
	c.vertexRanges[key] = primitiveVertexRange{
		Start: start,
		Count: count,
	}
}

// appendPrimitiveMeshData はprimitiveをPMX頂点・面・材質へ変換する。
func appendPrimitiveMeshData(
	modelData *model.PmxModel,
	doc *gltfDocument,
	binChunk []byte,
	nodeIndex int,
	meshIndex int,
	node gltfNode,
	primitive gltfPrimitive,
	primitiveName string,
	nodeToBoneIndex map[int]int,
	textureIndexesByImage []int,
	conversion vrmConversion,
	cache *accessorValueCache,
	targetMorphRegistry *targetMorphRegistry,
) error {
	positionAccessor, ok := primitive.Attributes["POSITION"]
	if !ok {
		return io_common.NewIoParseFailed("mesh.primitive に POSITION がありません", nil)
	}

	positions, err := cache.readFloatValues(doc, positionAccessor, binChunk)
	if err != nil {
		return io_common.NewIoParseFailed("POSITION属性の読み取りに失敗しました(accessor=%d)", err, positionAccessor)
	}
	if len(positions) == 0 {
		return nil
	}

	normals, err := readOptionalFloatAttribute(doc, primitive.Attributes, "NORMAL", binChunk, cache)
	if err != nil {
		logVrmWarn(
			"VRM法線の読み取りに失敗したため既定法線で継続します: node=%d primitive=%s err=%s",
			nodeIndex,
			primitiveName,
			err.Error(),
		)
		normals = nil
	}
	uvs, err := readOptionalFloatAttribute(doc, primitive.Attributes, "TEXCOORD_0", binChunk, cache)
	if err != nil {
		return err
	}
	joints, err := readOptionalIntAttribute(doc, primitive.Attributes, "JOINTS_0", binChunk, cache)
	if err != nil {
		return err
	}
	weights, err := readOptionalFloatAttribute(doc, primitive.Attributes, "WEIGHTS_0", binChunk, cache)
	if err != nil {
		return err
	}

	indices, err := readPrimitiveIndices(doc, primitive, len(positions), binChunk, cache)
	if err != nil {
		return err
	}
	mode := gltfPrimitiveModeTriangles
	if primitive.Mode != nil {
		mode = *primitive.Mode
	}
	triangles := triangulateIndices(indices, mode)
	if len(triangles) == 0 {
		return nil
	}

	vertexStart, appendedVertices := appendOrReusePrimitiveVertices(
		modelData,
		doc,
		nodeIndex,
		node,
		primitive,
		positions,
		normals,
		uvs,
		joints,
		weights,
		nodeToBoneIndex,
		conversion,
		cache,
	)
	if appendedVertices == 0 {
		logVrmDebug(
			"VRM頂点再利用: node=%d primitive=%s vertexCount=%d",
			nodeIndex,
			primitiveName,
			len(positions),
		)
	}
	appendPrimitiveTargetMorphs(
		modelData,
		doc,
		binChunk,
		nodeIndex,
		meshIndex,
		primitive,
		vertexStart,
		len(positions),
		conversion,
		cache,
		appendedVertices > 0,
		targetMorphRegistry,
	)

	materialIndex := appendPrimitiveMaterial(
		modelData,
		doc,
		primitive,
		primitiveName,
		textureIndexesByImage,
		len(triangles)*3,
		targetMorphRegistry,
	)
	for _, tri := range triangles {
		if tri[0] < 0 || tri[1] < 0 || tri[2] < 0 {
			continue
		}
		if tri[0] >= len(positions) || tri[1] >= len(positions) || tri[2] >= len(positions) {
			return io_common.NewIoParseFailed("indices が頂点数を超えています", nil)
		}

		v0 := vertexStart + tri[0]
		v1 := vertexStart + tri[1]
		v2 := vertexStart + tri[2]
		if conversion.ReverseWinding {
			v0, v2 = v2, v0
		}
		face := &model.Face{
			VertexIndexes: [3]int{v0, v1, v2},
		}
		modelData.Faces.AppendRaw(face)
		appendVertexMaterialIndex(modelData, v0, materialIndex)
		appendVertexMaterialIndex(modelData, v1, materialIndex)
		appendVertexMaterialIndex(modelData, v2, materialIndex)
	}
	return nil
}

// newTargetMorphRegistry は morph target 参照表を初期化する。
func newTargetMorphRegistry() *targetMorphRegistry {
	return &targetMorphRegistry{
		ByNodeAndTarget: map[nodeTargetKey]int{},
		ByMeshAndTarget: map[meshTargetKey]int{},
		ByGltfMaterial:  map[int][]int{},
		ByMaterialName:  map[string][]int{},
	}
}

// appendPrimitiveTargetMorphs は primitive.targets を PMX 頂点モーフとして取り込む。
func appendPrimitiveTargetMorphs(
	modelData *model.PmxModel,
	doc *gltfDocument,
	binChunk []byte,
	nodeIndex int,
	meshIndex int,
	primitive gltfPrimitive,
	vertexStart int,
	vertexCount int,
	conversion vrmConversion,
	cache *accessorValueCache,
	appendOffsets bool,
	registry *targetMorphRegistry,
) {
	if modelData == nil || doc == nil || cache == nil || registry == nil || len(primitive.Targets) == 0 {
		return
	}
	targetNames := buildPrimitiveTargetNames(primitive, len(primitive.Targets))
	for targetIndex, target := range primitive.Targets {
		targetName := targetNames[targetIndex]
		morphName := resolvePrimitiveTargetMorphName(meshIndex, targetIndex, targetName)
		morphData, morphIndex := ensureVertexTargetMorph(modelData, morphName)
		if morphData == nil || morphIndex < 0 {
			continue
		}
		registerTargetMorphIndex(registry, nodeIndex, meshIndex, targetIndex, morphIndex)

		if !appendOffsets {
			continue
		}
		positionAccessor, exists := target["POSITION"]
		if !exists {
			continue
		}
		positionDeltas, err := cache.readFloatValues(doc, positionAccessor, binChunk)
		if err != nil {
			logVrmWarn(
				"VRMモーフターゲットの読み取りに失敗したため継続します: node=%d mesh=%d target=%d accessor=%d err=%s",
				nodeIndex,
				meshIndex,
				targetIndex,
				positionAccessor,
				err.Error(),
			)
			continue
		}
		limit := len(positionDeltas)
		if vertexCount < limit {
			limit = vertexCount
		}
		for deltaIndex := 0; deltaIndex < limit; deltaIndex++ {
			positionDelta := toVec3(positionDeltas[deltaIndex], mmath.ZERO_VEC3)
			convertedDelta := convertVrmPositionToPmx(positionDelta, conversion)
			if isZeroMorphDelta(convertedDelta) {
				continue
			}
			morphData.Offsets = append(morphData.Offsets, &model.VertexMorphOffset{
				VertexIndex: vertexStart + deltaIndex,
				Position:    convertedDelta,
			})
		}
	}
}

// buildPrimitiveTargetNames は target 数に合わせた morph 名配列を返す。
func buildPrimitiveTargetNames(primitive gltfPrimitive, count int) []string {
	names := make([]string, count)
	for targetIndex := 0; targetIndex < count; targetIndex++ {
		names[targetIndex] = fmt.Sprintf("target_%03d", targetIndex)
	}
	if primitive.Extras == nil || len(primitive.Extras.TargetNames) == 0 {
		return names
	}
	limit := len(primitive.Extras.TargetNames)
	if count < limit {
		limit = count
	}
	for targetIndex := 0; targetIndex < limit; targetIndex++ {
		name := strings.TrimSpace(primitive.Extras.TargetNames[targetIndex])
		if name == "" {
			continue
		}
		names[targetIndex] = name
	}
	return names
}

// resolvePrimitiveTargetMorphName は内部用の morph 名を返す。
func resolvePrimitiveTargetMorphName(meshIndex int, targetIndex int, targetName string) string {
	normalizedName := strings.TrimSpace(targetName)
	if normalizedName == "" {
		normalizedName = fmt.Sprintf("target_%03d", targetIndex)
	}
	return fmt.Sprintf("__vrm_target_m%03d_t%03d_%s", meshIndex, targetIndex, normalizedName)
}

// ensureVertexTargetMorph は内部頂点モーフを取得または作成する。
func ensureVertexTargetMorph(modelData *model.PmxModel, morphName string) (*model.Morph, int) {
	if modelData == nil || modelData.Morphs == nil {
		return nil, -1
	}
	if existing, err := modelData.Morphs.GetByName(morphName); err == nil && existing != nil {
		return existing, existing.Index()
	}
	morphData := &model.Morph{
		Panel:     model.MORPH_PANEL_SYSTEM,
		MorphType: model.MORPH_TYPE_VERTEX,
		Offsets:   []model.IMorphOffset{},
		IsSystem:  true,
	}
	morphData.SetName(morphName)
	morphData.EnglishName = morphName
	morphIndex := modelData.Morphs.AppendRaw(morphData)
	return morphData, morphIndex
}

// registerTargetMorphIndex は node/index と mesh/index の参照を更新する。
func registerTargetMorphIndex(
	registry *targetMorphRegistry,
	nodeIndex int,
	meshIndex int,
	targetIndex int,
	morphIndex int,
) {
	if registry == nil || targetIndex < 0 || morphIndex < 0 {
		return
	}
	registry.ByNodeAndTarget[nodeTargetKey{NodeIndex: nodeIndex, TargetIndex: targetIndex}] = morphIndex
	registry.ByMeshAndTarget[meshTargetKey{MeshIndex: meshIndex, TargetIndex: targetIndex}] = morphIndex
}

// isZeroMorphDelta はオフセットが実質 0 か判定する。
func isZeroMorphDelta(v mmath.Vec3) bool {
	const epsilon = 1e-9
	return math.Abs(v.X) <= epsilon && math.Abs(v.Y) <= epsilon && math.Abs(v.Z) <= epsilon
}

// appendExpressionMorphsFromVrmDefinition は VRM 定義表情を PMX モーフへ反映する。
func appendExpressionMorphsFromVrmDefinition(
	modelData *model.PmxModel,
	doc *gltfDocument,
	registry *targetMorphRegistry,
) {
	if modelData == nil || modelData.Morphs == nil || doc == nil || registry == nil || doc.Extensions == nil {
		return
	}
	if raw, exists := doc.Extensions["VRMC_vrm"]; exists {
		applyVrm1ExpressionMorphs(modelData, raw, registry)
		return
	}
	if raw, exists := doc.Extensions["VRM"]; exists {
		applyVrm0BlendShapeMorphs(modelData, raw, registry)
	}
}

// vrm1MorphExpressionsSource は VRM1 expressions の最小構造を表す。
type vrm1MorphExpressionsSource struct {
	Expressions vrm1ExpressionsSource `json:"expressions"`
}

// vrm1ExpressionsSource は VRM1 preset/custom 表情セットを表す。
type vrm1ExpressionsSource struct {
	Preset map[string]vrm1ExpressionSource `json:"preset"`
	Custom map[string]vrm1ExpressionSource `json:"custom"`
}

// vrm1ExpressionSource は VRM1 expression の最小構造を表す。
type vrm1ExpressionSource struct {
	IsBinary              bool                            `json:"isBinary"`
	MorphTargetBinds      []vrm1ExpressionMorphBindEntry  `json:"morphTargetBinds"`
	MaterialColorBinds    []vrm1MaterialColorBindEntry    `json:"materialColorBinds"`
	TextureTransformBinds []vrm1TextureTransformBindEntry `json:"textureTransformBinds"`
}

// vrm1ExpressionMorphBindEntry は VRM1 expression の morphTargetBind を表す。
type vrm1ExpressionMorphBindEntry struct {
	Node   int     `json:"node"`
	Index  int     `json:"index"`
	Weight float64 `json:"weight"`
}

// vrm1MaterialColorBindEntry は VRM1 expression の materialColorBind を表す。
type vrm1MaterialColorBindEntry struct {
	Material    int       `json:"material"`
	Type        string    `json:"type"`
	TargetValue []float64 `json:"targetValue"`
}

// vrm1TextureTransformBindEntry は VRM1 expression の textureTransformBind を表す。
type vrm1TextureTransformBindEntry struct {
	Material int       `json:"material"`
	Scale    []float64 `json:"scale"`
	Offset   []float64 `json:"offset"`
}

// applyVrm1ExpressionMorphs は VRM1 expressions を PMX モーフへ変換する。
func applyVrm1ExpressionMorphs(modelData *model.PmxModel, raw json.RawMessage, registry *targetMorphRegistry) {
	if modelData == nil || modelData.Morphs == nil || len(raw) == 0 || registry == nil {
		return
	}
	source := vrm1MorphExpressionsSource{}
	if err := json.Unmarshal(raw, &source); err != nil {
		logVrmWarn("VRM1表情定義の解析に失敗したため継続します: err=%s", err.Error())
		return
	}
	presetKeys := sortedStringKeys(source.Expressions.Preset)
	for _, key := range presetKeys {
		entry := source.Expressions.Preset[key]
		expressionName := strings.TrimSpace(key)
		if expressionName == "" {
			continue
		}
		upsertExpressionMorphFromVrm1Entry(
			modelData,
			expressionName,
			resolveExpressionPanel(expressionName),
			entry,
			registry,
		)
	}
	customKeys := sortedStringKeys(source.Expressions.Custom)
	for _, key := range customKeys {
		entry := source.Expressions.Custom[key]
		expressionName := strings.TrimSpace(key)
		if expressionName == "" {
			continue
		}
		upsertExpressionMorphFromVrm1Entry(
			modelData,
			expressionName,
			resolveExpressionPanel(expressionName),
			entry,
			registry,
		)
	}
}

// upsertExpressionMorphFromVrm1Entry は VRM1 expression 要素を PMX モーフへ反映する。
func upsertExpressionMorphFromVrm1Entry(
	modelData *model.PmxModel,
	expressionName string,
	panel model.MorphPanel,
	entry vrm1ExpressionSource,
	registry *targetMorphRegistry,
) {
	if modelData == nil || modelData.Morphs == nil || strings.TrimSpace(expressionName) == "" || registry == nil {
		return
	}
	weightsByMorph := buildWeightsByMorphFromNodeBinds(entry.MorphTargetBinds, registry, entry.IsBinary)
	vertexOffsets := buildVertexOffsetsFromWeights(modelData, weightsByMorph)
	materialOffsets := buildMaterialOffsetsFromVrm1ColorBinds(modelData, entry.MaterialColorBinds, registry)
	uvOffsets := buildUvOffsetsFromVrm1TextureTransformBinds(modelData, entry.TextureTransformBinds, registry)
	upsertExpressionMorph(modelData, expressionName, panel, vertexOffsets, materialOffsets, uvOffsets)
}

// vrm0BlendShapeSource は VRM0 blendShapeMaster の最小構造を表す。
type vrm0BlendShapeSource struct {
	BlendShapeMaster vrm0BlendShapeMasterSource `json:"blendShapeMaster"`
}

// vrm0BlendShapeMasterSource は VRM0 blendShapeGroups を表す。
type vrm0BlendShapeMasterSource struct {
	BlendShapeGroups []vrm0BlendShapeGroupSource `json:"blendShapeGroups"`
}

// vrm0BlendShapeGroupSource は VRM0 blendShapeGroups 要素を表す。
type vrm0BlendShapeGroupSource struct {
	Name           string                     `json:"name"`
	PresetName     string                     `json:"presetName"`
	Binds          []vrm0BlendShapeBindRaw    `json:"binds"`
	MaterialValues []vrm0MaterialValueBindRaw `json:"materialValues"`
}

// vrm0BlendShapeBindRaw は VRM0 bind 要素を表す。
type vrm0BlendShapeBindRaw struct {
	Mesh   int     `json:"mesh"`
	Index  int     `json:"index"`
	Weight float64 `json:"weight"`
}

// vrm0MaterialValueBindRaw は VRM0 materialValues 要素を表す。
type vrm0MaterialValueBindRaw struct {
	MaterialName string    `json:"materialName"`
	PropertyName string    `json:"propertyName"`
	TargetValue  []float64 `json:"targetValue"`
}

// applyVrm0BlendShapeMorphs は VRM0 blendShapeGroups を PMX モーフへ変換する。
func applyVrm0BlendShapeMorphs(modelData *model.PmxModel, raw json.RawMessage, registry *targetMorphRegistry) {
	if modelData == nil || modelData.Morphs == nil || len(raw) == 0 || registry == nil {
		return
	}
	source := vrm0BlendShapeSource{}
	if err := json.Unmarshal(raw, &source); err != nil {
		logVrmWarn("VRM0表情定義の解析に失敗したため継続します: err=%s", err.Error())
		return
	}
	for _, group := range source.BlendShapeMaster.BlendShapeGroups {
		expressionName := strings.TrimSpace(group.Name)
		if expressionName == "" {
			expressionName = strings.TrimSpace(group.PresetName)
		}
		if expressionName == "" {
			continue
		}
		upsertExpressionMorphFromVrm0Group(
			modelData,
			expressionName,
			resolveExpressionPanel(expressionName),
			group,
			registry,
		)
	}
}

// upsertExpressionMorphFromVrm0Group は VRM0 blendShapeGroup を PMX モーフへ反映する。
func upsertExpressionMorphFromVrm0Group(
	modelData *model.PmxModel,
	expressionName string,
	panel model.MorphPanel,
	group vrm0BlendShapeGroupSource,
	registry *targetMorphRegistry,
) {
	if modelData == nil || modelData.Morphs == nil || strings.TrimSpace(expressionName) == "" || registry == nil {
		return
	}
	weightsByMorph := buildWeightsByMorphFromMeshBinds(group.Binds, registry)
	vertexOffsets := buildVertexOffsetsFromWeights(modelData, weightsByMorph)
	materialOffsets := buildMaterialOffsetsFromVrm0MaterialValues(modelData, group.MaterialValues, registry)
	uvOffsets := buildUvOffsetsFromVrm0MaterialValues(modelData, group.MaterialValues, registry)
	upsertExpressionMorph(modelData, expressionName, panel, vertexOffsets, materialOffsets, uvOffsets)
}

// buildWeightsByMorphFromNodeBinds は VRM1 node/index bind を morph index 係数へ変換する。
func buildWeightsByMorphFromNodeBinds(
	binds []vrm1ExpressionMorphBindEntry,
	registry *targetMorphRegistry,
	isBinary bool,
) map[int]float64 {
	if registry == nil {
		return nil
	}
	weightsByMorph := map[int]float64{}
	for _, bind := range binds {
		morphIndex, exists := registry.ByNodeAndTarget[nodeTargetKey{
			NodeIndex:   bind.Node,
			TargetIndex: bind.Index,
		}]
		if !exists {
			continue
		}
		weightsByMorph[morphIndex] += normalizeMorphBindWeight(bind.Weight, isBinary)
	}
	return weightsByMorph
}

// buildWeightsByMorphFromMeshBinds は VRM0 mesh/index bind を morph index 係数へ変換する。
func buildWeightsByMorphFromMeshBinds(
	binds []vrm0BlendShapeBindRaw,
	registry *targetMorphRegistry,
) map[int]float64 {
	if registry == nil {
		return nil
	}
	weightsByMorph := map[int]float64{}
	for _, bind := range binds {
		morphIndex, exists := registry.ByMeshAndTarget[meshTargetKey{
			MeshIndex:   bind.Mesh,
			TargetIndex: bind.Index,
		}]
		if !exists {
			continue
		}
		weightsByMorph[morphIndex] += normalizeMorphBindWeight(bind.Weight, false)
	}
	return weightsByMorph
}

// buildVertexOffsetsFromWeights は morph index 係数から頂点オフセット一覧を生成する。
func buildVertexOffsetsFromWeights(
	modelData *model.PmxModel,
	weightsByMorph map[int]float64,
) []model.IMorphOffset {
	if modelData == nil || modelData.Morphs == nil {
		return nil
	}
	if len(weightsByMorph) == 0 {
		return nil
	}
	morphIndexes := make([]int, 0, len(weightsByMorph))
	for morphIndex := range weightsByMorph {
		morphIndexes = append(morphIndexes, morphIndex)
	}
	sort.Ints(morphIndexes)
	offsetsByVertex := map[int]mmath.Vec3{}
	for _, morphIndex := range morphIndexes {
		weight := weightsByMorph[morphIndex]
		if weight <= 0 {
			continue
		}
		sourceMorph, err := modelData.Morphs.Get(morphIndex)
		if err != nil || sourceMorph == nil || sourceMorph.MorphType != model.MORPH_TYPE_VERTEX {
			continue
		}
		appendWeightedVertexOffsets(offsetsByVertex, sourceMorph.Offsets, weight)
	}
	return buildMergedVertexOffsets(offsetsByVertex)
}

// upsertExpressionMorph は表情名のモーフを頂点/材質オフセット構成で更新する。
func upsertExpressionMorph(
	modelData *model.PmxModel,
	expressionName string,
	panel model.MorphPanel,
	vertexOffsets []model.IMorphOffset,
	materialOffsets []model.IMorphOffset,
	uvOffsets []model.IMorphOffset,
) {
	if modelData == nil || modelData.Morphs == nil {
		return
	}
	expressionName = strings.TrimSpace(expressionName)
	if expressionName == "" {
		return
	}
	hasVertex := len(vertexOffsets) > 0
	hasMaterial := len(materialOffsets) > 0
	hasUv := len(uvOffsets) > 0
	componentCount := 0
	if hasVertex {
		componentCount++
	}
	if hasMaterial {
		componentCount++
	}
	if hasUv {
		componentCount++
	}
	if componentCount == 0 {
		return
	}
	if componentCount == 1 && hasVertex {
		upsertTypedExpressionMorph(modelData, expressionName, panel, model.MORPH_TYPE_VERTEX, vertexOffsets, false)
		return
	}
	if componentCount == 1 && hasMaterial {
		upsertTypedExpressionMorph(modelData, expressionName, panel, model.MORPH_TYPE_MATERIAL, materialOffsets, false)
		return
	}
	if componentCount == 1 && hasUv {
		upsertTypedExpressionMorph(
			modelData,
			expressionName,
			panel,
			model.MORPH_TYPE_EXTENDED_UV1,
			uvOffsets,
			false,
		)
		return
	}
	vertexMorphName := expressionName + "__vertex"
	materialMorphName := expressionName + "__material"
	uvMorphName := expressionName + "__uv1"
	groupOffsets := []model.IMorphOffset{}
	if hasVertex {
		vertexMorph := upsertTypedExpressionMorph(
			modelData,
			vertexMorphName,
			model.MORPH_PANEL_SYSTEM,
			model.MORPH_TYPE_VERTEX,
			vertexOffsets,
			false,
		)
		if vertexMorph != nil {
			groupOffsets = append(groupOffsets, &model.GroupMorphOffset{MorphIndex: vertexMorph.Index(), MorphFactor: 1.0})
		}
	}
	if hasMaterial {
		materialMorph := upsertTypedExpressionMorph(
			modelData,
			materialMorphName,
			model.MORPH_PANEL_SYSTEM,
			model.MORPH_TYPE_MATERIAL,
			materialOffsets,
			false,
		)
		if materialMorph != nil {
			groupOffsets = append(groupOffsets, &model.GroupMorphOffset{MorphIndex: materialMorph.Index(), MorphFactor: 1.0})
		}
	}
	if hasUv {
		uvMorph := upsertTypedExpressionMorph(
			modelData,
			uvMorphName,
			model.MORPH_PANEL_SYSTEM,
			model.MORPH_TYPE_EXTENDED_UV1,
			uvOffsets,
			false,
		)
		if uvMorph != nil {
			groupOffsets = append(groupOffsets, &model.GroupMorphOffset{MorphIndex: uvMorph.Index(), MorphFactor: 1.0})
		}
	}
	if len(groupOffsets) == 0 {
		return
	}
	upsertTypedExpressionMorph(modelData, expressionName, panel, model.MORPH_TYPE_GROUP, groupOffsets, false)
}

// upsertTypedExpressionMorph は指定型のモーフを作成または更新して返す。
func upsertTypedExpressionMorph(
	modelData *model.PmxModel,
	morphName string,
	panel model.MorphPanel,
	morphType model.MorphType,
	offsets []model.IMorphOffset,
	isSystem bool,
) *model.Morph {
	if modelData == nil || modelData.Morphs == nil {
		return nil
	}
	morphName = strings.TrimSpace(morphName)
	if morphName == "" || len(offsets) == 0 {
		return nil
	}
	existing, err := modelData.Morphs.GetByName(morphName)
	if err == nil && existing != nil {
		existing.Panel = panel
		existing.EnglishName = morphName
		existing.MorphType = morphType
		existing.Offsets = offsets
		existing.IsSystem = isSystem
		return existing
	}
	morphData := &model.Morph{
		Panel:     panel,
		MorphType: morphType,
		Offsets:   offsets,
		IsSystem:  isSystem,
	}
	morphData.SetName(morphName)
	morphData.EnglishName = morphName
	modelData.Morphs.AppendRaw(morphData)
	return morphData
}

// buildMaterialOffsetsFromVrm1ColorBinds は VRM1 materialColorBinds を材質モーフへ変換する。
func buildMaterialOffsetsFromVrm1ColorBinds(
	modelData *model.PmxModel,
	binds []vrm1MaterialColorBindEntry,
	registry *targetMorphRegistry,
) []model.IMorphOffset {
	if modelData == nil || modelData.Materials == nil || registry == nil || len(binds) == 0 {
		return nil
	}
	offsetsByMaterial := map[int]*model.MaterialMorphOffset{}
	for _, bind := range binds {
		materialIndexes := findMaterialIndexesByGltfIndex(registry, bind.Material)
		if len(materialIndexes) == 0 {
			continue
		}
		for _, materialIndex := range materialIndexes {
			baseMaterial, err := modelData.Materials.Get(materialIndex)
			if err != nil || baseMaterial == nil {
				continue
			}
			offsetData, exists := offsetsByMaterial[materialIndex]
			if !exists {
				offsetData = newMaterialMorphOffset(materialIndex)
				offsetsByMaterial[materialIndex] = offsetData
			}
			appendMaterialOffsetFromVrm1ColorBind(offsetData, baseMaterial, bind)
		}
	}
	return buildSortedMaterialOffsets(offsetsByMaterial)
}

// buildMaterialOffsetsFromVrm0MaterialValues は VRM0 materialValues を材質モーフへ変換する。
func buildMaterialOffsetsFromVrm0MaterialValues(
	modelData *model.PmxModel,
	values []vrm0MaterialValueBindRaw,
	registry *targetMorphRegistry,
) []model.IMorphOffset {
	if modelData == nil || modelData.Materials == nil || registry == nil || len(values) == 0 {
		return nil
	}
	offsetsByMaterial := map[int]*model.MaterialMorphOffset{}
	for _, value := range values {
		materialName := strings.TrimSpace(value.MaterialName)
		if materialName == "" {
			continue
		}
		materialIndexes := findMaterialIndexesByName(registry, materialName)
		if len(materialIndexes) == 0 {
			continue
		}
		for _, materialIndex := range materialIndexes {
			baseMaterial, err := modelData.Materials.Get(materialIndex)
			if err != nil || baseMaterial == nil {
				continue
			}
			offsetData, exists := offsetsByMaterial[materialIndex]
			if !exists {
				offsetData = newMaterialMorphOffset(materialIndex)
				offsetsByMaterial[materialIndex] = offsetData
			}
			appendMaterialOffsetFromVrm0MaterialValue(offsetData, baseMaterial, value)
		}
	}
	return buildSortedMaterialOffsets(offsetsByMaterial)
}

// buildUvOffsetsFromVrm1TextureTransformBinds は VRM1 textureTransformBinds をUV1モーフへ変換する。
func buildUvOffsetsFromVrm1TextureTransformBinds(
	modelData *model.PmxModel,
	binds []vrm1TextureTransformBindEntry,
	registry *targetMorphRegistry,
) []model.IMorphOffset {
	if modelData == nil || modelData.Vertices == nil || registry == nil || len(binds) == 0 {
		return nil
	}
	materialVertexMap := buildMaterialVertexIndexMap(modelData)
	offsetsByVertex := map[int]mmath.Vec4{}
	for _, bind := range binds {
		materialIndexes := findMaterialIndexesByGltfIndex(registry, bind.Material)
		if len(materialIndexes) == 0 {
			continue
		}
		scale := toVec2WithDefault(bind.Scale, mmath.Vec2{X: 1.0, Y: 1.0})
		offset := toVec2WithDefault(bind.Offset, mmath.ZERO_VEC2)
		appendUvOffsetsByMaterials(modelData, materialIndexes, materialVertexMap, scale, offset, offsetsByVertex)
	}
	return buildSortedUv1Offsets(modelData, offsetsByVertex)
}

// buildUvOffsetsFromVrm0MaterialValues は VRM0 materialValues のUV変換をUV1モーフへ変換する。
func buildUvOffsetsFromVrm0MaterialValues(
	modelData *model.PmxModel,
	values []vrm0MaterialValueBindRaw,
	registry *targetMorphRegistry,
) []model.IMorphOffset {
	if modelData == nil || modelData.Vertices == nil || registry == nil || len(values) == 0 {
		return nil
	}
	materialVertexMap := buildMaterialVertexIndexMap(modelData)
	offsetsByVertex := map[int]mmath.Vec4{}
	for _, value := range values {
		propertyName := strings.ToLower(strings.TrimSpace(value.PropertyName))
		if propertyName != "_maintex_st" && propertyName != "maintex_st" {
			continue
		}
		materialIndexes := findMaterialIndexesByName(registry, value.MaterialName)
		if len(materialIndexes) == 0 {
			continue
		}
		scale, offset := resolveMainTexTransform(value.TargetValue)
		appendUvOffsetsByMaterials(modelData, materialIndexes, materialVertexMap, scale, offset, offsetsByVertex)
	}
	return buildSortedUv1Offsets(modelData, offsetsByVertex)
}

// appendUvOffsetsByMaterials は材質集合に対応する頂点へUVオフセットを加算する。
func appendUvOffsetsByMaterials(
	modelData *model.PmxModel,
	materialIndexes []int,
	materialVertexMap map[int][]int,
	scale mmath.Vec2,
	offset mmath.Vec2,
	offsetsByVertex map[int]mmath.Vec4,
) {
	if modelData == nil || modelData.Vertices == nil || len(materialIndexes) == 0 || len(materialVertexMap) == 0 {
		return
	}
	for _, materialIndex := range materialIndexes {
		vertexIndexes := materialVertexMap[materialIndex]
		if len(vertexIndexes) == 0 {
			continue
		}
		for _, vertexIndex := range vertexIndexes {
			vertex, err := modelData.Vertices.Get(vertexIndex)
			if err != nil || vertex == nil {
				continue
			}
			du := vertex.Uv.X*(scale.X-1.0) + offset.X
			dv := vertex.Uv.Y*(scale.Y-1.0) + offset.Y
			if isZeroUvDelta(du, dv) {
				continue
			}
			delta := mmath.Vec4{X: du, Y: dv}
			current, exists := offsetsByVertex[vertexIndex]
			if !exists {
				offsetsByVertex[vertexIndex] = delta
				continue
			}
			offsetsByVertex[vertexIndex] = current.Added(delta)
		}
	}
}

// buildSortedUv1Offsets は頂点index順でUV1モーフ差分を返す。
func buildSortedUv1Offsets(
	modelData *model.PmxModel,
	offsetsByVertex map[int]mmath.Vec4,
) []model.IMorphOffset {
	if modelData == nil || modelData.Vertices == nil || len(offsetsByVertex) == 0 {
		return nil
	}
	vertexIndexes := make([]int, 0, len(offsetsByVertex))
	for vertexIndex, delta := range offsetsByVertex {
		if isZeroUvDelta(delta.X, delta.Y) {
			continue
		}
		vertexIndexes = append(vertexIndexes, vertexIndex)
	}
	if len(vertexIndexes) == 0 {
		return nil
	}
	sort.Ints(vertexIndexes)
	offsets := make([]model.IMorphOffset, 0, len(vertexIndexes))
	for _, vertexIndex := range vertexIndexes {
		delta := offsetsByVertex[vertexIndex]
		if isZeroUvDelta(delta.X, delta.Y) {
			continue
		}
		ensureVertexExtendedUv1(modelData, vertexIndex)
		offsets = append(offsets, &model.UvMorphOffset{
			VertexIndex: vertexIndex,
			Uv:          delta,
			UvType:      model.MORPH_TYPE_EXTENDED_UV1,
		})
	}
	return offsets
}

// buildMaterialVertexIndexMap は材質indexごとの頂点index一覧を構築する。
func buildMaterialVertexIndexMap(modelData *model.PmxModel) map[int][]int {
	if modelData == nil || modelData.Vertices == nil {
		return map[int][]int{}
	}
	vertexIndexesByMaterial := map[int][]int{}
	for _, vertex := range modelData.Vertices.Values() {
		if vertex == nil || len(vertex.MaterialIndexes) == 0 {
			continue
		}
		vertexIndex := vertex.Index()
		for _, materialIndex := range vertex.MaterialIndexes {
			if materialIndex < 0 {
				continue
			}
			vertexIndexesByMaterial[materialIndex] = appendUniqueInt(vertexIndexesByMaterial[materialIndex], vertexIndex)
		}
	}
	for materialIndex := range vertexIndexesByMaterial {
		sort.Ints(vertexIndexesByMaterial[materialIndex])
	}
	return vertexIndexesByMaterial
}

// ensureVertexExtendedUv1 は対象頂点に拡張UV1スロットを確保する。
func ensureVertexExtendedUv1(modelData *model.PmxModel, vertexIndex int) {
	if modelData == nil || modelData.Vertices == nil || vertexIndex < 0 {
		return
	}
	vertex, err := modelData.Vertices.Get(vertexIndex)
	if err != nil || vertex == nil {
		return
	}
	if len(vertex.ExtendedUvs) >= 1 {
		return
	}
	vertex.ExtendedUvs = append(vertex.ExtendedUvs, mmath.ZERO_VEC4)
}

// resolveMainTexTransform は materialValues の MainTex_ST から scale/offset を解決する。
func resolveMainTexTransform(values []float64) (mmath.Vec2, mmath.Vec2) {
	scale := mmath.Vec2{X: 1.0, Y: 1.0}
	offset := mmath.ZERO_VEC2
	if len(values) > 0 {
		scale.X = values[0]
	}
	if len(values) > 1 {
		scale.Y = values[1]
	}
	if len(values) > 2 {
		offset.X = values[2]
	}
	if len(values) > 3 {
		offset.Y = values[3]
	}
	return scale, offset
}

// isZeroUvDelta はUV差分が実質ゼロか判定する。
func isZeroUvDelta(du float64, dv float64) bool {
	const epsilon = 1e-9
	return math.Abs(du) <= epsilon && math.Abs(dv) <= epsilon
}

// appendMaterialOffsetFromVrm1ColorBind は VRM1 color bind を PMX 材質モーフ差分へ加算する。
func appendMaterialOffsetFromVrm1ColorBind(
	offsetData *model.MaterialMorphOffset,
	baseMaterial *model.Material,
	bind vrm1MaterialColorBindEntry,
) {
	if offsetData == nil || baseMaterial == nil {
		return
	}
	targetType := strings.ToLower(strings.TrimSpace(bind.Type))
	switch targetType {
	case "color":
		target := toVec4WithDefault(bind.TargetValue, baseMaterial.Diffuse)
		offsetData.Diffuse = offsetData.Diffuse.Added(target.Subed(baseMaterial.Diffuse))
	case "shadecolor":
		target := toVec3WithDefault(bind.TargetValue, baseMaterial.Ambient)
		offsetData.Ambient = offsetData.Ambient.Added(target.Subed(baseMaterial.Ambient))
	case "emissioncolor":
		target := toVec3WithDefault(bind.TargetValue, mmath.ZERO_VEC3)
		offsetData.Specular = offsetData.Specular.Added(mmath.Vec4{
			X: target.X - baseMaterial.Specular.X,
			Y: target.Y - baseMaterial.Specular.Y,
			Z: target.Z - baseMaterial.Specular.Z,
			W: 0.0,
		})
	case "outlinecolor", "rimcolor":
		target := toVec4WithDefault(bind.TargetValue, baseMaterial.Edge)
		offsetData.Edge = offsetData.Edge.Added(target.Subed(baseMaterial.Edge))
	case "matcapcolor":
		target := toVec4WithDefault(bind.TargetValue, baseMaterial.SphereTextureFactor)
		offsetData.SphereTextureFactor = offsetData.SphereTextureFactor.Added(
			target.Subed(baseMaterial.SphereTextureFactor),
		)
	default:
	}
}

// appendMaterialOffsetFromVrm0MaterialValue は VRM0 materialValue を PMX 材質モーフ差分へ加算する。
func appendMaterialOffsetFromVrm0MaterialValue(
	offsetData *model.MaterialMorphOffset,
	baseMaterial *model.Material,
	value vrm0MaterialValueBindRaw,
) {
	if offsetData == nil || baseMaterial == nil {
		return
	}
	propertyName := strings.ToLower(strings.TrimSpace(value.PropertyName))
	switch propertyName {
	case "_color", "color":
		target := toVec4WithDefault(value.TargetValue, baseMaterial.Diffuse)
		offsetData.Diffuse = offsetData.Diffuse.Added(target.Subed(baseMaterial.Diffuse))
	case "_shadecolor", "shadecolor":
		target := toVec3WithDefault(value.TargetValue, baseMaterial.Ambient)
		offsetData.Ambient = offsetData.Ambient.Added(target.Subed(baseMaterial.Ambient))
	case "_emissioncolor", "emissioncolor":
		target := toVec3WithDefault(value.TargetValue, mmath.ZERO_VEC3)
		offsetData.Specular = offsetData.Specular.Added(mmath.Vec4{
			X: target.X - baseMaterial.Specular.X,
			Y: target.Y - baseMaterial.Specular.Y,
			Z: target.Z - baseMaterial.Specular.Z,
			W: 0.0,
		})
	case "_outlinecolor", "outlinecolor", "_rimcolor", "rimcolor":
		target := toVec4WithDefault(value.TargetValue, baseMaterial.Edge)
		offsetData.Edge = offsetData.Edge.Added(target.Subed(baseMaterial.Edge))
	case "_outlinewidth", "outlinewidth":
		target := toFloatWithDefault(value.TargetValue, baseMaterial.EdgeSize)
		offsetData.EdgeSize += target - baseMaterial.EdgeSize
	default:
	}
}

// newMaterialMorphOffset は材質モーフ差分の初期値を返す。
func newMaterialMorphOffset(materialIndex int) *model.MaterialMorphOffset {
	return &model.MaterialMorphOffset{
		MaterialIndex:       materialIndex,
		CalcMode:            model.CALC_MODE_ADDITION,
		Diffuse:             mmath.ZERO_VEC4,
		Specular:            mmath.ZERO_VEC4,
		Ambient:             mmath.ZERO_VEC3,
		Edge:                mmath.ZERO_VEC4,
		EdgeSize:            0.0,
		TextureFactor:       mmath.ZERO_VEC4,
		SphereTextureFactor: mmath.ZERO_VEC4,
		ToonTextureFactor:   mmath.ZERO_VEC4,
	}
}

// findMaterialIndexesByGltfIndex は glTF 材質indexに対応する PMX 材質index一覧を返す。
func findMaterialIndexesByGltfIndex(registry *targetMorphRegistry, gltfMaterialIndex int) []int {
	if registry == nil || gltfMaterialIndex < 0 {
		return nil
	}
	indexes, exists := registry.ByGltfMaterial[gltfMaterialIndex]
	if !exists || len(indexes) == 0 {
		return nil
	}
	out := make([]int, 0, len(indexes))
	for _, materialIndex := range indexes {
		out = appendUniqueInt(out, materialIndex)
	}
	sort.Ints(out)
	return out
}

// findMaterialIndexesByName は材質名に対応する PMX 材質index一覧を返す。
func findMaterialIndexesByName(registry *targetMorphRegistry, materialName string) []int {
	if registry == nil {
		return nil
	}
	normalizedName := strings.TrimSpace(materialName)
	if normalizedName == "" {
		return nil
	}
	indexes, exists := registry.ByMaterialName[normalizedName]
	if !exists || len(indexes) == 0 {
		return nil
	}
	out := make([]int, 0, len(indexes))
	for _, materialIndex := range indexes {
		out = appendUniqueInt(out, materialIndex)
	}
	sort.Ints(out)
	return out
}

// buildSortedMaterialOffsets は材質index順で材質モーフ差分を返す。
func buildSortedMaterialOffsets(offsetsByMaterial map[int]*model.MaterialMorphOffset) []model.IMorphOffset {
	if len(offsetsByMaterial) == 0 {
		return nil
	}
	materialIndexes := make([]int, 0, len(offsetsByMaterial))
	for materialIndex, offsetData := range offsetsByMaterial {
		if offsetData == nil || isZeroMaterialMorphOffset(offsetData) {
			continue
		}
		materialIndexes = append(materialIndexes, materialIndex)
	}
	if len(materialIndexes) == 0 {
		return nil
	}
	sort.Ints(materialIndexes)
	offsets := make([]model.IMorphOffset, 0, len(materialIndexes))
	for _, materialIndex := range materialIndexes {
		offsetData := offsetsByMaterial[materialIndex]
		if offsetData == nil || isZeroMaterialMorphOffset(offsetData) {
			continue
		}
		offsets = append(offsets, offsetData)
	}
	return offsets
}

// isZeroMaterialMorphOffset は材質モーフ差分が実質ゼロか判定する。
func isZeroMaterialMorphOffset(offsetData *model.MaterialMorphOffset) bool {
	if offsetData == nil {
		return true
	}
	const epsilon = 1e-9
	if math.Abs(offsetData.EdgeSize) > epsilon {
		return false
	}
	if !offsetData.Diffuse.NearEquals(mmath.ZERO_VEC4, epsilon) {
		return false
	}
	if !offsetData.Specular.NearEquals(mmath.ZERO_VEC4, epsilon) {
		return false
	}
	if !offsetData.Ambient.NearEquals(mmath.ZERO_VEC3, epsilon) {
		return false
	}
	if !offsetData.Edge.NearEquals(mmath.ZERO_VEC4, epsilon) {
		return false
	}
	if !offsetData.TextureFactor.NearEquals(mmath.ZERO_VEC4, epsilon) {
		return false
	}
	if !offsetData.SphereTextureFactor.NearEquals(mmath.ZERO_VEC4, epsilon) {
		return false
	}
	if !offsetData.ToonTextureFactor.NearEquals(mmath.ZERO_VEC4, epsilon) {
		return false
	}
	return true
}

// toVec4WithDefault は float 配列を Vec4 へ変換し、不足分は既定値で補う。
func toVec4WithDefault(values []float64, defaultValue mmath.Vec4) mmath.Vec4 {
	result := defaultValue
	if len(values) > 0 {
		result.X = values[0]
	}
	if len(values) > 1 {
		result.Y = values[1]
	}
	if len(values) > 2 {
		result.Z = values[2]
	}
	if len(values) > 3 {
		result.W = values[3]
	}
	return result
}

// toVec3WithDefault は float 配列を Vec3 へ変換し、不足分は既定値で補う。
func toVec3WithDefault(values []float64, defaultValue mmath.Vec3) mmath.Vec3 {
	result := defaultValue
	if len(values) > 0 {
		result.X = values[0]
	}
	if len(values) > 1 {
		result.Y = values[1]
	}
	if len(values) > 2 {
		result.Z = values[2]
	}
	return result
}

// toVec2WithDefault は float 配列を Vec2 へ変換し、不足分は既定値で補う。
func toVec2WithDefault(values []float64, defaultValue mmath.Vec2) mmath.Vec2 {
	result := defaultValue
	if len(values) > 0 {
		result.X = values[0]
	}
	if len(values) > 1 {
		result.Y = values[1]
	}
	return result
}

// toFloatWithDefault は float 配列の先頭値を返し、欠損時は既定値を返す。
func toFloatWithDefault(values []float64, defaultValue float64) float64 {
	if len(values) == 0 {
		return defaultValue
	}
	return values[0]
}

// appendWeightedVertexOffsets は頂点オフセットへ重み付き差分を加算する。
func appendWeightedVertexOffsets(offsetsByVertex map[int]mmath.Vec3, offsets []model.IMorphOffset, weight float64) {
	if len(offsets) == 0 || weight <= 0 {
		return
	}
	for _, offset := range offsets {
		vertexOffset, ok := offset.(*model.VertexMorphOffset)
		if !ok {
			continue
		}
		if vertexOffset == nil {
			continue
		}
		weighted := vertexOffset.Position.MuledScalar(weight)
		current, exists := offsetsByVertex[vertexOffset.VertexIndex]
		if !exists {
			offsetsByVertex[vertexOffset.VertexIndex] = weighted
			continue
		}
		offsetsByVertex[vertexOffset.VertexIndex] = current.Added(weighted)
	}
}

// buildMergedVertexOffsets は頂点index順で統合済み頂点オフセットを返す。
func buildMergedVertexOffsets(offsetsByVertex map[int]mmath.Vec3) []model.IMorphOffset {
	if len(offsetsByVertex) == 0 {
		return nil
	}
	vertexIndexes := make([]int, 0, len(offsetsByVertex))
	for vertexIndex, position := range offsetsByVertex {
		if isZeroMorphDelta(position) {
			continue
		}
		vertexIndexes = append(vertexIndexes, vertexIndex)
	}
	if len(vertexIndexes) == 0 {
		return nil
	}
	sort.Ints(vertexIndexes)
	mergedOffsets := make([]model.IMorphOffset, 0, len(vertexIndexes))
	for _, vertexIndex := range vertexIndexes {
		position := offsetsByVertex[vertexIndex]
		if isZeroMorphDelta(position) {
			continue
		}
		mergedOffsets = append(mergedOffsets, &model.VertexMorphOffset{
			VertexIndex: vertexIndex,
			Position:    position,
		})
	}
	return mergedOffsets
}

// resolveExpressionPanel は表情名から PMX モーフパネルを推定する。
func resolveExpressionPanel(expressionName string) model.MorphPanel {
	lowerName := strings.ToLower(strings.TrimSpace(expressionName))
	if lowerName == "" {
		return model.MORPH_PANEL_OTHER_LOWER_RIGHT
	}
	if strings.Contains(lowerName, "brow") {
		return model.MORPH_PANEL_EYEBROW_LOWER_LEFT
	}
	if strings.Contains(lowerName, "eye") || strings.Contains(lowerName, "blink") || strings.Contains(lowerName, "look") {
		return model.MORPH_PANEL_EYE_UPPER_LEFT
	}
	if strings.Contains(lowerName, "mouth") || strings.Contains(lowerName, "jaw") {
		return model.MORPH_PANEL_LIP_UPPER_RIGHT
	}
	switch lowerName {
	case "a", "i", "u", "e", "o":
		return model.MORPH_PANEL_LIP_UPPER_RIGHT
	}
	return model.MORPH_PANEL_OTHER_LOWER_RIGHT
}

// normalizeMorphBindWeight は bind 重みを PMX 係数へ正規化する。
func normalizeMorphBindWeight(weight float64, isBinary bool) float64 {
	normalized := weight
	if normalized > 1 {
		normalized = normalized / 100.0
	}
	if normalized < 0 {
		normalized = 0
	}
	if isBinary {
		if normalized >= 0.5 {
			return 1.0
		}
		return 0.0
	}
	return normalized
}

// appendUniqueInt は未登録の値だけを追加して返す。
func appendUniqueInt(values []int, target int) []int {
	for _, value := range values {
		if value == target {
			return values
		}
	}
	return append(values, target)
}

// sortedStringKeys は map のキーを昇順で返す。
func sortedStringKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// appendOrReusePrimitiveVertices はprimitive頂点を生成または既存頂点を再利用する。
func appendOrReusePrimitiveVertices(
	modelData *model.PmxModel,
	doc *gltfDocument,
	nodeIndex int,
	node gltfNode,
	primitive gltfPrimitive,
	positions [][]float64,
	normals [][]float64,
	uvs [][]float64,
	joints [][]int,
	weights [][]float64,
	nodeToBoneIndex map[int]int,
	conversion vrmConversion,
	cache *accessorValueCache,
) (int, int) {
	vertexKey := primitiveVertexKey(nodeIndex, primitive)
	if cachedStart, ok := cache.getVertexRange(vertexKey, len(positions)); ok {
		return cachedStart, 0
	}

	defaultBoneIndex := resolveDefaultBoneIndex(modelData, nodeToBoneIndex, nodeIndex)
	skinJoints := resolveSkinJoints(doc, node)
	vertexStart := modelData.Vertices.Len()
	for vertexIndex := 0; vertexIndex < len(positions); vertexIndex++ {
		position := toVec3(positions[vertexIndex], mmath.ZERO_VEC3)
		normal := mmath.Vec3{Vec: r3.Vec{X: 0, Y: 1, Z: 0}}
		if vertexIndex < len(normals) {
			normal = toVec3(normals[vertexIndex], normal)
		}
		uv := mmath.ZERO_VEC2
		if vertexIndex < len(uvs) {
			uv = toVec2(uvs[vertexIndex], mmath.ZERO_VEC2)
		}

		jointValues := []int{}
		if vertexIndex < len(joints) {
			jointValues = joints[vertexIndex]
		}
		weightValues := []float64{}
		if vertexIndex < len(weights) {
			weightValues = weights[vertexIndex]
		}
		deform := buildVertexDeform(defaultBoneIndex, jointValues, weightValues, skinJoints, nodeToBoneIndex)
		vertex := &model.Vertex{
			Position:        convertVrmPositionToPmx(position, conversion),
			Normal:          convertVrmNormalToPmx(normal, conversion),
			Uv:              uv,
			ExtendedUvs:     []mmath.Vec4{},
			DeformType:      deform.DeformType(),
			Deform:          deform,
			EdgeFactor:      1.0,
			MaterialIndexes: []int{},
		}
		modelData.Vertices.AppendRaw(vertex)
	}
	cache.setVertexRange(vertexKey, vertexStart, len(positions))
	return vertexStart, len(positions)
}

// appendVertexMaterialIndex は頂点に材質indexを追加する。
func appendVertexMaterialIndex(modelData *model.PmxModel, vertexIndex int, materialIndex int) {
	if modelData == nil || modelData.Vertices == nil || materialIndex < 0 {
		return
	}
	vertex, err := modelData.Vertices.Get(vertexIndex)
	if err != nil || vertex == nil {
		return
	}
	vertex.MaterialIndexes = append(vertex.MaterialIndexes, materialIndex)
}

// resolveDefaultBoneIndex は頂点デフォーム未解決時に使う既定ボーンindexを返す。
func resolveDefaultBoneIndex(modelData *model.PmxModel, nodeToBoneIndex map[int]int, nodeIndex int) int {
	if idx, ok := nodeToBoneIndex[nodeIndex]; ok && idx >= 0 {
		return idx
	}
	if modelData != nil && modelData.Bones != nil && modelData.Bones.Len() > 0 {
		return 0
	}
	return 0
}

// resolveSkinJoints はnode.skin からjoint node index配列を返す。
func resolveSkinJoints(doc *gltfDocument, node gltfNode) []int {
	if node.Skin == nil || doc == nil {
		return nil
	}
	skinIndex := *node.Skin
	if skinIndex < 0 || skinIndex >= len(doc.Skins) {
		return nil
	}
	return doc.Skins[skinIndex].Joints
}

// buildVertexDeform はJOINTS/WEIGHTSからPMXデフォームを生成する。
func buildVertexDeform(
	defaultBoneIndex int,
	joints []int,
	weights []float64,
	skinJoints []int,
	nodeToBoneIndex map[int]int,
) model.IDeform {
	weightByBone := map[int]float64{}
	maxCount := len(joints)
	if len(weights) < maxCount {
		maxCount = len(weights)
	}
	for i := 0; i < maxCount; i++ {
		weight := weights[i]
		if weight <= 0 {
			continue
		}
		jointIndex := joints[i]
		if jointIndex < 0 {
			continue
		}

		nodeIndex := jointIndex
		if len(skinJoints) > 0 {
			if jointIndex >= len(skinJoints) {
				continue
			}
			nodeIndex = skinJoints[jointIndex]
		}
		boneIndex, ok := nodeToBoneIndex[nodeIndex]
		if !ok || boneIndex < 0 {
			continue
		}
		weightByBone[boneIndex] += weight
	}

	if len(weightByBone) == 0 {
		return model.NewBdef1(defaultBoneIndex)
	}

	weightedBones := make([]weightedBone, 0, len(weightByBone))
	totalWeight := 0.0
	for boneIndex, weight := range weightByBone {
		if weight <= 0 {
			continue
		}
		weightedBones = append(weightedBones, weightedBone{
			BoneIndex: boneIndex,
			Weight:    weight,
		})
		totalWeight += weight
	}
	if totalWeight <= 0 || len(weightedBones) == 0 {
		return model.NewBdef1(defaultBoneIndex)
	}

	sort.Slice(weightedBones, func(i int, j int) bool {
		if weightedBones[i].Weight == weightedBones[j].Weight {
			return weightedBones[i].BoneIndex < weightedBones[j].BoneIndex
		}
		return weightedBones[i].Weight > weightedBones[j].Weight
	})

	if len(weightedBones) == 1 {
		return model.NewBdef1(weightedBones[0].BoneIndex)
	}

	if len(weightedBones) == 2 {
		weight0 := weightedBones[0].Weight / (weightedBones[0].Weight + weightedBones[1].Weight)
		return model.NewBdef2(weightedBones[0].BoneIndex, weightedBones[1].BoneIndex, weight0)
	}

	if len(weightedBones) > 4 {
		weightedBones = weightedBones[:4]
	}
	totalTopWeight := 0.0
	for _, wb := range weightedBones {
		totalTopWeight += wb.Weight
	}
	if totalTopWeight <= 0 {
		return model.NewBdef1(defaultBoneIndex)
	}

	indexes := [4]int{defaultBoneIndex, defaultBoneIndex, defaultBoneIndex, defaultBoneIndex}
	values := [4]float64{0, 0, 0, 0}
	for i := 0; i < len(weightedBones) && i < 4; i++ {
		indexes[i] = weightedBones[i].BoneIndex
		values[i] = weightedBones[i].Weight / totalTopWeight
	}
	return model.NewBdef4(indexes, values)
}

// appendPrimitiveMaterial はprimitive用材質を追加し、そのindexを返す。
func appendPrimitiveMaterial(
	modelData *model.PmxModel,
	doc *gltfDocument,
	primitive gltfPrimitive,
	primitiveName string,
	textureIndexesByImage []int,
	verticesCount int,
	registry *targetMorphRegistry,
) int {
	material := model.NewMaterial()
	material.SetName(primitiveName)
	material.EnglishName = primitiveName
	material.Memo = "VRM primitive"
	material.Diffuse = mmath.Vec4{X: 1.0, Y: 1.0, Z: 1.0, W: 1.0}
	material.Specular = mmath.Vec4{X: 0.0, Y: 0.0, Z: 0.0, W: 1.0}
	material.Ambient = mmath.Vec3{Vec: r3.Vec{X: 0.5, Y: 0.5, Z: 0.5}}
	material.Edge = mmath.Vec4{X: 0.0, Y: 0.0, Z: 0.0, W: 1.0}
	material.EdgeSize = 1.0
	material.TextureFactor = mmath.ONE_VEC4
	material.SphereTextureFactor = mmath.ONE_VEC4
	material.ToonTextureFactor = mmath.ONE_VEC4
	material.DrawFlag = model.DRAW_FLAG_GROUND_SHADOW | model.DRAW_FLAG_DRAWING_ON_SELF_SHADOW_MAPS | model.DRAW_FLAG_DRAWING_SELF_SHADOWS
	material.VerticesCount = verticesCount
	material.TextureIndex = resolveMaterialTextureIndex(doc, primitive, textureIndexesByImage)

	if primitive.Material != nil {
		materialIndex := *primitive.Material
		if materialIndex >= 0 && materialIndex < len(doc.Materials) {
			sourceMaterial := doc.Materials[materialIndex]
			if sourceMaterial.Name != "" {
				material.SetName(sourceMaterial.Name)
				material.EnglishName = sourceMaterial.Name
			}
			if sourceMaterial.DoubleSided {
				material.DrawFlag |= model.DRAW_FLAG_DOUBLE_SIDED_DRAWING
			}
			if len(sourceMaterial.PbrMetallicRoughness.BaseColorFactor) == 4 {
				material.Diffuse = mmath.Vec4{
					X: sourceMaterial.PbrMetallicRoughness.BaseColorFactor[0],
					Y: sourceMaterial.PbrMetallicRoughness.BaseColorFactor[1],
					Z: sourceMaterial.PbrMetallicRoughness.BaseColorFactor[2],
					W: sourceMaterial.PbrMetallicRoughness.BaseColorFactor[3],
				}
			}
		}
	}

	materialIndex := modelData.Materials.AppendRaw(material)
	registerExpressionMaterialIndex(registry, doc, primitive.Material, material.Name(), materialIndex)
	return materialIndex
}

// registerExpressionMaterialIndex は表情材質バインド用の材質index参照を登録する。
func registerExpressionMaterialIndex(
	registry *targetMorphRegistry,
	doc *gltfDocument,
	gltfMaterialIndex *int,
	pmxMaterialName string,
	pmxMaterialIndex int,
) {
	if registry == nil || pmxMaterialIndex < 0 {
		return
	}
	if gltfMaterialIndex != nil && *gltfMaterialIndex >= 0 {
		registry.ByGltfMaterial[*gltfMaterialIndex] = appendUniqueInt(
			registry.ByGltfMaterial[*gltfMaterialIndex],
			pmxMaterialIndex,
		)
		if doc != nil && *gltfMaterialIndex < len(doc.Materials) {
			sourceMaterialName := strings.TrimSpace(doc.Materials[*gltfMaterialIndex].Name)
			if sourceMaterialName != "" {
				registry.ByMaterialName[sourceMaterialName] = appendUniqueInt(
					registry.ByMaterialName[sourceMaterialName],
					pmxMaterialIndex,
				)
			}
		}
	}
	normalizedName := strings.TrimSpace(pmxMaterialName)
	if normalizedName == "" {
		return
	}
	registry.ByMaterialName[normalizedName] = appendUniqueInt(
		registry.ByMaterialName[normalizedName],
		pmxMaterialIndex,
	)
}

// resolveMaterialTextureIndex はbaseColorTextureからPMXテクスチャindexを解決する。
func resolveMaterialTextureIndex(doc *gltfDocument, primitive gltfPrimitive, textureIndexesByImage []int) int {
	if doc == nil || primitive.Material == nil {
		return -1
	}
	materialIndex := *primitive.Material
	if materialIndex < 0 || materialIndex >= len(doc.Materials) {
		return -1
	}
	baseTexture := doc.Materials[materialIndex].PbrMetallicRoughness.BaseColorTexture
	if baseTexture == nil {
		return -1
	}
	if baseTexture.Index < 0 || baseTexture.Index >= len(doc.Textures) {
		return -1
	}
	texture := doc.Textures[baseTexture.Index]
	if texture.Source == nil {
		return -1
	}
	imageIndex := *texture.Source
	if imageIndex < 0 || imageIndex >= len(textureIndexesByImage) {
		return -1
	}
	return textureIndexesByImage[imageIndex]
}

// triangulateIndices はprimitive modeに従って三角形indexへ変換する。
func triangulateIndices(indexes []int, mode int) [][3]int {
	switch mode {
	case gltfPrimitiveModeTriangles:
		out := make([][3]int, 0, len(indexes)/3)
		for i := 0; i+2 < len(indexes); i += 3 {
			out = append(out, [3]int{indexes[i], indexes[i+1], indexes[i+2]})
		}
		return out
	case gltfPrimitiveModeTriangleStrip:
		out := make([][3]int, 0, len(indexes)-2)
		for i := 0; i+2 < len(indexes); i++ {
			if i%2 == 0 {
				out = append(out, [3]int{indexes[i], indexes[i+1], indexes[i+2]})
			} else {
				out = append(out, [3]int{indexes[i+1], indexes[i], indexes[i+2]})
			}
		}
		return out
	case gltfPrimitiveModeTriangleFan:
		out := make([][3]int, 0, len(indexes)-2)
		for i := 1; i+1 < len(indexes); i++ {
			out = append(out, [3]int{indexes[0], indexes[i], indexes[i+1]})
		}
		return out
	case gltfPrimitiveModePoints, gltfPrimitiveModeLines, gltfPrimitiveModeLineLoop, gltfPrimitiveModeLineStrip:
		return [][3]int{}
	default:
		return [][3]int{}
	}
}

// readPrimitiveIndices はprimitiveのindex配列を返す。
func readPrimitiveIndices(
	doc *gltfDocument,
	primitive gltfPrimitive,
	vertexCount int,
	binChunk []byte,
	cache *accessorValueCache,
) ([]int, error) {
	if primitive.Indices == nil {
		indices := make([]int, vertexCount)
		for i := 0; i < vertexCount; i++ {
			indices[i] = i
		}
		return indices, nil
	}
	accessorIndex := *primitive.Indices
	values, err := cache.readIntValues(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, io_common.NewIoParseFailed("indices の読み取りに失敗しました(accessor=%d)", err, accessorIndex)
	}
	indices := make([]int, 0, len(values))
	for _, row := range values {
		if len(row) == 0 {
			continue
		}
		indices = append(indices, row[0])
	}
	return indices, nil
}

// readOptionalFloatAttribute は任意属性accessorを読み取る。
func readOptionalFloatAttribute(
	doc *gltfDocument,
	attributes map[string]int,
	key string,
	binChunk []byte,
	cache *accessorValueCache,
) ([][]float64, error) {
	if attributes == nil {
		return nil, nil
	}
	accessorIndex, ok := attributes[key]
	if !ok {
		return nil, nil
	}
	values, err := cache.readFloatValues(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, io_common.NewIoParseFailed("%s属性の読み取りに失敗しました(accessor=%d)", err, key, accessorIndex)
	}
	return values, nil
}

// readOptionalIntAttribute は任意属性accessorを読み取る。
func readOptionalIntAttribute(
	doc *gltfDocument,
	attributes map[string]int,
	key string,
	binChunk []byte,
	cache *accessorValueCache,
) ([][]int, error) {
	if attributes == nil {
		return nil, nil
	}
	accessorIndex, ok := attributes[key]
	if !ok {
		return nil, nil
	}
	values, err := cache.readIntValues(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, io_common.NewIoParseFailed("%s属性の読み取りに失敗しました(accessor=%d)", err, key, accessorIndex)
	}
	return values, nil
}

// readAccessorFloatValues はaccessorをfloat値配列として読み取る。
func readAccessorFloatValues(doc *gltfDocument, accessorIndex int, binChunk []byte) ([][]float64, error) {
	plan, err := prepareAccessorRead(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, err
	}
	values := make([][]float64, plan.Accessor.Count)
	for i := 0; i < plan.Accessor.Count; i++ {
		row := make([]float64, plan.ComponentNum)
		elementBase := plan.BaseOffset + i*plan.Stride
		for c := 0; c < plan.ComponentNum; c++ {
			value, readErr := readComponentAsFloat(plan.Accessor, binChunk, elementBase+c*plan.ComponentSize)
			if readErr != nil {
				return nil, readErr
			}
			row[c] = value
		}
		values[i] = row
	}
	return values, nil
}

// readAccessorIntValues はaccessorをint値配列として読み取る。
func readAccessorIntValues(doc *gltfDocument, accessorIndex int, binChunk []byte) ([][]int, error) {
	plan, err := prepareAccessorRead(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, err
	}
	values := make([][]int, plan.Accessor.Count)
	for i := 0; i < plan.Accessor.Count; i++ {
		row := make([]int, plan.ComponentNum)
		elementBase := plan.BaseOffset + i*plan.Stride
		for c := 0; c < plan.ComponentNum; c++ {
			value, readErr := readComponentAsInt(plan.Accessor.ComponentType, binChunk, elementBase+c*plan.ComponentSize)
			if readErr != nil {
				return nil, readErr
			}
			row[c] = value
		}
		values[i] = row
	}
	return values, nil
}

// prepareAccessorRead はaccessor読み取りに必要な情報を検証して返す。
func prepareAccessorRead(doc *gltfDocument, accessorIndex int, binChunk []byte) (accessorReadPlan, error) {
	if doc == nil {
		return accessorReadPlan{}, io_common.NewIoParseFailed("gltf document が未設定です", nil)
	}
	if accessorIndex < 0 || accessorIndex >= len(doc.Accessors) {
		return accessorReadPlan{}, io_common.NewIoParseFailed("accessor index が不正です: %d", nil, accessorIndex)
	}
	accessor := doc.Accessors[accessorIndex]
	if accessor.BufferView == nil {
		return accessorReadPlan{}, io_common.NewIoParseFailed("sparse accessor は未対応です", nil)
	}
	if accessor.Count < 0 {
		return accessorReadPlan{}, io_common.NewIoParseFailed("accessor.count が不正です: %d", nil, accessor.Count)
	}

	viewIndex := *accessor.BufferView
	if viewIndex < 0 || viewIndex >= len(doc.BufferViews) {
		return accessorReadPlan{}, io_common.NewIoParseFailed("bufferView index が不正です: %d", nil, viewIndex)
	}
	view := doc.BufferViews[viewIndex]
	if view.Buffer != 0 {
		return accessorReadPlan{}, io_common.NewIoParseFailed("bufferView.buffer が未対応です: %d", nil, view.Buffer)
	}
	if view.ByteLength < 0 || view.ByteOffset < 0 {
		return accessorReadPlan{}, io_common.NewIoParseFailed("bufferView の byteOffset/byteLength が不正です", nil)
	}
	if view.ByteOffset+view.ByteLength > len(binChunk) {
		return accessorReadPlan{}, io_common.NewIoParseFailed("bufferView 範囲がBINチャンク外です", nil)
	}

	componentNum, err := accessorComponentNum(accessor.Type)
	if err != nil {
		return accessorReadPlan{}, err
	}
	componentSize, err := accessorComponentSize(accessor.ComponentType)
	if err != nil {
		return accessorReadPlan{}, err
	}
	elementSize := componentNum * componentSize
	stride := view.ByteStride
	if stride <= 0 {
		stride = elementSize
	}
	if stride < elementSize {
		return accessorReadPlan{}, io_common.NewIoParseFailed("bufferView.byteStride が要素サイズより小さいです", nil)
	}
	baseOffset := view.ByteOffset + accessor.ByteOffset
	if baseOffset < view.ByteOffset || baseOffset > view.ByteOffset+view.ByteLength {
		return accessorReadPlan{}, io_common.NewIoParseFailed("accessor.byteOffset が不正です", nil)
	}
	if accessor.Count > 0 {
		lastEnd := baseOffset + (accessor.Count-1)*stride + elementSize
		if lastEnd > view.ByteOffset+view.ByteLength || lastEnd > len(binChunk) {
			return accessorReadPlan{}, io_common.NewIoParseFailed("accessor 範囲がbufferViewを超えています", nil)
		}
	}

	return accessorReadPlan{
		Accessor:      accessor,
		ComponentSize: componentSize,
		ComponentNum:  componentNum,
		Stride:        stride,
		BaseOffset:    baseOffset,
		ViewStart:     view.ByteOffset,
		ViewEnd:       view.ByteOffset + view.ByteLength,
	}, nil
}

// accessorComponentNum はaccessor.typeから要素次元数を返す。
func accessorComponentNum(typeName string) (int, error) {
	switch typeName {
	case "SCALAR":
		return 1, nil
	case "VEC2":
		return 2, nil
	case "VEC3":
		return 3, nil
	case "VEC4":
		return 4, nil
	default:
		return 0, io_common.NewIoFormatNotSupported("accessor.type が未対応です: %s", nil, typeName)
	}
}

// accessorComponentSize はcomponentTypeのバイト幅を返す。
func accessorComponentSize(componentType int) (int, error) {
	switch componentType {
	case gltfComponentTypeByte, gltfComponentTypeUnsignedByte:
		return 1, nil
	case gltfComponentTypeShort, gltfComponentTypeUnsignedShort:
		return 2, nil
	case gltfComponentTypeUnsignedInt, gltfComponentTypeFloat:
		return 4, nil
	default:
		return 0, io_common.NewIoFormatNotSupported("accessor.componentType が未対応です: %d", nil, componentType)
	}
}

// readComponentAsFloat はcomponentTypeをfloat64へ変換する。
func readComponentAsFloat(accessor gltfAccessor, data []byte, offset int) (float64, error) {
	switch accessor.ComponentType {
	case gltfComponentTypeByte:
		value := float64(int8(data[offset]))
		if accessor.Normalized {
			return math.Max(value/127.0, -1.0), nil
		}
		return value, nil
	case gltfComponentTypeUnsignedByte:
		value := float64(data[offset])
		if accessor.Normalized {
			return value / 255.0, nil
		}
		return value, nil
	case gltfComponentTypeShort:
		value := float64(int16(binary.LittleEndian.Uint16(data[offset : offset+2])))
		if accessor.Normalized {
			return math.Max(value/32767.0, -1.0), nil
		}
		return value, nil
	case gltfComponentTypeUnsignedShort:
		value := float64(binary.LittleEndian.Uint16(data[offset : offset+2]))
		if accessor.Normalized {
			return value / 65535.0, nil
		}
		return value, nil
	case gltfComponentTypeUnsignedInt:
		value := float64(binary.LittleEndian.Uint32(data[offset : offset+4]))
		if accessor.Normalized {
			return value / 4294967295.0, nil
		}
		return value, nil
	case gltfComponentTypeFloat:
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(data[offset : offset+4]))), nil
	default:
		return 0, io_common.NewIoFormatNotSupported("float componentType が未対応です: %d", nil, accessor.ComponentType)
	}
}

// readComponentAsInt はcomponentTypeをintへ変換する。
func readComponentAsInt(componentType int, data []byte, offset int) (int, error) {
	switch componentType {
	case gltfComponentTypeByte:
		return int(int8(data[offset])), nil
	case gltfComponentTypeUnsignedByte:
		return int(data[offset]), nil
	case gltfComponentTypeShort:
		return int(int16(binary.LittleEndian.Uint16(data[offset : offset+2]))), nil
	case gltfComponentTypeUnsignedShort:
		return int(binary.LittleEndian.Uint16(data[offset : offset+2])), nil
	case gltfComponentTypeUnsignedInt:
		return int(binary.LittleEndian.Uint32(data[offset : offset+4])), nil
	default:
		return 0, io_common.NewIoFormatNotSupported("int componentType が未対応です: %d", nil, componentType)
	}
}

// toVec3 はfloat配列をVec3へ変換する。
func toVec3(values []float64, defaultValue mmath.Vec3) mmath.Vec3 {
	if len(values) < 3 {
		return defaultValue
	}
	return mmath.Vec3{
		Vec: r3.Vec{
			X: values[0],
			Y: values[1],
			Z: values[2],
		},
	}
}

// toVec2 はfloat配列をVec2へ変換する。
func toVec2(values []float64, defaultValue mmath.Vec2) mmath.Vec2 {
	if len(values) < 2 {
		return defaultValue
	}
	return mmath.Vec2{
		X: values[0],
		Y: values[1],
	}
}
