// 指示: miu200521358
package minteractor

import (
	"fmt"
	"sort"
	"strings"

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
)

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
	normalizeMappedRootParents(modelData.Bones)
	return nil
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
