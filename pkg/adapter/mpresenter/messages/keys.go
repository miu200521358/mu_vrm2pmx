// 指示: miu200521358
// Package messages はUI表示に使うメッセージキーを提供する。
package messages

// メッセージキー一覧。
const (
	HelpUsageTitle = "使い方"
	HelpUsage      = "使い方説明"

	LabelFile            = "ファイル"
	LabelVrmPath         = "VRM入力"
	LabelVrmPathTip      = "VRM入力説明"
	LabelMotionPath      = "モーション入力"
	LabelMotionPathTip   = "モーション入力説明"
	LabelMaterialView    = "材質ビュー"
	LabelMaterialViewTip = "材質ビュー説明"
	LabelPmxPath         = "PMX出力"
	LabelPmxPathTip      = "PMX出力説明"
	LabelConvert         = "変換開始"
	LabelConvertTip      = "変換開始説明"

	MessageLoadFailed      = "読み込み失敗"
	MessageSaveFailed      = "保存失敗"
	MessageConvertFailed   = "変換失敗"
	MessagePreviewRequired = "VRMを読み込んでプレビューを表示してください"
	MessageInputRequired   = "VRMファイルを指定してください"
	MessageOutputRequired  = "PMX出力パスを指定してください"
	MessageVrmDataMissing  = "VRMデータが見つかりません"

	LogLoadSuccess                           = "VRM読み込み成功: %s"
	LogConvertSuccess                        = "PMX保存成功: %s"
	LogMaterialReorderInfoStart              = "材質並べ替え開始(Info): materials=%d faces=%d"
	LogMaterialReorderInfoUVFetchStart       = "材質並べ替え: UV画像取得開始 materials=%d threshold=%.3f"
	LogMaterialReorderInfoTextureJudgeStart  = "材質並べ替え: テクスチャ判定開始 materials=%d threshold=%.3f"
	LogMaterialReorderInfoUVTransparencyDone = "材質並べ替え: UV透明率取得完了 materials=%d transparentCandidates=%d threshold=%.3f"
	LogMaterialReorderInfoTextureJudgeDone   = "材質並べ替え: テクスチャ判定完了 textures=%d succeeded=%d failed=%d threshold=%.3f"
	LogMaterialReorderInfoPairResolved       = "材質並べ替え: ペア判定解決 block=[%s] pairs=%d constraints=%d"
	LogMaterialReorderInfoConstraintResolved = "材質並べ替え: 制約解決完了 block=[%s] changed=%t"
	LogMaterialReorderInfoCompleted          = "材質並べ替え完了: changed=%t transparent=%d blocks=%d"
)
