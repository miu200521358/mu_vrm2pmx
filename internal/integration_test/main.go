// 指示: miu200521358
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/miu200521358/mlib_go/pkg/adapter/io_model/pmx"
	"github.com/miu200521358/mu_vrm2pmx/pkg/adapter/io_model/vrm"
	"github.com/miu200521358/mu_vrm2pmx/pkg/usecase/minteractor"
)

const (
	batchOutputDirMode = 0o755
)

var targetModelPaths = []string{
	"E:/MMD_E/202101_vroid/Vrm/Hub2/Akami - 【朱巳】あかみ -アカミ【Akami】.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/ricos - リコス.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub/Anime Girl1.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/PSO2 えめ☆いち(Vの姿).vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub/Nessa Reg Outfit.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub/Galaxy _ Galaxy.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub/Лиза Геншин.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/Liliana.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/Yelena.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/Asien.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/F2U Model.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/火華菜 衣桜梨 Iori Hibana.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/仮称 波座 さりい - 波座 さりい 2601新春ワンピVer..vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/ゆき - Yuki (DL OK) - ゆき - Yuki Ver.009s2505春.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/カルラ・グレイグ - 通常衣装.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/オリジナル - 【DL可】情熱の真っ赤なやえちゃん.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/VRoidStudio製　ラティ式ミク　配布用.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/T式ポムニちゃん_v0.96.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/T式ザンダクロス_v0.90.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/T式ウイングマン_v0.91.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/Sunspot - Cold Fusion.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/Raccoom suggestion - Outfit2.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/miku mu.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/Kana.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub/さくら_nmnngj.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub/scrap chara.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub/Клее.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub/Shin _ Shin.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub/pretty bunny gal.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub/kirito.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub/Jevy _ Beans.vrm",
	// "C:/Codex/mlib/mlib_go_t4/internal/test_resources/vrm/free character _ [FREE CHARACTER].vrm",
	// "C:/Codex/mlib/mlib_go_t4/internal/test_resources/vrm/vrm1.0_ロンスカ女子.vrm",
	// "C:/Codex/mlib/mlib_go_t4/internal/test_resources/vrm/vrm0.0_二重スカート4.vrm",
	// "C:/Codex/mlib/mlib_go_t4/internal/test_resources/vrm/other_Anime Girl1.vrm",
	// "C:/Codex/mlib/mlib_go_t4/internal/test_resources/vrm/vrm0.0_髪多色1.9.0.vrm",
	// "E:/MMD_E/202101_vroid/Vrm/Hub2/いおり 赤色スタジアムジャンパー.vrm",
	// "C:/Codex/mlib/mlib_go_t4/internal/test_resources/vrm/vrm0.0_ゴシック女子2.vrm",
}

// batchConfig はバッチ変換の実行設定を表す。
type batchConfig struct {
	OutputRoot string
	DryRun     bool
	FailFast   bool
}

// conversionEntry は1モデル分の変換入力情報を表す。
type conversionEntry struct {
	Index      int
	SourcePath string
	ModelName  string
	CaseDir    string
	OutputPath string
}

// conversionResult は1モデル分の変換結果を表す。
type conversionResult struct {
	Entry            conversionEntry
	Status           string
	Duration         time.Duration
	Err              error
	PrepareStageInfo string
}

// prepareProgressCollector は PrepareModel の進捗イベントを収集する。
type prepareProgressCollector struct {
	eventCounts map[minteractor.PrepareProgressEventType]int
	textureMax  int
	pairTotal   int
	blockTotal  int
	morphMax    int
}

// main はモーフ検証向けのVRM一括PMX変換を実行する。
func main() {
	os.Exit(run())
}

// run は実行設定を解決して一括変換を実行し、終了コードを返す。
func run() int {
	config, err := parseBatchConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "設定解析に失敗しました: %v\n", err)
		return 2
	}
	entries := buildConversionEntries(config.OutputRoot, targetModelPaths)
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "変換対象モデルがありません")
		return 2
	}

	results := executeBatchConversion(config, entries)
	printBatchSummary(results)

	hasFailed := false
	for _, result := range results {
		if result.Status == "failed" {
			hasFailed = true
			break
		}
	}
	if hasFailed {
		return 1
	}
	return 0
}

// parseBatchConfig はコマンドライン引数から実行設定を構築する。
func parseBatchConfig() (batchConfig, error) {
	defaultOutputRoot, err := resolveDefaultOutputRoot()
	if err != nil {
		return batchConfig{}, err
	}
	outputRoot := flag.String("output-root", defaultOutputRoot, "変換結果の出力ルートディレクトリ")
	dryRun := flag.Bool("dry-run", false, "実変換せず、入力解決と出力先計画のみ表示する")
	failFast := flag.Bool("fail-fast", false, "失敗時に即時終了する")
	flag.Parse()

	trimmedOutputRoot := strings.TrimSpace(*outputRoot)
	if trimmedOutputRoot == "" {
		return batchConfig{}, errors.New("output-root が空です")
	}
	return batchConfig{
		OutputRoot: filepath.Clean(trimmedOutputRoot),
		DryRun:     *dryRun,
		FailFast:   *failFast,
	}, nil
}

// resolveDefaultOutputRoot はスクリプト配置ディレクトリ基準の既定出力先を返す。
func resolveDefaultOutputRoot() (string, error) {
	_, currentFilePath, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("実行ファイル位置を取得できません")
	}
	currentDir := filepath.Dir(currentFilePath)
	return filepath.Join(currentDir, "output"), nil
}

// buildConversionEntries は入力パス一覧から変換対象エントリを生成する。
func buildConversionEntries(outputRoot string, inputPaths []string) []conversionEntry {
	entries := make([]conversionEntry, 0, len(inputPaths))
	for i, rawPath := range inputPaths {
		resolvedInputPath := normalizeInputPath(rawPath)
		modelName := resolveModelName(rawPath)
		safeModelName := sanitizePathComponent(modelName)
		caseDirName := fmt.Sprintf("%03d_%s", i+1, safeModelName)
		caseDir := filepath.Join(outputRoot, caseDirName)
		outputPath := filepath.Join(caseDir, safeModelName+".pmx")
		entries = append(entries, conversionEntry{
			Index:      i + 1,
			SourcePath: resolvedInputPath,
			ModelName:  modelName,
			CaseDir:    caseDir,
			OutputPath: outputPath,
		})
	}
	return entries
}

// executeBatchConversion は全モデルの変換処理を順次実行する。
func executeBatchConversion(config batchConfig, entries []conversionEntry) []conversionResult {
	results := make([]conversionResult, 0, len(entries))
	usecase := minteractor.NewVrm2PmxUsecase(minteractor.Vrm2PmxUsecaseDeps{
		ModelReader: vrm.NewVrmRepository(),
		ModelWriter: pmx.NewPmxRepository(),
	})

	total := len(entries)
	for _, entry := range entries {
		fmt.Printf("[%d/%d] 変換開始: model=%s\n", entry.Index, total, entry.ModelName)
		result := convertModelEntry(usecase, config, entry)
		results = append(results, result)
		switch result.Status {
		case "succeeded":
			fmt.Printf("[%d/%d] 変換成功: model=%s output=%s elapsed=%s\n", entry.Index, total, entry.ModelName, entry.OutputPath, result.Duration.Round(time.Millisecond))
			if strings.TrimSpace(result.PrepareStageInfo) != "" {
				fmt.Printf("[%d/%d] PrepareModel進捗: %s\n", entry.Index, total, result.PrepareStageInfo)
			}
		case "dry_run":
			fmt.Printf("[%d/%d] DRY-RUN: model=%s input=%s output=%s\n", entry.Index, total, entry.ModelName, entry.SourcePath, entry.OutputPath)
		case "skipped_missing":
			fmt.Printf("[%d/%d] 入力不足でスキップ: model=%s input=%s reason=%v\n", entry.Index, total, entry.ModelName, entry.SourcePath, result.Err)
		default:
			fmt.Printf("[%d/%d] 変換失敗: model=%s reason=%v\n", entry.Index, total, entry.ModelName, result.Err)
			if config.FailFast {
				return results
			}
		}
	}
	return results
}

// convertModelEntry は1モデル分の変換を実行する。
func convertModelEntry(usecase *minteractor.Vrm2PmxUsecase, config batchConfig, entry conversionEntry) conversionResult {
	result := conversionResult{
		Entry:  entry,
		Status: "failed",
	}
	if _, err := os.Stat(entry.SourcePath); err != nil {
		result.Status = "skipped_missing"
		result.Err = err
		return result
	}
	if config.DryRun {
		result.Status = "dry_run"
		return result
	}
	if err := os.MkdirAll(entry.CaseDir, batchOutputDirMode); err != nil {
		result.Err = fmt.Errorf("出力ディレクトリ作成に失敗しました: %w", err)
		return result
	}

	startedAt := time.Now()
	progressCollector := newPrepareProgressCollector()
	converted, err := usecase.LoadAndPrepareModelForViewer(minteractor.ConvertRequest{
		InputPath:        entry.SourcePath,
		OutputPath:       entry.OutputPath,
		ProgressReporter: progressCollector,
	})
	if err != nil {
		result.Err = fmt.Errorf("LoadAndPrepareModelForViewerに失敗しました: %w", err)
		return result
	}
	if converted == nil || converted.Model == nil {
		result.Err = errors.New("LoadAndPrepareModelForViewer結果が空です")
		return result
	}
	// UI の変換ボタンと同様に、保存直前に出力先パスをモデルへ再設定する。
	converted.Model.SetPath(converted.OutputPath)
	if err := usecase.SaveModel(nil, converted.OutputPath, converted.Model, minteractor.SaveOptions{}); err != nil {
		result.Err = fmt.Errorf("SaveModelに失敗しました: %w", err)
		return result
	}

	result.Status = "succeeded"
	result.Duration = time.Since(startedAt)
	result.PrepareStageInfo = progressCollector.Summary()
	return result
}

// printBatchSummary は変換結果の集計を標準出力へ表示する。
func printBatchSummary(results []conversionResult) {
	succeeded := 0
	failed := 0
	skipped := 0
	dryRun := 0
	for _, result := range results {
		switch result.Status {
		case "succeeded":
			succeeded++
		case "dry_run":
			dryRun++
		case "skipped_missing":
			skipped++
		default:
			failed++
		}
	}
	fmt.Printf(
		"バッチ変換サマリ: total=%d succeeded=%d failed=%d skipped_missing=%d dry_run=%d\n",
		len(results),
		succeeded,
		failed,
		skipped,
		dryRun,
	)
}

// resolveModelName は入力パスから拡張子を除いたモデル名を返す。
func resolveModelName(path string) string {
	base := strings.TrimSpace(filepath.Base(path))
	ext := filepath.Ext(base)
	name := strings.TrimSpace(strings.TrimSuffix(base, ext))
	if name == "" {
		return "model"
	}
	return name
}

// normalizeInputPath は入力パスを実行環境向けに正規化する。
func normalizeInputPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(convertWindowsPathToWsl(path))
}

// convertWindowsPathToWsl は Linux 実行時に Windows パスを WSL パスへ変換する。
func convertWindowsPathToWsl(path string) string {
	trimmed := strings.TrimSpace(path)
	if runtime.GOOS != "linux" {
		return trimmed
	}
	if len(trimmed) < 2 || trimmed[1] != ':' {
		return trimmed
	}
	drive := strings.ToLower(trimmed[:1])
	rest := strings.ReplaceAll(trimmed[2:], "\\", "/")
	if rest == "" {
		return filepath.ToSlash(filepath.Join("/mnt", drive))
	}
	if !strings.HasPrefix(rest, "/") {
		rest = "/" + rest
	}
	return filepath.ToSlash(filepath.Join("/mnt", drive) + rest)
}

// sanitizePathComponent は出力ディレクトリ/ファイル名に使えない文字を置換する。
func sanitizePathComponent(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "model"
	}
	replaced := strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '_'
		default:
			if r < 0x20 {
				return '_'
			}
			return r
		}
	}, trimmed)
	replaced = strings.Trim(replaced, " .")
	if replaced == "" {
		return "model"
	}
	return replaced
}

// newPrepareProgressCollector は PrepareModel 進捗収集器を生成する。
func newPrepareProgressCollector() *prepareProgressCollector {
	return &prepareProgressCollector{
		eventCounts: map[minteractor.PrepareProgressEventType]int{},
	}
}

// ReportPrepareProgress は PrepareModel の進捗イベントを収集する。
func (collector *prepareProgressCollector) ReportPrepareProgress(event minteractor.PrepareProgressEvent) {
	if collector == nil {
		return
	}
	if collector.eventCounts == nil {
		collector.eventCounts = map[minteractor.PrepareProgressEventType]int{}
	}
	collector.eventCounts[event.Type]++
	if event.TextureCount > collector.textureMax {
		collector.textureMax = event.TextureCount
	}
	collector.pairTotal += event.PairCount
	collector.blockTotal += event.BlockCount
	if event.MorphCount > collector.morphMax {
		collector.morphMax = event.MorphCount
	}
}

// Summary は収集した PrepareModel 進捗の要約文字列を返す。
func (collector *prepareProgressCollector) Summary() string {
	if collector == nil || len(collector.eventCounts) == 0 {
		return ""
	}
	types := make([]string, 0, len(collector.eventCounts))
	for stageType := range collector.eventCounts {
		types = append(types, string(stageType))
	}
	sort.Strings(types)
	return fmt.Sprintf(
		"events=%d textures=%d pairProgress=%d blockProgress=%d morphMax=%d stages=%s",
		len(collector.eventCounts),
		collector.textureMax,
		collector.pairTotal,
		collector.blockTotal,
		collector.morphMax,
		strings.Join(types, ","),
	)
}
