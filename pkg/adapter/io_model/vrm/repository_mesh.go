// 指示: miu200521358
package vrm

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/miu200521358/mlib_go/pkg/adapter/io_common"
	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
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
	createMorphEyeHideProjectionZOffset   = 0.03
	createMorphEyeFallbackScaleRatio      = 0.15
	createMorphProjectionLineHalfDistance = 1000.0
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

// morphPairLinkFallbackRule は MORPH_PAIRS の binds/split フォールバック規則を表す。
type morphPairLinkFallbackRule struct {
	Name   string
	Panel  model.MorphPanel
	Binds  []string
	Ratios []float64
	Split  string
}

// vrm1PresetExpressionNamePairs は VRM1 標準preset名を旧MORPH_PAIRS互換名へ正規化する対応を表す。
var vrm1PresetExpressionNamePairs = map[string]string{
	"aa":         "Fcl_MTH_A",
	"ih":         "Fcl_MTH_I",
	"ou":         "Fcl_MTH_U",
	"ee":         "Fcl_MTH_E",
	"oh":         "Fcl_MTH_O",
	"blink":      "Fcl_EYE_Close",
	"blinkleft":  "Fcl_EYE_Close_L",
	"blinkright": "Fcl_EYE_Close_R",
	"neutral":    "Fcl_ALL_Neutral",
	"angry":      "Fcl_ALL_Angry",
	"relaxed":    "Fcl_ALL_Fun",
	"happy":      "Fcl_ALL_Joy",
	"sad":        "Fcl_ALL_Sorrow",
	"surprised":  "Fcl_ALL_Surprised",
}

// vrm0PresetExpressionNamePairs は VRM0 標準preset名を旧MORPH_PAIRS互換名へ正規化する対応を表す。
var vrm0PresetExpressionNamePairs = map[string]string{
	"a":         "Fcl_MTH_A",
	"i":         "Fcl_MTH_I",
	"u":         "Fcl_MTH_U",
	"e":         "Fcl_MTH_E",
	"o":         "Fcl_MTH_O",
	"blink":     "Fcl_EYE_Close",
	"blink_l":   "Fcl_EYE_Close_L",
	"blink_r":   "Fcl_EYE_Close_R",
	"neutral":   "Fcl_ALL_Neutral",
	"angry":     "Fcl_ALL_Angry",
	"fun":       "Fcl_ALL_Fun",
	"joy":       "Fcl_ALL_Joy",
	"sorrow":    "Fcl_ALL_Sorrow",
	"surprised": "Fcl_ALL_Surprised",
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

// createMorphFallbackRules は旧参考実装の creates 対象を表す。
var createMorphFallbackRules = []createMorphRule{
	{
		Name:    "brow_Below_R",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "brow_Below_L",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "brow_Abobe_R",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "brow_Abobe_L",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "brow_Left_R",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "brow_Left_L",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "brow_Right_R",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "brow_Right_L",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "brow_Front_R",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "brow_Front_L",
		Panel:   model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Type:    createMorphRuleTypeBrow,
		Creates: []string{"FaceBrow"},
	},
	{
		Name:    "eye_Small_R",
		Panel:   model.MORPH_PANEL_EYE_UPPER_LEFT,
		Type:    createMorphRuleTypeEyeSmall,
		Creates: []string{"EyeIris", "EyeHighlight"},
	},
	{
		Name:    "eye_Small_L",
		Panel:   model.MORPH_PANEL_EYE_UPPER_LEFT,
		Type:    createMorphRuleTypeEyeSmall,
		Creates: []string{"EyeIris", "EyeHighlight"},
	},
	{
		Name:    "eye_Big_R",
		Panel:   model.MORPH_PANEL_EYE_UPPER_LEFT,
		Type:    createMorphRuleTypeEyeBig,
		Creates: []string{"EyeIris", "EyeHighlight"},
	},
	{
		Name:    "eye_Big_L",
		Panel:   model.MORPH_PANEL_EYE_UPPER_LEFT,
		Type:    createMorphRuleTypeEyeBig,
		Creates: []string{"EyeIris", "EyeHighlight"},
	},
	{
		Name:    "eye_Hide_Vertex",
		Panel:   model.MORPH_PANEL_SYSTEM,
		Type:    createMorphRuleTypeEyeHideVertex,
		Creates: []string{"EyeWhite"},
		Hides:   []string{"Eyeline", "Eyelash"},
	},
}

// morphPairLinkFallbackRules は旧 MORPH_PAIRS 由来の binds/split 規則を表す。
var morphPairLinkFallbackRules = []morphPairLinkFallbackRule{
	{
		Name:  "Fcl_BRW_Fun_R",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "Fcl_BRW_Fun",
	},
	{
		Name:  "Fcl_BRW_Fun_L",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "Fcl_BRW_Fun",
	},
	{
		Name:  "Fcl_BRW_Joy_R",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "Fcl_BRW_Joy",
	},
	{
		Name:  "Fcl_BRW_Joy_L",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "Fcl_BRW_Joy",
	},
	{
		Name:  "Fcl_BRW_Sorrow_R",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "Fcl_BRW_Sorrow",
	},
	{
		Name:  "Fcl_BRW_Sorrow_L",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "Fcl_BRW_Sorrow",
	},
	{
		Name:  "Fcl_BRW_Angry_R",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "Fcl_BRW_Angry",
	},
	{
		Name:  "Fcl_BRW_Angry_L",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "Fcl_BRW_Angry",
	},
	{
		Name:  "Fcl_BRW_Surprised_R",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "Fcl_BRW_Surprised",
	},
	{
		Name:  "Fcl_BRW_Surprised_L",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "Fcl_BRW_Surprised",
	},
	{
		Name:  "brow_Below",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"brow_Below_R", "brow_Below_L"},
	},
	{
		Name:  "brow_Abobe",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"brow_Abobe_R", "brow_Abobe_L"},
	},
	{
		Name:  "brow_Left",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"brow_Left_R", "brow_Left_L"},
	},
	{
		Name:  "brow_Right",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"brow_Right_R", "brow_Right_L"},
	},
	{
		Name:  "brow_Front",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"brow_Front_R", "brow_Front_L"},
	},
	{
		Name:   "brow_Serious_R",
		Panel:  model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds:  []string{"Fcl_BRW_Angry_R", "brow_Below_R"},
		Ratios: []float64{0.25, 0.7},
	},
	{
		Name:   "brow_Serious_L",
		Panel:  model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds:  []string{"Fcl_BRW_Angry_L", "brow_Below_L"},
		Ratios: []float64{0.25, 0.7},
	},
	{
		Name:   "brow_Serious",
		Panel:  model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds:  []string{"Fcl_BRW_Angry_R", "brow_Below_R", "Fcl_BRW_Angry_L", "brow_Below_L"},
		Ratios: []float64{0.25, 0.7, 0.25, 0.7},
	},
	{
		Name:   "brow_Frown_R",
		Panel:  model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds:  []string{"Fcl_BRW_Angry_R", "Fcl_BRW_Sorrow_R", "brow_Right_R"},
		Ratios: []float64{0.5, 0.5, 0.3},
	},
	{
		Name:   "brow_Frown_L",
		Panel:  model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds:  []string{"Fcl_BRW_Angry_L", "Fcl_BRW_Sorrow_L", "brow_Left_L"},
		Ratios: []float64{0.5, 0.5, 0.3},
	},
	{
		Name:   "brow_Frown",
		Panel:  model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds:  []string{"Fcl_BRW_Angry_R", "Fcl_BRW_Sorrow_R", "brow_Right_R", "Fcl_BRW_Angry_L", "Fcl_BRW_Sorrow_L", "brow_Left_L"},
		Ratios: []float64{0.5, 0.5, 0.3, 0.5, 0.5, 0.3},
	},
	{
		Name:  "browInnerUp_R",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "browInnerUp",
	},
	{
		Name:  "browInnerUp_L",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Split: "browInnerUp",
	},
	{
		Name:  "browDown",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"browDownRight", "browDownLeft"},
	},
	{
		Name:  "browOuter",
		Panel: model.MORPH_PANEL_EYEBROW_LOWER_LEFT,
		Binds: []string{"browOuterUpRight", "browOuterUpLeft"},
	},
	{
		Name:  "Fcl_EYE_Surprised_R",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Surprised",
	},
	{
		Name:  "Fcl_EYE_Surprised_L",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Surprised",
	},
	{
		Name:  "eye_Small",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"eye_Small_R", "eye_Small_L"},
	},
	{
		Name:  "eye_Big",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"eye_Big_R", "eye_Big_L"},
	},
	{
		Name:   "Fcl_EYE_Close_R_Group",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"brow_Below_R", "Fcl_EYE_Close_R", "eye_Small_R", "Fcl_EYE_Close_R_Bone", "brow_Front_R", "Fcl_BRW_Sorrow_R"},
		Ratios: []float64{0.2, 1.0, 0.3, 1.0, 0.1, 0.2},
	},
	{
		Name:   "Fcl_EYE_Close_L_Group",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"brow_Below_L", "Fcl_EYE_Close_L", "eye_Small_L", "Fcl_EYE_Close_L_Bone", "brow_Front_L", "Fcl_BRW_Sorrow_L"},
		Ratios: []float64{0.2, 1.0, 0.3, 1.0, 0.1, 0.2},
	},
	{
		Name:   "Fcl_EYE_Close_Group",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"brow_Below_R", "Fcl_EYE_Close_R", "eye_Small_R", "Fcl_EYE_Close_R_Bone", "brow_Front_R", "Fcl_BRW_Sorrow_R", "brow_Below_L", "Fcl_EYE_Close_L", "eye_Small_L", "Fcl_EYE_Close_L_Bone", "brow_Front_L", "Fcl_BRW_Sorrow_L"},
		Ratios: []float64{0.2, 1.0, 0.3, 1.0, 0.1, 0.2, 0.2, 1.0, 0.3, 1.0, 0.1, 0.2},
	},
	{
		Name:   "Fcl_EYE_Joy_R_Group",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"brow_Below_R", "Fcl_EYE_Joy_R", "eye_Small_R", "Fcl_EYE_Joy_R_Bone", "brow_Front_R", "Fcl_BRW_Fun_R"},
		Ratios: []float64{0.5, 1.0, 0.3, 1.0, 0.1, 0.5},
	},
	{
		Name:   "Fcl_EYE_Joy_L_Group",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"brow_Below_L", "Fcl_EYE_Joy_L", "eye_Small_L", "Fcl_EYE_Joy_L_Bone", "brow_Front_L", "Fcl_BRW_Fun_L"},
		Ratios: []float64{0.5, 1.0, 0.3, 1.0, 0.1, 0.5},
	},
	{
		Name:   "Fcl_EYE_Joy_Group",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"brow_Below_R", "Fcl_EYE_Joy_R", "eye_Small_R", "Fcl_EYE_Joy_R_Bone", "brow_Front_R", "Fcl_BRW_Fun_R", "brow_Below_L", "Fcl_EYE_Joy_L", "eye_Small_L", "Fcl_EYE_Joy_L_Bone", "brow_Front_L", "Fcl_BRW_Fun_L"},
		Ratios: []float64{0.5, 1.0, 0.3, 1.0, 0.1, 0.5, 0.5, 1.0, 0.3, 1.0, 0.1, 0.5},
	},
	{
		Name:  "Fcl_EYE_Fun_R",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Fun",
	},
	{
		Name:  "Fcl_EYE_Fun_L",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Fun",
	},
	{
		Name:  "raiseEyelid_R",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Fun_R",
	},
	{
		Name:  "raiseEyelid_L",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Fun_L",
	},
	{
		Name:  "raiseEyelid",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"raiseEyelid_R", "raiseEyelid_L"},
	},
	{
		Name:  "eyeSquint",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"eyeSquintRight", "eyeSquintLeft"},
	},
	{
		Name:  "Fcl_EYE_Angry_R",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Angry",
	},
	{
		Name:  "Fcl_EYE_Angry_L",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Angry",
	},
	{
		Name:  "noseSneer",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"noseSneerRight", "noseSneerLeft"},
	},
	{
		Name:  "Fcl_EYE_Sorrow_R",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Sorrow",
	},
	{
		Name:  "Fcl_EYE_Sorrow_L",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Sorrow",
	},
	{
		Name:  "Fcl_EYE_Spread_R",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Spread",
	},
	{
		Name:  "Fcl_EYE_Spread_L",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Spread",
	},
	{
		Name:   "eye_Nanu_R",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"Fcl_EYE_Surprised_R", "Fcl_EYE_Angry_R"},
		Ratios: []float64{1.0, 1.0},
	},
	{
		Name:   "eye_Nanu_L",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"Fcl_EYE_Surprised_L", "Fcl_EYE_Angry_L"},
		Ratios: []float64{1.0, 1.0},
	},
	{
		Name:   "eye_Nanu",
		Panel:  model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds:  []string{"Fcl_EYE_Surprised_R", "Fcl_EYE_Angry_R", "Fcl_EYE_Surprised_L", "Fcl_EYE_Angry_L"},
		Ratios: []float64{1.0, 1.0, 1.0, 1.0},
	},
	{
		Name:  "eye_Hau",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"eye_Hau_Material", "eye_Hide_Vertex"},
	},
	{
		Name:  "eye_Hachume",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"eye_Hachume_Material", "eye_Hide_Vertex"},
	},
	{
		Name:  "eye_Nagomi",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"eye_Nagomi_Material", "eye_Hide_Vertex"},
	},
	{
		Name:  "eye_Star",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"Fcl_EYE_Highlight_Hide", "eye_Star_Material"},
	},
	{
		Name:  "eye_Heart",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"Fcl_EYE_Highlight_Hide", "eye_Heart_Material"},
	},
	{
		Name:  "eyeWide",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"eyeSquintRight", "eyeSquintLeft"},
	},
	{
		Name:  "eyeLookUp",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"eyeLookUpRight", "eyeLookUpLeft"},
	},
	{
		Name:  "eyeLookDown",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"eyeLookDownRight", "eyeLookDownLeft"},
	},
	{
		Name:  "eyeLookIn",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"eyeLookInRight", "eyeLookInLeft"},
	},
	{
		Name:  "eyeLookOut",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"eyeLookOutRight", "eyeLookOutLeft"},
	},
	{
		Name:  "_eyeIrisMoveBack",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"_eyeIrisMoveBack_R", "_eyeIrisMoveBack_L"},
	},
	{
		Name:  "_eyeSquint+LowerUp",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Binds: []string{"_eyeSquint+LowerUp_R", "_eyeSquint+LowerUp_L"},
	},
	{
		Name:  "Fcl_EYE_Iris_Hide_R",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Iris_Hide",
	},
	{
		Name:  "Fcl_EYE_Iris_Hide_L",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Iris_Hide",
	},
	{
		Name:  "Fcl_EYE_Highlight_Hide_R",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Highlight_Hide",
	},
	{
		Name:  "Fcl_EYE_Highlight_Hide_L",
		Panel: model.MORPH_PANEL_EYE_UPPER_LEFT,
		Split: "Fcl_EYE_Highlight_Hide",
	},
	{
		Name:  "Fcl_MTH_A_Group",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"Fcl_MTH_A", "Fcl_MTH_A_Bone"},
	},
	{
		Name:  "Fcl_MTH_I_Group",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"Fcl_MTH_I", "Fcl_MTH_I_Bone"},
	},
	{
		Name:  "Fcl_MTH_U_Group",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"Fcl_MTH_U", "Fcl_MTH_U_Bone"},
	},
	{
		Name:  "Fcl_MTH_E_Group",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"Fcl_MTH_E", "Fcl_MTH_E_Bone"},
	},
	{
		Name:  "Fcl_MTH_O_Group",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"Fcl_MTH_O", "Fcl_MTH_O_Bone"},
	},
	{
		Name:  "Fcl_MTH_Angry_R",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "Fcl_MTH_Angry",
	},
	{
		Name:  "Fcl_MTH_Angry_L",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "Fcl_MTH_Angry",
	},
	{
		Name:   "Fcl_MTH_Sage_R",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"Fcl_MTH_Angry_R", "Fcl_MTH_Large"},
		Ratios: []float64{1.0, 0.5},
	},
	{
		Name:   "Fcl_MTH_Sage_L",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"Fcl_MTH_Angry_L", "Fcl_MTH_Large"},
		Ratios: []float64{1.0, 0.5},
	},
	{
		Name:   "Fcl_MTH_Sage",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"Fcl_MTH_Angry", "Fcl_MTH_Large"},
		Ratios: []float64{1.0, 0.5},
	},
	{
		Name:  "Fcl_MTH_Fun_R",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "Fcl_MTH_Fun",
	},
	{
		Name:  "Fcl_MTH_Fun_L",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "Fcl_MTH_Fun",
	},
	{
		Name:   "Fcl_MTH_Niko_R",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"Fcl_MTH_Fun_R", "Fcl_MTH_Large"},
		Ratios: []float64{1.0, -0.3},
	},
	{
		Name:   "Fcl_MTH_Niko_L",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"Fcl_MTH_Fun_L", "Fcl_MTH_Large"},
		Ratios: []float64{1.0, -0.3},
	},
	{
		Name:   "Fcl_MTH_Niko",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"Fcl_MTH_Fun_R", "Fcl_MTH_Fun_L", "Fcl_MTH_Large"},
		Ratios: []float64{0.5, 0.5, -0.3},
	},
	{
		Name:  "Fcl_MTH_Joy_Group",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"Fcl_MTH_Joy", "Fcl_MTH_Joy_Bone"},
	},
	{
		Name:  "Fcl_MTH_Sorrow_Group",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"Fcl_MTH_Sorrow", "Fcl_MTH_Sorrow_Bone"},
	},
	{
		Name:  "Fcl_MTH_Surprised_Group",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"Fcl_MTH_Surprised", "Fcl_MTH_Surprised_Bone"},
	},
	{
		Name:   "Fcl_MTH_tongueOut_Group",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"Fcl_MTH_A", "Fcl_MTH_I", "Fcl_MTH_tongueOut"},
		Ratios: []float64{0.12, 0.56, 1.0},
	},
	{
		Name:   "Fcl_MTH_tongueUp_Group",
		Panel:  model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds:  []string{"Fcl_MTH_A", "Fcl_MTH_Fun", "Fcl_MTH_tongueUp"},
		Ratios: []float64{0.12, 0.54, 1.0},
	},
	{
		Name:  "mouthRoll",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"mouthRollUpper", "mouthRollLower"},
	},
	{
		Name:  "mouthShrug",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"mouthShrugUpper", "mouthShrugLower"},
	},
	{
		Name:  "mouthDimple",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"mouthDimpleRight", "mouthDimpleLeft"},
	},
	{
		Name:  "mouthPress",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"mouthPressRight", "mouthPressLeft"},
	},
	{
		Name:  "mouthSmile",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"mouthSmileRight", "mouthSmileLeft"},
	},
	{
		Name:  "mouthUpperUp",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"mouthUpperUpRight", "mouthDimpleLeft"},
	},
	{
		Name:  "cheekSquint",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"cheekSquintRight", "cheekSquintLeft"},
	},
	{
		Name:  "mouthFrown",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"mouthFrownRight", "mouthFrownLeft"},
	},
	{
		Name:  "mouthLowerDown",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"mouthLowerDownRight", "mouthLowerDownLeft"},
	},
	{
		Name:  "mouthStretch",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Binds: []string{"mouthStretchRight", "mouthStretchLeft"},
	},
	{
		Name:  "cheekPuff_R",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "cheekPuff",
	},
	{
		Name:  "cheekPuff_L",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "cheekPuff",
	},
	{
		Name:  "Fcl_HA_Fung1_Up_R",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "Fcl_HA_Fung1_Up",
	},
	{
		Name:  "Fcl_HA_Fung1_Up_L",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "Fcl_HA_Fung1_Up",
	},
	{
		Name:  "Fcl_HA_Fung1_Low_R",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "Fcl_HA_Fung1_Low",
	},
	{
		Name:  "Fcl_HA_Fung1_Low_L",
		Panel: model.MORPH_PANEL_LIP_UPPER_RIGHT,
		Split: "Fcl_HA_Fung1_Low",
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
		appendCreateMorphsFromFallbackRules(modelData, registry)
		appendMorphPairLinkFallbackRules(modelData)
	}
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
		expressionName := strings.TrimSpace(key)
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
		usedPresetName := false
		if expressionName == "" {
			expressionName = strings.TrimSpace(group.PresetName)
			usedPresetName = true
		}
		if usedPresetName {
			expressionName = resolveVrm0PresetExpressionName(expressionName)
		}
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

// resolveVrm1PresetExpressionName は VRM1 標準preset名を内部互換モーフ名へ正規化する。
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

// resolveVrm0PresetExpressionName は VRM0 標準preset名を内部互換モーフ名へ正規化する。
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

// appendCreateMorphsFromFallbackRules は旧 creates 規則に基づく頂点モーフを生成する。
func appendCreateMorphsFromFallbackRules(modelData *model.PmxModel, registry *targetMorphRegistry) {
	if modelData == nil || modelData.Morphs == nil || modelData.Vertices == nil {
		return
	}
	stats := createMorphStats{RuleCount: len(createMorphFallbackRules)}
	logVrmInfo("createsモーフ生成開始: rules=%d", stats.RuleCount)

	materialVertexMap := buildMaterialVertexIndexMap(modelData)
	morphSemanticVertexSets := buildCreateMorphSemanticVertexSets(modelData)
	materialSemanticVertexSets := buildCreateMaterialSemanticVertexSets(modelData, materialVertexMap)
	closeOffsets := collectMorphVertexOffsetsByNames(modelData, []string{"Fcl_EYE_Close"})
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

// appendMorphPairLinkFallbackRules は MORPH_PAIRS の binds/split 規則を適用する。
func appendMorphPairLinkFallbackRules(modelData *model.PmxModel) {
	if modelData == nil || modelData.Morphs == nil || modelData.Vertices == nil {
		return
	}
	if len(morphPairLinkFallbackRules) == 0 {
		return
	}
	bindApplied := 0
	splitApplied := 0
	for _, rule := range morphPairLinkFallbackRules {
		if len(rule.Binds) > 0 {
			if applyMorphPairBindRule(modelData, rule) {
				bindApplied++
			}
			continue
		}
		if strings.TrimSpace(rule.Split) == "" {
			continue
		}
		if applyMorphPairSplitRule(modelData, rule) {
			splitApplied++
		}
	}
	logVrmInfo(
		"MORPH_PAIRS binds/split適用完了: rules=%d bindsApplied=%d splitApplied=%d",
		len(morphPairLinkFallbackRules),
		bindApplied,
		splitApplied,
	)
}

// applyMorphPairBindRule は binds 規則からグループモーフを生成または更新する。
func applyMorphPairBindRule(modelData *model.PmxModel, rule morphPairLinkFallbackRule) bool {
	if modelData == nil || modelData.Morphs == nil {
		return false
	}
	offsets := buildMorphPairBindOffsets(modelData, rule)
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

// buildMorphPairBindOffsets は binds 規則のグループモーフオフセット一覧を構築する。
func buildMorphPairBindOffsets(modelData *model.PmxModel, rule morphPairLinkFallbackRule) []model.IMorphOffset {
	if modelData == nil || modelData.Morphs == nil || len(rule.Binds) == 0 {
		return nil
	}
	limit := len(rule.Binds)
	useRatios := len(rule.Ratios) > 0
	if useRatios && len(rule.Ratios) < limit {
		// 旧実装の zip(binds, ratios) と同じく短い方に合わせる。
		limit = len(rule.Ratios)
	}
	offsets := make([]model.IMorphOffset, 0, limit)
	for bindIndex := 0; bindIndex < limit; bindIndex++ {
		bindName := strings.TrimSpace(rule.Binds[bindIndex])
		if bindName == "" {
			continue
		}
		bindMorph, err := modelData.Morphs.GetByName(bindName)
		if err != nil || bindMorph == nil {
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

// applyMorphPairSplitRule は split 規則から頂点モーフを生成または更新する。
func applyMorphPairSplitRule(modelData *model.PmxModel, rule morphPairLinkFallbackRule) bool {
	if modelData == nil || modelData.Morphs == nil || modelData.Vertices == nil {
		return false
	}
	sourceName := strings.TrimSpace(rule.Split)
	if sourceName == "" {
		return false
	}
	sourceMorph, err := modelData.Morphs.GetByName(sourceName)
	if err != nil || sourceMorph == nil {
		return false
	}
	offsets := buildMorphPairSplitOffsets(modelData, sourceMorph, rule)
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

// buildMorphPairSplitOffsets は split 規則の頂点モーフオフセット一覧を構築する。
func buildMorphPairSplitOffsets(
	modelData *model.PmxModel,
	sourceMorph *model.Morph,
	rule morphPairLinkFallbackRule,
) []model.IMorphOffset {
	if modelData == nil || modelData.Vertices == nil || sourceMorph == nil {
		return nil
	}
	if sourceMorph.MorphType != model.MORPH_TYPE_VERTEX {
		return nil
	}
	if strings.Contains(rule.Name, "raiseEyelid_") {
		return buildRaiseEyelidSplitOffsets(modelData, sourceMorph.Offsets)
	}
	offsets := make([]model.IMorphOffset, 0, len(sourceMorph.Offsets))
	for _, rawOffset := range sourceMorph.Offsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil || isZeroMorphDelta(offsetData.Position) {
			continue
		}
		if !shouldIncludeMorphPairSplitVertex(modelData, offsetData.VertexIndex, rule.Name) {
			continue
		}
		ratio := resolveMorphPairSplitRatio(modelData, offsetData.VertexIndex, rule)
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

// buildRaiseEyelidSplitOffsets は raiseEyelid 系 split 規則の頂点モーフオフセットを返す。
func buildRaiseEyelidSplitOffsets(modelData *model.PmxModel, sourceOffsets []model.IMorphOffset) []model.IMorphOffset {
	if modelData == nil || modelData.Vertices == nil || len(sourceOffsets) == 0 {
		return nil
	}
	vertexYs := make([]float64, 0, len(sourceOffsets))
	for _, rawOffset := range sourceOffsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil || isZeroMorphDelta(offsetData.Position) {
			continue
		}
		vertex, err := modelData.Vertices.Get(offsetData.VertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		vertexYs = append(vertexYs, vertex.Position.Y)
	}
	if len(vertexYs) == 0 {
		return nil
	}
	minY := vertexYs[0]
	maxY := vertexYs[0]
	sumY := 0.0
	for _, y := range vertexYs {
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
		sumY += y
	}
	meanY := sumY / float64(len(vertexYs))
	minLimitY := (minY + meanY) / 2.0
	maxLimitY := (maxY + meanY) / 2.0
	offsets := make([]model.IMorphOffset, 0, len(sourceOffsets))
	for _, rawOffset := range sourceOffsets {
		offsetData, ok := rawOffset.(*model.VertexMorphOffset)
		if !ok || offsetData == nil || isZeroMorphDelta(offsetData.Position) {
			continue
		}
		vertex, err := modelData.Vertices.Get(offsetData.VertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		if vertex.Position.Y > minLimitY {
			continue
		}
		ratio := 1.0
		if vertex.Position.Y >= maxLimitY {
			ratio = calcMorphPairLinearRatio(vertex.Position.Y, minY, maxLimitY, 0.0, 1.0)
		}
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

// shouldIncludeMorphPairSplitVertex は split 先モーフ名の左右接尾辞に対応する頂点か判定する。
func shouldIncludeMorphPairSplitVertex(modelData *model.PmxModel, vertexIndex int, morphName string) bool {
	if modelData == nil || modelData.Vertices == nil || vertexIndex < 0 {
		return false
	}
	vertex, err := modelData.Vertices.Get(vertexIndex)
	if err != nil || vertex == nil {
		return false
	}
	return isCreateVertexInMorphSide(vertex.Position, morphName)
}

// resolveMorphPairSplitRatio は split 時に適用する頂点オフセット比率を返す。
func resolveMorphPairSplitRatio(modelData *model.PmxModel, vertexIndex int, rule morphPairLinkFallbackRule) float64 {
	if modelData == nil || modelData.Vertices == nil || vertexIndex < 0 {
		return 1.0
	}
	if rule.Panel != model.MORPH_PANEL_LIP_UPPER_RIGHT {
		return 1.0
	}
	vertex, err := modelData.Vertices.Get(vertexIndex)
	if err != nil || vertex == nil {
		return 1.0
	}
	absX := math.Abs(vertex.Position.X)
	if absX >= 0.2 {
		return 1.0
	}
	return calcMorphPairLinearRatio(absX, 0.0, 0.2, 0.0, 1.0)
}

// calcMorphPairLinearRatio は旧 calc_ratio 相当の線形補間値を返す。
func calcMorphPairLinearRatio(value float64, oldMin float64, oldMax float64, newMin float64, newMax float64) float64 {
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
		semanticVertices := resolveCreateSemanticVertexSet(semantic, morphSemanticVertexSets, materialSemanticVertexSets)
		for vertexIndex := range semanticVertices {
			targetVertices[vertexIndex] = struct{}{}
		}
	}
	if len(targetVertices) == 0 && (rule.Type == createMorphRuleTypeEyeSmall || rule.Type == createMorphRuleTypeEyeBig) {
		for vertexIndex := range resolveCreateEyeSurprisedOffsets(modelData, rule.Name) {
			targetVertices[vertexIndex] = struct{}{}
		}
	}
	return filterCreateVertexSetBySide(modelData, targetVertices, rule.Name)
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

// buildCreateBrowOffsets は brow_* creates 規則のオフセットを生成する。
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
		if !strings.Contains(rule.Name, "_Front") {
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

// resolveCreateBrowBaseDelta は brow 種別ごとの基本移動量を返す。
func resolveCreateBrowBaseDelta(morphName string, offsetDistance float64) mmath.Vec3 {
	switch {
	case strings.Contains(morphName, "_Below"):
		return mmath.Vec3{Vec: r3.Vec{Y: -offsetDistance}}
	case strings.Contains(morphName, "_Abobe"):
		return mmath.Vec3{Vec: r3.Vec{Y: offsetDistance}}
	case strings.Contains(morphName, "_Left"):
		return mmath.Vec3{Vec: r3.Vec{X: offsetDistance}}
	case strings.Contains(morphName, "_Right"):
		return mmath.Vec3{Vec: r3.Vec{X: -offsetDistance}}
	case strings.Contains(morphName, "_Front"):
		return mmath.Vec3{Vec: r3.Vec{Z: -offsetDistance}}
	default:
		return mmath.ZERO_VEC3
	}
}

// buildCreateEyeScaleOffsets は eye_Small/eye_Big のオフセットを生成する。
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

// buildCreateEyeHideOffsets は eye_Hide_Vertex のオフセットを生成する。
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
		morphedPos := vertex.Position.Added(offset)
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
			createMorphEyeHideProjectionZOffset,
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

// resolveCreateEyeSurprisedOffsets は eye_Small/eye_Big 用基準オフセットを返す。
func resolveCreateEyeSurprisedOffsets(modelData *model.PmxModel, morphName string) map[int]mmath.Vec3 {
	candidates := []string{"Fcl_EYE_Surprised"}
	if strings.HasSuffix(morphName, "_R") {
		candidates = []string{"Fcl_EYE_Surprised_R", "Fcl_EYE_Surprised"}
	}
	if strings.HasSuffix(morphName, "_L") {
		candidates = []string{"Fcl_EYE_Surprised_L", "Fcl_EYE_Surprised"}
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
	if strings.HasSuffix(morphName, "_R") {
		return position.X < 0
	}
	if strings.HasSuffix(morphName, "_L") {
		return position.X > 0
	}
	return true
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
	material.Memo = "VRM primitive"
	material.Diffuse = mmath.Vec4{X: 1.0, Y: 1.0, Z: 1.0, W: 1.0}
	material.Specular = mmath.Vec4{X: 0.0, Y: 0.0, Z: 0.0, W: 1.0}
	material.Ambient = mmath.Vec3{Vec: r3.Vec{X: 0.5, Y: 0.5, Z: 0.5}}
	material.Edge = mmath.Vec4{X: 0.0, Y: 0.0, Z: 0.0, W: 1.0}
	material.EdgeSize = 1.0
	material.TextureFactor = mmath.ONE_VEC4
	material.SphereTextureFactor = mmath.ONE_VEC4
	material.ToonTextureFactor = mmath.ONE_VEC4
	material.DrawFlag = model.DRAW_FLAG_GROUND_SHADOW | model.DRAW_FLAG_DRAWING_ON_SELF_SHADOW_MAPS | model.DRAW_FLAG_DRAWING_SELF_SHADOWS
	material.VerticesCount = verticesCount
	material.TextureIndex = resolveMaterialTextureIndex(doc, primitive, textureIndexesByImage)

	if primitive.Material != nil {
		materialIndex := *primitive.Material
		if materialIndex >= 0 && materialIndex < len(doc.Materials) {
			sourceMaterial := doc.Materials[materialIndex]
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
		}
	}

	materialIndex := modelData.Materials.AppendRaw(material)
	registerExpressionMaterialIndex(registry, doc, primitive.Material, material.Name(), materialIndex)
	return materialIndex
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
