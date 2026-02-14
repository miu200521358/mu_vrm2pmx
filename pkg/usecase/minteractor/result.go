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

// PrepareProgressEventType は準備処理の進捗イベント種別を表す。
type PrepareProgressEventType string

const (
	// PrepareProgressEventTypeInputValidated は入力検証完了イベントを表す。
	PrepareProgressEventTypeInputValidated PrepareProgressEventType = "input_validated"
	// PrepareProgressEventTypeOutputPathResolved は出力パス解決完了イベントを表す。
	PrepareProgressEventTypeOutputPathResolved PrepareProgressEventType = "output_path_resolved"
	// PrepareProgressEventTypeModelValidated はモデル検証完了イベントを表す。
	PrepareProgressEventTypeModelValidated PrepareProgressEventType = "model_validated"
	// PrepareProgressEventTypeLayoutPrepared は出力レイアウト準備完了イベントを表す。
	PrepareProgressEventTypeLayoutPrepared PrepareProgressEventType = "layout_prepared"
	// PrepareProgressEventTypeModelPathApplied はモデル出力先パス反映完了イベントを表す。
	PrepareProgressEventTypeModelPathApplied PrepareProgressEventType = "model_path_applied"
	// PrepareProgressEventTypeReorderUvScanned はUV透明率取得完了イベントを表す。
	PrepareProgressEventTypeReorderUvScanned PrepareProgressEventType = "reorder_uv_scanned"
	// PrepareProgressEventTypeReorderTextureScanned はテクスチャ判定完了イベントを表す。
	PrepareProgressEventTypeReorderTextureScanned PrepareProgressEventType = "reorder_texture_scanned"
	// PrepareProgressEventTypeReorderBlocksPlanned は並べ替え対象ブロック計画確定イベントを表す。
	PrepareProgressEventTypeReorderBlocksPlanned PrepareProgressEventType = "reorder_blocks_planned"
	// PrepareProgressEventTypeReorderBlockProcessed は並べ替えブロック進行イベントを表す。
	PrepareProgressEventTypeReorderBlockProcessed PrepareProgressEventType = "reorder_block_processed"
	// PrepareProgressEventTypeReorderCompleted は材質並べ替え完了イベントを表す。
	PrepareProgressEventTypeReorderCompleted PrepareProgressEventType = "reorder_completed"
	// PrepareProgressEventTypeBoneMappingCompleted はボーンマッピング完了イベントを表す。
	PrepareProgressEventTypeBoneMappingCompleted PrepareProgressEventType = "bone_mapping_completed"
	// PrepareProgressEventTypeAstanceCompleted はAスタンス変換完了イベントを表す。
	PrepareProgressEventTypeAstanceCompleted PrepareProgressEventType = "a_stance_completed"
)

// PrepareProgressEvent は準備処理の進捗イベントを表す。
type PrepareProgressEvent struct {
	Type         PrepareProgressEventType
	TextureCount int
	PairCount    int
	BlockCount   int
}

// IPrepareProgressReporter は準備処理の進捗通知契約を表す。
type IPrepareProgressReporter interface {
	// ReportPrepareProgress は準備処理進捗を通知する。
	ReportPrepareProgress(event PrepareProgressEvent)
}

// ConvertRequest はVRM変換要求を表す。
type ConvertRequest struct {
	InputPath        string
	OutputPath       string
	ModelData        *ModelData
	Reader           moutput.IFileReader
	ProgressReporter IPrepareProgressReporter
}

// ConvertResult はVRM変換結果を表す。
type ConvertResult struct {
	Model      *ModelData
	OutputPath string
}
