//go:build windows
// +build windows

// 指示: miu200521358
package ui

import (
	"path/filepath"
	"strings"

	"github.com/miu200521358/mlib_go/pkg/adapter/audio_api"
	"github.com/miu200521358/mlib_go/pkg/adapter/io_common"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/infra/controller"
	"github.com/miu200521358/mlib_go/pkg/infra/controller/widget"
	"github.com/miu200521358/mlib_go/pkg/shared/base"
	"github.com/miu200521358/mlib_go/pkg/shared/base/i18n"
	"github.com/miu200521358/mlib_go/pkg/shared/base/logging"
	"github.com/miu200521358/mlib_go/pkg/usecase"
	"github.com/miu200521358/walk/pkg/declarative"
	"github.com/miu200521358/walk/pkg/walk"

	"github.com/miu200521358/mu_vrm2pmx/pkg/adapter/mpresenter/messages"
	"github.com/miu200521358/mu_vrm2pmx/pkg/usecase/minteractor"
)

const (
	vrmHistoryKey      = "vrm"
	motionHistoryKey   = "vmd"
	previewWindowIndex = 0
	previewModelIndex  = 0
)

// NewTabPages は mu_vrm2pmx のタブページ群を生成する。
func NewTabPages(mWidgets *controller.MWidgets, baseServices base.IBaseServices, initialVrmPath string, _ audio_api.IAudioPlayer, viewerUsecase *minteractor.Vrm2PmxUsecase) []declarative.TabPage {
	var fileTab *walk.TabPage

	var translator i18n.II18n
	var logger logging.ILogger
	var userConfig interface {
		GetStringSlice(key string) ([]string, error)
		SetStringSlice(key string, values []string, limit int) error
	}
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

	vrmLoadPicker := widget.NewVrmLoadFilePicker(
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
			_ = base.RunWithBoolState(
				func(v bool) {
					if cw != nil {
						cw.SetEnabledInPlaying(v)
					}
				},
				true,
				playing,
				func() error {
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
						InputPath:  path,
						OutputPath: currentOutputPath,
						ModelData:  modelData,
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
					currentOutputPath = result.OutputPath
					if pmxSavePicker != nil && strings.TrimSpace(currentOutputPath) != "" {
						pmxSavePicker.SetPath(currentOutputPath)
					}
					if cw != nil {
						cw.SetModel(previewWindowIndex, previewModelIndex, loadedModel)
					}
					logger.Info(i18n.TranslateOrMark(translator, messages.LogLoadSuccess), filepath.Base(path))
					return nil
				},
			)
		},
	)

	motionLoadPicker = widget.NewVmdVpdLoadFilePicker(
		userConfig,
		translator,
		motionHistoryKey,
		i18n.TranslateOrMark(translator, messages.LabelMotionPath),
		i18n.TranslateOrMark(translator, messages.LabelMotionPathTip),
		func(cw *controller.ControlWindow, rep io_common.IFileReader, path string) {
			loadMotion(logger, translator, cw, rep, path, previewWindowIndex, previewModelIndex)
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
		logger.Info(i18n.TranslateOrMark(translator, messages.LogConvertSuccess), filepath.Base(currentOutputPath))
	})

	if mWidgets != nil {
		mWidgets.Widgets = append(mWidgets.Widgets, vrmLoadPicker, motionLoadPicker, materialView, pmxSavePicker, convertButton)
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
func loadMotion(logger logging.ILogger, translator i18n.II18n, cw *controller.ControlWindow, rep io_common.IFileReader, path string, windowIndex, modelIndex int) {
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
