// 指示: miu200521358
package minteractor

import (
	"fmt"
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
