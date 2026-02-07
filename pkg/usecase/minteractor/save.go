// 指示: miu200521358
package minteractor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/miu200521358/mu_vrm2pmx/pkg/usecase/port/moutput"
)

// SaveModel はPMXモデルを保存する。
func (uc *Vrm2PmxUsecase) SaveModel(rep moutput.IFileWriter, path string, modelData *ModelData, opts SaveOptions) error {
	writer := rep
	if writer == nil {
		writer = uc.modelWriter
	}
	if writer == nil {
		return fmt.Errorf("モデル保存リポジトリが設定されていません")
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("保存先パスが未指定です")
	}
	if modelData == nil {
		return fmt.Errorf("保存対象モデルが未設定です")
	}
	return writer.Save(path, modelData, opts)
}

// PrepareModelForOutput はVRM入力を読み込み、PMX出力用の補助ファイルを準備する。
// PMX本体ファイルは保存しない。
func (uc *Vrm2PmxUsecase) PrepareModelForOutput(request ConvertRequest) (*ConvertResult, error) {
	if strings.TrimSpace(request.InputPath) == "" {
		return nil, fmt.Errorf("入力VRMパスが未指定です")
	}

	outputPath, err := resolvePmxOutputPath(request.InputPath, request.OutputPath)
	if err != nil {
		return nil, err
	}

	modelData, err := uc.resolveModelData(request.Reader, request.InputPath, request.ModelData)
	if err != nil {
		return nil, err
	}
	if err := prepareOutputLayout(request.InputPath, outputPath, modelData); err != nil {
		return nil, err
	}

	// プレビュー時に相対テクスチャを解決できるよう、保存先候補をモデルパスへ反映する。
	modelData.SetPath(outputPath)

	return &ConvertResult{Model: modelData, OutputPath: outputPath}, nil
}

// Convert はVRM入力を読み込み、PMXとして保存する。
func (uc *Vrm2PmxUsecase) Convert(request ConvertRequest) (*ConvertResult, error) {
	result, err := uc.PrepareModelForOutput(request)
	if err != nil {
		return nil, err
	}
	if err := uc.SaveModel(request.Writer, result.OutputPath, result.Model, request.SaveOptions); err != nil {
		return nil, err
	}
	return result, nil
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
