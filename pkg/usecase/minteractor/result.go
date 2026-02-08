// 指示: miu200521358
package minteractor

import (
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mu_vrm2pmx/pkg/usecase/port/moutput"
)

// ModelData は変換対象モデルを表す。
type ModelData = model.PmxModel

// SaveOptions は保存時オプションを表す。
type SaveOptions = moutput.SaveOptions

// ConvertRequest はVRM変換要求を表す。
type ConvertRequest struct {
	InputPath  string
	OutputPath string
	ModelData  *ModelData
	Reader     moutput.IFileReader
}

// ConvertResult はVRM変換結果を表す。
type ConvertResult struct {
	Model      *ModelData
	OutputPath string
}
