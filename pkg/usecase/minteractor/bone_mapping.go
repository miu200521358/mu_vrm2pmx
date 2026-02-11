// 指示: miu200521358
package minteractor

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
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
)

// explicitRemoveBoneNames は明示削除対象ボーン名を保持する。
var explicitRemoveBoneNames = map[string]struct{}{
	"face":      {},
	"body":      {},
	"hair":      {},
	"secondary": {},
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
	normalizeMappedRootParents(modelData.Bones)
	if err := removeExplicitBonesAndReindex(modelData); err != nil {
		return err
	}
	if err := normalizeBoneNamesAndEnglish(modelData.Bones); err != nil {
		return err
	}
	normalizeStandardBoneFlags(modelData.Bones)
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
	return plan
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
		if err := ensureToeTailBone(modelData, targetBoneIndexes, direction); err != nil {
			return err
		}
		if err := ensureHeelBone(modelData, targetBoneIndexes, direction); err != nil {
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
		unit := mmath.DegToRad(114.5916)
		bone.Ik = &model.Ik{
			BoneIndex:    ankle.Index(),
			LoopCount:    40,
			UnitRotation: mmath.Vec3{Vec: r3.Vec{X: unit, Y: unit, Z: unit}},
			Links:        ikLinks,
		}
	}
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
	if _, exists := getBoneByTargetName(modelData, targetBoneIndexes, targetName); exists {
		return nil
	}

	legIK, legIKOK := getBoneByTargetName(modelData, targetBoneIndexes, model.LEG_IK.StringFromDirection(direction))
	if !legIKOK {
		return nil
	}
	toeTarget, toeTargetOK := getBoneByTargetName(modelData, targetBoneIndexes, model.TOE_T.StringFromDirection(direction))
	if !toeTargetOK {
		if toeEx, toeExOK := getBoneByTargetName(modelData, targetBoneIndexes, model.TOE_EX.StringFromDirection(direction)); toeExOK {
			toeTarget = toeEx
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

	bone := model.NewBoneByName(targetName)
	bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE |
		model.BONE_FLAG_CAN_TRANSLATE | model.BONE_FLAG_IS_IK
	bone.Position = toeTarget.Position
	bone.ParentIndex = legIK.Index()
	ikLinks := make([]model.IkLink, 0, 1)
	if ankle, ok := getBoneByTargetName(modelData, targetBoneIndexes, model.ANKLE.StringFromDirection(direction)); ok {
		ikLinks = append(ikLinks, model.IkLink{
			BoneIndex: ankle.Index(),
		})
	}
	if len(ikLinks) > 0 {
		unit := mmath.DegToRad(229.1831)
		bone.Ik = &model.Ik{
			BoneIndex:    toeTarget.Index(),
			LoopCount:    3,
			UnitRotation: mmath.Vec3{Vec: r3.Vec{X: unit, Y: unit, Z: unit}},
			Links:        ikLinks,
		}
	}
	if err := insertSupplementBone(modelData, targetBoneIndexes, targetName, bone); err != nil {
		return err
	}
	if toeIK, ok := getBoneByTargetName(modelData, targetBoneIndexes, targetName); ok {
		legIK.BoneFlag |= model.BONE_FLAG_TAIL_IS_BONE
		legIK.TailIndex = toeIK.Index()
	}
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
	_, exists := explicitRemoveBoneNames[strings.ToLower(strings.TrimSpace(name))]
	return exists
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
			if bone.BoneFlag&model.BONE_FLAG_TAIL_IS_BONE != 0 {
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
	applyBoneEnglishNames(bones)
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
	return out
}

// normalizeStandardBoneFlags は標準ボーンのフラグを契約に沿って正規化する。
func normalizeStandardBoneFlags(bones *model.BoneCollection) {
	if bones == nil {
		return
	}
	for index := 0; index < bones.Len(); index++ {
		bone, err := bones.Get(index)
		if err != nil || bone == nil || bone.Config() == nil {
			continue
		}
		if overrideFlag, exists := standardBoneFlagOverrideByName[bone.Name()]; exists {
			bone.BoneFlag = overrideFlag
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
