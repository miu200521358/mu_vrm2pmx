// 指示: miu200521358
package minteractor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/miu200521358/mlib_go/pkg/domain/model"
	commonusecase "github.com/miu200521358/mlib_go/pkg/usecase"
	"github.com/miu200521358/mu_vrm2pmx/pkg/usecase/port/moutput"
)

// ConvertRequest はVRM変換要求を表す。
type ConvertRequest struct {
	InputPath   string
	OutputPath  string
	ModelData   *model.PmxModel
	Reader      moutput.IFileReader
	Writer      moutput.IFileWriter
	SaveOptions moutput.SaveOptions
}

// ConvertResult はVRM変換結果を表す。
type ConvertResult struct {
	Model      *model.PmxModel
	OutputPath string
}

// LoadModel はVRMモデルを読み込む。
func (uc *Vrm2PmxUsecase) LoadModel(rep moutput.IFileReader, path string) (*model.PmxModel, error) {
	repo := rep
	if repo == nil {
		repo = uc.modelReader
	}
	if repo == nil {
		return nil, fmt.Errorf("モデル読み込みリポジトリが設定されていません")
	}
	return commonusecase.LoadModel(repo, path)
}

// SaveModel はPMXモデルを保存する。
func (uc *Vrm2PmxUsecase) SaveModel(rep moutput.IFileWriter, path string, modelData *model.PmxModel, opts moutput.SaveOptions) error {
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

// Convert はVRM入力を読み込み、PMXとして保存する。
func (uc *Vrm2PmxUsecase) Convert(request ConvertRequest) (*ConvertResult, error) {
	if strings.TrimSpace(request.InputPath) == "" {
		return nil, fmt.Errorf("入力VRMパスが未指定です")
	}

	outputPath := strings.TrimSpace(request.OutputPath)
	if outputPath == "" {
		outputPath = defaultOutputPath(request.InputPath)
	}
	if strings.TrimSpace(outputPath) == "" {
		return nil, fmt.Errorf("保存先PMXパスが未指定です")
	}
	if !strings.EqualFold(filepath.Ext(outputPath), ".pmx") {
		return nil, fmt.Errorf("保存先拡張子が .pmx ではありません: %s", outputPath)
	}

	modelData := request.ModelData
	if modelData == nil {
		loaded, err := uc.LoadModel(request.Reader, request.InputPath)
		if err != nil {
			return nil, err
		}
		modelData = loaded
	}
	if modelData == nil {
		return nil, fmt.Errorf("モデル読み込み結果が空です")
	}
	if modelData.VrmData == nil {
		return nil, fmt.Errorf("VRMデータが見つかりません")
	}

	if err := uc.SaveModel(request.Writer, outputPath, modelData, request.SaveOptions); err != nil {
		return nil, err
	}
	return &ConvertResult{Model: modelData, OutputPath: outputPath}, nil
}

// defaultOutputPath は入力パスから既定のPMX出力パスを生成する。
func defaultOutputPath(inputPath string) string {
	dir := filepath.Dir(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	if strings.TrimSpace(base) == "" {
		return ""
	}
	return filepath.Join(dir, base+".pmx")
}
