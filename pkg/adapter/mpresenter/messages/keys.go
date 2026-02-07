// 指示: miu200521358
// Package messages はUI表示に使うメッセージキーを提供する。
package messages

// メッセージキー一覧。
const (
	HelpUsageTitle = "使い方"
	HelpUsage      = "使い方説明"

	LabelFile       = "ファイル"
	LabelVrmPath    = "VRM入力"
	LabelVrmPathTip = "VRM入力説明"
	LabelPmxPath    = "PMX出力"
	LabelPmxPathTip = "PMX出力説明"
	LabelConvert    = "変換開始"
	LabelConvertTip = "変換開始説明"

	MessageLoadFailed     = "読み込み失敗"
	MessageSaveFailed     = "保存失敗"
	MessageConvertFailed  = "変換失敗"
	MessageInputRequired  = "VRMファイルを指定してください"
	MessageOutputRequired = "PMX出力パスを指定してください"
	MessageVrmDataMissing = "VRMデータが見つかりません"

	LogLoadSuccess    = "VRM読み込み成功: %s"
	LogConvertSuccess = "PMX保存成功: %s"
)
