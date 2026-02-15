// 指示: miu200521358
package minteractor

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/collection"
	"github.com/miu200521358/mlib_go/pkg/domain/model/merrors"
	"github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
	"gonum.org/v1/gonum/spatial/r3"
)

const (
	leftToeHumanTargetName  = "左つま先"
	rightToeHumanTargetName = "右つま先"
	boneRenameTempPrefix    = "__mu_vrm2pmx_tmp_"
	kneeDepthOffsetRate     = 0.01
	weightSignEpsilon       = 1e-8
	tongueUvXThreshold      = 0.5
	tongueUvYThreshold      = 0.5
	tongueBone2RatioDefault = 0.3
	tongueBone3RatioDefault = 0.5
	tongueBone4RatioDefault = 0.8
	leftWristTipName        = "左手首先"
	rightWristTipName       = "右手首先"
	leftThumbTipName        = "左親指先"
	rightThumbTipName       = "右親指先"
	leftIndexTipName        = "左人指先"
	rightIndexTipName       = "右人指先"
	leftMiddleTipName       = "左中指先"
	rightMiddleTipName      = "右中指先"
	leftRingTipName         = "左薬指先"
	rightRingTipName        = "右薬指先"
	leftPinkyTipName        = "左小指先"
	rightPinkyTipName       = "右小指先"
	tongueBone1Name         = "舌1"
	tongueBone2Name         = "舌2"
	tongueBone3Name         = "舌3"
	tongueBone4Name         = "舌4"
	tongueMaterialHint      = "facemouth"
)

// tongueBoneMorphRule は舌ボーン系モーフの構成規則を表す。
type tongueBoneMorphRule struct {
	MorphNames []string
	Offsets    []tongueBoneMorphOffsetRule
}

// tongueBoneMorphOffsetRule は舌ボーン系モーフ1オフセット分の規則を表す。
type tongueBoneMorphOffsetRule struct {
	BoneName string
	Move     mmath.Vec3
	Rotate   mmath.Quaternion
}

// tongueBoneMorphRules は口系ボーンモーフを舌ボーン系列へ再構成する規則を表す。
var tongueBoneMorphRules = []tongueBoneMorphRule{
	{
		MorphNames: []string{"あボーン", "Fcl_MTH_A_Bone"},
		Offsets: []tongueBoneMorphOffsetRule{
			newTongueBoneMorphOffsetRule(tongueBone1Name, 0, 0, 0, -16, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone2Name, 0, 0, 0, -16, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone3Name, 0, 0, 0, -10, 0, 0),
		},
	},
	{
		MorphNames: []string{"いボーン", "Fcl_MTH_I_Bone"},
		Offsets: []tongueBoneMorphOffsetRule{
			newTongueBoneMorphOffsetRule(tongueBone1Name, 0, 0, 0, -6, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone2Name, 0, 0, 0, -6, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone3Name, 0, 0, 0, -3, 0, 0),
		},
	},
	{
		MorphNames: []string{"うボーン", "Fcl_MTH_U_Bone"},
		Offsets: []tongueBoneMorphOffsetRule{
			newTongueBoneMorphOffsetRule(tongueBone1Name, 0, 0, 0, -16, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone2Name, 0, 0, 0, -16, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone3Name, 0, 0, 0, -10, 0, 0),
		},
	},
	{
		MorphNames: []string{"えボーン", "Fcl_MTH_E_Bone"},
		Offsets: []tongueBoneMorphOffsetRule{
			newTongueBoneMorphOffsetRule(tongueBone1Name, 0, 0, 0, -6, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone2Name, 0, 0, 0, -6, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone3Name, 0, 0, 0, -3, 0, 0),
		},
	},
	{
		MorphNames: []string{"おボーン", "Fcl_MTH_O_Bone"},
		Offsets: []tongueBoneMorphOffsetRule{
			newTongueBoneMorphOffsetRule(tongueBone1Name, 0, 0, 0, -20, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone2Name, 0, 0, 0, -18, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone3Name, 0, 0, 0, -12, 0, 0),
		},
	},
	{
		MorphNames: []string{"ワボーン", "Fcl_MTH_Joy_Bone"},
		Offsets: []tongueBoneMorphOffsetRule{
			newTongueBoneMorphOffsetRule(tongueBone1Name, 0, 0, 0, -24, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone2Name, 0, 0, 0, -24, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone3Name, 0, 0, 0, 16, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone4Name, 0, 0, 0, 28, 0, 0),
		},
	},
	{
		MorphNames: []string{"▲ボーン", "Fcl_MTH_Sorrow_Bone"},
		Offsets: []tongueBoneMorphOffsetRule{
			newTongueBoneMorphOffsetRule(tongueBone1Name, 0, 0, 0, -6, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone2Name, 0, 0, 0, -6, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone3Name, 0, 0, 0, -3, 0, 0),
		},
	},
	{
		MorphNames: []string{"わーボーン", "Fcl_MTH_Surprised_Bone"},
		Offsets: []tongueBoneMorphOffsetRule{
			newTongueBoneMorphOffsetRule(tongueBone1Name, 0, 0, 0, -24, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone2Name, 0, 0, 0, -24, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone3Name, 0, 0, 0, 16, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone4Name, 0, 0, 0, 28, 0, 0),
		},
	},
	{
		MorphNames: []string{"べーボーン", "Fcl_MTH_tongueOut"},
		Offsets: []tongueBoneMorphOffsetRule{
			newTongueBoneMorphOffsetRule(tongueBone1Name, 0, 0, 0, -9, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone2Name, 0, 0, -0.24, -13.2, 0, 0),
			newTongueBoneMorphOffsetRule(tongueBone3Name, 0, 0, 0, -23.2, 0, 0),
		},
	},
	{
		MorphNames: []string{"ぺろりボーン", "Fcl_MTH_tongueUp"},
		Offsets: []tongueBoneMorphOffsetRule{
			newTongueBoneMorphOffsetRule(tongueBone1Name, 0, 0, 0, 0, -5, 0),
			newTongueBoneMorphOffsetRule(tongueBone2Name, 0, -0.03, -0.18, 33, -16, -4),
			newTongueBoneMorphOffsetRule(tongueBone3Name, 0, 0, 0, 15, 3.6, -1),
			newTongueBoneMorphOffsetRule(tongueBone4Name, 0, 0, 0, 20, 0, 0),
		},
	},
}

// newTongueBoneMorphOffsetRule は舌ボーン系モーフオフセット規則を生成する。
func newTongueBoneMorphOffsetRule(
	boneName string,
	moveX float64,
	moveY float64,
	moveZ float64,
	rotateX float64,
	rotateY float64,
	rotateZ float64,
) tongueBoneMorphOffsetRule {
	return tongueBoneMorphOffsetRule{
		BoneName: boneName,
		Move:     mmath.Vec3{Vec: r3.Vec{X: moveX, Y: moveY, Z: moveZ}},
		Rotate:   mmath.NewQuaternionFromDegrees(rotateX, rotateY, rotateZ),
	}
}

// tongueRatioConfig は舌ボーン配置とウェイト分割の比率設定を表す。
type tongueRatioConfig struct {
	Bone2Ratio float64
	Bone3Ratio float64
	Bone4Ratio float64
}

// explicitRemoveBoneNames は明示削除対象ボーン名を保持する。
var explicitRemoveBoneNames = map[string]struct{}{
	"face":      {},
	"body":      {},
	"hair":      {},
	"secondary": {},
}

const (
	viewerIdealDisplaySlotRootName     = "Root"
	viewerIdealDisplaySlotMorphName    = "表情"
	viewerIdealDisplaySlotCenterName   = "センター"
	viewerIdealDisplaySlotTrunkName    = "体幹"
	viewerIdealDisplaySlotFaceName     = "顔"
	viewerIdealDisplaySlotBustName     = "胸"
	viewerIdealDisplaySlotLeftArmName  = "左手"
	viewerIdealDisplaySlotLeftFgrName  = "左指"
	viewerIdealDisplaySlotRightArmName = "右手"
	viewerIdealDisplaySlotRightFgrName = "右指"
	viewerIdealDisplaySlotLeftLegName  = "左足"
	viewerIdealDisplaySlotRightLegName = "右足"
	viewerIdealDisplaySlotHairName     = "髪"
	viewerIdealDisplaySlotOtherName    = "その他"
)

// viewerIdealFixedDisplaySlotSpec は固定表示枠定義を表す。
type viewerIdealFixedDisplaySlotSpec struct {
	Name        string
	EnglishName string
	BoneNames   []string
}

// viewerIdealFixedDisplaySlotSpecs は固定表示枠の生成順を保持する。
var viewerIdealFixedDisplaySlotSpecs = []viewerIdealFixedDisplaySlotSpec{
	{
		Name:        viewerIdealDisplaySlotRootName,
		EnglishName: "Root",
		BoneNames:   []string{model.ROOT.String()},
	},
	{
		Name:        viewerIdealDisplaySlotCenterName,
		EnglishName: "Center",
		BoneNames: []string{
			model.CENTER.String(),
			model.GROOVE.String(),
		},
	},
	{
		Name:        viewerIdealDisplaySlotTrunkName,
		EnglishName: "Trunk",
		BoneNames: []string{
			model.WAIST.String(),
			model.LOWER.String(),
			model.UPPER.String(),
			model.UPPER2.String(),
			model.NECK.String(),
			model.HEAD.String(),
		},
	},
	{
		Name:        viewerIdealDisplaySlotFaceName,
		EnglishName: "Face",
		BoneNames: []string{
			model.EYES.String(),
			model.EYE.Left(),
			model.EYE.Right(),
			"両目光",
			"左目光",
			"右目光",
			tongueBone1Name,
			tongueBone2Name,
			tongueBone3Name,
			tongueBone4Name,
		},
	},
	{
		Name:        viewerIdealDisplaySlotBustName,
		EnglishName: "Bust",
		BoneNames: []string{
			"左胸",
			"右胸",
		},
	},
	{
		Name:        viewerIdealDisplaySlotLeftArmName,
		EnglishName: "LeftArm",
		BoneNames: []string{
			model.SHOULDER_P.Left(),
			model.SHOULDER.Left(),
			model.ARM.Left(),
			model.ARM_TWIST.Left(),
			model.ELBOW.Left(),
			model.WRIST_TWIST.Left(),
			model.WRIST.Left(),
		},
	},
	{
		Name:        viewerIdealDisplaySlotLeftFgrName,
		EnglishName: "LeftFinger",
		BoneNames: []string{
			model.THUMB0.Left(),
			model.THUMB1.Left(),
			model.THUMB2.Left(),
			model.INDEX1.Left(),
			model.INDEX2.Left(),
			model.INDEX3.Left(),
			model.MIDDLE1.Left(),
			model.MIDDLE2.Left(),
			model.MIDDLE3.Left(),
			model.RING1.Left(),
			model.RING2.Left(),
			model.RING3.Left(),
			model.PINKY1.Left(),
			model.PINKY2.Left(),
			model.PINKY3.Left(),
		},
	},
	{
		Name:        viewerIdealDisplaySlotRightArmName,
		EnglishName: "RightArm",
		BoneNames: []string{
			model.SHOULDER_P.Right(),
			model.SHOULDER.Right(),
			model.ARM.Right(),
			model.ARM_TWIST.Right(),
			model.ELBOW.Right(),
			model.WRIST_TWIST.Right(),
			model.WRIST.Right(),
		},
	},
	{
		Name:        viewerIdealDisplaySlotRightFgrName,
		EnglishName: "RightFinger",
		BoneNames: []string{
			model.THUMB0.Right(),
			model.THUMB1.Right(),
			model.THUMB2.Right(),
			model.INDEX1.Right(),
			model.INDEX2.Right(),
			model.INDEX3.Right(),
			model.MIDDLE1.Right(),
			model.MIDDLE2.Right(),
			model.MIDDLE3.Right(),
			model.RING1.Right(),
			model.RING2.Right(),
			model.RING3.Right(),
			model.PINKY1.Right(),
			model.PINKY2.Right(),
			model.PINKY3.Right(),
		},
	},
	{
		Name:        viewerIdealDisplaySlotLeftLegName,
		EnglishName: "LeftLeg",
		BoneNames: []string{
			model.LEG.Left(),
			model.KNEE.Left(),
			model.ANKLE.Left(),
			leftToeHumanTargetName,
			model.LEG_IK_PARENT.Left(),
			model.LEG_IK.Left(),
			model.TOE_IK.Left(),
			model.LEG_D.Left(),
			model.KNEE_D.Left(),
			model.ANKLE_D.Left(),
			model.TOE_EX.Left(),
		},
	},
	{
		Name:        viewerIdealDisplaySlotRightLegName,
		EnglishName: "RightLeg",
		BoneNames: []string{
			model.LEG.Right(),
			model.KNEE.Right(),
			model.ANKLE.Right(),
			rightToeHumanTargetName,
			model.LEG_IK_PARENT.Right(),
			model.LEG_IK.Right(),
			model.TOE_IK.Right(),
			model.LEG_D.Right(),
			model.KNEE_D.Right(),
			model.ANKLE_D.Right(),
			model.TOE_EX.Right(),
		},
	},
	{
		Name:        viewerIdealDisplaySlotHairName,
		EnglishName: "Hair",
		BoneNames:   []string{},
	},
}

// standardBoneEnglishTemplates は標準ボーン名から英名テンプレートへの対応を保持する。
var standardBoneEnglishTemplates = map[model.StandardBoneName]string{
	model.ROOT:           "Root",
	model.CENTER:         "Center",
	model.GROOVE:         "Groove",
	model.BODY_AXIS:      "BodyAxis",
	model.TRUNK_ROOT:     "TrunkRoot",
	model.WAIST:          "Waist",
	model.LOWER_ROOT:     "LowerRoot",
	model.LOWER:          "LowerBody",
	model.HIP:            "{Side}Hip",
	model.LEG_CENTER:     "LegCenter",
	model.UPPER_ROOT:     "UpperRoot",
	model.UPPER:          "UpperBody",
	model.UPPER2:         "UpperBody2",
	model.NECK_ROOT:      "NeckRoot",
	model.NECK:           "Neck",
	model.HEAD:           "Head",
	model.HEAD_TAIL:      "HeadTip",
	model.EYES:           "Eyes",
	model.EYE:            "{Side}Eye",
	model.SHOULDER_ROOT:  "{Side}ShoulderRoot",
	model.SHOULDER_P:     "{Side}ShoulderP",
	model.SHOULDER:       "{Side}Shoulder",
	model.SHOULDER_C:     "{Side}ShoulderC",
	model.ARM:            "{Side}Arm",
	model.ARM_TWIST:      "{Side}ArmTwist",
	model.ARM_TWIST1:     "{Side}ArmTwist1",
	model.ARM_TWIST2:     "{Side}ArmTwist2",
	model.ARM_TWIST3:     "{Side}ArmTwist3",
	model.ELBOW:          "{Side}Elbow",
	model.WRIST_TWIST:    "{Side}WristTwist",
	model.WRIST_TWIST1:   "{Side}WristTwist1",
	model.WRIST_TWIST2:   "{Side}WristTwist2",
	model.WRIST_TWIST3:   "{Side}WristTwist3",
	model.WRIST:          "{Side}Wrist",
	model.WRIST_TAIL:     "{Side}WristTip",
	model.THUMB0:         "{Side}Thumb0",
	model.THUMB1:         "{Side}Thumb1",
	model.THUMB2:         "{Side}Thumb2",
	model.THUMB_TAIL:     "{Side}ThumbTip",
	model.INDEX1:         "{Side}Index1",
	model.INDEX2:         "{Side}Index2",
	model.INDEX3:         "{Side}Index3",
	model.INDEX_TAIL:     "{Side}IndexTip",
	model.MIDDLE1:        "{Side}Middle1",
	model.MIDDLE2:        "{Side}Middle2",
	model.MIDDLE3:        "{Side}Middle3",
	model.MIDDLE_TAIL:    "{Side}MiddleTip",
	model.RING1:          "{Side}Ring1",
	model.RING2:          "{Side}Ring2",
	model.RING3:          "{Side}Ring3",
	model.RING_TAIL:      "{Side}RingTip",
	model.PINKY1:         "{Side}Little1",
	model.PINKY2:         "{Side}Little2",
	model.PINKY3:         "{Side}Little3",
	model.PINKY_TAIL:     "{Side}LittleTip",
	model.WAIST_CANCEL:   "WaistCancel{Side}",
	model.LEG_ROOT:       "{Side}LegRoot",
	model.LEG:            "{Side}Leg",
	model.KNEE:           "{Side}Knee",
	model.ANKLE:          "{Side}Ankle",
	model.ANKLE_GROUND:   "{Side}AnkleGround",
	model.HEEL:           "{Side}Heel",
	model.TOE_T:          "{Side}ToeTip",
	model.TOE_P:          "{Side}ToeParent",
	model.TOE_C:          "{Side}ToeChild",
	model.LEG_D:          "{Side}LegD",
	model.KNEE_D:         "{Side}KneeD",
	model.HEEL_D:         "{Side}HeelD",
	model.ANKLE_D:        "{Side}AnkleD",
	model.ANKLE_D_GROUND: "{Side}AnkleGroundD",
	model.TOE_T_D:        "{Side}ToeTipD",
	model.TOE_P_D:        "{Side}ToeParentD",
	model.TOE_C_D:        "{Side}ToeChildD",
	model.TOE_EX:         "{Side}ToeEX",
	model.LEG_IK_PARENT:  "{Side}LegIKParent",
	model.LEG_IK:         "{Side}LegIK",
	model.TOE_IK:         "{Side}ToeIK",
}

// standardBoneFlagOverrides は標準ボーンのフラグ固定値を保持する。
var standardBoneFlagOverrides = map[model.StandardBoneName]model.BoneFlag{
	model.ROOT:          model.BoneFlag(0x001E),
	model.CENTER:        model.BoneFlag(0x001E),
	model.GROOVE:        model.BoneFlag(0x001E),
	model.WAIST:         model.BoneFlag(0x001E),
	model.LEG_IK_PARENT: model.BoneFlag(0x001E),
	model.LEG_IK:        model.BoneFlag(0x003F),
	model.TOE_IK:        model.BoneFlag(0x003E),
	model.SHOULDER_C:    model.BoneFlag(0x0102),
	model.WAIST_CANCEL:  model.BoneFlag(0x0102),
	model.ARM_TWIST1:    model.BoneFlag(0x0100),
	model.ARM_TWIST2:    model.BoneFlag(0x0100),
	model.ARM_TWIST3:    model.BoneFlag(0x0100),
	model.WRIST_TWIST1:  model.BoneFlag(0x0100),
	model.WRIST_TWIST2:  model.BoneFlag(0x0100),
	model.WRIST_TWIST3:  model.BoneFlag(0x0100),
	model.LEG_D:         model.BoneFlag(0x011A),
	model.KNEE_D:        model.BoneFlag(0x011A),
	model.ANKLE_D:       model.BoneFlag(0x011A),
	model.ARM_TWIST:     model.BoneFlag(0x0C1A),
	model.WRIST_TWIST:   model.BoneFlag(0x0C1A),
	model.WRIST_TAIL:    model.BoneFlag(0x0002),
	model.THUMB_TAIL:    model.BoneFlag(0x0012),
	model.INDEX_TAIL:    model.BoneFlag(0x0012),
	model.MIDDLE_TAIL:   model.BoneFlag(0x0012),
	model.RING_TAIL:     model.BoneFlag(0x0012),
	model.PINKY_TAIL:    model.BoneFlag(0x0012),
}

// standardBoneEnglishByName は標準ボーン名から解決済み英名の辞書を保持する。
var standardBoneEnglishByName = buildStandardBoneEnglishByName()

// standardBoneFlagOverrideByName は標準ボーン名から解決済みフラグ固定値の辞書を保持する。
var standardBoneFlagOverrideByName = buildStandardBoneFlagOverrideByName()

// humanoidRenameRule は humanoid 名から PMX ボーン名への変換ルールを表す。
type humanoidRenameRule struct {
	HumanoidName string
	TargetName   string
	Priority     int
}

// renamePlanEntry は命名変更計画の1件を表す。
type renamePlanEntry struct {
	SourceIndex int
	TargetName  string
}

// selectedHumanoidNode は同名競合時に採用するノード情報を表す。
type selectedHumanoidNode struct {
	NodeIndex int
	Priority  int
}

// weightReplaceRule はウェイト置換ルールを表す。
type weightReplaceRule struct {
	FromIndex int
	ToIndex   int
}

// twistWeightSegment は捩りウェイト分割1区間を表す。
type twistWeightSegment struct {
	FromIndex int
	ToIndex   int
	FromX     float64
	ToX       float64
}

// twistWeightChain は捩りウェイト分割1系列を表す。
type twistWeightChain struct {
	BaseFromX        float64
	BaseDistance     float64
	CandidateIndexes []int
	Segments         []twistWeightSegment
}

// weightedJoint は頂点ウェイト正規化用のジョイント情報を表す。
type weightedJoint struct {
	Index  int
	Weight float64
}

// indexedBoneRename は index 指定のボーン名変更情報を表す。
type indexedBoneRename struct {
	Index   int
	NewName string
}

var humanoidRenameRules = []humanoidRenameRule{
	{HumanoidName: "hips", TargetName: model.LOWER.String(), Priority: 0},
	{HumanoidName: "spine", TargetName: model.UPPER.String(), Priority: 0},
	{HumanoidName: "chest", TargetName: model.UPPER2.String(), Priority: 5},
	{HumanoidName: "upperchest", TargetName: model.UPPER2.String(), Priority: 10},
	{HumanoidName: "neck", TargetName: model.NECK.String(), Priority: 0},
	{HumanoidName: "head", TargetName: model.HEAD.String(), Priority: 0},
	{HumanoidName: "leftshoulder", TargetName: model.SHOULDER.Left(), Priority: 0},
	{HumanoidName: "rightshoulder", TargetName: model.SHOULDER.Right(), Priority: 0},
	{HumanoidName: "leftupperarm", TargetName: model.ARM.Left(), Priority: 0},
	{HumanoidName: "rightupperarm", TargetName: model.ARM.Right(), Priority: 0},
	{HumanoidName: "leftlowerarm", TargetName: model.ELBOW.Left(), Priority: 0},
	{HumanoidName: "rightlowerarm", TargetName: model.ELBOW.Right(), Priority: 0},
	{HumanoidName: "lefthand", TargetName: model.WRIST.Left(), Priority: 0},
	{HumanoidName: "righthand", TargetName: model.WRIST.Right(), Priority: 0},
	{HumanoidName: "leftupperleg", TargetName: model.LEG.Left(), Priority: 0},
	{HumanoidName: "rightupperleg", TargetName: model.LEG.Right(), Priority: 0},
	{HumanoidName: "leftlowerleg", TargetName: model.KNEE.Left(), Priority: 0},
	{HumanoidName: "rightlowerleg", TargetName: model.KNEE.Right(), Priority: 0},
	{HumanoidName: "leftfoot", TargetName: model.ANKLE.Left(), Priority: 0},
	{HumanoidName: "rightfoot", TargetName: model.ANKLE.Right(), Priority: 0},
	{HumanoidName: "lefttoes", TargetName: leftToeHumanTargetName, Priority: 0},
	{HumanoidName: "righttoes", TargetName: rightToeHumanTargetName, Priority: 0},
	{HumanoidName: "lefteye", TargetName: model.EYE.Left(), Priority: 0},
	{HumanoidName: "righteye", TargetName: model.EYE.Right(), Priority: 0},
	{HumanoidName: "jaw", TargetName: "あご", Priority: 0},
	{HumanoidName: "leftthumbmetacarpal", TargetName: model.THUMB0.Left(), Priority: 0},
	{HumanoidName: "leftthumbproximal", TargetName: model.THUMB1.Left(), Priority: 0},
	{HumanoidName: "leftthumbintermediate", TargetName: model.THUMB2.Left(), Priority: 0},
	{HumanoidName: "leftthumbdistal", TargetName: model.THUMB2.Left(), Priority: -1},
	{HumanoidName: "rightthumbmetacarpal", TargetName: model.THUMB0.Right(), Priority: 0},
	{HumanoidName: "rightthumbproximal", TargetName: model.THUMB1.Right(), Priority: 0},
	{HumanoidName: "rightthumbintermediate", TargetName: model.THUMB2.Right(), Priority: 0},
	{HumanoidName: "rightthumbdistal", TargetName: model.THUMB2.Right(), Priority: -1},
	{HumanoidName: "leftindexproximal", TargetName: model.INDEX1.Left(), Priority: 0},
	{HumanoidName: "leftindexintermediate", TargetName: model.INDEX2.Left(), Priority: 0},
	{HumanoidName: "leftindexdistal", TargetName: model.INDEX3.Left(), Priority: 0},
	{HumanoidName: "rightindexproximal", TargetName: model.INDEX1.Right(), Priority: 0},
	{HumanoidName: "rightindexintermediate", TargetName: model.INDEX2.Right(), Priority: 0},
	{HumanoidName: "rightindexdistal", TargetName: model.INDEX3.Right(), Priority: 0},
	{HumanoidName: "leftmiddleproximal", TargetName: model.MIDDLE1.Left(), Priority: 0},
	{HumanoidName: "leftmiddleintermediate", TargetName: model.MIDDLE2.Left(), Priority: 0},
	{HumanoidName: "leftmiddledistal", TargetName: model.MIDDLE3.Left(), Priority: 0},
	{HumanoidName: "rightmiddleproximal", TargetName: model.MIDDLE1.Right(), Priority: 0},
	{HumanoidName: "rightmiddleintermediate", TargetName: model.MIDDLE2.Right(), Priority: 0},
	{HumanoidName: "rightmiddledistal", TargetName: model.MIDDLE3.Right(), Priority: 0},
	{HumanoidName: "leftringproximal", TargetName: model.RING1.Left(), Priority: 0},
	{HumanoidName: "leftringintermediate", TargetName: model.RING2.Left(), Priority: 0},
	{HumanoidName: "leftringdistal", TargetName: model.RING3.Left(), Priority: 0},
	{HumanoidName: "rightringproximal", TargetName: model.RING1.Right(), Priority: 0},
	{HumanoidName: "rightringintermediate", TargetName: model.RING2.Right(), Priority: 0},
	{HumanoidName: "rightringdistal", TargetName: model.RING3.Right(), Priority: 0},
	{HumanoidName: "leftlittleproximal", TargetName: model.PINKY1.Left(), Priority: 0},
	{HumanoidName: "leftlittleintermediate", TargetName: model.PINKY2.Left(), Priority: 0},
	{HumanoidName: "leftlittledistal", TargetName: model.PINKY3.Left(), Priority: 0},
	{HumanoidName: "rightlittleproximal", TargetName: model.PINKY1.Right(), Priority: 0},
	{HumanoidName: "rightlittleintermediate", TargetName: model.PINKY2.Right(), Priority: 0},
	{HumanoidName: "rightlittledistal", TargetName: model.PINKY3.Right(), Priority: 0},
}

// applyHumanoidBoneMappingAfterReorder は材質並べ替え後に不足ボーン追加と命名変更を適用する。
func applyHumanoidBoneMappingAfterReorder(modelData *ModelData) error {
	if modelData == nil || modelData.Bones == nil || modelData.VrmData == nil {
		return nil
	}

	humanoid := collectHumanoidNodeIndexes(modelData.VrmData)
	if len(humanoid) == 0 {
		return nil
	}

	plan := buildHumanoidRenamePlan(humanoid)
	targetBoneIndexes := resolveTargetBoneIndexes(modelData, plan)
	if err := ensureSupplementBones(modelData, targetBoneIndexes); err != nil {
		return err
	}
	if err := renameHumanoidBones(modelData.Bones, targetBoneIndexes, plan); err != nil {
		return err
	}
	applyKneeDepthOffset(modelData, targetBoneIndexes)
	applyVroidWeightTransfer(modelData, targetBoneIndexes)
	applyTongueWeightsAndBones(modelData)
	applyTongueBoneMorphRules(modelData)
	normalizeMappedRootParents(modelData.Bones)
	if err := normalizeViewerIdealBoneStructure(modelData); err != nil {
		return err
	}
	if err := removeExplicitBonesAndReindex(modelData); err != nil {
		return err
	}
	if err := normalizeBoneNamesAndEnglish(modelData.Bones); err != nil {
		return err
	}
	normalizeViewerIdealBoneOrder(modelData)
	normalizeStandardBoneFlags(modelData.Bones)
	applyViewerIdealDisplaySlots(modelData)
	return nil
}

// applyTongueBoneMorphRules は口系ボーンモーフを舌ボーン系列参照へ再構成する。
func applyTongueBoneMorphRules(modelData *ModelData) {
	if modelData == nil || modelData.Morphs == nil || modelData.Bones == nil {
		return
	}
	if len(tongueBoneMorphRules) == 0 {
		return
	}
	tongueBoneIndexes := map[string]int{}
	for _, tongueName := range []string{tongueBone1Name, tongueBone2Name, tongueBone3Name, tongueBone4Name} {
		tongueBone, err := modelData.Bones.GetByName(tongueName)
		if err != nil || tongueBone == nil {
			continue
		}
		tongueBoneIndexes[tongueName] = tongueBone.Index()
	}
	if len(tongueBoneIndexes) == 0 {
		return
	}

	for _, rule := range tongueBoneMorphRules {
		morphData := findMorphByNames(modelData.Morphs, rule.MorphNames)
		if morphData == nil {
			continue
		}
		offsets := make([]model.IMorphOffset, 0, len(rule.Offsets))
		for _, offsetRule := range rule.Offsets {
			boneIndex, exists := tongueBoneIndexes[offsetRule.BoneName]
			if !exists {
				continue
			}
			offsets = append(offsets, &model.BoneMorphOffset{
				BoneIndex: boneIndex,
				Position:  offsetRule.Move,
				Rotation:  offsetRule.Rotate,
			})
		}
		if len(offsets) == 0 {
			continue
		}
		morphData.Panel = model.MORPH_PANEL_SYSTEM
		morphData.MorphType = model.MORPH_TYPE_BONE
		morphData.Offsets = offsets
	}
}

// findMorphByNames は候補名配列から最初に見つかったモーフを返す。
func findMorphByNames(morphs *collection.NamedCollection[*model.Morph], names []string) *model.Morph {
	if morphs == nil || len(names) == 0 {
		return nil
	}
	for _, name := range names {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}
		morphData, err := morphs.GetByName(trimmedName)
		if err == nil && morphData != nil {
			return morphData
		}
	}
	return nil
}

// applyKneeDepthOffset はひざ/ひざDを足首距離の1%だけ手前(-Z)へ移動する。
func applyKneeDepthOffset(modelData *ModelData, targetBoneIndexes map[string]int) {
	if modelData == nil || modelData.Bones == nil {
		return
	}
	for _, direction := range []model.BoneDirection{model.BONE_DIRECTION_LEFT, model.BONE_DIRECTION_RIGHT} {
		knee, kneeOK := getBoneByTargetName(modelData, targetBoneIndexes, model.KNEE.StringFromDirection(direction))
		ankle, ankleOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ANKLE.StringFromDirection(direction))
		if !kneeOK || !ankleOK {
			continue
		}
		offset := knee.Position.Distance(ankle.Position) * kneeDepthOffsetRate
		if offset <= 0 {
			continue
		}
		knee.Position.Z -= offset
		if kneeD, kneeDOK := getBoneByTargetName(modelData, targetBoneIndexes, model.KNEE_D.StringFromDirection(direction)); kneeDOK {
			kneeD.Position.Z -= offset
		}
	}
}

// applyVroidWeightTransfer はVRoid準拠のD系/捩りウェイト乗せ換えを適用する。
func applyVroidWeightTransfer(modelData *ModelData, targetBoneIndexes map[string]int) {
	if modelData == nil || modelData.Bones == nil || modelData.Vertices == nil {
		return
	}

	replaceRules := buildWeightReplaceRules(modelData, targetBoneIndexes)
	twistChains := buildTwistWeightChains(modelData, targetBoneIndexes)
	for _, vertex := range modelData.Vertices.Values() {
		if vertex == nil || vertex.Deform == nil {
			continue
		}
		joints := append([]int(nil), vertex.Deform.Indexes()...)
		weights := append([]float64(nil), vertex.Deform.Weights()...)
		maxCount := len(joints)
		if len(weights) < maxCount {
			maxCount = len(weights)
		}
		if maxCount <= 0 {
			continue
		}
		joints = joints[:maxCount]
		weights = weights[:maxCount]

		applyDirectWeightReplaceRules(joints, replaceRules)
		for _, chain := range twistChains {
			applyTwistWeightChain(vertex.Position.X, &joints, &weights, chain)
		}

		fallbackIndex := resolveFallbackBoneIndex(joints)
		vertex.Deform = buildNormalizedDeform(joints, weights, fallbackIndex)
		vertex.DeformType = vertex.Deform.DeformType()
	}
}

// applyTongueWeightsAndBones は FaceMouth 材質の舌頂点を舌ボーン系列へ再割当する。
func applyTongueWeightsAndBones(modelData *ModelData) {
	if modelData == nil || modelData.Bones == nil || modelData.Materials == nil || modelData.Vertices == nil {
		return
	}
	if modelData.Faces == nil || modelData.Materials.Len() == 0 || modelData.Faces.Len() == 0 {
		return
	}
	normalizeTongueBoneRelations(modelData.Bones)
	tongue1, tongue1OK := getBoneByName(modelData.Bones, tongueBone1Name)
	tongue2, tongue2OK := getBoneByName(modelData.Bones, tongueBone2Name)
	tongue3, tongue3OK := getBoneByName(modelData.Bones, tongueBone3Name)
	tongue4, tongue4OK := getBoneByName(modelData.Bones, tongueBone4Name)
	if !tongue1OK || !tongue2OK || !tongue3OK || !tongue4OK {
		return
	}

	tongueMaterialIndex, hasTongueMaterial := resolveTongueMaterialIndex(modelData.Materials)
	if !hasTongueMaterial {
		return
	}
	tongueVertexIndexes := collectTongueVertexIndexesFromMaterial(modelData, tongueMaterialIndex)
	if len(tongueVertexIndexes) == 0 {
		return
	}

	tongueVertices := make([]*model.Vertex, 0, len(tongueVertexIndexes))
	tonguePositions := make([]mmath.Vec3, 0, len(tongueVertexIndexes))
	for _, vertexIndex := range tongueVertexIndexes {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		vertex.Deform = model.NewBdef1(tongue1.Index())
		vertex.DeformType = model.BDEF1
		tongueVertices = append(tongueVertices, vertex)
		tonguePositions = append(tonguePositions, vertex.Position)
	}
	if len(tongueVertices) == 0 {
		return
	}

	frontPos, backPos := resolveTongueFrontAndBackPositions(tonguePositions)
	defaultRatio := tongueRatioConfig{
		Bone2Ratio: tongueBone2RatioDefault,
		Bone3Ratio: tongueBone3RatioDefault,
		Bone4Ratio: tongueBone4RatioDefault,
	}
	applyTongueBoneRatios(tongue1, tongue2, tongue3, tongue4, frontPos, backPos, defaultRatio)
	normalizeTongueBoneRelations(modelData.Bones)
	applyTongueTailOffsetToTip(modelData.Bones, frontPos, backPos, defaultRatio.Bone4Ratio)
	applyTongueVertexWeightsByRatio(tongueVertices, tongue1, tongue2, tongue3, tongue4, frontPos, backPos, defaultRatio)
}

// resolveTongueMaterialIndex は舌再割当対象となる FaceMouth 材質indexを返す。
func resolveTongueMaterialIndex(materials *collection.NamedCollection[*model.Material]) (int, bool) {
	if materials == nil {
		return -1, false
	}
	for materialIndex, materialData := range materials.Values() {
		if materialData == nil {
			continue
		}
		nameLower := strings.ToLower(strings.TrimSpace(materialData.Name()))
		englishLower := strings.ToLower(strings.TrimSpace(materialData.EnglishName))
		if strings.Contains(nameLower, tongueMaterialHint) || strings.Contains(englishLower, tongueMaterialHint) {
			return materialIndex, true
		}
	}
	return -1, false
}

// collectTongueVertexIndexesFromMaterial は FaceMouth 材質の舌頂点index一覧を返す。
func collectTongueVertexIndexesFromMaterial(modelData *ModelData, materialIndex int) []int {
	if modelData == nil || modelData.Vertices == nil {
		return []int{}
	}
	candidates := collectMaterialVertexIndexes(modelData, materialIndex)
	if len(candidates) == 0 {
		return []int{}
	}
	indexes := make([]int, 0, len(candidates))
	for _, vertexIndex := range candidates {
		vertex, err := modelData.Vertices.Get(vertexIndex)
		if err != nil || vertex == nil {
			continue
		}
		if vertex.Uv.X >= tongueUvXThreshold && vertex.Uv.Y <= tongueUvYThreshold {
			indexes = append(indexes, vertexIndex)
		}
	}
	return indexes
}

// collectMaterialVertexIndexes は材質が参照する頂点index一覧を重複なく収集する。
func collectMaterialVertexIndexes(modelData *ModelData, materialIndex int) []int {
	if modelData == nil || modelData.Faces == nil || materialIndex < 0 {
		return []int{}
	}
	faceRanges, err := buildMaterialFaceRanges(modelData)
	if err != nil || materialIndex >= len(faceRanges) {
		return []int{}
	}
	faceRange := faceRanges[materialIndex]
	if faceRange.count <= 0 {
		return []int{}
	}
	uniqueIndexes := map[int]struct{}{}
	for faceIndex := faceRange.start; faceIndex < faceRange.start+faceRange.count; faceIndex++ {
		face, faceErr := modelData.Faces.Get(faceIndex)
		if faceErr != nil || face == nil {
			continue
		}
		for _, vertexIndex := range face.VertexIndexes {
			uniqueIndexes[vertexIndex] = struct{}{}
		}
	}
	vertexIndexes := make([]int, 0, len(uniqueIndexes))
	for vertexIndex := range uniqueIndexes {
		vertexIndexes = append(vertexIndexes, vertexIndex)
	}
	sort.Ints(vertexIndexes)
	return vertexIndexes
}

// resolveTongueFrontAndBackPositions は舌頂点群から前端/後端ボーン位置を算出する。
func resolveTongueFrontAndBackPositions(positions []mmath.Vec3) (mmath.Vec3, mmath.Vec3) {
	if len(positions) == 0 {
		return mmath.ZERO_VEC3, mmath.ZERO_VEC3
	}
	yMax := positions[0].Y
	yMin := positions[0].Y
	zMax := positions[0].Z
	zMin := positions[0].Z
	for _, pos := range positions[1:] {
		if pos.Y > yMax {
			yMax = pos.Y
		}
		if pos.Y < yMin {
			yMin = pos.Y
		}
		if pos.Z > zMax {
			zMax = pos.Z
		}
		if pos.Z < zMin {
			zMin = pos.Z
		}
	}
	front := mmath.Vec3{Vec: r3.Vec{X: 0, Y: yMax, Z: zMax}}
	back := mmath.Vec3{Vec: r3.Vec{X: 0, Y: yMin, Z: zMin}}
	return front, back
}

// applyTongueBoneRatios は舌長に対する比率で舌ボーン位置を再配置する。
func applyTongueBoneRatios(
	tongue1 *model.Bone,
	tongue2 *model.Bone,
	tongue3 *model.Bone,
	tongue4 *model.Bone,
	frontPos mmath.Vec3,
	backPos mmath.Vec3,
	ratio tongueRatioConfig,
) {
	if tongue1 == nil || tongue2 == nil || tongue3 == nil || tongue4 == nil {
		return
	}
	segment := backPos.Subed(frontPos)
	tongue1.Position = frontPos
	tongue2.Position = frontPos.Added(segment.MuledScalar(clampRatio01(ratio.Bone2Ratio)))
	tongue3.Position = frontPos.Added(segment.MuledScalar(clampRatio01(ratio.Bone3Ratio)))
	tongue4.Position = frontPos.Added(segment.MuledScalar(clampRatio01(ratio.Bone4Ratio)))
}

// applyTongueTailOffsetToTip は舌4の表示先を先端方向オフセットへ設定する。
func applyTongueTailOffsetToTip(
	bones *model.BoneCollection,
	frontPos mmath.Vec3,
	backPos mmath.Vec3,
	tongue4Ratio float64,
) {
	if bones == nil {
		return
	}
	if _, exists := getBoneByName(bones, tongueBone4Name); !exists {
		return
	}
	segment := backPos.Subed(frontPos)
	tongue4Position := frontPos.Added(segment.MuledScalar(clampRatio01(tongue4Ratio)))
	tongueTipPosition := frontPos.Added(segment)
	setBoneTailOffsetByName(bones, tongueBone4Name, tongueTipPosition.Subed(tongue4Position))
}

// applyTongueVertexWeightsByRatio は舌頂点を比率区間に応じて舌ボーンへ再割当する。
func applyTongueVertexWeightsByRatio(
	vertices []*model.Vertex,
	tongue1 *model.Bone,
	tongue2 *model.Bone,
	tongue3 *model.Bone,
	tongue4 *model.Bone,
	frontPos mmath.Vec3,
	backPos mmath.Vec3,
	ratio tongueRatioConfig,
) bool {
	if len(vertices) == 0 || tongue1 == nil || tongue2 == nil || tongue3 == nil || tongue4 == nil {
		return false
	}
	bone2Ratio := clampRatio01(ratio.Bone2Ratio)
	bone3Ratio := clampRatio01(ratio.Bone3Ratio)
	bone4Ratio := clampRatio01(ratio.Bone4Ratio)
	if bone2Ratio <= 0 {
		bone2Ratio = tongueBone2RatioDefault
	}
	if bone3Ratio <= bone2Ratio {
		bone3Ratio = bone2Ratio + 0.1
	}
	if bone4Ratio <= bone3Ratio {
		bone4Ratio = bone3Ratio + 0.1
	}
	if bone3Ratio > 1 {
		bone3Ratio = 1
	}
	if bone4Ratio > 1 {
		bone4Ratio = 1
	}

	tongue4Weighted := false
	for _, vertex := range vertices {
		if vertex == nil {
			continue
		}
		r := resolvePositionRatioOnSegment(vertex.Position, frontPos, backPos)
		switch {
		case r <= bone2Ratio:
			vertex.Deform = buildTongueSegmentBdef2(tongue2.Index(), tongue1.Index(), r, 0, bone2Ratio)
			vertex.DeformType = model.BDEF2
		case r <= bone3Ratio:
			vertex.Deform = buildTongueSegmentBdef2(tongue3.Index(), tongue2.Index(), r, bone2Ratio, bone3Ratio)
			vertex.DeformType = model.BDEF2
		case r <= bone4Ratio:
			vertex.Deform = buildTongueSegmentBdef2(tongue4.Index(), tongue3.Index(), r, bone3Ratio, bone4Ratio)
			vertex.DeformType = model.BDEF2
			tongue4Weighted = true
		default:
			vertex.Deform = model.NewBdef1(tongue4.Index())
			vertex.DeformType = model.BDEF1
			tongue4Weighted = true
		}
	}
	return tongue4Weighted
}

// buildTongueSegmentBdef2 は区間比率に応じたBdef2を生成する。
func buildTongueSegmentBdef2(toIndex int, fromIndex int, ratio float64, rangeStart float64, rangeEnd float64) model.IDeform {
	rangeLength := rangeEnd - rangeStart
	if rangeLength <= weightSignEpsilon {
		return model.NewBdef2(toIndex, fromIndex, 0)
	}
	weightTo := (ratio - rangeStart) / rangeLength
	if weightTo < 0 {
		weightTo = 0
	}
	if weightTo > 1 {
		weightTo = 1
	}
	return model.NewBdef2(toIndex, fromIndex, weightTo)
}

// resolvePositionRatioOnSegment は始点終点線分上への射影比率を返す。
func resolvePositionRatioOnSegment(position mmath.Vec3, start mmath.Vec3, end mmath.Vec3) float64 {
	segment := end.Subed(start)
	segmentLengthSq := segment.LengthSqr()
	if segmentLengthSq <= weightSignEpsilon {
		return 0
	}
	projected := position.Subed(start).Dot(segment) / segmentLengthSq
	return clampRatio01(projected)
}

// clampRatio01 は比率値を0〜1へ丸める。
func clampRatio01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

// applyTongueSegmentWeight は指定区間内の頂点をBdef2へ再割当する。
func applyTongueSegmentWeight(vertex *model.Vertex, from *model.Bone, to *model.Bone) bool {
	if vertex == nil || from == nil || to == nil {
		return false
	}
	tongueDistance := to.Position.Z - from.Position.Z
	if absSignValue(tongueDistance) <= weightSignEpsilon {
		return false
	}
	vertexDistance := vertex.Position.Z - from.Position.Z
	if !hasSameSign(tongueDistance, vertexDistance) {
		return false
	}
	weightTo := vertexDistance / tongueDistance
	if weightTo < 0 || weightTo > 1 {
		return false
	}
	vertex.Deform = model.NewBdef2(to.Index(), from.Index(), weightTo)
	vertex.DeformType = model.BDEF2
	return true
}

// buildWeightReplaceRules はD系へのウェイト置換ルールを構築する。
func buildWeightReplaceRules(modelData *ModelData, targetBoneIndexes map[string]int) []weightReplaceRule {
	rules := make([]weightReplaceRule, 0, 8)
	for _, direction := range []model.BoneDirection{model.BONE_DIRECTION_LEFT, model.BONE_DIRECTION_RIGHT} {
		candidates := [][2]string{
			{model.LEG.StringFromDirection(direction), model.LEG_D.StringFromDirection(direction)},
			{model.KNEE.StringFromDirection(direction), model.KNEE_D.StringFromDirection(direction)},
			{model.ANKLE.StringFromDirection(direction), model.ANKLE_D.StringFromDirection(direction)},
			{toeHumanTargetNameByDirection(direction), model.TOE_EX.StringFromDirection(direction)},
		}
		for _, candidate := range candidates {
			fromBone, fromOK := getBoneByTargetName(modelData, targetBoneIndexes, candidate[0])
			toBone, toOK := getBoneByTargetName(modelData, targetBoneIndexes, candidate[1])
			if !fromOK || !toOK {
				continue
			}
			rules = append(rules, weightReplaceRule{
				FromIndex: fromBone.Index(),
				ToIndex:   toBone.Index(),
			})
		}
	}
	return rules
}

// buildTwistWeightChains は捩りウェイト分割系列を構築する。
func buildTwistWeightChains(modelData *ModelData, targetBoneIndexes map[string]int) []twistWeightChain {
	chains := make([]twistWeightChain, 0, 4)
	for _, direction := range []model.BoneDirection{model.BONE_DIRECTION_LEFT, model.BONE_DIRECTION_RIGHT} {
		if chain, ok := buildTwistWeightChainByNames(
			modelData,
			targetBoneIndexes,
			model.ARM.StringFromDirection(direction),
			model.ELBOW.StringFromDirection(direction),
			[]string{
				model.ARM_TWIST1.StringFromDirection(direction),
				model.ARM_TWIST2.StringFromDirection(direction),
				model.ARM_TWIST3.StringFromDirection(direction),
			},
		); ok {
			chains = append(chains, chain)
		}
		if chain, ok := buildTwistWeightChainByNames(
			modelData,
			targetBoneIndexes,
			model.ELBOW.StringFromDirection(direction),
			model.WRIST.StringFromDirection(direction),
			[]string{
				model.WRIST_TWIST1.StringFromDirection(direction),
				model.WRIST_TWIST2.StringFromDirection(direction),
				model.WRIST_TWIST3.StringFromDirection(direction),
			},
		); ok {
			chains = append(chains, chain)
		}
	}
	return chains
}

// buildTwistWeightChainByNames は指定名から捩りウェイト分割系列を構築する。
func buildTwistWeightChainByNames(
	modelData *ModelData,
	targetBoneIndexes map[string]int,
	baseFromName string,
	baseToName string,
	twistNames []string,
) (twistWeightChain, bool) {
	baseFrom, baseFromOK := getBoneByTargetName(modelData, targetBoneIndexes, baseFromName)
	baseTo, baseToOK := getBoneByTargetName(modelData, targetBoneIndexes, baseToName)
	if !baseFromOK || !baseToOK || len(twistNames) != 3 {
		return twistWeightChain{}, false
	}
	twist1, twist1OK := getBoneByTargetName(modelData, targetBoneIndexes, twistNames[0])
	twist2, twist2OK := getBoneByTargetName(modelData, targetBoneIndexes, twistNames[1])
	twist3, twist3OK := getBoneByTargetName(modelData, targetBoneIndexes, twistNames[2])
	if !twist1OK || !twist2OK || !twist3OK {
		return twistWeightChain{}, false
	}

	return twistWeightChain{
		BaseFromX:    baseFrom.Position.X,
		BaseDistance: baseTo.Position.X - baseFrom.Position.X,
		CandidateIndexes: []int{
			baseFrom.Index(),
			twist1.Index(),
			twist2.Index(),
			twist3.Index(),
		},
		Segments: []twistWeightSegment{
			{
				FromIndex: baseFrom.Index(),
				ToIndex:   twist1.Index(),
				FromX:     baseFrom.Position.X,
				ToX:       twist1.Position.X,
			},
			{
				FromIndex: twist1.Index(),
				ToIndex:   twist2.Index(),
				FromX:     twist1.Position.X,
				ToX:       twist2.Position.X,
			},
			{
				FromIndex: twist2.Index(),
				ToIndex:   twist3.Index(),
				FromX:     twist2.Position.X,
				ToX:       twist3.Position.X,
			},
		},
	}, true
}

// applyDirectWeightReplaceRules はD系置換ルールを頂点ジョイントへ適用する。
func applyDirectWeightReplaceRules(joints []int, rules []weightReplaceRule) {
	if len(joints) == 0 || len(rules) == 0 {
		return
	}
	for _, rule := range rules {
		for i := range joints {
			if joints[i] == rule.FromIndex {
				joints[i] = rule.ToIndex
			}
		}
	}
}

// applyTwistWeightChain は捩り分割系列を頂点ジョイントへ適用する。
func applyTwistWeightChain(vertexX float64, joints *[]int, weights *[]float64, chain twistWeightChain) {
	if joints == nil || weights == nil || len(*joints) == 0 || len(*weights) == 0 {
		return
	}
	if !containsAnyJoint(*joints, chain.CandidateIndexes) {
		return
	}
	if !hasSameSign(chain.BaseDistance, vertexX-chain.BaseFromX) {
		return
	}

	for _, segment := range chain.Segments {
		twistDistance := segment.ToX - segment.FromX
		if absSignValue(twistDistance) <= weightSignEpsilon {
			continue
		}
		vectorDistance := vertexX - segment.FromX
		if !hasSameSign(twistDistance, vectorDistance) {
			continue
		}
		factor := vectorDistance / twistDistance
		applyTwistSegmentFactor(joints, weights, segment, factor)
	}
}

// applyTwistSegmentFactor は1区間分の捩り分割係数を適用する。
func applyTwistSegmentFactor(joints *[]int, weights *[]float64, segment twistWeightSegment, factor float64) {
	if joints == nil || weights == nil || len(*joints) == 0 || len(*weights) == 0 {
		return
	}
	if factor > 1.0 {
		for i := range *joints {
			if (*joints)[i] == segment.FromIndex {
				(*joints)[i] = segment.ToIndex
			}
		}
		return
	}
	if factor < 0 {
		return
	}

	currentLen := len(*joints)
	for i := 0; i < currentLen; i++ {
		if (*joints)[i] != segment.FromIndex {
			continue
		}
		fromWeight := (*weights)[i]
		if fromWeight <= 0 {
			continue
		}
		toWeight := fromWeight * factor
		(*weights)[i] = fromWeight * (1.0 - factor)
		if toWeight <= 0 {
			continue
		}
		*joints = append(*joints, segment.ToIndex)
		*weights = append(*weights, toWeight)
	}
}

// buildNormalizedDeform は重複合算後に正規化したデフォームを生成する。
func buildNormalizedDeform(joints []int, weights []float64, fallbackIndex int) model.IDeform {
	weightByBone := map[int]float64{}
	maxCount := len(joints)
	if len(weights) < maxCount {
		maxCount = len(weights)
	}
	for i := 0; i < maxCount; i++ {
		if joints[i] < 0 || weights[i] <= 0 {
			continue
		}
		weightByBone[joints[i]] += weights[i]
	}

	weightedJoints := make([]weightedJoint, 0, len(weightByBone))
	totalWeight := 0.0
	for index, weight := range weightByBone {
		if weight <= 0 {
			continue
		}
		weightedJoints = append(weightedJoints, weightedJoint{
			Index:  index,
			Weight: weight,
		})
		totalWeight += weight
	}
	if len(weightedJoints) == 0 || totalWeight <= 0 {
		if fallbackIndex < 0 {
			fallbackIndex = 0
		}
		return model.NewBdef1(fallbackIndex)
	}

	sort.Slice(weightedJoints, func(i int, j int) bool {
		if weightedJoints[i].Weight == weightedJoints[j].Weight {
			return weightedJoints[i].Index < weightedJoints[j].Index
		}
		return weightedJoints[i].Weight > weightedJoints[j].Weight
	})
	if len(weightedJoints) > 4 {
		weightedJoints = weightedJoints[:4]
	}

	totalTopWeight := 0.0
	for _, weighted := range weightedJoints {
		totalTopWeight += weighted.Weight
	}
	if totalTopWeight <= 0 {
		if fallbackIndex < 0 {
			fallbackIndex = 0
		}
		return model.NewBdef1(fallbackIndex)
	}

	if len(weightedJoints) == 1 {
		return model.NewBdef1(weightedJoints[0].Index)
	}
	if len(weightedJoints) == 2 {
		weight0 := weightedJoints[0].Weight / (weightedJoints[0].Weight + weightedJoints[1].Weight)
		return model.NewBdef2(weightedJoints[0].Index, weightedJoints[1].Index, weight0)
	}

	if fallbackIndex < 0 {
		fallbackIndex = weightedJoints[0].Index
	}
	indexes := [4]int{fallbackIndex, fallbackIndex, fallbackIndex, fallbackIndex}
	values := [4]float64{0, 0, 0, 0}
	for i := 0; i < len(weightedJoints) && i < 4; i++ {
		indexes[i] = weightedJoints[i].Index
		values[i] = weightedJoints[i].Weight / totalTopWeight
	}
	return model.NewBdef4(indexes, values)
}

// resolveFallbackBoneIndex はデフォーム生成失敗時の既定ボーンindexを返す。
func resolveFallbackBoneIndex(joints []int) int {
	for _, joint := range joints {
		if joint >= 0 {
			return joint
		}
	}
	return 0
}

// containsAnyJoint は候補ボーンindexが1つでも含まれるか判定する。
func containsAnyJoint(joints []int, candidates []int) bool {
	if len(joints) == 0 || len(candidates) == 0 {
		return false
	}
	candidateSet := map[int]struct{}{}
	for _, candidate := range candidates {
		candidateSet[candidate] = struct{}{}
	}
	for _, joint := range joints {
		if _, exists := candidateSet[joint]; exists {
			return true
		}
	}
	return false
}

// hasSameSign は2値の符号が一致するか判定する。
func hasSameSign(a float64, b float64) bool {
	return floatSign(a) == floatSign(b)
}

// floatSign は浮動小数の符号を返す。
func floatSign(v float64) int {
	if v > weightSignEpsilon {
		return 1
	}
	if v < -weightSignEpsilon {
		return -1
	}
	return 0
}

// absSignValue は絶対値を返す。
func absSignValue(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// collectHumanoidNodeIndexes は VRM Humanoid 定義を node index 対応へ変換する。
func collectHumanoidNodeIndexes(vrmData *vrm.VrmData) map[string]int {
	out := map[string]int{}
	if vrmData == nil {
		return out
	}
	if vrmData.Vrm1 != nil && vrmData.Vrm1.Humanoid != nil {
		for humanoidName, humanBone := range vrmData.Vrm1.Humanoid.HumanBones {
			if humanBone.Node < 0 {
				continue
			}
			out[strings.ToLower(strings.TrimSpace(humanoidName))] = humanBone.Node
		}
	}
	if len(out) == 0 && vrmData.Vrm0 != nil && vrmData.Vrm0.Humanoid != nil {
		for _, humanBone := range vrmData.Vrm0.Humanoid.HumanBones {
			if humanBone.Node < 0 {
				continue
			}
			out[strings.ToLower(strings.TrimSpace(humanBone.Bone))] = humanBone.Node
		}
	}
	return out
}

// buildHumanoidRenamePlan は humanoid から命名変更計画を生成する。
func buildHumanoidRenamePlan(humanoid map[string]int) map[string]int {
	selected := map[string]selectedHumanoidNode{}
	for _, rule := range humanoidRenameRules {
		nodeIndex, exists := humanoid[rule.HumanoidName]
		if !exists || nodeIndex < 0 {
			continue
		}
		if current, exists := selected[rule.TargetName]; exists {
			if rule.Priority < current.Priority {
				continue
			}
			if rule.Priority == current.Priority && nodeIndex >= current.NodeIndex {
				continue
			}
		}
		selected[rule.TargetName] = selectedHumanoidNode{
			NodeIndex: nodeIndex,
			Priority:  rule.Priority,
		}
	}

	plan := map[string]int{}
	for targetName, selectedNode := range selected {
		plan[targetName] = selectedNode.NodeIndex
	}
	applyThumbHumanoidRenamePlan(plan, humanoid)
	return plan
}

// applyThumbHumanoidRenamePlan はHumanoid定義から親指3節を再構築して命名計画へ反映する。
func applyThumbHumanoidRenamePlan(plan map[string]int, humanoid map[string]int) {
	if plan == nil || humanoid == nil {
		return
	}
	for _, side := range []struct {
		prefix string
		names  [3]string
	}{
		{
			prefix: "left",
			names: [3]string{
				model.THUMB0.Left(),
				model.THUMB1.Left(),
				model.THUMB2.Left(),
			},
		},
		{
			prefix: "right",
			names: [3]string{
				model.THUMB0.Right(),
				model.THUMB1.Right(),
				model.THUMB2.Right(),
			},
		},
	} {
		chain := collectThumbChainNodes(humanoid, side.prefix)
		if len(chain) == 0 {
			continue
		}
		for _, name := range side.names {
			delete(plan, name)
		}
		limit := 3
		if len(chain) < limit {
			limit = len(chain)
		}
		for i := 0; i < limit; i++ {
			plan[side.names[i]] = chain[i]
		}
	}
}

// collectThumbChainNodes は親指の根元→先端順ノードindexを返す。
func collectThumbChainNodes(humanoid map[string]int, sidePrefix string) []int {
	if humanoid == nil {
		return []int{}
	}
	keys := []string{
		sidePrefix + "thumbmetacarpal",
		sidePrefix + "thumbproximal",
		sidePrefix + "thumbintermediate",
		sidePrefix + "thumbdistal",
	}
	nodes := make([]int, 0, len(keys))
	for _, key := range keys {
		nodeIndex, exists := humanoid[key]
		if !exists || nodeIndex < 0 {
			continue
		}
		nodes = append(nodes, nodeIndex)
	}
	return nodes
}

// resolveTargetBoneIndexes は現在モデル内の対象ボーンindexを取得する。
func resolveTargetBoneIndexes(modelData *ModelData, plan map[string]int) map[string]int {
	out := map[string]int{}
	if modelData == nil || modelData.Bones == nil {
		return out
	}
	for targetName, sourceIndex := range plan {
		if source, err := modelData.Bones.Get(sourceIndex); err == nil && source != nil {
			out[targetName] = source.Index()
		}
	}
	for targetName := range plan {
		if _, exists := out[targetName]; exists {
			continue
		}
		if bone, err := modelData.Bones.GetByName(targetName); err == nil && bone != nil {
			out[targetName] = bone.Index()
		}
	}
	return out
}

// ensureSupplementBones は不足補完ボーンを Insert 方式で追加する。
func ensureSupplementBones(modelData *ModelData, targetBoneIndexes map[string]int) error {
	if modelData == nil || modelData.Bones == nil {
		return nil
	}
	if err := ensureRootAndCenterBones(modelData, targetBoneIndexes); err != nil {
		return err
	}
	if err := ensureGrooveBone(modelData, targetBoneIndexes); err != nil {
		return err
	}
	if err := ensureTrunkSupplementBones(modelData, targetBoneIndexes); err != nil {
		return err
	}
	if err := ensureWaistBone(modelData, targetBoneIndexes); err != nil {
		return err
	}
	if err := ensureEyesBone(modelData, targetBoneIndexes); err != nil {
		return err
	}
	if err := ensureTongueBones(modelData, targetBoneIndexes); err != nil {
		return err
	}
	for _, direction := range []model.BoneDirection{model.BONE_DIRECTION_LEFT, model.BONE_DIRECTION_RIGHT} {
		if err := ensureWaistCancelBone(modelData, targetBoneIndexes, direction); err != nil {
			return err
		}
		if err := ensureShoulderSupplementBones(modelData, targetBoneIndexes, direction); err != nil {
			return err
		}
		if err := ensureArmTwistBones(modelData, targetBoneIndexes, direction); err != nil {
			return err
		}
		if err := ensureWristTwistBones(modelData, targetBoneIndexes, direction); err != nil {
			return err
		}
		if err := ensureWristTailBone(modelData, targetBoneIndexes, direction); err != nil {
			return err
		}
		if err := ensureFingerTipBones(modelData, targetBoneIndexes, direction); err != nil {
			return err
		}
		if err := ensureLegIkSupplementBones(modelData, targetBoneIndexes, direction); err != nil {
			return err
		}
		if err := ensureLegDSupplementBones(modelData, targetBoneIndexes, direction); err != nil {
			return err
		}
	}
	return nil
}

// ensureRootAndCenterBones は root/center を補完する。
func ensureRootAndCenterBones(modelData *ModelData, targetBoneIndexes map[string]int) error {
	if modelData == nil || modelData.Bones == nil {
		return nil
	}

	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, model.ROOT.String()); !exists {
		root := model.NewBoneByName(model.ROOT.String())
		root.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE
		root.ParentIndex = -1
		if err := insertSupplementBone(modelData, targetBoneIndexes, model.ROOT.String(), root); err != nil {
			return err
		}
	}

	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, model.CENTER.String()); !exists {
		center := model.NewBoneByName(model.CENTER.String())
		center.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE
		center.Position = mmath.Vec3{Vec: r3.Vec{
			X: 0.0,
			Y: 0.0,
			Z: 0.0,
		}}
		if lower, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.LOWER.String()); ok {
			center.Position.Y = lower.Position.Y * 0.5
		}
		if root, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.ROOT.String()); ok {
			center.ParentIndex = root.Index()
		} else {
			center.ParentIndex = -1
		}
		if err := insertSupplementBone(modelData, targetBoneIndexes, model.CENTER.String(), center); err != nil {
			return err
		}
	}
	return nil
}

// ensureGrooveBone はグルーブを補完する。
func ensureGrooveBone(modelData *ModelData, targetBoneIndexes map[string]int) error {
	if modelData == nil || modelData.Bones == nil {
		return nil
	}
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, model.GROOVE.String()); exists {
		return nil
	}

	center, centerOK := getBoneByTargetName(modelData, targetBoneIndexes, model.CENTER.String())
	if !centerOK {
		return nil
	}

	bone := model.NewBoneByName(model.GROOVE.String())
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE
	bone.Position = mmath.Vec3{Vec: r3.Vec{
		X: 0.0,
		Y: 0.0,
		Z: 0.0,
	}}
	if lower, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.LOWER.String()); ok {
		bone.Position.Y = lower.Position.Y * 0.7
	}
	bone.ParentIndex = center.Index()
	return insertSupplementBone(modelData, targetBoneIndexes, model.GROOVE.String(), bone)
}

// ensureTrunkSupplementBones は体幹補完ボーンを追加する。
func ensureTrunkSupplementBones(modelData *ModelData, targetBoneIndexes map[string]int) error {
	if modelData == nil || modelData.Bones == nil {
		return nil
	}

	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, model.TRUNK_ROOT.String()); !exists {
		upper, upperOK := getBoneByTargetName(modelData, targetBoneIndexes, model.UPPER.String())
		lower, lowerOK := getBoneByTargetName(modelData, targetBoneIndexes, model.LOWER.String())
		if upperOK && lowerOK {
			bone := model.NewBoneByName(model.TRUNK_ROOT.String())
			bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE
			bone.IsSystem = true
			bone.Position = meanPosition(upper.Position, lower.Position)
			if center, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.CENTER.String()); ok {
				bone.ParentIndex = center.Index()
			} else {
				bone.ParentIndex = lower.ParentIndex
			}
			if err := insertSupplementBone(modelData, targetBoneIndexes, model.TRUNK_ROOT.String(), bone); err != nil {
				return err
			}
		}
	}

	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, model.LEG_CENTER.String()); !exists {
		leftLeg, leftOK := getBoneByTargetName(modelData, targetBoneIndexes, model.LEG.Left())
		rightLeg, rightOK := getBoneByTargetName(modelData, targetBoneIndexes, model.LEG.Right())
		if leftOK && rightOK {
			bone := model.NewBoneByName(model.LEG_CENTER.String())
			bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE
			bone.IsSystem = true
			bone.Position = meanPosition(leftLeg.Position, rightLeg.Position)
			if lower, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.LOWER.String()); ok {
				bone.ParentIndex = lower.Index()
			} else if center, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.CENTER.String()); ok {
				bone.ParentIndex = center.Index()
			} else {
				bone.ParentIndex = -1
			}
			if err := insertSupplementBone(modelData, targetBoneIndexes, model.LEG_CENTER.String(), bone); err != nil {
				return err
			}
		}
	}

	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, model.NECK_ROOT.String()); !exists {
		leftArm, leftOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ARM.Left())
		rightArm, rightOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ARM.Right())
		if leftOK && rightOK {
			bone := model.NewBoneByName(model.NECK_ROOT.String())
			bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE
			bone.IsSystem = true
			bone.Position = meanPosition(leftArm.Position, rightArm.Position)
			if upper2, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.UPPER2.String()); ok {
				bone.ParentIndex = upper2.Index()
			} else if upper, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.UPPER.String()); ok {
				bone.ParentIndex = upper.Index()
			} else {
				bone.ParentIndex = -1
			}
			if err := insertSupplementBone(modelData, targetBoneIndexes, model.NECK_ROOT.String(), bone); err != nil {
				return err
			}
		}
	}

	return nil
}

// ensureWaistBone は腰を補完する。
func ensureWaistBone(modelData *ModelData, targetBoneIndexes map[string]int) error {
	if modelData == nil || modelData.Bones == nil {
		return nil
	}
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, model.WAIST.String()); exists {
		return nil
	}

	upper, upperOK := getBoneByTargetName(modelData, targetBoneIndexes, model.UPPER.String())
	lower, lowerOK := getBoneByTargetName(modelData, targetBoneIndexes, model.LOWER.String())
	if !upperOK || !lowerOK {
		return nil
	}

	bone := model.NewBoneByName(model.WAIST.String())
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE
	bone.Position = meanPosition(upper.Position, lower.Position)
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.TRUNK_ROOT.String(),
		model.GROOVE.String(),
		model.CENTER.String(),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = lower.ParentIndex
	}
	return insertSupplementBone(modelData, targetBoneIndexes, model.WAIST.String(), bone)
}

// ensureEyesBone は両目を補完する。
func ensureEyesBone(modelData *ModelData, targetBoneIndexes map[string]int) error {
	if modelData == nil || modelData.Bones == nil {
		return nil
	}
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, model.EYES.String()); exists {
		return nil
	}

	leftEye, leftOK := getBoneByTargetName(modelData, targetBoneIndexes, model.EYE.Left())
	rightEye, rightOK := getBoneByTargetName(modelData, targetBoneIndexes, model.EYE.Right())
	if !leftOK || !rightOK {
		return nil
	}

	bone := model.NewBoneByName(model.EYES.String())
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE
	bone.Position = meanPosition(leftEye.Position, rightEye.Position)
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.NECK_ROOT.String(),
		model.UPPER2.String(),
		model.UPPER.String(),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = -1
	}
	return insertSupplementBone(modelData, targetBoneIndexes, model.EYES.String(), bone)
}

// ensureTongueBones は舌ボーン系列(舌1〜舌4)を補完する。
func ensureTongueBones(modelData *ModelData, targetBoneIndexes map[string]int) error {
	if modelData == nil || modelData.Bones == nil {
		return nil
	}
	head, headOK := getBoneByTargetName(modelData, targetBoneIndexes, model.HEAD.String())
	if !headOK {
		return nil
	}

	chainNames := []string{tongueBone1Name, tongueBone2Name, tongueBone3Name, tongueBone4Name}
	for chainIndex, chainName := range chainNames {
		if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, chainName); exists {
			continue
		}
		bone := model.NewBoneByName(chainName)
		bone.Position = defaultTonguePosition(head.Position, chainIndex)
		if chainIndex == 0 {
			bone.ParentIndex = head.Index()
			bone.BoneFlag = model.BoneFlag(0x081B)
		} else {
			parentName := chainNames[chainIndex-1]
			parent, parentOK := getBoneByTargetName(modelData, targetBoneIndexes, parentName)
			if parentOK {
				bone.ParentIndex = parent.Index()
			} else {
				bone.ParentIndex = head.Index()
			}
			bone.BoneFlag = model.BoneFlag(0x081F)
		}
		if err := insertSupplementBone(modelData, targetBoneIndexes, chainName, bone); err != nil {
			return err
		}
	}
	normalizeTongueBoneRelations(modelData.Bones)
	return nil
}

// defaultTonguePosition は頭位置基準の舌ボーン初期位置を返す。
func defaultTonguePosition(headPosition mmath.Vec3, chainIndex int) mmath.Vec3 {
	steps := []mmath.Vec3{
		{Vec: r3.Vec{X: 0, Y: -0.05, Z: -0.22}},
		{Vec: r3.Vec{X: 0, Y: -0.05, Z: -0.32}},
		{Vec: r3.Vec{X: 0, Y: -0.05, Z: -0.42}},
		{Vec: r3.Vec{X: 0, Y: -0.05, Z: -0.52}},
	}
	if chainIndex < 0 || chainIndex >= len(steps) {
		return headPosition
	}
	return headPosition.Added(steps[chainIndex])
}

// normalizeTongueBoneRelations は舌チェーンの親子・表示先・ローカル軸を正規化する。
func normalizeTongueBoneRelations(bones *model.BoneCollection) {
	if bones == nil {
		return
	}
	setBoneParentByName(bones, tongueBone1Name, model.HEAD.String())
	setBoneTailBoneByName(bones, tongueBone1Name, tongueBone2Name)
	setBoneParentByName(bones, tongueBone2Name, tongueBone1Name)
	setBoneTailBoneByName(bones, tongueBone2Name, tongueBone3Name)
	setBoneParentByName(bones, tongueBone3Name, tongueBone2Name)
	setBoneTailBoneByName(bones, tongueBone3Name, tongueBone4Name)
	setBoneParentByName(bones, tongueBone4Name, tongueBone3Name)
	setBoneTailOffsetByName(bones, tongueBone4Name, mmath.ZERO_VEC3)

	tongue1, tongue1OK := getBoneByName(bones, tongueBone1Name)
	tongue2, tongue2OK := getBoneByName(bones, tongueBone2Name)
	tongue3, tongue3OK := getBoneByName(bones, tongueBone3Name)
	tongue4, tongue4OK := getBoneByName(bones, tongueBone4Name)
	if tongue1OK && tongue2OK {
		setTongueLocalAxis(tongue1, tongue2)
	}
	if tongue2OK && tongue3OK {
		setTongueLocalAxis(tongue2, tongue3)
	}
	if tongue3OK && tongue4OK {
		setTongueLocalAxis(tongue3, tongue4)
	}
	if tongue4OK && tongue3OK {
		tongue4.LocalAxisX = tongue3.LocalAxisX
		tongue4.LocalAxisZ = tongue3.LocalAxisZ
	}
	for _, tongueName := range []string{tongueBone1Name, tongueBone2Name, tongueBone3Name, tongueBone4Name} {
		tongueBone, exists := getBoneByName(bones, tongueName)
		if !exists {
			continue
		}
		switch tongueName {
		case tongueBone1Name:
			tongueBone.BoneFlag |= model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE
			tongueBone.BoneFlag &^= model.BONE_FLAG_CAN_TRANSLATE
		case tongueBone2Name, tongueBone3Name, tongueBone4Name:
			tongueBone.BoneFlag |= model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE | model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE
		}
		applyBoneFlagConsistency(tongueBone)
	}
}

// setTongueLocalAxis は舌ボーンのローカル軸をfrom->to方向で設定する。
func setTongueLocalAxis(from *model.Bone, to *model.Bone) {
	if from == nil || to == nil {
		return
	}
	axisX := to.Position.Subed(from.Position).Normalized()
	if axisX.Length() <= weightSignEpsilon {
		axisX = mmath.UNIT_Z_NEG_VEC3
	}
	axisZ := axisX.Cross(mmath.UNIT_Y_NEG_VEC3).Normalized()
	if axisZ.Length() <= weightSignEpsilon {
		axisZ = mmath.UNIT_X_VEC3
	}
	from.LocalAxisX = axisX
	from.LocalAxisZ = axisZ
}

// ensureShoulderSupplementBones は左右の肩補助ボーンを補完する。
func ensureShoulderSupplementBones(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	if err := ensureShoulderPBone(modelData, targetBoneIndexes, direction); err != nil {
		return err
	}
	if err := ensureShoulderCBone(modelData, targetBoneIndexes, direction); err != nil {
		return err
	}
	return nil
}

// ensureShoulderPBone は左右の肩Pを補完する。
func ensureShoulderPBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.SHOULDER_P.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	shoulder, shoulderOK := getBoneByTargetName(modelData, targetBoneIndexes, model.SHOULDER.StringFromDirection(direction))
	if !shoulderOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE
	bone.Position = shoulder.Position
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.SHOULDER_ROOT.StringFromDirection(direction),
		model.NECK_ROOT.String(),
		model.UPPER2.String(),
		model.UPPER.String(),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = shoulder.ParentIndex
	}
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// ensureShoulderCBone は左右の肩Cを補完する。
func ensureShoulderCBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.SHOULDER_C.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	arm, armOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ARM.StringFromDirection(direction))
	if !armOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.Position = arm.Position
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.SHOULDER.StringFromDirection(direction),
		model.SHOULDER_P.StringFromDirection(direction),
		model.SHOULDER_ROOT.StringFromDirection(direction),
		model.NECK_ROOT.String(),
		model.UPPER2.String(),
		model.UPPER.String(),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = arm.ParentIndex
	}
	if shoulderP, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.SHOULDER_P.StringFromDirection(direction)); ok {
		bone.EffectIndex = shoulderP.Index()
		bone.EffectFactor = -1.0
		bone.BoneFlag |= model.BONE_FLAG_IS_EXTERNAL_ROTATION
	}
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// ensureArmTwistBones は左右の腕捩系列を補完する。
func ensureArmTwistBones(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	if err := ensureArmTwistBone(modelData, targetBoneIndexes, direction); err != nil {
		return err
	}
	for idx := 1; idx <= 3; idx++ {
		if err := ensureArmTwistChildBone(modelData, targetBoneIndexes, direction, idx); err != nil {
			return err
		}
	}
	return nil
}

// ensureArmTwistBone は左右の腕捩を補完する。
func ensureArmTwistBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.ARM_TWIST.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	arm, armOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ARM.StringFromDirection(direction))
	elbow, elbowOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ELBOW.StringFromDirection(direction))
	if !armOK || !elbowOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE
	bone.Position = meanPosition(arm.Position, elbow.Position)
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.ARM.StringFromDirection(direction),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = arm.ParentIndex
	}
	axisX := elbow.Position.Subed(bone.Position)
	if axisX.Length() > 1e-8 {
		bone.FixedAxis = axisX.Normalized()
		bone.BoneFlag |= model.BONE_FLAG_HAS_FIXED_AXIS

		localAxisZ := mmath.UNIT_Y_NEG_VEC3.Cross(bone.FixedAxis)
		if localAxisZ.Length() <= 1e-8 {
			localAxisZ = mmath.UNIT_X_VEC3.Cross(bone.FixedAxis)
		}
		if localAxisZ.Length() > 1e-8 {
			bone.LocalAxisX = bone.FixedAxis
			bone.LocalAxisZ = localAxisZ.Normalized()
			bone.BoneFlag |= model.BONE_FLAG_HAS_LOCAL_AXIS
		}
	}
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// ensureArmTwistChildBone は左右の腕捩分割ボーンを補完する。
func ensureArmTwistChildBone(
	modelData *ModelData,
	targetBoneIndexes map[string]int,
	direction model.BoneDirection,
	idx int,
) error {
	if idx < 1 || idx > 3 {
		return nil
	}
	targetName := model.ARM_TWIST.StringFromDirectionAndIdx(direction, idx)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	arm, armOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ARM.StringFromDirection(direction))
	elbow, elbowOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ELBOW.StringFromDirection(direction))
	if !armOK || !elbowOK {
		return nil
	}

	ratio := armTwistRatioByIndex(idx)
	bone := model.NewBoneByName(targetName)
	bone.Position = mmath.Vec3{Vec: r3.Vec{
		X: arm.Position.X + ((elbow.Position.X - arm.Position.X) * ratio),
		Y: arm.Position.Y + ((elbow.Position.Y - arm.Position.Y) * ratio),
		Z: arm.Position.Z + ((elbow.Position.Z - arm.Position.Z) * ratio),
	}}
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.ARM.StringFromDirection(direction),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = arm.ParentIndex
	}
	if armTwist, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.ARM_TWIST.StringFromDirection(direction)); ok {
		bone.EffectIndex = armTwist.Index()
		bone.EffectFactor = ratio
		bone.BoneFlag |= model.BONE_FLAG_IS_EXTERNAL_ROTATION
	}
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// ensureWristTwistBones は左右の手捩系列を補完する。
func ensureWristTwistBones(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	if err := ensureWristTwistBone(modelData, targetBoneIndexes, direction); err != nil {
		return err
	}
	for idx := 1; idx <= 3; idx++ {
		if err := ensureWristTwistChildBone(modelData, targetBoneIndexes, direction, idx); err != nil {
			return err
		}
	}
	return nil
}

// ensureWristTwistBone は左右の手捩を補完する。
func ensureWristTwistBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.WRIST_TWIST.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	elbow, elbowOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ELBOW.StringFromDirection(direction))
	wrist, wristOK := getBoneByTargetName(modelData, targetBoneIndexes, model.WRIST.StringFromDirection(direction))
	if !elbowOK || !wristOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE
	bone.Position = meanPosition(elbow.Position, wrist.Position)
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.ELBOW.StringFromDirection(direction),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = elbow.ParentIndex
	}
	axisX := wrist.Position.Subed(bone.Position)
	if axisX.Length() > 1e-8 {
		bone.FixedAxis = axisX.Normalized()
		bone.BoneFlag |= model.BONE_FLAG_HAS_FIXED_AXIS

		localAxisZ := mmath.UNIT_Y_NEG_VEC3.Cross(bone.FixedAxis)
		if localAxisZ.Length() <= 1e-8 {
			localAxisZ = mmath.UNIT_X_VEC3.Cross(bone.FixedAxis)
		}
		if localAxisZ.Length() > 1e-8 {
			bone.LocalAxisX = bone.FixedAxis
			bone.LocalAxisZ = localAxisZ.Normalized()
			bone.BoneFlag |= model.BONE_FLAG_HAS_LOCAL_AXIS
		}
	}
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// ensureWristTwistChildBone は左右の手捩分割ボーンを補完する。
func ensureWristTwistChildBone(
	modelData *ModelData,
	targetBoneIndexes map[string]int,
	direction model.BoneDirection,
	idx int,
) error {
	if idx < 1 || idx > 3 {
		return nil
	}
	targetName := model.WRIST_TWIST.StringFromDirectionAndIdx(direction, idx)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	elbow, elbowOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ELBOW.StringFromDirection(direction))
	wrist, wristOK := getBoneByTargetName(modelData, targetBoneIndexes, model.WRIST.StringFromDirection(direction))
	if !elbowOK || !wristOK {
		return nil
	}

	ratio := armTwistRatioByIndex(idx)
	bone := model.NewBoneByName(targetName)
	bone.Position = mmath.Vec3{Vec: r3.Vec{
		X: elbow.Position.X + ((wrist.Position.X - elbow.Position.X) * ratio),
		Y: elbow.Position.Y + ((wrist.Position.Y - elbow.Position.Y) * ratio),
		Z: elbow.Position.Z + ((wrist.Position.Z - elbow.Position.Z) * ratio),
	}}
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.ELBOW.StringFromDirection(direction),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = elbow.ParentIndex
	}
	if wristTwist, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.WRIST_TWIST.StringFromDirection(direction)); ok {
		bone.EffectIndex = wristTwist.Index()
		bone.EffectFactor = ratio
		bone.BoneFlag |= model.BONE_FLAG_IS_EXTERNAL_ROTATION
	}
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// ensureToeTailBone は左右のつま先先を補完する。
func ensureToeTailBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.TOE_T.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	ankle, ankleOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ANKLE.StringFromDirection(direction))
	if !ankleOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE
	bone.ParentIndex = ankle.Index()

	if toe, toeOK := getToeBaseBone(modelData, targetBoneIndexes, direction); toeOK {
		bone.Position = toe.Position
	} else {
		bone.Position = ankle.Position
	}
	bone.Position.Y = 0.0
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// ensureHeelBone は左右のかかとを補完する。
func ensureHeelBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.HEEL.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	ankle, ankleOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ANKLE.StringFromDirection(direction))
	if !ankleOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE
	bone.ParentIndex = ankle.Index()

	heelPos := ankle.Position
	if toeT, toeTOK := getBoneByTargetName(modelData, targetBoneIndexes, model.TOE_T.StringFromDirection(direction)); toeTOK {
		diff := mmath.Vec3{Vec: r3.Vec{
			X: ankle.Position.X - toeT.Position.X,
			Y: ankle.Position.Y - toeT.Position.Y,
			Z: ankle.Position.Z - toeT.Position.Z,
		}}
		heelPos = mmath.Vec3{Vec: r3.Vec{
			X: ankle.Position.X + (diff.X * 0.35),
			Y: ankle.Position.Y + (diff.Y * 0.35),
			Z: ankle.Position.Z + (diff.Z * 0.35),
		}}
	} else {
		heelPos.Z += 0.2
	}
	heelPos.Y = 0.0
	bone.Position = heelPos
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// ensureLegDSupplementBones は左右の下半身D系列を補完する。
func ensureLegDSupplementBones(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	if err := ensureLegDBone(modelData, targetBoneIndexes, direction); err != nil {
		return err
	}
	if err := ensureKneeDBone(modelData, targetBoneIndexes, direction); err != nil {
		return err
	}
	if err := ensureAnkleDBone(modelData, targetBoneIndexes, direction); err != nil {
		return err
	}
	if err := ensureToeExBone(modelData, targetBoneIndexes, direction); err != nil {
		return err
	}
	return nil
}

// ensureLegDBone は左右の足Dを補完する。
func ensureLegDBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.LEG_D.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	leg, legOK := getBoneByTargetName(modelData, targetBoneIndexes, model.LEG.StringFromDirection(direction))
	if !legOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE
	bone.Position = leg.Position
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.WAIST_CANCEL.StringFromDirection(direction),
		model.LEG_ROOT.StringFromDirection(direction),
		model.LEG_CENTER.String(),
		model.LOWER.String(),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = leg.ParentIndex
	}
	bone.EffectIndex = leg.Index()
	bone.EffectFactor = 1.0
	bone.BoneFlag |= model.BONE_FLAG_IS_EXTERNAL_ROTATION
	if err := insertSupplementBone(modelData, targetBoneIndexes, targetName, bone); err != nil {
		return err
	}
	applyLayerPlusOneFromEffectParent(modelData, targetBoneIndexes, targetName)
	return nil
}

// ensureKneeDBone は左右のひざDを補完する。
func ensureKneeDBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.KNEE_D.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	knee, kneeOK := getBoneByTargetName(modelData, targetBoneIndexes, model.KNEE.StringFromDirection(direction))
	if !kneeOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE
	bone.Position = knee.Position
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.LEG_D.StringFromDirection(direction),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = knee.ParentIndex
	}
	bone.EffectIndex = knee.Index()
	bone.EffectFactor = 1.0
	bone.BoneFlag |= model.BONE_FLAG_IS_EXTERNAL_ROTATION
	if err := insertSupplementBone(modelData, targetBoneIndexes, targetName, bone); err != nil {
		return err
	}
	applyLayerPlusOneFromEffectParent(modelData, targetBoneIndexes, targetName)
	return nil
}

// ensureAnkleDBone は左右の足首Dを補完する。
func ensureAnkleDBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.ANKLE_D.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	ankle, ankleOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ANKLE.StringFromDirection(direction))
	if !ankleOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE
	bone.Position = ankle.Position
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.KNEE_D.StringFromDirection(direction),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = ankle.ParentIndex
	}
	bone.EffectIndex = ankle.Index()
	bone.EffectFactor = 1.0
	bone.BoneFlag |= model.BONE_FLAG_IS_EXTERNAL_ROTATION
	if err := insertSupplementBone(modelData, targetBoneIndexes, targetName, bone); err != nil {
		return err
	}
	applyLayerPlusOneFromEffectParent(modelData, targetBoneIndexes, targetName)
	return nil
}

// ensureToeExBone は左右の足先EXを補完する。
func ensureToeExBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.TOE_EX.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	ankleD, ankleDOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ANKLE_D.StringFromDirection(direction))
	if !ankleDOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE
	bone.Position = ankleD.Position
	if toeT, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.TOE_T.StringFromDirection(direction)); ok {
		bone.Position = meanPosition(ankleD.Position, toeT.Position)
	} else if toeBase, toeBaseOK := getToeBaseBone(modelData, targetBoneIndexes, direction); toeBaseOK {
		bone.Position = meanPosition(ankleD.Position, toeBase.Position)
	}
	bone.ParentIndex = ankleD.Index()
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// ensureLegIkSupplementBones は左右のIK補助系列を補完する。
func ensureLegIkSupplementBones(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	if err := ensureLegIkParentBone(modelData, targetBoneIndexes, direction); err != nil {
		return err
	}
	if err := ensureLegIkBone(modelData, targetBoneIndexes, direction); err != nil {
		return err
	}
	if err := ensureToeIkBone(modelData, targetBoneIndexes, direction); err != nil {
		return err
	}
	return nil
}

// ensureLegIkBone は左右の足IKを補完する。
func ensureLegIkBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.LEG_IK.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	ankle, ankleOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ANKLE.StringFromDirection(direction))
	if !ankleOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE |
		model.BONE_FLAG_CAN_TRANSLATE | model.BONE_FLAG_IS_IK
	bone.Position = ankle.Position
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.LEG_IK_PARENT.StringFromDirection(direction),
		model.ROOT.String(),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = -1
	}
	ikLinks := make([]model.IkLink, 0, 2)
	if knee, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.KNEE.StringFromDirection(direction)); ok {
		ikLinks = append(ikLinks, model.IkLink{
			BoneIndex:     knee.Index(),
			AngleLimit:    true,
			MinAngleLimit: mmath.Vec3{Vec: r3.Vec{X: mmath.DegToRad(-180.0), Y: 0.0, Z: 0.0}},
			MaxAngleLimit: mmath.Vec3{Vec: r3.Vec{X: mmath.DegToRad(-0.5), Y: 0.0, Z: 0.0}},
		})
	}
	if leg, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.LEG.StringFromDirection(direction)); ok {
		ikLinks = append(ikLinks, model.IkLink{
			BoneIndex: leg.Index(),
		})
	}
	if len(ikLinks) > 0 {
		unit := 1.0
		bone.Ik = &model.Ik{
			BoneIndex:    ankle.Index(),
			LoopCount:    40,
			UnitRotation: mmath.Vec3{Vec: r3.Vec{X: unit, Y: unit, Z: unit}},
			Links:        ikLinks,
		}
	}
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// ensureFingerTipBones は左右の指先ボーンを補完する。
func ensureFingerTipBones(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	specs := []struct {
		Chain   []model.StandardBoneName
		TipTail model.StandardBoneName
	}{
		{Chain: []model.StandardBoneName{model.THUMB0, model.THUMB1, model.THUMB2}, TipTail: model.THUMB_TAIL},
		{Chain: []model.StandardBoneName{model.INDEX1, model.INDEX2, model.INDEX3}, TipTail: model.INDEX_TAIL},
		{Chain: []model.StandardBoneName{model.MIDDLE1, model.MIDDLE2, model.MIDDLE3}, TipTail: model.MIDDLE_TAIL},
		{Chain: []model.StandardBoneName{model.RING1, model.RING2, model.RING3}, TipTail: model.RING_TAIL},
		{Chain: []model.StandardBoneName{model.PINKY1, model.PINKY2, model.PINKY3}, TipTail: model.PINKY_TAIL},
	}
	for _, spec := range specs {
		if err := ensureFingerTipBone(modelData, targetBoneIndexes, direction, spec.Chain, spec.TipTail); err != nil {
			return err
		}
	}
	return nil
}

// ensureFingerTipBone は指定指系列の指先ボーンを補完する。
func ensureFingerTipBone(
	modelData *ModelData,
	targetBoneIndexes map[string]int,
	direction model.BoneDirection,
	chain []model.StandardBoneName,
	tipTail model.StandardBoneName,
) error {
	targetName := fingerTipAliasNameFromTail(tipTail, direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}
	chainBones := make([]*model.Bone, 0, len(chain))
	for _, standard := range chain {
		chainBone, chainOK := getBoneByTargetName(modelData, targetBoneIndexes, standard.StringFromDirection(direction))
		if !chainOK {
			continue
		}
		chainBones = append(chainBones, chainBone)
	}
	if len(chainBones) == 0 {
		return nil
	}
	distalBone := chainBones[len(chainBones)-1]
	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE
	bone.ParentIndex = distalBone.Index()
	bone.Position = distalBone.Position
	if len(chainBones) >= 2 {
		prevBone := chainBones[len(chainBones)-2]
		bone.Position = mmath.Vec3{Vec: r3.Vec{
			X: distalBone.Position.X + (distalBone.Position.X-prevBone.Position.X)*0.5,
			Y: distalBone.Position.Y + (distalBone.Position.Y-prevBone.Position.Y)*0.5,
			Z: distalBone.Position.Z + (distalBone.Position.Z-prevBone.Position.Z)*0.5,
		}}
	}
	bone.TailIndex = -1
	bone.TailPosition = mmath.Vec3{Vec: r3.Vec{}}
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// ensureLegIkParentBone は左右の足IK親を補完する。
func ensureLegIkParentBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.LEG_IK_PARENT.StringFromDirection(direction)
	legIK, legIKOK := getBoneByTargetName(modelData, targetBoneIndexes, model.LEG_IK.StringFromDirection(direction))
	ankle, ankleOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ANKLE.StringFromDirection(direction))

	if parentBone, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		if legIKOK {
			legIK.ParentIndex = parentBone.Index()
		}
		return nil
	}
	if !legIKOK && !ankleOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE
	switch {
	case legIKOK:
		bone.Position = mmath.Vec3{Vec: r3.Vec{
			X: legIK.Position.X,
			Y: 0.0,
			Z: legIK.Position.Z,
		}}
	case ankleOK:
		bone.Position = mmath.Vec3{Vec: r3.Vec{
			X: ankle.Position.X,
			Y: 0.0,
			Z: ankle.Position.Z,
		}}
	}
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.ROOT.String(),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = -1
	}
	if err := insertSupplementBone(modelData, targetBoneIndexes, targetName, bone); err != nil {
		return err
	}
	if legIKParent, ok := getBoneByTargetName(modelData, targetBoneIndexes, targetName); ok && legIKOK {
		legIK.ParentIndex = legIKParent.Index()
	}
	return nil
}

// ensureToeIkBone は左右のつま先IKを補完する。
func ensureToeIkBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.TOE_IK.StringFromDirection(direction)
	legIK, legIKOK := getBoneByTargetName(modelData, targetBoneIndexes, model.LEG_IK.StringFromDirection(direction))
	if !legIKOK {
		return nil
	}
	toeTarget, toeTargetOK := getBoneByTargetName(modelData, targetBoneIndexes, toeHumanTargetNameByDirection(direction))
	if !toeTargetOK {
		if toeEx, toeExOK := getBoneByTargetName(modelData, targetBoneIndexes, model.TOE_EX.StringFromDirection(direction)); toeExOK {
			toeTarget = toeEx
			toeTargetOK = true
		}
	}
	if !toeTargetOK {
		if toeT, toeTOK := getBoneByTargetName(modelData, targetBoneIndexes, model.TOE_T.StringFromDirection(direction)); toeTOK {
			toeTarget = toeT
			toeTargetOK = true
		}
	}
	if !toeTargetOK {
		if ankle, ankleOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ANKLE.StringFromDirection(direction)); ankleOK {
			toeTarget = ankle
			toeTargetOK = true
		}
	}
	if !toeTargetOK {
		return nil
	}

	toeIK, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName)
	if !exists {
		bone := model.NewBoneByName(targetName)
		bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE |
			model.BONE_FLAG_CAN_TRANSLATE | model.BONE_FLAG_IS_IK
		bone.Position = toeTarget.Position
		bone.ParentIndex = legIK.Index()
		bone.TailIndex = -1
		bone.TailPosition = mmath.Vec3{Vec: r3.Vec{}}
		unit := mmath.DegToRad(229.1831)
		bone.Ik = &model.Ik{
			BoneIndex:    toeTarget.Index(),
			LoopCount:    3,
			UnitRotation: mmath.Vec3{Vec: r3.Vec{X: unit, Y: unit, Z: unit}},
			Links:        []model.IkLink{},
		}
		if err := insertSupplementBone(modelData, targetBoneIndexes, targetName, bone); err != nil {
			return err
		}
		toeIK = bone
	} else {
		toeIK.ParentIndex = legIK.Index()
		toeIK.Position = toeTarget.Position
		toeIK.TailIndex = -1
	}

	ikLinks := make([]model.IkLink, 0, 1)
	if ankle, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.ANKLE.StringFromDirection(direction)); ok {
		ikLinks = append(ikLinks, model.IkLink{
			BoneIndex: ankle.Index(),
		})
	}
	if toeIK.Ik == nil {
		unit := 1.0
		toeIK.Ik = &model.Ik{
			BoneIndex:    toeTarget.Index(),
			LoopCount:    40,
			UnitRotation: mmath.Vec3{Vec: r3.Vec{X: unit, Y: unit, Z: unit}},
			Links:        ikLinks,
		}
	} else {
		toeIK.Ik.BoneIndex = toeTarget.Index()
		if len(ikLinks) > 0 {
			toeIK.Ik.Links = ikLinks
		}
	}
	normalizeToeIkSolver(toeIK)
	legIK.BoneFlag |= model.BONE_FLAG_TAIL_IS_BONE
	legIK.TailIndex = toeIK.Index()
	return nil
}

// ensureWristTailBone は左右の手首先先を補完する。
func ensureWristTailBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.WRIST_TAIL.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	wrist, wristOK := getBoneByTargetName(modelData, targetBoneIndexes, model.WRIST.StringFromDirection(direction))
	if !wristOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE
	bone.ParentIndex = wrist.Index()

	fingerPositions := make([]mmath.Vec3, 0, 5)
	for _, fingerName := range []string{
		model.THUMB1.StringFromDirection(direction),
		model.INDEX1.StringFromDirection(direction),
		model.MIDDLE1.StringFromDirection(direction),
		model.RING1.StringFromDirection(direction),
		model.PINKY1.StringFromDirection(direction),
	} {
		if finger, ok := getBoneByTargetName(modelData, targetBoneIndexes, fingerName); ok {
			fingerPositions = append(fingerPositions, finger.Position)
		}
	}
	if len(fingerPositions) > 0 {
		bone.Position = mmath.MeanVec3(fingerPositions)
	} else if elbow, elbowOK := getBoneByTargetName(modelData, targetBoneIndexes, model.ELBOW.StringFromDirection(direction)); elbowOK {
		bone.Position = mmath.Vec3{Vec: r3.Vec{
			X: wrist.Position.X + (wrist.Position.X-elbow.Position.X)*0.5,
			Y: wrist.Position.Y + (wrist.Position.Y-elbow.Position.Y)*0.5,
			Z: wrist.Position.Z + (wrist.Position.Z-elbow.Position.Z)*0.5,
		}}
	} else {
		bone.Position = wrist.Position
	}
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// ensureWaistCancelBone は左右の腰キャンセルを補完する。
func ensureWaistCancelBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) error {
	targetName := model.WAIST_CANCEL.StringFromDirection(direction)
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	leg, legOK := getBoneByTargetName(modelData, targetBoneIndexes, model.LEG.StringFromDirection(direction))
	waist, waistOK := getBoneByTargetName(modelData, targetBoneIndexes, model.WAIST.String())
	if !legOK || !waistOK {
		return nil
	}

	bone := model.NewBoneByName(targetName)
	bone.Position = leg.Position
	if parentIndex, ok := resolveParentIndexByTargetNames(modelData, targetBoneIndexes, []string{
		model.LEG_CENTER.String(),
		model.LOWER.String(),
		model.CENTER.String(),
	}); ok {
		bone.ParentIndex = parentIndex
	} else {
		bone.ParentIndex = leg.ParentIndex
	}
	bone.EffectIndex = waist.Index()
	bone.EffectFactor = -1.0
	bone.BoneFlag |= model.BONE_FLAG_IS_EXTERNAL_ROTATION
	return insertSupplementBone(modelData, targetBoneIndexes, targetName, bone)
}

// getToeBaseBone はつま先基準ボーンを優先順で取得する。
func getToeBaseBone(modelData *ModelData, targetBoneIndexes map[string]int, direction model.BoneDirection) (*model.Bone, bool) {
	candidates := []string{
		toeHumanTargetNameByDirection(direction),
		model.TOE_EX.StringFromDirection(direction),
	}
	for _, candidate := range candidates {
		if bone, ok := getBoneByTargetName(modelData, targetBoneIndexes, candidate); ok {
			return bone, true
		}
	}
	return nil, false
}

// resolveParentIndexByTargetNames は候補名順で親ボーンindexを解決する。
func resolveParentIndexByTargetNames(
	modelData *ModelData,
	targetBoneIndexes map[string]int,
	candidateNames []string,
) (int, bool) {
	for _, candidateName := range candidateNames {
		if parent, ok := getBoneByTargetName(modelData, targetBoneIndexes, candidateName); ok {
			return parent.Index(), true
		}
	}
	return -1, false
}

// toeHumanTargetNameByDirection は human bone 由来のつま先名を返す。
func toeHumanTargetNameByDirection(direction model.BoneDirection) string {
	switch direction {
	case model.BONE_DIRECTION_RIGHT:
		return rightToeHumanTargetName
	default:
		return leftToeHumanTargetName
	}
}

// armTwistRatioByIndex は腕捩分割比率を返す。
func armTwistRatioByIndex(idx int) float64 {
	switch idx {
	case 1:
		return 0.25
	case 2:
		return 0.5
	case 3:
		return 0.75
	default:
		return 0.5
	}
}

// applyLayerPlusOneFromEffectParent は対象ボーンの階層を付与親階層+1に合わせる。
func applyLayerPlusOneFromEffectParent(modelData *ModelData, targetBoneIndexes map[string]int, targetName string) {
	target, ok := getBoneByTargetName(modelData, targetBoneIndexes, targetName)
	if !ok || target.EffectIndex < 0 {
		return
	}
	effectParent, err := modelData.Bones.Get(target.EffectIndex)
	if err != nil || effectParent == nil {
		return
	}
	target.Layer = effectParent.Layer + 1
}

// insertSupplementBone は不足補完ボーンを Insert 方式で追加する。
func insertSupplementBone(modelData *ModelData, targetBoneIndexes map[string]int, targetName string, bone *model.Bone) error {
	if modelData == nil || modelData.Bones == nil || bone == nil {
		return nil
	}
	if existing, err := modelData.Bones.GetByName(targetName); err == nil && existing != nil {
		targetBoneIndexes[targetName] = existing.Index()
		return nil
	}
	idx, _, err := modelData.Bones.Insert(bone, bone.ParentIndex)
	if err != nil {
		return err
	}
	targetBoneIndexes[targetName] = idx
	return nil
}

// renameHumanoidBones は計画に基づいてボーン名を変更する。
func renameHumanoidBones(
	bones *model.BoneCollection,
	targetBoneIndexes map[string]int,
	plan map[string]int,
) error {
	if bones == nil || len(plan) == 0 {
		return nil
	}

	entries := buildRenameEntries(bones, plan)
	if len(entries) == 0 {
		return nil
	}
	blockedTargets := detectBlockedRenameTargets(bones, entries)

	tempSerial := 0
	for _, entry := range entries {
		if _, blocked := blockedTargets[entry.TargetName]; blocked {
			continue
		}
		source, err := bones.Get(entry.SourceIndex)
		if err != nil || source == nil {
			continue
		}
		if source.Name() == entry.TargetName {
			continue
		}
		tempName := nextTemporaryBoneName(bones, &tempSerial)
		if _, err := bones.Rename(entry.SourceIndex, tempName); err != nil {
			if merrors.IsNameConflictError(err) {
				continue
			}
			return err
		}
	}

	for _, entry := range entries {
		if _, blocked := blockedTargets[entry.TargetName]; blocked {
			continue
		}
		source, err := bones.Get(entry.SourceIndex)
		if err != nil || source == nil {
			continue
		}
		if source.Name() == entry.TargetName {
			targetBoneIndexes[entry.TargetName] = entry.SourceIndex
			continue
		}
		if existing, err := bones.GetByName(entry.TargetName); err == nil && existing.Index() != entry.SourceIndex {
			continue
		}
		if _, err := bones.Rename(entry.SourceIndex, entry.TargetName); err != nil {
			if merrors.IsNameConflictError(err) {
				continue
			}
			return err
		}
		targetBoneIndexes[entry.TargetName] = entry.SourceIndex
	}
	return nil
}

// buildRenameEntries は実在するボーンのみを命名変更対象として抽出する。
func buildRenameEntries(bones *model.BoneCollection, plan map[string]int) []renamePlanEntry {
	entries := make([]renamePlanEntry, 0, len(plan))
	for targetName, sourceIndex := range plan {
		if source, err := bones.Get(sourceIndex); err == nil && source != nil {
			entries = append(entries, renamePlanEntry{
				SourceIndex: sourceIndex,
				TargetName:  targetName,
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].TargetName < entries[j].TargetName
	})
	return entries
}

// detectBlockedRenameTargets は外部の既存名と衝突する対象名を抽出する。
func detectBlockedRenameTargets(bones *model.BoneCollection, entries []renamePlanEntry) map[string]struct{} {
	blocked := map[string]struct{}{}
	sourceIndexes := map[int]struct{}{}
	for _, entry := range entries {
		sourceIndexes[entry.SourceIndex] = struct{}{}
	}
	for _, entry := range entries {
		existing, err := bones.GetByName(entry.TargetName)
		if err != nil || existing == nil {
			continue
		}
		if existing.Index() == entry.SourceIndex {
			continue
		}
		if _, isSource := sourceIndexes[existing.Index()]; isSource {
			continue
		}
		blocked[entry.TargetName] = struct{}{}
	}
	return blocked
}

// nextTemporaryBoneName は競合しない一時ボーン名を採番して返す。
func nextTemporaryBoneName(bones *model.BoneCollection, serial *int) string {
	if serial == nil {
		return boneRenameTempPrefix + "000"
	}
	for {
		candidate := fmt.Sprintf("%s%03d", boneRenameTempPrefix, *serial)
		*serial = *serial + 1
		if !bones.ContainsByName(candidate) {
			return candidate
		}
	}
}

// getBoneByTargetName は対象名に対応するボーンを取得する。
func getBoneByTargetName(
	modelData *ModelData,
	targetBoneIndexes map[string]int,
	targetName string,
) (*model.Bone, bool) {
	if modelData == nil || modelData.Bones == nil {
		return nil, false
	}
	if idx, exists := targetBoneIndexes[targetName]; exists {
		if bone, err := modelData.Bones.Get(idx); err == nil && bone != nil {
			return bone, true
		}
	}
	if bone, err := modelData.Bones.GetByName(targetName); err == nil && bone != nil {
		targetBoneIndexes[targetName] = bone.Index()
		return bone, true
	}
	return nil, false
}

// normalizeMappedRootParents は主要ボーンの親子関係を最小補正する。
func normalizeMappedRootParents(bones *model.BoneCollection) {
	if bones == nil {
		return
	}
	root, rootOK := getBoneByName(bones, model.ROOT.String())
	center, centerOK := getBoneByName(bones, model.CENTER.String())
	groove, grooveOK := getBoneByName(bones, model.GROOVE.String())
	waist, waistOK := getBoneByName(bones, model.WAIST.String())
	trunkRoot, trunkRootOK := getBoneByName(bones, model.TRUNK_ROOT.String())
	lower, lowerOK := getBoneByName(bones, model.LOWER.String())
	eyes, eyesOK := getBoneByName(bones, model.EYES.String())
	neckRoot, neckRootOK := getBoneByName(bones, model.NECK_ROOT.String())
	upper2, upper2OK := getBoneByName(bones, model.UPPER2.String())
	upper, upperOK := getBoneByName(bones, model.UPPER.String())

	if centerOK {
		if rootOK {
			center.ParentIndex = root.Index()
		} else {
			center.ParentIndex = -1
		}
	}
	if grooveOK && centerOK {
		groove.ParentIndex = center.Index()
	}
	if waistOK {
		switch {
		case trunkRootOK:
			waist.ParentIndex = trunkRoot.Index()
		case grooveOK:
			waist.ParentIndex = groove.Index()
		case centerOK:
			waist.ParentIndex = center.Index()
		}
	}
	if lowerOK && centerOK && lower.ParentIndex < 0 {
		lower.ParentIndex = center.Index()
	}
	if eyesOK && eyes.ParentIndex < 0 {
		switch {
		case neckRootOK:
			eyes.ParentIndex = neckRoot.Index()
		case upper2OK:
			eyes.ParentIndex = upper2.Index()
		case upperOK:
			eyes.ParentIndex = upper.Index()
		}
	}
}

// normalizeViewerIdealBoneStructure は viewer_ideal 契約の親子・表示先・層を正規化する。
func normalizeViewerIdealBoneStructure(modelData *ModelData) error {
	if modelData == nil || modelData.Bones == nil {
		return nil
	}
	if err := canonicalizeRootBoneAlias(modelData); err != nil {
		return err
	}
	normalizeViewerIdealStandardRelations(modelData.Bones)
	normalizeViewerIdealLayers(modelData.Bones)
	return nil
}

// canonicalizeRootBoneAlias は Root/root と 全ての親 の重複を解消する。
func canonicalizeRootBoneAlias(modelData *ModelData) error {
	if modelData == nil || modelData.Bones == nil {
		return nil
	}
	bones := modelData.Bones
	root, rootExists := getBoneByName(bones, model.ROOT.String())
	if !rootExists {
		for _, aliasName := range []string{"Root", "root"} {
			aliasBone, aliasExists := getBoneByName(bones, aliasName)
			if !aliasExists {
				continue
			}
			if _, err := bones.Rename(aliasBone.Index(), model.ROOT.String()); err != nil {
				return err
			}
			return nil
		}
		return nil
	}

	for _, aliasName := range []string{"Root", "root"} {
		aliasBone, aliasExists := getBoneByName(bones, aliasName)
		if !aliasExists || aliasBone.Index() == root.Index() {
			continue
		}
		replaceBoneReference(modelData.Bones, aliasBone.Index(), root.Index())
		if err := removeBoneAndReindexModel(modelData, aliasBone.Index()); err != nil {
			return err
		}
		var refreshed bool
		root, refreshed = getBoneByName(modelData.Bones, model.ROOT.String())
		if !refreshed {
			break
		}
	}
	return nil
}

// replaceBoneReference は fromIndex を toIndex へ参照置換する。
func replaceBoneReference(bones *model.BoneCollection, fromIndex int, toIndex int) {
	if bones == nil || fromIndex < 0 || toIndex < 0 || fromIndex == toIndex {
		return
	}
	for _, bone := range bones.Values() {
		if bone == nil {
			continue
		}
		if bone.ParentIndex == fromIndex {
			bone.ParentIndex = toIndex
		}
		if bone.TailIndex == fromIndex {
			bone.TailIndex = toIndex
		}
		if bone.EffectIndex == fromIndex {
			bone.EffectIndex = toIndex
		}
		if bone.Ik == nil {
			continue
		}
		if bone.Ik.BoneIndex == fromIndex {
			bone.Ik.BoneIndex = toIndex
		}
		for i := range bone.Ik.Links {
			if bone.Ik.Links[i].BoneIndex == fromIndex {
				bone.Ik.Links[i].BoneIndex = toIndex
			}
		}
	}
}

// normalizeViewerIdealStandardRelations は標準骨格の親子・表示先・付与を正規化する。
func normalizeViewerIdealStandardRelations(bones *model.BoneCollection) {
	if bones == nil {
		return
	}
	setBoneParentByName(bones, model.ROOT.String(), "")
	setBoneTailOffsetByName(bones, model.ROOT.String(), mmath.Vec3{Vec: r3.Vec{}})

	setBoneParentByName(bones, model.CENTER.String(), model.ROOT.String())
	setBoneTailOffsetByName(bones, model.CENTER.String(), mmath.Vec3{Vec: r3.Vec{X: 0, Y: 1, Z: 0}})

	setBoneParentByName(bones, model.GROOVE.String(), model.CENTER.String())
	setBoneTailOffsetByName(bones, model.GROOVE.String(), mmath.Vec3{Vec: r3.Vec{X: 0, Y: -1, Z: 0}})

	setBoneParentByName(bones, model.WAIST.String(), model.GROOVE.String())
	setBoneTailOffsetByName(bones, model.WAIST.String(), mmath.Vec3{Vec: r3.Vec{X: 0, Y: -1, Z: 0}})

	setBoneParentByName(bones, model.LOWER.String(), model.WAIST.String())
	setBoneTailOffsetByName(bones, model.LOWER.String(), mmath.Vec3{Vec: r3.Vec{X: 0, Y: -1, Z: 0}})

	setBoneParentByName(bones, model.UPPER.String(), model.WAIST.String())
	if !setBoneTailBoneByName(bones, model.UPPER.String(), "J_Bip_C_Chest") {
		setBoneTailBoneByName(bones, model.UPPER.String(), model.UPPER2.String())
	}

	if getBoneByNameExists(bones, "J_Bip_C_Chest") {
		setBoneParentByName(bones, "J_Bip_C_Chest", model.UPPER.String())
		setBoneTailBoneByName(bones, "J_Bip_C_Chest", model.UPPER2.String())
		setBoneParentByName(bones, model.UPPER2.String(), "J_Bip_C_Chest")
	} else {
		setBoneParentByName(bones, model.UPPER2.String(), model.UPPER.String())
	}
	setBoneTailBoneByName(bones, model.UPPER2.String(), model.NECK.String())
	setBoneParentByName(bones, model.NECK.String(), model.UPPER2.String())
	setBoneTailBoneByName(bones, model.NECK.String(), model.HEAD.String())
	setBoneParentByName(bones, model.HEAD.String(), model.NECK.String())
	setBoneTailOffsetByName(bones, model.HEAD.String(), mmath.Vec3{Vec: r3.Vec{X: 0, Y: 1, Z: 0}})

	setBoneParentByName(bones, model.EYES.String(), "")
	setBoneTailOffsetByName(bones, model.EYES.String(), mmath.Vec3{Vec: r3.Vec{X: 0, Y: 0, Z: -1}})
	setBoneParentByName(bones, model.EYE.Left(), model.HEAD.String())
	setBoneTailOffsetByName(bones, model.EYE.Left(), mmath.Vec3{Vec: r3.Vec{}})
	setBoneEffectByName(bones, model.EYE.Left(), model.EYES.String(), 0.3)
	setBoneParentByName(bones, model.EYE.Right(), model.HEAD.String())
	setBoneTailOffsetByName(bones, model.EYE.Right(), mmath.Vec3{Vec: r3.Vec{}})
	setBoneEffectByName(bones, model.EYE.Right(), model.EYES.String(), 0.3)

	setBoneParentByName(bones, "左胸", model.UPPER2.String())
	setBoneTailBoneByName(bones, "左胸", "左胸先")
	setBoneParentByName(bones, "左胸先", "左胸")
	setBoneTailOffsetByName(bones, "左胸先", mmath.Vec3{Vec: r3.Vec{}})
	setBoneParentByName(bones, "右胸", model.UPPER2.String())
	setBoneTailBoneByName(bones, "右胸", "右胸先")
	setBoneParentByName(bones, "右胸先", "右胸")
	setBoneTailOffsetByName(bones, "右胸先", mmath.Vec3{Vec: r3.Vec{}})

	for _, direction := range []model.BoneDirection{model.BONE_DIRECTION_LEFT, model.BONE_DIRECTION_RIGHT} {
		shoulderP := model.SHOULDER_P.StringFromDirection(direction)
		shoulder := model.SHOULDER.StringFromDirection(direction)
		shoulderC := model.SHOULDER_C.StringFromDirection(direction)
		arm := model.ARM.StringFromDirection(direction)
		armTwist := model.ARM_TWIST.StringFromDirection(direction)
		armTwist1 := model.ARM_TWIST1.StringFromDirection(direction)
		armTwist2 := model.ARM_TWIST2.StringFromDirection(direction)
		armTwist3 := model.ARM_TWIST3.StringFromDirection(direction)
		elbow := model.ELBOW.StringFromDirection(direction)
		wristTwist := model.WRIST_TWIST.StringFromDirection(direction)
		wristTwist1 := model.WRIST_TWIST1.StringFromDirection(direction)
		wristTwist2 := model.WRIST_TWIST2.StringFromDirection(direction)
		wristTwist3 := model.WRIST_TWIST3.StringFromDirection(direction)
		wrist := model.WRIST.StringFromDirection(direction)
		wristTail := resolveWristTipBoneName(bones, direction)

		setBoneParentByName(bones, shoulderP, model.UPPER2.String())
		setBoneTailOffsetByName(bones, shoulderP, mmath.Vec3{Vec: r3.Vec{}})
		setBoneParentByName(bones, shoulder, shoulderP)
		setBoneTailBoneByName(bones, shoulder, arm)
		setBoneParentByName(bones, shoulderC, shoulder)
		setBoneTailOffsetByName(bones, shoulderC, mmath.Vec3{Vec: r3.Vec{}})
		setBoneEffectByName(bones, shoulderC, shoulderP, -1)
		setBoneParentByName(bones, arm, shoulderC)
		setBoneTailBoneByName(bones, arm, elbow)
		setBoneParentByName(bones, armTwist, arm)
		setBoneTailOffsetByName(bones, armTwist, mmath.Vec3{Vec: r3.Vec{}})
		setBoneParentByName(bones, armTwist1, arm)
		setBoneTailOffsetByName(bones, armTwist1, mmath.Vec3{Vec: r3.Vec{}})
		setBoneEffectByName(bones, armTwist1, armTwist, 0.25)
		setBoneParentByName(bones, armTwist2, arm)
		setBoneTailOffsetByName(bones, armTwist2, mmath.Vec3{Vec: r3.Vec{}})
		setBoneEffectByName(bones, armTwist2, armTwist, 0.5)
		setBoneParentByName(bones, armTwist3, arm)
		setBoneTailOffsetByName(bones, armTwist3, mmath.Vec3{Vec: r3.Vec{}})
		setBoneEffectByName(bones, armTwist3, armTwist, 0.75)
		setBoneParentByName(bones, elbow, armTwist)
		setBoneTailBoneByName(bones, elbow, wrist)
		setBoneParentByName(bones, wristTwist, elbow)
		setBoneTailOffsetByName(bones, wristTwist, mmath.Vec3{Vec: r3.Vec{}})
		setBoneParentByName(bones, wristTwist1, elbow)
		setBoneTailOffsetByName(bones, wristTwist1, mmath.Vec3{Vec: r3.Vec{}})
		setBoneEffectByName(bones, wristTwist1, wristTwist, 0.25)
		setBoneParentByName(bones, wristTwist2, elbow)
		setBoneTailOffsetByName(bones, wristTwist2, mmath.Vec3{Vec: r3.Vec{}})
		setBoneEffectByName(bones, wristTwist2, wristTwist, 0.5)
		setBoneParentByName(bones, wristTwist3, elbow)
		setBoneTailOffsetByName(bones, wristTwist3, mmath.Vec3{Vec: r3.Vec{}})
		setBoneEffectByName(bones, wristTwist3, wristTwist, 0.75)
		setBoneParentByName(bones, wrist, wristTwist)
		setBoneTailBoneByName(bones, wrist, wristTail)
		setBoneParentByName(bones, wristTail, wrist)
		setBoneTailOffsetByName(bones, wristTail, mmath.Vec3{Vec: r3.Vec{}})
		thumbTip := resolveFingerTipBoneName(bones, model.THUMB_TAIL, direction)
		indexTip := resolveFingerTipBoneName(bones, model.INDEX_TAIL, direction)
		middleTip := resolveFingerTipBoneName(bones, model.MIDDLE_TAIL, direction)
		ringTip := resolveFingerTipBoneName(bones, model.RING_TAIL, direction)
		pinkyTip := resolveFingerTipBoneName(bones, model.PINKY_TAIL, direction)

		normalizeFingerChain(bones, wrist, []string{
			model.THUMB0.StringFromDirection(direction),
			model.THUMB1.StringFromDirection(direction),
			model.THUMB2.StringFromDirection(direction),
			thumbTip,
		})
		normalizeFingerChain(bones, wrist, []string{
			model.INDEX1.StringFromDirection(direction),
			model.INDEX2.StringFromDirection(direction),
			model.INDEX3.StringFromDirection(direction),
			indexTip,
		})
		normalizeFingerChain(bones, wrist, []string{
			model.MIDDLE1.StringFromDirection(direction),
			model.MIDDLE2.StringFromDirection(direction),
			model.MIDDLE3.StringFromDirection(direction),
			middleTip,
		})
		normalizeFingerChain(bones, wrist, []string{
			model.RING1.StringFromDirection(direction),
			model.RING2.StringFromDirection(direction),
			model.RING3.StringFromDirection(direction),
			ringTip,
		})
		normalizeFingerChain(bones, wrist, []string{
			model.PINKY1.StringFromDirection(direction),
			model.PINKY2.StringFromDirection(direction),
			model.PINKY3.StringFromDirection(direction),
			pinkyTip,
		})

		waistCancel := model.WAIST_CANCEL.StringFromDirection(direction)
		leg := model.LEG.StringFromDirection(direction)
		knee := model.KNEE.StringFromDirection(direction)
		ankle := model.ANKLE.StringFromDirection(direction)
		toeBase := toeHumanTargetNameByDirection(direction)
		legIKParent := model.LEG_IK_PARENT.StringFromDirection(direction)
		legIK := model.LEG_IK.StringFromDirection(direction)
		toeIK := model.TOE_IK.StringFromDirection(direction)
		legD := model.LEG_D.StringFromDirection(direction)
		kneeD := model.KNEE_D.StringFromDirection(direction)
		ankleD := model.ANKLE_D.StringFromDirection(direction)
		toeEx := model.TOE_EX.StringFromDirection(direction)

		setBoneParentByName(bones, waistCancel, model.LOWER.String())
		setBoneTailOffsetByName(bones, waistCancel, mmath.Vec3{Vec: r3.Vec{}})
		setBoneEffectByName(bones, waistCancel, model.WAIST.String(), -1)
		setBoneParentByName(bones, leg, waistCancel)
		setBoneTailBoneByName(bones, leg, knee)
		setBoneParentByName(bones, knee, leg)
		setBoneTailBoneByName(bones, knee, ankle)
		setBoneParentByName(bones, ankle, knee)
		setBoneTailBoneByName(bones, ankle, toeBase)
		setBoneParentByName(bones, toeBase, ankle)
		setToeBaseTailOffset(bones, toeBase, ankle)

		setBoneParentByName(bones, legIKParent, model.ROOT.String())
		setBoneTailOffsetByName(bones, legIKParent, mmath.Vec3{Vec: r3.Vec{}})
		setBoneParentByName(bones, legIK, legIKParent)
		setBoneTailBoneByName(bones, legIK, toeIK)
		normalizeLegIkTargetAndSolverByDirection(bones, direction)
		setBoneParentByName(bones, toeIK, legIK)
		normalizeToeIkTailOffset(bones, toeIK)
		normalizeToeIkTargetByDirection(bones, direction)

		setBoneParentByName(bones, legD, waistCancel)
		setBoneTailOffsetByName(bones, legD, mmath.Vec3{Vec: r3.Vec{}})
		setBoneEffectByName(bones, legD, leg, 1.0)
		setBoneParentByName(bones, kneeD, legD)
		setBoneTailOffsetByName(bones, kneeD, mmath.Vec3{Vec: r3.Vec{}})
		setBoneEffectByName(bones, kneeD, knee, 1.0)
		setBoneParentByName(bones, ankleD, kneeD)
		setBoneTailOffsetByName(bones, ankleD, mmath.Vec3{Vec: r3.Vec{}})
		setBoneEffectByName(bones, ankleD, ankle, 1.0)
		setBoneParentByName(bones, toeEx, ankleD)
		setBoneTailOffsetByName(bones, toeEx, mmath.Vec3{Vec: r3.Vec{}})
		clearBoneEffectByName(bones, toeEx)
	}
}

// normalizeFingerChain は手首起点の指チェーン親子と表示先を正規化する。
func normalizeFingerChain(bones *model.BoneCollection, wristName string, chainNames []string) {
	if bones == nil || len(chainNames) == 0 {
		return
	}
	existingChain := make([]string, 0, len(chainNames))
	for _, chainName := range chainNames {
		if getBoneByNameExists(bones, chainName) {
			existingChain = append(existingChain, chainName)
		}
	}
	if len(existingChain) == 0 {
		return
	}
	setBoneParentByName(bones, existingChain[0], wristName)
	for i := 0; i < len(existingChain)-1; i++ {
		setBoneTailBoneByName(bones, existingChain[i], existingChain[i+1])
		setBoneParentByName(bones, existingChain[i+1], existingChain[i])
	}
	setBoneTailOffsetByName(bones, existingChain[len(existingChain)-1], mmath.Vec3{Vec: r3.Vec{}})
}

// setToeBaseTailOffset はつま先の表示先オフセットを足首差分の0.5倍に設定する。
func setToeBaseTailOffset(bones *model.BoneCollection, toeName string, ankleName string) {
	if bones == nil {
		return
	}
	toe, toeExists := getBoneByName(bones, toeName)
	ankle, ankleExists := getBoneByName(bones, ankleName)
	if !toeExists || !ankleExists {
		return
	}
	offset := mmath.Vec3{Vec: r3.Vec{
		X: (toe.Position.X - ankle.Position.X) * 0.5,
		Y: (toe.Position.Y - ankle.Position.Y) * 0.5,
		Z: (toe.Position.Z - ankle.Position.Z) * 0.5,
	}}
	setBoneTailOffsetByName(bones, toeName, offset)
}

// resolveWristTipBoneName は左右の手首先名を優先順で解決する。
func resolveWristTipBoneName(bones *model.BoneCollection, direction model.BoneDirection) string {
	if bones == nil {
		return model.WRIST_TAIL.StringFromDirection(direction)
	}
	switch direction {
	case model.BONE_DIRECTION_LEFT:
		if bones.ContainsByName(leftWristTipName) {
			return leftWristTipName
		}
		return model.WRIST_TAIL.Left()
	case model.BONE_DIRECTION_RIGHT:
		if bones.ContainsByName(rightWristTipName) {
			return rightWristTipName
		}
		return model.WRIST_TAIL.Right()
	default:
		return model.WRIST_TAIL.StringFromDirection(direction)
	}
}

// resolveFingerTipBoneName は指先名をエイリアス優先順で解決する。
func resolveFingerTipBoneName(
	bones *model.BoneCollection,
	standardTip model.StandardBoneName,
	direction model.BoneDirection,
) string {
	alias := fingerTipAliasNameFromTail(standardTip, direction)
	if bones != nil && alias != standardTip.StringFromDirection(direction) && bones.ContainsByName(alias) {
		return alias
	}
	return standardTip.StringFromDirection(direction)
}

// fingerTipAliasNameFromTail は標準指先名に対応する表示名エイリアスを返す。
func fingerTipAliasNameFromTail(standardTip model.StandardBoneName, direction model.BoneDirection) string {
	switch standardTip {
	case model.THUMB_TAIL:
		return thumbTipNameFromDirection(direction)
	case model.INDEX_TAIL:
		return indexTipNameFromDirection(direction)
	case model.MIDDLE_TAIL:
		return middleTipNameFromDirection(direction)
	case model.RING_TAIL:
		return ringTipNameFromDirection(direction)
	case model.PINKY_TAIL:
		return pinkyTipNameFromDirection(direction)
	default:
		return standardTip.StringFromDirection(direction)
	}
}

// thumbTipNameFromDirection は左右方向に対応する親指先名を返す。
func thumbTipNameFromDirection(direction model.BoneDirection) string {
	switch direction {
	case model.BONE_DIRECTION_LEFT:
		return leftThumbTipName
	case model.BONE_DIRECTION_RIGHT:
		return rightThumbTipName
	default:
		return model.THUMB_TAIL.StringFromDirection(direction)
	}
}

// indexTipNameFromDirection は左右方向に対応する人指先名を返す。
func indexTipNameFromDirection(direction model.BoneDirection) string {
	switch direction {
	case model.BONE_DIRECTION_LEFT:
		return leftIndexTipName
	case model.BONE_DIRECTION_RIGHT:
		return rightIndexTipName
	default:
		return model.INDEX_TAIL.StringFromDirection(direction)
	}
}

// middleTipNameFromDirection は左右方向に対応する中指先名を返す。
func middleTipNameFromDirection(direction model.BoneDirection) string {
	switch direction {
	case model.BONE_DIRECTION_LEFT:
		return leftMiddleTipName
	case model.BONE_DIRECTION_RIGHT:
		return rightMiddleTipName
	default:
		return model.MIDDLE_TAIL.StringFromDirection(direction)
	}
}

// ringTipNameFromDirection は左右方向に対応する薬指先名を返す。
func ringTipNameFromDirection(direction model.BoneDirection) string {
	switch direction {
	case model.BONE_DIRECTION_LEFT:
		return leftRingTipName
	case model.BONE_DIRECTION_RIGHT:
		return rightRingTipName
	default:
		return model.RING_TAIL.StringFromDirection(direction)
	}
}

// pinkyTipNameFromDirection は左右方向に対応する小指先名を返す。
func pinkyTipNameFromDirection(direction model.BoneDirection) string {
	switch direction {
	case model.BONE_DIRECTION_LEFT:
		return leftPinkyTipName
	case model.BONE_DIRECTION_RIGHT:
		return rightPinkyTipName
	default:
		return model.PINKY_TAIL.StringFromDirection(direction)
	}
}

// wristTipNameFromDirection は左右方向に対応する手首先名を返す。
func wristTipNameFromDirection(direction model.BoneDirection) string {
	switch direction {
	case model.BONE_DIRECTION_LEFT:
		return leftWristTipName
	case model.BONE_DIRECTION_RIGHT:
		return rightWristTipName
	default:
		return model.WRIST_TAIL.StringFromDirection(direction)
	}
}

// normalizeToeIkTailOffset はつま先IKの表示先をオフセット型へ正規化する。
func normalizeToeIkTailOffset(bones *model.BoneCollection, toeIkName string) {
	if bones == nil {
		return
	}
	toeIk, exists := getBoneByName(bones, toeIkName)
	if !exists {
		return
	}
	if toeIk.TailIndex >= 0 {
		toeIk.TailIndex = -1
		toeIk.TailPosition = mmath.Vec3{Vec: r3.Vec{}}
		return
	}
	if toeIk.TailPosition.Length() <= 1e-8 {
		toeIk.TailPosition = mmath.Vec3{Vec: r3.Vec{}}
	}
}

// normalizeToeIkTargetByDirection はつま先IKのターゲットを左右つま先へ正規化する。
func normalizeToeIkTargetByDirection(bones *model.BoneCollection, direction model.BoneDirection) {
	if bones == nil {
		return
	}
	toeIkName := model.TOE_IK.StringFromDirection(direction)
	toeIk, toeIkExists := getBoneByName(bones, toeIkName)
	toeBase, toeBaseExists := getBoneByName(bones, toeHumanTargetNameByDirection(direction))
	if !toeIkExists || !toeBaseExists {
		return
	}
	if toeIk.Ik == nil {
		toeIk.Ik = &model.Ik{
			BoneIndex:    toeBase.Index(),
			LoopCount:    40,
			UnitRotation: mmath.Vec3{Vec: r3.Vec{X: 1, Y: 1, Z: 1}},
			Links:        []model.IkLink{},
		}
	} else {
		toeIk.Ik.BoneIndex = toeBase.Index()
	}
	normalizeToeIkSolver(toeIk)
	if ankle, ankleExists := getBoneByName(bones, model.ANKLE.StringFromDirection(direction)); ankleExists {
		toeIk.Ik.Links = []model.IkLink{{BoneIndex: ankle.Index()}}
	}
}

// normalizeLegIkTargetAndSolverByDirection は足IKターゲットとソルバ値を理想値へ正規化する。
func normalizeLegIkTargetAndSolverByDirection(bones *model.BoneCollection, direction model.BoneDirection) {
	if bones == nil {
		return
	}
	legIkName := model.LEG_IK.StringFromDirection(direction)
	legIk, legIkExists := getBoneByName(bones, legIkName)
	if !legIkExists {
		return
	}
	ankle, ankleExists := getBoneByName(bones, model.ANKLE.StringFromDirection(direction))
	if !ankleExists {
		return
	}
	if legIk.Ik == nil {
		ikLinks := make([]model.IkLink, 0, 2)
		if knee, kneeExists := getBoneByName(bones, model.KNEE.StringFromDirection(direction)); kneeExists {
			ikLinks = append(ikLinks, model.IkLink{
				BoneIndex:     knee.Index(),
				AngleLimit:    true,
				MinAngleLimit: mmath.Vec3{Vec: r3.Vec{X: mmath.DegToRad(-180.0), Y: 0.0, Z: 0.0}},
				MaxAngleLimit: mmath.Vec3{Vec: r3.Vec{X: mmath.DegToRad(-0.5), Y: 0.0, Z: 0.0}},
			})
		}
		if leg, legExists := getBoneByName(bones, model.LEG.StringFromDirection(direction)); legExists {
			ikLinks = append(ikLinks, model.IkLink{BoneIndex: leg.Index()})
		}
		legIk.Ik = &model.Ik{
			BoneIndex:    ankle.Index(),
			LoopCount:    40,
			UnitRotation: mmath.Vec3{Vec: r3.Vec{X: 1, Y: 1, Z: 1}},
			Links:        ikLinks,
		}
		return
	}
	legIk.Ik.BoneIndex = ankle.Index()
	if legIk.Ik.LoopCount < 40 {
		legIk.Ik.LoopCount = 40
	}
	legIk.Ik.UnitRotation = mmath.Vec3{Vec: r3.Vec{X: 1, Y: 1, Z: 1}}
}

// normalizeToeIkSolver はつま先IKの反復回数と単位角を理想値へ正規化する。
func normalizeToeIkSolver(toeIk *model.Bone) {
	if toeIk == nil || toeIk.Ik == nil {
		return
	}
	if toeIk.Ik.LoopCount < 40 {
		toeIk.Ik.LoopCount = 40
	}
	toeIk.Ik.UnitRotation = mmath.Vec3{Vec: r3.Vec{X: 1, Y: 1, Z: 1}}
}

// getBoneByNameExists は指定名ボーンの存在を返す。
func getBoneByNameExists(bones *model.BoneCollection, name string) bool {
	_, exists := getBoneByName(bones, name)
	return exists
}

// setBoneParentByName はボーン親を名前指定で設定する。
func setBoneParentByName(bones *model.BoneCollection, boneName string, parentName string) {
	if bones == nil {
		return
	}
	bone, exists := getBoneByName(bones, boneName)
	if !exists {
		return
	}
	if parentName == "" {
		bone.ParentIndex = -1
		return
	}
	parent, parentExists := getBoneByName(bones, parentName)
	if !parentExists {
		return
	}
	bone.ParentIndex = parent.Index()
}

// setBoneTailBoneByName は表示先をボーン接続へ設定する。
func setBoneTailBoneByName(bones *model.BoneCollection, boneName string, tailBoneName string) bool {
	if bones == nil {
		return false
	}
	bone, exists := getBoneByName(bones, boneName)
	if !exists {
		return false
	}
	tailBone, tailExists := getBoneByName(bones, tailBoneName)
	if !tailExists {
		return false
	}
	bone.TailIndex = tailBone.Index()
	bone.TailPosition = mmath.Vec3{Vec: r3.Vec{}}
	return true
}

// setBoneTailOffsetByName は表示先をオフセット接続へ設定する。
func setBoneTailOffsetByName(bones *model.BoneCollection, boneName string, offset mmath.Vec3) {
	if bones == nil {
		return
	}
	bone, exists := getBoneByName(bones, boneName)
	if !exists {
		return
	}
	bone.TailIndex = -1
	bone.TailPosition = offset
}

// setBoneTailOffsetIfInvalidByName は表示先が未設定時のみオフセットを設定する。
func setBoneTailOffsetIfInvalidByName(bones *model.BoneCollection, boneName string, offset mmath.Vec3) {
	if bones == nil {
		return
	}
	bone, exists := getBoneByName(bones, boneName)
	if !exists {
		return
	}
	if bone.TailIndex >= 0 {
		return
	}
	if bone.TailPosition.Length() > 1e-8 {
		return
	}
	bone.TailPosition = offset
}

// setBoneEffectByName は回転付与設定を名前指定で設定する。
func setBoneEffectByName(bones *model.BoneCollection, boneName string, effectParentName string, factor float64) {
	if bones == nil {
		return
	}
	bone, exists := getBoneByName(bones, boneName)
	if !exists {
		return
	}
	parent, parentExists := getBoneByName(bones, effectParentName)
	if !parentExists {
		return
	}
	bone.EffectIndex = parent.Index()
	bone.EffectFactor = factor
}

// clearBoneEffectByName は付与設定を無効化する。
func clearBoneEffectByName(bones *model.BoneCollection, boneName string) {
	if bones == nil {
		return
	}
	bone, exists := getBoneByName(bones, boneName)
	if !exists {
		return
	}
	bone.EffectIndex = -1
	bone.EffectFactor = 0
}

// normalizeViewerIdealLayers は viewer_ideal の変形階層契約へ正規化する。
func normalizeViewerIdealLayers(bones *model.BoneCollection) {
	if bones == nil {
		return
	}
	for _, bone := range bones.Values() {
		if bone == nil {
			continue
		}
		bone.Layer = 0
	}
	for _, direction := range []model.BoneDirection{model.BONE_DIRECTION_LEFT, model.BONE_DIRECTION_RIGHT} {
		for _, name := range []string{
			model.LEG_D.StringFromDirection(direction),
			model.KNEE_D.StringFromDirection(direction),
			model.ANKLE_D.StringFromDirection(direction),
			model.TOE_EX.StringFromDirection(direction),
		} {
			bone, exists := getBoneByName(bones, name)
			if !exists {
				continue
			}
			bone.Layer = 1
		}
	}
}

// normalizeViewerIdealBoneOrder は標準骨格優先順へ並び替えて参照indexを再マッピングする。
func normalizeViewerIdealBoneOrder(modelData *ModelData) {
	if modelData == nil || modelData.Bones == nil {
		return
	}
	oldLen := modelData.Bones.Len()
	if oldLen <= 1 {
		return
	}
	preferred := buildViewerIdealPreferredBoneNames()
	orderIndexes := make([]int, 0, oldLen)
	usedIndexes := map[int]struct{}{}
	for _, name := range preferred {
		bone, exists := getBoneByName(modelData.Bones, name)
		if !exists {
			continue
		}
		if _, used := usedIndexes[bone.Index()]; used {
			continue
		}
		orderIndexes = append(orderIndexes, bone.Index())
		usedIndexes[bone.Index()] = struct{}{}
	}
	for index := 0; index < oldLen; index++ {
		if _, used := usedIndexes[index]; used {
			continue
		}
		orderIndexes = append(orderIndexes, index)
	}
	if len(orderIndexes) != oldLen {
		return
	}
	noChange := true
	for index, oldIndex := range orderIndexes {
		if index != oldIndex {
			noChange = false
			break
		}
	}
	if noChange {
		return
	}

	oldToNew := make([]int, oldLen)
	newBones := model.NewBoneCollection(oldLen)
	for newIndex, oldIndex := range orderIndexes {
		bone, err := modelData.Bones.Get(oldIndex)
		if err != nil || bone == nil {
			return
		}
		newBones.AppendRaw(bone)
		oldToNew[oldIndex] = newIndex
	}
	modelData.Bones = newBones
	applyBoneReindexToModel(modelData, oldToNew)
}

// buildViewerIdealPreferredBoneNames は viewer_ideal 向けの優先ボーン順を返す。
func buildViewerIdealPreferredBoneNames() []string {
	names := []string{
		model.ROOT.String(),
		model.CENTER.String(),
		model.GROOVE.String(),
		model.WAIST.String(),
		model.LOWER.String(),
		model.UPPER.String(),
		"J_Bip_C_Chest",
		model.UPPER2.String(),
		"左胸",
		"左胸先",
		"右胸",
		"右胸先",
		model.NECK.String(),
		model.HEAD.String(),
		tongueBone1Name,
		tongueBone2Name,
		tongueBone3Name,
		tongueBone4Name,
		model.EYES.String(),
		model.EYE.Left(),
		model.EYE.Right(),
	}
	for _, direction := range []model.BoneDirection{model.BONE_DIRECTION_LEFT, model.BONE_DIRECTION_RIGHT} {
		names = append(names,
			model.SHOULDER_P.StringFromDirection(direction),
			model.SHOULDER.StringFromDirection(direction),
			model.SHOULDER_C.StringFromDirection(direction),
			model.ARM.StringFromDirection(direction),
			model.ARM_TWIST.StringFromDirection(direction),
			model.ARM_TWIST1.StringFromDirection(direction),
			model.ARM_TWIST2.StringFromDirection(direction),
			model.ARM_TWIST3.StringFromDirection(direction),
			model.ELBOW.StringFromDirection(direction),
			model.WRIST_TWIST.StringFromDirection(direction),
			model.WRIST_TWIST1.StringFromDirection(direction),
			model.WRIST_TWIST2.StringFromDirection(direction),
			model.WRIST_TWIST3.StringFromDirection(direction),
			model.WRIST.StringFromDirection(direction),
			wristTipNameFromDirection(direction),
			model.WRIST_TAIL.StringFromDirection(direction),
			model.THUMB0.StringFromDirection(direction),
			model.THUMB1.StringFromDirection(direction),
			model.THUMB2.StringFromDirection(direction),
			thumbTipNameFromDirection(direction),
			model.THUMB_TAIL.StringFromDirection(direction),
			model.INDEX1.StringFromDirection(direction),
			model.INDEX2.StringFromDirection(direction),
			model.INDEX3.StringFromDirection(direction),
			indexTipNameFromDirection(direction),
			model.INDEX_TAIL.StringFromDirection(direction),
			model.MIDDLE1.StringFromDirection(direction),
			model.MIDDLE2.StringFromDirection(direction),
			model.MIDDLE3.StringFromDirection(direction),
			middleTipNameFromDirection(direction),
			model.MIDDLE_TAIL.StringFromDirection(direction),
			model.RING1.StringFromDirection(direction),
			model.RING2.StringFromDirection(direction),
			model.RING3.StringFromDirection(direction),
			ringTipNameFromDirection(direction),
			model.RING_TAIL.StringFromDirection(direction),
			model.PINKY1.StringFromDirection(direction),
			model.PINKY2.StringFromDirection(direction),
			model.PINKY3.StringFromDirection(direction),
			pinkyTipNameFromDirection(direction),
			model.PINKY_TAIL.StringFromDirection(direction),
			model.WAIST_CANCEL.StringFromDirection(direction),
			model.LEG.StringFromDirection(direction),
			model.KNEE.StringFromDirection(direction),
			model.ANKLE.StringFromDirection(direction),
			toeHumanTargetNameByDirection(direction),
			model.LEG_IK_PARENT.StringFromDirection(direction),
			model.LEG_IK.StringFromDirection(direction),
			model.TOE_IK.StringFromDirection(direction),
			model.LEG_D.StringFromDirection(direction),
			model.KNEE_D.StringFromDirection(direction),
			model.ANKLE_D.StringFromDirection(direction),
			model.TOE_EX.StringFromDirection(direction),
		)
	}
	return names
}

// removeExplicitBonesAndReindex は明示削除対象ボーンを削除し、参照indexを再マッピングする。
func removeExplicitBonesAndReindex(modelData *ModelData) error {
	if modelData == nil || modelData.Bones == nil {
		return nil
	}
	indexes := collectExplicitRemoveBoneIndexes(modelData.Bones)
	if len(indexes) == 0 {
		return nil
	}
	sort.Slice(indexes, func(i int, j int) bool {
		return indexes[i] > indexes[j]
	})
	for _, index := range indexes {
		if err := removeBoneAndReindexModel(modelData, index); err != nil {
			return err
		}
	}
	return nil
}

// collectExplicitRemoveBoneIndexes は明示削除対象ボーンのindex一覧を収集する。
func collectExplicitRemoveBoneIndexes(bones *model.BoneCollection) []int {
	if bones == nil {
		return []int{}
	}
	indexes := make([]int, 0, 4)
	for index := 0; index < bones.Len(); index++ {
		bone, err := bones.Get(index)
		if err != nil || bone == nil {
			continue
		}
		if shouldRemoveBoneByName(bone.Name()) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

// shouldRemoveBoneByName は明示削除対象名かを判定する。
func shouldRemoveBoneByName(name string) bool {
	if name == "" {
		return false
	}
	trimmed := strings.TrimSpace(name)
	_, exists := explicitRemoveBoneNames[strings.ToLower(trimmed)]
	if exists {
		return true
	}
	for _, direction := range []model.BoneDirection{model.BONE_DIRECTION_LEFT, model.BONE_DIRECTION_RIGHT} {
		if trimmed == model.TOE_T.StringFromDirection(direction) {
			return true
		}
		if trimmed == model.HEEL.StringFromDirection(direction) {
			return true
		}
	}
	return false
}

// removeBoneAndReindexModel はボーン1件を削除し、関連参照を再マッピングする。
func removeBoneAndReindexModel(modelData *ModelData, index int) error {
	if modelData == nil || modelData.Bones == nil {
		return nil
	}
	result, err := modelData.Bones.Remove(index)
	if err != nil {
		return err
	}
	applyBoneReindexToModel(modelData, result.OldToNew)
	return nil
}

// applyBoneReindexToModel はボーン削除後のindexマップをモデル全体へ適用する。
func applyBoneReindexToModel(modelData *ModelData, oldToNew []int) {
	if modelData == nil || len(oldToNew) == 0 {
		return
	}
	if modelData.Bones != nil {
		for _, bone := range modelData.Bones.Values() {
			if bone == nil {
				continue
			}
			bone.ParentIndex = remapBoneIndex(bone.ParentIndex, oldToNew)
			if bone.TailIndex >= 0 {
				bone.TailIndex = remapBoneIndex(bone.TailIndex, oldToNew)
				if bone.TailIndex < 0 {
					bone.BoneFlag &^= model.BONE_FLAG_TAIL_IS_BONE
				}
			}
			bone.EffectIndex = remapBoneIndex(bone.EffectIndex, oldToNew)
			if bone.EffectIndex < 0 {
				bone.BoneFlag &^= model.BONE_FLAG_IS_EXTERNAL_ROTATION
				bone.BoneFlag &^= model.BONE_FLAG_IS_EXTERNAL_TRANSLATION
			}
			if bone.Ik == nil {
				continue
			}
			bone.Ik.BoneIndex = remapBoneIndex(bone.Ik.BoneIndex, oldToNew)
			links := make([]model.IkLink, 0, len(bone.Ik.Links))
			for _, link := range bone.Ik.Links {
				link.BoneIndex = remapBoneIndex(link.BoneIndex, oldToNew)
				if link.BoneIndex < 0 {
					continue
				}
				links = append(links, link)
			}
			bone.Ik.Links = links
			if bone.Ik.BoneIndex < 0 {
				bone.Ik = nil
				bone.BoneFlag &^= model.BONE_FLAG_IS_IK
			}
		}
	}
	if modelData.Vertices != nil {
		for _, vertex := range modelData.Vertices.Values() {
			if vertex == nil || vertex.Deform == nil {
				continue
			}
			remapBoneDeform(vertex, oldToNew)
		}
	}
	if modelData.Morphs != nil {
		for _, morph := range modelData.Morphs.Values() {
			if morph == nil {
				continue
			}
			remapBoneMorphOffsets(morph, oldToNew)
		}
	}
	if modelData.DisplaySlots != nil {
		for _, slot := range modelData.DisplaySlots.Values() {
			if slot == nil {
				continue
			}
			remapDisplaySlotReferences(slot, oldToNew)
		}
	}
	if modelData.RigidBodies != nil {
		for _, rigidBody := range modelData.RigidBodies.Values() {
			if rigidBody == nil {
				continue
			}
			rigidBody.BoneIndex = remapBoneIndex(rigidBody.BoneIndex, oldToNew)
		}
	}
}

// remapBoneIndex は再マッピング後のボーンindexを返す。
func remapBoneIndex(index int, oldToNew []int) int {
	if index < 0 {
		return -1
	}
	if index >= len(oldToNew) {
		return -1
	}
	return oldToNew[index]
}

// remapBoneDeform は頂点デフォームのボーンindexを再マッピングする。
func remapBoneDeform(vertex *model.Vertex, oldToNew []int) {
	if vertex == nil || vertex.Deform == nil {
		return
	}
	joints := append([]int(nil), vertex.Deform.Indexes()...)
	weights := append([]float64(nil), vertex.Deform.Weights()...)
	for i := range joints {
		joints[i] = remapBoneIndex(joints[i], oldToNew)
	}
	fallbackIndex := resolveFallbackBoneIndex(joints)
	vertex.Deform = buildNormalizedDeform(joints, weights, fallbackIndex)
	vertex.DeformType = vertex.Deform.DeformType()
}

// remapBoneMorphOffsets はボーンモーフ参照indexを再マッピングする。
func remapBoneMorphOffsets(morph *model.Morph, oldToNew []int) {
	if morph == nil || len(morph.Offsets) == 0 {
		return
	}
	offsets := make([]model.IMorphOffset, 0, len(morph.Offsets))
	for _, offset := range morph.Offsets {
		boneOffset, isBoneOffset := offset.(*model.BoneMorphOffset)
		if !isBoneOffset {
			offsets = append(offsets, offset)
			continue
		}
		boneOffset.BoneIndex = remapBoneIndex(boneOffset.BoneIndex, oldToNew)
		if boneOffset.BoneIndex < 0 {
			continue
		}
		offsets = append(offsets, boneOffset)
	}
	morph.Offsets = offsets
}

// remapDisplaySlotReferences は表示枠のボーン参照indexを再マッピングする。
func remapDisplaySlotReferences(slot *model.DisplaySlot, oldToNew []int) {
	if slot == nil || len(slot.References) == 0 {
		return
	}
	references := make([]model.Reference, 0, len(slot.References))
	for _, reference := range slot.References {
		if reference.DisplayType != model.DISPLAY_TYPE_BONE {
			references = append(references, reference)
			continue
		}
		reference.DisplayIndex = remapBoneIndex(reference.DisplayIndex, oldToNew)
		if reference.DisplayIndex < 0 {
			continue
		}
		references = append(references, reference)
	}
	slot.References = references
}

// normalizeBoneNamesAndEnglish は英名設定とJ_Sec系名称正規化を適用する。
func normalizeBoneNamesAndEnglish(bones *model.BoneCollection) error {
	if bones == nil {
		return nil
	}
	if err := renameJSecBones(bones); err != nil {
		return err
	}
	if err := normalizeWristTipNames(bones); err != nil {
		return err
	}
	if err := normalizeFingerTipNames(bones); err != nil {
		return err
	}
	applyBoneEnglishNames(bones)
	return nil
}

// normalizeWristTipNames は手首先先を手首先へ正規化する。
func normalizeWristTipNames(bones *model.BoneCollection) error {
	if bones == nil {
		return nil
	}
	for _, pair := range [][2]string{
		{model.WRIST_TAIL.Left(), leftWristTipName},
		{model.WRIST_TAIL.Right(), rightWristTipName},
	} {
		source, sourceExists := getBoneByName(bones, pair[0])
		if !sourceExists {
			continue
		}
		if bones.ContainsByName(pair[1]) {
			continue
		}
		if _, err := bones.Rename(source.Index(), pair[1]); err != nil {
			return err
		}
	}
	return nil
}

// normalizeFingerTipNames は指先先を指先へ正規化する。
func normalizeFingerTipNames(bones *model.BoneCollection) error {
	if bones == nil {
		return nil
	}
	for _, direction := range []model.BoneDirection{model.BONE_DIRECTION_LEFT, model.BONE_DIRECTION_RIGHT} {
		for _, pair := range [][2]string{
			{model.THUMB_TAIL.StringFromDirection(direction), thumbTipNameFromDirection(direction)},
			{model.INDEX_TAIL.StringFromDirection(direction), indexTipNameFromDirection(direction)},
			{model.MIDDLE_TAIL.StringFromDirection(direction), middleTipNameFromDirection(direction)},
			{model.RING_TAIL.StringFromDirection(direction), ringTipNameFromDirection(direction)},
			{model.PINKY_TAIL.StringFromDirection(direction), pinkyTipNameFromDirection(direction)},
		} {
			source, sourceExists := getBoneByName(bones, pair[0])
			if !sourceExists {
				continue
			}
			if bones.ContainsByName(pair[1]) {
				continue
			}
			if _, err := bones.Rename(source.Index(), pair[1]); err != nil {
				return err
			}
		}
	}
	return nil
}

// renameJSecBones はJ_Sec系ボーン名を短縮正規化し、重複時は連番で解決する。
func renameJSecBones(bones *model.BoneCollection) error {
	if bones == nil {
		return nil
	}
	renames := collectJSecBoneRenames(bones)
	if len(renames) == 0 {
		return nil
	}
	assignUniqueRenameNames(bones, renames)
	return applyIndexedBoneRenames(bones, renames)
}

// collectJSecBoneRenames はJ_Sec系ボーン名変更候補を収集する。
func collectJSecBoneRenames(bones *model.BoneCollection) []indexedBoneRename {
	if bones == nil {
		return []indexedBoneRename{}
	}
	renames := make([]indexedBoneRename, 0, 16)
	for index := 0; index < bones.Len(); index++ {
		bone, err := bones.Get(index)
		if err != nil || bone == nil {
			continue
		}
		if !isJSecBoneName(bone.Name()) {
			continue
		}
		renames = append(renames, indexedBoneRename{
			Index:   index,
			NewName: abbreviateJSecBoneName(bone.Name()),
		})
	}
	return renames
}

// assignUniqueRenameNames は候補名の重複を連番で解決する。
func assignUniqueRenameNames(bones *model.BoneCollection, renames []indexedBoneRename) {
	if bones == nil || len(renames) == 0 {
		return
	}
	targetIndexes := map[int]struct{}{}
	for _, rename := range renames {
		targetIndexes[rename.Index] = struct{}{}
	}
	usedNames := map[string]struct{}{}
	for index := 0; index < bones.Len(); index++ {
		if _, isRenameTarget := targetIndexes[index]; isRenameTarget {
			continue
		}
		bone, err := bones.Get(index)
		if err != nil || bone == nil {
			continue
		}
		usedNames[bone.Name()] = struct{}{}
	}
	for i := range renames {
		base := strings.TrimSpace(renames[i].NewName)
		if base == "" {
			base = fmt.Sprintf("Bone_%d", renames[i].Index)
		}
		candidate := base
		serial := 2
		for {
			if _, exists := usedNames[candidate]; !exists {
				break
			}
			candidate = fmt.Sprintf("%s_%d", base, serial)
			serial++
		}
		renames[i].NewName = candidate
		usedNames[candidate] = struct{}{}
	}
}

// applyIndexedBoneRenames はindex指定の命名変更を安全に適用する。
func applyIndexedBoneRenames(bones *model.BoneCollection, renames []indexedBoneRename) error {
	if bones == nil || len(renames) == 0 {
		return nil
	}
	tempSerial := 0
	applied := make([]indexedBoneRename, 0, len(renames))
	for _, rename := range renames {
		bone, err := bones.Get(rename.Index)
		if err != nil || bone == nil {
			continue
		}
		if bone.Name() == rename.NewName {
			continue
		}
		tempName := nextTemporaryBoneName(bones, &tempSerial)
		if _, err := bones.Rename(rename.Index, tempName); err != nil {
			return err
		}
		applied = append(applied, rename)
	}
	for _, rename := range applied {
		if _, err := bones.Rename(rename.Index, rename.NewName); err != nil {
			return err
		}
	}
	return nil
}

// isJSecBoneName はJ_Sec系ボーン名かを判定する。
func isJSecBoneName(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), "J_Sec")
}

// trimJSecPrefix はJ_Sec接頭辞を除去した名称を返す。
func trimJSecPrefix(name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	if !strings.HasPrefix(trimmed, "J_Sec") {
		return "", false
	}
	trimmed = strings.TrimPrefix(trimmed, "J_Sec")
	trimmed = strings.TrimPrefix(trimmed, "_")
	return trimmed, true
}

// abbreviateJSecBoneName はJ_Sec系ボーン名を決定的に短縮正規化する。
func abbreviateJSecBoneName(name string) string {
	trimmed, ok := trimJSecPrefix(name)
	if !ok {
		return name
	}
	parts := strings.Split(trimmed, "_")
	if len(parts) == 0 {
		return trimmed
	}
	type tokenPart struct {
		Text      string
		IsNumeric bool
	}
	shortParts := make([]tokenPart, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		isNumeric := true
		for _, r := range part {
			if !unicode.IsDigit(r) {
				isNumeric = false
				break
			}
		}
		if isNumeric {
			shortParts = append(shortParts, tokenPart{
				Text:      part,
				IsNumeric: true,
			})
			continue
		}
		short := abbreviateJSecToken(part)
		if short == "" {
			short = part
		}
		shortParts = append(shortParts, tokenPart{
			Text:      short,
			IsNumeric: false,
		})
	}
	if len(shortParts) == 0 {
		return trimmed
	}
	builder := strings.Builder{}
	for i, part := range shortParts {
		if i > 0 && part.IsNumeric {
			builder.WriteString("_")
		}
		builder.WriteString(part.Text)
	}
	result := builder.String()
	if result == "" {
		return trimmed
	}
	return result
}

// abbreviateJSecToken は英字トークンを「大文字+次の子音小文字」で短縮する。
func abbreviateJSecToken(token string) string {
	runes := []rune(token)
	builder := strings.Builder{}
	hasUpper := false
	for i, r := range runes {
		if unicode.IsUpper(r) {
			hasUpper = true
			builder.WriteRune(r)
			if consonant, ok := findNextLowerConsonant(runes, i+1); ok {
				builder.WriteRune(consonant)
			}
		}
		if unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	if hasUpper {
		return builder.String()
	}
	builder.Reset()

	firstAlphaIndex := -1
	for i, r := range runes {
		if unicode.IsLetter(r) {
			firstAlphaIndex = i
			builder.WriteRune(unicode.ToUpper(r))
			break
		}
	}
	if firstAlphaIndex >= 0 {
		if consonant, ok := findNextLowerConsonant(runes, firstAlphaIndex+1); ok {
			builder.WriteRune(consonant)
		}
	}
	for _, r := range runes {
		if unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

// findNextLowerConsonant は開始位置以降で最初の子音小文字を返す。
func findNextLowerConsonant(runes []rune, start int) (rune, bool) {
	for i := start; i < len(runes); i++ {
		if isAsciiLowerConsonant(runes[i]) {
			return runes[i], true
		}
	}
	return 0, false
}

// isAsciiLowerConsonant はASCII小文字子音かを判定する。
func isAsciiLowerConsonant(r rune) bool {
	if r < 'a' || r > 'z' {
		return false
	}
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return false
	default:
		return true
	}
}

// applyBoneEnglishNames はボーン英名を契約どおりに設定する。
func applyBoneEnglishNames(bones *model.BoneCollection) {
	if bones == nil {
		return
	}
	for index := 0; index < bones.Len(); index++ {
		bone, err := bones.Get(index)
		if err != nil || bone == nil {
			continue
		}
		if standardEnglishName, ok := resolveStandardBoneEnglishName(bone.Name()); ok {
			bone.EnglishName = standardEnglishName
			continue
		}
		bone.EnglishName = bone.Name()
	}
}

// resolveStandardBoneEnglishName は標準ボーン名から英名を解決する。
func resolveStandardBoneEnglishName(name string) (string, bool) {
	englishName, exists := standardBoneEnglishByName[name]
	return englishName, exists
}

// buildStandardBoneEnglishByName は標準ボーン名辞書を構築する。
func buildStandardBoneEnglishByName() map[string]string {
	out := map[string]string{}
	for standardBoneName, template := range standardBoneEnglishTemplates {
		if strings.Contains(standardBoneName.String(), model.BONE_DIRECTION_PREFIX) {
			leftName := standardBoneName.Left()
			rightName := standardBoneName.Right()
			out[leftName] = strings.ReplaceAll(template, "{Side}", "Left")
			out[rightName] = strings.ReplaceAll(template, "{Side}", "Right")
			continue
		}
		out[standardBoneName.String()] = template
	}
	out[leftToeHumanTargetName] = "LeftToe"
	out[rightToeHumanTargetName] = "RightToe"
	out[leftWristTipName] = "LeftWristTip"
	out[rightWristTipName] = "RightWristTip"
	out[leftThumbTipName] = "LeftThumbTip"
	out[rightThumbTipName] = "RightThumbTip"
	out[leftIndexTipName] = "LeftIndexTip"
	out[rightIndexTipName] = "RightIndexTip"
	out[leftMiddleTipName] = "LeftMiddleTip"
	out[rightMiddleTipName] = "RightMiddleTip"
	out[leftRingTipName] = "LeftRingTip"
	out[rightRingTipName] = "RightRingTip"
	out[leftPinkyTipName] = "LeftLittleTip"
	out[rightPinkyTipName] = "RightLittleTip"
	out["あご"] = "Jaw"
	out["J_Bip_C_Chest"] = "J_Bip_C_Chest"
	return out
}

// buildStandardBoneFlagOverrideByName は標準ボーン名フラグ固定辞書を構築する。
func buildStandardBoneFlagOverrideByName() map[string]model.BoneFlag {
	out := map[string]model.BoneFlag{}
	for standardBoneName, flag := range standardBoneFlagOverrides {
		if strings.Contains(standardBoneName.String(), model.BONE_DIRECTION_PREFIX) {
			out[standardBoneName.Left()] = flag
			out[standardBoneName.Right()] = flag
			continue
		}
		out[standardBoneName.String()] = flag
	}
	out[leftWristTipName] = model.BoneFlag(0x0002)
	out[rightWristTipName] = model.BoneFlag(0x0002)
	out[leftThumbTipName] = model.BoneFlag(0x0012)
	out[rightThumbTipName] = model.BoneFlag(0x0012)
	out[leftIndexTipName] = model.BoneFlag(0x0012)
	out[rightIndexTipName] = model.BoneFlag(0x0012)
	out[leftMiddleTipName] = model.BoneFlag(0x0012)
	out[rightMiddleTipName] = model.BoneFlag(0x0012)
	out[leftRingTipName] = model.BoneFlag(0x0012)
	out[rightRingTipName] = model.BoneFlag(0x0012)
	out[leftPinkyTipName] = model.BoneFlag(0x0012)
	out[rightPinkyTipName] = model.BoneFlag(0x0012)
	return out
}

// normalizeStandardBoneFlags は標準ボーンのフラグを契約に沿って正規化する。
func normalizeStandardBoneFlags(bones *model.BoneCollection) {
	if bones == nil {
		return
	}
	for index := 0; index < bones.Len(); index++ {
		bone, err := bones.Get(index)
		if err != nil || bone == nil {
			continue
		}
		if overrideFlag, exists := standardBoneFlagOverrideByName[bone.Name()]; exists {
			bone.BoneFlag = overrideFlag
			applyBoneFlagConsistency(bone)
			continue
		}
		if bone.Config() == nil {
			continue
		}
		if bone.BoneFlag&model.BONE_FLAG_TAIL_IS_BONE != 0 {
			bone.BoneFlag = model.BONE_FLAG_TAIL_IS_BONE |
				model.BONE_FLAG_CAN_ROTATE |
				model.BONE_FLAG_IS_VISIBLE |
				model.BONE_FLAG_CAN_MANIPULATE
		} else {
			bone.BoneFlag = model.BONE_FLAG_CAN_ROTATE |
				model.BONE_FLAG_IS_VISIBLE |
				model.BONE_FLAG_CAN_MANIPULATE
		}
		applyBoneFlagConsistency(bone)
	}
}

// applyViewerIdealDisplaySlots は viewer_ideal 契約に従って表示枠を再構築する。
func applyViewerIdealDisplaySlots(modelData *ModelData) {
	if modelData == nil || modelData.Bones == nil {
		return
	}
	slots := collection.NewNamedCollection[*model.DisplaySlot](len(viewerIdealFixedDisplaySlotSpecs) + 8)
	fixedSlotIndexes := map[string]int{}
	usedSlotNames := map[string]struct{}{}
	assignedBoneIndexes := map[int]struct{}{}

	for _, bone := range modelData.Bones.Values() {
		if bone == nil {
			continue
		}
		bone.DisplaySlotIndex = 0
	}

	if len(viewerIdealFixedDisplaySlotSpecs) == 0 {
		modelData.DisplaySlots = slots
		return
	}
	rootSpec := viewerIdealFixedDisplaySlotSpecs[0]
	rootSlot := newViewerIdealDisplaySlot(rootSpec.Name, rootSpec.EnglishName, model.SPECIAL_FLAG_ON)
	rootIndex := slots.AppendRaw(rootSlot)
	fixedSlotIndexes[rootSpec.Name] = rootIndex
	usedSlotNames[rootSpec.Name] = struct{}{}
	assignViewerIdealFixedBonesToSlot(modelData.Bones, rootSlot, rootSpec.BoneNames, assignedBoneIndexes)

	morphSlot := newViewerIdealDisplaySlot(viewerIdealDisplaySlotMorphName, "Exp", model.SPECIAL_FLAG_ON)
	morphIndex := slots.AppendRaw(morphSlot)
	fixedSlotIndexes[viewerIdealDisplaySlotMorphName] = morphIndex
	usedSlotNames[viewerIdealDisplaySlotMorphName] = struct{}{}
	assignViewerIdealMorphsToSlot(modelData, morphSlot)

	for i := 1; i < len(viewerIdealFixedDisplaySlotSpecs); i++ {
		spec := viewerIdealFixedDisplaySlotSpecs[i]
		slot := newViewerIdealDisplaySlot(spec.Name, spec.EnglishName, model.SPECIAL_FLAG_OFF)
		slotIndex := slots.AppendRaw(slot)
		fixedSlotIndexes[spec.Name] = slotIndex
		usedSlotNames[spec.Name] = struct{}{}
		assignViewerIdealFixedBonesToSlot(modelData.Bones, slot, spec.BoneNames, assignedBoneIndexes)
	}

	standardNameSet := buildViewerIdealStandardBoneNameSet()
	assignViewerIdealFallbackStandardSlots(modelData, slots, fixedSlotIndexes, standardNameSet, assignedBoneIndexes)

	unassignedIndexes := collectViewerIdealUnassignedBoneIndexes(modelData.Bones, assignedBoneIndexes)
	components := collectViewerIdealBoneComponents(modelData.Bones, unassignedIndexes)
	componentsByMaterial := map[int][][]int{}
	otherComponents := make([][]int, 0, len(components))

	for _, component := range components {
		if isViewerIdealHairComponent(modelData.Bones, component) {
			hairSlot, exists := getViewerIdealSlotByName(slots, viewerIdealDisplaySlotHairName)
			if !exists {
				continue
			}
			appendViewerIdealComponentToSlot(modelData.Bones, hairSlot, component, assignedBoneIndexes)
			continue
		}
		materialIndex, exists := resolveViewerIdealRepresentativeMaterialIndex(modelData, component)
		if !exists {
			otherComponents = append(otherComponents, component)
			continue
		}
		componentsByMaterial[materialIndex] = append(componentsByMaterial[materialIndex], component)
	}

	materialIndexes := make([]int, 0, len(componentsByMaterial))
	for materialIndex := range componentsByMaterial {
		materialIndexes = append(materialIndexes, materialIndex)
	}
	sort.Ints(materialIndexes)
	for _, materialIndex := range materialIndexes {
		slotBaseName := resolveViewerIdealMaterialDisplaySlotBaseName(modelData, materialIndex)
		slotName := buildUniqueViewerIdealDisplaySlotName(usedSlotNames, slotBaseName)
		slotEnglishName := resolveViewerIdealDisplaySlotEnglishName(slotName)
		slot := newViewerIdealDisplaySlot(slotName, slotEnglishName, model.SPECIAL_FLAG_OFF)
		slots.AppendRaw(slot)
		usedSlotNames[slotName] = struct{}{}
		for _, component := range componentsByMaterial[materialIndex] {
			appendViewerIdealComponentToSlot(modelData.Bones, slot, component, assignedBoneIndexes)
		}
	}

	if len(otherComponents) > 0 {
		otherSlot := ensureViewerIdealOtherSlot(slots, usedSlotNames)
		for _, component := range otherComponents {
			appendViewerIdealComponentToSlot(modelData.Bones, otherSlot, component, assignedBoneIndexes)
		}
	}

	remainingIndexes := collectViewerIdealUnassignedBoneIndexes(modelData.Bones, assignedBoneIndexes)
	if len(remainingIndexes) > 0 {
		otherSlot := ensureViewerIdealOtherSlot(slots, usedSlotNames)
		for _, boneIndex := range remainingIndexes {
			addViewerIdealBoneToSlotByIndex(modelData.Bones, otherSlot, boneIndex, assignedBoneIndexes)
		}
	}

	modelData.DisplaySlots = slots
}

// newViewerIdealDisplaySlot は表示枠を生成する。
func newViewerIdealDisplaySlot(name string, englishName string, specialFlag model.SpecialFlag) *model.DisplaySlot {
	slot := &model.DisplaySlot{
		EnglishName: englishName,
		SpecialFlag: specialFlag,
		References:  []model.Reference{},
	}
	slot.SetName(name)
	return slot
}

// assignViewerIdealMorphsToSlot はモーフを表情表示枠へ追加する。
func assignViewerIdealMorphsToSlot(modelData *ModelData, slot *model.DisplaySlot) {
	if modelData == nil || modelData.Morphs == nil || slot == nil {
		return
	}
	for _, morph := range modelData.Morphs.Values() {
		if morph == nil {
			continue
		}
		slot.References = append(slot.References, model.Reference{
			DisplayType:  model.DISPLAY_TYPE_MORPH,
			DisplayIndex: morph.Index(),
		})
	}
}

// assignViewerIdealFixedBonesToSlot は固定表示枠へ骨を追加する。
func assignViewerIdealFixedBonesToSlot(
	bones *model.BoneCollection,
	slot *model.DisplaySlot,
	boneNames []string,
	assignedBoneIndexes map[int]struct{},
) {
	if bones == nil || slot == nil {
		return
	}
	for _, boneName := range boneNames {
		addViewerIdealBoneToSlotByName(bones, slot, boneName, assignedBoneIndexes)
	}
}

// addViewerIdealBoneToSlotByName は名前指定で骨を表示枠へ追加する。
func addViewerIdealBoneToSlotByName(
	bones *model.BoneCollection,
	slot *model.DisplaySlot,
	boneName string,
	assignedBoneIndexes map[int]struct{},
) {
	if bones == nil || slot == nil {
		return
	}
	bone, exists := getBoneByName(bones, boneName)
	if !exists {
		return
	}
	addViewerIdealBoneToSlotByIndex(bones, slot, bone.Index(), assignedBoneIndexes)
}

// addViewerIdealBoneToSlotByIndex はindex指定で骨を表示枠へ追加する。
func addViewerIdealBoneToSlotByIndex(
	bones *model.BoneCollection,
	slot *model.DisplaySlot,
	boneIndex int,
	assignedBoneIndexes map[int]struct{},
) {
	if bones == nil || slot == nil || boneIndex < 0 {
		return
	}
	for _, reference := range slot.References {
		if reference.DisplayType == model.DISPLAY_TYPE_BONE && reference.DisplayIndex == boneIndex {
			if assignedBoneIndexes != nil {
				assignedBoneIndexes[boneIndex] = struct{}{}
			}
			if bone, err := bones.Get(boneIndex); err == nil && bone != nil {
				bone.DisplaySlotIndex = slot.Index()
			}
			return
		}
	}
	slot.References = append(slot.References, model.Reference{
		DisplayType:  model.DISPLAY_TYPE_BONE,
		DisplayIndex: boneIndex,
	})
	if assignedBoneIndexes != nil {
		assignedBoneIndexes[boneIndex] = struct{}{}
	}
	if bone, err := bones.Get(boneIndex); err == nil && bone != nil {
		bone.DisplaySlotIndex = slot.Index()
	}
}

// buildViewerIdealStandardBoneNameSet は標準骨格判定用の名称集合を生成する。
func buildViewerIdealStandardBoneNameSet() map[string]struct{} {
	out := map[string]struct{}{}
	for standardName := range model.GetStandardBoneConfigs() {
		baseName := standardName.String()
		if strings.Contains(baseName, model.BONE_DIRECTION_PREFIX) {
			out[standardName.Left()] = struct{}{}
			out[standardName.Right()] = struct{}{}
			continue
		}
		out[baseName] = struct{}{}
	}
	for _, aliasName := range []string{
		leftWristTipName,
		rightWristTipName,
		leftThumbTipName,
		rightThumbTipName,
		leftIndexTipName,
		rightIndexTipName,
		leftMiddleTipName,
		rightMiddleTipName,
		leftRingTipName,
		rightRingTipName,
		leftPinkyTipName,
		rightPinkyTipName,
		leftToeHumanTargetName,
		rightToeHumanTargetName,
		"両目光",
		"左目光",
		"右目光",
		tongueBone1Name,
		tongueBone2Name,
		tongueBone3Name,
		tongueBone4Name,
		"左胸",
		"左胸先",
		"右胸",
		"右胸先",
		"あご",
		"J_Bip_C_Chest",
	} {
		out[aliasName] = struct{}{}
	}
	return out
}

// assignViewerIdealFallbackStandardSlots は未割当の標準骨を固定枠へ割当する。
func assignViewerIdealFallbackStandardSlots(
	modelData *ModelData,
	slots *collection.NamedCollection[*model.DisplaySlot],
	fixedSlotIndexes map[string]int,
	standardNameSet map[string]struct{},
	assignedBoneIndexes map[int]struct{},
) {
	if modelData == nil || modelData.Bones == nil || slots == nil {
		return
	}
	for _, bone := range modelData.Bones.Values() {
		if bone == nil {
			continue
		}
		if _, exists := assignedBoneIndexes[bone.Index()]; exists {
			continue
		}
		if _, exists := standardNameSet[bone.Name()]; !exists {
			continue
		}
		slotName, slotExists := resolveViewerIdealFallbackSlotName(bone.Name())
		if !slotExists {
			continue
		}
		slotIndex, hasSlot := fixedSlotIndexes[slotName]
		if !hasSlot {
			continue
		}
		slot, err := slots.Get(slotIndex)
		if err != nil || slot == nil {
			continue
		}
		addViewerIdealBoneToSlotByIndex(modelData.Bones, slot, bone.Index(), assignedBoneIndexes)
	}
}

// resolveViewerIdealFallbackSlotName は標準骨名から固定表示枠名を推定する。
func resolveViewerIdealFallbackSlotName(boneName string) (string, bool) {
	if boneName == "" {
		return "", false
	}
	if strings.HasPrefix(boneName, "左") {
		if containsAnySubstring(boneName, []string{"親指", "人指", "中指", "薬指", "小指"}) {
			return viewerIdealDisplaySlotLeftFgrName, true
		}
		if containsAnySubstring(boneName, []string{"肩", "腕", "ひじ", "手首", "手捩"}) {
			return viewerIdealDisplaySlotLeftArmName, true
		}
		if containsAnySubstring(boneName, []string{"足", "ひざ", "つま先", "かかと", "ＩＫ", "IK"}) {
			return viewerIdealDisplaySlotLeftLegName, true
		}
		if strings.Contains(boneName, "目") {
			return viewerIdealDisplaySlotFaceName, true
		}
		if strings.Contains(boneName, "胸") {
			return viewerIdealDisplaySlotBustName, true
		}
	}
	if strings.HasPrefix(boneName, "右") {
		if containsAnySubstring(boneName, []string{"親指", "人指", "中指", "薬指", "小指"}) {
			return viewerIdealDisplaySlotRightFgrName, true
		}
		if containsAnySubstring(boneName, []string{"肩", "腕", "ひじ", "手首", "手捩"}) {
			return viewerIdealDisplaySlotRightArmName, true
		}
		if containsAnySubstring(boneName, []string{"足", "ひざ", "つま先", "かかと", "ＩＫ", "IK"}) {
			return viewerIdealDisplaySlotRightLegName, true
		}
		if strings.Contains(boneName, "目") {
			return viewerIdealDisplaySlotFaceName, true
		}
		if strings.Contains(boneName, "胸") {
			return viewerIdealDisplaySlotBustName, true
		}
	}
	if containsAnySubstring(boneName, []string{"目", "舌", "あご"}) {
		return viewerIdealDisplaySlotFaceName, true
	}
	if strings.Contains(boneName, "胸") {
		return viewerIdealDisplaySlotBustName, true
	}
	if containsAnySubstring(boneName, []string{"腰", "下半身", "上半身", "首", "頭", "体幹"}) {
		return viewerIdealDisplaySlotTrunkName, true
	}
	if boneName == model.CENTER.String() || boneName == model.GROOVE.String() {
		return viewerIdealDisplaySlotCenterName, true
	}
	if boneName == model.ROOT.String() {
		return viewerIdealDisplaySlotRootName, true
	}
	return "", false
}

// containsAnySubstring は候補部分文字列のいずれかを含むか判定する。
func containsAnySubstring(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

// collectViewerIdealUnassignedBoneIndexes は未割当ボーンindex一覧を返す。
func collectViewerIdealUnassignedBoneIndexes(
	bones *model.BoneCollection,
	assignedBoneIndexes map[int]struct{},
) []int {
	if bones == nil {
		return []int{}
	}
	indexes := make([]int, 0, bones.Len())
	for _, bone := range bones.Values() {
		if bone == nil {
			continue
		}
		if _, exists := assignedBoneIndexes[bone.Index()]; exists {
			continue
		}
		indexes = append(indexes, bone.Index())
	}
	sort.Ints(indexes)
	return indexes
}

// collectViewerIdealBoneComponents は候補ボーンの連結成分を返す。
func collectViewerIdealBoneComponents(bones *model.BoneCollection, candidateIndexes []int) [][]int {
	if bones == nil || len(candidateIndexes) == 0 {
		return [][]int{}
	}
	candidateSet := map[int]struct{}{}
	for _, boneIndex := range candidateIndexes {
		candidateSet[boneIndex] = struct{}{}
	}
	adjacency := map[int][]int{}
	for _, boneIndex := range candidateIndexes {
		bone, err := bones.Get(boneIndex)
		if err != nil || bone == nil {
			continue
		}
		if bone.ParentIndex >= 0 {
			if _, exists := candidateSet[bone.ParentIndex]; exists {
				adjacency[boneIndex] = append(adjacency[boneIndex], bone.ParentIndex)
				adjacency[bone.ParentIndex] = append(adjacency[bone.ParentIndex], boneIndex)
			}
		}
	}

	visited := map[int]struct{}{}
	components := make([][]int, 0, len(candidateIndexes))
	for _, startIndex := range candidateIndexes {
		if _, seen := visited[startIndex]; seen {
			continue
		}
		stack := []int{startIndex}
		component := make([]int, 0, 8)
		visited[startIndex] = struct{}{}
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			component = append(component, current)
			neighbors := adjacency[current]
			sort.Ints(neighbors)
			for _, neighbor := range neighbors {
				if _, seen := visited[neighbor]; seen {
					continue
				}
				visited[neighbor] = struct{}{}
				stack = append(stack, neighbor)
			}
		}
		sort.Ints(component)
		components = append(components, component)
	}
	sort.Slice(components, func(i int, j int) bool {
		if len(components[i]) == 0 || len(components[j]) == 0 {
			return len(components[i]) < len(components[j])
		}
		return components[i][0] < components[j][0]
	})
	return components
}

// isViewerIdealHairComponent は根本親が頭の連結成分かを判定する。
func isViewerIdealHairComponent(bones *model.BoneCollection, component []int) bool {
	if bones == nil || len(component) == 0 {
		return false
	}
	componentSet := map[int]struct{}{}
	for _, boneIndex := range component {
		componentSet[boneIndex] = struct{}{}
	}
	roots := make([]int, 0, len(component))
	for _, boneIndex := range component {
		bone, err := bones.Get(boneIndex)
		if err != nil || bone == nil {
			continue
		}
		if bone.ParentIndex >= 0 {
			if _, exists := componentSet[bone.ParentIndex]; exists {
				continue
			}
		}
		roots = append(roots, boneIndex)
	}
	sort.Ints(roots)
	for _, rootIndex := range roots {
		rootBone, rootErr := bones.Get(rootIndex)
		if rootErr != nil || rootBone == nil || rootBone.ParentIndex < 0 {
			continue
		}
		parentBone, parentErr := bones.Get(rootBone.ParentIndex)
		if parentErr != nil || parentBone == nil {
			continue
		}
		if parentBone.Name() == model.HEAD.String() {
			return true
		}
	}
	return false
}

// appendViewerIdealComponentToSlot は成分内のボーンを親子順で表示枠へ追加する。
func appendViewerIdealComponentToSlot(
	bones *model.BoneCollection,
	slot *model.DisplaySlot,
	component []int,
	assignedBoneIndexes map[int]struct{},
) {
	if bones == nil || slot == nil || len(component) == 0 {
		return
	}
	orderedIndexes := orderViewerIdealComponentBoneIndexes(bones, component)
	for _, boneIndex := range orderedIndexes {
		addViewerIdealBoneToSlotByIndex(bones, slot, boneIndex, assignedBoneIndexes)
	}
}

// orderViewerIdealComponentBoneIndexes は連結成分を親→子の順へ並べる。
func orderViewerIdealComponentBoneIndexes(bones *model.BoneCollection, component []int) []int {
	if bones == nil || len(component) == 0 {
		return []int{}
	}
	componentSet := map[int]struct{}{}
	for _, boneIndex := range component {
		componentSet[boneIndex] = struct{}{}
	}
	childrenByParent := map[int][]int{}
	roots := make([]int, 0, len(component))
	for _, boneIndex := range component {
		bone, err := bones.Get(boneIndex)
		if err != nil || bone == nil {
			continue
		}
		if bone.ParentIndex >= 0 {
			if _, exists := componentSet[bone.ParentIndex]; exists {
				childrenByParent[bone.ParentIndex] = append(childrenByParent[bone.ParentIndex], boneIndex)
				continue
			}
		}
		roots = append(roots, boneIndex)
	}
	sort.Ints(roots)
	for parentIndex := range childrenByParent {
		sort.Ints(childrenByParent[parentIndex])
	}

	ordered := make([]int, 0, len(component))
	visited := map[int]struct{}{}
	var walk func(int)
	walk = func(current int) {
		if _, exists := visited[current]; exists {
			return
		}
		visited[current] = struct{}{}
		ordered = append(ordered, current)
		for _, childIndex := range childrenByParent[current] {
			walk(childIndex)
		}
	}
	for _, rootIndex := range roots {
		walk(rootIndex)
	}
	for _, boneIndex := range component {
		if _, exists := visited[boneIndex]; exists {
			continue
		}
		walk(boneIndex)
	}
	return ordered
}

// resolveViewerIdealRepresentativeMaterialIndex は成分代表材質indexを返す。
func resolveViewerIdealRepresentativeMaterialIndex(modelData *ModelData, component []int) (int, bool) {
	if modelData == nil || modelData.Materials == nil || modelData.Vertices == nil || len(component) == 0 {
		return -1, false
	}
	componentSet := map[int]struct{}{}
	for _, boneIndex := range component {
		componentSet[boneIndex] = struct{}{}
	}
	materialScores := map[int]float64{}
	for _, vertex := range modelData.Vertices.Values() {
		if vertex == nil || vertex.Deform == nil || len(vertex.MaterialIndexes) == 0 {
			continue
		}
		joints := vertex.Deform.Indexes()
		weights := vertex.Deform.Weights()
		maxCount := len(joints)
		if len(weights) < maxCount {
			maxCount = len(weights)
		}
		if maxCount <= 0 {
			continue
		}
		componentWeight := 0.0
		for i := 0; i < maxCount; i++ {
			if weights[i] <= 0 {
				continue
			}
			if _, exists := componentSet[joints[i]]; !exists {
				continue
			}
			componentWeight += weights[i]
		}
		if componentWeight <= 0 {
			continue
		}
		materialSet := map[int]struct{}{}
		for _, materialIndex := range vertex.MaterialIndexes {
			if materialIndex < 0 || materialIndex >= modelData.Materials.Len() {
				continue
			}
			materialSet[materialIndex] = struct{}{}
		}
		if len(materialSet) == 0 {
			continue
		}
		contribution := componentWeight / float64(len(materialSet))
		for materialIndex := range materialSet {
			materialScores[materialIndex] += contribution
		}
	}
	if len(materialScores) == 0 {
		return -1, false
	}
	bestIndex := -1
	bestScore := -1.0
	for materialIndex, score := range materialScores {
		if bestIndex < 0 || score > bestScore || (score == bestScore && materialIndex < bestIndex) {
			bestIndex = materialIndex
			bestScore = score
		}
	}
	if bestIndex < 0 {
		return -1, false
	}
	return bestIndex, true
}

// resolveViewerIdealMaterialDisplaySlotBaseName は材質由来表示枠の基底名を返す。
func resolveViewerIdealMaterialDisplaySlotBaseName(modelData *ModelData, materialIndex int) string {
	if modelData == nil || modelData.Materials == nil || materialIndex < 0 || materialIndex >= modelData.Materials.Len() {
		return fmt.Sprintf("material_%d", materialIndex)
	}
	materialData, err := modelData.Materials.Get(materialIndex)
	if err != nil || materialData == nil {
		return fmt.Sprintf("material_%d", materialIndex)
	}
	baseName := abbreviateMaterialName(materialData.Name())
	if strings.TrimSpace(baseName) == "" {
		return fmt.Sprintf("material_%d", materialIndex)
	}
	return baseName
}

// buildUniqueViewerIdealDisplaySlotName は重複しない表示枠名を生成する。
func buildUniqueViewerIdealDisplaySlotName(usedSlotNames map[string]struct{}, baseName string) string {
	base := strings.TrimSpace(baseName)
	if base == "" {
		base = "slot"
	}
	candidate := base
	serial := 2
	for {
		if _, exists := usedSlotNames[candidate]; !exists {
			return candidate
		}
		candidate = fmt.Sprintf("%s_%d", base, serial)
		serial++
	}
}

// resolveViewerIdealDisplaySlotEnglishName は表示枠名に対応する英名を返す。
func resolveViewerIdealDisplaySlotEnglishName(slotName string) string {
	for _, spec := range viewerIdealFixedDisplaySlotSpecs {
		if spec.Name == slotName {
			return spec.EnglishName
		}
	}
	switch slotName {
	case viewerIdealDisplaySlotMorphName:
		return "Exp"
	case viewerIdealDisplaySlotOtherName:
		return "Other"
	default:
		return slotName
	}
}

// ensureViewerIdealOtherSlot はその他表示枠を取得または生成する。
func ensureViewerIdealOtherSlot(
	slots *collection.NamedCollection[*model.DisplaySlot],
	usedSlotNames map[string]struct{},
) *model.DisplaySlot {
	if existing, exists := getViewerIdealSlotByName(slots, viewerIdealDisplaySlotOtherName); exists {
		return existing
	}
	slotName := buildUniqueViewerIdealDisplaySlotName(usedSlotNames, viewerIdealDisplaySlotOtherName)
	slot := newViewerIdealDisplaySlot(
		slotName,
		resolveViewerIdealDisplaySlotEnglishName(slotName),
		model.SPECIAL_FLAG_OFF,
	)
	slots.AppendRaw(slot)
	usedSlotNames[slotName] = struct{}{}
	return slot
}

// getViewerIdealSlotByName は表示枠を名前で取得する。
func getViewerIdealSlotByName(
	slots *collection.NamedCollection[*model.DisplaySlot],
	name string,
) (*model.DisplaySlot, bool) {
	if slots == nil {
		return nil, false
	}
	slot, err := slots.GetByName(name)
	if err != nil || slot == nil {
		return nil, false
	}
	return slot, true
}

// applyBoneFlagConsistency はtail/付与/IK/軸の整合フラグを補正する。
func applyBoneFlagConsistency(bone *model.Bone) {
	if bone == nil {
		return
	}
	if bone.TailIndex >= 0 {
		bone.BoneFlag |= model.BONE_FLAG_TAIL_IS_BONE
	} else {
		bone.BoneFlag &^= model.BONE_FLAG_TAIL_IS_BONE
	}
	if bone.EffectIndex >= 0 && absSignValue(bone.EffectFactor) > weightSignEpsilon {
		bone.BoneFlag |= model.BONE_FLAG_IS_EXTERNAL_ROTATION
	} else {
		bone.EffectIndex = -1
		bone.EffectFactor = 0
		bone.BoneFlag &^= model.BONE_FLAG_IS_EXTERNAL_ROTATION
		bone.BoneFlag &^= model.BONE_FLAG_IS_EXTERNAL_TRANSLATION
	}
	if bone.Ik != nil {
		bone.BoneFlag |= model.BONE_FLAG_IS_IK
	} else {
		bone.BoneFlag &^= model.BONE_FLAG_IS_IK
	}
	if bone.FixedAxis.Length() > 1e-8 {
		bone.BoneFlag |= model.BONE_FLAG_HAS_FIXED_AXIS
	} else {
		bone.BoneFlag &^= model.BONE_FLAG_HAS_FIXED_AXIS
	}
	if bone.LocalAxisX.Length() > 1e-8 && bone.LocalAxisZ.Length() > 1e-8 {
		bone.BoneFlag |= model.BONE_FLAG_HAS_LOCAL_AXIS
	} else {
		bone.BoneFlag &^= model.BONE_FLAG_HAS_LOCAL_AXIS
	}
}

// getBoneByName はボーン名を取得して可否を返す。
func getBoneByName(bones *model.BoneCollection, name string) (*model.Bone, bool) {
	if bones == nil {
		return nil, false
	}
	bone, err := bones.GetByName(name)
	if err != nil || bone == nil {
		return nil, false
	}
	return bone, true
}

// meanPosition は2点の中点を返す。
func meanPosition(a mmath.Vec3, b mmath.Vec3) mmath.Vec3 {
	return mmath.Vec3{Vec: r3.Vec{
		X: (a.X + b.X) * 0.5,
		Y: (a.Y + b.Y) * 0.5,
		Z: (a.Z + b.Z) * 0.5,
	}}
}
