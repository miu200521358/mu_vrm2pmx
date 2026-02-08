// 指示: miu200521358
package minteractor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/adapter/io_model/vrm"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
)

type materialTestStruct struct {
	Path          string   // vrmフルパス
	WantMaterials []string // 期待する材質の並び順
}

var materialTests = []materialTestStruct{
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/Akami - 【朱巳】あかみ -アカミ【Akami】.vrm",
		WantMaterials: []string{
			"N00_000_00_FaceMouth_00_FACE (Instance)",
			"N00_000_00_EyeIris_00_EYE (Instance)",
			"N00_000_00_EyeHighlight_00_EYE (Instance)",
			"N00_000_00_Face_00_SKIN (Instance)",
			"N00_000_00_EyeWhite_00_EYE (Instance)",
			"N00_000_00_FaceBrow_00_FACE (Instance)",
			"N00_000_00_FaceEyelash_00_FACE (Instance)",
			"N00_000_00_FaceEyeline_00_FACE (Instance)",
			"N00_000_00_Body_00_SKIN (Instance)",
			"N00_004_01_Shoes_01_CLOTH (Instance)",
			"N00_000_00_HairBack_00_HAIR (Instance)",
			"N00_010_01_Onepiece_00_CLOTH (Instance)",
			"N00_002_01_Tops_01_CLOTH_02 (Instance)",
			"N00_002_01_Tops_01_CLOTH_01 (Instance)",
			"N00_007_01_Tops_01_CLOTH (Instance)",
			"N00_002_01_Tops_01_CLOTH_03 (Instance)",
			"N00_000_Hair_00_HAIR_01 (Instance)",
			"N00_000_Hair_00_HAIR_02 (Instance)",
			"N00_000_Hair_00_HAIR_03 (Instance)",
		},
	},
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/Liliana.vrm",
		WantMaterials: []string{
			"N00_000_00_FaceMouth_00_FACE (Instance)",
			"N00_000_00_EyeIris_00_EYE (Instance)",
			"N00_000_00_EyeHighlight_00_EYE (Instance)",
			"N00_000_00_Face_00_SKIN (Instance)",
			"N00_000_00_EyeWhite_00_EYE (Instance)",
			"N00_000_00_FaceBrow_00_FACE (Instance)",
			"N00_000_00_FaceEyeline_00_FACE (Instance)",
			"N00_000_00_Body_00_SKIN (Instance)",
			"N00_010_01_Onepiece_00_CLOTH_01 (Instance)",
			"N00_010_01_Onepiece_00_CLOTH_02 (Instance)",
			"N00_010_01_Onepiece_00_CLOTH_03 (Instance)",
			"N00_003_01_Bottoms_01_CLOTH (Instance)",
			"N00_002_01_Tops_01_CLOTH (Instance)",
			"N00_008_01_Shoes_01_CLOTH_01 (Instance)",
			"N00_008_01_Shoes_01_CLOTH_02 (Instance)",
			"N00_000_Hair_00_HAIR_01 (Instance)",
			"N00_000_Hair_00_HAIR_02 (Instance)",
			"N00_000_Hair_00_HAIR_03 (Instance)",
		},
	},
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/ricos - リコス.vrm",
		WantMaterials: []string{
			"N00_000_00_FaceMouth_00_FACE (Instance)",
			"N00_000_00_EyeIris_00_EYE (Instance)",
			"N00_000_00_EyeHighlight_00_EYE (Instance)",
			"N00_000_00_Face_00_SKIN (Instance)",
			"N00_000_00_EyeWhite_00_EYE (Instance)",
			"N00_000_00_FaceBrow_00_FACE (Instance)",
			"N00_000_00_FaceEyelash_00_FACE (Instance)",
			"N00_000_00_FaceEyeline_00_FACE (Instance)",
			"N00_000_00_Body_00_SKIN (Instance)",
			"N00_010_01_Onepiece_00_CLOTH_01 (Instance)",
			"N00_003_01_Shoes_01_CLOTH (Instance)",
			"N00_000_00_HairBack_00_HAIR (Instance)",
			"N00_010_01_Onepiece_00_CLOTH_02 (Instance)",
			"N00_002_01_Shoes_01_CLOTH (Instance)",
			"N00_002_03_Tops_01_CLOTH_03 (Instance)",
			"N00_002_03_Tops_01_CLOTH_04 (Instance)",
			"N00_002_03_Tops_01_CLOTH_02 (Instance)",
			"N00_002_03_Tops_01_CLOTH_01 (Instance)",
			"N00_010_01_Onepiece_00_CLOTH_03 (Instance)",
			"N00_000_Hair_00_HAIR_01 (Instance)",
			"N00_000_Hair_00_HAIR_02 (Instance)",
		},
	},
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/Yelena.vrm",
		WantMaterials: []string{
			"N00_000_00_FaceMouth_00_FACE (Instance)",
			"N00_000_00_EyeIris_00_EYE (Instance)",
			"N00_000_00_EyeHighlight_00_EYE (Instance)",
			"N00_000_00_Face_00_SKIN (Instance)",
			"N00_000_00_EyeWhite_00_EYE (Instance)",
			"N00_000_00_FaceBrow_00_FACE (Instance)",
			"N00_000_00_FaceEyelash_00_FACE (Instance)",
			"N00_000_00_FaceEyeline_00_FACE (Instance)",
			"N00_000_00_Body_00_SKIN (Instance)",
			"N00_000_00_HairBack_00_HAIR (Instance)",
			"N00_002_04_Tops_01_CLOTH (Instance)",
			"N00_001_03_Bottoms_01_CLOTH (Instance)",
			"N00_007_01_Tops_01_CLOTH (Instance)",
			"N00_010_01_Onepiece_00_CLOTH (Instance)",
			"N00_009_01_Shoes_01_CLOTH_01 (Instance)",
			"N00_009_01_Shoes_01_CLOTH_02 (Instance)",
			"N00_008_01_Shoes_01_CLOTH (Instance)",
			"N00_000_Hair_00_HAIR_01 (Instance)",
			"N00_000_Hair_00_HAIR_02 (Instance)",
		},
	},
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/いおり 赤色スタジアムジャンパー.vrm",
		WantMaterials: []string{
			"N00_000_00_FaceMouth_00_FACE (Instance)",
			"N00_000_00_EyeIris_00_EYE (Instance)",
			"N00_000_00_EyeHighlight_00_EYE (Instance)",
			"N00_000_00_Face_00_SKIN (Instance)",
			"N00_000_00_EyeWhite_00_EYE (Instance)",
			"N00_000_00_FaceBrow_00_FACE (Instance)",
			"N00_000_00_FaceEyelash_00_FACE (Instance)",
			"N00_000_00_FaceEyeline_00_FACE (Instance)",
			"N00_000_00_Body_00_SKIN (Instance)",
			"N00_000_00_HairBack_00_HAIR (Instance)",
			"Accessory_WitchHat_01_CLOTH (Instance)",
			"N00_007_03_Tops_01_CLOTH_01 (Instance)",
			"N00_006_01_Shoes_01_CLOTH (Instance)",
			"N00_010_01_Onepiece_00_CLOTH (Instance)",
			"N00_007_01_Tops_01_CLOTH (Instance)",
			"N00_011_02_Bottoms_01_CLOTH_02 (Instance)",
			"N00_011_02_Bottoms_01_CLOTH_01 (Instance)",
			"N00_007_03_Tops_01_CLOTH_02 (Instance)",
			"N00_000_Hair_00_HAIR_01 (Instance)",
			"N00_000_Hair_00_HAIR_02 (Instance)",
			"N00_000_Hair_00_HAIR_03 (Instance)",
			"N00_000_Hair_00_HAIR_05 (Instance)",
		},
	},
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/オリジナル - 【DL可】情熱の真っ赤なやえちゃん.vrm",
		WantMaterials: []string{
			"ref_A",
			"body",
			"ref_B",
			"tp",
			"face",
			"eco",
			"body",
			"ref_A",
			"hair",
			"rose_R",
		},
	},
	{
		Path: "E:/MMD_E/202101_vroid/Vrm/Hub2/ゆき - Yuki (DL OK) - ゆき - Yuki Ver.009s2505春.vrm",
		WantMaterials: []string{
			"N00_000_00_FaceMouth_00_FACE (Instance)",
			"N00_000_00_EyeIris_00_EYE (Instance)",
			"N00_000_00_EyeHighlight_00_EYE (Instance)",
			"N00_000_00_Face_00_SKIN (Instance)",
			"N00_000_00_EyeWhite_00_EYE (Instance)",
			"N00_000_00_FaceBrow_00_FACE (Instance)",
			"N00_000_00_FaceEyelash_00_FACE (Instance)",
			"N00_000_00_FaceEyeline_00_FACE (Instance)",
			"N00_000_00_Body_00_SKIN (Instance)",
			"N00_000_00_HairBack_00_HAIR (Instance)",
			"N00_008_01_Shoes_01_CLOTH_01 (Instance)",
			"N00_010_01_Onepiece_00_CLOTH (Instance)",
			"N00_002_03_Tops_01_CLOTH_04 (Instance)",
			"N00_002_03_Tops_01_CLOTH_01 (Instance)",
			"N00_002_03_Tops_01_CLOTH_02 (Instance)",
			"N00_002_03_Tops_01_CLOTH_03 (Instance)",
			"N00_008_01_Shoes_01_CLOTH_02 (Instance)",
			"N00_000_Hair_00_HAIR_01 (Instance)",
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

			logTransparentMaterialSummary(t, "after", len(after))
			if len(after) >= 2 {
				if err := verifyTransparentScoreOrder(after); err != nil {
					t.Fatalf("並べ替え後のスコア順が不正です: %v", err)
				}
			}
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
	textureAlphaCache := map[int]textureAlphaCacheEntry{}
	bodyPoints := collectBodyPointsForSorting(modelData, faceRanges, textureAlphaCache)

	snapshots := make([]transparentMaterialSnapshot, 0)
	for materialIndex, materialData := range modelData.Materials.Values() {
		if !isTransparentMaterial(modelData, materialData, textureAlphaCache) {
			continue
		}
		score, ok := calculateBodyProximityScore(modelData, faceRanges[materialIndex], bodyPoints)
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

// verifyTransparentScoreOrder は半透明材質がボディ近傍スコア昇順か確認する。
func verifyTransparentScoreOrder(snapshots []transparentMaterialSnapshot) error {
	if len(snapshots) < 2 {
		return nil
	}
	const eps = 1e-7
	previous := snapshots[0]
	for i := 1; i < len(snapshots); i++ {
		current := snapshots[i]
		if previous.HasScore && current.HasScore && previous.Score > current.Score+eps {
			return fmt.Errorf(
				"index=%d(%s score=%.6f) が index=%d(%s score=%.6f) より後方にあるべきです",
				previous.Index, previous.Name, previous.Score,
				current.Index, current.Name, current.Score,
			)
		}
		previous = current
	}
	return nil
}

// verifyMaterialOrder は材質並びが期待値と一致するか確認する。
func verifyMaterialOrder(modelData *ModelData, wantMaterials []string) error {
	if len(wantMaterials) == 0 {
		return nil
	}
	gotMaterials := listMaterialNames(modelData)
	if len(gotMaterials) != len(wantMaterials) {
		return fmt.Errorf("材質数不一致: got=%d want=%d", len(gotMaterials), len(wantMaterials))
	}

	for i := range wantMaterials {
		if gotMaterials[i] == wantMaterials[i] {
			continue
		}
		return fmt.Errorf("index=%d got=%q want=%q", i, gotMaterials[i], wantMaterials[i])
	}
	return nil
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
