// 指示: miu200521358
package minteractor

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mu_vrm2pmx/pkg/adapter/io_model/vrm"
)

type materialTestStruct struct {
	Path          string     // vrmフルパス
	WantMaterials [][]string // 並び替えが必要な材質グループの並び順
}

var materialTests = []materialTestStruct{
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/Akami - 【朱巳】あかみ -アカミ【Akami】.vrm",
		WantMaterials: [][]string{
			{
				"N00_000_00_Face_00_SKIN (Instance)",
				"N00_000_00_FaceBrow_00_FACE (Instance)",
				"N00_000_00_FaceEyeline_00_FACE (Instance)",
				"N00_000_00_FaceEyelash_00_FACE (Instance)",
			},
			{
				"N00_000_00_Face_00_SKIN (Instance)",
				"N00_000_00_HairBack_00_HAIR (Instance)",
				"N00_000_Hair_00_HAIR_01 (Instance)",
				"N00_000_Hair_00_HAIR_02 (Instance)",
				"N00_000_Hair_00_HAIR_03 (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_004_01_Shoes_01_CLOTH (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_010_01_Onepiece_00_CLOTH (Instance)",
				"N00_007_01_Tops_01_CLOTH (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_010_01_Onepiece_00_CLOTH (Instance)",
				"N00_002_01_Tops_01_CLOTH_01 (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_002_01_Tops_01_CLOTH_02 (Instance)",
				"N00_002_01_Tops_01_CLOTH_01 (Instance)",
				"N00_002_01_Tops_01_CLOTH_03 (Instance)",
			},
		},
	},
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/Liliana.vrm",
		WantMaterials: [][]string{
			{
				"N00_000_00_Face_00_SKIN (Instance)",
				"N00_000_00_FaceBrow_00_FACE (Instance)",
				"N00_000_00_FaceEyeline_00_FACE (Instance)",
			},
			{
				"N00_000_00_Face_00_SKIN (Instance)",
				"N00_000_Hair_00_HAIR_01 (Instance)",
				"N00_000_Hair_00_HAIR_02 (Instance)",
				"N00_000_Hair_00_HAIR_03 (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_008_01_Shoes_01_CLOTH_01 (Instance)",
				"N00_008_01_Shoes_01_CLOTH_02 (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_010_01_Onepiece_00_CLOTH_01 (Instance)",
				"N00_010_01_Onepiece_00_CLOTH_02 (Instance)",
				"N00_003_01_Bottoms_01_CLOTH (Instance)",
				"N00_002_01_Tops_01_CLOTH (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_010_01_Onepiece_00_CLOTH_01 (Instance)",
				"N00_010_01_Onepiece_00_CLOTH_03 (Instance)",
			},
		},
	},
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/ricos - リコス.vrm",
		WantMaterials: [][]string{
			{
				"N00_000_00_Face_00_SKIN (Instance)",
				"N00_000_00_FaceBrow_00_FACE (Instance)",
				"N00_000_00_FaceEyeline_00_FACE (Instance)",
				"N00_000_00_FaceEyelash_00_FACE (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_010_01_Onepiece_00_CLOTH_03 (Instance)",
				"N00_010_01_Onepiece_00_CLOTH_01 (Instance)",
				"N00_010_01_Onepiece_00_CLOTH_02 (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_002_03_Tops_01_CLOTH_02 (Instance)",
				"N00_002_03_Tops_01_CLOTH_01 (Instance)",
				"N00_002_03_Tops_01_CLOTH_03 (Instance)", // リボン
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_002_03_Tops_01_CLOTH_04 (Instance)",
			},
			{
				"N00_000_00_Face_00_SKIN (Instance)",
				"N00_000_00_HairBack_00_HAIR (Instance)",
				"N00_000_Hair_00_HAIR_01 (Instance)",
				"N00_000_Hair_00_HAIR_02 (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_003_01_Shoes_01_CLOTH (Instance)",
				"N00_002_01_Shoes_01_CLOTH (Instance)",
			},
		},
	},
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/Yelena.vrm",
		WantMaterials: [][]string{
			{
				"N00_000_00_Face_00_SKIN (Instance)",
				"N00_000_00_FaceBrow_00_FACE (Instance)",
				"N00_000_00_FaceEyeline_00_FACE (Instance)",
				"N00_000_00_FaceEyelash_00_FACE (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_001_03_Bottoms_01_CLOTH (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_002_04_Tops_01_CLOTH (Instance)",
			},
		},
	},
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/いおり 赤色スタジアムジャンパー.vrm",
		WantMaterials: [][]string{
			{
				"N00_000_00_Face_00_SKIN (Instance)",
				"N00_000_00_FaceBrow_00_FACE (Instance)",
				"N00_000_00_FaceEyeline_00_FACE (Instance)",
				"N00_000_00_FaceEyelash_00_FACE (Instance)",
			},
			{
				"N00_000_00_Face_00_SKIN (Instance)",
				"N00_000_00_EyeWhite_00_EYE (Instance)",
				"N00_000_00_EyeIris_00_EYE (Instance)",
				"N00_000_00_EyeHighlight_00_EYE (Instance)",
			},
			{
				"N00_000_00_EyeIris_00_EYE (Instance)",
				"N00_000_00_EyeHighlight_00_EYE (Instance)",
				"N00_000_00_FaceEyelash_00_FACE (Instance)",
			},
			{
				"N00_010_01_Onepiece_00_CLOTH (Instance)",
				"N00_007_03_Tops_01_CLOTH_02 (Instance)",
			},
			{
				"N00_011_02_Bottoms_01_CLOTH_02 (Instance)",
				"N00_011_02_Bottoms_01_CLOTH_01 (Instance)",
				"N00_007_03_Tops_01_CLOTH_02 (Instance)",
			},
		},
	},
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/オリジナル - 【DL可】情熱の真っ赤なやえちゃん.vrm",
		WantMaterials: [][]string{
			{
				"ref_B",
				"tp",
			},
		},
	},
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/ゆき - Yuki (DL OK) - ゆき - Yuki Ver.009s2505春.vrm",
		WantMaterials: [][]string{
			{
				"N00_000_00_Face_00_SKIN (Instance)",
				"N00_000_00_FaceBrow_00_FACE (Instance)",
				"N00_000_00_FaceEyeline_00_FACE (Instance)",
				"N00_000_00_FaceEyelash_00_FACE (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_010_01_Onepiece_00_CLOTH (Instance)",
			},
			{
				"N00_000_00_Body_00_SKIN (Instance)",
				"N00_002_03_Tops_01_CLOTH_04 (Instance)",
				"N00_002_03_Tops_01_CLOTH_01 (Instance)",
				"N00_002_03_Tops_01_CLOTH_02 (Instance)",
				"N00_002_03_Tops_01_CLOTH_03 (Instance)",
			},
		},
	},
}

// transparentMaterialSnapshot は半透明材質の並び確認用スナップショットを表す。
type transparentMaterialSnapshot struct {
	Index     int
	Name      string
	HasScore  bool
	Score     float64
	Alpha     float64
	TextureID int
}

func TestApplyBodyDepthMaterialOrderWithExternalVrmPath(t *testing.T) {
	if len(materialTests) == 0 {
		t.Skip("materialTests が空のためスキップ")
	}
	for i, materialTest := range materialTests {
		materialTest := materialTest
		t.Run(buildMaterialReorderTestName(i, materialTest.Path), func(t *testing.T) {
			if strings.TrimSpace(materialTest.Path) == "" {
				t.Skip("Path が空のためスキップ")
			}
			vrmPath := convertWindowsPathToWslForReorderTest(materialTest.Path)
			if _, err := os.Stat(vrmPath); err != nil {
				if os.IsNotExist(err) {
					t.Skipf("指定VRMが見つからないためスキップ: %s", vrmPath)
				}
				t.Fatalf("指定VRMの確認に失敗しました: %v", err)
			}

			uc := NewVrm2PmxUsecase(Vrm2PmxUsecaseDeps{
				ModelReader: vrm.NewVrmRepository(),
			})
			modelData, err := uc.LoadModel(nil, vrmPath)
			if err != nil {
				t.Fatalf("VRM読み込みに失敗しました: %v", err)
			}
			if modelData == nil {
				t.Fatalf("VRM読み込み結果がnilです")
			}

			before, beforeBodyPoints, err := captureTransparentMaterialSnapshots(modelData)
			if err != nil {
				t.Fatalf("並べ替え前スナップショット取得に失敗しました: %v", err)
			}
			logTransparentMaterialSnapshots(t, "before", before, beforeBodyPoints)
			logTransparentMaterialSummary(t, "before", len(before))

			outputPath := filepath.Join(t.TempDir(), "material_reorder_external_test.pmx")
			result, err := uc.PrepareModel(ConvertRequest{
				InputPath:  vrmPath,
				OutputPath: outputPath,
				ModelData:  modelData,
			})
			if err != nil {
				t.Fatalf("PrepareModelに失敗しました: %v", err)
			}
			if result == nil || result.Model == nil {
				t.Fatalf("PrepareModelの結果が不正です")
			}

			after, afterBodyPoints, err := captureTransparentMaterialSnapshots(result.Model)
			if err != nil {
				t.Fatalf("並べ替え後スナップショット取得に失敗しました: %v", err)
			}
			logTransparentMaterialSnapshots(t, "after", after, afterBodyPoints)
			if testing.Verbose() {
				logOverlapPairScores(t, result.Model, after)
			}

			logTransparentMaterialSummary(t, "after", len(after))
			if err := verifyMaterialOrder(result.Model, materialTest.WantMaterials); err != nil {
				t.Fatalf("期待材質順の検証に失敗しました: %v", err)
			}
		})
	}
}

// logTransparentMaterialSummary は半透明材質評価をスキップした理由をログ出力する。
func logTransparentMaterialSummary(t *testing.T, phase string, count int) {
	t.Helper()
	if count >= 2 {
		return
	}
	t.Logf("[%s] 半透明材質が2件未満のため近傍スコア順検証をスキップ: count=%d", phase, count)
}

// captureTransparentMaterialSnapshots は半透明材質の評価情報を収集する。
func captureTransparentMaterialSnapshots(modelData *ModelData) ([]transparentMaterialSnapshot, int, error) {
	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil {
		return nil, 0, err
	}
	textureImageCache := map[int]textureImageCacheEntry{}
	materialTransparencyScores := buildMaterialTransparencyScores(
		modelData,
		faceRanges,
		textureImageCache,
		textureAlphaTransparentThreshold,
	)
	transparentIndexes := collectTransparentMaterialIndexesFromScores(modelData, materialTransparencyScores)
	if len(transparentIndexes) < 2 {
		fallbackScores := buildMaterialTransparencyScores(
			modelData,
			faceRanges,
			textureImageCache,
			textureAlphaFallbackThreshold,
		)
		fallbackTransparentIndexes := collectTransparentMaterialIndexesFromScores(modelData, fallbackScores)
		if len(fallbackTransparentIndexes) >= 2 {
			materialTransparencyScores = fallbackScores
			transparentIndexes = fallbackTransparentIndexes
		}
	}
	if len(transparentIndexes) < 2 {
		fallbackTransparentIndexes := collectDoubleSidedTextureMaterialIndexes(modelData)
		if len(fallbackTransparentIndexes) >= 2 {
			transparentIndexes = fallbackTransparentIndexes
		}
	}
	transparentMaterialIndexSet := map[int]struct{}{}
	for _, materialIndex := range transparentIndexes {
		transparentMaterialIndexSet[materialIndex] = struct{}{}
	}
	blockSize := len(transparentIndexes)
	if blockSize < 1 {
		blockSize = 1
	}
	bodyPoints := collectBodyPointsForSorting(modelData, faceRanges, transparentMaterialIndexSet, blockSize)

	snapshots := make([]transparentMaterialSnapshot, 0)
	for _, materialIndex := range transparentIndexes {
		materialData := modelData.Materials.Values()[materialIndex]
		score, ok := calculateBodyProximityScore(modelData, faceRanges[materialIndex], bodyPoints, blockSize)
		snapshots = append(snapshots, transparentMaterialSnapshot{
			Index:     materialIndex,
			Name:      resolveMaterialName(materialData, materialIndex),
			HasScore:  ok,
			Score:     score,
			Alpha:     materialData.Diffuse.W,
			TextureID: materialData.TextureIndex,
		})
	}
	return snapshots, len(bodyPoints), nil
}

// verifyMaterialOrder は材質並びがサブリスト内順序を満たすか確認する。
func verifyMaterialOrder(modelData *ModelData, wantMaterialGroups [][]string) error {
	if len(wantMaterialGroups) == 0 {
		return nil
	}

	gotMaterials := listMaterialNames(modelData)
	if len(gotMaterials) == 0 {
		return fmt.Errorf("材質が空です")
	}

	materialPositions := make(map[string][]int, len(gotMaterials))
	for i, name := range gotMaterials {
		materialPositions[name] = append(materialPositions[name], i)
	}

	for groupIndex, group := range wantMaterialGroups {
		if len(group) < 2 {
			continue
		}
		lastIndex := -1
		for _, materialName := range group {
			positions := resolveExpectedMaterialPositions(materialPositions, materialName)
			if len(positions) == 0 {
				return fmt.Errorf("group=%d material=%q が存在しません", groupIndex, materialName)
			}

			found := false
			for _, pos := range positions {
				if pos <= lastIndex {
					continue
				}
				lastIndex = pos
				found = true
				break
			}
			if found {
				continue
			}
			return fmt.Errorf(
				"group=%d の順序不一致: material=%q の位置が前要素より後に存在しません (lastIndex=%d positions=%v)",
				groupIndex,
				materialName,
				lastIndex,
				positions,
			)
		}
	}

	return nil
}

// resolveExpectedMaterialPositions は期待材質名に対応する実材質位置一覧を返す。
func resolveExpectedMaterialPositions(materialPositions map[string][]int, materialName string) []int {
	if positions, exists := materialPositions[materialName]; exists {
		return positions
	}
	base := abbreviateMaterialName(materialName)
	if base == "" || base == materialName {
		return nil
	}
	positions := make([]int, 0, 4)
	if basePositions, exists := materialPositions[base]; exists {
		positions = append(positions, basePositions...)
	}
	for candidateName, candidatePositions := range materialPositions {
		if !hasSerialSuffix(candidateName, base) {
			continue
		}
		positions = append(positions, candidatePositions...)
	}
	for candidateName, candidatePositions := range materialPositions {
		if !hasMaterialVariantSuffix(candidateName) {
			continue
		}
		candidateBase := resolveMaterialVariantBaseName(candidateName)
		if candidateBase != base {
			normalizedCandidateBase, changed := normalizeMaterialNameByPrefixAndSuffix(candidateBase)
			if !changed || normalizedCandidateBase != base {
				continue
			}
		}
		positions = append(positions, candidatePositions...)
	}
	sort.Ints(positions)
	return positions
}

// hasSerialSuffix は base に `_連番` が付いた候補名かを判定する。
func hasSerialSuffix(candidateName string, base string) bool {
	if candidateName == "" || base == "" {
		return false
	}
	prefix := base + "_"
	if !strings.HasPrefix(candidateName, prefix) {
		return false
	}
	suffix := strings.TrimPrefix(candidateName, prefix)
	if suffix == "" {
		return false
	}
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// listMaterialNames は材質名の配列を現在順で返す。
func listMaterialNames(modelData *ModelData) []string {
	if modelData == nil || modelData.Materials == nil {
		return []string{}
	}
	names := make([]string, 0, modelData.Materials.Len())
	for i, materialData := range modelData.Materials.Values() {
		names = append(names, resolveMaterialName(materialData, i))
	}
	return names
}

// resolveMaterialName はログ出力用に材質名を整形する。
func resolveMaterialName(materialData *model.Material, index int) string {
	if materialData == nil {
		return fmt.Sprintf("<nil:%d>", index)
	}
	name := strings.TrimSpace(materialData.Name())
	if name == "" {
		return fmt.Sprintf("<index:%d>", index)
	}
	return name
}

// logTransparentMaterialSnapshots は半透明材質スナップショットをテストログへ出力する。
func logTransparentMaterialSnapshots(t *testing.T, phase string, snapshots []transparentMaterialSnapshot, bodyPointCount int) {
	t.Helper()
	t.Logf("[%s] bodyPoints=%d transparentMaterials=%d", phase, bodyPointCount, len(snapshots))
	for _, s := range snapshots {
		if s.HasScore {
			t.Logf("[%s] index=%d name=%q score=%.6f alpha=%.4f texture=%d", phase, s.Index, s.Name, s.Score, s.Alpha, s.TextureID)
			continue
		}
		t.Logf("[%s] index=%d name=%q score=NA alpha=%.4f texture=%d", phase, s.Index, s.Name, s.Alpha, s.TextureID)
	}
}

// logOverlapPairScores は重なり比較のペアスコアをログ出力する。
func logOverlapPairScores(t *testing.T, modelData *ModelData, snapshots []transparentMaterialSnapshot) {
	t.Helper()
	if modelData == nil {
		return
	}
	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil {
		t.Logf("pair-score: face range error: %v", err)
		return
	}
	textureImageCache := map[int]textureImageCacheEntry{}
	materialTransparencyScores := buildMaterialTransparencyScores(
		modelData,
		faceRanges,
		textureImageCache,
		textureAlphaTransparentThreshold,
	)
	transparentIndexes := make([]int, 0, len(snapshots))
	for _, snapshot := range snapshots {
		transparentIndexes = append(transparentIndexes, snapshot.Index)
	}
	blockSize := len(transparentIndexes)
	if blockSize < 1 {
		blockSize = 1
	}
	transparentMaterialIndexSet := map[int]struct{}{}
	for _, materialIndex := range transparentIndexes {
		transparentMaterialIndexSet[materialIndex] = struct{}{}
	}
	bodyPoints := collectBodyPointsForSorting(modelData, faceRanges, transparentMaterialIndexSet, blockSize)
	if len(bodyPoints) == 0 {
		t.Log("pair-score: body points が空です")
		return
	}
	spatialInfoMap := collectMaterialSpatialInfos(modelData, faceRanges, transparentIndexes, bodyPoints, blockSize)
	modelScale := estimatePointCloudScale(bodyPoints)
	if modelScale <= 0 {
		modelScale = 1
	}
	threshold := math.Max(modelScale*overlapPointScaleRatio, overlapPointDistanceMin)
	for i := 0; i < len(transparentIndexes)-1; i++ {
		leftIndex := transparentIndexes[i]
		leftInfo, ok := spatialInfoMap[leftIndex]
		if !ok {
			continue
		}
		for j := i + 1; j < len(transparentIndexes); j++ {
			rightIndex := transparentIndexes[j]
			rightInfo, rightOK := spatialInfoMap[rightIndex]
			if !rightOK {
				continue
			}
			leftScore, rightScore, leftCoverage, rightCoverage, pairOK := calculateOverlapBodyMetrics(
				leftInfo,
				rightInfo,
				threshold,
			)
			if !pairOK {
				continue
			}
			leftName := resolveMaterialName(modelData.Materials.Values()[leftIndex], leftIndex)
			rightName := resolveMaterialName(modelData.Materials.Values()[rightIndex], rightIndex)
			leftBeforeRight, confidence, hasOrder := resolvePairOrderByOverlap(
				leftIndex,
				rightIndex,
				spatialInfoMap,
				threshold,
				materialTransparencyScores,
				nil,
			)
			t.Logf(
				"pair-score: left=%d:%s(%.6f) right=%d:%s(%.6f) delta=%.6f cov=(%.4f,%.4f) ts=(%.6f,%.6f) conf=%.4f order=%t/%t",
				leftIndex,
				leftName,
				leftScore,
				rightIndex,
				rightName,
				rightScore,
				leftScore-rightScore,
				leftCoverage,
				rightCoverage,
				materialTransparencyScores[leftIndex],
				materialTransparencyScores[rightIndex],
				confidence,
				hasOrder,
				leftBeforeRight,
			)
		}
	}
}

// buildMaterialReorderTestName はサブテスト名を返す。
func buildMaterialReorderTestName(index int, path string) string {
	base := filepath.Base(path)
	if strings.TrimSpace(base) == "" {
		base = fmt.Sprintf("material_%d", index)
	}
	return fmt.Sprintf("%02d_%s", index, base)
}

// convertWindowsPathToWslForReorderTest はLinux環境でWindowsパスをWSLパスへ変換する。
func convertWindowsPathToWslForReorderTest(path string) string {
	if runtime.GOOS != "linux" {
		return path
	}
	if len(path) < 2 || path[1] != ':' {
		return path
	}
	drive := strings.ToLower(path[:1])
	rest := strings.ReplaceAll(path[2:], "\\", "/")
	if rest == "" {
		return "/mnt/" + drive
	}
	if !strings.HasPrefix(rest, "/") {
		rest = "/" + rest
	}
	return "/mnt/" + drive + rest
}
