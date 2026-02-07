// 指示: miu200521358
package minteractor

import (
	"fmt"

	"github.com/miu200521358/mlib_go/pkg/usecase"
	"github.com/miu200521358/mu_vrm2pmx/pkg/usecase/port/moutput"
)

// LoadModel はVRMモデルを読み込む。
func (uc *Vrm2PmxUsecase) LoadModel(rep moutput.IFileReader, path string) (*ModelData, error) {
	repo := rep
	if repo == nil {
		repo = uc.modelReader
	}
	if repo == nil {
		return nil, fmt.Errorf("モデル読み込みリポジトリが設定されていません")
	}
	return usecase.LoadModel(repo, path)
}
