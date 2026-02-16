// 指示: miu200521358
package vrm

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/domain/model"
)

const (
	groupMorphRatioAssertTolerance = 1e-6
)

var defaultMorphGroupTestCases = []morphGroupTestCase{
	{
		Name:      "0426_2_v2.1.4_export_csv",
		ModelPath: "E:/MMD_E/202101_vroid/Vrm/0426_2_v2.1.4.vrm",
		CSVPath:   "E:/MMD_E/202101_vroid/Vrm/0426_2_v2.1.4/20250426_150205/morph.csv",
	},
}

type morphGroupTestCase struct {
	Name      string
	ModelPath string
	CSVPath   string
}

type wantGroupMorph struct {
	Name         string
	GroupOffsets []wantGroupOffset
}

type wantGroupOffset struct {
	Name  string
	Ratio float64
}

type parsedGroupOffsetRow struct {
	OffsetIndex int
	Name        string
	Ratio       float64
}

// TestGenerateGroupMorph はVRM変換後のグループモーフ構成をCSV期待値と照合する。
func TestGenerateGroupMorph(t *testing.T) {
	testCases, err := resolveMorphGroupTestCases()
	if err != nil {
		t.Fatalf("グループモーフ検証ケースの解決に失敗しました: %v", err)
	}
	if len(testCases) == 0 {
		t.Skip("グループモーフ検証ケースが未定義のためスキップ")
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			if _, err := os.Stat(testCase.ModelPath); err != nil {
				if os.IsNotExist(err) {
					t.Skip("指定VRMが見つからないためスキップ")
				}
				t.Fatalf("指定VRMの確認に失敗しました: %v", err)
			}
			if _, err := os.Stat(testCase.CSVPath); err != nil {
				if os.IsNotExist(err) {
					t.Skip("morph.csvが見つからないためスキップ")
				}
				t.Fatalf("morph.csvの確認に失敗しました: %v", err)
			}

			wantGroupMorphs := loadWantGroupMorphsFromCSV(t, testCase.CSVPath)
			if len(wantGroupMorphs) == 0 {
				t.Fatalf("morph.csvからグループモーフ期待値を取得できませんでした")
			}

			repository := NewVrmRepository()
			hashableModel, err := repository.Load(testCase.ModelPath)
			if err != nil {
				t.Fatalf("VRM読込に失敗しました: %v", err)
			}
			pmxModel, ok := hashableModel.(*model.PmxModel)
			if !ok {
				t.Fatalf("読込結果の型が不正です: %T", hashableModel)
			}

			verifyGeneratedGroupMorphs(t, pmxModel, wantGroupMorphs)
		})
	}
}

// resolveMorphGroupTestCases はグループモーフ検証ケースの絶対パスを返す。
func resolveMorphGroupTestCases() ([]morphGroupTestCase, error) {
	_, currentFilePath, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("テストファイル位置を取得できません")
	}
	baseDir := filepath.Dir(currentFilePath)
	mlibRoot := filepath.Clean(filepath.Join(baseDir, "..", "..", "..", "..", ".."))
	defaultLegacyCSVPath := filepath.Join(mlibRoot, "参考", "morph.csv")

	testCases := make([]morphGroupTestCase, 0, len(defaultMorphGroupTestCases))
	for index, testCase := range defaultMorphGroupTestCases {
		name := strings.TrimSpace(testCase.Name)
		if name == "" {
			return nil, fmt.Errorf("検証ケース名が空です: index=%d", index)
		}
		modelPath := convertWindowsPathToWslForMorphTest(testCase.ModelPath)
		if strings.TrimSpace(modelPath) == "" {
			return nil, fmt.Errorf("modelPathが空です: case=%s", name)
		}
		csvPath := strings.TrimSpace(testCase.CSVPath)
		if csvPath == "" {
			csvPath = defaultLegacyCSVPath
		}

		testCases = append(testCases, morphGroupTestCase{
			Name:      name,
			ModelPath: modelPath,
			CSVPath:   convertWindowsPathToWslForMorphTest(csvPath),
		})
	}

	return testCases, nil
}

// convertWindowsPathToWslForMorphTest はWindows形式パスをWSL形式へ変換する。
func convertWindowsPathToWslForMorphTest(path string) string {
	normalizedPath := strings.TrimSpace(path)
	if runtime.GOOS != "linux" {
		return normalizedPath
	}
	if len(normalizedPath) < 2 || normalizedPath[1] != ':' {
		return normalizedPath
	}
	drive := strings.ToLower(normalizedPath[:1])
	rest := strings.ReplaceAll(normalizedPath[2:], "\\", "/")
	if rest == "" {
		return "/mnt/" + drive
	}
	if !strings.HasPrefix(rest, "/") {
		rest = "/" + rest
	}
	return "/mnt/" + drive + rest
}

// loadWantGroupMorphsFromCSV はmorph.csvからグループモーフ期待値を読み込む。
func loadWantGroupMorphsFromCSV(t *testing.T, csvPath string) []wantGroupMorph {
	t.Helper()
	csvFile, err := os.Open(csvPath)
	if err != nil {
		t.Fatalf("morph.csvのオープンに失敗しました: %v", err)
	}
	defer func() {
		if closeErr := csvFile.Close(); closeErr != nil {
			t.Fatalf("morph.csvのクローズに失敗しました: %v", closeErr)
		}
	}()

	reader := csv.NewReader(csvFile)
	reader.FieldsPerRecord = -1

	groupMorphOrder := make([]string, 0, 64)
	groupMorphSet := map[string]struct{}{}
	offsetsByParent := map[string][]parsedGroupOffsetRow{}

	rowNo := 0
	for {
		row, readErr := reader.Read()
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			t.Fatalf("morph.csvの読み込みに失敗しました: row=%d err=%v", rowNo+1, readErr)
		}
		rowNo++
		if len(row) == 0 {
			continue
		}

		recordType := normalizeMorphCSVCell(row[0])
		if recordType == "" || strings.HasPrefix(recordType, ";") {
			continue
		}

		switch recordType {
		case "PmxMorph":
			if len(row) < 5 {
				t.Fatalf("PmxMorph行の列数が不足しています: row=%d cols=%d", rowNo, len(row))
			}
			morphType, parseErr := strconv.Atoi(normalizeMorphCSVCell(row[4]))
			if parseErr != nil {
				t.Fatalf("PmxMorphのモーフ種類解析に失敗しました: row=%d value=%s err=%v", rowNo, row[4], parseErr)
			}
			if morphType != int(model.MORPH_TYPE_GROUP) {
				continue
			}
			morphName := normalizeMorphCSVCell(row[1])
			if morphName == "" {
				continue
			}
			if _, exists := groupMorphSet[morphName]; exists {
				continue
			}
			groupMorphSet[morphName] = struct{}{}
			groupMorphOrder = append(groupMorphOrder, morphName)

		case "PmxGroupMorph":
			if len(row) < 5 {
				t.Fatalf("PmxGroupMorph行の列数が不足しています: row=%d cols=%d", rowNo, len(row))
			}
			parentName := normalizeMorphCSVCell(row[1])
			offsetIndex, parseIndexErr := strconv.Atoi(normalizeMorphCSVCell(row[2]))
			if parseIndexErr != nil {
				t.Fatalf("PmxGroupMorphのoffsetIndex解析に失敗しました: row=%d value=%s err=%v", rowNo, row[2], parseIndexErr)
			}
			targetName := normalizeMorphCSVCell(row[3])
			ratio, parseRatioErr := strconv.ParseFloat(normalizeMorphCSVCell(row[4]), 64)
			if parseRatioErr != nil {
				t.Fatalf("PmxGroupMorphのratio解析に失敗しました: row=%d value=%s err=%v", rowNo, row[4], parseRatioErr)
			}
			offsetsByParent[parentName] = append(offsetsByParent[parentName], parsedGroupOffsetRow{
				OffsetIndex: offsetIndex,
				Name:        targetName,
				Ratio:       ratio,
			})
		}
	}

	wantGroupMorphs := make([]wantGroupMorph, 0, len(groupMorphOrder))
	for _, groupName := range groupMorphOrder {
		rows := offsetsByParent[groupName]
		sort.SliceStable(rows, func(i, j int) bool {
			return rows[i].OffsetIndex < rows[j].OffsetIndex
		})
		wantOffsets := make([]wantGroupOffset, 0, len(rows))
		for _, row := range rows {
			wantOffsets = append(wantOffsets, wantGroupOffset{
				Name:  row.Name,
				Ratio: row.Ratio,
			})
		}
		wantGroupMorphs = append(wantGroupMorphs, wantGroupMorph{
			Name:         groupName,
			GroupOffsets: wantOffsets,
		})
	}

	return wantGroupMorphs
}

// normalizeMorphCSVCell はCSVセル値を正規化する。
func normalizeMorphCSVCell(value string) string {
	return strings.TrimSpace(strings.TrimPrefix(value, "\ufeff"))
}

// verifyGeneratedGroupMorphs は生成されたグループモーフが期待値に一致するか検証する。
func verifyGeneratedGroupMorphs(t *testing.T, pmxModel *model.PmxModel, wantGroupMorphs []wantGroupMorph) {
	t.Helper()
	if pmxModel == nil || pmxModel.Morphs == nil {
		t.Fatalf("検証対象モデルが不正です")
	}

	for _, wantMorph := range wantGroupMorphs {
		generatedMorph, err := pmxModel.Morphs.GetByName(wantMorph.Name)
		if err != nil || generatedMorph == nil {
			t.Fatalf("グループモーフが見つかりません: morph=%s err=%v", wantMorph.Name, err)
		}
		if generatedMorph.MorphType != model.MORPH_TYPE_GROUP {
			t.Fatalf("モーフ種類が不正です: morph=%s got=%d want=%d", wantMorph.Name, generatedMorph.MorphType, model.MORPH_TYPE_GROUP)
		}

		gotOffsets, resolveErr := resolveGroupMorphOffsetsByName(pmxModel, generatedMorph)
		if resolveErr != nil {
			t.Fatalf("グループオフセット解決に失敗しました: morph=%s err=%v", wantMorph.Name, resolveErr)
		}
		if len(gotOffsets) != len(wantMorph.GroupOffsets) {
			t.Fatalf(
				"グループオフセット数が不一致です: morph=%s got=%d want=%d",
				wantMorph.Name,
				len(gotOffsets),
				len(wantMorph.GroupOffsets),
			)
		}

		for offsetIndex, wantOffset := range wantMorph.GroupOffsets {
			gotOffset := gotOffsets[offsetIndex]
			if gotOffset.Name != wantOffset.Name {
				t.Fatalf(
					"グループオフセット名が不一致です: morph=%s index=%d got=%s want=%s",
					wantMorph.Name,
					offsetIndex,
					gotOffset.Name,
					wantOffset.Name,
				)
			}
			diff := gotOffset.Ratio - wantOffset.Ratio
			if diff < 0 {
				diff = -diff
			}
			if diff > groupMorphRatioAssertTolerance {
				t.Fatalf(
					"グループオフセット比率が不一致です: morph=%s index=%d target=%s got=%.12f want=%.12f",
					wantMorph.Name,
					offsetIndex,
					wantOffset.Name,
					gotOffset.Ratio,
					wantOffset.Ratio,
				)
			}
		}
	}
}

// resolveGroupMorphOffsetsByName はグループモーフの参照先モーフ名と比率を返す。
func resolveGroupMorphOffsetsByName(pmxModel *model.PmxModel, groupMorph *model.Morph) ([]wantGroupOffset, error) {
	if pmxModel == nil || pmxModel.Morphs == nil || groupMorph == nil {
		return nil, fmt.Errorf("入力が不正です")
	}
	gotOffsets := make([]wantGroupOffset, 0, len(groupMorph.Offsets))
	for offsetIndex, rawOffset := range groupMorph.Offsets {
		offsetData, ok := rawOffset.(*model.GroupMorphOffset)
		if !ok || offsetData == nil {
			return nil, fmt.Errorf("offset型がGroupMorphOffsetではありません: index=%d type=%T", offsetIndex, rawOffset)
		}
		targetMorph, err := pmxModel.Morphs.Get(offsetData.MorphIndex)
		if err != nil || targetMorph == nil {
			return nil, fmt.Errorf("参照先モーフが見つかりません: index=%d targetIndex=%d err=%v", offsetIndex, offsetData.MorphIndex, err)
		}
		gotOffsets = append(gotOffsets, wantGroupOffset{
			Name:  targetMorph.Name(),
			Ratio: offsetData.MorphFactor,
		})
	}
	return gotOffsets, nil
}
