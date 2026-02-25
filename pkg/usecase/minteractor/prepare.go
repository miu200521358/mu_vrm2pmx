// 指示: miu200521358
package minteractor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/miu200521358/mu_vrm2pmx/pkg/usecase/port/moutput"
)

// PrepareModel はVRM入力を読み込み、PMX出力用の補助ファイルを準備する。
// PMX本体ファイルは保存しない。
func (uc *Vrm2PmxUsecase) PrepareModel(request ConvertRequest) (*ConvertResult, error) {
	if strings.TrimSpace(request.InputPath) == "" {
		return nil, fmt.Errorf("入力VRMパスが未指定です")
	}
	reportPrepareProgress(request.ProgressReporter, PrepareProgressEvent{
		Type: PrepareProgressEventTypeInputValidated,
	})

	outputPath, err := resolvePmxOutputPath(request.InputPath, request.OutputPath)
	if err != nil {
		return nil, err
	}
	reportPrepareProgress(request.ProgressReporter, PrepareProgressEvent{
		Type: PrepareProgressEventTypeOutputPathResolved,
	})

	modelData, err := uc.resolveModelData(request.Reader, request.InputPath, request.ModelData)
	if err != nil {
		return nil, err
	}
	reportPrepareProgress(request.ProgressReporter, PrepareProgressEvent{
		Type: PrepareProgressEventTypeModelValidated,
	})
	if err := prepareOutputLayout(request.InputPath, outputPath, modelData); err != nil {
		return nil, err
	}
	reportPrepareProgress(request.ProgressReporter, PrepareProgressEvent{
		Type: PrepareProgressEventTypeLayoutPrepared,
	})

	// プレビュー時に相対テクスチャを解決できるよう、保存先候補をモデルパスへ反映する。
	modelData.SetPath(outputPath)
	reportPrepareProgress(request.ProgressReporter, PrepareProgressEvent{
		Type: PrepareProgressEventTypeModelPathApplied,
	})
	// 旧VroidExportService準拠対象は、材質バリアント(表面/裏面/エッジ)準備を材質並べ替え前に固定する範囲までとする。
	if err := prepareVroidMaterialVariantsBeforeReorder(modelData); err != nil {
		return nil, fmt.Errorf("VRoid材質バリアント準備に失敗しました: %w", err)
	}
	reportPrepareProgress(request.ProgressReporter, PrepareProgressEvent{
		Type: PrepareProgressEventTypeVroidMaterialPrepared,
	})
	if err := abbreviateMaterialNamesBeforeReorder(modelData); err != nil {
		return nil, fmt.Errorf("材質名略称処理に失敗しました: %w", err)
	}
	applyBodyDepthMaterialOrderWithProgress(modelData, request.ProgressReporter)
	if err := applyHumanoidBoneMappingAfterReorder(modelData); err != nil {
		return nil, fmt.Errorf("ボーンマッピング処理に失敗しました: %w", err)
	}
	reportPrepareProgress(request.ProgressReporter, PrepareProgressEvent{
		Type: PrepareProgressEventTypeBoneMappingCompleted,
	})
	if err := applyAstanceBeforeViewer(modelData); err != nil {
		return nil, fmt.Errorf("Aスタンス変換処理に失敗しました: %w", err)
	}
	reportPrepareProgress(request.ProgressReporter, PrepareProgressEvent{
		Type: PrepareProgressEventTypeAstanceCompleted,
	})
	applyMorphRenameOnlyBeforeViewer(modelData, request.ProgressReporter)

	return &ConvertResult{Model: modelData, OutputPath: outputPath}, nil
}

// resolvePmxOutputPath はPMX保存先パスを解決し、拡張子を検証する。
func resolvePmxOutputPath(inputPath string, outputPath string) (string, error) {
	resolved := strings.TrimSpace(outputPath)
	if resolved == "" {
		resolved = BuildDefaultOutputPath(inputPath)
	}
	if strings.TrimSpace(resolved) == "" {
		return "", fmt.Errorf("保存先PMXパスが未指定です")
	}
	if !strings.EqualFold(filepath.Ext(resolved), ".pmx") {
		return "", fmt.Errorf("保存先拡張子が .pmx ではありません: %s", resolved)
	}
	return resolved, nil
}

// resolveModelData は変換対象モデルを解決し、VRMデータを検証する。
func (uc *Vrm2PmxUsecase) resolveModelData(rep moutput.IFileReader, inputPath string, modelData *ModelData) (*ModelData, error) {
	resolved := modelData
	if resolved == nil {
		loaded, err := uc.LoadModel(rep, inputPath)
		if err != nil {
			return nil, err
		}
		resolved = loaded
	}
	if resolved == nil {
		return nil, fmt.Errorf("モデル読み込み結果が空です")
	}
	if resolved.VrmData == nil {
		return nil, fmt.Errorf("VRMデータが見つかりません")
	}
	return resolved, nil
}

// reportPrepareProgress は準備処理の進捗を通知する。
func reportPrepareProgress(reporter IPrepareProgressReporter, event PrepareProgressEvent) {
	if reporter == nil {
		return
	}
	reporter.ReportPrepareProgress(event)
}
