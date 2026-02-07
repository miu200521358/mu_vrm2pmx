//go:build windows
// +build windows

// 指示: miu200521358
package main

import (
	"embed"
	"os"
	"runtime"

	"github.com/miu200521358/walk/pkg/declarative"
	"github.com/miu200521358/walk/pkg/walk"

	"github.com/miu200521358/mu_vrm2pmx/pkg/infra/controller/ui"
	"github.com/miu200521358/mu_vrm2pmx/pkg/usecase/minteractor"

	"github.com/miu200521358/mlib_go/pkg/adapter/audio_api"
	"github.com/miu200521358/mlib_go/pkg/adapter/io_model/pmx"
	io_model_vrm "github.com/miu200521358/mlib_go/pkg/adapter/io_model/vrm"
	"github.com/miu200521358/mlib_go/pkg/infra/app"
	"github.com/miu200521358/mlib_go/pkg/infra/controller"
	"github.com/miu200521358/mlib_go/pkg/shared/base"
	"github.com/miu200521358/mlib_go/pkg/shared/base/config"
)

// env はビルド時の -ldflags で埋め込む環境値。
var env string

// init はOSスレッド固定とコンソール登録を行う。
func init() {
	runtime.LockOSThread()

	walk.AppendToWalkInit(func() {
		walk.MustRegisterWindowClass(controller.ConsoleViewClass)
	})
}

//go:embed app/*
var appFiles embed.FS

//go:embed i18n/*
var appI18nFiles embed.FS

// main は mu_vrm2pmx を起動する。
func main() {
	initialVrmPath := app.FindInitialPath(os.Args, ".vrm")

	app.Run(app.RunOptions{
		ViewerCount: 1,
		AppFiles:    appFiles,
		I18nFiles:   appI18nFiles,
		AdjustConfig: func(appConfig *config.AppConfig) {
			config.ApplyBuildEnv(appConfig, env)
		},
		BuildMenuItems: func(baseServices base.IBaseServices) []declarative.MenuItem {
			return ui.NewMenuItems(baseServices.I18n(), baseServices.Logger())
		},
		BuildTabPages: func(widgets *controller.MWidgets, baseServices base.IBaseServices, audioPlayer audio_api.IAudioPlayer) []declarative.TabPage {
			viewerUsecase := minteractor.NewVrm2PmxUsecase(minteractor.Vrm2PmxUsecaseDeps{
				ModelReader: io_model_vrm.NewVrmRepository(),
				ModelWriter: pmx.NewPmxRepository(),
			})
			return ui.NewTabPages(widgets, baseServices, initialVrmPath, audioPlayer, viewerUsecase)
		},
	})
}
