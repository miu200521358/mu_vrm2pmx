//go:build windows
// +build windows

// 指示: miu200521358
package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/miu200521358/mlib_go/pkg/adapter/audio_api"
	"github.com/miu200521358/mlib_go/pkg/adapter/io_common"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/motion"
	"github.com/miu200521358/mlib_go/pkg/infra/controller"
	"github.com/miu200521358/mlib_go/pkg/infra/controller/widget"
	"github.com/miu200521358/mlib_go/pkg/shared/base"
	"github.com/miu200521358/mlib_go/pkg/shared/base/config"
	"github.com/miu200521358/mlib_go/pkg/shared/base/i18n"
	"github.com/miu200521358/mlib_go/pkg/shared/base/logging"
	"github.com/miu200521358/mlib_go/pkg/usecase"
	"github.com/miu200521358/walk/pkg/declarative"
	"github.com/miu200521358/walk/pkg/walk"

	"github.com/miu200521358/mu_vrm2pmx/pkg/adapter/io_model/vrm"
	"github.com/miu200521358/mu_vrm2pmx/pkg/adapter/mpresenter/messages"
	"github.com/miu200521358/mu_vrm2pmx/pkg/usecase/minteractor"
)

const (
	vrmHistoryKey      = "vrm"
	motionHistoryKey   = "vmd"
	previewWindowIndex = 0
	previewModelIndex  = 0

	loadIoChunkBytes        = 8 * 1024 * 1024
	loadParseChunkSize      = 500
	loadPrimitiveChunkSize  = 50
	reorderTextureChunkSize = 8
	reorderPairChunkSize    = 200
	reorderBlockChunkSize   = 4
	morphRenameChunkSize    = 25
)

// prepareProgressStage は準備処理進捗のステージ識別子を表す。
type prepareProgressStage string

const (
	prepareProgressStageInputValidated       prepareProgressStage = "input_validated"
	prepareProgressStageOutputResolved       prepareProgressStage = "output_resolved"
	prepareProgressStageLoadIO               prepareProgressStage = "load_io"
	prepareProgressStageLoadParse            prepareProgressStage = "load_parse"
	prepareProgressStageLoadPrimitive        prepareProgressStage = "load_primitive"
	prepareProgressStageLoadCompleted        prepareProgressStage = "load_completed"
	prepareProgressStageModelValidated       prepareProgressStage = "model_validated"
	prepareProgressStageLayoutPrepared       prepareProgressStage = "layout_prepared"
	prepareProgressStageModelPathApplied     prepareProgressStage = "model_path_applied"
	prepareProgressStageReorderUV            prepareProgressStage = "reorder_uv"
	prepareProgressStageReorderTexture       prepareProgressStage = "reorder_texture"
	prepareProgressStageReorderCandidates    prepareProgressStage = "reorder_candidates"
	prepareProgressStageReorderPair          prepareProgressStage = "reorder_pair"
	prepareProgressStageReorderBlock         prepareProgressStage = "reorder_block"
	prepareProgressStageReorderCompleted     prepareProgressStage = "reorder_completed"
	prepareProgressStageBoneMappingCompleted prepareProgressStage = "bone_mapping_completed"
	prepareProgressStageAstanceCompleted     prepareProgressStage = "a_stance_completed"
	prepareProgressStageMorphRenamePlanned   prepareProgressStage = "morph_rename_planned"
	prepareProgressStageMorphRenameApply     prepareProgressStage = "morph_rename_apply"
	prepareProgressStageMorphRenameCompleted prepareProgressStage = "morph_rename_completed"
	prepareProgressStageMaterialViewApplied  prepareProgressStage = "material_view_applied"
	prepareProgressStageViewerApplied        prepareProgressStage = "viewer_applied"
)

// fixedPrepareProgressStages は固定工程ステージ一覧を表す。
var fixedPrepareProgressStages = []prepareProgressStage{
	prepareProgressStageInputValidated,
	prepareProgressStageOutputResolved,
	prepareProgressStageLoadCompleted,
	prepareProgressStageModelValidated,
	prepareProgressStageLayoutPrepared,
	prepareProgressStageModelPathApplied,
	prepareProgressStageReorderCandidates,
	prepareProgressStageReorderCompleted,
	prepareProgressStageBoneMappingCompleted,
	prepareProgressStageAstanceCompleted,
	prepareProgressStageMorphRenamePlanned,
	prepareProgressStageMorphRenameCompleted,
	prepareProgressStageMaterialViewApplied,
	prepareProgressStageViewerApplied,
}

// prepareProgressTracker は準備処理進捗のmax/value管理を行う。
type prepareProgressTracker struct {
	progressBar *controller.ProgressBar
	maxValue    int
	value       int
	targets     map[prepareProgressStage]int
	completed   map[prepareProgressStage]int

	reorderUvPassCount       int
	reorderTextureTotalUnits int
	reorderPairProcessedRaw  int
	reorderBlockProcessedRaw int
	morphRenameProcessedRaw  int
}

// newPrepareProgressTracker は準備処理用の進捗トラッカーを生成する。
func newPrepareProgressTracker(cw *controller.ControlWindow, inputPath string) *prepareProgressTracker {
	tracker := &prepareProgressTracker{
		targets:   map[prepareProgressStage]int{},
		completed: map[prepareProgressStage]int{},
	}
	if cw != nil {
		tracker.progressBar = cw.ProgressBar()
	}
	tracker.initialize(inputPath)
	return tracker
}

// initialize は入力ファイルに基づく初期ステージ構成を設定する。
func (t *prepareProgressTracker) initialize(inputPath string) {
	if t == nil {
		return
	}
	for _, stage := range fixedPrepareProgressStages {
		t.setStageTarget(stage, 1)
	}
	t.setStageTarget(prepareProgressStageLoadIO, chunkUnits(detectInputFileSize(inputPath), loadIoChunkBytes))
	t.setStageTarget(prepareProgressStageLoadParse, 1)
	t.setStageTarget(prepareProgressStageLoadPrimitive, 1)
	t.setStageTarget(prepareProgressStageReorderUV, 1)
	t.setStageTarget(prepareProgressStageReorderTexture, 1)
	t.setStageTarget(prepareProgressStageReorderPair, 1)
	t.setStageTarget(prepareProgressStageReorderBlock, 1)
	t.setStageTarget(prepareProgressStageMorphRenameApply, 1)
	t.applyProgressBar()
}

// reset は進捗バー表示を初期化する。
func (t *prepareProgressTracker) reset() {
	if t == nil {
		return
	}
	if t.progressBar == nil {
		return
	}
	t.progressBar.SetMax(0)
	t.progressBar.SetValue(0)
}

// ReportPrepareProgress はPrepareModel側の進捗イベントを反映する。
func (t *prepareProgressTracker) ReportPrepareProgress(event minteractor.PrepareProgressEvent) {
	if t == nil {
		return
	}
	switch event.Type {
	case minteractor.PrepareProgressEventTypeInputValidated:
		t.advanceStage(prepareProgressStageInputValidated, 1)
	case minteractor.PrepareProgressEventTypeOutputPathResolved:
		t.advanceStage(prepareProgressStageOutputResolved, 1)
	case minteractor.PrepareProgressEventTypeModelValidated:
		t.advanceStage(prepareProgressStageModelValidated, 1)
	case minteractor.PrepareProgressEventTypeLayoutPrepared:
		t.advanceStage(prepareProgressStageLayoutPrepared, 1)
	case minteractor.PrepareProgressEventTypeModelPathApplied:
		t.advanceStage(prepareProgressStageModelPathApplied, 1)
	case minteractor.PrepareProgressEventTypeReorderUvScanned:
		t.reorderUvPassCount++
		uvUnits := maxInt(1, t.reorderUvPassCount)
		t.setStageTarget(prepareProgressStageReorderUV, uvUnits)
		t.advanceStageTo(prepareProgressStageReorderUV, uvUnits)
	case minteractor.PrepareProgressEventTypeReorderTextureScanned:
		t.reorderTextureTotalUnits += chunkUnits(event.TextureCount, reorderTextureChunkSize)
		textureUnits := maxInt(1, t.reorderTextureTotalUnits)
		t.setStageTarget(prepareProgressStageReorderTexture, textureUnits)
		t.advanceStageTo(prepareProgressStageReorderTexture, textureUnits)
	case minteractor.PrepareProgressEventTypeReorderBlocksPlanned:
		t.advanceStage(prepareProgressStageReorderCandidates, 1)
		t.reorderPairProcessedRaw = 0
		t.reorderBlockProcessedRaw = 0
		t.setStageTarget(prepareProgressStageReorderPair, chunkUnits(event.PairCount, reorderPairChunkSize))
		t.setStageTarget(prepareProgressStageReorderBlock, chunkUnits(event.BlockCount, reorderBlockChunkSize))
	case minteractor.PrepareProgressEventTypeReorderBlockProcessed:
		if event.PairCount > 0 {
			t.reorderPairProcessedRaw += event.PairCount
		}
		if event.BlockCount > 0 {
			t.reorderBlockProcessedRaw += event.BlockCount
		}
		t.advanceStageTo(prepareProgressStageReorderPair, chunkDoneUnits(t.reorderPairProcessedRaw, reorderPairChunkSize))
		t.advanceStageTo(prepareProgressStageReorderBlock, chunkDoneUnits(t.reorderBlockProcessedRaw, reorderBlockChunkSize))
	case minteractor.PrepareProgressEventTypeReorderCompleted:
		t.completeReorderStages()
	case minteractor.PrepareProgressEventTypeBoneMappingCompleted:
		t.advanceStage(prepareProgressStageBoneMappingCompleted, 1)
	case minteractor.PrepareProgressEventTypeAstanceCompleted:
		t.advanceStage(prepareProgressStageAstanceCompleted, 1)
	case minteractor.PrepareProgressEventTypeMorphRenamePlanned:
		t.morphRenameProcessedRaw = 0
		t.setStageTarget(prepareProgressStageMorphRenameApply, chunkUnits(event.MorphCount, morphRenameChunkSize))
		t.advanceStage(prepareProgressStageMorphRenamePlanned, 1)
	case minteractor.PrepareProgressEventTypeMorphRenameProcessed:
		if event.MorphCount > 0 {
			t.morphRenameProcessedRaw += event.MorphCount
		}
		t.advanceStageTo(
			prepareProgressStageMorphRenameApply,
			chunkDoneUnits(t.morphRenameProcessedRaw, morphRenameChunkSize),
		)
	case minteractor.PrepareProgressEventTypeMorphRenameCompleted:
		t.completeStage(prepareProgressStageMorphRenameApply)
		t.advanceStage(prepareProgressStageMorphRenameCompleted, 1)
	}
}

// handleLoadProgress はVRM読込側の進捗イベントを反映する。
func (t *prepareProgressTracker) handleLoadProgress(event vrm.LoadProgressEvent) {
	if t == nil {
		return
	}
	switch event.Type {
	case vrm.LoadProgressEventTypeFileReadComplete:
		t.setStageTarget(prepareProgressStageLoadIO, chunkUnits(event.FileSizeBytes, loadIoChunkBytes))
		t.advanceStageTo(prepareProgressStageLoadIO, chunkDoneUnits(event.ReadBytes, loadIoChunkBytes))
	case vrm.LoadProgressEventTypeJsonParsed:
		parseTotal := event.NodeCount + event.AccessorCount
		t.setStageTarget(prepareProgressStageLoadParse, chunkUnits(parseTotal, loadParseChunkSize))
		t.advanceStageTo(prepareProgressStageLoadParse, chunkDoneUnits(parseTotal, loadParseChunkSize))
		t.setStageTarget(prepareProgressStageLoadPrimitive, chunkUnits(event.PrimitiveTotal, loadPrimitiveChunkSize))
	case vrm.LoadProgressEventTypePrimitiveProcessed:
		t.setStageTarget(prepareProgressStageLoadPrimitive, chunkUnits(event.PrimitiveTotal, loadPrimitiveChunkSize))
		t.advanceStageTo(prepareProgressStageLoadPrimitive, chunkDoneUnits(event.PrimitiveDone, loadPrimitiveChunkSize))
	case vrm.LoadProgressEventTypeCompleted:
		t.completeLoadStages()
	}
}

// completeLoadStages は読込関連ステージを完了状態へ揃える。
func (t *prepareProgressTracker) completeLoadStages() {
	if t == nil {
		return
	}
	t.completeStage(prepareProgressStageLoadIO)
	t.completeStage(prepareProgressStageLoadParse)
	t.completeStage(prepareProgressStageLoadPrimitive)
	t.advanceStage(prepareProgressStageLoadCompleted, 1)
}

// completeReorderStages は材質並べ替え関連ステージを完了状態へ揃える。
func (t *prepareProgressTracker) completeReorderStages() {
	if t == nil {
		return
	}
	t.advanceStage(prepareProgressStageReorderCandidates, 1)
	t.completeStage(prepareProgressStageReorderUV)
	t.completeStage(prepareProgressStageReorderTexture)
	t.completeStage(prepareProgressStageReorderPair)
	t.completeStage(prepareProgressStageReorderBlock)
	t.advanceStage(prepareProgressStageReorderCompleted, 1)
}

// advanceStage は指定ステージを相対件数で進める。
func (t *prepareProgressTracker) advanceStage(stage prepareProgressStage, delta int) {
	if t == nil || delta <= 0 {
		return
	}
	current := t.completed[stage]
	t.advanceStageTo(stage, current+delta)
}

// advanceStageTo は指定ステージを絶対件数まで進める。
func (t *prepareProgressTracker) advanceStageTo(stage prepareProgressStage, targetDone int) {
	if t == nil || targetDone <= 0 {
		return
	}
	stageTarget := t.targets[stage]
	if stageTarget <= 0 {
		stageTarget = 1
		t.setStageTarget(stage, stageTarget)
	}
	if targetDone > stageTarget {
		targetDone = stageTarget
	}
	current := t.completed[stage]
	if targetDone <= current {
		return
	}
	delta := targetDone - current
	t.completed[stage] = targetDone
	t.value += delta
	if t.value > t.maxValue {
		t.value = t.maxValue
	}
	t.applyProgressBar()
}

// completeStage は指定ステージを完了まで進める。
func (t *prepareProgressTracker) completeStage(stage prepareProgressStage) {
	if t == nil {
		return
	}
	stageTarget := t.targets[stage]
	if stageTarget <= 0 {
		stageTarget = 1
		t.setStageTarget(stage, stageTarget)
	}
	t.advanceStageTo(stage, stageTarget)
}

// setStageTarget は指定ステージの目標件数を設定する。
func (t *prepareProgressTracker) setStageTarget(stage prepareProgressStage, target int) {
	if t == nil {
		return
	}
	target = maxInt(1, target)
	currentDone := t.completed[stage]
	if target < currentDone {
		target = currentDone
	}
	currentTarget := t.targets[stage]
	if currentTarget == target {
		return
	}
	t.targets[stage] = target
	t.maxValue += target - currentTarget
	if t.maxValue < 0 {
		t.maxValue = 0
	}
	if t.value > t.maxValue {
		t.value = t.maxValue
	}
	t.applyProgressBar()
}

// applyProgressBar は現在のmax/valueを進捗バーへ反映する。
func (t *prepareProgressTracker) applyProgressBar() {
	if t == nil || t.progressBar == nil {
		return
	}
	t.progressBar.SetMax(t.maxValue)
	t.progressBar.SetValue(t.value)
}

// detectInputFileSize は入力ファイルサイズを返す。
func detectInputFileSize(path string) int {
	info, err := os.Stat(path)
	if err != nil || info == nil {
		return 0
	}
	size := info.Size()
	if size < 0 {
		return 0
	}
	return int(size)
}

// chunkUnits は総件数をチャンク単位へ繰り上げ換算する。
func chunkUnits(total int, chunkSize int) int {
	if chunkSize <= 0 {
		return 1
	}
	if total <= 0 {
		return 1
	}
	return ceilDiv(total, chunkSize)
}

// chunkDoneUnits は進行件数をチャンク単位へ繰り上げ換算する。
func chunkDoneUnits(done int, chunkSize int) int {
	if chunkSize <= 0 || done <= 0 {
		return 0
	}
	return ceilDiv(done, chunkSize)
}

// ceilDiv は整数除算の繰り上げ値を返す。
func ceilDiv(value int, divisor int) int {
	if divisor <= 0 {
		return 0
	}
	if value <= 0 {
		return 0
	}
	return (value + divisor - 1) / divisor
}

// maxInt は2値の大きい方を返す。
func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

// NewTabPages は mu_vrm2pmx のタブページ群を生成する。
func NewTabPages(mWidgets *controller.MWidgets, baseServices base.IBaseServices, initialVrmPath string, audioPlayer audio_api.IAudioPlayer, viewerUsecase *minteractor.Vrm2PmxUsecase) []declarative.TabPage {
	var fileTab *walk.TabPage

	var translator i18n.II18n
	var logger logging.ILogger
	var userConfig config.IUserConfig
	if baseServices != nil {
		translator = baseServices.I18n()
		logger = baseServices.Logger()
		if cfg := baseServices.Config(); cfg != nil {
			userConfig = cfg.UserConfig()
		}
	}
	if logger == nil {
		logger = logging.DefaultLogger()
	}
	if viewerUsecase == nil {
		viewerUsecase = minteractor.NewVrm2PmxUsecase(minteractor.Vrm2PmxUsecaseDeps{})
	}

	var currentInputPath string
	var currentOutputPath string
	var loadedModel *model.PmxModel
	var motionLoadPicker *widget.FilePicker
	var materialView *widget.MaterialTableView
	var pmxSavePicker *widget.FilePicker

	player := widget.NewMotionPlayer(translator)
	player.SetAudioPlayer(audioPlayer, userConfig)

	materialView = widget.NewMaterialTableView(
		translator,
		i18n.TranslateOrMark(translator, messages.LabelMaterialViewTip),
		func(cw *controller.ControlWindow, indexes []int) {
			if cw == nil {
				return
			}
			cw.SetSelectedMaterialIndexes(previewWindowIndex, previewModelIndex, indexes)
		},
	)

	vrmLoadPicker := widget.NewLoadFilePicker(
		userConfig,
		translator,
		vrmHistoryKey,
		i18n.TranslateOrMark(translator, messages.LabelVrmPath),
		i18n.TranslateOrMark(translator, messages.LabelVrmPathTip),
		func(cw *controller.ControlWindow, rep io_common.IFileReader, path string) {
			currentInputPath = path
			if strings.TrimSpace(path) == "" {
				loadedModel = nil
				if materialView != nil {
					materialView.ResetRows(nil)
				}
				if cw != nil {
					cw.SetModel(previewWindowIndex, previewModelIndex, nil)
				}
				return
			}
			playing := false
			if cw != nil {
				playing = cw.Playing()
			}
			progressTracker := newPrepareProgressTracker(cw, path)
			defer progressTracker.reset()
			_ = base.RunWithBoolState(
				func(v bool) {
					if cw != nil {
						cw.SetEnabledInPlaying(v)
					}
				},
				true,
				playing,
				func() error {
					if progressAwareReader, ok := rep.(interface {
						SetLoadProgressReporter(func(vrm.LoadProgressEvent))
					}); ok {
						progressAwareReader.SetLoadProgressReporter(progressTracker.handleLoadProgress)
						defer progressAwareReader.SetLoadProgressReporter(nil)
					}

					modelData, err := viewerUsecase.LoadModel(rep, path)
					if err != nil {
						logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageLoadFailed), err)
						loadedModel = nil
						if materialView != nil {
							materialView.ResetRows(nil)
						}
						if cw != nil {
							cw.SetModel(previewWindowIndex, previewModelIndex, nil)
						}
						return nil
					}
					progressTracker.completeLoadStages()
					if modelData == nil {
						logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageLoadFailed), nil)
						loadedModel = nil
						if materialView != nil {
							materialView.ResetRows(nil)
						}
						if cw != nil {
							cw.SetModel(previewWindowIndex, previewModelIndex, nil)
						}
						return nil
					}
					if modelData.VrmData == nil {
						logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageLoadFailed), nil)
						loadedModel = nil
						if materialView != nil {
							materialView.ResetRows(nil)
						}
						if cw != nil {
							cw.SetModel(previewWindowIndex, previewModelIndex, nil)
						}
						return nil
					}

					currentOutputPath = buildOutputPath(path)
					if pmxSavePicker != nil && strings.TrimSpace(currentOutputPath) != "" {
						pmxSavePicker.SetPath(currentOutputPath)
					}

					result, err := viewerUsecase.PrepareModel(minteractor.ConvertRequest{
						InputPath:        path,
						OutputPath:       currentOutputPath,
						ModelData:        modelData,
						ProgressReporter: progressTracker,
					})
					if err != nil {
						logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageConvertFailed), err)
						loadedModel = nil
						if materialView != nil {
							materialView.ResetRows(nil)
						}
						if cw != nil {
							cw.SetModel(previewWindowIndex, previewModelIndex, nil)
						}
						return nil
					}
					progressTracker.completeReorderStages()
					if result == nil || result.Model == nil {
						logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageConvertFailed), nil)
						loadedModel = nil
						if materialView != nil {
							materialView.ResetRows(nil)
						}
						if cw != nil {
							cw.SetModel(previewWindowIndex, previewModelIndex, nil)
						}
						return nil
					}

					loadedModel = result.Model
					if materialView != nil {
						materialView.ResetRows(loadedModel)
					}
					progressTracker.advanceStage(prepareProgressStageMaterialViewApplied, 1)
					currentOutputPath = result.OutputPath
					if pmxSavePicker != nil && strings.TrimSpace(currentOutputPath) != "" {
						pmxSavePicker.SetPath(currentOutputPath)
					}
					if cw != nil {
						cw.SetModel(previewWindowIndex, previewModelIndex, loadedModel)
					}
					progressTracker.advanceStage(prepareProgressStageViewerApplied, 1)
					logger.Info(i18n.TranslateOrMark(translator, messages.LogLoadSuccess), filepath.Base(path))
					return nil
				},
			)
		},
		[]widget.FileFilterExtension{
			{Extension: "*.vrm", Description: "Vrm Files (*.vrm)"},
			{Extension: "*.*", Description: "All Files (*.*)"},
		},
		vrm.NewVrmRepository(),
	)

	motionLoadPicker = widget.NewVmdVpdLoadFilePicker(
		userConfig,
		translator,
		motionHistoryKey,
		i18n.TranslateOrMark(translator, messages.LabelMotionPath),
		i18n.TranslateOrMark(translator, messages.LabelMotionPathTip),
		func(cw *controller.ControlWindow, rep io_common.IFileReader, path string) {
			loadMotion(logger, translator, cw, rep, player, path, previewWindowIndex, previewModelIndex)
		},
	)

	pmxSavePicker = widget.NewPmxSaveFilePicker(
		userConfig,
		translator,
		i18n.TranslateOrMark(translator, messages.LabelPmxPath),
		i18n.TranslateOrMark(translator, messages.LabelPmxPathTip),
		func(cw *controller.ControlWindow, rep io_common.IFileReader, path string) {
			_ = cw
			_ = rep
			currentOutputPath = path
		},
	)

	convertButton := widget.NewMPushButton()
	convertButton.SetLabel(i18n.TranslateOrMark(translator, messages.LabelConvert))
	convertButton.SetTooltip(i18n.TranslateOrMark(translator, messages.LabelConvertTip))
	convertButton.SetOnClicked(func(cw *controller.ControlWindow) {
		if strings.TrimSpace(currentInputPath) == "" {
			logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageSaveFailed), nil)
			logger.Error(i18n.TranslateOrMark(translator, messages.MessageInputRequired))
			return
		}

		if strings.TrimSpace(currentOutputPath) == "" {
			currentOutputPath = buildOutputPath(currentInputPath)
			if pmxSavePicker != nil && strings.TrimSpace(currentOutputPath) != "" {
				pmxSavePicker.SetPath(currentOutputPath)
			}
		}
		if strings.TrimSpace(currentOutputPath) == "" {
			logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageSaveFailed), nil)
			logger.Error(i18n.TranslateOrMark(translator, messages.MessageOutputRequired))
			return
		}
		if loadedModel == nil {
			logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageSaveFailed), nil)
			logger.Error(i18n.TranslateOrMark(translator, messages.MessagePreviewRequired))
			return
		}

		loadedModel.SetPath(currentOutputPath)
		if err := viewerUsecase.SaveModel(nil, currentOutputPath, loadedModel, io_common.SaveOptions{}); err != nil {
			logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageSaveFailed), err)
			return
		}
		if cw != nil {
			cw.SetModel(previewWindowIndex, previewModelIndex, loadedModel)
		}
		controller.Beep()
		logInfoTitle(
			logger,
			i18n.TranslateOrMark(translator, messages.LogConvertSuccess),
			messages.LogConvertSuccessDetail,
			currentOutputPath,
		)
	})

	if mWidgets != nil {
		mWidgets.Widgets = append(mWidgets.Widgets, vrmLoadPicker, motionLoadPicker, materialView, pmxSavePicker, player, convertButton)
		mWidgets.SetOnLoaded(func() {
			if mWidgets == nil || mWidgets.Window() == nil {
				return
			}
			mWidgets.Window().SetOnEnabledInPlaying(func(playing bool) {
				for _, w := range mWidgets.Widgets {
					w.SetEnabledInPlaying(playing)
				}
			})
			if strings.TrimSpace(initialVrmPath) != "" {
				vrmLoadPicker.SetPath(initialVrmPath)
			}
		})
	}

	fileTabPage := declarative.TabPage{
		Title:    i18n.TranslateOrMark(translator, messages.LabelFile),
		AssignTo: &fileTab,
		Layout:   declarative.VBox{},
		Background: declarative.SolidColorBrush{
			Color: controller.ColorTabBackground,
		},
		Children: []declarative.Widget{
			declarative.Composite{
				Layout: declarative.VBox{},
				Children: []declarative.Widget{
					vrmLoadPicker.Widgets(),
					motionLoadPicker.Widgets(),
					declarative.TextLabel{Text: i18n.TranslateOrMark(translator, messages.LabelMaterialView)},
					materialView.Widgets(),
					pmxSavePicker.Widgets(),
					declarative.VSeparator{},
					player.Widgets(),
					declarative.VSeparator{},
					convertButton.Widgets(),
				},
			},
		},
	}

	return []declarative.TabPage{fileTabPage}
}

// NewTabPage は mu_vrm2pmx の単一タブを生成する。
func NewTabPage(mWidgets *controller.MWidgets, baseServices base.IBaseServices, initialVrmPath string, audioPlayer audio_api.IAudioPlayer, viewerUsecase *minteractor.Vrm2PmxUsecase) declarative.TabPage {
	return NewTabPages(mWidgets, baseServices, initialVrmPath, audioPlayer, viewerUsecase)[0]
}

// buildOutputPath は入力VRMパスからPMX出力パスを生成する。
func buildOutputPath(inputPath string) string {
	return minteractor.BuildDefaultOutputPath(inputPath)
}

// loadMotion はモーション読み込み結果をControlWindowへ反映する。
func loadMotion(logger logging.ILogger, translator i18n.II18n, cw *controller.ControlWindow, rep io_common.IFileReader, player *widget.MotionPlayer, path string, windowIndex, modelIndex int) {
	if cw == nil {
		return
	}
	if strings.TrimSpace(path) == "" {
		cw.SetMotion(windowIndex, modelIndex, nil)
		return
	}

	motionResult, err := usecase.LoadMotionWithMeta(rep, path)
	if err != nil {
		logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageLoadFailed), err)
		cw.SetMotion(windowIndex, modelIndex, nil)
		return
	}
	if motionResult == nil || motionResult.Motion == nil {
		cw.SetMotion(windowIndex, modelIndex, nil)
		return
	}
	maxFrame := motion.Frame(0)
	if motionResult != nil {
		maxFrame = motionResult.MaxFrame
	}
	if player != nil {
		player.Reset(maxFrame)
	}
	cw.SetMotion(windowIndex, modelIndex, motionResult.Motion)
}

// logErrorTitle はタイトル付きエラーを出力する。
func logErrorTitle(logger logging.ILogger, title string, err error) {
	if logger == nil {
		return
	}
	if titled, ok := logger.(interface {
		ErrorTitle(title string, err error, msg string, params ...any)
	}); ok {
		titled.ErrorTitle(title, err, "")
		return
	}
	if err == nil {
		logger.Error("%s", title)
		return
	}
	logger.Error("%s: %s", title, err.Error())
}

// logInfoTitle はタイトル付き情報ログを出力する。
func logInfoTitle(logger logging.ILogger, title, message string, params ...any) {
	if logger == nil {
		logger = logging.DefaultLogger()
	}
	if titled, ok := logger.(interface {
		InfoTitle(title, msg string, params ...any)
	}); ok {
		titled.InfoTitle(title, message, params...)
		return
	}
	logger.Info("%s", title)
	logger.Info(message, params...)
}
