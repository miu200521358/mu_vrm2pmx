// 指示: miu200521358
package minteractor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/collection"
	"github.com/miu200521358/mlib_go/pkg/shared/base/logging"
)

const (
	morphRenameProgressChunkSize = 25
	morphRenameLogChunk          = 25
	morphRenameTempPrefix        = "__mu_vrm2pmx_morph_tmp_"

	morphRenameInfoStartFormat    = "モーフ名称変換開始(Info): targets=%d mappings=%d"
	morphRenameInfoProgressFormat = "モーフ名称変換: processed=%d/%d renamed=%d unchanged=%d"
	morphRenameInfoDoneFormat     = "モーフ名称変換完了: processed=%d renamed=%d unchanged=%d notFound=%d"
)

// morphRenameRule はrename-only移植用のモーフ対応を表す。
type morphRenameRule struct {
	Name  string
	Panel model.MorphPanel
}

// morphRenameOperation は1モーフ分のrename-only適用情報を表す。
type morphRenameOperation struct {
	Index       int
	SourceName  string
	TargetName  string
	TargetPanel model.MorphPanel
}

// morphRenameSummary はrename-only適用結果の集計を表す。
type morphRenameSummary struct {
	Targets   int
	Mappings  int
	Processed int
	Renamed   int
	Unchanged int
	NotFound  int
}

// morphRenameApplyResult は1モーフ適用結果の詳細を表す。
type morphRenameApplyResult struct {
	NameRenamed        bool
	PanelChanged       bool
	EnglishNameChanged bool
	RenameError        error
	Status             string
}

// morphRenameMappingRow は MMDモーフ名中心の置換定義1件を表す。
type morphRenameMappingRow struct {
	Name    string
	Panel   model.MorphPanel
	Sources []string
}

// morphRenameMappings は SKILLS準拠のMMDモーフ名中心置換定義を表す。
var morphRenameMappings = []morphRenameMappingRow{
	{Name: "▲ボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"▲ボーン"}},
	{Name: "▲頂点", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"▲頂点"}},
	{Name: "あボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"あボーン"}},
	{Name: "あ頂点", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"あ頂点", "aa", "a"}},
	{Name: "いボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"いボーン"}},
	{Name: "い頂点", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"い頂点", "ih", "i"}},
	{Name: "うボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"うボーン"}},
	{Name: "う頂点", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"う頂点", "ou", "u"}},
	{Name: "えボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"えボーン"}},
	{Name: "え頂点", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"え頂点", "ee", "e"}},
	{Name: "おボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"おボーン"}},
	{Name: "お頂点", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"お頂点", "oh", "o"}},
	{Name: "なごみ材質", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"なごみ材質"}},
	{Name: "はぁと材質", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"はぁと材質"}},
	{Name: "はぅ材質", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"はぅ材質"}},
	{Name: "はちゅ目材質", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"はちゅ目材質"}},
	{Name: "べーボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"べーボーン"}},
	{Name: "ぺろりボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"ぺろりボーン"}},
	{Name: "わーボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"わーボーン"}},
	{Name: "わー頂点", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"わー頂点"}},
	{Name: "ウィンクボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"ウィンクボーン"}},
	{Name: "ウィンク右ボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"ウィンク右ボーン"}},
	{Name: "ウィンク２ボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"ウィンク２ボーン"}},
	{Name: "ワボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"ワボーン"}},
	{Name: "ワ頂点", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"ワ頂点"}},
	{Name: "星目材質", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"星目材質"}},
	{Name: "目隠し頂点", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"目隠し頂点"}},
	{Name: "ｳｨﾝｸ２右ボーン", Panel: model.MORPH_PANEL_SYSTEM, Sources: []string{"ｳｨﾝｸ２右ボーン"}},
	{Name: "にこり", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"にこり"}},
	{Name: "にこり2", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"にこり2"}},
	{Name: "にこり2右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"にこり2右"}},
	{Name: "にこり2左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"にこり2左"}},
	{Name: "にこり右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"にこり右"}},
	{Name: "にこり左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"にこり左"}},
	{Name: "はんっ", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"はんっ"}},
	{Name: "はんっ右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"はんっ右"}},
	{Name: "はんっ左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"はんっ左"}},
	{Name: "ひそめ", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"ひそめ"}},
	{Name: "ひそめる2", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"ひそめる2"}},
	{Name: "ひそめる2右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"ひそめる2右"}},
	{Name: "ひそめる2左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"ひそめる2左"}},
	{Name: "ひそめ右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"ひそめ右"}},
	{Name: "ひそめ左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"ひそめ左"}},
	{Name: "上", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"上"}},
	{Name: "上右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"上右"}},
	{Name: "上左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"上左"}},
	{Name: "下", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"下"}},
	{Name: "下右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"下右"}},
	{Name: "下左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"下左"}},
	{Name: "右眉右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"右眉右"}},
	{Name: "右眉左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"右眉左"}},
	{Name: "右眉手前", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"右眉手前"}},
	{Name: "困る", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"困る"}},
	{Name: "困る右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"困る右"}},
	{Name: "困る左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"困る左"}},
	{Name: "左眉右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"左眉右"}},
	{Name: "左眉左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"左眉左"}},
	{Name: "左眉手前", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"左眉手前"}},
	{Name: "怒り", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"怒り"}},
	{Name: "怒り右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"怒り右"}},
	{Name: "怒り左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"怒り左"}},
	{Name: "眉右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"眉右"}},
	{Name: "眉左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"眉左"}},
	{Name: "眉手前", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"眉手前"}},
	{Name: "真面目", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"真面目"}},
	{Name: "真面目2", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"真面目2"}},
	{Name: "真面目2右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"真面目2右"}},
	{Name: "真面目2左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"真面目2左"}},
	{Name: "真面目右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"真面目右"}},
	{Name: "真面目左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"真面目左"}},
	{Name: "驚き", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"驚き"}},
	{Name: "驚き右", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"驚き右"}},
	{Name: "驚き左", Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT, Sources: []string{"驚き左"}},
	{Name: "じと目", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"じと目"}},
	{Name: "じと目右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"じと目右"}},
	{Name: "じと目左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"じと目左"}},
	{Name: "なごみ", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"なごみ"}},
	{Name: "なぬ！", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"なぬ！"}},
	{Name: "なぬ！右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"なぬ！右"}},
	{Name: "なぬ！左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"なぬ！左"}},
	{Name: "にんまり", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"にんまり"}},
	{Name: "にんまり右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"にんまり右"}},
	{Name: "にんまり左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"にんまり左"}},
	{Name: "はぁと", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"はぁと"}},
	{Name: "はぅ", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"はぅ"}},
	{Name: "はちゅ目", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"はちゅ目"}},
	{Name: "びっくり", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"びっくり"}},
	{Name: "びっくり2", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"びっくり2"}},
	{Name: "びっくり2右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"びっくり2右"}},
	{Name: "びっくり2左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"びっくり2左"}},
	{Name: "びっくり右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"びっくり右"}},
	{Name: "びっくり左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"びっくり左"}},
	{Name: "まばたき", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"まばたき", "blink"}},
	{Name: "まばたき連動", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"まばたき連動"}},
	{Name: "ウィンク", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ウィンク"}},
	{Name: "ウィンク右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ウィンク右"}},
	{Name: "ウィンク右連動", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ウィンク右連動"}},
	{Name: "ウィンク連動", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ウィンク連動"}},
	{Name: "ウィンク２", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ウィンク２", "blinkLeft", "blink_l"}},
	{Name: "ウィンク２連動", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ウィンク２連動"}},
	{Name: "ナチュラル", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ナチュラル"}},
	{Name: "ハイライトなし", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ハイライトなし"}},
	{Name: "ハイライトなし右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ハイライトなし右"}},
	{Name: "ハイライトなし左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ハイライトなし左"}},
	{Name: "上瞼↑", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"上瞼↑"}},
	{Name: "上瞼↑右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"上瞼↑右"}},
	{Name: "上瞼↑左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"上瞼↑左"}},
	{Name: "下瞼上げ", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"下瞼上げ"}},
	{Name: "下瞼上げ2", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"下瞼上げ2"}},
	{Name: "下瞼上げ2右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"下瞼上げ2右"}},
	{Name: "下瞼上げ2左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"下瞼上げ2左"}},
	{Name: "下瞼上げ右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"下瞼上げ右"}},
	{Name: "下瞼上げ左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"下瞼上げ左"}},
	{Name: "星目", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"星目"}},
	{Name: "白目", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"白目"}},
	{Name: "白目右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"白目右"}},
	{Name: "白目左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"白目左"}},
	{Name: "目を細める", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目を細める"}},
	{Name: "目を細める右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目を細める右"}},
	{Name: "目を細める左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目を細める左"}},
	{Name: "目上", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目上"}},
	{Name: "目上右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目上右"}},
	{Name: "目上左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目上左"}},
	{Name: "目下", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目下"}},
	{Name: "目下右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目下右"}},
	{Name: "目下左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目下左"}},
	{Name: "目尻広", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目尻広"}},
	{Name: "目尻広右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目尻広右"}},
	{Name: "目尻広左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目尻広左"}},
	{Name: "目頭広", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目頭広"}},
	{Name: "目頭広右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目頭広右"}},
	{Name: "目頭広左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"目頭広左"}},
	{Name: "瞳大", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"瞳大"}},
	{Name: "瞳大右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"瞳大右"}},
	{Name: "瞳大左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"瞳大左"}},
	{Name: "瞳小", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"瞳小"}},
	{Name: "瞳小2", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"瞳小2"}},
	{Name: "瞳小2右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"瞳小2右"}},
	{Name: "瞳小2左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"瞳小2左"}},
	{Name: "瞳小右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"瞳小右"}},
	{Name: "瞳小左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"瞳小左"}},
	{Name: "笑い", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"笑い"}},
	{Name: "笑い連動", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"笑い連動"}},
	{Name: "ｳｨﾝｸ２右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ｳｨﾝｸ２右", "blinkRight", "blink_r"}},
	{Name: "ｳｨﾝｸ２右連動", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ｳｨﾝｸ２右連動"}},
	{Name: "ｷﾘｯ", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ｷﾘｯ"}},
	{Name: "ｷﾘｯ2", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ｷﾘｯ2"}},
	{Name: "ｷﾘｯ2右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ｷﾘｯ2右"}},
	{Name: "ｷﾘｯ2左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ｷﾘｯ2左"}},
	{Name: "ｷﾘｯ右", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ｷﾘｯ右"}},
	{Name: "ｷﾘｯ左", Panel: model.MORPH_PANEL_EYE_UPPER_LEFT, Sources: []string{"ｷﾘｯ左"}},
	{Name: "Λ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"Λ"}},
	{Name: "Λ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"Λ右"}},
	{Name: "Λ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"Λ左"}},
	{Name: "ω口", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"_mouthPress+CatMouth"}},
	{Name: "ω口2", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"_mouthPress+CatMouth-ex"}},
	{Name: "ω口3", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"_mouthPress+DuckMouth"}},
	{Name: "▲", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"▲"}},
	{Name: "あ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"あ"}},
	{Name: "あああ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"あああ"}},
	{Name: "い", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"い"}},
	{Name: "う", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"う"}},
	{Name: "うう", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"うう", "mouthPucker"}},
	{Name: "うほっ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"_mouthFunnel+SharpenLips"}},
	{Name: "うー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"うー"}},
	{Name: "え", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"え"}},
	{Name: "お", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"お"}},
	{Name: "ぎりっ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ぎりっ"}},
	{Name: "ぎりっ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ぎりっ右"}},
	{Name: "ぎりっ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ぎりっ左"}},
	{Name: "ちっ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ちっ"}},
	{Name: "ちっ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ちっ右"}},
	{Name: "ちっ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ちっ左"}},
	{Name: "にこ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にこ"}},
	{Name: "にこ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にこ右"}},
	{Name: "にこ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にこ左"}},
	{Name: "にっこり", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にっこり"}},
	{Name: "にっこり右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にっこり右"}},
	{Name: "にっこり左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にっこり左"}},
	{Name: "にひ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にひ"}},
	{Name: "にひひ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にひひ"}},
	{Name: "にひひ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にひひ右"}},
	{Name: "にひひ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にひひ左"}},
	{Name: "にひ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にひ右"}},
	{Name: "にひ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にひ左"}},
	{Name: "にやり2", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にやり2"}},
	{Name: "にやり2右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にやり2右"}},
	{Name: "にやり2左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"にやり2左"}},
	{Name: "ぷくー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ぷくー"}},
	{Name: "ぷくー右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ぷくー右"}},
	{Name: "ぷくー左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ぷくー左"}},
	{Name: "べー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"べー"}},
	{Name: "ぺろり", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ぺろり"}},
	{Name: "むっ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"むっ"}},
	{Name: "むっ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"むっ右"}},
	{Name: "むっ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"むっ左"}},
	{Name: "むむ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"むむ"}},
	{Name: "わー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"わー"}},
	{Name: "ん", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ん"}},
	{Name: "んむー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"mouthFunnel", "mouthRoll"}},
	{Name: "ギザ歯", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ギザ歯"}},
	{Name: "ギザ歯上", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ギザ歯上"}},
	{Name: "ギザ歯下", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ギザ歯下"}},
	{Name: "ワ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"ワ"}},
	{Name: "一文字", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"一文字"}},
	{Name: "上唇むむ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"上唇むむ"}},
	{Name: "上唇んむー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"上唇んむー"}},
	{Name: "下唇むむ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"下唇むむ"}},
	{Name: "下唇んむー", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"下唇んむー"}},
	{Name: "口上", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"口上"}},
	{Name: "口下", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"口下"}},
	{Name: "口右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"口右"}},
	{Name: "口左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"口左"}},
	{Name: "口幅広", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"口幅広"}},
	{Name: "口幅広右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"口幅広右"}},
	{Name: "口幅広左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"口幅広左"}},
	{Name: "口横広げ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"口横広げ"}},
	{Name: "口角下げ", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"口角下げ"}},
	{Name: "口角下げ右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"口角下げ右"}},
	{Name: "口角下げ左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"口角下げ左"}},
	{Name: "歯短", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"歯短"}},
	{Name: "歯短上", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"歯短上"}},
	{Name: "歯短下", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"歯短下"}},
	{Name: "歯隠", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"歯隠"}},
	{Name: "牙", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"牙"}},
	{Name: "牙上", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"牙上"}},
	{Name: "牙上右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"牙上右"}},
	{Name: "牙上左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"牙上左"}},
	{Name: "牙下", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"牙下"}},
	{Name: "牙下右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"牙下右"}},
	{Name: "牙下左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"牙下左"}},
	{Name: "真ん中牙", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"真ん中牙"}},
	{Name: "真ん中牙上", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"真ん中牙上"}},
	{Name: "真ん中牙下", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"真ん中牙下"}},
	{Name: "肌牙", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"肌牙"}},
	{Name: "肌牙右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"肌牙右"}},
	{Name: "肌牙左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"肌牙左"}},
	{Name: "薄笑い", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"薄笑い"}},
	{Name: "薄笑い右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"薄笑い右"}},
	{Name: "薄笑い左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"薄笑い左"}},
	{Name: "顎前", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"顎前"}},
	{Name: "顎右", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"顎右"}},
	{Name: "顎左", Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT, Sources: []string{"顎左"}},
	{Name: "エッジOFF", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT, Sources: []string{"Edge_Off"}},
	{Name: "ニュートラル", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT, Sources: []string{"ニュートラル", "neutral"}},
	{Name: "哀", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT, Sources: []string{"哀", "sad", "sorrow"}},
	{Name: "喜", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT, Sources: []string{"喜", "happy", "joy"}},
	{Name: "怒", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT, Sources: []string{"怒", "angry"}},
	{Name: "楽", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT, Sources: []string{"楽", "relaxed", "fun"}},
	{Name: "照れ", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT, Sources: []string{"Cheek_Dye"}},
	{Name: "驚", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT, Sources: []string{"驚", "surprised", "surpriosed"}},
}

// morphRenameSourceRules は入力モーフ名からMMDモーフ名への変換参照表を表す。
var morphRenameSourceRules = buildMorphRenameSourceRules(morphRenameMappings)

// buildMorphRenameSourceRules はMMDモーフ名中心定義から入力名lookupを構築する。
func buildMorphRenameSourceRules(mappings []morphRenameMappingRow) map[string]morphRenameRule {
	rules := map[string]morphRenameRule{}
	for _, mapping := range mappings {
		if strings.TrimSpace(mapping.Name) == "" {
			continue
		}
		for _, sourceName := range mapping.Sources {
			normalizedSourceName := strings.TrimSpace(sourceName)
			if normalizedSourceName == "" {
				continue
			}
			rules[normalizedSourceName] = morphRenameRule{Name: mapping.Name, Panel: mapping.Panel}
		}
	}
	return rules
}

// applyMorphRenameOnlyBeforeViewer はrename-only対応表に基づきモーフ名・パネルを補正する。
func applyMorphRenameOnlyBeforeViewer(modelData *ModelData, progressReporter IPrepareProgressReporter) morphRenameSummary {
	summary := morphRenameSummary{Mappings: len(morphRenameSourceRules)}
	summary.Targets = resolveMorphRenameTargetCount(modelData)

	reportPrepareProgress(progressReporter, PrepareProgressEvent{
		Type:       PrepareProgressEventTypeMorphRenamePlanned,
		MorphCount: summary.Targets,
	})
	logMorphRenameInfo(morphRenameInfoStartFormat, summary.Targets, summary.Mappings)
	logMorphRenameModelNames(modelData)

	if modelData == nil || modelData.Morphs == nil || summary.Targets == 0 {
		summary.NotFound = summary.Mappings
		reportPrepareProgress(progressReporter, PrepareProgressEvent{Type: PrepareProgressEventTypeMorphRenameCompleted})
		logMorphRenameInfo(
			morphRenameInfoDoneFormat,
			summary.Processed,
			summary.Renamed,
			summary.Unchanged,
			summary.NotFound,
		)
		return summary
	}

	operations, notFound := collectMorphRenameOperations(modelData.Morphs)
	summary.NotFound = notFound
	renamePlanByIndex := buildMorphRenamePlannedFlags(modelData.Morphs, operations)
	tempRenamed := applyMorphTemporaryRenames(modelData.Morphs, operations, renamePlanByIndex)

	processedPending := 0
	for index := 0; index < modelData.Morphs.Len(); index++ {
		summary.Processed++
		processedPending++

		morphData, err := modelData.Morphs.Get(index)
		if err != nil || morphData == nil {
			summary.Unchanged++
			logMorphRenameDebug(
				"モーフ名称変換詳細: index=%d mapped=%t status=%s",
				index,
				false,
				"morph_nil_or_unreadable",
			)
			processedPending = flushMorphRenameProgress(progressReporter, summary, processedPending, false)
			continue
		}

		sourceName := strings.TrimSpace(morphData.Name())
		beforePanel := morphData.Panel
		targetName := ""
		targetPanel := beforePanel
		renamePlanned := false
		tempName := ""
		applyResult := morphRenameApplyResult{Status: "no_mapping"}
		changed := false
		if operation, exists := operations[index]; exists {
			sourceName = operation.SourceName
			targetName = operation.TargetName
			targetPanel = operation.TargetPanel
			renamePlanned = renamePlanByIndex[index]
			tempName = tempRenamed[index]
			applyResult = applyMorphRenameOperation(
				modelData.Morphs,
				morphData,
				operation,
				renamePlanned,
				tempName,
			)
			changed = applyResult.NameRenamed || applyResult.PanelChanged || applyResult.EnglishNameChanged
		} else {
			logMorphRenameDebug(
				"モーフ名称変換詳細: index=%d source=%s mapped=%t status=%s",
				index,
				sourceName,
				false,
				"no_mapping",
			)
		}
		if changed {
			summary.Renamed++
		} else {
			summary.Unchanged++
		}
		if targetName != "" {
			logMorphRenameDebug(
				"モーフ名称変換詳細: index=%d source=%s target=%s mapped=%t renamePlanned=%t tempAssigned=%t nameRenamed=%t panelChanged=%t englishChanged=%t beforePanel=%d afterPanel=%d status=%s err=%v",
				index,
				sourceName,
				targetName,
				true,
				renamePlanned,
				tempName != "",
				applyResult.NameRenamed,
				applyResult.PanelChanged,
				applyResult.EnglishNameChanged,
				beforePanel,
				targetPanel,
				applyResult.Status,
				applyResult.RenameError,
			)
		}
		processedPending = flushMorphRenameProgress(progressReporter, summary, processedPending, false)
	}
	flushMorphRenameProgress(progressReporter, summary, processedPending, true)

	reportPrepareProgress(progressReporter, PrepareProgressEvent{Type: PrepareProgressEventTypeMorphRenameCompleted})
	logMorphRenameInfo(
		morphRenameInfoDoneFormat,
		summary.Processed,
		summary.Renamed,
		summary.Unchanged,
		summary.NotFound,
	)
	return summary
}

// logMorphRenameModelNames はモデルに存在するモーフ名一覧をDEBUGログ出力する。
func logMorphRenameModelNames(modelData *ModelData) {
	if modelData == nil || modelData.Morphs == nil {
		logMorphRenameDebug("モーフ名称一覧: count=0 status=model_or_morphs_nil")
		return
	}

	count := modelData.Morphs.Len()
	logMorphRenameDebug("モーフ名称一覧開始: count=%d", count)
	for index := 0; index < count; index++ {
		morphData, err := modelData.Morphs.Get(index)
		if err != nil || morphData == nil {
			logMorphRenameDebug("モーフ名称一覧: index=%d status=morph_nil_or_unreadable", index)
			continue
		}
		name := strings.TrimSpace(morphData.Name())
		if name == "" {
			logMorphRenameDebug("モーフ名称一覧: index=%d panel=%d status=name_empty", index, morphData.Panel)
			continue
		}
		logMorphRenameDebug("モーフ名称一覧: index=%d name=%s panel=%d", index, name, morphData.Panel)
	}
	logMorphRenameDebug("モーフ名称一覧終了: count=%d", count)
}

// resolveMorphRenameTargetCount はrename-only対象件数（モーフ総数）を返す。
func resolveMorphRenameTargetCount(modelData *ModelData) int {
	if modelData == nil || modelData.Morphs == nil {
		return 0
	}
	return modelData.Morphs.Len()
}

// collectMorphRenameOperations はモデル内モーフ名と対応表を照合し、操作一覧と未検出件数を返す。
func collectMorphRenameOperations(morphs *collection.NamedCollection[*model.Morph]) (map[int]morphRenameOperation, int) {
	operations := map[int]morphRenameOperation{}
	if morphs == nil {
		return operations, len(morphRenameSourceRules)
	}

	foundSources := map[string]struct{}{}
	for index := 0; index < morphs.Len(); index++ {
		morphData, err := morphs.Get(index)
		if err != nil || morphData == nil {
			continue
		}
		sourceName := strings.TrimSpace(morphData.Name())
		if sourceName == "" {
			continue
		}
		rule, exists := morphRenameSourceRules[sourceName]
		if !exists {
			rule, exists = morphRenameSourceRules[strings.ToLower(sourceName)]
		}
		if !exists {
			continue
		}
		operations[index] = morphRenameOperation{
			Index:       index,
			SourceName:  sourceName,
			TargetName:  rule.Name,
			TargetPanel: rule.Panel,
		}
		foundSources[sourceName] = struct{}{}
	}
	notFound := len(morphRenameSourceRules) - len(foundSources)
	if notFound < 0 {
		notFound = 0
	}
	return operations, notFound
}

// buildMorphRenamePlannedFlags は各モーフのrename実行可否を返す。
func buildMorphRenamePlannedFlags(
	morphs *collection.NamedCollection[*model.Morph],
	operations map[int]morphRenameOperation,
) map[int]bool {
	planned := map[int]bool{}
	renameOps := map[int]morphRenameOperation{}
	targets := map[string][]int{}

	for index, operation := range operations {
		if operation.SourceName == operation.TargetName {
			planned[index] = false
			continue
		}
		planned[index] = true
		renameOps[index] = operation
		targets[operation.TargetName] = append(targets[operation.TargetName], index)
	}

	for _, indexes := range targets {
		if len(indexes) < 2 {
			continue
		}
		for _, index := range indexes {
			planned[index] = false
		}
	}

	for index, operation := range renameOps {
		if !planned[index] || morphs == nil {
			continue
		}
		existing, err := morphs.GetByName(operation.TargetName)
		if err != nil || existing == nil {
			continue
		}
		existingIndex := existing.Index()
		if existingIndex == index {
			continue
		}
		if _, moving := renameOps[existingIndex]; moving {
			continue
		}
		planned[index] = false
	}

	return planned
}

// applyMorphTemporaryRenames は衝突回避のためrename対象モーフへ一時名を割り当てる。
func applyMorphTemporaryRenames(
	morphs *collection.NamedCollection[*model.Morph],
	operations map[int]morphRenameOperation,
	planned map[int]bool,
) map[int]string {
	tempRenamed := map[int]string{}
	if morphs == nil {
		return tempRenamed
	}

	indexes := make([]int, 0, len(planned))
	for index, canRename := range planned {
		if !canRename {
			continue
		}
		if _, exists := operations[index]; !exists {
			continue
		}
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	serial := 0
	for _, index := range indexes {
		tempName := nextTemporaryMorphName(morphs, &serial)
		renamed, err := morphs.Rename(index, tempName)
		if err != nil || !renamed {
			planned[index] = false
			continue
		}
		tempRenamed[index] = tempName
	}

	return tempRenamed
}

// applyMorphRenameOperation は1モーフ分のrename-only補正を適用し、変更有無を返す。
func applyMorphRenameOperation(
	morphs *collection.NamedCollection[*model.Morph],
	morphData *model.Morph,
	operation morphRenameOperation,
	canRename bool,
	tempName string,
) morphRenameApplyResult {
	result := morphRenameApplyResult{Status: "unchanged"}
	if morphData == nil {
		result.Status = "morph_nil"
		return result
	}

	if canRename {
		if tempName != "" && morphs != nil {
			renamed, err := morphs.Rename(operation.Index, operation.TargetName)
			if err != nil {
				result.RenameError = err
				result.Status = "rename_error"
			} else if renamed {
				result.NameRenamed = true
				result.Status = "name_renamed"
			} else {
				result.Status = "rename_not_applied"
			}
			if refreshed, getErr := morphs.Get(operation.Index); getErr == nil && refreshed != nil {
				morphData = refreshed
			}
		} else {
			result.Status = "rename_skipped_temp_not_assigned"
		}
	} else {
		result.Status = "rename_skipped"
	}

	if morphData.Panel != operation.TargetPanel {
		morphData.Panel = operation.TargetPanel
		result.PanelChanged = true
	}

	if morphData.Name() == operation.TargetName && morphData.EnglishName != operation.TargetName {
		morphData.EnglishName = operation.TargetName
		result.EnglishNameChanged = true
	}

	if result.NameRenamed && result.PanelChanged && result.EnglishNameChanged {
		result.Status = "name_panel_english_updated"
	} else if result.NameRenamed && result.PanelChanged {
		result.Status = "name_panel_updated"
	} else if result.NameRenamed {
		result.Status = "name_updated"
	} else if result.PanelChanged {
		result.Status = "panel_updated_only"
	} else if result.EnglishNameChanged {
		result.Status = "english_updated_only"
	}

	return result
}

// nextTemporaryMorphName は重複しない一時モーフ名を返す。
func nextTemporaryMorphName(morphs *collection.NamedCollection[*model.Morph], serial *int) string {
	if serial == nil {
		return morphRenameTempPrefix + "000"
	}
	for {
		candidate := fmt.Sprintf("%s%03d", morphRenameTempPrefix, *serial)
		*serial = *serial + 1
		if morphs == nil {
			return candidate
		}
		if _, err := morphs.GetByName(candidate); err != nil {
			return candidate
		}
	}
}

// flushMorphRenameProgress は進捗イベント/INFOログをチャンク単位で出力し、未送信件数を返す。
func flushMorphRenameProgress(
	progressReporter IPrepareProgressReporter,
	summary morphRenameSummary,
	pending int,
	force bool,
) int {
	if pending < morphRenameProgressChunkSize && !(force && pending > 0) {
		return pending
	}
	reportPrepareProgress(progressReporter, PrepareProgressEvent{
		Type:       PrepareProgressEventTypeMorphRenameProcessed,
		MorphCount: pending,
	})
	logMorphRenameInfo(
		morphRenameInfoProgressFormat,
		summary.Processed,
		summary.Targets,
		summary.Renamed,
		summary.Unchanged,
	)
	return 0
}

// logMorphRenameInfo はモーフ名称変換のINFOログを出力する。
func logMorphRenameInfo(format string, params ...any) {
	logger := logging.DefaultLogger()
	if logger == nil {
		return
	}
	logger.Info(format, params...)
	if logger.IsVerboseEnabled(logging.VERBOSE_INDEX_VIEWER) {
		logger.Verbose(logging.VERBOSE_INDEX_VIEWER, "[INFO] "+format, params...)
	}
}

// logMorphRenameDebug はモーフ名称変換のDEBUGログを出力する。
func logMorphRenameDebug(format string, params ...any) {
	logger := logging.DefaultLogger()
	if logger == nil {
		return
	}
	logger.Debug(format, params...)
	if logger.IsVerboseEnabled(logging.VERBOSE_INDEX_VIEWER) {
		logger.Verbose(logging.VERBOSE_INDEX_VIEWER, "[DEBUG] "+format, params...)
	}
}

// vrm1ExpressionDefinitionCheck はVRM1表情定義の有無判定用構造を表す。
type vrm1ExpressionDefinitionCheck struct {
	Expressions vrm1ExpressionSetDefinitionCheck `json:"expressions"`
}

// vrm1ExpressionSetDefinitionCheck はVRM1 preset/custom表情定義を表す。
type vrm1ExpressionSetDefinitionCheck struct {
	Preset map[string]json.RawMessage `json:"preset"`
	Custom map[string]json.RawMessage `json:"custom"`
}

// vrm0BlendShapeDefinitionCheck はVRM0表情定義の有無判定用構造を表す。
type vrm0BlendShapeDefinitionCheck struct {
	BlendShapeMaster vrm0BlendShapeMasterDefinitionCheck `json:"blendShapeMaster"`
}

// vrm0BlendShapeMasterDefinitionCheck はVRM0 blendShapeGroups定義を表す。
type vrm0BlendShapeMasterDefinitionCheck struct {
	BlendShapeGroups []json.RawMessage `json:"blendShapeGroups"`
}

// hasVrmExpressionDefinitions はVRM拡張に表情定義があるか判定する。
func hasVrmExpressionDefinitions(modelData *ModelData) bool {
	if modelData == nil || modelData.VrmData == nil || modelData.VrmData.RawExtensions == nil {
		return false
	}
	rawExtensions := modelData.VrmData.RawExtensions
	if raw, exists := rawExtensions["VRMC_vrm"]; exists && hasVrm1ExpressionDefinitions(raw) {
		return true
	}
	if raw, exists := rawExtensions["VRM"]; exists && hasVrm0BlendShapeDefinitions(raw) {
		return true
	}
	return false
}

// hasVrm1ExpressionDefinitions はVRM1 expressions定義の有無を返す。
func hasVrm1ExpressionDefinitions(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	check := vrm1ExpressionDefinitionCheck{}
	if err := json.Unmarshal(raw, &check); err != nil {
		return false
	}
	return len(check.Expressions.Preset) > 0 || len(check.Expressions.Custom) > 0
}

// hasVrm0BlendShapeDefinitions はVRM0 blendShapeGroups定義の有無を返す。
func hasVrm0BlendShapeDefinitions(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	check := vrm0BlendShapeDefinitionCheck{}
	if err := json.Unmarshal(raw, &check); err != nil {
		return false
	}
	return len(check.BlendShapeMaster.BlendShapeGroups) > 0
}
