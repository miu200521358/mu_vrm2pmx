// 指示: miu200521358
package minteractor

import "github.com/miu200521358/mu_vrm2pmx/pkg/usecase/port/moutput"

// Vrm2PmxUsecaseDeps はVRM変換ユースケースの依存を表す。
type Vrm2PmxUsecaseDeps struct {
	ModelReader moutput.IFileReader
	ModelWriter moutput.IFileWriter
}

// Vrm2PmxUsecase はVRMからPMXへの変換処理をまとめたユースケースを表す。
type Vrm2PmxUsecase struct {
	modelReader moutput.IFileReader
	modelWriter moutput.IFileWriter
}

// NewVrm2PmxUsecase はVRM変換ユースケースを生成する。
func NewVrm2PmxUsecase(deps Vrm2PmxUsecaseDeps) *Vrm2PmxUsecase {
	return &Vrm2PmxUsecase{
		modelReader: deps.ModelReader,
		modelWriter: deps.ModelWriter,
	}
}
