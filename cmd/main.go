//go:build !windows
// +build !windows

// 指示: miu200521358
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/miu200521358/mlib_go/pkg/adapter/io_common"
	"github.com/miu200521358/mlib_go/pkg/adapter/io_model"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
)

// options はCLI引数を保持する。
type options struct {
	inputPath  string
	outputPath string
}

// main はVRMからPMXへの変換を実行する。
func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run はCLI処理全体を実行する。
func run(args []string, out io.Writer, errOut io.Writer) error {
	opts, err := parseOptions(args, errOut)
	if err != nil {
		return err
	}

	repository := io_model.NewModelRepository()
	if !repository.CanLoad(opts.inputPath) {
		return fmt.Errorf("入力形式が未対応です: %s", opts.inputPath)
	}

	fmt.Fprintf(out, "[mu_vrm2pmx] 読み込み開始: %s\n", opts.inputPath)
	hashableModel, err := repository.Load(opts.inputPath)
	if err != nil {
		return fmt.Errorf("VRM読み込みに失敗しました: %w", err)
	}
	pmxModel, ok := hashableModel.(*model.PmxModel)
	if !ok {
		return fmt.Errorf("読み込み結果の型が不正です: %T", hashableModel)
	}

	if pmxModel.VrmData == nil {
		return fmt.Errorf("VRMデータがモデルへ設定されていません")
	}

	outputPath, err := resolveOutputPath(opts.inputPath, opts.outputPath)
	if err != nil {
		return err
	}
	if err := ensureOutputDir(outputPath); err != nil {
		return err
	}

	fmt.Fprintf(out, "[mu_vrm2pmx] 保存開始: %s\n", outputPath)
	if err := repository.Save(outputPath, pmxModel, io_common.SaveOptions{}); err != nil {
		return fmt.Errorf("PMX保存に失敗しました: %w", err)
	}
	fmt.Fprintf(out, "[mu_vrm2pmx] 変換完了: %s\n", outputPath)
	return nil
}

// parseOptions はCLI引数を解析する。
func parseOptions(args []string, errOut io.Writer) (options, error) {
	fs := flag.NewFlagSet("mu_vrm2pmx", flag.ContinueOnError)
	fs.SetOutput(errOut)

	in := fs.String("in", "", "入力VRMファイルパス")
	out := fs.String("out", "", "出力PMXファイルパス")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}

	if *in == "" && fs.NArg() > 0 {
		*in = fs.Arg(0)
	}
	if *out == "" && fs.NArg() > 1 {
		*out = fs.Arg(1)
	}
	if *in == "" {
		return options{}, fmt.Errorf("入力VRMファイルを指定してください (-in)")
	}

	if !strings.EqualFold(filepath.Ext(*in), ".vrm") {
		return options{}, fmt.Errorf("入力拡張子が .vrm ではありません: %s", *in)
	}

	return options{inputPath: *in, outputPath: *out}, nil
}

// resolveOutputPath は出力PMXパスを解決する。
func resolveOutputPath(inputPath string, outputPath string) (string, error) {
	if strings.TrimSpace(outputPath) == "" {
		dir := filepath.Dir(inputPath)
		base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		return filepath.Join(dir, base+".pmx"), nil
	}
	if !strings.EqualFold(filepath.Ext(outputPath), ".pmx") {
		return "", fmt.Errorf("出力拡張子が .pmx ではありません: %s", outputPath)
	}
	return outputPath, nil
}

// ensureOutputDir は出力先ディレクトリを作成する。
func ensureOutputDir(outputPath string) error {
	dir := filepath.Dir(outputPath)
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("出力先ディレクトリの作成に失敗しました: %w", err)
	}
	return nil
}
