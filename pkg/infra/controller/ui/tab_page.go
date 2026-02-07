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
	"github.com/miu200521358/walk/pkg/declarative"
	"github.com/miu200521358/walk/pkg/walk"

	"github.com/miu200521358/mu_vrm2pmx/pkg/adapter/mpresenter/messages"
	"github.com/miu200521358/mu_vrm2pmx/pkg/usecase/minteractor"
)

const (
	vrmHistoryKey = "vrm"
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
	var pmxSavePicker *widget.FilePicker

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
				if cw != nil {
					cw.SetModel(0, 0, nil)
				}
				return
			}

			modelData, err := viewerUsecase.LoadModel(rep, path)
			if err != nil {
				logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageLoadFailed), err)
				loadedModel = nil
				if cw != nil {
					cw.SetModel(0, 0, nil)
				}
				return
			}
			if modelData == nil {
				logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageLoadFailed), nil)
				loadedModel = nil
				if cw != nil {
					cw.SetModel(0, 0, nil)
				}
				return
			}
			if modelData.VrmData == nil {
				logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageLoadFailed), nil)
				loadedModel = nil
				if cw != nil {
					cw.SetModel(0, 0, nil)
				}
				return
			}

			loadedModel = modelData
			if cw != nil {
				cw.SetModel(0, 0, modelData)
			}
			logger.Info(i18n.TranslateOrMark(translator, messages.LogLoadSuccess), filepath.Base(path))

			if strings.TrimSpace(currentOutputPath) == "" {
				currentOutputPath = buildOutputPath(path)
				if pmxSavePicker != nil && strings.TrimSpace(currentOutputPath) != "" {
					pmxSavePicker.SetPath(currentOutputPath)
				}
			}
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
			logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageConvertFailed), nil)
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
			logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageConvertFailed), nil)
			logger.Error(i18n.TranslateOrMark(translator, messages.MessageOutputRequired))
			return
		}

		result, err := viewerUsecase.Convert(minteractor.ConvertRequest{
			InputPath:   currentInputPath,
			OutputPath:  currentOutputPath,
			ModelData:   loadedModel,
			SaveOptions: io_common.SaveOptions{},
		})
		if err != nil {
			logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageConvertFailed), err)
			return
		}
		if result == nil || result.Model == nil {
			logErrorTitle(logger, i18n.TranslateOrMark(translator, messages.MessageConvertFailed), nil)
			return
		}

		loadedModel = result.Model
		if cw != nil {
			cw.SetModel(0, 0, loadedModel)
		}
		controller.Beep()
		logger.Info(i18n.TranslateOrMark(translator, messages.LogConvertSuccess), filepath.Base(result.OutputPath))
	})

	if mWidgets != nil {
		mWidgets.Widgets = append(mWidgets.Widgets, vrmLoadPicker, pmxSavePicker, convertButton)
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
	dir := filepath.Dir(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	if strings.TrimSpace(base) == "" {
		return ""
	}
	return filepath.Join(dir, base+".pmx")
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
