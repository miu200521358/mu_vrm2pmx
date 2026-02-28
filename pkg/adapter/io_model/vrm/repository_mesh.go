// 指示: miu200521358
package vrm

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/miu200521358/mlib_go/pkg/adapter/io_common"
	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
	warningid "github.com/miu200521358/mu_vrm2pmx/pkg/domain/model"
	"golang.org/x/image/bmp"
	"gonum.org/v1/gonum/spatial/r3"
)

const (
	gltfComponentTypeByte          = 5120
	gltfComponentTypeUnsignedByte  = 5121
	gltfComponentTypeShort         = 5122
	gltfComponentTypeUnsignedShort = 5123
	gltfComponentTypeUnsignedInt   = 5125
	gltfComponentTypeFloat         = 5126

	gltfPrimitiveModePoints        = 0
	gltfPrimitiveModeLines         = 1
	gltfPrimitiveModeLineLoop      = 2
	gltfPrimitiveModeLineStrip     = 3
	gltfPrimitiveModeTriangles     = 4
	gltfPrimitiveModeTriangleStrip = 5
	gltfPrimitiveModeTriangleFan   = 6

	vroidMeterScale = 12.5

	createMorphBrowDefaultDistance        = 0.2
	createMorphBrowDistanceRatio          = 0.6
	createMorphBrowProjectionZOffset      = 0.02
	createMorphEyeHideScaleY              = 1.05
	createMorphEyeHideFaceFrontZOffset    = 0.1
	createMorphEyeFallbackScaleRatio      = 0.15
	createMorphProjectionLineHalfDistance = 1000.0

	legacyGeneratedSphereDirName = "sphere"
)

// vrmConversion はVRM->PMX変換時の座標設定を表す。
type vrmConversion struct {
	Scale          float64
	Axis           mmath.Vec3
	ReverseWinding bool
}

// accessorReadPlan はaccessorの読み取り計画を表す。
type accessorReadPlan struct {
	Accessor      gltfAccessor
	ComponentSize int
	ComponentNum  int
	Stride        int
	BaseOffset    int
	ViewStart     int
	ViewEnd       int
}

// accessorValueCache はaccessor値の再読込みを抑止するキャッシュ。
type accessorValueCache struct {
	floatValues  map[int][][]float64
	intValues    map[int][][]int
	vertexRanges map[string]primitiveVertexRange
}

// primitiveVertexRange はprimitiveで生成した頂点範囲を表す。
type primitiveVertexRange struct {
	Start int
	Count int
}

// weightedBone はボーンウェイト計算用の一時構造体。
type weightedBone struct {
	BoneIndex int
	Weight    float64
}

// nodeTargetKey は VRM1 expression bind の node/index を識別する。
type nodeTargetKey struct {
	NodeIndex   int
	TargetIndex int
}

// meshTargetKey は VRM0 blendShape bind の mesh/index を識別する。
type meshTargetKey struct {
	MeshIndex   int
	TargetIndex int
}

// targetMorphRegistry は morph target 由来モーフの参照表を表す。
type targetMorphRegistry struct {
	ByNodeAndTarget map[nodeTargetKey]int
	ByMeshAndTarget map[meshTargetKey]int
	ByGltfMaterial  map[int][]int
	ByMaterialName  map[string][]int
}

// vrm0MaterialPropertiesSource は VRM0 materialProperties の最小構造を表す。
type vrm0MaterialPropertiesSource struct {
	MaterialProperties []vrm0MaterialPropertySource `json:"materialProperties"`
}

// vrm0MaterialPropertySource は VRM0 materialProperties 要素を表す。
type vrm0MaterialPropertySource struct {
	Name              string               `json:"name"`
	FloatProperties   map[string]float64   `json:"floatProperties"`
	VectorProperties  map[string][]float64 `json:"vectorProperties"`
	TextureProperties map[string]float64   `json:"textureProperties"`
}

// gltfMaterialMToonSource は VRMC_materials_mtoon 拡張の最小構造を表す。
type gltfMaterialMToonSource struct {
	OutlineWidthMode   string          `json:"outlineWidthMode"`
	OutlineWidthFactor float64         `json:"outlineWidthFactor"`
	OutlineColorFactor []float64       `json:"outlineColorFactor"`
	ShadeColorFactor   []float64       `json:"shadeColorFactor"`
	MatcapTexture      *gltfTextureRef `json:"matcapTexture"`
	MatcapFactor       []float64       `json:"matcapFactor"`
}

// gltfMaterialEmissiveStrengthSource は KHR_materials_emissive_strength 拡張の最小構造を表す。
type gltfMaterialEmissiveStrengthSource struct {
	EmissiveStrength float64 `json:"emissiveStrength"`
}

// createMorphRuleType は creates フォールバック生成種別を表す。
type createMorphRuleType int

const (
	createMorphRuleTypeBrow createMorphRuleType = iota
	createMorphRuleTypeEyeSmall
	createMorphRuleTypeEyeBig
	createMorphRuleTypeEyeHideVertex
)

const (
	createSemanticBrow      = "brow"
	createSemanticIris      = "iris"
	createSemanticHighlight = "highlight"
	createSemanticEyeWhite  = "eyewhite"
	createSemanticEyeLine   = "eyeline"
	createSemanticEyeLash   = "eyelash"
	createSemanticFace      = "face"
	createSemanticSkin      = "skin"
)

// createMorphRule は creates フォールバック生成規則を表す。
type createMorphRule struct {
	Name    string
	Panel   model.MorphPanel
	Type    createMorphRuleType
	Creates []string
	Hides   []string
}

// expressionLinkRule は binds/split の表情連動規則を表す。
type expressionLinkRule struct {
	Name   string
	Panel  model.MorphPanel
	Binds  []string
	Ratios []float64
	Split  string
}

// expressionSidePairGroupFallbackRule は左右モーフから親グループを補完する規則を表す。
type expressionSidePairGroupFallbackRule struct {
	Name  string
	Panel model.MorphPanel
	Binds []string
}

const (
	specialEyeClassIris    = "iris"
	specialEyeClassWhite   = "white"
	specialEyeClassEyeLine = "eyeline"
	specialEyeClassEyeLash = "eyelash"
	specialEyeClassFace    = "face"
)

// specialEyeAugmentRule は特殊目追加材質の生成規則を表す。
type specialEyeAugmentRule struct {
	EyeClass     string
	TextureToken string
}

// specialEyeMaterialMorphRule は特殊目材質モーフ生成規則を表す。
type specialEyeMaterialMorphRule struct {
	MorphName    string
	Panel        model.MorphPanel
	TextureToken string
	HideClasses  []string
}

// specialEyeMaterialInfo は特殊目判定用の材質情報を表す。
type specialEyeMaterialInfo struct {
	MaterialIndex          int
	Name                   string
	EnglishName            string
	TextureName            string
	TextureURI             string
	NormalizedName         string
	NormalizedEnglishName  string
	NormalizedTextureMatch string
	Classes                map[string]struct{}
	IsOverlay              bool
}

// specialEyeMaterialAugmentStats は特殊目追加材質生成の集計情報を表す。
type specialEyeMaterialAugmentStats struct {
	RuleCount          int
	GeneratedMaterials int
	GeneratedFaces     int
	SkippedNoBase      int
	SkippedNoTexture   int
}

// specialEyeMaterialMorphStats は特殊目材質モーフ生成の集計情報を表す。
type specialEyeMaterialMorphStats struct {
	RuleCount       int
	Generated       int
	SkippedExisting int
	SkippedNoTarget int
}

var specialEyeOverlayTextureTokens = []string{
	"eye_star",
	"eye_heart",
	"eye_hau",
	"eye_hachume",
	"eye_nagomi",
	"cheek_dye",
}

var specialEyeAugmentRules = []specialEyeAugmentRule{
	{EyeClass: specialEyeClassIris, TextureToken: "eye_star"},
	{EyeClass: specialEyeClassIris, TextureToken: "eye_heart"},
	{EyeClass: specialEyeClassFace, TextureToken: "cheek_dye"},
	{EyeClass: specialEyeClassWhite, TextureToken: "eye_hau"},
	{EyeClass: specialEyeClassWhite, TextureToken: "eye_hachume"},
	{EyeClass: specialEyeClassWhite, TextureToken: "eye_nagomi"},
}

var specialEyeMaterialMorphFallbackRules = []specialEyeMaterialMorphRule{
	{MorphName: "はぅ材質", Panel: model.MORPH_PANEL_SYSTEM, TextureToken: "eye_hau", HideClasses: []string{specialEyeClassWhite, specialEyeClassEyeLine, specialEyeClassEyeLash}},
	{MorphName: "はちゅ目材質", Panel: model.MORPH_PANEL_SYSTEM, TextureToken: "eye_hachume", HideClasses: []string{specialEyeClassWhite, specialEyeClassEyeLine, specialEyeClassEyeLash}},
	{MorphName: "なごみ材質", Panel: model.MORPH_PANEL_SYSTEM, TextureToken: "eye_nagomi", HideClasses: []string{specialEyeClassWhite, specialEyeClassEyeLine, specialEyeClassEyeLash}},
	{MorphName: "星目材質", Panel: model.MORPH_PANEL_SYSTEM, TextureToken: "eye_star"},
	{MorphName: "はぁと材質", Panel: model.MORPH_PANEL_SYSTEM, TextureToken: "eye_heart"},
	{MorphName: "照れ", Panel: model.MORPH_PANEL_OTHER_LOWER_RIGHT, TextureToken: "cheek_dye"},
}

var primitiveTargetMorphPrefixRegexp = regexp.MustCompile(`(?i)^__vrm_target_m[0-9]+_t[0-9]+_`)

// vrm1PresetExpressionNamePairs は VRM1 標準preset名を MMD モーフ名へ正規化する対応を表す。
var vrm1PresetExpressionNamePairs = map[string]string{
	"aa":         "あ頂点",
	"ih":         "い頂点",
	"ou":         "う頂点",
	"ee":         "え頂点",
	"oh":         "お頂点",
	"blink":      "まばたき",
	"blinkleft":  "ウィンク２",
	"blinkright": "ｳｨﾝｸ２右",
	"neutral":    "ニュートラル",
	"angry":      "怒",
	"relaxed":    "楽",
	"happy":      "喜",
	"sad":        "哀",
	"surprised":  "驚",
}

// vrm0PresetExpressionNamePairs は VRM0 標準preset名を MMD モーフ名へ正規化する対応を表す。
var vrm0PresetExpressionNamePairs = map[string]string{
	"a":         "あ頂点",
	"i":         "い頂点",
	"u":         "う頂点",
	"e":         "え頂点",
	"o":         "お頂点",
	"blink":     "まばたき",
	"blink_l":   "ウィンク２",
	"blink_r":   "ｳｨﾝｸ２右",
	"neutral":   "ニュートラル",
	"angry":     "怒",
	"fun":       "楽",
	"joy":       "喜",
	"sorrow":    "哀",
	"surprised": "驚",
}

// legacyExpressionNamePairs は Fcl/旧キーを MMD モーフ名へ正規化する対応を表す。
var legacyExpressionNamePairs = map[string]string{
	"fcl_all_neutral":   "ニュートラル",
	"fcl_all_angry":     "怒",
	"fcl_all_fun":       "楽",
	"fcl_all_joy":       "喜",
	"fcl_all_sorrow":    "哀",
	"fcl_all_surprised": "驚",

	"fcl_brw_angry":     "怒り",
	"fcl_brw_fun":       "にこり",
	"fcl_brw_joy":       "にこり2",
	"fcl_brw_sorrow":    "困る",
	"fcl_brw_surprised": "驚き",

	"fcl_eye_natural":          "ナチュラル",
	"fcl_eye_angry":            "ｷﾘｯ",
	"fcl_eye_close":            "まばたき",
	"fcl_eye_close_r":          "ｳｨﾝｸ２右",
	"fcl_eye_close_l":          "ウィンク２",
	"fcl_eye_fun":              "目を細める",
	"fcl_eye_joy":              "笑い",
	"fcl_eye_joy_r":            "ウィンク右",
	"fcl_eye_joy_l":            "ウィンク",
	"fcl_eye_sorrow":           "じと目",
	"fcl_eye_spread":           "上瞼↑",
	"fcl_eye_surprised":        "びっくり",
	"fcl_eye_iris_hide":        "白目",
	"fcl_eye_highlight_hide":   "目光なし",
	"fcl_eye_highlight_hide_r": "目光なし右",
	"fcl_eye_highlight_hide_l": "目光なし左",
	"fcl_eye_iris_hide_r":      "白目右",
	"fcl_eye_iris_hide_l":      "白目左",
	"fcl_eye_surprised_r":      "びっくり右",
	"fcl_eye_surprised_l":      "びっくり左",
	"fcl_eye_spread_r":         "上瞼↑右",
	"fcl_eye_spread_l":         "上瞼↑左",
	"fcl_eye_fun_r":            "目を細める右",
	"fcl_eye_fun_l":            "目を細める左",

	"fcl_brw_angry_r":     "怒り右",
	"fcl_brw_angry_l":     "怒り左",
	"fcl_brw_fun_r":       "にこり右",
	"fcl_brw_fun_l":       "にこり左",
	"fcl_brw_joy_r":       "にこり2右",
	"fcl_brw_joy_l":       "にこり2左",
	"fcl_brw_sorrow_r":    "困る右",
	"fcl_brw_sorrow_l":    "困る左",
	"fcl_brw_surprised_r": "驚き右",
	"fcl_brw_surprised_l": "驚き左",

	"raiseeyelid_r": "下瞼上げ右",
	"raiseeyelid_l": "下瞼上げ左",
	"raiseeyelid":   "下瞼上げ",

	"eyesquintright": "にんまり右",
	"eyesquintleft":  "にんまり左",
	"eyesquint":      "にんまり",
	"eyewideright":   "びっくり2右",
	"eyewideleft":    "びっくり2左",
	"eyewide":        "びっくり2",

	"eyelookupright":   "目上右",
	"eyelookupleft":    "目上左",
	"eyelookup":        "目上",
	"eyelookdownright": "目下右",
	"eyelookdownleft":  "目下左",
	"eyelookdown":      "目下",
	"eyelookinright":   "目頭広右",
	"eyelookinleft":    "目頭広左",
	"eyelookin":        "目頭広",
	"eyelookoutright":  "目尻広左",
	"eyelookoutleft":   "目尻広右",
	"eyelookout":       "目尻広",

	"_eyeirismoveback_r": "瞳小2右",
	"_eyeirismoveback_l": "瞳小2左",
	"_eyeirismoveback":   "瞳小2",

	"_eyesquint+lowerup_r": "下瞼上げ2右",
	"_eyesquint+lowerup_l": "下瞼上げ2左",
	"_eyesquint+lowerup":   "下瞼上げ2",

	"eye_nanu_r": "なぬ！右",
	"eye_nanu_l": "なぬ！左",
	"eye_nanu":   "なぬ！",

	"brow_below_r":     "下右",
	"brow_below_l":     "下左",
	"brow_below":       "下",
	"brow_abobe_r":     "上右",
	"brow_abobe_l":     "上左",
	"brow_abobe":       "上",
	"brow_left_r":      "右眉左",
	"brow_left_l":      "左眉左",
	"brow_left":        "眉左",
	"brow_right_r":     "右眉右",
	"brow_right_l":     "左眉右",
	"brow_right":       "眉右",
	"brow_front_r":     "右眉手前",
	"brow_front_l":     "左眉手前",
	"brow_front":       "眉手前",
	"brow_serious_r":   "真面目右",
	"brow_serious_l":   "真面目左",
	"brow_serious":     "真面目",
	"brow_frown_r":     "ひそめ右",
	"brow_frown_l":     "ひそめ左",
	"brow_frown":       "ひそめ",
	"browinnerup_r":    "ひそめる2右",
	"browinnerup_l":    "ひそめる2左",
	"browinnerup":      "ひそめる2",
	"browdownright":    "真面目2右",
	"browdownleft":     "真面目2左",
	"browdown":         "真面目2",
	"browouterupright": "はんっ右",
	"browouterupleft":  "はんっ左",
	"browouter":        "はんっ",

	"eye_small_r":          "瞳小右",
	"eye_small_l":          "瞳小左",
	"eye_small":            "瞳小",
	"eye_big_r":            "瞳大右",
	"eye_big_l":            "瞳大左",
	"eye_big":              "瞳大",
	"eye_hide_vertex":      "目隠し頂点",
	"eye_hau_material":     "はぅ材質",
	"eye_hau":              "はぅ",
	"eye_hachume_material": "はちゅ目材質",
	"eye_hachume":          "はちゅ目",
	"eye_nagomi_material":  "なごみ材質",
	"eye_nagomi":           "なごみ",
	"eye_star_material":    "星目材質",
	"eye_heart_material":   "はぁと材質",
	"eye_star":             "星目",
	"eye_heart":            "はぁと",

	"fcl_eye_close_r_bone":  "ｳｨﾝｸ２右ボーン",
	"fcl_eye_close_r_group": "ｳｨﾝｸ２右連動",
	"fcl_eye_close_l_bone":  "ウィンク２ボーン",
	"fcl_eye_close_l_group": "ウィンク２連動",
	"fcl_eye_close_group":   "まばたき連動",
	"fcl_eye_joy_r_bone":    "ウィンク右ボーン",
	"fcl_eye_joy_r_group":   "ウィンク右連動",
	"fcl_eye_joy_l_bone":    "ウィンクボーン",
	"fcl_eye_joy_l_group":   "ウィンク連動",
	"fcl_eye_joy_group":     "笑い連動",
	"fcl_eye_angry_r":       "ｷﾘｯ右",
	"fcl_eye_angry_l":       "ｷﾘｯ左",
	"nosesneerright":        "ｷﾘｯ2右",
	"nosesneerleft":         "ｷﾘｯ2左",
	"nosesneer":             "ｷﾘｯ2",
	"fcl_eye_sorrow_r":      "じと目右",
	"fcl_eye_sorrow_l":      "じと目左",

	"fcl_mth_neutral":         "ん",
	"fcl_mth_close":           "一文字",
	"fcl_mth_up":              "口上",
	"fcl_mth_down":            "口下",
	"fcl_mth_angry_r":         "Λ右",
	"fcl_mth_angry_l":         "Λ左",
	"fcl_mth_angry":           "Λ",
	"fcl_mth_sage_r":          "口角下げ右",
	"fcl_mth_sage_l":          "口角下げ左",
	"fcl_mth_sage":            "口角下げ",
	"fcl_mth_small":           "うー",
	"fcl_mth_large":           "口横広げ",
	"fcl_mth_fun_r":           "にっこり右",
	"fcl_mth_fun_l":           "にっこり左",
	"fcl_mth_fun":             "にっこり",
	"fcl_mth_niko_r":          "にこ右",
	"fcl_mth_niko_l":          "にこ左",
	"fcl_mth_niko":            "にこ",
	"fcl_mth_tongueout":       "べーボーン",
	"fcl_mth_tongueout_group": "べー",
	"fcl_mth_tongueup":        "ぺろりボーン",
	"fcl_mth_tongueup_group":  "ぺろり",

	"jawopen":    "あああ",
	"jawforward": "顎前",
	"jawleft":    "顎左",
	"jawright":   "顎右",

	"mouthfunnel":              "んむー",
	"mouthpucker":              "うー",
	"mouthleft":                "口左",
	"mouthright":               "口右",
	"mouthrollupper":           "上唇んむー",
	"mouthrolllower":           "下唇んむー",
	"mouthroll":                "んむー",
	"mouthshrugupper":          "上唇むむ",
	"mouthshruglower":          "下唇むむ",
	"mouthshrug":               "むむ",
	"mouthdimpleright":         "口幅広右",
	"mouthdimpleleft":          "口幅広左",
	"mouthdimple":              "口幅広",
	"mouthpressright":          "薄笑い右",
	"mouthpressleft":           "薄笑い左",
	"mouthpress":               "薄笑い",
	"mouthsmileright":          "にやり2右",
	"mouthsmileleft":           "にやり2左",
	"mouthsmile":               "にやり2",
	"mouthupperupright":        "にひ右",
	"mouthupperupleft":         "にひ左",
	"mouthupperup":             "にひ",
	"cheeksquintright":         "にひひ右",
	"cheeksquintleft":          "にひひ左",
	"cheeksquint":              "にひひ",
	"mouthfrownright":          "ちっ右",
	"mouthfrownleft":           "ちっ左",
	"mouthfrown":               "ちっ",
	"mouthlowerdownright":      "むっ右",
	"mouthlowerdownleft":       "むっ左",
	"mouthlowerdown":           "むっ",
	"mouthstretchright":        "ぎりっ右",
	"mouthstretchleft":         "ぎりっ左",
	"mouthstretch":             "ぎりっ",
	"tongueout":                "べー",
	"_mouthfunnel+sharpenlips": "うほっ",
	"_mouthpress+catmouth":     "ω口",
	"_mouthpress+catmouth-ex":  "ω口2",
	"_mouthpress+duckmouth":    "ω口3",
	"cheekpuff_r":              "ぷくー右",
	"cheekpuff_l":              "ぷくー左",
	"cheekpuff":                "ぷくー",

	"fcl_mth_skinfung_l": "肌牙左",
	"fcl_mth_skinfung_r": "肌牙右",
	"fcl_mth_skinfung":   "肌牙",
	"fcl_ha_fung1":       "牙",
	"fcl_ha_fung1_up_r":  "牙上右",
	"fcl_ha_fung1_up_l":  "牙上左",
	"fcl_ha_fung1_up":    "牙上",
	"fcl_ha_fung1_low_r": "牙下右",
	"fcl_ha_fung1_low_l": "牙下左",
	"fcl_ha_fung1_low":   "牙下",
	"fcl_ha_fung2_up":    "ギザ歯上",
	"fcl_ha_fung2_low":   "ギザ歯下",
	"fcl_ha_fung2":       "ギザ歯",
	"fcl_ha_fung3_up":    "真ん中牙上",
	"fcl_ha_fung3_low":   "真ん中牙下",
	"fcl_ha_fung3":       "真ん中牙",
	"fcl_ha_hide":        "歯隠",
	"fcl_ha_short_up":    "歯短上",
	"fcl_ha_short_low":   "歯短下",
	"fcl_ha_short":       "歯短",

	"cheek_dye": "照れ",
	"edge_off":  "エッジOFF",
}

// createFaceTriangle は顔面射影用の三角形情報を表す。
type createFaceTriangle struct {
	V0     mmath.Vec3
	V1     mmath.Vec3
	V2     mmath.Vec3
	Center mmath.Vec3
}

// createMorphStats は creates フォールバック生成の集計情報を表す。
type createMorphStats struct {
	RuleCount       int
	Generated       int
	SkippedExisting int
	SkippedNoTarget int
	SkippedNoOffset int
}

// createMorphFallbackRules は creates 対象を表す。
var createMorphFallbackRules = []createMorphRule{
	{
		Name:    "下右",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "下左",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "上右",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "上左",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "右眉左",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "左眉左",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "右眉右",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "左眉右",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "右眉手前",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "左眉手前",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "瞳小右",
		Panel:   model.MORPH_PANEL_EYE_UPPER_LEFT,
		Type:    createMorphRuleTypeEyeSmall,
		Creates: []string{"EyeIris", "EyeHighlight"},
	},
	{
		Name:    "瞳小左",
		Panel:   model.MORPH_PANEL_EYE_UPPER_LEFT,
		Type:    createMorphRuleTypeEyeSmall,
		Creates: []string{"EyeIris", "EyeHighlight"},
	},
	{
		Name:    "瞳大右",
		Panel:   model.MORPH_PANEL_EYE_UPPER_LEFT,
		Type:    createMorphRuleTypeEyeBig,
		Creates: []string{"EyeIris", "EyeHighlight"},
	},
	{
		Name:    "瞳大左",
		Panel:   model.MORPH_PANEL_EYE_UPPER_LEFT,
		Type:    createMorphRuleTypeEyeBig,
		Creates: []string{"EyeIris", "EyeHighlight"},
	},
	{
		Name:    "目隠し頂点",
		Panel:   model.MORPH_PANEL_SYSTEM,
		Type:    createMorphRuleTypeEyeHideVertex,
		Creates: []string{"EyeWhite"},
		Hides:   []string{"Eyeline", "Eyelash"},
	},
}

// expressionLinkRules は binds/split の表情連動規則を表す。
var expressionLinkRules = []expressionLinkRule{
	{
		Name:  "にこり右",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "にこり",
	},
	{
		Name:  "にこり左",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "にこり",
	},
	{
		Name:  "にこり2右",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "にこり2",
	},
	{
		Name:  "にこり2左",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "にこり2",
	},
	{
		Name:  "困る右",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "困る",
	},
	{
		Name:  "困る左",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "困る",
	},
	{
		Name:  "怒り右",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "怒り",
	},
	{
		Name:  "怒り左",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "怒り",
	},
	{
		Name:  "驚き右",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "驚き",
	},
	{
		Name:  "驚き左",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "驚き",
	},
	{
		Name:  "下",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"下右", "下左"},
	},
	{
		Name:  "上",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"上右", "上左"},
	},
	{
		Name:  "眉左",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"右眉左", "左眉左"},
	},
	{
		Name:  "眉右",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"右眉右", "左眉右"},
	},
	{
		Name:  "眉手前",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"右眉手前", "左眉手前"},
	},
	{
		Name:   "真面目右",
		Panel:  model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds:  []string{"怒り右", "下右"},
		Ratios: []float64{0.25, 0.7},
	},
	{
		Name:   "真面目左",
		Panel:  model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds:  []string{"怒り左", "下左"},
		Ratios: []float64{0.25, 0.7},
	},
	{
		Name:   "真面目",
		Panel:  model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds:  []string{"怒り右", "下右", "怒り左", "下左"},
		Ratios: []float64{0.25, 0.7, 0.25, 0.7},
	},
	{
		Name:   "ひそめ右",
		Panel:  model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds:  []string{"怒り右", "困る右", "右眉右"},
		Ratios: []float64{0.5, 0.5, 0.3},
	},
	{
		Name:   "ひそめ左",
		Panel:  model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds:  []string{"怒り左", "困る左", "左眉左"},
		Ratios: []float64{0.5, 0.5, 0.3},
	},
	{
		Name:   "ひそめ",
		Panel:  model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds:  []string{"怒り右", "困る右", "右眉右", "怒り左", "困る左", "左眉左"},
		Ratios: []float64{0.5, 0.5, 0.3, 0.5, 0.5, 0.3},
	},
	{
		Name:  "ひそめる2右",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "ひそめる2",
	},
	{
		Name:  "ひそめる2左",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "ひそめる2",
	},
	{
		Name:  "真面目2",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"真面目2右", "真面目2左"},
	},
	{
		Name:  "はんっ",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"はんっ右", "はんっ左"},
	},
	{
		Name:  "びっくり右",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "びっくり",
	},
	{
		Name:  "びっくり左",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "びっくり",
	},
	{
		Name:  "瞳小",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"瞳小右", "瞳小左"},
	},
	{
		Name:  "瞳大",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"瞳大右", "瞳大左"},
	},
	{
		Name:   "ｳｨﾝｸ２右連動",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"下右", "ｳｨﾝｸ２右", "瞳小右", "ｳｨﾝｸ２右ボーン", "右眉手前", "困る右"},
		Ratios: []float64{0.2, 1.0, 0.3, 1.0, 0.1, 0.2},
	},
	{
		Name:   "ウィンク２連動",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"下左", "ウィンク２", "瞳小左", "ウィンク２ボーン", "左眉手前", "困る左"},
		Ratios: []float64{0.2, 1.0, 0.3, 1.0, 0.1, 0.2},
	},
	{
		Name:   "まばたき連動",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"下右", "ｳｨﾝｸ２右", "瞳小右", "ｳｨﾝｸ２右ボーン", "右眉手前", "困る右", "下左", "ウィンク２", "瞳小左", "ウィンク２ボーン", "左眉手前", "困る左"},
		Ratios: []float64{0.2, 1.0, 0.3, 1.0, 0.1, 0.2, 0.2, 1.0, 0.3, 1.0, 0.1, 0.2},
	},
	{
		Name:   "ウィンク右連動",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"下右", "ウィンク右", "瞳小右", "ウィンク右ボーン", "右眉手前", "にこり右"},
		Ratios: []float64{0.5, 1.0, 0.3, 1.0, 0.1, 0.5},
	},
	{
		Name:   "ウィンク連動",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"下左", "ウィンク", "瞳小左", "ウィンクボーン", "左眉手前", "にこり左"},
		Ratios: []float64{0.5, 1.0, 0.3, 1.0, 0.1, 0.5},
	},
	{
		Name:   "笑い連動",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"下右", "ウィンク右", "瞳小右", "ウィンク右ボーン", "右眉手前", "にこり右", "下左", "ウィンク", "瞳小左", "ウィンクボーン", "左眉手前", "にこり左"},
		Ratios: []float64{0.5, 1.0, 0.3, 1.0, 0.1, 0.5, 0.5, 1.0, 0.3, 1.0, 0.1, 0.5},
	},
	{
		Name:  "目を細める右",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "目を細める",
	},
	{
		Name:  "目を細める左",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "目を細める",
	},
	{
		Name:  "下瞼上げ右",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "目を細める右",
	},
	{
		Name:  "下瞼上げ左",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "目を細める左",
	},
	{
		Name:  "下瞼上げ",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"下瞼上げ右", "下瞼上げ左"},
	},
	{
		Name:  "にんまり",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"にんまり右", "にんまり左"},
	},
	{
		Name:  "ｷﾘｯ右",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "ｷﾘｯ",
	},
	{
		Name:  "ｷﾘｯ左",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "ｷﾘｯ",
	},
	{
		Name:  "ｷﾘｯ2",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"ｷﾘｯ2右", "ｷﾘｯ2左"},
	},
	{
		Name:  "じと目右",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "じと目",
	},
	{
		Name:  "じと目左",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "じと目",
	},
	{
		Name:  "上瞼↑右",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "上瞼↑",
	},
	{
		Name:  "上瞼↑左",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "上瞼↑",
	},
	{
		Name:   "なぬ！右",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"びっくり右", "ｷﾘｯ右"},
		Ratios: []float64{1.0, 1.0},
	},
	{
		Name:   "なぬ！左",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"びっくり左", "ｷﾘｯ左"},
		Ratios: []float64{1.0, 1.0},
	},
	{
		Name:   "なぬ！",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"びっくり右", "ｷﾘｯ右", "びっくり左", "ｷﾘｯ左"},
		Ratios: []float64{1.0, 1.0, 1.0, 1.0},
	},
	{
		Name:  "はぅ",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"はぅ材質", "目隠し頂点"},
	},
	{
		Name:  "はちゅ目",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"はちゅ目材質", "目隠し頂点"},
	},
	{
		Name:  "なごみ",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"なごみ材質", "目隠し頂点"},
	},
	{
		Name:  "星目",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"目光なし", "星目材質"},
	},
	{
		Name:  "はぁと",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"目光なし", "はぁと材質"},
	},
	{
		Name:  "びっくり2",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"にんまり右", "にんまり左"},
	},
	{
		Name:  "目上",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"目上右", "目上左"},
	},
	{
		Name:  "目下",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"目下右", "目下左"},
	},
	{
		Name:  "目頭広",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"目頭広右", "目頭広左"},
	},
	{
		Name:  "目尻広",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"目尻広左", "目尻広右"},
	},
	{
		Name:  "瞳小2",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"瞳小2右", "瞳小2左"},
	},
	{
		Name:  "下瞼上げ2",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"下瞼上げ2右", "下瞼上げ2左"},
	},
	{
		Name:  "白目右",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "白目",
	},
	{
		Name:  "白目左",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "白目",
	},
	{
		Name:  "目光なし右",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "目光なし",
	},
	{
		Name:  "目光なし左",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "目光なし",
	},
	{
		Name:  "あ",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"あ頂点", "あボーン"},
	},
	{
		Name:  "い",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"い頂点", "いボーン"},
	},
	{
		Name:  "う",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"う頂点", "うボーン"},
	},
	{
		Name:  "え",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"え頂点", "えボーン"},
	},
	{
		Name:  "お",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"お頂点", "おボーン"},
	},
	{
		Name:  "Λ右",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "Λ",
	},
	{
		Name:  "Λ左",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "Λ",
	},
	{
		Name:   "口角下げ右",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"Λ右", "口横広げ"},
		Ratios: []float64{1.0, 0.5},
	},
	{
		Name:   "口角下げ左",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"Λ左", "口横広げ"},
		Ratios: []float64{1.0, 0.5},
	},
	{
		Name:   "口角下げ",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"Λ", "口横広げ"},
		Ratios: []float64{1.0, 0.5},
	},
	{
		Name:  "にっこり右",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "にっこり",
	},
	{
		Name:  "にっこり左",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "にっこり",
	},
	{
		Name:   "にこ右",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"にっこり右", "口横広げ"},
		Ratios: []float64{1.0, -0.3},
	},
	{
		Name:   "にこ左",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"にっこり左", "口横広げ"},
		Ratios: []float64{1.0, -0.3},
	},
	{
		Name:   "にこ",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"にっこり右", "にっこり左", "口横広げ"},
		Ratios: []float64{0.5, 0.5, -0.3},
	},
	{
		Name:  "ワ",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"ワ頂点", "ワボーン"},
	},
	{
		Name:  "▲",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"▲頂点", "▲ボーン"},
	},
	{
		Name:  "わー",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"わー頂点", "わーボーン"},
	},
	{
		Name:   "べー",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"あ頂点", "い頂点", "べーボーン"},
		Ratios: []float64{0.12, 0.56, 1.0},
	},
	{
		Name:   "ぺろり",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"あ頂点", "にっこり", "ぺろりボーン"},
		Ratios: []float64{0.12, 0.54, 1.0},
	},
	{
		Name:  "mouthRoll",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"上唇んむー", "下唇んむー"},
	},
	{
		Name:  "むむ",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"上唇むむ", "下唇むむ"},
	},
	{
		Name:  "口幅広",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"口幅広右", "口幅広左"},
	},
	{
		Name:  "薄笑い",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"薄笑い右", "薄笑い左"},
	},
	{
		Name:  "にやり2",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"にやり2右", "にやり2左"},
	},
	{
		Name:  "にひ",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"にひ右", "口幅広左"},
	},
	{
		Name:  "にひひ",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"にひひ右", "にひひ左"},
	},
	{
		Name:  "ちっ",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"ちっ右", "ちっ左"},
	},
	{
		Name:  "むっ",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"むっ右", "むっ左"},
	},
	{
		Name:  "ぎりっ",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"ぎりっ右", "ぎりっ左"},
	},
	{
		Name:  "ぷくー右",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "ぷくー",
	},
	{
		Name:  "ぷくー左",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "ぷくー",
	},
	{
		Name:  "牙上右",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "牙上",
	},
	{
		Name:  "牙上左",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "牙上",
	},
	{
		Name:  "牙下右",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "牙下",
	},
	{
		Name:  "牙下左",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "牙下",
	},
}

// expressionSidePairGroupFallbackRules は左右のみ存在する場合に親グループを補完する規則を表す。
var expressionSidePairGroupFallbackRules = []expressionSidePairGroupFallbackRule{
	{
		Name:  "白目",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"白目右", "白目左"},
	},
	{
		Name:  "目光なし",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"目光なし右", "目光なし左"},
	},
}

// buildVrmConversion はVRMプロファイルに応じた座標変換設定を返す。
func buildVrmConversion(vrmData *vrm.VrmData) vrmConversion {
	conversion := vrmConversion{
		Scale: vroidMeterScale,
		Axis: mmath.Vec3{
			Vec: r3.Vec{X: -1.0, Y: 1.0, Z: 1.0},
		},
	}
	if vrmData != nil && vrmData.Profile == vrm.VRM_PROFILE_VROID {
		// VRoidはVRM0/VRM1で座標配置が異なるため、バージョンごとに軸変換を切り替える。
		// VRM0: 既存規約どおり X 反転（-1, 1, 1）
		// VRM1: VRoid Studio 1.x 出力の前向きを保つため標準軸（1, 1, -1）
		if vrmData.Version == vrm.VRM_VERSION_1 {
			conversion.Axis = mmath.Vec3{
				Vec: r3.Vec{X: 1.0, Y: 1.0, Z: -1.0},
			}
		}
	}
	conversion.ReverseWinding = conversion.Axis.X*conversion.Axis.Y*conversion.Axis.Z < 0
	return conversion
}

// convertVrmNormalToPmx は法線をVRM座標系からPMX座標系へ変換する。
func convertVrmNormalToPmx(v mmath.Vec3, conversion vrmConversion) mmath.Vec3 {
	converted := mmath.Vec3{
		Vec: r3.Vec{
			X: v.X * conversion.Axis.X,
			Y: v.Y * conversion.Axis.Y,
			Z: v.Z * conversion.Axis.Z,
		},
	}
	return converted.Normalized()
}

// appendMeshData はglTF mesh/primitives からPMXの頂点・面・材質を生成する。
func appendMeshData(
	modelData *model.PmxModel,
	doc *gltfDocument,
	binChunk []byte,
	nodeToBoneIndex map[int]int,
	conversion vrmConversion,
	progressReporter func(LoadProgressEvent),
) (*targetMorphRegistry, error) {
	if modelData == nil || doc == nil || len(doc.Meshes) == 0 {
		return newTargetMorphRegistry(), nil
	}

	textureIndexesByImage := appendImageTextures(modelData, doc.Images)
	appendEmbeddedSpecialEyeTextures(modelData)
	totalPrimitives := countGltfPrimitives(doc.Meshes)
	cache := newAccessorValueCache()
	targetMorphRegistry := newTargetMorphRegistry()
	primitiveStep := 0
	meshUniquePrimitiveIndex := map[int]map[string]int{}
	logVrmInfo(
		"VRMメッシュ変換開始: nodes=%d meshes=%d primitives=%d textures=%d",
		len(doc.Nodes),
		len(doc.Meshes),
		totalPrimitives,
		len(textureIndexesByImage),
	)

	for nodeIndex, node := range doc.Nodes {
		if node.Mesh == nil {
			continue
		}
		meshIndex := *node.Mesh
		if meshIndex < 0 || meshIndex >= len(doc.Meshes) {
			return targetMorphRegistry, io_common.NewIoParseFailed("node.mesh のindexが不正です: %d", nil, meshIndex)
		}

		mesh := doc.Meshes[meshIndex]
		logVrmInfo(
			"VRMメッシュ変換ステップ: node=%d mesh=%d primitives=%d",
			nodeIndex,
			meshIndex,
			len(mesh.Primitives),
		)
		seenByKey := meshUniquePrimitiveIndex[meshIndex]
		if seenByKey == nil {
			seenByKey = map[string]int{}
			meshUniquePrimitiveIndex[meshIndex] = seenByKey
		}
		for primitiveIndex, primitive := range mesh.Primitives {
			primitiveStep++
			primitiveName := resolvePrimitiveName(mesh, meshIndex, primitiveIndex)
			if shouldSkipPrimitiveForUnsupportedTargets(primitive, primitiveIndex, seenByKey) {
				logVrmDebug(
					"VRMプリミティブ変換スキップ: step=%d/%d node=%d mesh=%d primitive=%d name=%s reason=%s",
					primitiveStep,
					totalPrimitives,
					nodeIndex,
					meshIndex,
					primitiveIndex,
					primitiveName,
					"morph targets重複base primitiveを省略",
				)
				if progressReporter != nil {
					progressReporter(LoadProgressEvent{
						Type:           LoadProgressEventTypePrimitiveProcessed,
						PrimitiveTotal: totalPrimitives,
						PrimitiveDone:  primitiveStep,
					})
				}
				continue
			}
			beforeVertexCount := modelData.Vertices.Len()
			beforeFaceCount := modelData.Faces.Len()
			beforeMaterialCount := modelData.Materials.Len()
			logVrmDebug(
				"VRMプリミティブ変換開始: step=%d/%d node=%d mesh=%d primitive=%d name=%s",
				primitiveStep,
				totalPrimitives,
				nodeIndex,
				meshIndex,
				primitiveIndex,
				primitiveName,
			)
			if err := appendPrimitiveMeshData(
				modelData,
				doc,
				binChunk,
				nodeIndex,
				meshIndex,
				node,
				primitive,
				primitiveName,
				nodeToBoneIndex,
				textureIndexesByImage,
				conversion,
				cache,
				targetMorphRegistry,
			); err != nil {
				return targetMorphRegistry, err
			}
			logVrmDebug(
				"VRMプリミティブ変換完了: step=%d/%d node=%d mesh=%d primitive=%d name=%s addVertices=%d addFaces=%d addMaterials=%d",
				primitiveStep,
				totalPrimitives,
				nodeIndex,
				meshIndex,
				primitiveIndex,
				primitiveName,
				modelData.Vertices.Len()-beforeVertexCount,
				modelData.Faces.Len()-beforeFaceCount,
				modelData.Materials.Len()-beforeMaterialCount,
			)
			if progressReporter != nil {
				progressReporter(LoadProgressEvent{
					Type:           LoadProgressEventTypePrimitiveProcessed,
					PrimitiveTotal: totalPrimitives,
					PrimitiveDone:  primitiveStep,
				})
			}
		}
	}
	logVrmInfo(
		"VRMメッシュ変換完了: vertices=%d faces=%d materials=%d textures=%d",
		modelData.Vertices.Len(),
		modelData.Faces.Len(),
		modelData.Materials.Len(),
		modelData.Textures.Len(),
	)
	return targetMorphRegistry, nil
}

// appendEmbeddedSpecialEyeTextures は埋め込み特殊目テクスチャをモデルへ登録する。
func appendEmbeddedSpecialEyeTextures(modelData *model.PmxModel) {
	if modelData == nil || modelData.Textures == nil {
		return
	}
	addedCount := 0
	for _, fileName := range specialEyeEmbeddedTextureAssetFileNames {
		trimmedName := strings.TrimSpace(fileName)
		if trimmedName == "" {
			continue
		}
		if findSpecialEyeTextureIndexByToken(modelData, normalizeSpecialEyeToken(trimmedName)) >= 0 {
			continue
		}
		texture := model.NewTexture()
		texturePath := filepath.Join("tex", trimmedName)
		texture.SetName(texturePath)
		texture.EnglishName = texturePath
		texture.SetValid(true)
		modelData.Textures.AppendRaw(texture)
		addedCount++
	}
	if addedCount > 0 {
		logVrmInfo("特殊目埋め込みテクスチャ登録: added=%d", addedCount)
	}
}

// appendImageTextures はglTF image配列に対応するPMX Textureを作成する。
func appendImageTextures(modelData *model.PmxModel, images []gltfImage) []int {
	textureIndexes := make([]int, len(images))
	for imageIndex, image := range images {
		texture := model.NewTexture()
		texture.SetName(resolveImageTextureName(image, imageIndex))
		texture.SetValid(true)
		textureIndexes[imageIndex] = modelData.Textures.AppendRaw(texture)
	}
	return textureIndexes
}

// resolveImageTextureName はimage要素からPMX用テクスチャ名を決定する。
func resolveImageTextureName(image gltfImage, imageIndex int) string {
	uri := strings.TrimSpace(image.URI)
	if uri != "" && !strings.HasPrefix(uri, "data:") {
		return filepath.Base(filepath.FromSlash(uri))
	}
	name := strings.TrimSpace(image.Name)
	if name != "" {
		return name
	}
	return fmt.Sprintf("image_%03d", imageIndex)
}

// resolvePrimitiveName はメッシュ名とindexから材質名の基本名を生成する。
func resolvePrimitiveName(mesh gltfMesh, meshIndex int, primitiveIndex int) string {
	meshName := strings.TrimSpace(mesh.Name)
	if meshName == "" {
		meshName = fmt.Sprintf("mesh_%03d", meshIndex)
	}
	return fmt.Sprintf("%s_%03d", meshName, primitiveIndex)
}

// shouldSkipPrimitiveForUnsupportedTargets は同一base primitive の重複展開要否を判定する。
func shouldSkipPrimitiveForUnsupportedTargets(
	primitive gltfPrimitive,
	primitiveIndex int,
	seenByKey map[string]int,
) bool {
	if seenByKey == nil {
		return false
	}
	key := primitiveBaseKey(primitive)
	if key == "" {
		return false
	}
	if firstIndex, exists := seenByKey[key]; exists {
		// 同一base primitiveの重複展開を抑止する。
		return len(primitive.Targets) > 0 && firstIndex != primitiveIndex
	}
	seenByKey[key] = primitiveIndex
	return false
}

// primitiveBaseKey はtargetsを除くprimitiveの識別キーを返す。
func primitiveBaseKey(primitive gltfPrimitive) string {
	attributesKey := primitiveAttributesKey(primitive.Attributes)
	if attributesKey == "" {
		return ""
	}
	indices := -1
	if primitive.Indices != nil {
		indices = *primitive.Indices
	}
	material := -1
	if primitive.Material != nil {
		material = *primitive.Material
	}
	mode := gltfPrimitiveModeTriangles
	if primitive.Mode != nil {
		mode = *primitive.Mode
	}
	return fmt.Sprintf("attr=%s|idx=%d|mat=%d|mode=%d", attributesKey, indices, material, mode)
}

// primitiveAttributesKey はattribute mapの安定キーを生成する。
func primitiveAttributesKey(attributes map[string]int) string {
	if len(attributes) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", key, attributes[key]))
	}
	return strings.Join(parts, ",")
}

// primitiveVertexKey は頂点配列を再利用するためのキーを返す。
func primitiveVertexKey(nodeIndex int, primitive gltfPrimitive) string {
	return fmt.Sprintf("node=%d|attrs=%s", nodeIndex, primitiveAttributesKey(primitive.Attributes))
}

// newAccessorValueCache はaccessorキャッシュを初期化する。
func newAccessorValueCache() *accessorValueCache {
	return &accessorValueCache{
		floatValues:  map[int][][]float64{},
		intValues:    map[int][][]int{},
		vertexRanges: map[string]primitiveVertexRange{},
	}
}

// readFloatValues はfloat accessor値をキャッシュ付きで返す。
func (c *accessorValueCache) readFloatValues(doc *gltfDocument, accessorIndex int, binChunk []byte) ([][]float64, error) {
	if c == nil {
		return readAccessorFloatValues(doc, accessorIndex, binChunk)
	}
	if values, ok := c.floatValues[accessorIndex]; ok {
		return values, nil
	}
	values, err := readAccessorFloatValues(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, err
	}
	c.floatValues[accessorIndex] = values
	return values, nil
}

// readIntValues はint accessor値をキャッシュ付きで返す。
func (c *accessorValueCache) readIntValues(doc *gltfDocument, accessorIndex int, binChunk []byte) ([][]int, error) {
	if c == nil {
		return readAccessorIntValues(doc, accessorIndex, binChunk)
	}
	if values, ok := c.intValues[accessorIndex]; ok {
		return values, nil
	}
	values, err := readAccessorIntValues(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, err
	}
	c.intValues[accessorIndex] = values
	return values, nil
}

// getVertexRange は頂点範囲キャッシュから開始indexを取得する。
func (c *accessorValueCache) getVertexRange(key string, expectedCount int) (int, bool) {
	if c == nil || strings.TrimSpace(key) == "" {
		return 0, false
	}
	r, ok := c.vertexRanges[key]
	if !ok {
		return 0, false
	}
	if r.Count != expectedCount {
		return 0, false
	}
	return r.Start, true
}

// setVertexRange は頂点範囲キャッシュを登録する。
func (c *accessorValueCache) setVertexRange(key string, start int, count int) {
	if c == nil || strings.TrimSpace(key) == "" || start < 0 || count <= 0 {
		return
	}
	c.vertexRanges[key] = primitiveVertexRange{
		Start: start,
		Count: count,
	}
}

// appendPrimitiveMeshData はprimitiveをPMX頂点・面・材質へ変換する。
func appendPrimitiveMeshData(
	modelData *model.PmxModel,
	doc *gltfDocument,
	binChunk []byte,
	nodeIndex int,
	meshIndex int,
	node gltfNode,
	primitive gltfPrimitive,
	primitiveName string,
	nodeToBoneIndex map[int]int,
	textureIndexesByImage []int,
	conversion vrmConversion,
	cache *accessorValueCache,
	targetMorphRegistry *targetMorphRegistry,
) error {
	positionAccessor, ok := primitive.Attributes["POSITION"]
	if !ok {
		return io_common.NewIoParseFailed("mesh.primitive に POSITION がありません", nil)
	}

	positions, err := cache.readFloatValues(doc, positionAccessor, binChunk)
	if err != nil {
		return io_common.NewIoParseFailed("POSITION属性の読み取りに失敗しました(accessor=%d)", err, positionAccessor)
	}
	if len(positions) == 0 {
		return nil
	}

	normals, err := readOptionalFloatAttribute(doc, primitive.Attributes, "NORMAL", binChunk, cache)
	if err != nil {
		logVrmWarn(
			"VRM法線の読み取りに失敗したため既定法線で継続します: node=%d primitive=%s err=%s",
			nodeIndex,
			primitiveName,
			err.Error(),
		)
		normals = nil
	}
	uvs, err := readOptionalFloatAttribute(doc, primitive.Attributes, "TEXCOORD_0", binChunk, cache)
	if err != nil {
		return err
	}
	joints, err := readOptionalIntAttribute(doc, primitive.Attributes, "JOINTS_0", binChunk, cache)
	if err != nil {
		return err
	}
	weights, err := readOptionalFloatAttribute(doc, primitive.Attributes, "WEIGHTS_0", binChunk, cache)
	if err != nil {
		return err
	}

	indices, err := readPrimitiveIndices(doc, primitive, len(positions), binChunk, cache)
	if err != nil {
		return err
	}
	mode := gltfPrimitiveModeTriangles
	if primitive.Mode != nil {
		mode = *primitive.Mode
	}
	triangles := triangulateIndices(indices, mode)
	if len(triangles) == 0 {
		return nil
	}

	vertexStart, appendedVertices := appendOrReusePrimitiveVertices(
		modelData,
		doc,
		nodeIndex,
		node,
		primitive,
		positions,
		normals,
		uvs,
		joints,
		weights,
		nodeToBoneIndex,
		conversion,
		cache,
	)
	if appendedVertices == 0 {
		logVrmDebug(
			"VRM頂点再利用: node=%d primitive=%s vertexCount=%d",
			nodeIndex,
			primitiveName,
			len(positions),
		)
	}
	appendPrimitiveTargetMorphs(
		modelData,
		doc,
		binChunk,
		nodeIndex,
		meshIndex,
		primitive,
		vertexStart,
		len(positions),
		conversion,
		cache,
		appendedVertices > 0,
		targetMorphRegistry,
	)

	materialIndex := appendPrimitiveMaterial(
		modelData,
		doc,
		primitive,
		primitiveName,
		textureIndexesByImage,
		len(triangles)*3,
		targetMorphRegistry,
	)
	for _, tri := range triangles {
		if tri[0] < 0 || tri[1] < 0 || tri[2] < 0 {
			continue
		}
		if tri[0] >= len(positions) || tri[1] >= len(positions) || tri[2] >= len(positions) {
			return io_common.NewIoParseFailed("indices が頂点数を超えています", nil)
		}

		v0 := vertexStart + tri[0]
		v1 := vertexStart + tri[1]
		v2 := vertexStart + tri[2]
		if conversion.ReverseWinding {
			v0, v2 = v2, v0
		}
		face := &model.Face{
			VertexIndexes: [3]int{v0, v1, v2},
		}
		modelData.Faces.AppendRaw(face)
		appendVertexMaterialIndex(modelData, v0, materialIndex)
		appendVertexMaterialIndex(modelData, v1, materialIndex)
		appendVertexMaterialIndex(modelData, v2, materialIndex)
	}
	return nil
}

// newTargetMorphRegistry は morph target 参照表を初期化する。
func newTargetMorphRegistry() *targetMorphRegistry {
	return &targetMorphRegistry{
		ByNodeAndTarget: map[nodeTargetKey]int{},
		ByMeshAndTarget: map[meshTargetKey]int{},
		ByGltfMaterial:  map[int][]int{},
		ByMaterialName:  map[string][]int{},
	}
}

// appendPrimitiveTargetMorphs は primitive.targets を PMX 頂点モーフとして取り込む。
func appendPrimitiveTargetMorphs(
	modelData *model.PmxModel,
	doc *gltfDocument,
	binChunk []byte,
	nodeIndex int,
	meshIndex int,
	primitive gltfPrimitive,
	vertexStart int,
	vertexCount int,
	conversion vrmConversion,
	cache *accessorValueCache,
	appendOffsets bool,
	registry *targetMorphRegistry,
) {
	if modelData == nil || doc == nil || cache == nil || registry == nil || len(primitive.Targets) == 0 {
		return
	}
	targetNames := buildPrimitiveTargetNames(primitive, len(primitive.Targets))
	for targetIndex, target := range primitive.Targets {
		targetName := targetNames[targetIndex]
		morphName := resolvePrimitiveTargetMorphName(meshIndex, targetIndex, targetName)
		morphData, morphIndex := ensureVertexTargetMorph(modelData, morphName)
		if morphData == nil || morphIndex < 0 {
			continue
		}
		registerTargetMorphIndex(registry, nodeIndex, meshIndex, targetIndex, morphIndex)

		if !appendOffsets {
			continue
		}
		positionAccessor, exists := target["POSITION"]
		if !exists {
			continue
		}
		positionDeltas, err := cache.readFloatValues(doc, positionAccessor, binChunk)
		if err != nil {
			logVrmWarn(
				"VRMモーフターゲットの読み取りに失敗したため継続します: node=%d mesh=%d target=%d accessor=%d err=%s",
				nodeIndex,
				meshIndex,
				targetIndex,
				positionAccessor,
				err.Error(),
			)
			continue
		}
		limit := len(positionDeltas)
		if vertexCount < limit {
			limit = vertexCount
		}
		for deltaIndex := 0; deltaIndex < limit; deltaIndex++ {
			positionDelta := toVec3(positionDeltas[deltaIndex], mmath.ZERO_VEC3)
			convertedDelta := convertVrmPositionToPmx(positionDelta, conversion)
			if isZeroMorphDelta(convertedDelta) {
				continue
			}
			morphData.Offsets = append(morphData.Offsets, &model.VertexMorphOffset{
				VertexIndex: vertexStart + deltaIndex,
				Position:    convertedDelta,
			})
		}
	}
}

// buildPrimitiveTargetNames は target 数に合わせた morph 名配列を返す。
func buildPrimitiveTargetNames(primitive gltfPrimitive, count int) []string {
	names := make([]string, count)
	for targetIndex := 0; targetIndex < count; targetIndex++ {
		names[targetIndex] = fmt.Sprintf("target_%03d", targetIndex)
	}
	if primitive.Extras == nil || len(primitive.Extras.TargetNames) == 0 {
		return names
	}
	limit := len(primitive.Extras.TargetNames)
	if count < limit {
		limit = count
	}
	for targetIndex := 0; targetIndex < limit; targetIndex++ {
		name := strings.TrimSpace(primitive.Extras.TargetNames[targetIndex])
		if name == "" {
			continue
		}
		names[targetIndex] = name
	}
	return names
}

// resolvePrimitiveTargetMorphName は内部用の morph 名を返す。
func resolvePrimitiveTargetMorphName(meshIndex int, targetIndex int, targetName string) string {
	normalizedName := strings.TrimSpace(targetName)
	if normalizedName == "" {
		normalizedName = fmt.Sprintf("target_%03d", targetIndex)
	}
	return fmt.Sprintf("__vrm_target_m%03d_t%03d_%s", meshIndex, targetIndex, normalizedName)
}

// ensureVertexTargetMorph は内部頂点モーフを取得または作成する。
func ensureVertexTargetMorph(modelData *model.PmxModel, morphName string) (*model.Morph, int) {
	if modelData == nil || modelData.Morphs == nil {
		return nil, -1
	}
	if existing, err := modelData.Morphs.GetByName(morphName); err == nil && existing != nil {
		return existing, existing.Index()
	}
	morphData := &model.Morph{
		Panel:     model.MORPH_PANEL_SYSTEM,
		MorphType: model.MORPH_TYPE_VERTEX,
		Offsets:   []model.IMorphOffset{},
		IsSystem:  true,
	}
	morphData.SetName(morphName)
	morphData.EnglishName = morphName
	morphIndex := modelData.Morphs.AppendRaw(morphData)
	return morphData, morphIndex
}

// registerTargetMorphIndex は node/index と mesh/index の参照を更新する。
func registerTargetMorphIndex(
	registry *targetMorphRegistry,
	nodeIndex int,
	meshIndex int,
	targetIndex int,
	morphIndex int,
) {
	if registry == nil || targetIndex < 0 || morphIndex < 0 {
		return
	}
	registry.ByNodeAndTarget[nodeTargetKey{NodeIndex: nodeIndex, TargetIndex: targetIndex}] = morphIndex
	registry.ByMeshAndTarget[meshTargetKey{MeshIndex: meshIndex, TargetIndex: targetIndex}] = morphIndex
}

// isZeroMorphDelta はオフセットが実質 0 か判定する。
func isZeroMorphDelta(v mmath.Vec3) bool {
	const epsilon = 1e-9
	return math.Abs(v.X) <= epsilon && math.Abs(v.Y) <= epsilon && math.Abs(v.Z) <= epsilon
}

// appendExpressionMorphsFromVrmDefinition は VRM 定義表情を PMX モーフへ反映する。
func appendExpressionMorphsFromVrmDefinition(
	modelData *model.PmxModel,
	doc *gltfDocument,
	registry *targetMorphRegistry,
) {
	if modelData == nil || modelData.Morphs == nil || doc == nil || registry == nil || doc.Extensions == nil {
		return
	}
	loaded := false
	if raw, exists := doc.Extensions["VRMC_vrm"]; exists {
		applyVrm1ExpressionMorphs(modelData, raw, registry)
		loaded = true
	}
	if !loaded {
		raw, exists := doc.Extensions["VRM"]
		if !exists {
			return
		}
		applyVrm0BlendShapeMorphs(modelData, raw, registry)
		loaded = true
	}
	if loaded {
		appendCanonicalMorphsFromPrimitiveTargets(modelData)
		appendSpecialEyeMaterialMorphsFromFallbackRules(modelData, doc, registry)
		appendCreateMorphsFromFallbackRules(modelData, registry)
		appendExpressionEdgeFallbackMorph(modelData)
		appendExpressionBoneFallbackMorphs(modelData)
		appendExpressionLinkRules(modelData)
	}
}

// appendCanonicalMorphsFromPrimitiveTargets は内部ターゲット頂点モーフから正規名モーフを補完する。
func appendCanonicalMorphsFromPrimitiveTargets(modelData *model.PmxModel) {
	if modelData == nil || modelData.Morphs == nil {
		return
	}
	generated := 0
	for _, morphData := range modelData.Morphs.Values() {
		if morphData == nil || morphData.MorphType != model.MORPH_TYPE_VERTEX {
			continue
		}
		sourceName := strings.TrimSpace(morphData.Name())
		if !strings.HasPrefix(sourceName, "__vrm_target_") {
			continue
		}
		targetName := strings.TrimSpace(stripPrimitiveTargetMorphPrefix(sourceName))
		if targetName == "" {
			continue
		}
		canonicalName := strings.TrimSpace(resolveCanonicalExpressionName(targetName))
		if canonicalName == "" || canonicalName == targetName {
			continue
		}
		if existingMorph, getErr := modelData.Morphs.GetByName(canonicalName); getErr == nil && existingMorph != nil {
			continue
		}
		if len(morphData.Offsets) == 0 {
			if upsertTypedExpressionMorphAllowEmpty(
				modelData,
				canonicalName,
				resolveExpressionPanel(canonicalName),
				model.MORPH_TYPE_VERTEX,
				false,
			) != nil {
				generated++
			}
			continue
		}
		clonedOffsets := cloneVertexMorphOffsets(morphData.Offsets)
		if len(clonedOffsets) == 0 {
			continue
		}
		upsertTypedExpressionMorph(
			modelData,
			canonicalName,
			resolveExpressionPanel(canonicalName),
			model.MORPH_TYPE_VERTEX,
			clonedOffsets,
			false,
		)
		generated++
	}
	if generated > 0 {
		logVrmInfo("内部ターゲット正規名モーフ補完: generated=%d", generated)
	}
}

// upsertTypedExpressionMorphAllowEmpty はオフセットなしモーフを含む指定型モーフを作成または更新して返す。
func upsertTypedExpressionMorphAllowEmpty(
	modelData *model.PmxModel,
	morphName string,
	panel model.MorphPanel,
	morphType model.MorphType,
	isSystem bool,
) *model.Morph {
	if modelData == nil || modelData.Morphs == nil {
		return nil
	}
	normalizedName := strings.TrimSpace(morphName)
	if normalizedName == "" {
		return nil
	}
	if existing, err := modelData.Morphs.GetByName(normalizedName); err == nil && existing != nil {
		existing.Panel = panel
		existing.EnglishName = normalizedName
		existing.MorphType = morphType
		existing.Offsets = []model.IMorphOffset{}
		existing.IsSystem = isSystem
		return existing
	}
	morphData := &model.Morph{
		Panel:     panel,
		MorphType: morphType,
		Offsets:   []model.IMorphOffset{},
		IsSystem:  isSystem,
	}
	morphData.SetName(normalizedName)
	morphData.EnglishName = normalizedName
	modelData.Morphs.AppendRaw(morphData)
	return morphData
}

// vrm1MorphExpressionsSource は VRM1 expressions の最小構造を表す。
type vrm1MorphExpressionsSource struct {
	Expressions vrm1ExpressionsSource `json:"expressions"`
}

// vrm1ExpressionsSource は VRM1 preset/custom 表情セットを表す。
type vrm1ExpressionsSource struct {
	Preset map[string]vrm1ExpressionSource `json:"preset"`
	Custom map[string]vrm1ExpressionSource `json:"custom"`
}

// vrm1ExpressionSource は VRM1 expression の最小構造を表す。
type vrm1ExpressionSource struct {
	IsBinary              bool                            `json:"isBinary"`
	MorphTargetBinds      []vrm1ExpressionMorphBindEntry  `json:"morphTargetBinds"`
	MaterialColorBinds    []vrm1MaterialColorBindEntry    `json:"materialColorBinds"`
	TextureTransformBinds []vrm1TextureTransformBindEntry `json:"textureTransformBinds"`
}

// vrm1ExpressionMorphBindEntry は VRM1 expression の morphTargetBind を表す。
type vrm1ExpressionMorphBindEntry struct {
	Node   int     `json:"node"`
	Index  int     `json:"index"`
	Weight float64 `json:"weight"`
}

// vrm1MaterialColorBindEntry は VRM1 expression の materialColorBind を表す。
type vrm1MaterialColorBindEntry struct {
	Material    int       `json:"material"`
	Type        string    `json:"type"`
	TargetValue []float64 `json:"targetValue"`
}

// vrm1TextureTransformBindEntry は VRM1 expression の textureTransformBind を表す。
type vrm1TextureTransformBindEntry struct {
	Material int       `json:"material"`
	Scale    []float64 `json:"scale"`
	Offset   []float64 `json:"offset"`
}

// applyVrm1ExpressionMorphs は VRM1 expressions を PMX モーフへ変換する。
func applyVrm1ExpressionMorphs(modelData *model.PmxModel, raw json.RawMessage, registry *targetMorphRegistry) {
	if modelData == nil || modelData.Morphs == nil || len(raw) == 0 || registry == nil {
		return
	}
	source := vrm1MorphExpressionsSource{}
	if err := json.Unmarshal(raw, &source); err != nil {
		logVrmWarn("VRM1表情定義の解析に失敗したため継続します: err=%s", err.Error())
		return
	}
	presetKeys := sortedStringKeys(source.Expressions.Preset)
	for _, key := range presetKeys {
		entry := source.Expressions.Preset[key]
		expressionName := resolveVrm1PresetExpressionName(key)
		if expressionName == "" {
			continue
		}
		upsertExpressionMorphFromVrm1Entry(
			modelData,
			expressionName,
			resolveExpressionPanel(expressionName),
			entry,
			registry,
		)
	}
	customKeys := sortedStringKeys(source.Expressions.Custom)
	for _, key := range customKeys {
		entry := source.Expressions.Custom[key]
		expressionName := resolveCanonicalExpressionName(key)
		if expressionName == "" {
			continue
		}
		upsertExpressionMorphFromVrm1Entry(
			modelData,
			expressionName,
			resolveExpressionPanel(expressionName),
			entry,
			registry,
		)
	}
}

// upsertExpressionMorphFromVrm1Entry は VRM1 expression 要素を PMX モーフへ反映する。
func upsertExpressionMorphFromVrm1Entry(
	modelData *model.PmxModel,
	expressionName string,
	panel model.MorphPanel,
	entry vrm1ExpressionSource,
	registry *targetMorphRegistry,
) {
	if modelData == nil || modelData.Morphs == nil || strings.TrimSpace(expressionName) == "" || registry == nil {
		return
	}
	weightsByMorph := buildWeightsByMorphFromNodeBinds(entry.MorphTargetBinds, registry, entry.IsBinary)
	vertexOffsets := buildVertexOffsetsFromWeights(modelData, weightsByMorph)
	materialOffsets := buildMaterialOffsetsFromVrm1ColorBinds(modelData, entry.MaterialColorBinds, registry)
	uvOffsets := buildUvOffsetsFromVrm1TextureTransformBinds(modelData, entry.TextureTransformBinds, registry)
	upsertExpressionMorph(modelData, expressionName, panel, vertexOffsets, materialOffsets, uvOffsets)
}

// vrm0BlendShapeSource は VRM0 blendShapeMaster の最小構造を表す。
type vrm0BlendShapeSource struct {
	BlendShapeMaster vrm0BlendShapeMasterSource `json:"blendShapeMaster"`
}

// vrm0BlendShapeMasterSource は VRM0 blendShapeGroups を表す。
type vrm0BlendShapeMasterSource struct {
	BlendShapeGroups []vrm0BlendShapeGroupSource `json:"blendShapeGroups"`
}

// vrm0BlendShapeGroupSource は VRM0 blendShapeGroups 要素を表す。
type vrm0BlendShapeGroupSource struct {
	Name           string                     `json:"name"`
	PresetName     string                     `json:"presetName"`
	Binds          []vrm0BlendShapeBindRaw    `json:"binds"`
	MaterialValues []vrm0MaterialValueBindRaw `json:"materialValues"`
}

// vrm0BlendShapeBindRaw は VRM0 bind 要素を表す。
type vrm0BlendShapeBindRaw struct {
	Mesh   int     `json:"mesh"`
	Index  int     `json:"index"`
	Weight float64 `json:"weight"`
}

// vrm0MaterialValueBindRaw は VRM0 materialValues 要素を表す。
type vrm0MaterialValueBindRaw struct {
	MaterialName string    `json:"materialName"`
	PropertyName string    `json:"propertyName"`
	TargetValue  []float64 `json:"targetValue"`
}

// applyVrm0BlendShapeMorphs は VRM0 blendShapeGroups を PMX モーフへ変換する。
func applyVrm0BlendShapeMorphs(modelData *model.PmxModel, raw json.RawMessage, registry *targetMorphRegistry) {
	if modelData == nil || modelData.Morphs == nil || len(raw) == 0 || registry == nil {
		return
	}
	source := vrm0BlendShapeSource{}
	if err := json.Unmarshal(raw, &source); err != nil {
		logVrmWarn("VRM0表情定義の解析に失敗したため継続します: err=%s", err.Error())
		return
	}
	for _, group := range source.BlendShapeMaster.BlendShapeGroups {
		expressionName := strings.TrimSpace(group.Name)
		if expressionName == "" {
			expressionName = strings.TrimSpace(group.PresetName)
		}
		expressionName = resolveCanonicalExpressionName(expressionName)
		if expressionName == "" {
			continue
		}
		upsertExpressionMorphFromVrm0Group(
			modelData,
			expressionName,
			resolveExpressionPanel(expressionName),
			group,
			registry,
		)
	}
}

// upsertExpressionMorphFromVrm0Group は VRM0 blendShapeGroup を PMX モーフへ反映する。
func upsertExpressionMorphFromVrm0Group(
	modelData *model.PmxModel,
	expressionName string,
	panel model.MorphPanel,
	group vrm0BlendShapeGroupSource,
	registry *targetMorphRegistry,
) {
	if modelData == nil || modelData.Morphs == nil || strings.TrimSpace(expressionName) == "" || registry == nil {
		return
	}
	weightsByMorph := buildWeightsByMorphFromMeshBinds(group.Binds, registry)
	vertexOffsets := buildVertexOffsetsFromWeights(modelData, weightsByMorph)
	materialOffsets := buildMaterialOffsetsFromVrm0MaterialValues(modelData, group.MaterialValues, registry)
	uvOffsets := buildUvOffsetsFromVrm0MaterialValues(modelData, group.MaterialValues, registry)
	upsertExpressionMorph(modelData, expressionName, panel, vertexOffsets, materialOffsets, uvOffsets)
}

// buildWeightsByMorphFromNodeBinds は VRM1 node/index bind を morph index 係数へ変換する。
func buildWeightsByMorphFromNodeBinds(
	binds []vrm1ExpressionMorphBindEntry,
	registry *targetMorphRegistry,
	isBinary bool,
) map[int]float64 {
	if registry == nil {
		return nil
	}
	weightsByMorph := map[int]float64{}
	for _, bind := range binds {
		morphIndex, exists := registry.ByNodeAndTarget[nodeTargetKey{
			NodeIndex:   bind.Node,
			TargetIndex: bind.Index,
		}]
		if !exists {
			continue
		}
		weightsByMorph[morphIndex] += normalizeMorphBindWeight(bind.Weight, isBinary)
	}
	return weightsByMorph
}

// buildWeightsByMorphFromMeshBinds は VRM0 mesh/index bind を morph index 係数へ変換する。
func buildWeightsByMorphFromMeshBinds(
	binds []vrm0BlendShapeBindRaw,
	registry *targetMorphRegistry,
) map[int]float64 {
	if registry == nil {
		return nil
	}
	weightsByMorph := map[int]float64{}
	for _, bind := range binds {
		morphIndex, exists := registry.ByMeshAndTarget[meshTargetKey{
			MeshIndex:   bind.Mesh,
			TargetIndex: bind.Index,
		}]
		if !exists {
			continue
		}
		weightsByMorph[morphIndex] += normalizeMorphBindWeight(bind.Weight, false)
	}
	return weightsByMorph
}

// buildVertexOffsetsFromWeights は morph index 係数から頂点オフセット一覧を生成する。
func buildVertexOffsetsFromWeights(
	modelData *model.PmxModel,
	weightsByMorph map[int]float64,
) []model.IMorphOffset {
	if modelData == nil || modelData.Morphs == nil {
		return nil
	}
	if len(weightsByMorph) == 0 {
		return nil
	}
	morphIndexes := make([]int, 0, len(weightsByMorph))
	for morphIndex := range weightsByMorph {
		morphIndexes = append(morphIndexes, morphIndex)
	}
	sort.Ints(morphIndexes)
	offsetsByVertex := map[int]mmath.Vec3{}
	for _, morphIndex := range morphIndexes {
		weight := weightsByMorph[morphIndex]
		if weight <= 0 {
			continue
		}
		sourceMorph, err := modelData.Morphs.Get(morphIndex)
		if err != nil || sourceMorph == nil || sourceMorph.MorphType != model.MORPH_TYPE_VERTEX {
			continue
		}
		appendWeightedVertexOffsets(offsetsByVertex, sourceMorph.Offsets, weight)
	}
	return buildMergedVertexOffsets(offsetsByVertex)
}

// upsertExpressionMorph は表情名のモーフを頂点/材質オフセット構成で更新する。
func upsertExpressionMorph(
	modelData *model.PmxModel,
	expressionName string,
	panel model.MorphPanel,
	vertexOffsets []model.IMorphOffset,
	materialOffsets []model.IMorphOffset,
	uvOffsets []model.IMorphOffset,
) {
	if modelData == nil || modelData.Morphs == nil {
		return
	}
	expressionName = strings.TrimSpace(expressionName)
	if expressionName == "" {
		return
	}
	hasVertex := len(vertexOffsets) > 0
	hasMaterial := len(materialOffsets) > 0
	hasUv := len(uvOffsets) > 0
	componentCount := 0
	if hasVertex {
		componentCount++
	}
	if hasMaterial {
		componentCount++
	}
	if hasUv {
		componentCount++
	}
	if componentCount == 0 {
		return
	}
	if componentCount == 1 && hasVertex {
		upsertTypedExpressionMorph(modelData, expressionName, panel, model.MORPH_TYPE_VERTEX, vertexOffsets, false)
		return
	}
	if componentCount == 1 && hasMaterial {
		upsertTypedExpressionMorph(modelData, expressionName, panel, model.MORPH_TYPE_MATERIAL, materialOffsets, false)
		return
	}
	if componentCount == 1 && hasUv {
		upsertTypedExpressionMorph(
			modelData,
			expressionName,
			panel,
			model.MORPH_TYPE_EXTENDED_UV1,
			uvOffsets,
			false,
		)
		return
	}
	vertexMorphName := expressionName + "__vertex"
	materialMorphName := expressionName + "__material"
	uvMorphName := expressionName + "__uv1"
	groupOffsets := []model.IMorphOffset{}
	if hasVertex {
		vertexMorph := upsertTypedExpressionMorph(
			modelData,
			vertexMorphName,
			model.MORPH_PANEL_SYSTEM,
			model.MORPH_TYPE_VERTEX,
			vertexOffsets,
			false,
		)
		if vertexMorph != nil {
			groupOffsets = append(groupOffsets, &model.GroupMorphOffset{MorphIndex: vertexMorph.Index(), MorphFactor: 1.0})
		}
	}
	if hasMaterial {
		materialMorph := upsertTypedExpressionMorph(
			modelData,
			materialMorphName,
			model.MORPH_PANEL_SYSTEM,
			model.MORPH_TYPE_MATERIAL,
			materialOffsets,
			false,
		)
		if materialMorph != nil {
			groupOffsets = append(groupOffsets, &model.GroupMorphOffset{MorphIndex: materialMorph.Index(), MorphFactor: 1.0})
		}
	}
	if hasUv {
		uvMorph := upsertTypedExpressionMorph(
			modelData,
			uvMorphName,
			model.MORPH_PANEL_SYSTEM,
			model.MORPH_TYPE_EXTENDED_UV1,
			uvOffsets,
			false,
		)
		if uvMorph != nil {
			groupOffsets = append(groupOffsets, &model.GroupMorphOffset{MorphIndex: uvMorph.Index(), MorphFactor: 1.0})
		}
	}
	if len(groupOffsets) == 0 {
		return
	}
	upsertTypedExpressionMorph(modelData, expressionName, panel, model.MORPH_TYPE_GROUP, groupOffsets, false)
}

// upsertTypedExpressionMorph は指定型のモーフを作成または更新して返す。
func upsertTypedExpressionMorph(
	modelData *model.PmxModel,
	morphName string,
	panel model.MorphPanel,
	morphType model.MorphType,
	offsets []model.IMorphOffset,
	isSystem bool,
) *model.Morph {
	if modelData == nil || modelData.Morphs == nil {
		return nil
	}
	morphName = strings.TrimSpace(morphName)
	if morphName == "" || len(offsets) == 0 {
		return nil
	}
	existing, err := modelData.Morphs.GetByName(morphName)
	if err == nil && existing != nil {
		existing.Panel = panel
		existing.EnglishName = morphName
		existing.MorphType = morphType
		existing.Offsets = offsets
		existing.IsSystem = isSystem
		return existing
	}
	morphData := &model.Morph{
		Panel:     panel,
		MorphType: morphType,
		Offsets:   offsets,
		IsSystem:  isSystem,
	}
	morphData.SetName(morphName)
	morphData.EnglishName = morphName
	modelData.Morphs.AppendRaw(morphData)
	return morphData
}

// buildMaterialOffsetsFromVrm1ColorBinds は VRM1 materialColorBinds を材質モーフへ変換する。
func buildMaterialOffsetsFromVrm1ColorBinds(
	modelData *model.PmxModel,
	binds []vrm1MaterialColorBindEntry,
	registry *targetMorphRegistry,
) []model.IMorphOffset {
	if modelData == nil || modelData.Materials == nil || registry == nil || len(binds) == 0 {
		return nil
	}
	offsetsByMaterial := map[int]*model.MaterialMorphOffset{}
	for _, bind := range binds {
		materialIndexes := findMaterialIndexesByGltfIndex(registry, bind.Material)
		if len(materialIndexes) == 0 {
			continue
		}
		for _, materialIndex := range materialIndexes {
			baseMaterial, err := modelData.Materials.Get(materialIndex)
			if err != nil || baseMaterial == nil {
				continue
			}
			offsetData, exists := offsetsByMaterial[materialIndex]
			if !exists {
				offsetData = newMaterialMorphOffset(materialIndex)
				offsetsByMaterial[materialIndex] = offsetData
			}
			appendMaterialOffsetFromVrm1ColorBind(offsetData, baseMaterial, bind)
		}
	}
	return buildSortedMaterialOffsets(offsetsByMaterial)
}

// buildMaterialOffsetsFromVrm0MaterialValues は VRM0 materialValues を材質モーフへ変換する。
func buildMaterialOffsetsFromVrm0MaterialValues(
	modelData *model.PmxModel,
	values []vrm0MaterialValueBindRaw,
	registry *targetMorphRegistry,
) []model.IMorphOffset {
	if modelData == nil || modelData.Materials == nil || registry == nil || len(values) == 0 {
		return nil
	}
	offsetsByMaterial := map[int]*model.MaterialMorphOffset{}
	for _, value := range values {
		materialName := strings.TrimSpace(value.MaterialName)
		if materialName == "" {
			continue
		}
		materialIndexes := findMaterialIndexesByName(registry, materialName)
		if len(materialIndexes) == 0 {
			continue
		}
		for _, materialIndex := range materialIndexes {
			baseMaterial, err := modelData.Materials.Get(materialIndex)
			if err != nil || baseMaterial == nil {
				continue
			}
			offsetData, exists := offsetsByMaterial[materialIndex]
			if !exists {
				offsetData = newMaterialMorphOffset(materialIndex)
				offsetsByMaterial[materialIndex] = offsetData
			}
			appendMaterialOffsetFromVrm0MaterialValue(offsetData, baseMaterial, value)
		}
	}
	return buildSortedMaterialOffsets(offsetsByMaterial)
}

// buildUvOffsetsFromVrm1TextureTransformBinds は VRM1 textureTransformBinds をUV1モーフへ変換する。
func buildUvOffsetsFromVrm1TextureTransformBinds(
	modelData *model.PmxModel,
	binds []vrm1TextureTransformBindEntry,
	registry *targetMorphRegistry,
) []model.IMorphOffset {
	if modelData == nil || modelData.Vertices == nil || registry == nil || len(binds) == 0 {
		return nil
	}
	materialVertexMap := buildMaterialVertexIndexMap(modelData)
	offsetsByVertex := map[int]mmath.Vec4{}
	for _, bind := range binds {
		materialIndexes := findMaterialIndexesByGltfIndex(registry, bind.Material)
		if len(materialIndexes) == 0 {
			continue
		}
		scale := toVec2WithDefault(bind.Scale, mmath.Vec2{X: 1.0, Y: 1.0})
		offset := toVec2WithDefault(bind.Offset, mmath.ZERO_VEC2)
		appendUvOffsetsByMaterials(modelData, materialIndexes, materialVertexMap, scale, offset, offsetsByVertex)
	}
	return buildSortedUv1Offsets(modelData, offsetsByVertex)
}

// buildUvOffsetsFromVrm0MaterialValues は VRM0 materialValues のUV変換をUV1モーフへ変換する。
func buildUvOffsetsFromVrm0MaterialValues(
	modelData *model.PmxModel,
	values []vrm0MaterialValueBindRaw,
	registry *targetMorphRegistry,
) []model.IMorphOffset {
	if modelData == nil || modelData.Vertices == nil || registry == nil || len(values) == 0 {
		return nil
	}
	materialVertexMap := buildMaterialVertexIndexMap(modelData)
	offsetsByVertex := map[int]mmath.Vec4{}
	for _, value := range values {
		propertyName := strings.ToLower(strings.TrimSpace(value.PropertyName))
		if propertyName != "_maintex_st" && propertyName != "maintex_st" {
			continue
		}
		materialIndexes := findMaterialIndexesByName(registry, value.MaterialName)
		if len(materialIndexes) == 0 {
			continue
		}
		scale, offset := resolveMainTexTransform(value.TargetValue)
		appendUvOffsetsByMaterials(modelData, materialIndexes, materialVertexMap, scale, offset, offsetsByVertex)
	}
	return buildSortedUv1Offsets(modelData, offsetsByVertex)
}

// appendUvOffsetsByMaterials は材質集合に対応する頂点へUVオフセットを加算する。
func appendUvOffsetsByMaterials(
	modelData *model.PmxModel,
	materialIndexes []int,
	materialVertexMap map[int][]int,
	scale mmath.Vec2,
	offset mmath.Vec2,
	offsetsByVertex map[int]mmath.Vec4,
) {
	if modelData == nil || modelData.Vertices == nil || len(materialIndexes) == 0 || len(materialVertexMap) == 0 {
		return
	}
	for _, materialIndex := range materialIndexes {
		vertexIndexes := materialVertexMap[materialIndex]
		if len(vertexIndexes) == 0 {
			continue
		}
		for _, vertexIndex := range vertexIndexes {
			vertex, err := modelData.Vertices.Get(vertexIndex)
			if err != nil || vertex == nil {
				continue
			}
			du := vertex.Uv.X*(scale.X-1.0) + offset.X
			dv := vertex.Uv.Y*(scale.Y-1.0) + offset.Y
			if isZeroUvDelta(du, dv) {
				continue
			}
			delta := mmath.Vec4{X: du, Y: dv}
			current, exists := offsetsByVertex[vertexIndex]
			if !exists {
				offsetsByVertex[vertexIndex] = delta
				continue
			}
			offsetsByVertex[vertexIndex] = current.Added(delta)
		}
	}
}

// buildSortedUv1Offsets は頂点index順でUV1モーフ差分を返す。
func buildSortedUv1Offsets(
	modelData *model.PmxModel,
	offsetsByVertex map[int]mmath.Vec4,
) []model.IMorphOffset {
	if modelData == nil || modelData.Vertices == nil || len(offsetsByVertex) == 0 {
		return nil
	}
	vertexIndexes := make([]int, 0, len(offsetsByVertex))
	for vertexIndex, delta := range offsetsByVertex {
		if isZeroUvDelta(delta.X, delta.Y) {
			continue
		}
		vertexIndexes = append(vertexIndexes, vertexIndex)
	}
	if len(vertexIndexes) == 0 {
		return nil
	}
	sort.Ints(vertexIndexes)
	offsets := make([]model.IMorphOffset, 0, len(vertexIndexes))
	for _, vertexIndex := range vertexIndexes {
		delta := offsetsByVertex[vertexIndex]
		if isZeroUvDelta(delta.X, delta.Y) {
			continue
		}
		ensureVertexExtendedUv1(modelData, vertexIndex)
		offsets = append(offsets, &model.UvMorphOffset{
			VertexIndex: vertexIndex,
			Uv:          delta,
			UvType:      model.MORPH_TYPE_EXTENDED_UV1,
		})
	}
	return offsets
}

// buildMaterialVertexIndexMap は材質indexごとの頂点index一覧を構築する。
func buildMaterialVertexIndexMap(modelData *model.PmxModel) map[int][]int {
	if modelData == nil || modelData.Vertices == nil {
		return map[int][]int{}
	}
	vertexIndexesByMaterial := map[int][]int{}
	for _, vertex := range modelData.Vertices.Values() {
		if vertex == nil || len(vertex.MaterialIndexes) == 0 {
			continue
		}
		vertexIndex := vertex.Index()
		for _, materialIndex := range vertex.MaterialIndexes {
			if materialIndex < 0 {
				continue
			}
			vertexIndexesByMaterial[materialIndex] = appendUniqueInt(vertexIndexesByMaterial[materialIndex], vertexIndex)
		}
	}
	for materialIndex := range vertexIndexesByMaterial {
		sort.Ints(vertexIndexesByMaterial[materialIndex])
	}
	return vertexIndexesByMaterial
}

// ensureVertexExtendedUv1 は対象頂点に拡張UV1スロットを確保する。
func ensureVertexExtendedUv1(modelData *model.PmxModel, vertexIndex int) {
	if modelData == nil || modelData.Vertices == nil || vertexIndex < 0 {
		return
	}
	vertex, err := modelData.Vertices.Get(vertexIndex)
	if err != nil || vertex == nil {
		return
	}
	if len(vertex.ExtendedUvs) >= 1 {
		return
	}
	vertex.ExtendedUvs = append(vertex.ExtendedUvs, mmath.ZERO_VEC4)
}

// resolveMainTexTransform は materialValues の MainTex_ST から scale/offset を解決する。
func resolveMainTexTransform(values []float64) (mmath.Vec2, mmath.Vec2) {
	scale := mmath.Vec2{X: 1.0, Y: 1.0}
	offset := mmath.ZERO_VEC2
	if len(values) > 0 {
		scale.X = values[0]
	}
	if len(values) > 1 {
		scale.Y = values[1]
	}
	if len(values) > 2 {
		offset.X = values[2]
	}
	if len(values) > 3 {
		offset.Y = values[3]
	}
	return scale, offset
}

// isZeroUvDelta はUV差分が実質ゼロか判定する。
func isZeroUvDelta(du float64, dv float64) bool {
	const epsilon = 1e-9
	return math.Abs(du) <= epsilon && math.Abs(dv) <= epsilon
}

// appendMaterialOffsetFromVrm1ColorBind は VRM1 color bind を PMX 材質モーフ差分へ加算する。
func appendMaterialOffsetFromVrm1ColorBind(
	offsetData *model.MaterialMorphOffset,
	baseMaterial *model.Material,
	bind vrm1MaterialColorBindEntry,
) {
	if offsetData == nil || baseMaterial == nil {
		return
	}
	targetType := strings.ToLower(strings.TrimSpace(bind.Type))
	switch targetType {
	case "color":
		target := toVec4WithDefault(bind.TargetValue, baseMaterial.Diffuse)
		offsetData.Diffuse = offsetData.Diffuse.Added(target.Subed(baseMaterial.Diffuse))
	case "shadecolor":
		target := toVec3WithDefault(bind.TargetValue, baseMaterial.Ambient)
		offsetData.Ambient = offsetData.Ambient.Added(target.Subed(baseMaterial.Ambient))
	case "emissioncolor":
		target := toVec3WithDefault(bind.TargetValue, mmath.ZERO_VEC3)
		offsetData.Specular = offsetData.Specular.Added(mmath.Vec4{
			X: target.X - baseMaterial.Specular.X,
			Y: target.Y - baseMaterial.Specular.Y,
			Z: target.Z - baseMaterial.Specular.Z,
			W: 0.0,
		})
	case "outlinecolor", "rimcolor":
		target := toVec4WithDefault(bind.TargetValue, baseMaterial.Edge)
		offsetData.Edge = offsetData.Edge.Added(target.Subed(baseMaterial.Edge))
	case "matcapcolor":
		target := toVec4WithDefault(bind.TargetValue, baseMaterial.SphereTextureFactor)
		offsetData.SphereTextureFactor = offsetData.SphereTextureFactor.Added(
			target.Subed(baseMaterial.SphereTextureFactor),
		)
	default:
	}
}

// appendMaterialOffsetFromVrm0MaterialValue は VRM0 materialValue を PMX 材質モーフ差分へ加算する。
func appendMaterialOffsetFromVrm0MaterialValue(
	offsetData *model.MaterialMorphOffset,
	baseMaterial *model.Material,
	value vrm0MaterialValueBindRaw,
) {
	if offsetData == nil || baseMaterial == nil {
		return
	}
	propertyName := strings.ToLower(strings.TrimSpace(value.PropertyName))
	switch propertyName {
	case "_color", "color":
		target := toVec4WithDefault(value.TargetValue, baseMaterial.Diffuse)
		offsetData.Diffuse = offsetData.Diffuse.Added(target.Subed(baseMaterial.Diffuse))
	case "_shadecolor", "shadecolor":
		target := toVec3WithDefault(value.TargetValue, baseMaterial.Ambient)
		offsetData.Ambient = offsetData.Ambient.Added(target.Subed(baseMaterial.Ambient))
	case "_emissioncolor", "emissioncolor":
		target := toVec3WithDefault(value.TargetValue, mmath.ZERO_VEC3)
		offsetData.Specular = offsetData.Specular.Added(mmath.Vec4{
			X: target.X - baseMaterial.Specular.X,
			Y: target.Y - baseMaterial.Specular.Y,
			Z: target.Z - baseMaterial.Specular.Z,
			W: 0.0,
		})
	case "_outlinecolor", "outlinecolor", "_rimcolor", "rimcolor":
		target := toVec4WithDefault(value.TargetValue, baseMaterial.Edge)
		offsetData.Edge = offsetData.Edge.Added(target.Subed(baseMaterial.Edge))
	case "_outlinewidth", "outlinewidth":
		target := toFloatWithDefault(value.TargetValue, baseMaterial.EdgeSize)
		offsetData.EdgeSize += target - baseMaterial.EdgeSize
	default:
	}
}

// newMaterialMorphOffset は材質モーフ差分の初期値を返す。
func newMaterialMorphOffset(materialIndex int) *model.MaterialMorphOffset {
	return &model.MaterialMorphOffset{
		MaterialIndex:       materialIndex,
		CalcMode:            model.CALC_MODE_ADDITION,
		Diffuse:             mmath.ZERO_VEC4,
		Specular:            mmath.ZERO_VEC4,
		Ambient:             mmath.ZERO_VEC3,
		Edge:                mmath.ZERO_VEC4,
		EdgeSize:            0.0,
		TextureFactor:       mmath.ZERO_VEC4,
		SphereTextureFactor: mmath.ZERO_VEC4,
		ToonTextureFactor:   mmath.ZERO_VEC4,
	}
}

// findMaterialIndexesByGltfIndex は glTF 材質indexに対応する PMX 材質index一覧を返す。
func findMaterialIndexesByGltfIndex(registry *targetMorphRegistry, gltfMaterialIndex int) []int {
	if registry == nil || gltfMaterialIndex < 0 {
		return nil
	}
	indexes, exists := registry.ByGltfMaterial[gltfMaterialIndex]
	if !exists || len(indexes) == 0 {
		return nil
	}
	out := make([]int, 0, len(indexes))
	for _, materialIndex := range indexes {
		out = appendUniqueInt(out, materialIndex)
	}
	sort.Ints(out)
	return out
}

// findMaterialIndexesByName は材質名に対応する PMX 材質index一覧を返す。
func findMaterialIndexesByName(registry *targetMorphRegistry, materialName string) []int {
	if registry == nil {
		return nil
	}
	normalizedName := strings.TrimSpace(materialName)
	if normalizedName == "" {
		return nil
	}
	indexes, exists := registry.ByMaterialName[normalizedName]
	if !exists || len(indexes) == 0 {
		return nil
	}
	out := make([]int, 0, len(indexes))
	for _, materialIndex := range indexes {
		out = appendUniqueInt(out, materialIndex)
	}
	sort.Ints(out)
	return out
}

// buildSortedMaterialOffsets は材質index順で材質モーフ差分を返す。
func buildSortedMaterialOffsets(offsetsByMaterial map[int]*model.MaterialMorphOffset) []model.IMorphOffset {
	if len(offsetsByMaterial) == 0 {
		return nil
	}
	materialIndexes := make([]int, 0, len(offsetsByMaterial))
	for materialIndex, offsetData := range offsetsByMaterial {
		if offsetData == nil || isZeroMaterialMorphOffset(offsetData) {
			continue
		}
		materialIndexes = append(materialIndexes, materialIndex)
	}
	if len(materialIndexes) == 0 {
		return nil
	}
	sort.Ints(materialIndexes)
	offsets := make([]model.IMorphOffset, 0, len(materialIndexes))
	for _, materialIndex := range materialIndexes {
		offsetData := offsetsByMaterial[materialIndex]
		if offsetData == nil || isZeroMaterialMorphOffset(offsetData) {
			continue
		}
		offsets = append(offsets, offsetData)
	}
	return offsets
}

// isZeroMaterialMorphOffset は材質モーフ差分が実質ゼロか判定する。
func isZeroMaterialMorphOffset(offsetData *model.MaterialMorphOffset) bool {
	if offsetData == nil {
		return true
	}
	const epsilon = 1e-9
	if math.Abs(offsetData.EdgeSize) > epsilon {
		return false
	}
	if !offsetData.Diffuse.NearEquals(mmath.ZERO_VEC4, epsilon) {
		return false
	}
	if !offsetData.Specular.NearEquals(mmath.ZERO_VEC4, epsilon) {
		return false
	}
	if !offsetData.Ambient.NearEquals(mmath.ZERO_VEC3, epsilon) {
		return false
	}
	if !offsetData.Edge.NearEquals(mmath.ZERO_VEC4, epsilon) {
		return false
	}
	if !offsetData.TextureFactor.NearEquals(mmath.ZERO_VEC4, epsilon) {
		return false
	}
	if !offsetData.SphereTextureFactor.NearEquals(mmath.ZERO_VEC4, epsilon) {
		return false
	}
	if !offsetData.ToonTextureFactor.NearEquals(mmath.ZERO_VEC4, epsilon) {
		return false
	}
	return true
}

// appendSpecialEyeMaterialMorphsFromFallbackRules は特殊目用の材質追加と材質モーフ補完を実行する。
func appendSpecialEyeMaterialMorphsFromFallbackRules(
	modelData *model.PmxModel,
	doc *gltfDocument,
	registry *targetMorphRegistry,
) {
	if modelData == nil || modelData.Materials == nil || modelData.Faces == nil {
		return
	}
	augmentStats := appendSpecialEyeAugmentedMaterials(modelData, doc, registry)
	morphStats := appendSpecialEyeMaterialMorphFallbacks(modelData, doc, registry)
	logVrmInfo(
		"特殊目材質補完完了: augmentRules=%d augmentMaterials=%d augmentFaces=%d skippedNoBase=%d skippedNoTexture=%d morphRules=%d generated=%d skippedExisting=%d skippedNoTarget=%d",
		augmentStats.RuleCount,
		augmentStats.GeneratedMaterials,
		augmentStats.GeneratedFaces,
		augmentStats.SkippedNoBase,
		augmentStats.SkippedNoTexture,
		morphStats.RuleCount,
		morphStats.Generated,
		morphStats.SkippedExisting,
		morphStats.SkippedNoTarget,
	)
}

// appendSpecialEyeAugmentedMaterials は特殊目オーバーレイ材質を追加する。
func appendSpecialEyeAugmentedMaterials(
	modelData *model.PmxModel,
	doc *gltfDocument,
	registry *targetMorphRegistry,
) specialEyeMaterialAugmentStats {
	stats := specialEyeMaterialAugmentStats{RuleCount: len(specialEyeAugmentRules)}
	if modelData == nil || modelData.Materials == nil || modelData.Faces == nil {
		return stats
	}
	textureIndexByToken := resolveSpecialEyeTextureIndexByToken(modelData, specialEyeOverlayTextureTokens)
	materialInfos := collectSpecialEyeMaterialInfos(modelData, doc, registry)
	faceRanges := buildCreateMaterialFaceRanges(modelData)

	for _, rule := range specialEyeAugmentRules {
		baseMaterialIndexes := resolveSpecialEyeBaseMaterialIndexes(materialInfos, rule.EyeClass)
		if len(baseMaterialIndexes) == 0 {
			stats.SkippedNoBase++
			continue
		}
		normalizedToken := normalizeSpecialEyeToken(rule.TextureToken)
		textureIndex, exists := textureIndexByToken[normalizedToken]
		if !exists || textureIndex < 0 {
			stats.SkippedNoTexture++
			continue
		}
		for _, baseMaterialIndex := range baseMaterialIndexes {
			if baseMaterialIndex < 0 || baseMaterialIndex >= len(faceRanges) {
				continue
			}
			baseMaterial, err := modelData.Materials.Get(baseMaterialIndex)
			if err != nil || baseMaterial == nil {
				continue
			}
			faceRange := faceRanges[baseMaterialIndex]
			if faceRange.End <= faceRange.Start {
				continue
			}
			overlayMaterialName := resolveSpecialEyeAugmentedMaterialName(baseMaterial.Name(), rule.TextureToken)
			overlayMaterialName = ensureUniqueSpecialEyeMaterialName(modelData, overlayMaterialName)
			overlayMaterial := cloneSpecialEyeMaterial(baseMaterial, overlayMaterialName, textureIndex)
			overlayMaterialIndex := modelData.Materials.AppendRaw(overlayMaterial)
			registerExpressionMaterialIndex(registry, nil, nil, overlayMaterial.Name(), overlayMaterialIndex)
			copiedFaces := appendSpecialEyeFacesForMaterial(
				modelData,
				faceRange.Start,
				faceRange.End,
				overlayMaterialIndex,
			)
			overlayMaterial.VerticesCount = copiedFaces * 3
			if copiedFaces <= 0 {
				continue
			}
			stats.GeneratedMaterials++
			stats.GeneratedFaces += copiedFaces
		}
	}
	return stats
}

// appendSpecialEyeMaterialMorphFallbacks は特殊目材質モーフを補完生成する。
func appendSpecialEyeMaterialMorphFallbacks(
	modelData *model.PmxModel,
	doc *gltfDocument,
	registry *targetMorphRegistry,
) specialEyeMaterialMorphStats {
	stats := specialEyeMaterialMorphStats{RuleCount: len(specialEyeMaterialMorphFallbackRules)}
	if modelData == nil || modelData.Morphs == nil || modelData.Materials == nil {
		return stats
	}
	materialInfos := collectSpecialEyeMaterialInfos(modelData, doc, registry)
	hideMaterialIndexesByClass := resolveSpecialEyeHideMaterialIndexesByClass(materialInfos)

	for _, rule := range specialEyeMaterialMorphFallbackRules {
		existingMorph, err := modelData.Morphs.GetByName(rule.MorphName)
		if err == nil && existingMorph != nil && len(existingMorph.Offsets) > 0 {
			stats.SkippedExisting++
			continue
		}

		offsetsByMaterial := map[int]*model.MaterialMorphOffset{}
		targetMatchedCount := 0
		for _, materialInfo := range materialInfos {
			if resolveSpecialEyeTokenMatchLevel(materialInfo, rule.TextureToken) == 0 {
				continue
			}
			appendSpecialEyeShowOffset(modelData, offsetsByMaterial, materialInfo.MaterialIndex)
			targetMatchedCount++
		}
		if targetMatchedCount == 0 {
			if len(rule.HideClasses) == 0 {
				stats.SkippedNoTarget++
				if normalizeSpecialEyeToken(rule.TextureToken) == normalizeSpecialEyeToken("cheek_dye") {
					logVrmWarn(
						"特殊目材質モーフ生成スキップ: morph=%s reason=target_material_not_found token=%s",
						rule.MorphName,
						rule.TextureToken,
					)
				} else {
					logVrmDebug(
						"特殊目材質モーフ生成スキップ: morph=%s reason=target_material_not_found token=%s",
						rule.MorphName,
						rule.TextureToken,
					)
				}
				continue
			}
		}
		for _, hideClass := range rule.HideClasses {
			for _, materialIndex := range hideMaterialIndexesByClass[hideClass] {
				appendSpecialEyeHideOffset(modelData, offsetsByMaterial, materialIndex)
			}
		}

		offsets := buildSortedMaterialOffsets(offsetsByMaterial)
		if len(offsets) == 0 {
			stats.SkippedNoTarget++
			logVrmDebug(
				"特殊目材質モーフ生成スキップ: morph=%s reason=offset_not_generated token=%s",
				rule.MorphName,
				rule.TextureToken,
			)
			continue
		}
		upsertTypedExpressionMorph(
			modelData,
			rule.MorphName,
			rule.Panel,
			model.MORPH_TYPE_MATERIAL,
			offsets,
			false,
		)
		stats.Generated++
	}
	return stats
}

// appendExpressionEdgeFallbackMorph は旧仕様互換のエッジOFF材質モーフを補完する。
func appendExpressionEdgeFallbackMorph(modelData *model.PmxModel) {
	if modelData == nil || modelData.Morphs == nil || modelData.Materials == nil {
		return
	}
	offsets := buildExpressionEdgeFallbackOffsets(modelData)
	if len(offsets) == 0 {
		return
	}
	upsertTypedExpressionMorph(
		modelData,
		"エッジOFF",
		model.MORPH_PANEL_OTHER_LOWER_RIGHT,
		model.MORPH_TYPE_MATERIAL,
		offsets,
		false,
	)
	logVrmInfo("エッジOFF材質モーフ補完: offsets=%d", len(offsets))
}

// buildExpressionEdgeFallbackOffsets はエッジOFF材質モーフの材質オフセット一覧を返す。
func buildExpressionEdgeFallbackOffsets(modelData *model.PmxModel) []model.IMorphOffset {
	if modelData == nil || modelData.Materials == nil {
		return nil
	}
	offsets := make([]model.IMorphOffset, 0, modelData.Materials.Len())
	for materialIndex := 0; materialIndex < modelData.Materials.Len(); materialIndex++ {
		materialData, err := modelData.Materials.Get(materialIndex)
		if err != nil || materialData == nil {
			continue
		}
		if (materialData.DrawFlag & model.DRAW_FLAG_DRAWING_EDGE) != 0 {
			offsets = append(offsets, &model.MaterialMorphOffset{
				MaterialIndex:       materialIndex,
				CalcMode:            model.CALC_MODE_MULTIPLICATION,
				Diffuse:             mmath.ONE_VEC4,
				Specular:            mmath.ONE_VEC4,
				Ambient:             mmath.ONE_VEC3,
				Edge:                mmath.Vec4{X: 1.0, Y: 1.0, Z: 1.0, W: 0.0},
				EdgeSize:            0.0,
				TextureFactor:       mmath.ONE_VEC4,
				SphereTextureFactor: mmath.ONE_VEC4,
				ToonTextureFactor:   mmath.ONE_VEC4,
			})
			continue
		}
		if strings.HasSuffix(strings.TrimSpace(materialData.Name()), "_エッジ") {
			offsets = append(offsets, &model.MaterialMorphOffset{
				MaterialIndex:       materialIndex,
				CalcMode:            model.CALC_MODE_MULTIPLICATION,
				Diffuse:             mmath.ZERO_VEC4,
				Specular:            mmath.ZERO_VEC4,
				Ambient:             mmath.ZERO_VEC3,
				Edge:                mmath.ZERO_VEC4,
				EdgeSize:            0.0,
				TextureFactor:       mmath.ZERO_VEC4,
				SphereTextureFactor: mmath.ZERO_VEC4,
				ToonTextureFactor:   mmath.ZERO_VEC4,
			})
		}
	}
	if len(offsets) > 0 {
		return offsets
	}
	for materialIndex := 0; materialIndex < modelData.Materials.Len(); materialIndex++ {
		offsets = append(offsets, &model.MaterialMorphOffset{
			MaterialIndex:       materialIndex,
			CalcMode:            model.CALC_MODE_MULTIPLICATION,
			Diffuse:             mmath.ONE_VEC4,
			Specular:            mmath.ONE_VEC4,
			Ambient:             mmath.ONE_VEC3,
			Edge:                mmath.Vec4{X: 1.0, Y: 1.0, Z: 1.0, W: 0.0},
			EdgeSize:            0.0,
			TextureFactor:       mmath.ONE_VEC4,
			SphereTextureFactor: mmath.ONE_VEC4,
			ToonTextureFactor:   mmath.ONE_VEC4,
		})
	}
	return offsets
}

// collectSpecialEyeMaterialInfos は特殊目判定に必要な材質情報を収集する。
func collectSpecialEyeMaterialInfos(
	modelData *model.PmxModel,
	doc *gltfDocument,
	registry *targetMorphRegistry,
) []specialEyeMaterialInfo {
	if modelData == nil || modelData.Materials == nil {
		return nil
	}
	pmxToGltfMaterialIndexes := buildPmxMaterialToGltfIndexes(registry)
	materialInfos := make([]specialEyeMaterialInfo, 0, modelData.Materials.Len())
	for materialIndex := 0; materialIndex < modelData.Materials.Len(); materialIndex++ {
		materialData, err := modelData.Materials.Get(materialIndex)
		if err != nil || materialData == nil {
			continue
		}
		materialName := strings.TrimSpace(materialData.Name())
		materialEnglishName := strings.TrimSpace(materialData.EnglishName)
		textureName := resolveSpecialEyeTextureName(modelData, materialData.TextureIndex)
		textureURI := resolveSpecialEyeTextureURI(doc, pmxToGltfMaterialIndexes[materialIndex])
		classificationSource := strings.TrimSpace(
			materialName + " " + materialEnglishName + " " + textureName + " " + textureURI,
		)
		tags := classifyCreateSemanticTags(classificationSource)
		classes := map[string]struct{}{}
		if containsCreateSemantic(tags, createSemanticIris) {
			classes[specialEyeClassIris] = struct{}{}
		}
		if containsCreateSemantic(tags, createSemanticEyeWhite) {
			classes[specialEyeClassWhite] = struct{}{}
		}
		if containsCreateSemantic(tags, createSemanticEyeLine) {
			classes[specialEyeClassEyeLine] = struct{}{}
		}
		if containsCreateSemantic(tags, createSemanticEyeLash) {
			classes[specialEyeClassEyeLash] = struct{}{}
		}
		if containsCreateSemantic(tags, createSemanticFace) && containsCreateSemantic(tags, createSemanticSkin) {
			classes[specialEyeClassFace] = struct{}{}
		}
		if hasLegacyFaceMaterialEnglishName(materialEnglishName) {
			classes[specialEyeClassFace] = struct{}{}
		}
		appendSpecialEyeClassByLocalizedFallback(classes, materialName, materialEnglishName, textureName, textureURI)
		hasEyeClass := hasSpecialEyeClass(classes, specialEyeClassIris) || hasSpecialEyeClass(classes, specialEyeClassWhite)
		if !hasEyeClass {
			normalizedNames := normalizeCreateSemanticName(strings.TrimSpace(materialName + " " + materialEnglishName))
			fallbackApplied := false
			if strings.Contains(normalizedNames, "eyeiris") {
				classes[specialEyeClassIris] = struct{}{}
				fallbackApplied = true
			}
			if strings.Contains(normalizedNames, "eyewhite") {
				classes[specialEyeClassWhite] = struct{}{}
				fallbackApplied = true
			}
			if fallbackApplied {
				logVrmWarn("特殊目材質分類フォールバック: material=%s", materialName)
			}
		}
		materialInfo := specialEyeMaterialInfo{
			MaterialIndex:          materialIndex,
			Name:                   materialName,
			EnglishName:            materialEnglishName,
			TextureName:            textureName,
			TextureURI:             textureURI,
			NormalizedName:         normalizeCreateSemanticName(materialName),
			NormalizedEnglishName:  normalizeCreateSemanticName(materialEnglishName),
			NormalizedTextureMatch: normalizeCreateSemanticName(strings.TrimSpace(textureName + " " + textureURI)),
			Classes:                classes,
			IsOverlay:              false,
		}
		for _, token := range specialEyeOverlayTextureTokens {
			if resolveSpecialEyeTokenMatchLevel(materialInfo, token) > 0 {
				materialInfo.IsOverlay = true
				break
			}
		}
		materialInfos = append(materialInfos, materialInfo)
	}
	return materialInfos
}

// buildPmxMaterialToGltfIndexes は PMX材質index から glTF材質index一覧を返す。
func buildPmxMaterialToGltfIndexes(registry *targetMorphRegistry) map[int][]int {
	pmxToGltfMaterialIndexes := map[int][]int{}
	if registry == nil {
		return pmxToGltfMaterialIndexes
	}
	for gltfMaterialIndex, pmxMaterialIndexes := range registry.ByGltfMaterial {
		for _, pmxMaterialIndex := range pmxMaterialIndexes {
			pmxToGltfMaterialIndexes[pmxMaterialIndex] = appendUniqueInt(
				pmxToGltfMaterialIndexes[pmxMaterialIndex],
				gltfMaterialIndex,
			)
		}
	}
	for materialIndex := range pmxToGltfMaterialIndexes {
		sort.Ints(pmxToGltfMaterialIndexes[materialIndex])
	}
	return pmxToGltfMaterialIndexes
}

// resolveSpecialEyeTextureURI は glTF材質情報からテクスチャURI候補を返す。
func resolveSpecialEyeTextureURI(doc *gltfDocument, gltfMaterialIndexes []int) string {
	if doc == nil || len(gltfMaterialIndexes) == 0 {
		return ""
	}
	for _, gltfMaterialIndex := range gltfMaterialIndexes {
		if gltfMaterialIndex < 0 || gltfMaterialIndex >= len(doc.Materials) {
			continue
		}
		baseTexture := doc.Materials[gltfMaterialIndex].PbrMetallicRoughness.BaseColorTexture
		if baseTexture == nil || baseTexture.Index < 0 || baseTexture.Index >= len(doc.Textures) {
			continue
		}
		texture := doc.Textures[baseTexture.Index]
		if texture.Source == nil || *texture.Source < 0 || *texture.Source >= len(doc.Images) {
			continue
		}
		image := doc.Images[*texture.Source]
		if strings.TrimSpace(image.URI) != "" {
			return strings.TrimSpace(image.URI)
		}
		if strings.TrimSpace(image.Name) != "" {
			return strings.TrimSpace(image.Name)
		}
	}
	return ""
}

// resolveSpecialEyeTextureName は PMX材質のテクスチャ名を返す。
func resolveSpecialEyeTextureName(modelData *model.PmxModel, textureIndex int) string {
	if modelData == nil || modelData.Textures == nil || textureIndex < 0 {
		return ""
	}
	textureData, err := modelData.Textures.Get(textureIndex)
	if err != nil || textureData == nil {
		return ""
	}
	return strings.TrimSpace(textureData.Name())
}

// resolveSpecialEyeTextureIndexByToken は特殊目トークンに対応するテクスチャindexを返す。
func resolveSpecialEyeTextureIndexByToken(modelData *model.PmxModel, tokens []string) map[string]int {
	textureIndexByToken := map[string]int{}
	for _, token := range tokens {
		normalizedToken := normalizeSpecialEyeToken(token)
		if normalizedToken == "" {
			continue
		}
		if _, exists := textureIndexByToken[normalizedToken]; exists {
			continue
		}
		textureIndex := findSpecialEyeTextureIndexByToken(modelData, normalizedToken)
		if textureIndex >= 0 {
			textureIndexByToken[normalizedToken] = textureIndex
		}
	}
	return textureIndexByToken
}

// findSpecialEyeTextureIndexByToken はテクスチャ名からトークン一致するindexを返す。
func findSpecialEyeTextureIndexByToken(modelData *model.PmxModel, normalizedToken string) int {
	if modelData == nil || modelData.Textures == nil || normalizedToken == "" {
		return -1
	}
	for textureIndex := 0; textureIndex < modelData.Textures.Len(); textureIndex++ {
		textureData, err := modelData.Textures.Get(textureIndex)
		if err != nil || textureData == nil {
			continue
		}
		normalizedTextureName := normalizeCreateSemanticName(
			strings.TrimSpace(textureData.Name() + " " + textureData.EnglishName),
		)
		if strings.Contains(normalizedTextureName, normalizedToken) {
			return textureIndex
		}
	}
	return -1
}

// resolveSpecialEyeBaseMaterialIndexes は指定EyeClassのベース材質index一覧を返す。
func resolveSpecialEyeBaseMaterialIndexes(materialInfos []specialEyeMaterialInfo, eyeClass string) []int {
	materialIndexes := make([]int, 0, len(materialInfos))
	for _, materialInfo := range materialInfos {
		if materialInfo.IsOverlay {
			continue
		}
		if !hasSpecialEyeClass(materialInfo.Classes, eyeClass) {
			continue
		}
		materialIndexes = append(materialIndexes, materialInfo.MaterialIndex)
	}
	sort.Ints(materialIndexes)
	return materialIndexes
}

// resolveSpecialEyeHideMaterialIndexesByClass は非表示対象材質indexをEyeClassごとに返す。
func resolveSpecialEyeHideMaterialIndexesByClass(materialInfos []specialEyeMaterialInfo) map[string][]int {
	materialIndexesByClass := map[string][]int{}
	for _, materialInfo := range materialInfos {
		if materialInfo.IsOverlay {
			continue
		}
		for className := range materialInfo.Classes {
			switch className {
			case specialEyeClassWhite, specialEyeClassEyeLine, specialEyeClassEyeLash:
				materialIndexesByClass[className] = appendUniqueInt(
					materialIndexesByClass[className],
					materialInfo.MaterialIndex,
				)
			}
		}
	}
	for className := range materialIndexesByClass {
		sort.Ints(materialIndexesByClass[className])
	}
	return materialIndexesByClass
}

// resolveSpecialEyeAugmentedMaterialName はベース材質名とトークンから追加材質名を返す。
func resolveSpecialEyeAugmentedMaterialName(baseMaterialName string, token string) string {
	trimmedBaseName := trimSpecialEyeMaterialBaseName(baseMaterialName)
	if trimmedBaseName == "" {
		trimmedBaseName = "special_eye"
	}
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return trimmedBaseName
	}
	return fmt.Sprintf("%s_%s", trimmedBaseName, trimmedToken)
}

// trimSpecialEyeMaterialBaseName は特殊目追加材質名のベース名から不要な接尾辞を除去する。
func trimSpecialEyeMaterialBaseName(baseMaterialName string) string {
	trimmedName := strings.TrimSpace(baseMaterialName)
	if trimmedName == "" {
		return ""
	}
	loweredName := strings.ToLower(trimmedName)
	if strings.HasSuffix(loweredName, " (instance)") {
		return strings.TrimSpace(trimmedName[:len(trimmedName)-len(" (Instance)")])
	}
	if strings.HasSuffix(loweredName, "(instance)") {
		return strings.TrimSpace(trimmedName[:len(trimmedName)-len("(Instance)")])
	}
	return trimmedName
}

// ensureUniqueSpecialEyeMaterialName は同名衝突を回避した材質名を返す。
func ensureUniqueSpecialEyeMaterialName(modelData *model.PmxModel, baseName string) string {
	if modelData == nil || modelData.Materials == nil {
		return strings.TrimSpace(baseName)
	}
	trimmedBaseName := strings.TrimSpace(baseName)
	if trimmedBaseName == "" {
		trimmedBaseName = "special_eye"
	}
	if _, err := modelData.Materials.GetByName(trimmedBaseName); err != nil {
		return trimmedBaseName
	}
	for suffixNo := 1; ; suffixNo++ {
		candidateName := fmt.Sprintf("%s_%03d", trimmedBaseName, suffixNo)
		if _, err := modelData.Materials.GetByName(candidateName); err != nil {
			return candidateName
		}
	}
}

// cloneSpecialEyeMaterial は特殊目用の追加材質を複製生成する。
func cloneSpecialEyeMaterial(baseMaterial *model.Material, materialName string, textureIndex int) *model.Material {
	clonedMaterial := model.NewMaterial()
	if baseMaterial != nil {
		clonedMaterial.Memo = baseMaterial.Memo
		clonedMaterial.Diffuse = baseMaterial.Diffuse
		clonedMaterial.Specular = baseMaterial.Specular
		clonedMaterial.Ambient = baseMaterial.Ambient
		clonedMaterial.DrawFlag = baseMaterial.DrawFlag
		clonedMaterial.Edge = baseMaterial.Edge
		clonedMaterial.EdgeSize = baseMaterial.EdgeSize
		clonedMaterial.TextureFactor = baseMaterial.TextureFactor
		clonedMaterial.SphereTextureFactor = baseMaterial.SphereTextureFactor
		clonedMaterial.ToonTextureFactor = baseMaterial.ToonTextureFactor
		clonedMaterial.SphereTextureIndex = baseMaterial.SphereTextureIndex
		clonedMaterial.SphereMode = baseMaterial.SphereMode
		clonedMaterial.ToonSharingFlag = baseMaterial.ToonSharingFlag
		clonedMaterial.ToonTextureIndex = baseMaterial.ToonTextureIndex
	}
	clonedMaterial.SetName(materialName)
	clonedMaterial.EnglishName = materialName
	clonedMaterial.TextureIndex = textureIndex
	clonedMaterial.Diffuse.W = 0.0
	clonedMaterial.DrawFlag &^= model.DRAW_FLAG_DRAWING_EDGE
	clonedMaterial.VerticesCount = 0
	return clonedMaterial
}

// appendSpecialEyeFacesForMaterial は対象面範囲を複製して材質へ割り当てる。
func appendSpecialEyeFacesForMaterial(
	modelData *model.PmxModel,
	faceStart int,
	faceEnd int,
	materialIndex int,
) int {
	if modelData == nil || modelData.Faces == nil || modelData.Vertices == nil || materialIndex < 0 {
		return 0
	}
	if faceStart < 0 {
		faceStart = 0
	}
	if faceEnd > modelData.Faces.Len() {
		faceEnd = modelData.Faces.Len()
	}
	if faceEnd <= faceStart {
		return 0
	}
	copiedFaceCount := 0
	duplicatedVertexIndexes := map[int]int{}
	for faceIndex := faceStart; faceIndex < faceEnd; faceIndex++ {
		faceData, err := modelData.Faces.Get(faceIndex)
		if err != nil || faceData == nil {
			continue
		}
		copiedVertexIndexes := [3]int{}
		allVerticesReady := true
		for i, sourceVertexIndex := range faceData.VertexIndexes {
			if duplicatedVertexIndex, exists := duplicatedVertexIndexes[sourceVertexIndex]; exists {
				copiedVertexIndexes[i] = duplicatedVertexIndex
				continue
			}
			sourceVertex, vertexErr := modelData.Vertices.Get(sourceVertexIndex)
			if vertexErr != nil || sourceVertex == nil {
				allVerticesReady = false
				break
			}
			duplicatedVertex := &model.Vertex{
				Position:        sourceVertex.Position,
				Normal:          sourceVertex.Normal,
				Uv:              sourceVertex.Uv,
				ExtendedUvs:     append([]mmath.Vec4(nil), sourceVertex.ExtendedUvs...),
				DeformType:      sourceVertex.DeformType,
				Deform:          sourceVertex.Deform,
				EdgeFactor:      sourceVertex.EdgeFactor,
				MaterialIndexes: []int{materialIndex},
			}
			duplicatedVertexIndex := modelData.Vertices.AppendRaw(duplicatedVertex)
			duplicatedVertexIndexes[sourceVertexIndex] = duplicatedVertexIndex
			copiedVertexIndexes[i] = duplicatedVertexIndex
		}
		if !allVerticesReady {
			continue
		}
		copiedFace := &model.Face{
			VertexIndexes: copiedVertexIndexes,
		}
		modelData.Faces.AppendRaw(copiedFace)
		copiedFaceCount++
	}
	return copiedFaceCount
}

// resolveSpecialEyeTokenMatchLevel はトークン一致優先度(1:Texture 2:EnglishName 3:NameSuffix)を返す。
func resolveSpecialEyeTokenMatchLevel(materialInfo specialEyeMaterialInfo, token string) int {
	normalizedToken := normalizeSpecialEyeToken(token)
	if normalizedToken == "" {
		return 0
	}
	if strings.Contains(materialInfo.NormalizedTextureMatch, normalizedToken) {
		return 1
	}
	if strings.Contains(materialInfo.NormalizedEnglishName, normalizedToken) {
		return 2
	}
	if strings.HasSuffix(materialInfo.NormalizedName, normalizedToken) {
		return 3
	}
	return 0
}

// normalizeSpecialEyeToken は特殊目判定トークンを正規化する。
func normalizeSpecialEyeToken(token string) string {
	return normalizeCreateSemanticName(token)
}

// appendSpecialEyeClassByLocalizedFallback は日本語/命名揺れで EyeClass を補完する。
func appendSpecialEyeClassByLocalizedFallback(
	classes map[string]struct{},
	materialName string,
	materialEnglishName string,
	textureName string,
	textureURI string,
) {
	if classes == nil {
		return
	}
	source := strings.ToLower(strings.TrimSpace(
		materialName + " " + materialEnglishName + " " + textureName + " " + textureURI,
	))
	if source == "" {
		return
	}
	if containsSpecialEyeToken(source, []string{"瞳", "虹彩", "iris", "pupil"}) {
		classes[specialEyeClassIris] = struct{}{}
	}
	if containsSpecialEyeToken(source, []string{"白目", "eyewhite", "sclera", "irishide"}) {
		classes[specialEyeClassWhite] = struct{}{}
	}
	if containsSpecialEyeToken(source, []string{"アイライン", "eyeline"}) {
		classes[specialEyeClassEyeLine] = struct{}{}
	}
	if containsSpecialEyeToken(source, []string{"まつげ", "睫毛", "eyelash", "lash"}) {
		classes[specialEyeClassEyeLash] = struct{}{}
	}
}

// hasLegacyFaceMaterialEnglishName は旧仕様の `_Face_` 判定で頬染め対象材質かを返す。
func hasLegacyFaceMaterialEnglishName(materialEnglishName string) bool {
	normalizedName := strings.ToLower(strings.TrimSpace(materialEnglishName))
	if normalizedName == "" {
		return false
	}
	return strings.Contains(normalizedName, "_face_")
}

// containsSpecialEyeToken は文字列がいずれかのトークンを含むか判定する。
func containsSpecialEyeToken(source string, tokens []string) bool {
	if strings.TrimSpace(source) == "" || len(tokens) == 0 {
		return false
	}
	for _, token := range tokens {
		if strings.TrimSpace(token) == "" {
			continue
		}
		if strings.Contains(source, strings.ToLower(token)) {
			return true
		}
	}
	return false
}

// hasSpecialEyeClass は特殊目材質クラス集合に対象が含まれるか判定する。
func hasSpecialEyeClass(classes map[string]struct{}, className string) bool {
	if len(classes) == 0 || strings.TrimSpace(className) == "" {
		return false
	}
	_, exists := classes[className]
	return exists
}

// appendSpecialEyeShowOffset は材質表示側のアルファ差分を加算する。
func appendSpecialEyeShowOffset(
	modelData *model.PmxModel,
	offsetsByMaterial map[int]*model.MaterialMorphOffset,
	materialIndex int,
) {
	if modelData == nil || modelData.Materials == nil || materialIndex < 0 {
		return
	}
	baseMaterial, err := modelData.Materials.Get(materialIndex)
	if err != nil || baseMaterial == nil {
		return
	}
	alphaDelta := 1.0 - baseMaterial.Diffuse.W
	if math.Abs(alphaDelta) <= 1e-9 {
		return
	}
	offsetData, exists := offsetsByMaterial[materialIndex]
	if !exists {
		offsetData = newMaterialMorphOffset(materialIndex)
		offsetsByMaterial[materialIndex] = offsetData
	}
	offsetData.Diffuse.W += alphaDelta
}

// appendSpecialEyeHideOffset は材質非表示側のアルファ差分を加算する。
func appendSpecialEyeHideOffset(
	modelData *model.PmxModel,
	offsetsByMaterial map[int]*model.MaterialMorphOffset,
	materialIndex int,
) {
	if modelData == nil || modelData.Materials == nil || materialIndex < 0 {
		return
	}
	baseMaterial, err := modelData.Materials.Get(materialIndex)
	if err != nil || baseMaterial == nil {
		return
	}
	alphaDelta := -baseMaterial.Diffuse.W
	if math.Abs(alphaDelta) <= 1e-9 {
		return
	}
	offsetData, exists := offsetsByMaterial[materialIndex]
	if !exists {
		offsetData = newMaterialMorphOffset(materialIndex)
		offsetsByMaterial[materialIndex] = offsetData
	}
	offsetData.Diffuse.W += alphaDelta
}

// toVec4WithDefault は float 配列を Vec4 へ変換し、不足分は既定値で補う。
func toVec4WithDefault(values []float64, defaultValue mmath.Vec4) mmath.Vec4 {
	result := defaultValue
	if len(values) > 0 {
		result.X = values[0]
	}
	if len(values) > 1 {
		result.Y = values[1]
	}
	if len(values) > 2 {
		result.Z = values[2]
	}
	if len(values) > 3 {
		result.W = values[3]
	}
	return result
}

// toVec3WithDefault は float 配列を Vec3 へ変換し、不足分は既定値で補う。
func toVec3WithDefault(values []float64, defaultValue mmath.Vec3) mmath.Vec3 {
	result := defaultValue
	if len(values) > 0 {
		result.X = values[0]
	}
	if len(values) > 1 {
		result.Y = values[1]
	}
	if len(values) > 2 {
		result.Z = values[2]
	}
	return result
}

// toVec2WithDefault は float 配列を Vec2 へ変換し、不足分は既定値で補う。
func toVec2WithDefault(values []float64, defaultValue mmath.Vec2) mmath.Vec2 {
	result := defaultValue
	if len(values) > 0 {
		result.X = values[0]
	}
	if len(values) > 1 {
		result.Y = values[1]
	}
	return result
}

// toFloatWithDefault は float 配列の先頭値を返し、欠損時は既定値を返す。
func toFloatWithDefault(values []float64, defaultValue float64) float64 {
	if len(values) == 0 {
		return defaultValue
	}
	return values[0]
}

// appendWeightedVertexOffsets は頂点オフセットへ重み付き差分を加算する。
func appendWeightedVertexOffsets(offsetsByVertex map[int]mmath.Vec3, offsets []model.IMorphOffset, weight float64) {
	if len(offsets) == 0 || weight <= 0 {
		return
	}
	for _, offset := range offsets {
		vertexOffset, ok := offset.(*model.VertexMorphOffset)
		if !ok {
			continue
		}
		if vertexOffset == nil {
			continue
		}
		weighted := vertexOffset.Position.MuledScalar(weight)
		current, exists := offsetsByVertex[vertexOffset.VertexIndex]
		if !exists {
			offsetsByVertex[vertexOffset.VertexIndex] = weighted
			continue
		}
		offsetsByVertex[vertexOffset.VertexIndex] = current.Added(weighted)
	}
}

// buildMergedVertexOffsets は頂点index順で統合済み頂点オフセットを返す。
func buildMergedVertexOffsets(offsetsByVertex map[int]mmath.Vec3) []model.IMorphOffset {
	if len(offsetsByVertex) == 0 {
		return nil
	}
	vertexIndexes := make([]int, 0, len(offsetsByVertex))
	for vertexIndex, position := range offsetsByVertex {
		if isZeroMorphDelta(position) {
			continue
		}
		vertexIndexes = append(vertexIndexes, vertexIndex)
	}
	if len(vertexIndexes) == 0 {
		return nil
	}
	sort.Ints(vertexIndexes)
	mergedOffsets := make([]model.IMorphOffset, 0, len(vertexIndexes))
	for _, vertexIndex := range vertexIndexes {
		position := offsetsByVertex[vertexIndex]
		if isZeroMorphDelta(position) {
			continue
		}
		mergedOffsets = append(mergedOffsets, &model.VertexMorphOffset{
			VertexIndex: vertexIndex,
			Position:    position,
		})
	}
	return mergedOffsets
}

// resolveExpressionPanel は表情名から PMX モーフパネルを推定する。
func resolveExpressionPanel(expressionName string) model.MorphPanel {
	lowerName := strings.ToLower(strings.TrimSpace(expressionName))
	if lowerName == "" {
		return model.MORPH_PANEL_OTHER_LOWER_RIGHT
	}
	if strings.Contains(lowerName, "brow") {
		return model.MORPH_PANEL_EYEBROW_LOWER_LEFT
	}
	if strings.Contains(lowerName, "eye") || strings.Contains(lowerName, "blink") || strings.Contains(lowerName, "look") {
		return model.MORPH_PANEL_EYE_UPPER_LEFT
	}
	if strings.Contains(lowerName, "mouth") || strings.Contains(lowerName, "jaw") {
		return model.MORPH_PANEL_LIP_UPPER_RIGHT
	}
	switch lowerName {
	case "a", "i", "u", "e", "o":
		return model.MORPH_PANEL_LIP_UPPER_RIGHT
	}
	return model.MORPH_PANEL_OTHER_LOWER_RIGHT
}

// resolveVrm1PresetExpressionName は VRM1 標準preset名を MMD モーフ名へ正規化する。
func resolveVrm1PresetExpressionName(expressionName string) string {
	normalized := strings.TrimSpace(expressionName)
	if normalized == "" {
		return ""
	}
	lowerName := strings.ToLower(normalized)
	if canonical, exists := vrm1PresetExpressionNamePairs[lowerName]; exists {
		return canonical
	}
	return normalized
}

// resolveVrm0PresetExpressionName は VRM0 標準preset名を MMD モーフ名へ正規化する。
func resolveVrm0PresetExpressionName(expressionName string) string {
	normalized := strings.TrimSpace(expressionName)
	if normalized == "" {
		return ""
	}
	lowerName := strings.ToLower(normalized)
	if canonical, exists := vrm0PresetExpressionNamePairs[lowerName]; exists {
		return canonical
	}
	return normalized
}

// resolveCanonicalExpressionName は VRM0/1 互換の既知表情名を MMD モーフ名へ正規化する。
func resolveCanonicalExpressionName(expressionName string) string {
	normalized := strings.TrimSpace(expressionName)
	if normalized == "" {
		return ""
	}
	for _, candidate := range buildCanonicalExpressionCandidates(normalized) {
		if canonical, exists := resolveLegacyCanonicalExpressionName(candidate); exists {
			return canonical
		}
		if canonical := resolveFclMouthExpressionName(candidate); canonical != candidate {
			return canonical
		}
	}
	return normalized
}

// buildCanonicalExpressionCandidates は正規化照合に使用する候補名配列を返す。
func buildCanonicalExpressionCandidates(expressionName string) []string {
	trimmedName := strings.TrimSpace(expressionName)
	if trimmedName == "" {
		return nil
	}
	candidates := make([]string, 0, 4)
	appendCandidate := func(value string) {
		normalizedValue := strings.TrimSpace(value)
		if normalizedValue == "" {
			return
		}
		for _, existing := range candidates {
			if existing == normalizedValue {
				return
			}
		}
		candidates = append(candidates, normalizedValue)
	}
	appendCandidate(trimmedName)
	appendCandidate(strings.ToLower(trimmedName))
	strippedName := stripPrimitiveTargetMorphPrefix(trimmedName)
	appendCandidate(strippedName)
	appendCandidate(strings.ToLower(strippedName))
	return candidates
}

// stripPrimitiveTargetMorphPrefix は __vrm_target_m***_t***_ 接頭辞を除去する。
func stripPrimitiveTargetMorphPrefix(name string) string {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return ""
	}
	return primitiveTargetMorphPrefixRegexp.ReplaceAllString(trimmedName, "")
}

// resolveLegacyCanonicalExpressionName は Legacy/Fcl/VRM標準presetの正規化結果を返す。
func resolveLegacyCanonicalExpressionName(expressionName string) (string, bool) {
	lowerName := strings.ToLower(strings.TrimSpace(expressionName))
	if lowerName == "" {
		return "", false
	}
	if canonical, exists := vrm1PresetExpressionNamePairs[lowerName]; exists {
		return canonical, true
	}
	if canonical, exists := vrm0PresetExpressionNamePairs[lowerName]; exists {
		return canonical, true
	}
	if canonical, exists := legacyExpressionNamePairs[lowerName]; exists {
		return canonical, true
	}
	return "", false
}

// resolveFclMouthExpressionName は Fcl_MTH_* 系表情名を規則で MMD モーフ名へ正規化する。
func resolveFclMouthExpressionName(expressionName string) string {
	normalized := strings.TrimSpace(expressionName)
	if normalized == "" {
		return ""
	}
	lowerName := strings.ToLower(normalized)
	if !strings.HasPrefix(lowerName, "fcl_mth_") {
		return normalized
	}
	baseName, kind := resolveFclMouthExpressionBaseAndKind(strings.TrimPrefix(lowerName, "fcl_mth_"))
	switch baseName {
	case "a":
		return resolveFclMouthExpressionNameByKind(kind, "あ頂点", "あボーン", "あ")
	case "i":
		return resolveFclMouthExpressionNameByKind(kind, "い頂点", "いボーン", "い")
	case "u":
		return resolveFclMouthExpressionNameByKind(kind, "う頂点", "うボーン", "う")
	case "e":
		return resolveFclMouthExpressionNameByKind(kind, "え頂点", "えボーン", "え")
	case "o":
		return resolveFclMouthExpressionNameByKind(kind, "お頂点", "おボーン", "お")
	case "joy":
		return resolveFclMouthExpressionNameByKind(kind, "ワ頂点", "ワボーン", "ワ")
	case "sorrow":
		return resolveFclMouthExpressionNameByKind(kind, "▲頂点", "▲ボーン", "▲")
	case "surprised":
		return resolveFclMouthExpressionNameByKind(kind, "わー頂点", "わーボーン", "わー")
	default:
		return normalized
	}
}

// resolveFclMouthExpressionBaseAndKind は Fcl_MTH_* 名からベース名と種別を返す。
func resolveFclMouthExpressionBaseAndKind(name string) (string, string) {
	normalized := strings.TrimSpace(strings.ToLower(name))
	if strings.HasSuffix(normalized, "_bone") {
		return strings.TrimSuffix(normalized, "_bone"), "bone"
	}
	if strings.HasSuffix(normalized, "_group") {
		return strings.TrimSuffix(normalized, "_group"), "group"
	}
	return normalized, "vertex"
}

// resolveFclMouthExpressionNameByKind は種別ごとに MMD モーフ名を返す。
func resolveFclMouthExpressionNameByKind(kind string, vertexName string, boneName string, groupName string) string {
	switch kind {
	case "bone":
		return boneName
	case "group":
		return groupName
	default:
		return vertexName
	}
}

const (
	expressionBoneSemanticRightEyeLight = "right_eye_light"
	expressionBoneSemanticLeftEyeLight  = "left_eye_light"
	expressionBoneSemanticTongue1       = "tongue_1"
	expressionBoneSemanticTongue2       = "tongue_2"
	expressionBoneSemanticTongue3       = "tongue_3"
	expressionBoneSemanticTongue4       = "tongue_4"
)

// expressionBoneOffsetRule はボーンモーフ1オフセット分の解決規則を表す。
type expressionBoneOffsetRule struct {
	Semantic string
	Move     mmath.Vec3
	Rotate   mmath.Quaternion
}

// expressionBoneMorphFallbackRule はボーンモーフ補完規則を表す。
type expressionBoneMorphFallbackRule struct {
	Name    string
	Offsets []expressionBoneOffsetRule
}

// expressionBoneFallbackRules は旧参考実装のボーンモーフ初期値を表す。
var expressionBoneFallbackRules = []expressionBoneMorphFallbackRule{
	{
		Name: "ｳｨﾝｸ２右ボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticRightEyeLight, 0, 0, -0.015, -12, 0, 0),
		},
	},
	{
		Name: "ウィンク２ボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticLeftEyeLight, 0, 0, -0.015, -12, 0, 0),
		},
	},
	{
		Name: "ウィンク右ボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticRightEyeLight, 0, 0, 0.025, 8, 0, 0),
		},
	},
	{
		Name: "ウィンクボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticLeftEyeLight, 0, 0, 0.025, 8, 0, 0),
		},
	},
	{
		Name: "あボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue1, 0, 0, 0, -16, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue2, 0, 0, 0, -16, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue3, 0, 0, 0, -10, 0, 0),
		},
	},
	{
		Name: "いボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue1, 0, 0, 0, -6, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue2, 0, 0, 0, -6, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue3, 0, 0, 0, -3, 0, 0),
		},
	},
	{
		Name: "うボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue1, 0, 0, 0, -16, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue2, 0, 0, 0, -16, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue3, 0, 0, 0, -10, 0, 0),
		},
	},
	{
		Name: "えボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue1, 0, 0, 0, -6, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue2, 0, 0, 0, -6, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue3, 0, 0, 0, -3, 0, 0),
		},
	},
	{
		Name: "おボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue1, 0, 0, 0, -20, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue2, 0, 0, 0, -18, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue3, 0, 0, 0, -12, 0, 0),
		},
	},
	{
		Name: "ワボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue1, 0, 0, 0, -24, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue2, 0, 0, 0, -24, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue3, 0, 0, 0, 16, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue4, 0, 0, 0, 28, 0, 0),
		},
	},
	{
		Name: "▲ボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue1, 0, 0, 0, -6, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue2, 0, 0, 0, -6, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue3, 0, 0, 0, -3, 0, 0),
		},
	},
	{
		Name: "わーボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue1, 0, 0, 0, -24, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue2, 0, 0, 0, -24, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue3, 0, 0, 0, 16, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue4, 0, 0, 0, 28, 0, 0),
		},
	},
	{
		Name: "べーボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue1, 0, 0, 0, -9, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue2, 0, 0, -0.24, -13.2, 0, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue3, 0, 0, 0, -23.2, 0, 0),
		},
	},
	{
		Name: "ぺろりボーン",
		Offsets: []expressionBoneOffsetRule{
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue1, 0, 0, 0, 0, -5, 0),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue2, 0, -0.03, -0.18, 33, -16, -4),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue3, 0, 0, 0, 15, 3.6, -1),
			newExpressionBoneOffsetRule(expressionBoneSemanticTongue4, 0, 0, 0, 20, 0, 0),
		},
	},
}

// newExpressionBoneOffsetRule はボーンモーフ補完オフセット規則を生成する。
func newExpressionBoneOffsetRule(
	semantic string,
	moveX float64,
	moveY float64,
	moveZ float64,
	rotateX float64,
	rotateY float64,
	rotateZ float64,
) expressionBoneOffsetRule {
	return expressionBoneOffsetRule{
		Semantic: semantic,
		Move:     mmath.Vec3{Vec: r3.Vec{X: moveX, Y: moveY, Z: moveZ}},
		Rotate:   mmath.NewQuaternionFromDegrees(rotateX, rotateY, rotateZ),
	}
}

// appendExpressionBoneFallbackMorphs は連動規則で参照するボーンモーフの不足分を実値で補完する。
func appendExpressionBoneFallbackMorphs(modelData *model.PmxModel) {
	if modelData == nil || modelData.Morphs == nil || modelData.Bones == nil || modelData.Bones.Len() == 0 {
		return
	}
	generated := 0
	updated := 0
	skippedNoBone := 0
	for _, rule := range expressionBoneFallbackRules {
		if strings.TrimSpace(rule.Name) == "" {
			continue
		}
		offsets := buildExpressionBoneFallbackOffsets(modelData, rule)
		if len(offsets) == 0 {
			skippedNoBone++
			logVrmDebug("ボーンモーフ補完スキップ: name=%s reason=target_bone_not_found", rule.Name)
			continue
		}
		alreadyExists := false
		if _, err := modelData.Morphs.GetByName(rule.Name); err == nil {
			alreadyExists = true
		}
		if upsertTypedExpressionMorph(modelData, rule.Name, model.MORPH_PANEL_SYSTEM, model.MORPH_TYPE_BONE, offsets, false) == nil {
			continue
		}
		if alreadyExists {
			updated++
		} else {
			generated++
		}
	}
	if generated > 0 || updated > 0 || skippedNoBone > 0 {
		logVrmInfo("ボーンモーフ補完完了: generated=%d updated=%d skippedNoBone=%d", generated, updated, skippedNoBone)
	}
}

// buildExpressionBoneFallbackOffsets はボーンモーフ補完規則からオフセット一覧を構築する。
func buildExpressionBoneFallbackOffsets(modelData *model.PmxModel, rule expressionBoneMorphFallbackRule) []model.IMorphOffset {
	if modelData == nil || modelData.Bones == nil || len(rule.Offsets) == 0 {
		return nil
	}
	offsets := make([]model.IMorphOffset, 0, len(rule.Offsets))
	resolvedBoneIndexes := map[int]struct{}{}
	for _, offsetRule := range rule.Offsets {
		boneIndex, exists := resolveExpressionBoneIndexBySemanticOrFallback(modelData, offsetRule.Semantic)
		if !exists {
			continue
		}
		if _, duplicated := resolvedBoneIndexes[boneIndex]; duplicated {
			continue
		}
		resolvedBoneIndexes[boneIndex] = struct{}{}
		offsets = append(offsets, &model.BoneMorphOffset{
			BoneIndex: boneIndex,
			Position:  offsetRule.Move,
			Rotation:  offsetRule.Rotate,
		})
	}
	return offsets
}

// resolveExpressionBoneIndexBySemanticOrFallback はセマンティクス一致を優先し、未一致時は汎用候補へフォールバックする。
func resolveExpressionBoneIndexBySemanticOrFallback(modelData *model.PmxModel, semantic string) (int, bool) {
	if index, exists := resolveExpressionBoneIndexBySemantic(modelData, semantic); exists {
		return index, true
	}
	switch semantic {
	case expressionBoneSemanticRightEyeLight:
		if index, exists := resolveExpressionEyeBoneIndexByDirection(modelData, true); exists {
			return index, true
		}
	case expressionBoneSemanticLeftEyeLight:
		if index, exists := resolveExpressionEyeBoneIndexByDirection(modelData, false); exists {
			return index, true
		}
	case expressionBoneSemanticTongue1, expressionBoneSemanticTongue2, expressionBoneSemanticTongue3, expressionBoneSemanticTongue4:
		if index, exists := resolveExpressionTongueBoneIndexBySemantic(modelData, semantic); exists {
			return index, true
		}
		if index, exists := resolveExpressionAnyTongueBoneIndex(modelData); exists {
			return index, true
		}
	}
	if index, exists := resolveExpressionHeadBoneIndex(modelData); exists {
		return index, true
	}
	if modelData != nil && modelData.Bones != nil && modelData.Bones.Len() > 0 {
		firstBone, err := modelData.Bones.Get(0)
		if err == nil && firstBone != nil {
			return firstBone.Index(), true
		}
	}
	return -1, false
}

// resolveExpressionBoneIndexBySemantic は補完規則セマンティクスに対応するボーンindexを返す。
func resolveExpressionBoneIndexBySemantic(modelData *model.PmxModel, semantic string) (int, bool) {
	if modelData == nil || modelData.Bones == nil || strings.TrimSpace(semantic) == "" {
		return -1, false
	}
	for _, boneData := range modelData.Bones.Values() {
		if boneData == nil {
			continue
		}
		if matchesExpressionBoneSemanticName(boneData.Name(), semantic) || matchesExpressionBoneSemanticName(boneData.EnglishName, semantic) {
			return boneData.Index(), true
		}
	}
	return -1, false
}

// resolveExpressionEyeBoneIndexByDirection は左右方向付き目ボーンindexを返す。
func resolveExpressionEyeBoneIndexByDirection(modelData *model.PmxModel, right bool) (int, bool) {
	if modelData == nil || modelData.Bones == nil {
		return -1, false
	}
	// まず目光を優先する。
	for _, boneData := range modelData.Bones.Values() {
		if boneData == nil {
			continue
		}
		for _, targetName := range []string{boneData.Name(), boneData.EnglishName} {
			lowerName := strings.ToLower(strings.TrimSpace(targetName))
			if lowerName == "" {
				continue
			}
			if !isExpressionEyeLightName(lowerName) {
				continue
			}
			if right && isExpressionRightName(lowerName) {
				return boneData.Index(), true
			}
			if !right && isExpressionLeftName(lowerName) {
				return boneData.Index(), true
			}
		}
	}
	// 次に目ボーン一般へフォールバックする。
	for _, boneData := range modelData.Bones.Values() {
		if boneData == nil {
			continue
		}
		for _, targetName := range []string{boneData.Name(), boneData.EnglishName} {
			lowerName := strings.ToLower(strings.TrimSpace(targetName))
			if lowerName == "" {
				continue
			}
			if !(strings.Contains(lowerName, "目") || strings.Contains(lowerName, "eye")) {
				continue
			}
			if right && isExpressionRightName(lowerName) {
				return boneData.Index(), true
			}
			if !right && isExpressionLeftName(lowerName) {
				return boneData.Index(), true
			}
		}
	}
	return -1, false
}

// resolveExpressionTongueBoneIndexBySemantic は舌セマンティクス連番に一致するボーンindexを返す。
func resolveExpressionTongueBoneIndexBySemantic(modelData *model.PmxModel, semantic string) (int, bool) {
	if modelData == nil || modelData.Bones == nil {
		return -1, false
	}
	no := 0
	switch semantic {
	case expressionBoneSemanticTongue1:
		no = 1
	case expressionBoneSemanticTongue2:
		no = 2
	case expressionBoneSemanticTongue3:
		no = 3
	case expressionBoneSemanticTongue4:
		no = 4
	}
	if no <= 0 {
		return -1, false
	}
	for _, boneData := range modelData.Bones.Values() {
		if boneData == nil {
			continue
		}
		if matchesExpressionBoneSemanticName(boneData.Name(), semantic) || matchesExpressionBoneSemanticName(boneData.EnglishName, semantic) {
			return boneData.Index(), true
		}
	}
	return -1, false
}

// resolveExpressionAnyTongueBoneIndex は舌系ボーンの先頭indexを返す。
func resolveExpressionAnyTongueBoneIndex(modelData *model.PmxModel) (int, bool) {
	if modelData == nil || modelData.Bones == nil {
		return -1, false
	}
	for _, boneData := range modelData.Bones.Values() {
		if boneData == nil {
			continue
		}
		for _, targetName := range []string{boneData.Name(), boneData.EnglishName} {
			lowerName := strings.ToLower(strings.TrimSpace(targetName))
			if lowerName == "" {
				continue
			}
			if strings.Contains(lowerName, "舌") || strings.Contains(lowerName, "tongue") {
				return boneData.Index(), true
			}
		}
	}
	return -1, false
}

// resolveExpressionHeadBoneIndex は頭系ボーンの先頭indexを返す。
func resolveExpressionHeadBoneIndex(modelData *model.PmxModel) (int, bool) {
	if modelData == nil || modelData.Bones == nil {
		return -1, false
	}
	for _, boneData := range modelData.Bones.Values() {
		if boneData == nil {
			continue
		}
		for _, targetName := range []string{boneData.Name(), boneData.EnglishName} {
			lowerName := strings.ToLower(strings.TrimSpace(targetName))
			if lowerName == "" {
				continue
			}
			if strings.Contains(lowerName, "頭") || strings.Contains(lowerName, "head") {
				return boneData.Index(), true
			}
		}
	}
	return -1, false
}

// matchesExpressionBoneSemanticName はボーン名が補完規則セマンティクスへ一致するか判定する。
func matchesExpressionBoneSemanticName(boneName string, semantic string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(boneName))
	if lowerName == "" {
		return false
	}
	switch semantic {
	case expressionBoneSemanticRightEyeLight:
		return isExpressionEyeLightName(lowerName) && isExpressionRightName(lowerName)
	case expressionBoneSemanticLeftEyeLight:
		return isExpressionEyeLightName(lowerName) && isExpressionLeftName(lowerName)
	case expressionBoneSemanticTongue1:
		return isExpressionTongueName(lowerName, 1)
	case expressionBoneSemanticTongue2:
		return isExpressionTongueName(lowerName, 2)
	case expressionBoneSemanticTongue3:
		return isExpressionTongueName(lowerName, 3)
	case expressionBoneSemanticTongue4:
		return isExpressionTongueName(lowerName, 4)
	default:
		return false
	}
}

// isExpressionEyeLightName は目光ボーン相当の名前か判定する。
func isExpressionEyeLightName(lowerName string) bool {
	if strings.Contains(lowerName, "目光") {
		return true
	}
	return strings.Contains(lowerName, "eyelight") || (strings.Contains(lowerName, "eye") && strings.Contains(lowerName, "light"))
}

// isExpressionRightName は右側ボーン名か判定する。
func isExpressionRightName(lowerName string) bool {
	return strings.Contains(lowerName, "右") || strings.Contains(lowerName, "right") || strings.Contains(lowerName, "_r")
}

// isExpressionLeftName は左側ボーン名か判定する。
func isExpressionLeftName(lowerName string) bool {
	return strings.Contains(lowerName, "左") || strings.Contains(lowerName, "left") || strings.Contains(lowerName, "_l")
}

// isExpressionTongueName は舌ボーン名と連番が一致するか判定する。
func isExpressionTongueName(lowerName string, no int) bool {
	if no <= 0 {
		return false
	}
	indexText := fmt.Sprintf("%d", no)
	return strings.Contains(lowerName, "舌"+indexText) ||
		strings.Contains(lowerName, "tongue"+indexText) ||
		strings.Contains(lowerName, "tongue_"+indexText) ||
		strings.Contains(lowerName, "tongue0"+indexText)
}

// findMorphByNameOrCanonical はモーフ名の完全一致を優先し、未一致時は正規化名一致で検索する。
func findMorphByNameOrCanonical(modelData *model.PmxModel, morphName string) *model.Morph {
	if modelData == nil || modelData.Morphs == nil {
		return nil
	}
	trimmedName := strings.TrimSpace(morphName)
	if trimmedName == "" {
		return nil
	}
	if morphData, err := modelData.Morphs.GetByName(trimmedName); err == nil && morphData != nil {
		return morphData
	}
	targetKeys := map[string]struct{}{
		strings.ToLower(trimmedName): {},
	}
	if canonicalName := strings.TrimSpace(resolveCanonicalExpressionName(trimmedName)); canonicalName != "" {
		targetKeys[strings.ToLower(canonicalName)] = struct{}{}
	}
	for _, morphData := range modelData.Morphs.Values() {
		if morphData == nil {
			continue
		}
		candidateNames := []string{morphData.Name(), morphData.EnglishName}
		for _, candidateName := range candidateNames {
			trimmedCandidateName := strings.TrimSpace(candidateName)
			if trimmedCandidateName == "" {
				continue
			}
			if _, exists := targetKeys[strings.ToLower(trimmedCandidateName)]; exists {
				return morphData
			}
			canonicalCandidateName := strings.TrimSpace(resolveCanonicalExpressionName(trimmedCandidateName))
			if canonicalCandidateName == "" {
				continue
			}
			if _, exists := targetKeys[strings.ToLower(canonicalCandidateName)]; exists {
				return morphData
			}
		}
	}
	return nil
}

// normalizeMorphBindWeight は bind 重みを PMX 係数へ正規化する。
func normalizeMorphBindWeight(weight float64, isBinary bool) float64 {
	normalized := weight
	if normalized > 1 {
		normalized = normalized / 100.0
	}
	if normalized < 0 {
		normalized = 0
	}
	if isBinary {
		if normalized >= 0.5 {
			return 1.0
		}
		return 0.0
	}
	return normalized
}

// appendCreateMorphsFromFallbackRules は creates 規則に基づく頂点モーフを生成する。
func appendCreateMorphsFromFallbackRules(modelData *model.PmxModel, registry *targetMorphRegistry) {
	if modelData == nil || modelData.Morphs == nil || modelData.Vertices == nil {
		return
	}
	stats := createMorphStats{RuleCount: len(createMorphFallbackRules)}
	logVrmInfo("createsモーフ生成開始: rules=%d", stats.RuleCount)

	materialVertexMap := buildMaterialVertexIndexMap(modelData)
	morphSemanticVertexSets := buildCreateMorphSemanticVertexSets(modelData)
	materialSemanticVertexSets := buildCreateMaterialSemanticVertexSets(modelData, materialVertexMap)
	closeOffsets := collectMorphVertexOffsetsByNames(modelData, []string{"まばたき"})
	openFaceTriangles, leftClosedFaceTriangles, rightClosedFaceTriangles := buildCreateFaceTriangles(
		modelData,
		registry,
		closeOffsets,
	)

	for _, rule := range createMorphFallbackRules {
		existing, err := modelData.Morphs.GetByName(rule.Name)
		if err == nil && existing != nil && len(existing.Offsets) > 0 {
			stats.SkippedExisting++
			logVrmDebug("createsモーフ生成スキップ: name=%s reason=already_exists", rule.Name)
			continue
		}

		targetVertices := buildCreateTargetVertexSet(rule, modelData, morphSemanticVertexSets, materialSemanticVertexSets)
		if len(targetVertices) == 0 {
			stats.SkippedNoTarget++
			logVrmDebug("createsモーフ生成スキップ: name=%s reason=target_vertices_not_found", rule.Name)
			continue
		}
		hideVertices := buildCreateHideVertexSet(rule, modelData, morphSemanticVertexSets, materialSemanticVertexSets)
		offsets := buildCreateRuleOffsets(
			rule,
			modelData,
			targetVertices,
			hideVertices,
			closeOffsets,
			morphSemanticVertexSets,
			materialSemanticVertexSets,
			openFaceTriangles,
			leftClosedFaceTriangles,
			rightClosedFaceTriangles,
		)
		if len(offsets) == 0 {
			stats.SkippedNoOffset++
			logVrmDebug("createsモーフ生成スキップ: name=%s reason=offsets_not_generated", rule.Name)
			continue
		}
		upsertTypedExpressionMorph(
			modelData,
			rule.Name,
			rule.Panel,
			model.MORPH_TYPE_VERTEX,
			offsets,
			false,
		)
		stats.Generated++
		logVrmDebug("createsモーフ生成: name=%s offsets=%d", rule.Name, len(offsets))
	}

	logVrmInfo(
		"createsモーフ生成完了: rules=%d generated=%d skippedExisting=%d skippedNoTarget=%d skippedNoOffset=%d",
		stats.RuleCount,
		stats.Generated,
		stats.SkippedExisting,
		stats.SkippedNoTarget,
		stats.SkippedNoOffset,
	)
}

// appendExpressionLinkRules は binds/split の表情連動規則を適用する。
func appendExpressionLinkRules(modelData *model.PmxModel) {
	if modelData == nil || modelData.Morphs == nil || modelData.Vertices == nil {
		return
	}
	if len(expressionLinkRules) == 0 {
		return
	}
	pairFallbackApplied := appendExpressionSidePairGroupFallbacks(modelData)
	bindApplied := 0
	splitApplied := 0
	for _, rule := range expressionLinkRules {
		if len(rule.Binds) > 0 {
			if applyExpressionBindRule(modelData, rule) {
				bindApplied++
			}
			continue
		}
		if strings.TrimSpace(rule.Split) == "" {
			continue
		}
		if applyExpressionSplitRule(modelData, rule) {
			splitApplied++
		}
	}
	logVrmInfo(
		"表情連動規則適用完了: rules=%d pairFallbackApplied=%d bindsApplied=%d splitApplied=%d",
		len(expressionLinkRules),
		pairFallbackApplied,
		bindApplied,
		splitApplied,
	)
}

// appendExpressionSidePairGroupFallbacks は左右モーフから親グループを補完する。
func appendExpressionSidePairGroupFallbacks(modelData *model.PmxModel) int {
	if modelData == nil || modelData.Morphs == nil {
		return 0
	}
	applied := 0
	for _, rule := range expressionSidePairGroupFallbackRules {
		if applyExpressionSidePairGroupFallbackRule(modelData, rule) {
			applied++
		}
	}
	return applied
}

// applyExpressionSidePairGroupFallbackRule は左右モーフから親グループを生成する。
func applyExpressionSidePairGroupFallbackRule(
	modelData *model.PmxModel,
	rule expressionSidePairGroupFallbackRule,
) bool {
	if modelData == nil || modelData.Morphs == nil {
		return false
	}
	ruleName := strings.TrimSpace(rule.Name)
	if ruleName == "" || len(rule.Binds) == 0 {
		return false
	}
	if existingMorph, err := modelData.Morphs.GetByName(ruleName); err == nil && existingMorph != nil {
		return false
	}
	offsets := buildExpressionBindOffsets(
		modelData,
		expressionLinkRule{
			Name:  ruleName,
			Panel: rule.Panel,
			Binds: rule.Binds,
		},
	)
	if len(offsets) == 0 {
		return false
	}
	upsertTypedExpressionMorph(
		modelData,
		ruleName,
		rule.Panel,
		model.MORPH_TYPE_GROUP,
		offsets,
		false,
	)
	logVrmDebug("表情左右統合補完: name=%s offsets=%d", ruleName, len(offsets))
	return true
}

// applyExpressionBindRule は binds 規則からグループモーフを生成または更新する。
func applyExpressionBindRule(modelData *model.PmxModel, rule expressionLinkRule) bool {
	if modelData == nil || modelData.Morphs == nil {
		return false
	}
	ensureVertexBindSourcesFromTargetMorph(modelData, rule)
	ensureBindMorphSourcesFromPrimitiveTargets(modelData, rule)
	offsets := buildExpressionBindOffsets(modelData, rule)
	if len(offsets) == 0 {
		return false
	}
	upsertTypedExpressionMorph(
		modelData,
		rule.Name,
		rule.Panel,
		model.MORPH_TYPE_GROUP,
		offsets,
		false,
	)
	return true
}

// ensureVertexBindSourcesFromTargetMorph は target 名の頂点モーフを bind 用頂点モーフへ補完する。
func ensureVertexBindSourcesFromTargetMorph(modelData *model.PmxModel, rule expressionLinkRule) {
	if modelData == nil || modelData.Morphs == nil {
		return
	}
	targetName := strings.TrimSpace(rule.Name)
	if targetName == "" || len(rule.Binds) == 0 {
		return
	}
	for _, bindName := range rule.Binds {
		normalizedBindName := strings.TrimSpace(bindName)
		if normalizedBindName == "" || !strings.HasSuffix(normalizedBindName, "頂点") {
			continue
		}
		if existingMorph, getErr := modelData.Morphs.GetByName(normalizedBindName); getErr == nil && existingMorph != nil {
			if existingMorph.MorphType == model.MORPH_TYPE_VERTEX &&
				len(existingMorph.Offsets) > 0 &&
				shouldFilterVertexBindSourceByMouth(normalizedBindName, existingMorph.Name()) {
				filteredOffsets := filterVertexOffsetsByMouthVertexSet(modelData, existingMorph.Offsets)
				beforeCount, beforeMin, beforeMax := summarizeVertexOffsetIndexRange(existingMorph.Offsets)
				afterCount, afterMin, afterMax := summarizeVertexOffsetIndexRange(filteredOffsets)
				logVrmDebug(
					"表情連動頂点既存フィルタ: target=%s bind=%s source=%s before=%d(%d-%d) after=%d(%d-%d)",
					targetName,
					normalizedBindName,
					existingMorph.Name(),
					beforeCount,
					beforeMin,
					beforeMax,
					afterCount,
					afterMin,
					afterMax,
				)
				existingMorph.Offsets = filteredOffsets
			}
			existingCount, existingMin, existingMax := summarizeVertexOffsetIndexRange(existingMorph.Offsets)
			logVrmDebug(
				"表情連動頂点補完スキップ: bind=%s reason=already_exists type=%d offsets=%d min=%d max=%d",
				normalizedBindName,
				existingMorph.MorphType,
				existingCount,
				existingMin,
				existingMax,
			)
			continue
		}
		sourceMorph, clonedOffsets := resolveVertexBindSourceMorph(modelData, targetName, normalizedBindName)
		if len(clonedOffsets) == 0 {
			continue
		}
		upsertTypedExpressionMorph(
			modelData,
			normalizedBindName,
			model.MORPH_PANEL_SYSTEM,
			model.MORPH_TYPE_VERTEX,
			clonedOffsets,
			false,
		)
		clonedCount, clonedMin, clonedMax := summarizeVertexOffsetIndexRange(clonedOffsets)
		logVrmDebug(
			"表情連動頂点補完: target=%s bind=%s source=%s offsets=%d min=%d max=%d",
			targetName,
			normalizedBindName,
			sourceMorph.Name(),
			clonedCount,
			clonedMin,
			clonedMax,
		)
	}
}

// ensureBindMorphSourcesFromPrimitiveTargets は bind 名不足時に内部ターゲット頂点モーフから補完する。
func ensureBindMorphSourcesFromPrimitiveTargets(modelData *model.PmxModel, rule expressionLinkRule) {
	if modelData == nil || modelData.Morphs == nil || len(rule.Binds) == 0 {
		return
	}
	for _, bindName := range rule.Binds {
		normalizedBindName := strings.TrimSpace(bindName)
		if shouldSkipPrimitiveBindSourceCompletion(normalizedBindName) {
			continue
		}
		if existingMorph, getErr := modelData.Morphs.GetByName(normalizedBindName); getErr == nil && existingMorph != nil {
			continue
		}

		sourceMorph := findMorphByNameOrCanonical(modelData, normalizedBindName)
		if sourceMorph == nil || sourceMorph.MorphType != model.MORPH_TYPE_VERTEX || len(sourceMorph.Offsets) == 0 {
			continue
		}
		sourceName := strings.TrimSpace(sourceMorph.Name())
		if !strings.HasPrefix(sourceName, "__vrm_target_") {
			continue
		}

		clonedOffsets := cloneVertexMorphOffsets(sourceMorph.Offsets)
		if len(clonedOffsets) == 0 {
			continue
		}
		upsertTypedExpressionMorph(
			modelData,
			normalizedBindName,
			resolveExpressionPanel(normalizedBindName),
			model.MORPH_TYPE_VERTEX,
			clonedOffsets,
			false,
		)
		clonedCount, clonedMin, clonedMax := summarizeVertexOffsetIndexRange(clonedOffsets)
		logVrmDebug(
			"表情連動bind補完: bind=%s source=%s offsets=%d min=%d max=%d",
			normalizedBindName,
			sourceName,
			clonedCount,
			clonedMin,
			clonedMax,
		)
	}
}

// shouldSkipPrimitiveBindSourceCompletion は内部ターゲット補完対象外の bind 名か判定する。
func shouldSkipPrimitiveBindSourceCompletion(bindName string) bool {
	trimmedName := strings.TrimSpace(bindName)
	if trimmedName == "" {
		return true
	}
	return strings.HasSuffix(trimmedName, "頂点") ||
		strings.HasSuffix(trimmedName, "ボーン") ||
		strings.HasSuffix(trimmedName, "材質")
}

// resolveVertexBindSourceMorph は bind 先頂点モーフ補完に使用する元モーフを返す。
func resolveVertexBindSourceMorph(
	modelData *model.PmxModel,
	targetName string,
	bindName string,
) (*model.Morph, []model.IMorphOffset) {
	if modelData == nil || modelData.Morphs == nil {
		return nil, nil
	}
	if targetMorph := findMorphByNameOrCanonical(modelData, targetName); targetMorph != nil {
		if targetMorph.MorphType == model.MORPH_TYPE_VERTEX && len(targetMorph.Offsets) > 0 {
			clonedOffsets := cloneVertexMorphOffsets(targetMorph.Offsets)
			if len(clonedOffsets) > 0 {
				return targetMorph, clonedOffsets
			}
		}
	}
	for _, fallbackName := range resolveVertexBindSourceNames(bindName) {
		fallbackMorph := findMorphByNameOrCanonical(modelData, fallbackName)
		if fallbackMorph == nil {
			continue
		}
		if fallbackMorph.MorphType != model.MORPH_TYPE_VERTEX || len(fallbackMorph.Offsets) == 0 {
			continue
		}
		clonedOffsets := cloneVertexMorphOffsets(fallbackMorph.Offsets)
		if len(clonedOffsets) == 0 {
			continue
		}
		if shouldFilterVertexBindSourceByMouth(bindName, fallbackMorph.Name()) {
			filteredOffsets := filterVertexOffsetsByMouthVertexSet(modelData, clonedOffsets)
			beforeCount, beforeMin, beforeMax := summarizeVertexOffsetIndexRange(clonedOffsets)
			afterCount, afterMin, afterMax := summarizeVertexOffsetIndexRange(filteredOffsets)
			logVrmDebug(
				"表情連動頂点補完フィルタ: bind=%s source=%s before=%d(%d-%d) after=%d(%d-%d)",
				bindName,
				fallbackMorph.Name(),
				beforeCount,
				beforeMin,
				beforeMax,
				afterCount,
				afterMin,
				afterMax,
			)
			if len(filteredOffsets) == 0 {
				continue
			}
			return fallbackMorph, filteredOffsets
		}
		return fallbackMorph, clonedOffsets
	}
	return nil, nil
}

// resolveVertexBindSourceNames は bind 名から頂点補完元の候補名一覧を返す。
func resolveVertexBindSourceNames(bindName string) []string {
	normalizedBindName := strings.TrimSpace(bindName)
	if normalizedBindName == "" || !strings.HasSuffix(normalizedBindName, "頂点") {
		return nil
	}
	baseName := strings.TrimSpace(strings.TrimSuffix(normalizedBindName, "頂点"))
	candidates := []string{normalizedBindName}
	if baseName != "" {
		candidates = append(candidates, baseName)
	}
	switch baseName {
	case "ワ":
		candidates = append(candidates, "喜")
	case "▲":
		candidates = append(candidates, "哀")
	case "わー":
		candidates = append(candidates, "驚")
	}
	result := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		trimmedCandidate := strings.TrimSpace(candidate)
		if trimmedCandidate == "" {
			continue
		}
		if _, exists := seen[trimmedCandidate]; exists {
			continue
		}
		seen[trimmedCandidate] = struct{}{}
		result = append(result, trimmedCandidate)
	}
	return result
}

// shouldFilterVertexBindSourceByMouth は口形状補完時に口周辺頂点へ絞り込むべきか判定する。
func shouldFilterVertexBindSourceByMouth(bindName string, sourceMorphName string) bool {
	normalizedBindName := strings.TrimSpace(bindName)
	if normalizedBindName == "" || !strings.HasSuffix(normalizedBindName, "頂点") {
		return false
	}
	bindBaseName := strings.TrimSpace(strings.TrimSuffix(normalizedBindName, "頂点"))
	canonicalSourceName := strings.TrimSpace(resolveCanonicalExpressionName(sourceMorphName))
	switch bindBaseName {
	case "ワ":
		return canonicalSourceName == "喜" || canonicalSourceName == "ワ頂点"
	case "▲":
		return canonicalSourceName == "哀" || canonicalSourceName == "▲頂点"
	case "わー":
		return canonicalSourceName == "驚" || canonicalSourceName == "わー頂点"
	default:
		return false
	}
}

// filterVertexOffsetsByMouthVertexSet は口周辺頂点集合に含まれるオフセットだけを返す。
func filterVertexOffsetsByMouthVertexSet(modelData *model.PmxModel, offsets []model.IMorphOffset) []model.IMorphOffset {
	if len(offsets) == 0 {
		return nil
	}
	sourceVertexSet := collectVertexIndexSetFromOffsets(offsets)
	tokenMouthVertexSet := resolveMouthVertexSetFromMaterials(modelData)
	lowerFaceMouthVertexSet := resolveMouthVertexSetByFaceLowerArea(modelData, sourceVertexSet)
	tongueMaterialVertexSet := resolveTongueVertexSetByMaterialAndUv(modelData, sourceVertexSet)
	tongueBoneIndexSet := resolveTongueBoneIndexSet(modelData)
	mouthVertexSet := map[int]struct{}{}
	for vertexIndex := range tokenMouthVertexSet {
		mouthVertexSet[vertexIndex] = struct{}{}
	}
	for vertexIndex := range lowerFaceMouthVertexSet {
		mouthVertexSet[vertexIndex] = struct{}{}
	}
	tongueBoneIndexSet = refineTongueBoneIndexSetByInfluence(modelData, mouthVertexSet, tongueBoneIndexSet)
	if len(tongueBoneIndexSet) == 0 {
		tongueBoneIndexSet = inferTongueBoneIndexSetByInfluence(modelData, mouthVertexSet)
	}
	logVrmDebug(
		"口頂点抽出: source=%d token=%d lowerFace=%d union=%d tongueBones=%d tongueMaterial=%d",
		len(sourceVertexSet),
		len(tokenMouthVertexSet),
		len(lowerFaceMouthVertexSet),
		len(mouthVertexSet),
		len(tongueBoneIndexSet),
		len(tongueMaterialVertexSet),
	)
	if len(mouthVertexSet) == 0 {
		return nil
	}
	filteredOffsets := make([]model.IMorphOffset, 0, len(offsets))
	tongueBoneExcludedCount := 0
	tongueMaterialExcludedCount := 0
	for _, rawOffset := range offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil || offsetData.VertexIndex < 0 {
			continue
		}
		if _, exists := mouthVertexSet[offsetData.VertexIndex]; !exists {
			continue
		}
		if isTongueDeformVertex(modelData, offsetData.VertexIndex, tongueBoneIndexSet) {
			tongueBoneExcludedCount++
			continue
		}
		if _, exists := tongueMaterialVertexSet[offsetData.VertexIndex]; exists {
			tongueMaterialExcludedCount++
			continue
		}
		filteredOffsets = append(filteredOffsets, &model.VertexMorphOffset{
			VertexIndex: offsetData.VertexIndex,
			Position:    offsetData.Position,
		})
	}
	filteredCount, filteredMin, filteredMax := summarizeVertexOffsetIndexRange(filteredOffsets)
	logVrmDebug(
		"口頂点抽出結果: kept=%d min=%d max=%d tongueBoneExcluded=%d tongueMaterialExcluded=%d",
		filteredCount,
		filteredMin,
		filteredMax,
		tongueBoneExcludedCount,
		tongueMaterialExcludedCount,
	)
	return filteredOffsets
}

// boneInfluenceStat はボーンの頂点影響範囲統計を表す。
type boneInfluenceStat struct {
	GlobalCount int
	MouthCount  int
}

const (
	tongueBoneRefineMinMouthRatio         = 0.005
	tongueBoneRefineMinMouthConcentration = 0.55
	tongueBoneRefineBroadMouthCoverage    = 0.95
	tongueBoneRefineBroadMaxConcentration = 0.90
	tongueBoneInferMinMouthCount          = 12
	tongueBoneInferMinMouthConcentration  = 0.75
	tongueBoneInferMaxMouthCoverage       = 0.70
	tongueBoneInferMaxCount               = 8
	tongueMaterialHint                    = "facemouth"
	tongueUvXThreshold                    = 0.5
	tongueUvYThreshold                    = 0.5
)

// refineTongueBoneIndexSetByInfluence は舌ボーン候補から広域ボーンを除外する。
func refineTongueBoneIndexSetByInfluence(
	modelData *model.PmxModel,
	mouthVertexSet map[int]struct{},
	tongueBoneIndexSet map[int]struct{},
) map[int]struct{} {
	if modelData == nil || modelData.Vertices == nil || len(mouthVertexSet) == 0 || len(tongueBoneIndexSet) == 0 {
		return tongueBoneIndexSet
	}
	influenceStats := buildBoneInfluenceStats(modelData, mouthVertexSet)
	refinedSet := map[int]struct{}{}
	removedCount := 0
	for boneIndex := range tongueBoneIndexSet {
		stats, exists := influenceStats[boneIndex]
		if !exists || stats.GlobalCount == 0 || stats.MouthCount == 0 {
			removedCount++
			continue
		}
		mouthRatio := float64(stats.MouthCount) / float64(len(mouthVertexSet))
		mouthConcentration := float64(stats.MouthCount) / float64(stats.GlobalCount)
		if mouthRatio < tongueBoneRefineMinMouthRatio {
			removedCount++
			continue
		}
		if mouthRatio >= tongueBoneRefineBroadMouthCoverage && mouthConcentration < tongueBoneRefineBroadMaxConcentration {
			removedCount++
			continue
		}
		if mouthConcentration < tongueBoneRefineMinMouthConcentration {
			removedCount++
			continue
		}
		refinedSet[boneIndex] = struct{}{}
	}
	if removedCount > 0 {
		logVrmDebug("舌ボーン精査: before=%d after=%d removed=%d", len(tongueBoneIndexSet), len(refinedSet), removedCount)
	}
	return refinedSet
}

// inferTongueBoneIndexSetByInfluence は口領域への影響分布から舌ボーン候補を推定する。
func inferTongueBoneIndexSetByInfluence(modelData *model.PmxModel, mouthVertexSet map[int]struct{}) map[int]struct{} {
	inferredSet := map[int]struct{}{}
	if modelData == nil || modelData.Vertices == nil || len(mouthVertexSet) == 0 {
		return inferredSet
	}
	influenceStats := buildBoneInfluenceStats(modelData, mouthVertexSet)
	type tongueBoneCandidate struct {
		BoneIndex          int
		MouthCount         int
		MouthConcentration float64
		MouthCoverage      float64
	}
	candidates := make([]tongueBoneCandidate, 0, len(influenceStats))
	for boneIndex, stats := range influenceStats {
		if stats.GlobalCount == 0 || stats.MouthCount < tongueBoneInferMinMouthCount {
			continue
		}
		mouthConcentration := float64(stats.MouthCount) / float64(stats.GlobalCount)
		mouthCoverage := float64(stats.MouthCount) / float64(len(mouthVertexSet))
		if mouthConcentration < tongueBoneInferMinMouthConcentration {
			continue
		}
		if mouthCoverage > tongueBoneInferMaxMouthCoverage {
			continue
		}
		candidates = append(candidates, tongueBoneCandidate{
			BoneIndex:          boneIndex,
			MouthCount:         stats.MouthCount,
			MouthConcentration: mouthConcentration,
			MouthCoverage:      mouthCoverage,
		})
	}
	sort.Slice(candidates, func(left int, right int) bool {
		if candidates[left].MouthConcentration != candidates[right].MouthConcentration {
			return candidates[left].MouthConcentration > candidates[right].MouthConcentration
		}
		if candidates[left].MouthCount != candidates[right].MouthCount {
			return candidates[left].MouthCount > candidates[right].MouthCount
		}
		return candidates[left].BoneIndex < candidates[right].BoneIndex
	})
	maxCount := tongueBoneInferMaxCount
	if len(candidates) < maxCount {
		maxCount = len(candidates)
	}
	for candidateIndex := 0; candidateIndex < maxCount; candidateIndex++ {
		inferredSet[candidates[candidateIndex].BoneIndex] = struct{}{}
	}
	if len(inferredSet) > 0 {
		logVrmDebug("舌ボーン推定: candidates=%d selected=%d", len(candidates), len(inferredSet))
	}
	return inferredSet
}

// buildBoneInfluenceStats はボーンごとの全体/口領域頂点影響件数を集計する。
func buildBoneInfluenceStats(modelData *model.PmxModel, mouthVertexSet map[int]struct{}) map[int]boneInfluenceStat {
	statsByBone := map[int]boneInfluenceStat{}
	if modelData == nil || modelData.Vertices == nil {
		return statsByBone
	}
	for _, vertexData := range modelData.Vertices.Values() {
		if vertexData == nil || vertexData.Deform == nil {
			continue
		}
		_, isMouthVertex := mouthVertexSet[vertexData.Index()]
		boneIndexes := vertexData.Deform.Indexes()
		boneWeights := vertexData.Deform.Weights()
		for weightIndex, boneIndex := range boneIndexes {
			if boneIndex < 0 {
				continue
			}
			weightValue := 1.0
			if len(boneWeights) > 0 {
				if weightIndex >= len(boneWeights) {
					weightValue = 0.0
				} else {
					weightValue = boneWeights[weightIndex]
				}
			}
			if weightValue <= 0 {
				continue
			}
			stats := statsByBone[boneIndex]
			stats.GlobalCount++
			if isMouthVertex {
				stats.MouthCount++
			}
			statsByBone[boneIndex] = stats
		}
	}
	return statsByBone
}

// resolveTongueVertexSetByMaterialAndUv は FaceMouth 材質の舌UV領域にある頂点集合を返す。
func resolveTongueVertexSetByMaterialAndUv(
	modelData *model.PmxModel,
	sourceVertexSet map[int]struct{},
) map[int]struct{} {
	tongueVertexSet := map[int]struct{}{}
	if modelData == nil || modelData.Materials == nil || modelData.Vertices == nil || len(sourceVertexSet) == 0 {
		return tongueVertexSet
	}
	materialVertexMap := buildMaterialVertexIndexMap(modelData)
	if len(materialVertexMap) == 0 {
		return tongueVertexSet
	}
	for materialIndex, vertexIndexes := range materialVertexMap {
		if len(vertexIndexes) == 0 {
			continue
		}
		materialData, err := modelData.Materials.Get(materialIndex)
		if err != nil || materialData == nil || !isTongueMaterialName(materialData.Name(), materialData.EnglishName) {
			continue
		}
		for _, vertexIndex := range vertexIndexes {
			if _, exists := sourceVertexSet[vertexIndex]; !exists {
				continue
			}
			vertexData, vertexErr := modelData.Vertices.Get(vertexIndex)
			if vertexErr != nil || vertexData == nil {
				continue
			}
			if vertexData.Uv.X < tongueUvXThreshold || vertexData.Uv.Y > tongueUvYThreshold {
				continue
			}
			tongueVertexSet[vertexIndex] = struct{}{}
		}
	}
	return tongueVertexSet
}

// isTongueMaterialName は材質名が舌候補材質か判定する。
func isTongueMaterialName(name string, englishName string) bool {
	joinedName := strings.TrimSpace(name)
	if strings.TrimSpace(englishName) != "" {
		joinedName = strings.TrimSpace(joinedName + " " + englishName)
	}
	normalized := normalizeCreateSemanticName(joinedName)
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, tongueMaterialHint)
}

// collectVertexIndexSetFromOffsets は頂点オフセット配列から頂点index集合を返す。
func collectVertexIndexSetFromOffsets(offsets []model.IMorphOffset) map[int]struct{} {
	vertexSet := map[int]struct{}{}
	if len(offsets) == 0 {
		return vertexSet
	}
	for _, rawOffset := range offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil || offsetData.VertexIndex < 0 {
			continue
		}
		vertexSet[offsetData.VertexIndex] = struct{}{}
	}
	return vertexSet
}

// resolveMouthVertexSetFromMaterials は mouth/lip/jaw/facemouth トークン一致で口周辺頂点集合を返す。
func resolveMouthVertexSetFromMaterials(modelData *model.PmxModel) map[int]struct{} {
	mouthVertexSet := map[int]struct{}{}
	if modelData == nil || modelData.Materials == nil || modelData.Vertices == nil {
		return mouthVertexSet
	}
	materialVertexMap := buildMaterialVertexIndexMap(modelData)
	if len(materialVertexMap) == 0 {
		return mouthVertexSet
	}
	for materialIndex, vertexIndexes := range materialVertexMap {
		if len(vertexIndexes) == 0 {
			continue
		}
		materialData, err := modelData.Materials.Get(materialIndex)
		if err != nil || materialData == nil {
			continue
		}
		joinedName := strings.TrimSpace(materialData.Name())
		if strings.TrimSpace(materialData.EnglishName) != "" {
			joinedName = strings.TrimSpace(joinedName + " " + materialData.EnglishName)
		}
		if !containsMouthSemanticToken(joinedName) {
			continue
		}
		for _, vertexIndex := range vertexIndexes {
			mouthVertexSet[vertexIndex] = struct{}{}
		}
	}
	return mouthVertexSet
}

// resolveMouthVertexSetByFaceLowerArea は顔面材質のうち下側領域にある頂点集合を返す。
func resolveMouthVertexSetByFaceLowerArea(
	modelData *model.PmxModel,
	sourceVertexSet map[int]struct{},
) map[int]struct{} {
	mouthVertexSet := map[int]struct{}{}
	if modelData == nil || modelData.Vertices == nil || len(sourceVertexSet) == 0 {
		return mouthVertexSet
	}
	materialVertexMap := buildMaterialVertexIndexMap(modelData)
	if len(materialVertexMap) == 0 {
		return mouthVertexSet
	}
	faceVertexSet := resolveFaceVertexSetByMaterials(modelData, materialVertexMap)
	if len(faceVertexSet) == 0 {
		return mouthVertexSet
	}
	faceYThreshold, ok := resolveVertexSetYQuantile(modelData, faceVertexSet, 0.45)
	if !ok {
		return mouthVertexSet
	}
	for vertexIndex := range sourceVertexSet {
		if _, exists := faceVertexSet[vertexIndex]; !exists {
			continue
		}
		vertexData, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertexData == nil {
			continue
		}
		if vertexData.Position.Y > faceYThreshold {
			continue
		}
		mouthVertexSet[vertexIndex] = struct{}{}
	}
	return mouthVertexSet
}

// resolveFaceVertexSetByMaterials は face セマンティクス材質の頂点集合を返す。
func resolveFaceVertexSetByMaterials(
	modelData *model.PmxModel,
	materialVertexMap map[int][]int,
) map[int]struct{} {
	faceVertexSet := map[int]struct{}{}
	if modelData == nil || modelData.Materials == nil || len(materialVertexMap) == 0 {
		return faceVertexSet
	}
	primaryFaceMaterialIndexes := []int{}
	secondaryFaceMaterialIndexes := []int{}
	for materialIndex, vertexIndexes := range materialVertexMap {
		if len(vertexIndexes) == 0 {
			continue
		}
		materialData, err := modelData.Materials.Get(materialIndex)
		if err != nil || materialData == nil {
			continue
		}
		joinedName := strings.TrimSpace(materialData.Name())
		if strings.TrimSpace(materialData.EnglishName) != "" {
			joinedName = strings.TrimSpace(joinedName + " " + materialData.EnglishName)
		}
		tags := classifyCreateSemanticTags(joinedName)
		if !containsCreateSemantic(tags, createSemanticFace) {
			continue
		}
		if containsCreateSemantic(tags, createSemanticSkin) {
			primaryFaceMaterialIndexes = append(primaryFaceMaterialIndexes, materialIndex)
		} else {
			secondaryFaceMaterialIndexes = append(secondaryFaceMaterialIndexes, materialIndex)
		}
	}
	targetMaterialIndexes := primaryFaceMaterialIndexes
	if len(targetMaterialIndexes) == 0 {
		targetMaterialIndexes = secondaryFaceMaterialIndexes
	}
	for _, materialIndex := range targetMaterialIndexes {
		for _, vertexIndex := range materialVertexMap[materialIndex] {
			faceVertexSet[vertexIndex] = struct{}{}
		}
	}
	return faceVertexSet
}

// resolveVertexSetYQuantile は頂点集合のY値分位点を返す。
func resolveVertexSetYQuantile(
	modelData *model.PmxModel,
	vertexSet map[int]struct{},
	quantile float64,
) (float64, bool) {
	if modelData == nil || modelData.Vertices == nil || len(vertexSet) == 0 {
		return 0, false
	}
	if quantile < 0 {
		quantile = 0
	}
	if quantile > 1 {
		quantile = 1
	}
	yValues := make([]float64, 0, len(vertexSet))
	for vertexIndex := range vertexSet {
		vertexData, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertexData == nil {
			continue
		}
		yValues = append(yValues, vertexData.Position.Y)
	}
	if len(yValues) == 0 {
		return 0, false
	}
	sort.Float64s(yValues)
	quantileIndex := int(math.Floor(float64(len(yValues)-1) * quantile))
	if quantileIndex < 0 {
		quantileIndex = 0
	}
	if quantileIndex >= len(yValues) {
		quantileIndex = len(yValues) - 1
	}
	return yValues[quantileIndex], true
}

// resolveTongueBoneIndexSet は舌系ボーンindex集合を返す。
func resolveTongueBoneIndexSet(modelData *model.PmxModel) map[int]struct{} {
	tongueBoneIndexSet := map[int]struct{}{}
	if modelData == nil || modelData.Bones == nil {
		return tongueBoneIndexSet
	}
	nameMatchedCount := 0
	for _, boneData := range modelData.Bones.Values() {
		if boneData == nil {
			continue
		}
		if isTongueBoneName(boneData.Name()) ||
			isTongueBoneName(boneData.EnglishName) ||
			isTongueSemanticBoneName(boneData.Name()) ||
			isTongueSemanticBoneName(boneData.EnglishName) {
			if _, exists := tongueBoneIndexSet[boneData.Index()]; !exists {
				tongueBoneIndexSet[boneData.Index()] = struct{}{}
				nameMatchedCount++
			}
		}
	}
	morphMatchedCount := appendTongueBoneIndexesFromTongueMorphs(modelData, tongueBoneIndexSet)
	logVrmDebug("舌ボーン抽出: byName=%d byMorph=%d total=%d", nameMatchedCount, morphMatchedCount, len(tongueBoneIndexSet))
	return tongueBoneIndexSet
}

// isTongueBoneName はボーン名が舌系か判定する。
func isTongueBoneName(name string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	if lowerName == "" {
		return false
	}
	return strings.Contains(lowerName, "tongue") || strings.Contains(lowerName, "舌")
}

// isTongueSemanticBoneName は舌系の補助命名規則に一致するか判定する。
func isTongueSemanticBoneName(name string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	if lowerName == "" {
		return false
	}
	// VRoid では舌ボーンが FaceMouth 系命名になるケースがある。
	return strings.Contains(lowerName, "facemouth") ||
		isExpressionTongueName(lowerName, 1) ||
		isExpressionTongueName(lowerName, 2) ||
		isExpressionTongueName(lowerName, 3) ||
		isExpressionTongueName(lowerName, 4)
}

// appendTongueBoneIndexesFromTongueMorphs は舌系ボーンモーフ参照先を舌ボーン集合へ追加する。
func appendTongueBoneIndexesFromTongueMorphs(modelData *model.PmxModel, tongueBoneIndexSet map[int]struct{}) int {
	if modelData == nil || modelData.Morphs == nil {
		return 0
	}
	addedCount := 0
	for _, morphData := range modelData.Morphs.Values() {
		if morphData == nil || morphData.MorphType != model.MORPH_TYPE_BONE {
			continue
		}
		if !isTongueRelatedBoneMorphName(morphData.Name()) && !isTongueRelatedBoneMorphName(morphData.EnglishName) {
			continue
		}
		for _, rawOffset := range morphData.Offsets {
			offsetData, ok := rawOffset.(*model.BoneMorphOffset)
			if !ok || offsetData == nil || offsetData.BoneIndex < 0 {
				continue
			}
			if _, exists := tongueBoneIndexSet[offsetData.BoneIndex]; exists {
				continue
			}
			tongueBoneIndexSet[offsetData.BoneIndex] = struct{}{}
			addedCount++
		}
	}
	return addedCount
}

// isTongueRelatedBoneMorphName は舌系ボーンモーフ名か判定する。
func isTongueRelatedBoneMorphName(name string) bool {
	canonicalName := strings.TrimSpace(resolveCanonicalExpressionName(name))
	if canonicalName == "" {
		return false
	}
	switch canonicalName {
	case "あボーン", "いボーン", "うボーン", "えボーン", "おボーン", "ワボーン", "▲ボーン", "わーボーン", "べーボーン", "ぺろりボーン":
		return true
	default:
		return false
	}
}

// isTongueDeformVertex は頂点のウェイトに舌系ボーンが含まれるか判定する。
func isTongueDeformVertex(
	modelData *model.PmxModel,
	vertexIndex int,
	tongueBoneIndexSet map[int]struct{},
) bool {
	if modelData == nil || modelData.Vertices == nil || vertexIndex < 0 || len(tongueBoneIndexSet) == 0 {
		return false
	}
	vertexData, err := modelData.Vertices.Get(vertexIndex)
	if err != nil || vertexData == nil {
		return false
	}
	if vertexData.Deform == nil {
		return false
	}
	boneIndexes := vertexData.Deform.Indexes()
	if len(boneIndexes) == 0 {
		return false
	}
	boneWeights := vertexData.Deform.Weights()
	for weightIndex, boneIndex := range boneIndexes {
		if boneIndex < 0 {
			continue
		}
		if _, exists := tongueBoneIndexSet[boneIndex]; !exists {
			continue
		}
		weightValue := 1.0
		if len(boneWeights) > 0 {
			if weightIndex >= len(boneWeights) {
				weightValue = 0.0
			} else {
				weightValue = boneWeights[weightIndex]
			}
		}
		if weightValue > 0 {
			return true
		}
	}
	return false
}

// containsMouthSemanticToken は mouth/lip/jaw/facemouth トークンが含まれるか判定する。
func containsMouthSemanticToken(name string) bool {
	normalized := normalizeCreateSemanticName(name)
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "facemouth") ||
		strings.Contains(normalized, "mouth") ||
		strings.Contains(normalized, "lip") ||
		strings.Contains(normalized, "jaw")
}

// summarizeVertexOffsetIndexRange は頂点オフセット件数と頂点index範囲(min/max)を返す。
func summarizeVertexOffsetIndexRange(offsets []model.IMorphOffset) (int, int, int) {
	if len(offsets) == 0 {
		return 0, -1, -1
	}
	count := 0
	minVertexIndex := math.MaxInt
	maxVertexIndex := math.MinInt
	for _, rawOffset := range offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil || offsetData.VertexIndex < 0 {
			continue
		}
		count++
		if offsetData.VertexIndex < minVertexIndex {
			minVertexIndex = offsetData.VertexIndex
		}
		if offsetData.VertexIndex > maxVertexIndex {
			maxVertexIndex = offsetData.VertexIndex
		}
	}
	if count == 0 {
		return 0, -1, -1
	}
	return count, minVertexIndex, maxVertexIndex
}

// cloneVertexMorphOffsets は頂点モーフ差分の複製を返す。
func cloneVertexMorphOffsets(offsets []model.IMorphOffset) []model.IMorphOffset {
	if len(offsets) == 0 {
		return nil
	}
	cloned := make([]model.IMorphOffset, 0, len(offsets))
	for _, rawOffset := range offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil {
			continue
		}
		cloned = append(cloned, &model.VertexMorphOffset{
			VertexIndex: offsetData.VertexIndex,
			Position:    offsetData.Position,
		})
	}
	return cloned
}

// buildExpressionBindOffsets は binds 規則のグループモーフオフセット一覧を構築する。
func buildExpressionBindOffsets(modelData *model.PmxModel, rule expressionLinkRule) []model.IMorphOffset {
	if modelData == nil || modelData.Morphs == nil || len(rule.Binds) == 0 {
		return nil
	}
	limit := len(rule.Binds)
	useRatios := len(rule.Ratios) > 0
	if useRatios && len(rule.Ratios) < limit {
		// binds と ratios は短い方の要素数まで適用する。
		limit = len(rule.Ratios)
	}
	offsets := make([]model.IMorphOffset, 0, limit)
	for bindIndex := 0; bindIndex < limit; bindIndex++ {
		bindName := strings.TrimSpace(rule.Binds[bindIndex])
		if bindName == "" {
			continue
		}
		bindMorph := findMorphByNameOrCanonical(modelData, bindName)
		if bindMorph == nil {
			continue
		}
		factor := 1.0
		if useRatios {
			factor = rule.Ratios[bindIndex]
		}
		offsets = append(offsets, &model.GroupMorphOffset{
			MorphIndex:  bindMorph.Index(),
			MorphFactor: factor,
		})
	}
	return offsets
}

// applyExpressionSplitRule は split 規則から頂点モーフを生成または更新する。
func applyExpressionSplitRule(modelData *model.PmxModel, rule expressionLinkRule) bool {
	if modelData == nil || modelData.Morphs == nil || modelData.Vertices == nil {
		return false
	}
	sourceName := strings.TrimSpace(rule.Split)
	if sourceName == "" {
		return false
	}
	sourceMorph := findMorphByNameOrCanonical(modelData, sourceName)
	if sourceMorph == nil {
		return false
	}
	offsets := buildExpressionSplitOffsets(modelData, sourceMorph, rule)
	if len(offsets) == 0 {
		return false
	}
	upsertTypedExpressionMorph(
		modelData,
		rule.Name,
		rule.Panel,
		model.MORPH_TYPE_VERTEX,
		offsets,
		false,
	)
	return true
}

// buildExpressionSplitOffsets は split 規則の頂点モーフオフセット一覧を構築する。
func buildExpressionSplitOffsets(
	modelData *model.PmxModel,
	sourceMorph *model.Morph,
	rule expressionLinkRule,
) []model.IMorphOffset {
	if modelData == nil || modelData.Vertices == nil || sourceMorph == nil {
		return nil
	}
	if sourceMorph.MorphType != model.MORPH_TYPE_VERTEX {
		return nil
	}
	lowerEyelidRange, hasLowerEyelidRange := buildLowerEyelidSplitRange(modelData, sourceMorph)
	isLowerEyelidRule := hasLowerEyelidRange && isLowerEyelidRaiseMorph(rule.Name)

	offsets := make([]model.IMorphOffset, 0, len(sourceMorph.Offsets))
	for _, rawOffset := range sourceMorph.Offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil || isZeroMorphDelta(offsetData.Position) {
			continue
		}
		vertexData, err := modelData.Vertices.Get(offsetData.VertexIndex)
		if err != nil || vertexData == nil {
			continue
		}
		if !isCreateVertexInMorphSide(vertexData.Position, rule.Name) {
			continue
		}
		if isLowerEyelidRule && !shouldIncludeLowerEyelidSplitVertex(vertexData.Position, lowerEyelidRange) {
			continue
		}
		ratio := resolveExpressionSplitRatio(vertexData.Position, rule, lowerEyelidRange, isLowerEyelidRule)
		newOffset := offsetData.Position.MuledScalar(ratio)
		if isZeroMorphDelta(newOffset) {
			continue
		}
		offsets = append(offsets, &model.VertexMorphOffset{
			VertexIndex: offsetData.VertexIndex,
			Position:    newOffset,
		})
	}
	return offsets
}

// shouldIncludeExpressionSplitVertex は split 先モーフ名の左右接尾辞に対応する頂点か判定する。
func shouldIncludeExpressionSplitVertex(modelData *model.PmxModel, vertexIndex int, morphName string) bool {
	if modelData == nil || modelData.Vertices == nil || vertexIndex < 0 {
		return false
	}
	vertex, err := modelData.Vertices.Get(vertexIndex)
	if err != nil || vertex == nil {
		return false
	}
	return isCreateVertexInMorphSide(vertex.Position, morphName)
}

// lowerEyelidSplitRange は下瞼上げ split 用のY範囲を表す。
type lowerEyelidSplitRange struct {
	MinY      float64
	MaxY      float64
	MeanY     float64
	MinLimitY float64
	MaxLimitY float64
}

// buildLowerEyelidSplitRange は source モーフ頂点から下瞼上げ split 用Y範囲を構築する。
func buildLowerEyelidSplitRange(modelData *model.PmxModel, sourceMorph *model.Morph) (lowerEyelidSplitRange, bool) {
	if modelData == nil || modelData.Vertices == nil || sourceMorph == nil || len(sourceMorph.Offsets) == 0 {
		return lowerEyelidSplitRange{}, false
	}
	minY := math.MaxFloat64
	maxY := -math.MaxFloat64
	sumY := 0.0
	count := 0
	for _, rawOffset := range sourceMorph.Offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil || isZeroMorphDelta(offsetData.Position) {
			continue
		}
		vertexData, err := modelData.Vertices.Get(offsetData.VertexIndex)
		if err != nil || vertexData == nil {
			continue
		}
		y := vertexData.Position.Y
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
		sumY += y
		count++
	}
	if count <= 0 {
		return lowerEyelidSplitRange{}, false
	}
	meanY := sumY / float64(count)
	return lowerEyelidSplitRange{
		MinY:      minY,
		MaxY:      maxY,
		MeanY:     meanY,
		MinLimitY: (minY + meanY) / 2.0,
		MaxLimitY: (maxY + meanY) / 2.0,
	}, true
}

// isLowerEyelidRaiseMorph は下瞼上げ系 split 名か判定する。
func isLowerEyelidRaiseMorph(morphName string) bool {
	trimmedName := strings.TrimSpace(morphName)
	if trimmedName == "" {
		return false
	}
	return strings.HasSuffix(trimmedName, "下瞼上げ右") || strings.HasSuffix(trimmedName, "下瞼上げ左")
}

// shouldIncludeLowerEyelidSplitVertex は下瞼上げ split 対象頂点か判定する。
func shouldIncludeLowerEyelidSplitVertex(position mmath.Vec3, splitRange lowerEyelidSplitRange) bool {
	return position.Y <= splitRange.MinLimitY
}

// resolveExpressionSplitRatio は split 時に適用する頂点オフセット比率を返す。
func resolveExpressionSplitRatio(
	vertexPosition mmath.Vec3,
	rule expressionLinkRule,
	lowerEyelidRange lowerEyelidSplitRange,
	isLowerEyelidRule bool,
) float64 {
	if isLowerEyelidRule {
		if vertexPosition.Y < lowerEyelidRange.MaxLimitY {
			return 1.0
		}
		return calcExpressionLinearRatio(vertexPosition.Y, lowerEyelidRange.MinY, lowerEyelidRange.MaxLimitY, 0.0, 1.0)
	}
	if rule.Panel != model.MORPH_PANEL_LIP_UPPER_RIGHT {
		return 1.0
	}
	absX := math.Abs(vertexPosition.X)
	if absX >= 0.2 {
		return 1.0
	}
	return calcExpressionLinearRatio(absX, 0.0, 0.2, 0.0, 1.0)
}

// calcExpressionLinearRatio は線形補間値を返す。
func calcExpressionLinearRatio(value float64, oldMin float64, oldMax float64, newMin float64, newMax float64) float64 {
	if oldMax == oldMin {
		return newMin
	}
	return (((value - oldMin) * (newMax - newMin)) / (oldMax - oldMin)) + newMin
}

// buildCreateRuleOffsets は creates 規則のオフセットを生成する。
func buildCreateRuleOffsets(
	rule createMorphRule,
	modelData *model.PmxModel,
	targetVertices map[int]struct{},
	hideVertices map[int]struct{},
	closeOffsets map[int]mmath.Vec3,
	morphSemanticVertexSets map[string]map[int]struct{},
	materialSemanticVertexSets map[string]map[int]struct{},
	openFaceTriangles []createFaceTriangle,
	leftClosedFaceTriangles []createFaceTriangle,
	rightClosedFaceTriangles []createFaceTriangle,
) []model.IMorphOffset {
	if len(targetVertices) == 0 {
		return nil
	}
	switch rule.Type {
	case createMorphRuleTypeBrow:
		return buildCreateBrowOffsets(
			rule,
			modelData,
			targetVertices,
			morphSemanticVertexSets,
			materialSemanticVertexSets,
			openFaceTriangles,
		)
	case createMorphRuleTypeEyeSmall:
		baseOffsets := resolveCreateEyeSurprisedOffsets(modelData, rule.Name)
		return buildCreateEyeScaleOffsets(modelData, targetVertices, baseOffsets, false)
	case createMorphRuleTypeEyeBig:
		baseOffsets := resolveCreateEyeSurprisedOffsets(modelData, rule.Name)
		return buildCreateEyeScaleOffsets(modelData, targetVertices, baseOffsets, true)
	case createMorphRuleTypeEyeHideVertex:
		return buildCreateEyeHideOffsets(
			modelData,
			targetVertices,
			hideVertices,
			closeOffsets,
			openFaceTriangles,
			leftClosedFaceTriangles,
			rightClosedFaceTriangles,
		)
	default:
		return nil
	}
}

// buildCreateTargetVertexSet は creates 対象頂点集合を解決する。
func buildCreateTargetVertexSet(
	rule createMorphRule,
	modelData *model.PmxModel,
	morphSemanticVertexSets map[string]map[int]struct{},
	materialSemanticVertexSets map[string]map[int]struct{},
) map[int]struct{} {
	targetVertices := map[int]struct{}{}
	for _, semantic := range resolveCreateRuleSemantics(rule.Creates) {
		semanticVertices := map[int]struct{}{}
		if rule.Type == createMorphRuleTypeEyeHideVertex {
			if vertices := materialSemanticVertexSets[semantic]; len(vertices) > 0 {
				semanticVertices = vertices
			} else {
				semanticVertices = resolveCreateSemanticVertexSet(semantic, morphSemanticVertexSets, materialSemanticVertexSets)
			}
		} else {
			semanticVertices = resolveCreateSemanticVertexSet(semantic, morphSemanticVertexSets, materialSemanticVertexSets)
		}
		for vertexIndex := range semanticVertices {
			targetVertices[vertexIndex] = struct{}{}
		}
	}
	if len(targetVertices) == 0 && (rule.Type == createMorphRuleTypeEyeSmall || rule.Type == createMorphRuleTypeEyeBig) {
		for vertexIndex := range resolveCreateEyeSurprisedOffsets(modelData, rule.Name) {
			targetVertices[vertexIndex] = struct{}{}
		}
	}
	if rule.Type == createMorphRuleTypeEyeHideVertex {
		overlayTargets := map[int]struct{}{}
		for vertexIndex := range targetVertices {
			if isSpecialEyeOverlayVertex(modelData, vertexIndex) {
				overlayTargets[vertexIndex] = struct{}{}
			}
		}
		if len(overlayTargets) > 0 {
			targetVertices = overlayTargets
		}

		originalTargets := map[int]struct{}{}
		for vertexIndex := range targetVertices {
			originalTargets[vertexIndex] = struct{}{}
		}
		irisVertices := resolveCreateSemanticVertexSet(createSemanticIris, morphSemanticVertexSets, materialSemanticVertexSets)
		filteredTargets := map[int]struct{}{}
		for vertexIndex := range targetVertices {
			filteredTargets[vertexIndex] = struct{}{}
		}
		for irisVertexIndex := range irisVertices {
			delete(filteredTargets, irisVertexIndex)
		}
		if len(filteredTargets) > 0 {
			targetVertices = filteredTargets
		} else {
			targetVertices = originalTargets
		}
	} else {
		for vertexIndex := range targetVertices {
			if isSpecialEyeOverlayVertex(modelData, vertexIndex) {
				delete(targetVertices, vertexIndex)
			}
		}
	}
	return filterCreateVertexSetBySide(modelData, targetVertices, rule.Name)
}

// isSpecialEyeOverlayVertex は頂点が特殊目オーバーレイ材質専用か判定する。
func isSpecialEyeOverlayVertex(modelData *model.PmxModel, vertexIndex int) bool {
	if modelData == nil || modelData.Vertices == nil || modelData.Materials == nil || vertexIndex < 0 {
		return false
	}
	vertexData, err := modelData.Vertices.Get(vertexIndex)
	if err != nil || vertexData == nil {
		return false
	}
	if len(vertexData.MaterialIndexes) == 0 {
		return false
	}
	for _, materialIndex := range vertexData.MaterialIndexes {
		materialData, materialErr := modelData.Materials.Get(materialIndex)
		if materialErr != nil || materialData == nil {
			continue
		}
		normalizedName := normalizeCreateSemanticName(strings.TrimSpace(materialData.Name() + " " + materialData.EnglishName))
		for _, token := range specialEyeOverlayTextureTokens {
			if strings.Contains(normalizedName, normalizeSpecialEyeToken(token)) {
				return true
			}
		}
	}
	return false
}

// buildCreateHideVertexSet は hides 対象頂点集合を解決する。
func buildCreateHideVertexSet(
	rule createMorphRule,
	modelData *model.PmxModel,
	morphSemanticVertexSets map[string]map[int]struct{},
	materialSemanticVertexSets map[string]map[int]struct{},
) map[int]struct{} {
	hideVertices := map[int]struct{}{}
	for _, semantic := range resolveCreateHideSemantics(rule.Hides) {
		semanticVertices := resolveCreateSemanticVertexSet(semantic, morphSemanticVertexSets, materialSemanticVertexSets)
		for vertexIndex := range semanticVertices {
			hideVertices[vertexIndex] = struct{}{}
		}
	}
	return filterCreateVertexSetBySide(modelData, hideVertices, rule.Name)
}

// buildCreateBrowOffsets は眉 creates 規則のオフセットを生成する。
func buildCreateBrowOffsets(
	rule createMorphRule,
	modelData *model.PmxModel,
	targetVertices map[int]struct{},
	morphSemanticVertexSets map[string]map[int]struct{},
	materialSemanticVertexSets map[string]map[int]struct{},
	openFaceTriangles []createFaceTriangle,
) []model.IMorphOffset {
	if modelData == nil || modelData.Vertices == nil || len(targetVertices) == 0 {
		return nil
	}
	eyelineVertices := resolveCreateSemanticVertexSet(createSemanticEyeLine, morphSemanticVertexSets, materialSemanticVertexSets)
	offsetDistance := resolveCreateBrowOffsetDistance(modelData, targetVertices, eyelineVertices)
	offsetsByVertex := map[int]mmath.Vec3{}
	for _, vertexIndex := range sortedCreateVertexIndexes(targetVertices) {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		offset := resolveCreateBrowBaseDelta(rule.Name, offsetDistance)
		if !isCreateBrowFrontMorph(rule.Name) {
			morphedPos := vertex.Position.Added(offset)
			if projectedZ, ok := projectCreateOffsetToFace(morphedPos, openFaceTriangles, createMorphBrowProjectionZOffset); ok {
				offset.Z = projectedZ
			}
		}
		if isZeroMorphDelta(offset) {
			continue
		}
		offsetsByVertex[vertexIndex] = offsetsByVertex[vertexIndex].Added(offset)
	}
	return buildMergedVertexOffsets(offsetsByVertex)
}

// resolveCreateBrowOffsetDistance は眉オフセット量を推定する。
func resolveCreateBrowOffsetDistance(
	modelData *model.PmxModel,
	targetVertices map[int]struct{},
	eyelineVertices map[int]struct{},
) float64 {
	if modelData == nil || modelData.Vertices == nil || len(targetVertices) == 0 || len(eyelineVertices) == 0 {
		return createMorphBrowDefaultDistance
	}
	maxTargetY := math.Inf(-1)
	for vertexIndex := range targetVertices {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		if vertex.Position.Y > maxTargetY {
			maxTargetY = vertex.Position.Y
		}
	}
	maxEyeLineY := math.Inf(-1)
	for vertexIndex := range eyelineVertices {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		if vertex.Position.Y > maxEyeLineY {
			maxEyeLineY = vertex.Position.Y
		}
	}
	if math.IsInf(maxTargetY, -1) || math.IsInf(maxEyeLineY, -1) {
		return createMorphBrowDefaultDistance
	}
	diff := math.Abs(maxTargetY - maxEyeLineY)
	if diff <= 1e-9 {
		return createMorphBrowDefaultDistance
	}
	return diff * createMorphBrowDistanceRatio
}

// resolveCreateBrowBaseDelta は眉モーフ種別ごとの基本移動量を返す。
func resolveCreateBrowBaseDelta(morphName string, offsetDistance float64) mmath.Vec3 {
	switch morphName {
	case "下右", "下左":
		return mmath.Vec3{Vec: r3.Vec{Y: -offsetDistance}}
	case "上右", "上左":
		return mmath.Vec3{Vec: r3.Vec{Y: offsetDistance}}
	case "右眉左", "左眉左":
		return mmath.Vec3{Vec: r3.Vec{X: offsetDistance}}
	case "右眉右", "左眉右":
		return mmath.Vec3{Vec: r3.Vec{X: -offsetDistance}}
	case "右眉手前", "左眉手前":
		return mmath.Vec3{Vec: r3.Vec{Z: -offsetDistance}}
	default:
		return mmath.ZERO_VEC3
	}
}

// isCreateBrowFrontMorph は眉create名が手前方向かを返す。
func isCreateBrowFrontMorph(morphName string) bool {
	return strings.Contains(morphName, "手前")
}

// buildCreateEyeScaleOffsets は 瞳小/瞳大 のオフセットを生成する。
func buildCreateEyeScaleOffsets(
	modelData *model.PmxModel,
	targetVertices map[int]struct{},
	baseOffsets map[int]mmath.Vec3,
	invert bool,
) []model.IMorphOffset {
	if modelData == nil || modelData.Vertices == nil || len(targetVertices) == 0 {
		return nil
	}
	meanPos, hasMean := calcCreateVertexSetMean(modelData, targetVertices)
	offsetsByVertex := map[int]mmath.Vec3{}
	for _, vertexIndex := range sortedCreateVertexIndexes(targetVertices) {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		offset, exists := baseOffsets[vertexIndex]
		if !exists || isZeroMorphDelta(offset) {
			if !hasMean {
				continue
			}
			offset = meanPos.Subed(vertex.Position).MuledScalar(createMorphEyeFallbackScaleRatio)
		}
		if invert {
			offset = offset.MuledScalar(-1)
		}
		if isZeroMorphDelta(offset) {
			continue
		}
		offsetsByVertex[vertexIndex] = offsetsByVertex[vertexIndex].Added(offset)
	}
	return buildMergedVertexOffsets(offsetsByVertex)
}

// buildCreateEyeHideOffsets は 目隠し頂点 のオフセットを生成する。
func buildCreateEyeHideOffsets(
	modelData *model.PmxModel,
	targetVertices map[int]struct{},
	hideVertices map[int]struct{},
	closeOffsets map[int]mmath.Vec3,
	openFaceTriangles []createFaceTriangle,
	leftClosedFaceTriangles []createFaceTriangle,
	rightClosedFaceTriangles []createFaceTriangle,
) []model.IMorphOffset {
	if modelData == nil || modelData.Vertices == nil || len(targetVertices) == 0 || len(closeOffsets) == 0 {
		return nil
	}
	offsetsByVertex := map[int]mmath.Vec3{}
	leftVertices := map[int]struct{}{}
	rightVertices := map[int]struct{}{}
	for vertexIndex := range targetVertices {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		if vertex.Position.X > 0 {
			leftVertices[vertexIndex] = struct{}{}
		} else if vertex.Position.X < 0 {
			rightVertices[vertexIndex] = struct{}{}
		}
	}
	allMean, hasAllMean := calcCreateVertexSetMean(modelData, targetVertices)
	leftMean, hasLeftMean := calcCreateVertexSetMean(modelData, leftVertices)
	rightMean, hasRightMean := calcCreateVertexSetMean(modelData, rightVertices)
	for vertexIndex, baseOffset := range closeOffsets {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		offset := baseOffset.Muled(mmath.Vec3{Vec: r3.Vec{X: 1.0, Y: createMorphEyeHideScaleY, Z: 1.0}})
		if _, isHideTarget := hideVertices[vertexIndex]; isHideTarget {
			targetCenter := allMean
			hasCenter := hasAllMean
			if vertex.Position.X > 0 && hasLeftMean {
				targetCenter = leftMean
				hasCenter = true
			}
			if vertex.Position.X < 0 && hasRightMean {
				targetCenter = rightMean
				hasCenter = true
			}
			if hasCenter {
				offset = targetCenter.Subed(vertex.Position)
			}
		}
		if isZeroMorphDelta(offset) {
			continue
		}
		offsetsByVertex[vertexIndex] = offsetsByVertex[vertexIndex].Added(offset)
	}

	allStats := newCreateVertexStats()
	leftStats := newCreateVertexStats()
	rightStats := newCreateVertexStats()
	for vertexIndex := range targetVertices {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		allStats.Add(vertex.Position)
		if vertex.Position.X > 0 {
			leftStats.Add(vertex.Position)
		}
		if vertex.Position.X < 0 {
			rightStats.Add(vertex.Position)
		}
	}
	for _, vertexIndex := range sortedCreateVertexIndexes(targetVertices) {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		stats := allStats
		if vertex.Position.X > 0 && leftStats.Count > 0 {
			stats = leftStats
		}
		if vertex.Position.X < 0 && rightStats.Count > 0 {
			stats = rightStats
		}
		meanPos, hasMean := stats.Mean()
		if !hasMean {
			continue
		}
		offset := mmath.ZERO_VEC3
		diffX := stats.Max.X - stats.Min.X
		diffY := stats.Max.Y - stats.Min.Y
		if diffX > 1e-9 && diffY > 1e-9 {
			if diffX > diffY {
				base := math.Abs(vertex.Position.X - meanPos.X)
				diff := math.Abs(base*diffY/diffX - base)
				offset.X = diff * math.Copysign(1.0, meanPos.X-vertex.Position.X)
			} else {
				base := math.Abs(vertex.Position.Y - meanPos.Y)
				diff := math.Abs(base*diffX/diffY - base)
				offset.Y = diff * math.Copysign(1.0, meanPos.Y-vertex.Position.Y)
			}
		}
		offset = offset.Added(vertex.Position.Subed(meanPos).Muled(mmath.Vec3{Vec: r3.Vec{X: 0.1, Y: 0.1, Z: 0.0}}))
		// 目隠し頂点は、既存のまばたき変形を適用した後の位置を基準に顔面へ射影する。
		morphedPos := vertex.Position.Added(closeOffsets[vertexIndex]).Added(offset)
		targetFaceTriangles := openFaceTriangles
		if vertex.Position.X > 0 && len(leftClosedFaceTriangles) > 0 {
			targetFaceTriangles = leftClosedFaceTriangles
		}
		if vertex.Position.X < 0 && len(rightClosedFaceTriangles) > 0 {
			targetFaceTriangles = rightClosedFaceTriangles
		}
		if projectedZ, ok := projectCreateOffsetToFace(
			morphedPos,
			targetFaceTriangles,
			createMorphEyeHideFaceFrontZOffset,
		); ok {
			offset.Z = projectedZ
		}
		if isZeroMorphDelta(offset) {
			continue
		}
		offsetsByVertex[vertexIndex] = offsetsByVertex[vertexIndex].Added(offset)
	}
	return buildMergedVertexOffsets(offsetsByVertex)
}

// resolveCreateEyeSurprisedOffsets は 瞳小/瞳大 用基準オフセットを返す。
func resolveCreateEyeSurprisedOffsets(modelData *model.PmxModel, morphName string) map[int]mmath.Vec3 {
	candidates := []string{"びっくり"}
	if isCreateMorphRight(morphName) {
		candidates = []string{"びっくり右", "びっくり"}
	}
	if isCreateMorphLeft(morphName) {
		candidates = []string{"びっくり左", "びっくり"}
	}
	offsets := collectMorphVertexOffsetsByNames(modelData, candidates)
	return filterCreateOffsetsBySide(modelData, offsets, morphName)
}

// collectMorphVertexOffsetsByNames は候補モーフ名の先頭一致オフセットを返す。
func collectMorphVertexOffsetsByNames(modelData *model.PmxModel, candidates []string) map[int]mmath.Vec3 {
	for _, candidate := range candidates {
		offsets := collectMorphVertexOffsetsByName(modelData, candidate)
		if len(offsets) > 0 {
			return offsets
		}
	}
	return map[int]mmath.Vec3{}
}

// collectMorphVertexOffsetsByName はモーフ名から頂点オフセットを展開して返す。
func collectMorphVertexOffsetsByName(modelData *model.PmxModel, morphName string) map[int]mmath.Vec3 {
	if modelData == nil || modelData.Morphs == nil {
		return map[int]mmath.Vec3{}
	}
	normalizedName := strings.TrimSpace(morphName)
	if normalizedName == "" {
		return map[int]mmath.Vec3{}
	}
	morphData, err := modelData.Morphs.GetByName(normalizedName)
	if err != nil || morphData == nil {
		return map[int]mmath.Vec3{}
	}
	return collectMorphVertexOffsets(modelData, morphData.Index())
}

// collectMorphVertexOffsets は頂点/グループモーフを再帰展開して頂点差分を返す。
func collectMorphVertexOffsets(modelData *model.PmxModel, morphIndex int) map[int]mmath.Vec3 {
	offsetsByVertex := map[int]mmath.Vec3{}
	if modelData == nil || modelData.Morphs == nil || morphIndex < 0 {
		return offsetsByVertex
	}
	collectMorphVertexOffsetsRecursive(modelData, morphIndex, 1.0, offsetsByVertex, map[int]struct{}{})
	return offsetsByVertex
}

// collectMorphVertexOffsetsRecursive はモーフ参照を再帰展開して頂点差分を加算する。
func collectMorphVertexOffsetsRecursive(
	modelData *model.PmxModel,
	morphIndex int,
	factor float64,
	offsetsByVertex map[int]mmath.Vec3,
	visitStack map[int]struct{},
) {
	if modelData == nil || modelData.Morphs == nil || morphIndex < 0 || factor == 0 || offsetsByVertex == nil {
		return
	}
	if _, exists := visitStack[morphIndex]; exists {
		return
	}
	morphData, err := modelData.Morphs.Get(morphIndex)
	if err != nil || morphData == nil {
		return
	}
	visitStack[morphIndex] = struct{}{}
	defer delete(visitStack, morphIndex)

	switch morphData.MorphType {
	case model.MORPH_TYPE_VERTEX:
		for _, rawOffset := range morphData.Offsets {
			offsetData, ok := rawOffset.(*model.VertexMorphOffset)
			if !ok || offsetData == nil || offsetData.VertexIndex < 0 {
				continue
			}
			offsetsByVertex[offsetData.VertexIndex] = offsetsByVertex[offsetData.VertexIndex].Added(
				offsetData.Position.MuledScalar(factor),
			)
		}
	case model.MORPH_TYPE_GROUP:
		for _, rawOffset := range morphData.Offsets {
			offsetData, ok := rawOffset.(*model.GroupMorphOffset)
			if !ok || offsetData == nil || offsetData.MorphIndex < 0 {
				continue
			}
			collectMorphVertexOffsetsRecursive(
				modelData,
				offsetData.MorphIndex,
				factor*offsetData.MorphFactor,
				offsetsByVertex,
				visitStack,
			)
		}
	default:
	}
}

// buildCreateMorphSemanticVertexSets はモーフ名セマンティクスごとの頂点集合を返す。
func buildCreateMorphSemanticVertexSets(modelData *model.PmxModel) map[string]map[int]struct{} {
	semanticVertexSets := map[string]map[int]struct{}{}
	if modelData == nil || modelData.Morphs == nil {
		return semanticVertexSets
	}
	for _, morphData := range modelData.Morphs.Values() {
		if morphData == nil {
			continue
		}
		tags := classifyCreateSemanticTags(morphData.Name())
		if len(tags) == 0 {
			continue
		}
		offsets := collectMorphVertexOffsets(modelData, morphData.Index())
		if len(offsets) == 0 {
			continue
		}
		for _, tag := range tags {
			if _, exists := semanticVertexSets[tag]; !exists {
				semanticVertexSets[tag] = map[int]struct{}{}
			}
			for vertexIndex := range offsets {
				semanticVertexSets[tag][vertexIndex] = struct{}{}
			}
		}
	}
	return semanticVertexSets
}

// buildCreateMaterialSemanticVertexSets は材質名セマンティクスごとの頂点集合を返す。
func buildCreateMaterialSemanticVertexSets(
	modelData *model.PmxModel,
	materialVertexMap map[int][]int,
) map[string]map[int]struct{} {
	semanticVertexSets := map[string]map[int]struct{}{}
	if modelData == nil || modelData.Materials == nil || len(materialVertexMap) == 0 {
		return semanticVertexSets
	}
	for materialIndex := 0; materialIndex < modelData.Materials.Len(); materialIndex++ {
		vertexIndexes := materialVertexMap[materialIndex]
		if len(vertexIndexes) == 0 {
			continue
		}
		materialData, err := modelData.Materials.Get(materialIndex)
		if err != nil || materialData == nil {
			continue
		}
		joinedName := strings.TrimSpace(materialData.Name())
		if strings.TrimSpace(materialData.EnglishName) != "" {
			joinedName = strings.TrimSpace(joinedName + " " + materialData.EnglishName)
		}
		tags := classifyCreateSemanticTags(joinedName)
		for _, tag := range tags {
			if _, exists := semanticVertexSets[tag]; !exists {
				semanticVertexSets[tag] = map[int]struct{}{}
			}
			for _, vertexIndex := range vertexIndexes {
				semanticVertexSets[tag][vertexIndex] = struct{}{}
			}
		}
	}
	return semanticVertexSets
}

// buildCreateFaceTriangles は顔面三角形と閉眼近似三角形を構築する。
func buildCreateFaceTriangles(
	modelData *model.PmxModel,
	registry *targetMorphRegistry,
	closeOffsets map[int]mmath.Vec3,
) ([]createFaceTriangle, []createFaceTriangle, []createFaceTriangle) {
	if modelData == nil || modelData.Materials == nil || modelData.Faces == nil || modelData.Vertices == nil {
		return nil, nil, nil
	}
	faceMaterialIndexes := resolveCreateFaceMaterialIndexes(modelData, registry)
	if len(faceMaterialIndexes) == 0 {
		return nil, nil, nil
	}
	faceRanges := buildCreateMaterialFaceRanges(modelData)
	openFaceTriangles := make([]createFaceTriangle, 0)
	leftClosedFaceTriangles := make([]createFaceTriangle, 0)
	rightClosedFaceTriangles := make([]createFaceTriangle, 0)

	for _, materialIndex := range faceMaterialIndexes {
		if materialIndex < 0 || materialIndex >= len(faceRanges) {
			continue
		}
		faceRange := faceRanges[materialIndex]
		for faceIndex := faceRange.Start; faceIndex < faceRange.End && faceIndex < modelData.Faces.Len(); faceIndex++ {
			faceData, err := modelData.Faces.Get(faceIndex)
			if err != nil || faceData == nil {
				continue
			}
			v0, err0 := modelData.Vertices.Get(faceData.VertexIndexes[0])
			v1, err1 := modelData.Vertices.Get(faceData.VertexIndexes[1])
			v2, err2 := modelData.Vertices.Get(faceData.VertexIndexes[2])
			if err0 != nil || err1 != nil || err2 != nil || v0 == nil || v1 == nil || v2 == nil {
				continue
			}
			openFaceTriangles = append(openFaceTriangles, newCreateFaceTriangle(v0.Position, v1.Position, v2.Position))

			closedV0 := v0.Position.Added(closeOffsets[v0.Index()])
			closedV1 := v1.Position.Added(closeOffsets[v1.Index()])
			closedV2 := v2.Position.Added(closeOffsets[v2.Index()])
			closedTriangle := newCreateFaceTriangle(closedV0, closedV1, closedV2)
			if closedTriangle.Center.X >= 0 {
				leftClosedFaceTriangles = append(leftClosedFaceTriangles, closedTriangle)
			} else {
				rightClosedFaceTriangles = append(rightClosedFaceTriangles, closedTriangle)
			}
		}
	}
	return openFaceTriangles, leftClosedFaceTriangles, rightClosedFaceTriangles
}

// resolveCreateFaceMaterialIndexes は顔面射影対象の材質indexを返す。
func resolveCreateFaceMaterialIndexes(modelData *model.PmxModel, registry *targetMorphRegistry) []int {
	_ = registry
	if modelData == nil || modelData.Materials == nil {
		return nil
	}
	primary := []int{}
	secondary := []int{}
	fallback := []int{}
	for materialIndex := 0; materialIndex < modelData.Materials.Len(); materialIndex++ {
		materialData, err := modelData.Materials.Get(materialIndex)
		if err != nil || materialData == nil {
			continue
		}
		joinedName := strings.TrimSpace(materialData.Name())
		if strings.TrimSpace(materialData.EnglishName) != "" {
			joinedName = strings.TrimSpace(joinedName + " " + materialData.EnglishName)
		}
		tags := classifyCreateSemanticTags(joinedName)
		hasFace := containsCreateSemantic(tags, createSemanticFace)
		if !hasFace {
			continue
		}
		fallback = append(fallback, materialIndex)
		if containsCreateSemantic(tags, createSemanticSkin) {
			primary = append(primary, materialIndex)
			continue
		}
		if !containsCreateSemantic(tags, createSemanticBrow) &&
			!containsCreateSemantic(tags, createSemanticIris) &&
			!containsCreateSemantic(tags, createSemanticHighlight) &&
			!containsCreateSemantic(tags, createSemanticEyeWhite) &&
			!containsCreateSemantic(tags, createSemanticEyeLine) &&
			!containsCreateSemantic(tags, createSemanticEyeLash) {
			secondary = append(secondary, materialIndex)
		}
	}
	if len(primary) > 0 {
		return primary
	}
	if len(secondary) > 0 {
		return secondary
	}
	return fallback
}

// createMaterialFaceRange は材質ごとの面index範囲を表す。
type createMaterialFaceRange struct {
	Start int
	End   int
}

// buildCreateMaterialFaceRanges は材質ごとの面範囲を返す。
func buildCreateMaterialFaceRanges(modelData *model.PmxModel) []createMaterialFaceRange {
	if modelData == nil || modelData.Materials == nil {
		return nil
	}
	faceRanges := make([]createMaterialFaceRange, modelData.Materials.Len())
	faceStart := 0
	for materialIndex := 0; materialIndex < modelData.Materials.Len(); materialIndex++ {
		materialData, err := modelData.Materials.Get(materialIndex)
		faceCount := 0
		if err == nil && materialData != nil && materialData.VerticesCount > 0 {
			faceCount = materialData.VerticesCount / 3
		}
		faceRanges[materialIndex] = createMaterialFaceRange{
			Start: faceStart,
			End:   faceStart + faceCount,
		}
		faceStart += faceCount
	}
	return faceRanges
}

// newCreateFaceTriangle は射影計算用三角形を生成する。
func newCreateFaceTriangle(v0 mmath.Vec3, v1 mmath.Vec3, v2 mmath.Vec3) createFaceTriangle {
	return createFaceTriangle{
		V0: v0,
		V1: v1,
		V2: v2,
		Center: mmath.Vec3{
			Vec: r3.Vec{
				X: (v0.X + v1.X + v2.X) / 3.0,
				Y: (v0.Y + v1.Y + v2.Y) / 3.0,
				Z: (v0.Z + v1.Z + v2.Z) / 3.0,
			},
		},
	}
}

// projectCreateOffsetToFace は射影後のZオフセットを返す。
func projectCreateOffsetToFace(
	morphedPos mmath.Vec3,
	faceTriangles []createFaceTriangle,
	zOffset float64,
) (float64, bool) {
	nearestFace, exists := findNearestCreateFaceTriangleByXY(faceTriangles, morphedPos)
	if !exists {
		return 0, false
	}
	near := morphedPos.Added(mmath.Vec3{Vec: r3.Vec{Z: -createMorphProjectionLineHalfDistance}})
	far := morphedPos.Added(mmath.Vec3{Vec: r3.Vec{Z: createMorphProjectionLineHalfDistance}})
	forward := nearestFace.V1.Subed(nearestFace.V0)
	right := nearestFace.V2.Subed(nearestFace.V1)
	intersect, err := mmath.IntersectLinePlane(near, far, forward, right, mmath.ZERO_VEC3, nearestFace.Center)
	if err != nil {
		return 0, false
	}
	return intersect.Z - morphedPos.Z - zOffset, true
}

// findNearestCreateFaceTriangleByXY はXY距離が最小の三角形を返す。
func findNearestCreateFaceTriangleByXY(faceTriangles []createFaceTriangle, target mmath.Vec3) (createFaceTriangle, bool) {
	if len(faceTriangles) == 0 {
		return createFaceTriangle{}, false
	}
	bestIndex := -1
	bestDistance := math.MaxFloat64
	for triangleIndex, faceTriangle := range faceTriangles {
		dx0 := faceTriangle.V0.X - target.X
		dy0 := faceTriangle.V0.Y - target.Y
		dx1 := faceTriangle.V1.X - target.X
		dy1 := faceTriangle.V1.Y - target.Y
		dx2 := faceTriangle.V2.X - target.X
		dy2 := faceTriangle.V2.Y - target.Y
		score := (dx0 * dx0) + (dy0 * dy0) + (dx1 * dx1) + (dy1 * dy1) + (dx2 * dx2) + (dy2 * dy2)
		if score < bestDistance {
			bestDistance = score
			bestIndex = triangleIndex
		}
	}
	if bestIndex < 0 {
		return createFaceTriangle{}, false
	}
	return faceTriangles[bestIndex], true
}

// resolveCreateRuleSemantics は creates 指定からセマンティクス一覧を返す。
func resolveCreateRuleSemantics(creates []string) []string {
	semanticSet := map[string]struct{}{}
	for _, createName := range creates {
		normalized := normalizeCreateSemanticName(createName)
		switch {
		case strings.Contains(normalized, "facebrow"):
			semanticSet[createSemanticBrow] = struct{}{}
		case strings.Contains(normalized, "eyeiris"):
			semanticSet[createSemanticIris] = struct{}{}
		case strings.Contains(normalized, "eyehighlight"):
			semanticSet[createSemanticHighlight] = struct{}{}
		case strings.Contains(normalized, "eyewhite"):
			semanticSet[createSemanticEyeWhite] = struct{}{}
		default:
			for _, semantic := range classifyCreateSemanticTags(createName) {
				switch semantic {
				case createSemanticBrow, createSemanticIris, createSemanticHighlight, createSemanticEyeWhite:
					semanticSet[semantic] = struct{}{}
				}
			}
		}
	}
	semantics := make([]string, 0, len(semanticSet))
	for semantic := range semanticSet {
		semantics = append(semantics, semantic)
	}
	sort.Strings(semantics)
	return semantics
}

// resolveCreateHideSemantics は hides 指定からセマンティクス一覧を返す。
func resolveCreateHideSemantics(hides []string) []string {
	semanticSet := map[string]struct{}{}
	for _, hideName := range hides {
		normalized := normalizeCreateSemanticName(hideName)
		switch {
		case strings.Contains(normalized, "eyeline"):
			semanticSet[createSemanticEyeLine] = struct{}{}
		case strings.Contains(normalized, "eyelash"):
			semanticSet[createSemanticEyeLash] = struct{}{}
		default:
			for _, semantic := range classifyCreateSemanticTags(hideName) {
				if semantic == createSemanticEyeLine || semantic == createSemanticEyeLash {
					semanticSet[semantic] = struct{}{}
				}
			}
		}
	}
	semantics := make([]string, 0, len(semanticSet))
	for semantic := range semanticSet {
		semantics = append(semantics, semantic)
	}
	sort.Strings(semantics)
	return semantics
}

// resolveCreateSemanticVertexSet はモーフ優先でセマンティクス頂点集合を返す。
func resolveCreateSemanticVertexSet(
	semantic string,
	morphSemanticVertexSets map[string]map[int]struct{},
	materialSemanticVertexSets map[string]map[int]struct{},
) map[int]struct{} {
	if semantic == "" {
		return map[int]struct{}{}
	}
	if vertices := morphSemanticVertexSets[semantic]; len(vertices) > 0 {
		return vertices
	}
	if vertices := materialSemanticVertexSets[semantic]; len(vertices) > 0 {
		return vertices
	}
	return map[int]struct{}{}
}

// filterCreateVertexSetBySide はモーフ名の左右接尾辞に従って頂点集合を絞る。
func filterCreateVertexSetBySide(
	modelData *model.PmxModel,
	vertexSet map[int]struct{},
	morphName string,
) map[int]struct{} {
	filtered := map[int]struct{}{}
	if len(vertexSet) == 0 {
		return filtered
	}
	for vertexIndex := range vertexSet {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		if !isCreateVertexInMorphSide(vertex.Position, morphName) {
			continue
		}
		filtered[vertexIndex] = struct{}{}
	}
	return filtered
}

// filterCreateOffsetsBySide はモーフ名の左右接尾辞に従ってオフセット集合を絞る。
func filterCreateOffsetsBySide(
	modelData *model.PmxModel,
	offsets map[int]mmath.Vec3,
	morphName string,
) map[int]mmath.Vec3 {
	filtered := map[int]mmath.Vec3{}
	if len(offsets) == 0 {
		return filtered
	}
	for vertexIndex, offset := range offsets {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		if !isCreateVertexInMorphSide(vertex.Position, morphName) {
			continue
		}
		filtered[vertexIndex] = offset
	}
	return filtered
}

// isCreateVertexInMorphSide は左右接尾辞に対応する頂点か判定する。
func isCreateVertexInMorphSide(position mmath.Vec3, morphName string) bool {
	if isCreateMorphRight(morphName) {
		return position.X < 0
	}
	if isCreateMorphLeft(morphName) {
		return position.X > 0
	}
	return true
}

// isCreateMorphRight はcreate名が右側モーフかを返す。
func isCreateMorphRight(morphName string) bool {
	return strings.HasSuffix(morphName, "右")
}

// isCreateMorphLeft はcreate名が左側モーフかを返す。
func isCreateMorphLeft(morphName string) bool {
	return strings.HasSuffix(morphName, "左")
}

// sortedCreateVertexIndexes は頂点集合を昇順index配列へ変換する。
func sortedCreateVertexIndexes(vertexSet map[int]struct{}) []int {
	if len(vertexSet) == 0 {
		return nil
	}
	vertexIndexes := make([]int, 0, len(vertexSet))
	for vertexIndex := range vertexSet {
		vertexIndexes = append(vertexIndexes, vertexIndex)
	}
	sort.Ints(vertexIndexes)
	return vertexIndexes
}

// calcCreateVertexSetMean は頂点集合の重心を返す。
func calcCreateVertexSetMean(modelData *model.PmxModel, vertexSet map[int]struct{}) (mmath.Vec3, bool) {
	if modelData == nil || modelData.Vertices == nil || len(vertexSet) == 0 {
		return mmath.ZERO_VEC3, false
	}
	sum := mmath.ZERO_VEC3
	count := 0
	for vertexIndex := range vertexSet {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		sum = sum.Added(vertex.Position)
		count++
	}
	if count == 0 {
		return mmath.ZERO_VEC3, false
	}
	return sum.DivedScalar(float64(count)), true
}

// createVertexStats は頂点集合の統計値を表す。
type createVertexStats struct {
	Count int
	Sum   mmath.Vec3
	Min   mmath.Vec3
	Max   mmath.Vec3
}

// newCreateVertexStats は頂点統計の初期値を返す。
func newCreateVertexStats() createVertexStats {
	return createVertexStats{
		Count: 0,
		Sum:   mmath.ZERO_VEC3,
		Min:   mmath.VEC3_MAX_VAL,
		Max:   mmath.VEC3_MIN_VAL,
	}
}

// Add は頂点位置を統計へ加算する。
func (s *createVertexStats) Add(position mmath.Vec3) {
	if s == nil {
		return
	}
	s.Count++
	s.Sum = s.Sum.Added(position)
	s.Min = mmath.Vec3{
		Vec: r3.Vec{
			X: math.Min(s.Min.X, position.X),
			Y: math.Min(s.Min.Y, position.Y),
			Z: math.Min(s.Min.Z, position.Z),
		},
	}
	s.Max = mmath.Vec3{
		Vec: r3.Vec{
			X: math.Max(s.Max.X, position.X),
			Y: math.Max(s.Max.Y, position.Y),
			Z: math.Max(s.Max.Z, position.Z),
		},
	}
}

// Mean は統計対象の重心を返す。
func (s createVertexStats) Mean() (mmath.Vec3, bool) {
	if s.Count == 0 {
		return mmath.ZERO_VEC3, false
	}
	return s.Sum.DivedScalar(float64(s.Count)), true
}

// containsCreateSemantic はタグ配列に対象タグが含まれるか判定する。
func containsCreateSemantic(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

// classifyCreateSemanticTags は名前文字列から creates 用セマンティクスタグを抽出する。
func classifyCreateSemanticTags(name string) []string {
	normalized := normalizeCreateSemanticName(name)
	if normalized == "" {
		return nil
	}
	tagSet := map[string]struct{}{}
	if strings.Contains(normalized, "brow") || strings.Contains(normalized, "eyebrow") {
		tagSet[createSemanticBrow] = struct{}{}
	}
	if strings.Contains(normalized, "iris") || strings.Contains(normalized, "pupil") {
		tagSet[createSemanticIris] = struct{}{}
	}
	if strings.Contains(normalized, "highlight") {
		tagSet[createSemanticHighlight] = struct{}{}
	}
	if strings.Contains(normalized, "eyewhite") ||
		strings.Contains(normalized, "sclera") ||
		strings.Contains(normalized, "irishide") {
		tagSet[createSemanticEyeWhite] = struct{}{}
	}
	if strings.Contains(normalized, "eyeline") {
		tagSet[createSemanticEyeLine] = struct{}{}
	}
	if strings.Contains(normalized, "eyelash") || strings.Contains(normalized, "lash") {
		tagSet[createSemanticEyeLash] = struct{}{}
	}
	if strings.Contains(normalized, "face") {
		tagSet[createSemanticFace] = struct{}{}
	}
	if strings.Contains(normalized, "skin") {
		tagSet[createSemanticSkin] = struct{}{}
	}
	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

// normalizeCreateSemanticName はASCII英数字のみへ正規化する。
func normalizeCreateSemanticName(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	builder := strings.Builder{}
	for _, r := range strings.ToLower(value) {
		if ('a' <= r && r <= 'z') || ('0' <= r && r <= '9') {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

// appendUniqueInt は未登録の値だけを追加して返す。
func appendUniqueInt(values []int, target int) []int {
	for _, value := range values {
		if value == target {
			return values
		}
	}
	return append(values, target)
}

// sortedStringKeys は map のキーを昇順で返す。
func sortedStringKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// appendOrReusePrimitiveVertices はprimitive頂点を生成または既存頂点を再利用する。
func appendOrReusePrimitiveVertices(
	modelData *model.PmxModel,
	doc *gltfDocument,
	nodeIndex int,
	node gltfNode,
	primitive gltfPrimitive,
	positions [][]float64,
	normals [][]float64,
	uvs [][]float64,
	joints [][]int,
	weights [][]float64,
	nodeToBoneIndex map[int]int,
	conversion vrmConversion,
	cache *accessorValueCache,
) (int, int) {
	vertexKey := primitiveVertexKey(nodeIndex, primitive)
	if cachedStart, ok := cache.getVertexRange(vertexKey, len(positions)); ok {
		return cachedStart, 0
	}

	defaultBoneIndex := resolveDefaultBoneIndex(modelData, nodeToBoneIndex, nodeIndex)
	skinJoints := resolveSkinJoints(doc, node)
	vertexStart := modelData.Vertices.Len()
	for vertexIndex := 0; vertexIndex < len(positions); vertexIndex++ {
		position := toVec3(positions[vertexIndex], mmath.ZERO_VEC3)
		normal := mmath.Vec3{Vec: r3.Vec{X: 0, Y: 1, Z: 0}}
		if vertexIndex < len(normals) {
			normal = toVec3(normals[vertexIndex], normal)
		}
		uv := mmath.ZERO_VEC2
		if vertexIndex < len(uvs) {
			uv = toVec2(uvs[vertexIndex], mmath.ZERO_VEC2)
		}

		jointValues := []int{}
		if vertexIndex < len(joints) {
			jointValues = joints[vertexIndex]
		}
		weightValues := []float64{}
		if vertexIndex < len(weights) {
			weightValues = weights[vertexIndex]
		}
		deform := buildVertexDeform(defaultBoneIndex, jointValues, weightValues, skinJoints, nodeToBoneIndex)
		vertex := &model.Vertex{
			Position:        convertVrmPositionToPmx(position, conversion),
			Normal:          convertVrmNormalToPmx(normal, conversion),
			Uv:              uv,
			ExtendedUvs:     []mmath.Vec4{},
			DeformType:      deform.DeformType(),
			Deform:          deform,
			EdgeFactor:      1.0,
			MaterialIndexes: []int{},
		}
		modelData.Vertices.AppendRaw(vertex)
	}
	cache.setVertexRange(vertexKey, vertexStart, len(positions))
	return vertexStart, len(positions)
}

// appendVertexMaterialIndex は頂点に材質indexを追加する。
func appendVertexMaterialIndex(modelData *model.PmxModel, vertexIndex int, materialIndex int) {
	if modelData == nil || modelData.Vertices == nil || materialIndex < 0 {
		return
	}
	vertex, err := modelData.Vertices.Get(vertexIndex)
	if err != nil || vertex == nil {
		return
	}
	vertex.MaterialIndexes = append(vertex.MaterialIndexes, materialIndex)
}

// resolveDefaultBoneIndex は頂点デフォーム未解決時に使う既定ボーンindexを返す。
func resolveDefaultBoneIndex(modelData *model.PmxModel, nodeToBoneIndex map[int]int, nodeIndex int) int {
	if idx, ok := nodeToBoneIndex[nodeIndex]; ok && idx >= 0 {
		return idx
	}
	if modelData != nil && modelData.Bones != nil && modelData.Bones.Len() > 0 {
		return 0
	}
	return 0
}

// resolveSkinJoints はnode.skin からjoint node index配列を返す。
func resolveSkinJoints(doc *gltfDocument, node gltfNode) []int {
	if node.Skin == nil || doc == nil {
		return nil
	}
	skinIndex := *node.Skin
	if skinIndex < 0 || skinIndex >= len(doc.Skins) {
		return nil
	}
	return doc.Skins[skinIndex].Joints
}

// buildVertexDeform はJOINTS/WEIGHTSからPMXデフォームを生成する。
func buildVertexDeform(
	defaultBoneIndex int,
	joints []int,
	weights []float64,
	skinJoints []int,
	nodeToBoneIndex map[int]int,
) model.IDeform {
	weightByBone := map[int]float64{}
	maxCount := len(joints)
	if len(weights) < maxCount {
		maxCount = len(weights)
	}
	for i := 0; i < maxCount; i++ {
		weight := weights[i]
		if weight <= 0 {
			continue
		}
		jointIndex := joints[i]
		if jointIndex < 0 {
			continue
		}

		nodeIndex := jointIndex
		if len(skinJoints) > 0 {
			if jointIndex >= len(skinJoints) {
				continue
			}
			nodeIndex = skinJoints[jointIndex]
		}
		boneIndex, ok := nodeToBoneIndex[nodeIndex]
		if !ok || boneIndex < 0 {
			continue
		}
		weightByBone[boneIndex] += weight
	}

	if len(weightByBone) == 0 {
		return model.NewBdef1(defaultBoneIndex)
	}

	weightedBones := make([]weightedBone, 0, len(weightByBone))
	totalWeight := 0.0
	for boneIndex, weight := range weightByBone {
		if weight <= 0 {
			continue
		}
		weightedBones = append(weightedBones, weightedBone{
			BoneIndex: boneIndex,
			Weight:    weight,
		})
		totalWeight += weight
	}
	if totalWeight <= 0 || len(weightedBones) == 0 {
		return model.NewBdef1(defaultBoneIndex)
	}

	sort.Slice(weightedBones, func(i int, j int) bool {
		if weightedBones[i].Weight == weightedBones[j].Weight {
			return weightedBones[i].BoneIndex < weightedBones[j].BoneIndex
		}
		return weightedBones[i].Weight > weightedBones[j].Weight
	})

	if len(weightedBones) == 1 {
		return model.NewBdef1(weightedBones[0].BoneIndex)
	}

	if len(weightedBones) == 2 {
		weight0 := weightedBones[0].Weight / (weightedBones[0].Weight + weightedBones[1].Weight)
		return model.NewBdef2(weightedBones[0].BoneIndex, weightedBones[1].BoneIndex, weight0)
	}

	if len(weightedBones) > 4 {
		weightedBones = weightedBones[:4]
	}
	totalTopWeight := 0.0
	for _, wb := range weightedBones {
		totalTopWeight += wb.Weight
	}
	if totalTopWeight <= 0 {
		return model.NewBdef1(defaultBoneIndex)
	}

	indexes := [4]int{defaultBoneIndex, defaultBoneIndex, defaultBoneIndex, defaultBoneIndex}
	values := [4]float64{0, 0, 0, 0}
	for i := 0; i < len(weightedBones) && i < 4; i++ {
		indexes[i] = weightedBones[i].BoneIndex
		values[i] = weightedBones[i].Weight / totalTopWeight
	}
	return model.NewBdef4(indexes, values)
}

// appendPrimitiveMaterial はprimitive用材質を追加し、そのindexを返す。
func appendPrimitiveMaterial(
	modelData *model.PmxModel,
	doc *gltfDocument,
	primitive gltfPrimitive,
	primitiveName string,
	textureIndexesByImage []int,
	verticesCount int,
	registry *targetMorphRegistry,
) int {
	material := model.NewMaterial()
	material.SetName(primitiveName)
	material.EnglishName = primitiveName
	material.Memo = buildPrimitiveMaterialMemo("")
	material.Diffuse = mmath.Vec4{X: 1.0, Y: 1.0, Z: 1.0, W: 1.0}
	material.Specular = mmath.ZERO_VEC4
	material.Ambient = mmath.Vec3{Vec: r3.Vec{X: 0.5, Y: 0.5, Z: 0.5}}
	material.Edge = mmath.Vec4{X: 0.0, Y: 0.0, Z: 0.0, W: 1.0}
	material.EdgeSize = 1.0
	material.TextureFactor = mmath.ONE_VEC4
	material.SphereTextureFactor = mmath.ONE_VEC4
	material.ToonTextureFactor = mmath.ONE_VEC4
	material.DrawFlag = model.DRAW_FLAG_GROUND_SHADOW | model.DRAW_FLAG_DRAWING_ON_SELF_SHADOW_MAPS | model.DRAW_FLAG_DRAWING_SELF_SHADOWS
	material.VerticesCount = verticesCount
	material.TextureIndex = resolveMaterialTextureIndex(doc, primitive, textureIndexesByImage)
	material.SphereTextureIndex = 0
	material.SphereMode = model.SPHERE_MODE_INVALID
	alphaMode := ""
	sourceMaterialIndex := -1
	sourceMaterial := gltfMaterial{}
	hasSourceMaterial := false

	if primitive.Material != nil {
		sourceMaterialIndex = *primitive.Material
		if sourceMaterialIndex >= 0 && sourceMaterialIndex < len(doc.Materials) {
			sourceMaterial = doc.Materials[sourceMaterialIndex]
			hasSourceMaterial = true
			alphaMode = sourceMaterial.AlphaMode
			material.Memo = buildPrimitiveMaterialMemo(sourceMaterial.AlphaMode)
			if sourceMaterial.Name != "" {
				material.SetName(sourceMaterial.Name)
				material.EnglishName = sourceMaterial.Name
			}
			if sourceMaterial.DoubleSided {
				material.DrawFlag |= model.DRAW_FLAG_DOUBLE_SIDED_DRAWING
			}
			if len(sourceMaterial.PbrMetallicRoughness.BaseColorFactor) == 4 {
				material.Diffuse = mmath.Vec4{
					X: sourceMaterial.PbrMetallicRoughness.BaseColorFactor[0],
					Y: sourceMaterial.PbrMetallicRoughness.BaseColorFactor[1],
					Z: sourceMaterial.PbrMetallicRoughness.BaseColorFactor[2],
					W: sourceMaterial.PbrMetallicRoughness.BaseColorFactor[3],
				}
			}
			material.Edge, material.EdgeSize = resolvePrimitiveMaterialEdgeSettings(
				doc,
				sourceMaterial,
				sourceMaterialIndex,
				material.Edge,
				material.EdgeSize,
			)
		}
	}
	if shouldEnablePrimitiveMaterialEdge(alphaMode, material.Name(), material.EnglishName, material.EdgeSize, material.TextureIndex) {
		material.DrawFlag |= model.DRAW_FLAG_DRAWING_EDGE
	} else {
		material.DrawFlag &^= model.DRAW_FLAG_DRAWING_EDGE
	}
	if shouldApplyLegacyVroidMaterialConversion(modelData) {
		resolveLegacyVroidToonTexture(
			modelData,
			doc,
			sourceMaterial,
			sourceMaterialIndex,
			hasSourceMaterial,
			material,
		)
		resolveLegacyVroidSphereTexture(
			modelData,
			doc,
			sourceMaterial,
			sourceMaterialIndex,
			hasSourceMaterial,
			textureIndexesByImage,
			material,
		)
	}

	materialIndex := modelData.Materials.AppendRaw(material)
	registerExpressionMaterialIndex(registry, doc, primitive.Material, material.Name(), materialIndex)
	return materialIndex
}

// shouldApplyLegacyVroidMaterialConversion は旧VRoid材質変換(ton/sphere)を適用するか判定する。
func shouldApplyLegacyVroidMaterialConversion(modelData *model.PmxModel) bool {
	if modelData == nil || modelData.VrmData == nil {
		return false
	}
	return modelData.VrmData.Profile == vrm.VRM_PROFILE_VROID
}

// resolveLegacyVroidToonTexture は旧仕様の toon 生成/フォールバックを適用する。
func resolveLegacyVroidToonTexture(
	modelData *model.PmxModel,
	doc *gltfDocument,
	sourceMaterial gltfMaterial,
	sourceMaterialIndex int,
	hasSourceMaterial bool,
	materialData *model.Material,
) {
	if materialData == nil {
		return
	}
	if !hasSourceMaterial {
		applyLegacySharedToonFallback(materialData)
		recordLegacyMaterialWarning(
			modelData,
			warningid.VrmWarningToonTextureGenerationFailed,
			"toon生成フォールバック: material=%s reason=%s",
			strings.TrimSpace(materialData.Name()),
			"source material not found",
		)
		return
	}

	shadeColor, hasShadeColor, shadeErr := resolveLegacyToonShadeColor(doc, sourceMaterial, sourceMaterialIndex)
	if shadeErr != nil || !hasShadeColor {
		applyLegacySharedToonFallback(materialData)
		reason := "shade color missing"
		if shadeErr != nil {
			reason = shadeErr.Error()
		}
		recordLegacyMaterialWarning(
			modelData,
			warningid.VrmWarningToonTextureGenerationFailed,
			"toon生成フォールバック: material=%s reason=%s",
			strings.TrimSpace(materialData.Name()),
			reason,
		)
		return
	}

	if _, err := buildLegacyToonBmp32(shadeColor); err != nil {
		applyLegacySharedToonFallback(materialData)
		recordLegacyMaterialWarning(
			modelData,
			warningid.VrmWarningToonTextureGenerationFailed,
			"toon生成フォールバック: material=%s reason=%s",
			strings.TrimSpace(materialData.Name()),
			err.Error(),
		)
		return
	}

	normalizedMaterialIndex := normalizeLegacyGeneratedTextureMaterialIndex(sourceMaterialIndex)
	toonFileName := fmt.Sprintf(
		"toon%02d.bmp",
		normalizedMaterialIndex+1,
	)
	toonTextureName := filepath.ToSlash(filepath.Join("tex", toonFileName))
	toonTextureIndex, err := ensureGeneratedTextureIndex(modelData, toonTextureName, model.TEXTURE_TYPE_TOON)
	if err != nil {
		applyLegacySharedToonFallback(materialData)
		recordLegacyMaterialWarning(
			modelData,
			warningid.VrmWarningToonTextureGenerationFailed,
			"toon生成フォールバック: material=%s reason=%s",
			strings.TrimSpace(materialData.Name()),
			err.Error(),
		)
		return
	}

	materialData.ToonSharingFlag = model.TOON_SHARING_INDIVIDUAL
	materialData.ToonTextureIndex = toonTextureIndex
}

type legacySphereCandidateType int

const (
	legacySphereCandidateSphereAdd legacySphereCandidateType = iota
	legacySphereCandidateHair
	legacySphereCandidateMatcap
	legacySphereCandidateEmissive
)

// resolveLegacyVroidSphereTexture は旧仕様の sphere 優先順位とフォールバックを適用する。
func resolveLegacyVroidSphereTexture(
	modelData *model.PmxModel,
	doc *gltfDocument,
	sourceMaterial gltfMaterial,
	sourceMaterialIndex int,
	hasSourceMaterial bool,
	textureIndexesByImage []int,
	materialData *model.Material,
) {
	if materialData == nil {
		return
	}
	if !hasSourceMaterial {
		materialData.SphereTextureIndex = 0
		materialData.SphereMode = model.SPHERE_MODE_INVALID
		return
	}

	materialData.SphereTextureIndex = 0
	materialData.SphereMode = model.SPHERE_MODE_INVALID

	isHairMaterial := resolvePrimitiveMaterialKind(materialData.Name(), materialData.EnglishName) == primitiveMaterialKindHair
	hasEmissiveInput := hasLegacyEmissiveInput(sourceMaterial, doc, textureIndexesByImage)
	candidates := legacySphereCandidatesByPriority(isHairMaterial)

	for _, candidate := range candidates {
		sphereTextureIndex, resolved, generationFailed := resolveLegacySphereTextureCandidate(
			modelData,
			doc,
			sourceMaterial,
			sourceMaterialIndex,
			textureIndexesByImage,
			materialData,
			candidate,
		)
		if !resolved {
			if generationFailed {
				recordLegacyMaterialWarning(
					modelData,
					warningid.VrmWarningSphereTextureGenerationFailed,
					"sphere候補不採用: material=%s candidate=%s reason=%s",
					strings.TrimSpace(materialData.Name()),
					legacySphereCandidateLabel(candidate),
					"texture generation failed",
				)
			} else {
				recordLegacyMaterialWarning(
					modelData,
					warningid.VrmWarningSphereTextureSourceMissing,
					"sphere候補不採用: material=%s candidate=%s reason=%s",
					strings.TrimSpace(materialData.Name()),
					legacySphereCandidateLabel(candidate),
					"source not found",
				)
			}
			continue
		}

		materialData.SphereTextureIndex = sphereTextureIndex
		materialData.SphereMode = model.SPHERE_MODE_ADDITION
		if hasEmissiveInput && candidate != legacySphereCandidateEmissive {
			recordLegacyMaterialWarning(
				modelData,
				warningid.VrmWarningEmissiveIgnoredBySpherePriority,
				"sphere優先順位でemissive不採用: material=%s selected=%s",
				strings.TrimSpace(materialData.Name()),
				legacySphereCandidateLabel(candidate),
			)
		}
		return
	}
}

// legacySphereCandidatesByPriority は材質種別に応じた sphere 候補優先順位を返す。
func legacySphereCandidatesByPriority(isHairMaterial bool) []legacySphereCandidateType {
	if isHairMaterial {
		return []legacySphereCandidateType{
			legacySphereCandidateSphereAdd,
			legacySphereCandidateHair,
			legacySphereCandidateMatcap,
			legacySphereCandidateEmissive,
		}
	}
	return []legacySphereCandidateType{
		legacySphereCandidateSphereAdd,
		legacySphereCandidateMatcap,
		legacySphereCandidateEmissive,
	}
}

// resolveLegacySphereTextureCandidate は sphere 候補を解決し、採用可否を返す。
func resolveLegacySphereTextureCandidate(
	modelData *model.PmxModel,
	doc *gltfDocument,
	sourceMaterial gltfMaterial,
	sourceMaterialIndex int,
	textureIndexesByImage []int,
	materialData *model.Material,
	candidate legacySphereCandidateType,
) (int, bool, bool) {
	switch candidate {
	case legacySphereCandidateSphereAdd:
		sphereAddTextureIndex, ok := resolveLegacySphereAddTextureIndex(
			doc,
			sourceMaterial,
			sourceMaterialIndex,
			textureIndexesByImage,
		)
		if !ok {
			return -1, false, false
		}
		return sphereAddTextureIndex, true, false
	case legacySphereCandidateHair:
		if materialData == nil || materialData.TextureIndex < 0 {
			return -1, false, false
		}
		normalizedMaterialIndex := normalizeLegacyGeneratedTextureMaterialIndex(sourceMaterialIndex)
		hairSphereTextureName := filepath.ToSlash(
			filepath.Join("tex", legacyGeneratedSphereDirName, fmt.Sprintf("hair_sphere_%03d.png", normalizedMaterialIndex)),
		)
		textureIndex, err := ensureGeneratedTextureIndex(modelData, hairSphereTextureName, model.TEXTURE_TYPE_SPHERE)
		if err != nil {
			return -1, false, true
		}
		return textureIndex, true, false
	case legacySphereCandidateMatcap:
		if _, ok := resolveLegacyMatcapTextureIndex(sourceMaterial, doc, textureIndexesByImage); !ok {
			return -1, false, false
		}
		normalizedMaterialIndex := normalizeLegacyGeneratedTextureMaterialIndex(sourceMaterialIndex)
		matcapSphereTextureName := filepath.ToSlash(
			filepath.Join("tex", legacyGeneratedSphereDirName, fmt.Sprintf("matcap_sphere_%03d.png", normalizedMaterialIndex)),
		)
		textureIndex, err := ensureGeneratedTextureIndex(modelData, matcapSphereTextureName, model.TEXTURE_TYPE_SPHERE)
		if err != nil {
			return -1, false, true
		}
		return textureIndex, true, false
	case legacySphereCandidateEmissive:
		if !hasLegacyEmissiveInput(sourceMaterial, doc, textureIndexesByImage) {
			return -1, false, false
		}
		normalizedMaterialIndex := normalizeLegacyGeneratedTextureMaterialIndex(sourceMaterialIndex)
		emissiveSphereTextureName := filepath.ToSlash(
			filepath.Join("tex", legacyGeneratedSphereDirName, fmt.Sprintf("emissive_sphere_%03d.png", normalizedMaterialIndex)),
		)
		textureIndex, err := ensureGeneratedTextureIndex(modelData, emissiveSphereTextureName, model.TEXTURE_TYPE_SPHERE)
		if err != nil {
			return -1, false, true
		}
		return textureIndex, true, false
	default:
		return -1, false, false
	}
}

// resolveLegacyToonShadeColor は toon 生成用の shade 色を取得する。
func resolveLegacyToonShadeColor(
	doc *gltfDocument,
	sourceMaterial gltfMaterial,
	sourceMaterialIndex int,
) ([3]uint8, bool, error) {
	if property, ok := resolveVrm0MaterialProperty(doc, sourceMaterial, sourceMaterialIndex); ok {
		if vectorValues, exists := lookupVrm0MaterialVectorProperty(property, "_ShadeColor", "ShadeColor"); exists {
			shadeColor, converted := toLegacyRGBColor(vectorValues)
			if converted {
				return shadeColor, true, nil
			}
		}
	}

	mtoonSource, hasMtoonSource, err := resolveMToonSource(sourceMaterial)
	if err != nil {
		return [3]uint8{}, false, fmt.Errorf("VRMC_materials_mtoon parse failed: %w", err)
	}
	if !hasMtoonSource || len(mtoonSource.ShadeColorFactor) == 0 {
		return [3]uint8{}, false, nil
	}

	shadeColor, converted := toLegacyRGBColor(mtoonSource.ShadeColorFactor)
	return shadeColor, converted, nil
}

// resolveLegacySphereAddTextureIndex は VRM0 _SphereAdd 参照から PMX テクスチャindexを解決する。
func resolveLegacySphereAddTextureIndex(
	doc *gltfDocument,
	sourceMaterial gltfMaterial,
	sourceMaterialIndex int,
	textureIndexesByImage []int,
) (int, bool) {
	property, ok := resolveVrm0MaterialProperty(doc, sourceMaterial, sourceMaterialIndex)
	if !ok {
		return -1, false
	}
	gltfTextureIndex, textureExists := lookupVrm0MaterialTextureProperty(property, "_SphereAdd", "SphereAdd")
	if !textureExists {
		return -1, false
	}
	return resolvePmxTextureIndexFromGltfTextureIndex(doc, textureIndexesByImage, gltfTextureIndex)
}

// resolveLegacyMatcapTextureIndex は matcapTexture 参照から PMX テクスチャindexを解決する。
func resolveLegacyMatcapTextureIndex(
	sourceMaterial gltfMaterial,
	doc *gltfDocument,
	textureIndexesByImage []int,
) (int, bool) {
	mtoonSource, hasMtoonSource, err := resolveMToonSource(sourceMaterial)
	if err != nil || !hasMtoonSource || mtoonSource.MatcapTexture == nil {
		return -1, false
	}
	return resolvePmxTextureIndexByTextureRef(doc, textureIndexesByImage, mtoonSource.MatcapTexture)
}

// resolveLegacyEmissiveTextureIndex は emissiveTexture 参照から PMX テクスチャindexを解決する。
func resolveLegacyEmissiveTextureIndex(
	sourceMaterial gltfMaterial,
	doc *gltfDocument,
	textureIndexesByImage []int,
) (int, bool) {
	return resolvePmxTextureIndexByTextureRef(doc, textureIndexesByImage, sourceMaterial.EmissiveTexture)
}

// hasLegacyEmissiveInput は emissive 入力が存在するか判定する。
func hasLegacyEmissiveInput(
	sourceMaterial gltfMaterial,
	doc *gltfDocument,
	textureIndexesByImage []int,
) bool {
	if _, hasEmissiveTexture := resolveLegacyEmissiveTextureIndex(sourceMaterial, doc, textureIndexesByImage); hasEmissiveTexture {
		return true
	}

	emissiveStrength := resolveLegacyEmissiveStrength(sourceMaterial)
	factorValues := sourceMaterial.EmissiveFactor
	if len(factorValues) < 3 {
		return false
	}
	for _, value := range factorValues[:3] {
		if math.Abs(value*emissiveStrength) > 1e-9 {
			return true
		}
	}
	return false
}

// resolveLegacyEmissiveStrength は KHR_materials_emissive_strength の係数を取得する。
func resolveLegacyEmissiveStrength(sourceMaterial gltfMaterial) float64 {
	if sourceMaterial.Extensions == nil {
		return 1.0
	}
	raw, exists := sourceMaterial.Extensions["KHR_materials_emissive_strength"]
	if !exists || len(raw) == 0 {
		return 1.0
	}
	source := gltfMaterialEmissiveStrengthSource{}
	if err := json.Unmarshal(raw, &source); err != nil {
		return 1.0
	}
	if source.EmissiveStrength <= 0 {
		return 1.0
	}
	return source.EmissiveStrength
}

// resolveMToonSource は VRMC_materials_mtoon 拡張を解析する。
func resolveMToonSource(sourceMaterial gltfMaterial) (gltfMaterialMToonSource, bool, error) {
	if sourceMaterial.Extensions == nil {
		return gltfMaterialMToonSource{}, false, nil
	}
	raw, exists := sourceMaterial.Extensions["VRMC_materials_mtoon"]
	if !exists || len(raw) == 0 {
		return gltfMaterialMToonSource{}, false, nil
	}
	source := gltfMaterialMToonSource{}
	if err := json.Unmarshal(raw, &source); err != nil {
		return gltfMaterialMToonSource{}, false, err
	}
	return source, true, nil
}

// buildLegacyToonBmp32 は旧仕様の 32x32 toon BMP を生成する。
func buildLegacyToonBmp32(shadeColor [3]uint8) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	upperColor := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	lowerColor := color.RGBA{R: shadeColor[0], G: shadeColor[1], B: shadeColor[2], A: 0xff}
	for y := 0; y < 32; y++ {
		lineColor := upperColor
		if y >= 24 {
			lineColor = lowerColor
		}
		for x := 0; x < 32; x++ {
			img.SetRGBA(x, y, lineColor)
		}
	}

	var out bytes.Buffer
	if err := bmp.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// toLegacyRGBColor は float RGB を 8bit RGB へ変換する。
func toLegacyRGBColor(values []float64) ([3]uint8, bool) {
	if len(values) < 3 {
		return [3]uint8{}, false
	}
	return [3]uint8{
		clampLegacyColor8(values[0]),
		clampLegacyColor8(values[1]),
		clampLegacyColor8(values[2]),
	}, true
}

// clampLegacyColor8 は 0..1 の色値を 8bit に丸める。
func clampLegacyColor8(value float64) uint8 {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	return uint8(math.Round(value * 255))
}

// applyLegacySharedToonFallback は共有 toon へフォールバックする。
func applyLegacySharedToonFallback(materialData *model.Material) {
	if materialData == nil {
		return
	}
	materialData.ToonSharingFlag = model.TOON_SHARING_SHARING
	materialData.ToonTextureIndex = 1
}

// ensureGeneratedTextureIndex は生成テクスチャ名を解決し、未登録なら追加する。
func ensureGeneratedTextureIndex(
	modelData *model.PmxModel,
	textureName string,
	textureType model.TextureType,
) (int, error) {
	if modelData == nil || modelData.Textures == nil {
		return -1, fmt.Errorf("texture collection unavailable")
	}
	normalizedTextureName := filepath.ToSlash(strings.TrimSpace(textureName))
	if normalizedTextureName == "" {
		return -1, fmt.Errorf("texture name is empty")
	}
	for textureIndex, textureData := range modelData.Textures.Values() {
		if textureData == nil {
			continue
		}
		if strings.EqualFold(filepath.ToSlash(strings.TrimSpace(textureData.Name())), normalizedTextureName) {
			return textureIndex, nil
		}
	}

	textureData := model.NewTexture()
	textureData.SetName(normalizedTextureName)
	textureData.EnglishName = normalizedTextureName
	textureData.TextureType = textureType
	textureData.SetValid(true)
	return modelData.Textures.AppendRaw(textureData), nil
}

// resolvePmxTextureIndexByTextureRef は textureRef から PMX テクスチャindexを解決する。
func resolvePmxTextureIndexByTextureRef(
	doc *gltfDocument,
	textureIndexesByImage []int,
	textureRef *gltfTextureRef,
) (int, bool) {
	if textureRef == nil {
		return -1, false
	}
	return resolvePmxTextureIndexFromGltfTextureIndex(doc, textureIndexesByImage, textureRef.Index)
}

// resolvePmxTextureIndexFromGltfTextureIndex は glTF texture index から PMX テクスチャindexを解決する。
func resolvePmxTextureIndexFromGltfTextureIndex(
	doc *gltfDocument,
	textureIndexesByImage []int,
	gltfTextureIndex int,
) (int, bool) {
	if doc == nil || gltfTextureIndex < 0 || gltfTextureIndex >= len(doc.Textures) {
		return -1, false
	}
	texture := doc.Textures[gltfTextureIndex]
	if texture.Source == nil {
		return -1, false
	}
	imageIndex := *texture.Source
	if imageIndex < 0 || imageIndex >= len(textureIndexesByImage) {
		return -1, false
	}
	pmxTextureIndex := textureIndexesByImage[imageIndex]
	if pmxTextureIndex < 0 {
		return -1, false
	}
	return pmxTextureIndex, true
}

// lookupVrm0MaterialTextureProperty は VRM0 materialProperty から texture index を取得する。
func lookupVrm0MaterialTextureProperty(property vrm0MaterialPropertySource, keys ...string) (int, bool) {
	if property.TextureProperties == nil {
		return -1, false
	}
	for _, key := range keys {
		value, exists := property.TextureProperties[key]
		if !exists {
			continue
		}
		rounded := int(math.Round(value))
		if rounded < 0 {
			continue
		}
		return rounded, true
	}
	return -1, false
}

// recordLegacyMaterialWarning は warning ID を記録し、警告ログを出力する。
func recordLegacyMaterialWarning(
	modelData *model.PmxModel,
	warningID string,
	messageFormat string,
	params ...any,
) {
	warningID = strings.TrimSpace(warningID)
	if warningID == "" {
		return
	}
	appendLegacyMaterialWarningID(modelData, warningID)
	if strings.TrimSpace(messageFormat) == "" {
		logVrmWarn("%s", warningID)
		return
	}
	logVrmWarn("%s: %s", warningID, fmt.Sprintf(messageFormat, params...))
}

// appendLegacyMaterialWarningID は warning ID を VrmData.RawExtensions へ追記する。
func appendLegacyMaterialWarningID(modelData *model.PmxModel, warningID string) {
	if modelData == nil || modelData.VrmData == nil {
		return
	}
	if modelData.VrmData.RawExtensions == nil {
		modelData.VrmData.RawExtensions = map[string]json.RawMessage{}
	}

	warningIDs := []string{}
	if rawWarnings, exists := modelData.VrmData.RawExtensions[warningid.VrmWarningRawExtensionKey]; exists && len(rawWarnings) > 0 {
		if err := json.Unmarshal(rawWarnings, &warningIDs); err != nil {
			warningIDs = []string{}
		}
	}
	for _, existingWarningID := range warningIDs {
		if strings.TrimSpace(existingWarningID) == warningID {
			return
		}
	}
	warningIDs = append(warningIDs, warningID)
	encodedWarnings, err := json.Marshal(warningIDs)
	if err != nil {
		return
	}
	modelData.VrmData.RawExtensions[warningid.VrmWarningRawExtensionKey] = encodedWarnings
}

// legacySphereCandidateLabel は sphere 候補の表示名を返す。
func legacySphereCandidateLabel(candidate legacySphereCandidateType) string {
	switch candidate {
	case legacySphereCandidateSphereAdd:
		return "_SphereAdd"
	case legacySphereCandidateHair:
		return "hair sphere"
	case legacySphereCandidateMatcap:
		return "matcap"
	case legacySphereCandidateEmissive:
		return "emissive"
	default:
		return "unknown"
	}
}

// normalizeLegacyGeneratedTextureMaterialIndex は生成ファイル名用の材質indexを正規化する。
func normalizeLegacyGeneratedTextureMaterialIndex(materialIndex int) int {
	if materialIndex < 0 {
		return 0
	}
	return materialIndex
}

// resolvePrimitiveMaterialEdgeSettings は primitive 材質のエッジ色とエッジ幅を解決する。
func resolvePrimitiveMaterialEdgeSettings(
	doc *gltfDocument,
	sourceMaterial gltfMaterial,
	materialIndex int,
	defaultEdge mmath.Vec4,
	defaultEdgeSize float64,
) (mmath.Vec4, float64) {
	edge := defaultEdge
	edgeSize := defaultEdgeSize
	if vrm0Edge, vrm0EdgeSize, ok := resolvePrimitiveMaterialEdgeFromVrm0(doc, sourceMaterial, materialIndex, edge, edgeSize); ok {
		edge = vrm0Edge
		edgeSize = vrm0EdgeSize
	}
	if mtoonEdge, mtoonEdgeSize, ok := resolvePrimitiveMaterialEdgeFromMToon(sourceMaterial, edge, edgeSize); ok {
		edge = mtoonEdge
		edgeSize = mtoonEdgeSize
	}
	if edgeSize < 0 {
		edgeSize = 0
	}
	return edge, edgeSize
}

// resolvePrimitiveMaterialEdgeFromVrm0 は VRM0 materialProperties からエッジ設定を解決する。
func resolvePrimitiveMaterialEdgeFromVrm0(
	doc *gltfDocument,
	sourceMaterial gltfMaterial,
	materialIndex int,
	defaultEdge mmath.Vec4,
	defaultEdgeSize float64,
) (mmath.Vec4, float64, bool) {
	property, ok := resolveVrm0MaterialProperty(doc, sourceMaterial, materialIndex)
	if !ok {
		return defaultEdge, defaultEdgeSize, false
	}

	edge := defaultEdge
	if colorValues, colorExists := lookupVrm0MaterialVectorProperty(
		property,
		"_OutlineColor",
		"OutlineColor",
	); colorExists {
		edge = toVec4WithDefault(colorValues, edge)
	}

	edgeSize := defaultEdgeSize
	outlineWidth, hasOutlineWidth := lookupVrm0MaterialFloatProperty(
		property,
		"_OutlineWidth",
		"OutlineWidth",
	)
	outlineWidthMode, hasOutlineWidthMode := lookupVrm0MaterialFloatProperty(
		property,
		"_OutlineWidthMode",
		"OutlineWidthMode",
	)
	if !hasOutlineWidthMode {
		outlineWidthMode = 1.0
	}
	if hasOutlineWidth || hasOutlineWidthMode {
		if !hasOutlineWidth {
			outlineWidth = defaultEdgeSize / vroidMeterScale
		}
		edgeSize = convertVrm0OutlineWidthToEdgeSize(outlineWidth, outlineWidthMode)
	}

	return edge, edgeSize, true
}

// resolvePrimitiveMaterialEdgeFromMToon は VRMC_materials_mtoon からエッジ設定を解決する。
func resolvePrimitiveMaterialEdgeFromMToon(
	sourceMaterial gltfMaterial,
	defaultEdge mmath.Vec4,
	defaultEdgeSize float64,
) (mmath.Vec4, float64, bool) {
	if sourceMaterial.Extensions == nil {
		return defaultEdge, defaultEdgeSize, false
	}
	raw, exists := sourceMaterial.Extensions["VRMC_materials_mtoon"]
	if !exists || len(raw) == 0 {
		return defaultEdge, defaultEdgeSize, false
	}

	source := gltfMaterialMToonSource{}
	if err := json.Unmarshal(raw, &source); err != nil {
		logVrmWarn(
			"VRMC_materials_mtoon の解析に失敗したため既定値で継続します: material=%s err=%s",
			sourceMaterial.Name,
			err.Error(),
		)
		return defaultEdge, defaultEdgeSize, false
	}

	edge := defaultEdge
	if len(source.OutlineColorFactor) > 0 {
		edge = toVec4WithDefault(source.OutlineColorFactor, edge)
	}
	edgeSize := convertMToonOutlineWidthToEdgeSize(source.OutlineWidthFactor, source.OutlineWidthMode)
	return edge, edgeSize, true
}

// resolveVrm0MaterialProperty は材質インデックス/材質名に対応する VRM0 materialProperty を返す。
func resolveVrm0MaterialProperty(
	doc *gltfDocument,
	sourceMaterial gltfMaterial,
	materialIndex int,
) (vrm0MaterialPropertySource, bool) {
	properties := loadVrm0MaterialProperties(doc)
	if len(properties) == 0 {
		return vrm0MaterialPropertySource{}, false
	}
	if materialIndex >= 0 && materialIndex < len(properties) {
		return properties[materialIndex], true
	}

	sourceName := strings.TrimSpace(sourceMaterial.Name)
	if sourceName == "" {
		return vrm0MaterialPropertySource{}, false
	}
	for _, property := range properties {
		if strings.TrimSpace(property.Name) == sourceName {
			return property, true
		}
	}
	return vrm0MaterialPropertySource{}, false
}

// loadVrm0MaterialProperties は VRM0 materialProperties を遅延解析して返す。
func loadVrm0MaterialProperties(doc *gltfDocument) []vrm0MaterialPropertySource {
	if doc == nil {
		return nil
	}
	if doc.V0MaterialPropertiesCached {
		return doc.V0MaterialPropertiesCache
	}
	doc.V0MaterialPropertiesCached = true
	doc.V0MaterialPropertiesCache = nil

	if doc.Extensions == nil {
		return nil
	}
	raw, exists := doc.Extensions["VRM"]
	if !exists || len(raw) == 0 {
		return nil
	}

	source := vrm0MaterialPropertiesSource{}
	if err := json.Unmarshal(raw, &source); err != nil {
		logVrmWarn("VRM0 materialProperties の解析に失敗したため既定値で継続します: err=%s", err.Error())
		return nil
	}
	doc.V0MaterialPropertiesCache = source.MaterialProperties
	return doc.V0MaterialPropertiesCache
}

// lookupVrm0MaterialVectorProperty は VRM0 materialProperty からベクトル値を取得する。
func lookupVrm0MaterialVectorProperty(property vrm0MaterialPropertySource, keys ...string) ([]float64, bool) {
	if property.VectorProperties == nil {
		return nil, false
	}
	for _, key := range keys {
		if value, exists := property.VectorProperties[key]; exists {
			return value, true
		}
	}
	return nil, false
}

// lookupVrm0MaterialFloatProperty は VRM0 materialProperty から float 値を取得する。
func lookupVrm0MaterialFloatProperty(property vrm0MaterialPropertySource, keys ...string) (float64, bool) {
	if property.FloatProperties == nil {
		return 0, false
	}
	for _, key := range keys {
		if value, exists := property.FloatProperties[key]; exists {
			return value, true
		}
	}
	return 0, false
}

// convertVrm0OutlineWidthToEdgeSize は VRM0 _OutlineWidth(_Mode) を PMX EdgeSize へ変換する。
func convertVrm0OutlineWidthToEdgeSize(outlineWidth float64, outlineWidthMode float64) float64 {
	switch {
	case outlineWidthMode <= 0:
		return 0
	case outlineWidthMode >= 2:
		// screenCoordinates は PMX で再現できないため無効化する。
		return 0
	case outlineWidth <= 0:
		return 0
	default:
		return outlineWidth * vroidMeterScale
	}
}

// convertMToonOutlineWidthToEdgeSize は VRMC_materials_mtoon の outline 情報を PMX EdgeSize へ変換する。
func convertMToonOutlineWidthToEdgeSize(outlineWidthFactor float64, outlineWidthMode string) float64 {
	switch strings.ToLower(strings.TrimSpace(outlineWidthMode)) {
	case "none":
		return 0
	case "screencoordinates":
		// screenCoordinates は PMX で再現できないため無効化する。
		return 0
	default:
		if outlineWidthFactor <= 0 {
			return 0
		}
		return outlineWidthFactor * vroidMeterScale
	}
}

type primitiveMaterialKind int

const (
	primitiveMaterialKindUnknown primitiveMaterialKind = iota
	primitiveMaterialKindBody
	primitiveMaterialKindFace
	primitiveMaterialKindHair
	primitiveMaterialKindCloth
	primitiveMaterialKindAccessory
)

// shouldEnablePrimitiveMaterialEdge は primitive 材質にエッジ描画フラグを付与するか判定する。
func shouldEnablePrimitiveMaterialEdge(
	alphaMode string,
	materialName string,
	materialEnglishName string,
	edgeSize float64,
	textureIndex int,
) bool {
	if edgeSize <= 0 {
		return false
	}
	if isSpecialEyeOverlayPrimitiveMaterialName(materialName, materialEnglishName) {
		return false
	}
	switch resolvePrimitiveMaterialKind(materialName, materialEnglishName) {
	case primitiveMaterialKindBody, primitiveMaterialKindFace, primitiveMaterialKindHair, primitiveMaterialKindAccessory:
		return true
	case primitiveMaterialKindCloth:
		normalizedAlphaMode := strings.ToUpper(strings.TrimSpace(alphaMode))
		return (normalizedAlphaMode == "MASK" || normalizedAlphaMode == "BLEND") && textureIndex >= 0
	default:
		return false
	}
}

// resolvePrimitiveMaterialKind は VRoid 向け材質種別を判定する。
func resolvePrimitiveMaterialKind(materialName string, materialEnglishName string) primitiveMaterialKind {
	normalized := normalizeCreateSemanticName(strings.TrimSpace(materialName + " " + materialEnglishName))
	if normalized == "" {
		return primitiveMaterialKindUnknown
	}
	switch {
	case strings.Contains(normalized, "body"):
		return primitiveMaterialKindBody
	case strings.Contains(normalized, "face"):
		if strings.Contains(normalized, "facemouth") ||
			strings.Contains(normalized, "facebrow") ||
			strings.Contains(normalized, "faceeyeline") ||
			strings.Contains(normalized, "faceeyelash") ||
			strings.Contains(normalized, "eyewhite") ||
			strings.Contains(normalized, "eyeiris") ||
			strings.Contains(normalized, "eyehighlight") {
			return primitiveMaterialKindUnknown
		}
		return primitiveMaterialKindFace
	case strings.Contains(normalized, "hair"):
		return primitiveMaterialKindHair
	case strings.Contains(normalized, "cloth"):
		return primitiveMaterialKindCloth
	case strings.Contains(normalized, "accessory"):
		return primitiveMaterialKindAccessory
	default:
		return primitiveMaterialKindUnknown
	}
}

// isSpecialEyeOverlayPrimitiveMaterialName は特殊目オーバーレイ材質名か判定する。
func isSpecialEyeOverlayPrimitiveMaterialName(materialName string, materialEnglishName string) bool {
	normalizedName := normalizeCreateSemanticName(strings.TrimSpace(materialName + " " + materialEnglishName))
	if normalizedName == "" {
		return false
	}
	for _, token := range specialEyeOverlayTextureTokens {
		if strings.Contains(normalizedName, normalizeSpecialEyeToken(token)) {
			return true
		}
	}
	return false
}

// buildPrimitiveMaterialMemo は primitive 材質へ埋め込む付加情報メモを生成する。
func buildPrimitiveMaterialMemo(alphaMode string) string {
	const memoPrefix = "VRM primitive"
	trimmedAlphaMode := strings.ToUpper(strings.TrimSpace(alphaMode))
	if trimmedAlphaMode == "" {
		return memoPrefix
	}
	return fmt.Sprintf("%s alphaMode=%s", memoPrefix, trimmedAlphaMode)
}

// registerExpressionMaterialIndex は表情材質バインド用の材質index参照を登録する。
func registerExpressionMaterialIndex(
	registry *targetMorphRegistry,
	doc *gltfDocument,
	gltfMaterialIndex *int,
	pmxMaterialName string,
	pmxMaterialIndex int,
) {
	if registry == nil || pmxMaterialIndex < 0 {
		return
	}
	if gltfMaterialIndex != nil && *gltfMaterialIndex >= 0 {
		registry.ByGltfMaterial[*gltfMaterialIndex] = appendUniqueInt(
			registry.ByGltfMaterial[*gltfMaterialIndex],
			pmxMaterialIndex,
		)
		if doc != nil && *gltfMaterialIndex < len(doc.Materials) {
			sourceMaterialName := strings.TrimSpace(doc.Materials[*gltfMaterialIndex].Name)
			if sourceMaterialName != "" {
				registry.ByMaterialName[sourceMaterialName] = appendUniqueInt(
					registry.ByMaterialName[sourceMaterialName],
					pmxMaterialIndex,
				)
			}
		}
	}
	normalizedName := strings.TrimSpace(pmxMaterialName)
	if normalizedName == "" {
		return
	}
	registry.ByMaterialName[normalizedName] = appendUniqueInt(
		registry.ByMaterialName[normalizedName],
		pmxMaterialIndex,
	)
}

// resolveMaterialTextureIndex はbaseColorTextureからPMXテクスチャindexを解決する。
func resolveMaterialTextureIndex(doc *gltfDocument, primitive gltfPrimitive, textureIndexesByImage []int) int {
	if doc == nil || primitive.Material == nil {
		return -1
	}
	materialIndex := *primitive.Material
	if materialIndex < 0 || materialIndex >= len(doc.Materials) {
		return -1
	}
	baseTexture := doc.Materials[materialIndex].PbrMetallicRoughness.BaseColorTexture
	if baseTexture == nil {
		return -1
	}
	if baseTexture.Index < 0 || baseTexture.Index >= len(doc.Textures) {
		return -1
	}
	texture := doc.Textures[baseTexture.Index]
	if texture.Source == nil {
		return -1
	}
	imageIndex := *texture.Source
	if imageIndex < 0 || imageIndex >= len(textureIndexesByImage) {
		return -1
	}
	return textureIndexesByImage[imageIndex]
}

// triangulateIndices はprimitive modeに従って三角形indexへ変換する。
func triangulateIndices(indexes []int, mode int) [][3]int {
	switch mode {
	case gltfPrimitiveModeTriangles:
		out := make([][3]int, 0, len(indexes)/3)
		for i := 0; i+2 < len(indexes); i += 3 {
			out = append(out, [3]int{indexes[i], indexes[i+1], indexes[i+2]})
		}
		return out
	case gltfPrimitiveModeTriangleStrip:
		out := make([][3]int, 0, len(indexes)-2)
		for i := 0; i+2 < len(indexes); i++ {
			if i%2 == 0 {
				out = append(out, [3]int{indexes[i], indexes[i+1], indexes[i+2]})
			} else {
				out = append(out, [3]int{indexes[i+1], indexes[i], indexes[i+2]})
			}
		}
		return out
	case gltfPrimitiveModeTriangleFan:
		out := make([][3]int, 0, len(indexes)-2)
		for i := 1; i+1 < len(indexes); i++ {
			out = append(out, [3]int{indexes[0], indexes[i], indexes[i+1]})
		}
		return out
	case gltfPrimitiveModePoints, gltfPrimitiveModeLines, gltfPrimitiveModeLineLoop, gltfPrimitiveModeLineStrip:
		return [][3]int{}
	default:
		return [][3]int{}
	}
}

// readPrimitiveIndices はprimitiveのindex配列を返す。
func readPrimitiveIndices(
	doc *gltfDocument,
	primitive gltfPrimitive,
	vertexCount int,
	binChunk []byte,
	cache *accessorValueCache,
) ([]int, error) {
	if primitive.Indices == nil {
		indices := make([]int, vertexCount)
		for i := 0; i < vertexCount; i++ {
			indices[i] = i
		}
		return indices, nil
	}
	accessorIndex := *primitive.Indices
	values, err := cache.readIntValues(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, io_common.NewIoParseFailed("indices の読み取りに失敗しました(accessor=%d)", err, accessorIndex)
	}
	indices := make([]int, 0, len(values))
	for _, row := range values {
		if len(row) == 0 {
			continue
		}
		indices = append(indices, row[0])
	}
	return indices, nil
}

// readOptionalFloatAttribute は任意属性accessorを読み取る。
func readOptionalFloatAttribute(
	doc *gltfDocument,
	attributes map[string]int,
	key string,
	binChunk []byte,
	cache *accessorValueCache,
) ([][]float64, error) {
	if attributes == nil {
		return nil, nil
	}
	accessorIndex, ok := attributes[key]
	if !ok {
		return nil, nil
	}
	values, err := cache.readFloatValues(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, io_common.NewIoParseFailed("%s属性の読み取りに失敗しました(accessor=%d)", err, key, accessorIndex)
	}
	return values, nil
}

// readOptionalIntAttribute は任意属性accessorを読み取る。
func readOptionalIntAttribute(
	doc *gltfDocument,
	attributes map[string]int,
	key string,
	binChunk []byte,
	cache *accessorValueCache,
) ([][]int, error) {
	if attributes == nil {
		return nil, nil
	}
	accessorIndex, ok := attributes[key]
	if !ok {
		return nil, nil
	}
	values, err := cache.readIntValues(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, io_common.NewIoParseFailed("%s属性の読み取りに失敗しました(accessor=%d)", err, key, accessorIndex)
	}
	return values, nil
}

// readAccessorFloatValues はaccessorをfloat値配列として読み取る。
func readAccessorFloatValues(doc *gltfDocument, accessorIndex int, binChunk []byte) ([][]float64, error) {
	plan, err := prepareAccessorRead(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, err
	}
	values := make([][]float64, plan.Accessor.Count)
	for i := 0; i < plan.Accessor.Count; i++ {
		row := make([]float64, plan.ComponentNum)
		elementBase := plan.BaseOffset + i*plan.Stride
		for c := 0; c < plan.ComponentNum; c++ {
			value, readErr := readComponentAsFloat(plan.Accessor, binChunk, elementBase+c*plan.ComponentSize)
			if readErr != nil {
				return nil, readErr
			}
			row[c] = value
		}
		values[i] = row
	}
	return values, nil
}

// readAccessorIntValues はaccessorをint値配列として読み取る。
func readAccessorIntValues(doc *gltfDocument, accessorIndex int, binChunk []byte) ([][]int, error) {
	plan, err := prepareAccessorRead(doc, accessorIndex, binChunk)
	if err != nil {
		return nil, err
	}
	values := make([][]int, plan.Accessor.Count)
	for i := 0; i < plan.Accessor.Count; i++ {
		row := make([]int, plan.ComponentNum)
		elementBase := plan.BaseOffset + i*plan.Stride
		for c := 0; c < plan.ComponentNum; c++ {
			value, readErr := readComponentAsInt(plan.Accessor.ComponentType, binChunk, elementBase+c*plan.ComponentSize)
			if readErr != nil {
				return nil, readErr
			}
			row[c] = value
		}
		values[i] = row
	}
	return values, nil
}

// prepareAccessorRead はaccessor読み取りに必要な情報を検証して返す。
func prepareAccessorRead(doc *gltfDocument, accessorIndex int, binChunk []byte) (accessorReadPlan, error) {
	if doc == nil {
		return accessorReadPlan{}, io_common.NewIoParseFailed("gltf document が未設定です", nil)
	}
	if accessorIndex < 0 || accessorIndex >= len(doc.Accessors) {
		return accessorReadPlan{}, io_common.NewIoParseFailed("accessor index が不正です: %d", nil, accessorIndex)
	}
	accessor := doc.Accessors[accessorIndex]
	if accessor.BufferView == nil {
		return accessorReadPlan{}, io_common.NewIoParseFailed("sparse accessor は未対応です", nil)
	}
	if accessor.Count < 0 {
		return accessorReadPlan{}, io_common.NewIoParseFailed("accessor.count が不正です: %d", nil, accessor.Count)
	}

	viewIndex := *accessor.BufferView
	if viewIndex < 0 || viewIndex >= len(doc.BufferViews) {
		return accessorReadPlan{}, io_common.NewIoParseFailed("bufferView index が不正です: %d", nil, viewIndex)
	}
	view := doc.BufferViews[viewIndex]
	if view.Buffer != 0 {
		return accessorReadPlan{}, io_common.NewIoParseFailed("bufferView.buffer が未対応です: %d", nil, view.Buffer)
	}
	if view.ByteLength < 0 || view.ByteOffset < 0 {
		return accessorReadPlan{}, io_common.NewIoParseFailed("bufferView の byteOffset/byteLength が不正です", nil)
	}
	if view.ByteOffset+view.ByteLength > len(binChunk) {
		return accessorReadPlan{}, io_common.NewIoParseFailed("bufferView 範囲がBINチャンク外です", nil)
	}

	componentNum, err := accessorComponentNum(accessor.Type)
	if err != nil {
		return accessorReadPlan{}, err
	}
	componentSize, err := accessorComponentSize(accessor.ComponentType)
	if err != nil {
		return accessorReadPlan{}, err
	}
	elementSize := componentNum * componentSize
	stride := view.ByteStride
	if stride <= 0 {
		stride = elementSize
	}
	if stride < elementSize {
		return accessorReadPlan{}, io_common.NewIoParseFailed("bufferView.byteStride が要素サイズより小さいです", nil)
	}
	baseOffset := view.ByteOffset + accessor.ByteOffset
	if baseOffset < view.ByteOffset || baseOffset > view.ByteOffset+view.ByteLength {
		return accessorReadPlan{}, io_common.NewIoParseFailed("accessor.byteOffset が不正です", nil)
	}
	if accessor.Count > 0 {
		lastEnd := baseOffset + (accessor.Count-1)*stride + elementSize
		if lastEnd > view.ByteOffset+view.ByteLength || lastEnd > len(binChunk) {
			return accessorReadPlan{}, io_common.NewIoParseFailed("accessor 範囲がbufferViewを超えています", nil)
		}
	}

	return accessorReadPlan{
		Accessor:      accessor,
		ComponentSize: componentSize,
		ComponentNum:  componentNum,
		Stride:        stride,
		BaseOffset:    baseOffset,
		ViewStart:     view.ByteOffset,
		ViewEnd:       view.ByteOffset + view.ByteLength,
	}, nil
}

// accessorComponentNum はaccessor.typeから要素次元数を返す。
func accessorComponentNum(typeName string) (int, error) {
	switch typeName {
	case "SCALAR":
		return 1, nil
	case "VEC2":
		return 2, nil
	case "VEC3":
		return 3, nil
	case "VEC4":
		return 4, nil
	default:
		return 0, io_common.NewIoFormatNotSupported("accessor.type が未対応です: %s", nil, typeName)
	}
}

// accessorComponentSize はcomponentTypeのバイト幅を返す。
func accessorComponentSize(componentType int) (int, error) {
	switch componentType {
	case gltfComponentTypeByte, gltfComponentTypeUnsignedByte:
		return 1, nil
	case gltfComponentTypeShort, gltfComponentTypeUnsignedShort:
		return 2, nil
	case gltfComponentTypeUnsignedInt, gltfComponentTypeFloat:
		return 4, nil
	default:
		return 0, io_common.NewIoFormatNotSupported("accessor.componentType が未対応です: %d", nil, componentType)
	}
}

// readComponentAsFloat はcomponentTypeをfloat64へ変換する。
func readComponentAsFloat(accessor gltfAccessor, data []byte, offset int) (float64, error) {
	switch accessor.ComponentType {
	case gltfComponentTypeByte:
		value := float64(int8(data[offset]))
		if accessor.Normalized {
			return math.Max(value/127.0, -1.0), nil
		}
		return value, nil
	case gltfComponentTypeUnsignedByte:
		value := float64(data[offset])
		if accessor.Normalized {
			return value / 255.0, nil
		}
		return value, nil
	case gltfComponentTypeShort:
		value := float64(int16(binary.LittleEndian.Uint16(data[offset : offset+2])))
		if accessor.Normalized {
			return math.Max(value/32767.0, -1.0), nil
		}
		return value, nil
	case gltfComponentTypeUnsignedShort:
		value := float64(binary.LittleEndian.Uint16(data[offset : offset+2]))
		if accessor.Normalized {
			return value / 65535.0, nil
		}
		return value, nil
	case gltfComponentTypeUnsignedInt:
		value := float64(binary.LittleEndian.Uint32(data[offset : offset+4]))
		if accessor.Normalized {
			return value / 4294967295.0, nil
		}
		return value, nil
	case gltfComponentTypeFloat:
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(data[offset : offset+4]))), nil
	default:
		return 0, io_common.NewIoFormatNotSupported("float componentType が未対応です: %d", nil, accessor.ComponentType)
	}
}

// readComponentAsInt はcomponentTypeをintへ変換する。
func readComponentAsInt(componentType int, data []byte, offset int) (int, error) {
	switch componentType {
	case gltfComponentTypeByte:
		return int(int8(data[offset])), nil
	case gltfComponentTypeUnsignedByte:
		return int(data[offset]), nil
	case gltfComponentTypeShort:
		return int(int16(binary.LittleEndian.Uint16(data[offset : offset+2]))), nil
	case gltfComponentTypeUnsignedShort:
		return int(binary.LittleEndian.Uint16(data[offset : offset+2])), nil
	case gltfComponentTypeUnsignedInt:
		return int(binary.LittleEndian.Uint32(data[offset : offset+4])), nil
	default:
		return 0, io_common.NewIoFormatNotSupported("int componentType が未対応です: %d", nil, componentType)
	}
}

// toVec3 はfloat配列をVec3へ変換する。
func toVec3(values []float64, defaultValue mmath.Vec3) mmath.Vec3 {
	if len(values) < 3 {
		return defaultValue
	}
	return mmath.Vec3{
		Vec: r3.Vec{
			X: values[0],
			Y: values[1],
			Z: values[2],
		},
	}
}

// toVec2 はfloat配列をVec2へ変換する。
func toVec2(values []float64, defaultValue mmath.Vec2) mmath.Vec2 {
	if len(values) < 2 {
		return defaultValue
	}
	return mmath.Vec2{
		X: values[0],
		Y: values[1],
	}
}
