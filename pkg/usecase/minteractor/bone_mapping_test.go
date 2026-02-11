// 指示: miu200521358
package minteractor

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/adapter/io_model/pmx"
	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
	vrmrepository "github.com/miu200521358/mu_vrm2pmx/pkg/adapter/io_model/vrm"
	"gonum.org/v1/gonum/spatial/r3"
)

func TestApplyHumanoidBoneMappingAfterReorderAddsSupplementAndRenames(t *testing.T) {
	modelData := newBoneMappingTargetModel()

	if err := applyHumanoidBoneMappingAfterReorder(modelData); err != nil {
		t.Fatalf("mapping failed: %v", err)
	}

	for _, name := range []string{
		model.ROOT.String(),
		model.CENTER.String(),
		model.GROOVE.String(),
		model.WAIST.String(),
		model.LOWER.String(),
		model.UPPER.String(),
		model.UPPER2.String(),
		model.NECK.String(),
		model.HEAD.String(),
		model.EYE.Left(),
		model.EYE.Right(),
		model.EYES.String(),
		model.SHOULDER.Left(),
		model.SHOULDER.Right(),
		model.SHOULDER_P.Left(),
		model.SHOULDER_P.Right(),
		model.SHOULDER_C.Left(),
		model.SHOULDER_C.Right(),
		model.LEG.Left(),
		model.LEG.Right(),
		model.KNEE.Left(),
		model.KNEE.Right(),
		model.ANKLE.Left(),
		model.ANKLE.Right(),
		model.LEG_D.Left(),
		model.LEG_D.Right(),
		model.KNEE_D.Left(),
		model.KNEE_D.Right(),
		model.ANKLE_D.Left(),
		model.ANKLE_D.Right(),
		model.TOE_EX.Left(),
		model.TOE_EX.Right(),
		model.LEG_IK_PARENT.Left(),
		model.LEG_IK_PARENT.Right(),
		model.LEG_IK.Left(),
		model.LEG_IK.Right(),
		model.TOE_IK.Left(),
		model.TOE_IK.Right(),
		model.WAIST_CANCEL.Left(),
		model.WAIST_CANCEL.Right(),
		leftToeHumanTargetName,
		rightToeHumanTargetName,
		model.ARM_TWIST.Left(),
		model.ARM_TWIST.Right(),
		model.ARM_TWIST1.Left(),
		model.ARM_TWIST1.Right(),
		model.ARM_TWIST2.Left(),
		model.ARM_TWIST2.Right(),
		model.ARM_TWIST3.Left(),
		model.ARM_TWIST3.Right(),
		model.WRIST_TWIST.Left(),
		model.WRIST_TWIST.Right(),
		model.WRIST_TWIST1.Left(),
		model.WRIST_TWIST1.Right(),
		model.WRIST_TWIST2.Left(),
		model.WRIST_TWIST2.Right(),
		model.WRIST_TWIST3.Left(),
		model.WRIST_TWIST3.Right(),
		model.WRIST.Left(),
		model.WRIST.Right(),
		leftWristTipName,
		rightWristTipName,
		model.THUMB0.Left(),
		model.THUMB0.Right(),
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
	} {
		if _, err := modelData.Bones.GetByName(name); err != nil {
			t.Fatalf("expected bone %s to exist: %v", name, err)
		}
	}
	for _, name := range []string{
		model.WRIST_TAIL.Left(),
		model.WRIST_TAIL.Right(),
		model.TOE_T.Left(),
		model.TOE_T.Right(),
		model.HEEL.Left(),
		model.HEEL.Right(),
	} {
		if _, err := modelData.Bones.GetByName(name); err == nil {
			t.Fatalf("expected bone %s to be removed/normalized", name)
		}
	}

	if _, err := modelData.Bones.GetByName("leftUpperLeg"); err == nil {
		t.Fatalf("expected leftUpperLeg to be renamed")
	}
	for _, name := range []string{"Face", "Body", "Hair", "secondary"} {
		if _, err := modelData.Bones.GetByName(name); err == nil {
			t.Fatalf("expected %s to be removed", name)
		}
	}
	if _, err := modelData.Bones.GetByName("J_Sec_R_SkirtBack0_01"); err == nil {
		t.Fatalf("expected J_Sec bone to be renamed")
	}
	if _, err := modelData.Bones.GetByName("RSkBc0_01_2"); err != nil {
		t.Fatalf("expected renamed J_Sec bone RSkBc0_01_2: %v", err)
	}

	leftToe, _ := modelData.Bones.GetByName(leftToeHumanTargetName)
	leftAnkle, _ := modelData.Bones.GetByName(model.ANKLE.Left())
	if leftToe.ParentIndex != leftAnkle.Index() {
		t.Fatalf("expected 左つま先 parent to be 左足首: got=%d want=%d", leftToe.ParentIndex, leftAnkle.Index())
	}
	center, _ := modelData.Bones.GetByName(model.CENTER.String())
	root, _ := modelData.Bones.GetByName(model.ROOT.String())
	waist, _ := modelData.Bones.GetByName(model.WAIST.String())
	lower, _ := modelData.Bones.GetByName(model.LOWER.String())
	upper, _ := modelData.Bones.GetByName(model.UPPER.String())
	if root.Index() != 0 || center.Index() != 1 {
		t.Fatalf("expected root/center ordered at top: root=%d center=%d", root.Index(), center.Index())
	}
	if lower.ParentIndex != waist.Index() {
		t.Fatalf("expected 下半身 parent to be 腰: got=%d want=%d", lower.ParentIndex, waist.Index())
	}
	if upper.ParentIndex != waist.Index() {
		t.Fatalf("expected 上半身 parent to be 腰: got=%d want=%d", upper.ParentIndex, waist.Index())
	}
	if center.Position.X != 0.0 || center.Position.Z != 0.0 {
		t.Fatalf("expected センター XZ to be 0: got=(%f,%f)", center.Position.X, center.Position.Z)
	}
	if center.Position.Y != 5.0 {
		t.Fatalf("expected センター Y to be 下半身Yの半分(5.0): got=%f", center.Position.Y)
	}
	if center.EnglishName != "Center" {
		t.Fatalf("expected センター英名 Center: got=%s", center.EnglishName)
	}
	if center.BoneFlag != model.BoneFlag(0x001E) {
		t.Fatalf("expected センターBoneFlag 0x001E: got=0x%04X", int(center.BoneFlag))
	}
	groove, _ := modelData.Bones.GetByName(model.GROOVE.String())
	if groove.Position.X != 0.0 || groove.Position.Z != 0.0 {
		t.Fatalf("expected グルーブ XZ to be 0: got=(%f,%f)", groove.Position.X, groove.Position.Z)
	}
	if groove.Position.Y != 7.0 {
		t.Fatalf("expected グルーブ Y to be 下半身Yの7割(7.0): got=%f", groove.Position.Y)
	}

	leftWaistCancel, _ := modelData.Bones.GetByName(model.WAIST_CANCEL.Left())
	if leftWaistCancel.EffectIndex != waist.Index() {
		t.Fatalf("expected 左腰キャンセル effect parent to be 腰: got=%d want=%d", leftWaistCancel.EffectIndex, waist.Index())
	}
	if leftWaistCancel.EffectFactor != -1.0 {
		t.Fatalf("expected 左腰キャンセル effect factor -1.0: got=%f", leftWaistCancel.EffectFactor)
	}

	leftShoulderP, _ := modelData.Bones.GetByName(model.SHOULDER_P.Left())
	leftShoulderC, _ := modelData.Bones.GetByName(model.SHOULDER_C.Left())
	if leftShoulderC.EffectIndex != leftShoulderP.Index() {
		t.Fatalf("expected 左肩C effect parent to be 左肩P: got=%d want=%d", leftShoulderC.EffectIndex, leftShoulderP.Index())
	}
	if leftShoulderC.EffectFactor != -1.0 {
		t.Fatalf("expected 左肩C effect factor -1.0: got=%f", leftShoulderC.EffectFactor)
	}

	leftArmTwist, _ := modelData.Bones.GetByName(model.ARM_TWIST.Left())
	leftArmTwist1, _ := modelData.Bones.GetByName(model.ARM_TWIST1.Left())
	if leftArmTwist1.EffectIndex != leftArmTwist.Index() {
		t.Fatalf("expected 左腕捩1 effect parent to be 左腕捩: got=%d want=%d", leftArmTwist1.EffectIndex, leftArmTwist.Index())
	}
	if leftArmTwist1.EffectFactor != 0.25 {
		t.Fatalf("expected 左腕捩1 effect factor 0.25: got=%f", leftArmTwist1.EffectFactor)
	}
	if leftArmTwist1.BoneFlag != model.BoneFlag(0x0100) {
		t.Fatalf("expected 左腕捩1BoneFlag 0x0100: got=0x%04X", int(leftArmTwist1.BoneFlag))
	}

	leftWristTwist, _ := modelData.Bones.GetByName(model.WRIST_TWIST.Left())
	leftWristTwist3, _ := modelData.Bones.GetByName(model.WRIST_TWIST3.Left())
	if leftWristTwist3.EffectIndex != leftWristTwist.Index() {
		t.Fatalf("expected 左手捩3 effect parent to be 左手捩: got=%d want=%d", leftWristTwist3.EffectIndex, leftWristTwist.Index())
	}
	if leftWristTwist3.EffectFactor != 0.75 {
		t.Fatalf("expected 左手捩3 effect factor 0.75: got=%f", leftWristTwist3.EffectFactor)
	}

	leftLeg, _ := modelData.Bones.GetByName(model.LEG.Left())
	leftLegD, _ := modelData.Bones.GetByName(model.LEG_D.Left())
	if leftLegD.EffectIndex != leftLeg.Index() {
		t.Fatalf("expected 左足D effect parent to be 左足: got=%d want=%d", leftLegD.EffectIndex, leftLeg.Index())
	}
	if leftLegD.EffectFactor != 1.0 {
		t.Fatalf("expected 左足D effect factor 1.0: got=%f", leftLegD.EffectFactor)
	}
	if leftLegD.BoneFlag != model.BoneFlag(0x011A) {
		t.Fatalf("expected 左足DBoneFlag 0x011A: got=0x%04X", int(leftLegD.BoneFlag))
	}
	if leftLeg.Layer != 0 || leftLegD.Layer != 1 {
		t.Fatalf("expected 左足 layer=0 and 左足D layer=1: leg=%d legD=%d", leftLeg.Layer, leftLegD.Layer)
	}

	leftKnee, _ := modelData.Bones.GetByName(model.KNEE.Left())
	leftKneeD, _ := modelData.Bones.GetByName(model.KNEE_D.Left())
	if leftKnee.Layer != 0 || leftKneeD.Layer != 1 {
		t.Fatalf("expected 左ひざ layer=0 and 左ひざD layer=1: knee=%d kneeD=%d", leftKnee.Layer, leftKneeD.Layer)
	}
	leftKneeBase := mmath.Vec3{Vec: r3.Vec{X: 0.8, Y: 5.5, Z: 0.0}}
	leftAnkleBase := mmath.Vec3{Vec: r3.Vec{X: 0.8, Y: 2.0, Z: 0.3}}
	leftKneeOffset := leftKneeBase.Distance(leftAnkleBase) * 0.01
	if math.Abs(leftKnee.Position.Z-(-leftKneeOffset)) > 1e-6 {
		t.Fatalf("expected 左ひざ Z offset -%.6f: got=%f", leftKneeOffset, leftKnee.Position.Z)
	}
	if math.Abs(leftKneeD.Position.Z-(-leftKneeOffset)) > 1e-6 {
		t.Fatalf("expected 左ひざD Z offset -%.6f: got=%f", leftKneeOffset, leftKneeD.Position.Z)
	}

	leftAnkleD, _ := modelData.Bones.GetByName(model.ANKLE_D.Left())
	if leftAnkle.Layer != 0 || leftAnkleD.Layer != 1 {
		t.Fatalf("expected 左足首 layer=0 and 左足首D layer=1: ankle=%d ankleD=%d", leftAnkle.Layer, leftAnkleD.Layer)
	}
	leftToeEx, _ := modelData.Bones.GetByName(model.TOE_EX.Left())
	if leftToeEx.ParentIndex != leftAnkleD.Index() {
		t.Fatalf("expected 左足先EX parent to be 左足首D: got=%d want=%d", leftToeEx.ParentIndex, leftAnkleD.Index())
	}

	leftLegIKParent, _ := modelData.Bones.GetByName(model.LEG_IK_PARENT.Left())
	leftLegIK, _ := modelData.Bones.GetByName(model.LEG_IK.Left())
	if leftLegIK.ParentIndex != leftLegIKParent.Index() {
		t.Fatalf("expected 左足ＩＫ parent to be 左足IK親: got=%d want=%d", leftLegIK.ParentIndex, leftLegIKParent.Index())
	}
	if leftLegIK.BoneFlag != model.BoneFlag(0x003F) {
		t.Fatalf("expected 左足ＩＫBoneFlag 0x003F: got=0x%04X", int(leftLegIK.BoneFlag))
	}
	if leftLegIK.Ik == nil {
		t.Fatalf("expected 左足ＩＫ IK setting")
	}
	if leftLegIK.Ik != nil && leftLegIK.Ik.BoneIndex != leftAnkle.Index() {
		t.Fatalf("expected 左足ＩＫ IK target to be 左足首: got=%d want=%d", leftLegIK.Ik.BoneIndex, leftAnkle.Index())
	}
	if leftLegIK.Ik != nil {
		unit := leftLegIK.Ik.UnitRotation
		if math.Abs(unit.X-1.0) > 1e-6 || math.Abs(unit.Y-1.0) > 1e-6 || math.Abs(unit.Z-1.0) > 1e-6 {
			t.Fatalf("expected 左足ＩＫ IK unit rotation to be (1,1,1): got=(%f,%f,%f)", unit.X, unit.Y, unit.Z)
		}
	}

	leftToeIK, _ := modelData.Bones.GetByName(model.TOE_IK.Left())
	if leftToeIK.ParentIndex != leftLegIK.Index() {
		t.Fatalf("expected 左つま先ＩＫ parent to be 左足ＩＫ: got=%d want=%d", leftToeIK.ParentIndex, leftLegIK.Index())
	}
	if leftToeIK.Ik == nil {
		t.Fatalf("expected 左つま先ＩＫ IK setting")
	}
	if leftToeIK.Ik != nil && leftToeIK.Ik.BoneIndex != leftToe.Index() {
		t.Fatalf("expected 左つま先ＩＫ IK target to be 左つま先: got=%d want=%d", leftToeIK.Ik.BoneIndex, leftToe.Index())
	}
	if leftToeIK.Ik != nil && leftToeIK.Ik.LoopCount < 40 {
		t.Fatalf("expected 左つま先ＩＫ IK loop to be >= 40: got=%d", leftToeIK.Ik.LoopCount)
	}
	if leftToeIK.Ik != nil {
		unit := leftToeIK.Ik.UnitRotation
		if math.Abs(unit.X-1.0) > 1e-6 || math.Abs(unit.Y-1.0) > 1e-6 || math.Abs(unit.Z-1.0) > 1e-6 {
			t.Fatalf("expected 左つま先ＩＫ IK unit rotation to be (1,1,1): got=(%f,%f,%f)", unit.X, unit.Y, unit.Z)
		}
	}
	if leftToeIK.TailIndex >= 0 {
		t.Fatalf("expected 左つま先ＩＫ tail to be offset mode: tailIndex=%d", leftToeIK.TailIndex)
	}

	leftLegWeightedVertex, _ := modelData.Vertices.Get(0)
	if leftLegWeightedVertex == nil || leftLegWeightedVertex.Deform == nil {
		t.Fatalf("expected 左足ウェイト検証頂点")
	}
	if !containsBoneIndex(leftLegWeightedVertex.Deform.Indexes(), leftLegD.Index()) {
		t.Fatalf("expected 左足ウェイトが左足Dへ置換される: joints=%v", leftLegWeightedVertex.Deform.Indexes())
	}
	if containsBoneIndex(leftLegWeightedVertex.Deform.Indexes(), leftLeg.Index()) {
		t.Fatalf("expected 左足ウェイトから左足を除外: joints=%v", leftLegWeightedVertex.Deform.Indexes())
	}

	leftArm, _ := modelData.Bones.GetByName(model.ARM.Left())
	if leftArm.EnglishName != "LeftArm" {
		t.Fatalf("expected 左腕英名 LeftArm: got=%s", leftArm.EnglishName)
	}
	leftThumbTip, _ := modelData.Bones.GetByName(leftThumbTipName)
	leftThumbParent := leftThumbTip.ParentIndex
	parentIsThumbChain := false
	for _, name := range []string{model.THUMB2.Left(), model.THUMB1.Left(), model.THUMB0.Left()} {
		if thumbBone, err := modelData.Bones.GetByName(name); err == nil {
			if leftThumbParent == thumbBone.Index() {
				parentIsThumbChain = true
				if thumbBone.TailIndex != leftThumbTip.Index() {
					t.Fatalf("expected %s tail to be 左親指先: got=%d want=%d", name, thumbBone.TailIndex, leftThumbTip.Index())
				}
				break
			}
		}
	}
	if !parentIsThumbChain {
		t.Fatalf("expected 左親指先 parent to be 左親指系列: got=%d", leftThumbParent)
	}
	leftArmWeightedVertex, _ := modelData.Vertices.Get(1)
	if leftArmWeightedVertex == nil || leftArmWeightedVertex.Deform == nil {
		t.Fatalf("expected 左腕ウェイト検証頂点")
	}
	leftArmWeight := weightByBoneIndex(leftArmWeightedVertex.Deform.Indexes(), leftArmWeightedVertex.Deform.Weights(), leftArm.Index())
	leftArmTwist1Weight := weightByBoneIndex(leftArmWeightedVertex.Deform.Indexes(), leftArmWeightedVertex.Deform.Weights(), leftArmTwist1.Index())
	if math.Abs(leftArmWeight-0.6) > 1e-6 {
		t.Fatalf("expected 左腕ウェイト 0.6: got=%f joints=%v weights=%v", leftArmWeight, leftArmWeightedVertex.Deform.Indexes(), leftArmWeightedVertex.Deform.Weights())
	}
	if math.Abs(leftArmTwist1Weight-0.4) > 1e-6 {
		t.Fatalf("expected 左腕捩1ウェイト 0.4: got=%f joints=%v weights=%v", leftArmTwist1Weight, leftArmWeightedVertex.Deform.Indexes(), leftArmWeightedVertex.Deform.Weights())
	}

	leftToeWeightedVertex, _ := modelData.Vertices.Get(2)
	if leftToeWeightedVertex == nil || leftToeWeightedVertex.Deform == nil {
		t.Fatalf("expected 左つま先ウェイト検証頂点")
	}
	if !containsBoneIndex(leftToeWeightedVertex.Deform.Indexes(), leftToeEx.Index()) {
		t.Fatalf("expected 左つま先ウェイトが左足先EXへ置換される: joints=%v", leftToeWeightedVertex.Deform.Indexes())
	}
	if containsBoneIndex(leftToeWeightedVertex.Deform.Indexes(), leftToe.Index()) {
		t.Fatalf("expected 左つま先は直接ウェイト対象にしない: joints=%v", leftToeWeightedVertex.Deform.Indexes())
	}
	if leftToe.EnglishName != "LeftToe" {
		t.Fatalf("expected 左つま先英名 LeftToe: got=%s", leftToe.EnglishName)
	}
	if existingSkirt, err := modelData.Bones.GetByName("RSkBc0_01"); err == nil {
		if existingSkirt.EnglishName != "RSkBc0_01" {
			t.Fatalf("expected 非標準ボーン英名を同名維持: got=%s", existingSkirt.EnglishName)
		}
	}
	if renamedSkirt, err := modelData.Bones.GetByName("RSkBc0_01_2"); err == nil {
		if renamedSkirt.EnglishName != "RSkBc0_01_2" {
			t.Fatalf("expected J_Sec短縮ボーン英名を同名維持: got=%s", renamedSkirt.EnglishName)
		}
	}
}

func TestApplyHumanoidBoneMappingAfterReorderAppliesTongueWeightsFromFaceMouth(t *testing.T) {
	modelData := newBoneMappingTargetModel()
	headBefore, err := modelData.Bones.GetByName("head")
	if err != nil || headBefore == nil {
		t.Fatalf("head bone missing before mapping: err=%v", err)
	}

	faceMouthMaterial := newMaterial("FaceMouth", 1.0, 6)
	modelData.Materials.AppendRaw(faceMouthMaterial)

	tongueVertexStart := modelData.Vertices.Len()
	appendBoneMappingUvVertex(modelData, mmath.Vec3{Vec: r3.Vec{X: 0.05, Y: 17.85, Z: -0.20}}, mmath.Vec2{X: 0.55, Y: 0.45}, headBefore.Index())
	appendBoneMappingUvVertex(modelData, mmath.Vec3{Vec: r3.Vec{X: 0.00, Y: 17.70, Z: -0.35}}, mmath.Vec2{X: 0.65, Y: 0.35}, headBefore.Index())
	appendBoneMappingUvVertex(modelData, mmath.Vec3{Vec: r3.Vec{X: -0.05, Y: 17.60, Z: -0.50}}, mmath.Vec2{X: 0.75, Y: 0.25}, headBefore.Index())
	appendBoneMappingUvVertex(modelData, mmath.Vec3{Vec: r3.Vec{X: 0.04, Y: 17.90, Z: -0.22}}, mmath.Vec2{X: 0.20, Y: 0.60}, headBefore.Index())
	appendBoneMappingUvVertex(modelData, mmath.Vec3{Vec: r3.Vec{X: 0.00, Y: 17.75, Z: -0.37}}, mmath.Vec2{X: 0.30, Y: 0.70}, headBefore.Index())
	appendBoneMappingUvVertex(modelData, mmath.Vec3{Vec: r3.Vec{X: -0.04, Y: 17.65, Z: -0.48}}, mmath.Vec2{X: 0.40, Y: 0.80}, headBefore.Index())

	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{tongueVertexStart, tongueVertexStart + 1, tongueVertexStart + 2}})
	modelData.Faces.AppendRaw(&model.Face{VertexIndexes: [3]int{tongueVertexStart + 3, tongueVertexStart + 4, tongueVertexStart + 5}})

	if err := applyHumanoidBoneMappingAfterReorder(modelData); err != nil {
		t.Fatalf("mapping failed: %v", err)
	}

	headAfter, err := modelData.Bones.GetByName(model.HEAD.String())
	if err != nil || headAfter == nil {
		t.Fatalf("head bone missing after mapping: err=%v", err)
	}
	tongue1, err := modelData.Bones.GetByName(tongueBone1Name)
	if err != nil || tongue1 == nil {
		t.Fatalf("tongue1 missing: err=%v", err)
	}
	tongue2, err := modelData.Bones.GetByName(tongueBone2Name)
	if err != nil || tongue2 == nil {
		t.Fatalf("tongue2 missing: err=%v", err)
	}
	tongue3, err := modelData.Bones.GetByName(tongueBone3Name)
	if err != nil || tongue3 == nil {
		t.Fatalf("tongue3 missing: err=%v", err)
	}
	tongue4, err := modelData.Bones.GetByName(tongueBone4Name)
	if err != nil || tongue4 == nil {
		t.Fatalf("tongue4 missing: err=%v", err)
	}
	if tongue1.ParentIndex != headAfter.Index() {
		t.Fatalf("expected tongue1 parent to be head: got=%d want=%d", tongue1.ParentIndex, headAfter.Index())
	}
	if tongue1.TailIndex != tongue2.Index() {
		t.Fatalf("expected tongue1 tail to be tongue2: got=%d want=%d", tongue1.TailIndex, tongue2.Index())
	}
	if tongue2.TailIndex != tongue3.Index() {
		t.Fatalf("expected tongue2 tail to be tongue3: got=%d want=%d", tongue2.TailIndex, tongue3.Index())
	}
	if tongue3.TailIndex != tongue4.Index() {
		t.Fatalf("expected tongue3 tail to be tongue4: got=%d want=%d", tongue3.TailIndex, tongue4.Index())
	}
	if tongue4.TailIndex >= 0 {
		t.Fatalf("expected tongue4 tail to be offset: got tailIndex=%d", tongue4.TailIndex)
	}
	tongueTip := tongue4.Position.Added(tongue4.TailPosition)
	ratio2 := resolvePositionRatioOnSegment(tongue2.Position, tongue1.Position, tongueTip)
	ratio3 := resolvePositionRatioOnSegment(tongue3.Position, tongue1.Position, tongueTip)
	ratio4 := resolvePositionRatioOnSegment(tongue4.Position, tongue1.Position, tongueTip)
	if math.Abs(ratio2-tongueBone2RatioFine) > 1e-6 {
		t.Fatalf("expected tongue2 ratio %.3f: got=%.6f", tongueBone2RatioFine, ratio2)
	}
	if math.Abs(ratio3-tongueBone3RatioFine) > 1e-6 {
		t.Fatalf("expected tongue3 ratio %.3f: got=%.6f", tongueBone3RatioFine, ratio3)
	}
	if math.Abs(ratio4-tongueBone4RatioFine) > 1e-6 {
		t.Fatalf("expected tongue4 ratio %.3f: got=%.6f", tongueBone4RatioFine, ratio4)
	}
	if tongue1.LocalAxisX.Length() <= 1e-8 || tongue1.LocalAxisZ.Length() <= 1e-8 {
		t.Fatalf("expected tongue1 local axis to be configured")
	}

	tongueBoneIndexes := map[int]struct{}{
		tongue1.Index(): {},
		tongue2.Index(): {},
		tongue3.Index(): {},
		tongue4.Index(): {},
	}
	tongueMappedCount := 0
	bdef2Count := 0
	for _, vertexIndex := range []int{tongueVertexStart, tongueVertexStart + 1, tongueVertexStart + 2} {
		vertex, vertexErr := modelData.Vertices.Get(vertexIndex)
		if vertexErr != nil || vertex == nil || vertex.Deform == nil {
			t.Fatalf("tongue vertex missing after mapping: idx=%d err=%v", vertexIndex, vertexErr)
		}
		hasTongueBone := false
		for _, jointIndex := range vertex.Deform.Indexes() {
			if _, exists := tongueBoneIndexes[jointIndex]; exists {
				hasTongueBone = true
			}
			if jointIndex == headAfter.Index() {
				t.Fatalf("tongue vertex should not keep head joint: idx=%d joints=%v", vertexIndex, vertex.Deform.Indexes())
			}
		}
		if hasTongueBone {
			tongueMappedCount++
		}
		if vertex.DeformType == model.BDEF2 {
			bdef2Count++
		}
	}
	if tongueMappedCount != 3 {
		t.Fatalf("expected all tongue target vertices to map to tongue bones: got=%d", tongueMappedCount)
	}
	if bdef2Count == 0 {
		t.Fatalf("expected at least one tongue vertex to be BDEF2")
	}

	nonTongueVertex, err := modelData.Vertices.Get(tongueVertexStart + 3)
	if err != nil || nonTongueVertex == nil || nonTongueVertex.Deform == nil {
		t.Fatalf("non tongue vertex missing after mapping: err=%v", err)
	}
	if !containsBoneIndex(nonTongueVertex.Deform.Indexes(), headAfter.Index()) {
		t.Fatalf("expected non tongue uv vertex to keep head weight: joints=%v", nonTongueVertex.Deform.Indexes())
	}
}

func TestPrepareModelAppliesBoneMappingAfterMaterialReorder(t *testing.T) {
	tempDir := t.TempDir()
	inPath := filepath.Join(tempDir, "sample.vrm")
	outPath := filepath.Join(tempDir, "sample.pmx")

	writeGLBForUsecaseTest(t, inPath, map[string]any{
		"asset": map[string]any{
			"version": "2.0",
		},
		"extensionsUsed": []string{"VRMC_vrm"},
		"nodes": []any{
			map[string]any{
				"name":     "hips",
				"children": []int{1},
			},
			map[string]any{
				"name": "spine",
			},
		},
		"extensions": map[string]any{
			"VRMC_vrm": map[string]any{
				"specVersion": "1.0",
				"humanoid": map[string]any{
					"humanBones": map[string]any{
						"hips":  map[string]any{"node": 0},
						"spine": map[string]any{"node": 1},
					},
				},
			},
		},
	}, nil)

	uc := NewVrm2PmxUsecase(Vrm2PmxUsecaseDeps{
		ModelReader: vrmrepository.NewVrmRepository(),
		ModelWriter: pmx.NewPmxRepository(),
	})

	result, err := uc.PrepareModel(ConvertRequest{InputPath: inPath, OutputPath: outPath})
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	if result == nil || result.Model == nil {
		t.Fatalf("result/model is nil")
	}
	if _, err := result.Model.Bones.GetByName(model.LOWER.String()); err != nil {
		t.Fatalf("expected mapped bone 下半身: %v", err)
	}
	if _, err := result.Model.Bones.GetByName(model.UPPER.String()); err != nil {
		t.Fatalf("expected mapped bone 上半身: %v", err)
	}
	if _, err := result.Model.Bones.GetByName("hips"); err == nil {
		t.Fatalf("expected raw bone name hips to be renamed")
	}
}

// newBoneMappingTargetModel は補完・命名変更テスト用モデルを生成する。
func newBoneMappingTargetModel() *ModelData {
	modelData := model.NewPmxModel()
	modelData.VrmData = vrm.NewVrmData()
	modelData.VrmData.Version = vrm.VRM_VERSION_1
	modelData.VrmData.Vrm1 = vrm.NewVrm1Data()

	type nodeSpec struct {
		Name   string
		Parent int
		Pos    mmath.Vec3
	}

	specs := []nodeSpec{
		{Name: "hips", Parent: -1, Pos: mmath.Vec3{Vec: r3.Vec{X: 0, Y: 10, Z: 0}}},
		{Name: "spine", Parent: 0, Pos: mmath.Vec3{Vec: r3.Vec{X: 0, Y: 12, Z: 0}}},
		{Name: "chest", Parent: 1, Pos: mmath.Vec3{Vec: r3.Vec{X: 0, Y: 14, Z: 0}}},
		{Name: "leftShoulder", Parent: 2, Pos: mmath.Vec3{Vec: r3.Vec{X: 0.8, Y: 14.6, Z: 0}}},
		{Name: "leftUpperArm", Parent: 3, Pos: mmath.Vec3{Vec: r3.Vec{X: 1.2, Y: 14.5, Z: 0}}},
		{Name: "leftLowerArm", Parent: 4, Pos: mmath.Vec3{Vec: r3.Vec{X: 2.2, Y: 14.0, Z: 0}}},
		{Name: "leftHand", Parent: 5, Pos: mmath.Vec3{Vec: r3.Vec{X: 3.0, Y: 13.5, Z: 0}}},
		{Name: "rightShoulder", Parent: 2, Pos: mmath.Vec3{Vec: r3.Vec{X: -0.8, Y: 14.6, Z: 0}}},
		{Name: "rightUpperArm", Parent: 7, Pos: mmath.Vec3{Vec: r3.Vec{X: -1.2, Y: 14.5, Z: 0}}},
		{Name: "rightLowerArm", Parent: 8, Pos: mmath.Vec3{Vec: r3.Vec{X: -2.2, Y: 14.0, Z: 0}}},
		{Name: "rightHand", Parent: 9, Pos: mmath.Vec3{Vec: r3.Vec{X: -3.0, Y: 13.5, Z: 0}}},
		{Name: "leftUpperLeg", Parent: 0, Pos: mmath.Vec3{Vec: r3.Vec{X: 0.8, Y: 8.8, Z: 0}}},
		{Name: "leftLowerLeg", Parent: 11, Pos: mmath.Vec3{Vec: r3.Vec{X: 0.8, Y: 5.5, Z: 0}}},
		{Name: "leftFoot", Parent: 12, Pos: mmath.Vec3{Vec: r3.Vec{X: 0.8, Y: 2.0, Z: 0.3}}},
		{Name: "leftToes", Parent: 13, Pos: mmath.Vec3{Vec: r3.Vec{X: 1.1, Y: 0.8, Z: -0.8}}},
		{Name: "rightUpperLeg", Parent: 0, Pos: mmath.Vec3{Vec: r3.Vec{X: -0.8, Y: 8.8, Z: 0}}},
		{Name: "rightLowerLeg", Parent: 15, Pos: mmath.Vec3{Vec: r3.Vec{X: -0.8, Y: 5.5, Z: 0}}},
		{Name: "rightFoot", Parent: 16, Pos: mmath.Vec3{Vec: r3.Vec{X: -0.8, Y: 2.0, Z: 0.3}}},
		{Name: "rightToes", Parent: 17, Pos: mmath.Vec3{Vec: r3.Vec{X: -1.1, Y: 0.8, Z: -0.8}}},
		{Name: "leftThumbProximal", Parent: 6, Pos: mmath.Vec3{Vec: r3.Vec{X: 3.2, Y: 13.6, Z: 0.2}}},
		{Name: "leftIndexProximal", Parent: 6, Pos: mmath.Vec3{Vec: r3.Vec{X: 3.5, Y: 13.6, Z: 0.1}}},
		{Name: "leftMiddleProximal", Parent: 6, Pos: mmath.Vec3{Vec: r3.Vec{X: 3.7, Y: 13.5, Z: 0.0}}},
		{Name: "leftRingProximal", Parent: 6, Pos: mmath.Vec3{Vec: r3.Vec{X: 3.6, Y: 13.4, Z: -0.1}}},
		{Name: "leftLittleProximal", Parent: 6, Pos: mmath.Vec3{Vec: r3.Vec{X: 3.4, Y: 13.3, Z: -0.2}}},
		{Name: "rightThumbProximal", Parent: 10, Pos: mmath.Vec3{Vec: r3.Vec{X: -3.2, Y: 13.6, Z: 0.2}}},
		{Name: "rightIndexProximal", Parent: 10, Pos: mmath.Vec3{Vec: r3.Vec{X: -3.5, Y: 13.6, Z: 0.1}}},
		{Name: "rightMiddleProximal", Parent: 10, Pos: mmath.Vec3{Vec: r3.Vec{X: -3.7, Y: 13.5, Z: 0.0}}},
		{Name: "rightRingProximal", Parent: 10, Pos: mmath.Vec3{Vec: r3.Vec{X: -3.6, Y: 13.4, Z: -0.1}}},
		{Name: "rightLittleProximal", Parent: 10, Pos: mmath.Vec3{Vec: r3.Vec{X: -3.4, Y: 13.3, Z: -0.2}}},
		{Name: "neck", Parent: 2, Pos: mmath.Vec3{Vec: r3.Vec{X: 0, Y: 16.0, Z: 0}}},
		{Name: "head", Parent: 29, Pos: mmath.Vec3{Vec: r3.Vec{X: 0, Y: 18.0, Z: 0}}},
		{Name: "leftEye", Parent: 30, Pos: mmath.Vec3{Vec: r3.Vec{X: 0.4, Y: 18.2, Z: -0.2}}},
		{Name: "rightEye", Parent: 30, Pos: mmath.Vec3{Vec: r3.Vec{X: -0.4, Y: 18.2, Z: -0.2}}},
		{Name: "Face", Parent: 30, Pos: mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 17.8, Z: -0.3}}},
		{Name: "Body", Parent: 0, Pos: mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 11.0, Z: 0.0}}},
		{Name: "Hair", Parent: 30, Pos: mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 19.0, Z: 0.2}}},
		{Name: "secondary", Parent: 30, Pos: mmath.Vec3{Vec: r3.Vec{X: 0.0, Y: 18.7, Z: 0.1}}},
		{Name: "RSkBc0_01", Parent: 0, Pos: mmath.Vec3{Vec: r3.Vec{X: -0.4, Y: 8.5, Z: -0.1}}},
		{Name: "J_Sec_R_SkirtBack0_01", Parent: 0, Pos: mmath.Vec3{Vec: r3.Vec{X: 0.4, Y: 8.5, Z: -0.1}}},
	}

	nodeIndexes := map[string]int{}
	for i, spec := range specs {
		node := vrm.NewNode(i)
		node.Name = spec.Name
		node.ParentIndex = spec.Parent
		modelData.VrmData.Nodes = append(modelData.VrmData.Nodes, *node)
		nodeIndexes[spec.Name] = i

		bone := model.NewBoneByName(spec.Name)
		bone.Position = spec.Pos
		bone.ParentIndex = spec.Parent
		bone.BoneFlag = model.BONE_FLAG_IS_VISIBLE | model.BONE_FLAG_CAN_MANIPULATE | model.BONE_FLAG_CAN_ROTATE | model.BONE_FLAG_CAN_TRANSLATE
		modelData.Bones.AppendRaw(bone)
	}

	modelData.VrmData.Vrm1.Humanoid.HumanBones = map[string]vrm.Vrm1HumanBone{
		"hips":                {Node: nodeIndexes["hips"]},
		"spine":               {Node: nodeIndexes["spine"]},
		"chest":               {Node: nodeIndexes["chest"]},
		"neck":                {Node: nodeIndexes["neck"]},
		"head":                {Node: nodeIndexes["head"]},
		"leftShoulder":        {Node: nodeIndexes["leftShoulder"]},
		"rightShoulder":       {Node: nodeIndexes["rightShoulder"]},
		"leftUpperArm":        {Node: nodeIndexes["leftUpperArm"]},
		"leftLowerArm":        {Node: nodeIndexes["leftLowerArm"]},
		"leftHand":            {Node: nodeIndexes["leftHand"]},
		"rightUpperArm":       {Node: nodeIndexes["rightUpperArm"]},
		"rightLowerArm":       {Node: nodeIndexes["rightLowerArm"]},
		"rightHand":           {Node: nodeIndexes["rightHand"]},
		"leftEye":             {Node: nodeIndexes["leftEye"]},
		"rightEye":            {Node: nodeIndexes["rightEye"]},
		"leftUpperLeg":        {Node: nodeIndexes["leftUpperLeg"]},
		"leftLowerLeg":        {Node: nodeIndexes["leftLowerLeg"]},
		"leftFoot":            {Node: nodeIndexes["leftFoot"]},
		"leftToes":            {Node: nodeIndexes["leftToes"]},
		"rightUpperLeg":       {Node: nodeIndexes["rightUpperLeg"]},
		"rightLowerLeg":       {Node: nodeIndexes["rightLowerLeg"]},
		"rightFoot":           {Node: nodeIndexes["rightFoot"]},
		"rightToes":           {Node: nodeIndexes["rightToes"]},
		"leftThumbProximal":   {Node: nodeIndexes["leftThumbProximal"]},
		"leftIndexProximal":   {Node: nodeIndexes["leftIndexProximal"]},
		"leftMiddleProximal":  {Node: nodeIndexes["leftMiddleProximal"]},
		"leftRingProximal":    {Node: nodeIndexes["leftRingProximal"]},
		"leftLittleProximal":  {Node: nodeIndexes["leftLittleProximal"]},
		"rightThumbProximal":  {Node: nodeIndexes["rightThumbProximal"]},
		"rightIndexProximal":  {Node: nodeIndexes["rightIndexProximal"]},
		"rightMiddleProximal": {Node: nodeIndexes["rightMiddleProximal"]},
		"rightRingProximal":   {Node: nodeIndexes["rightRingProximal"]},
		"rightLittleProximal": {Node: nodeIndexes["rightLittleProximal"]},
	}

	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:   mmath.Vec3{Vec: r3.Vec{X: 0.8, Y: 7.0, Z: 0.0}},
		Normal:     mmath.UNIT_Y_VEC3,
		Uv:         mmath.ZERO_VEC2,
		DeformType: model.BDEF1,
		Deform:     model.NewBdef1(nodeIndexes["leftUpperLeg"]),
		EdgeFactor: 1.0,
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:   mmath.Vec3{Vec: r3.Vec{X: 1.3, Y: 14.3, Z: 0.0}},
		Normal:     mmath.UNIT_Y_VEC3,
		Uv:         mmath.ZERO_VEC2,
		DeformType: model.BDEF1,
		Deform:     model.NewBdef1(nodeIndexes["leftUpperArm"]),
		EdgeFactor: 1.0,
	})
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:   mmath.Vec3{Vec: r3.Vec{X: 1.1, Y: 0.8, Z: -0.8}},
		Normal:     mmath.UNIT_Y_VEC3,
		Uv:         mmath.ZERO_VEC2,
		DeformType: model.BDEF1,
		Deform:     model.NewBdef1(nodeIndexes["leftToes"]),
		EdgeFactor: 1.0,
	})

	return modelData
}

// appendBoneMappingUvVertex はUV付き検証頂点を追加する。
func appendBoneMappingUvVertex(modelData *ModelData, position mmath.Vec3, uv mmath.Vec2, boneIndex int) {
	modelData.Vertices.AppendRaw(&model.Vertex{
		Position:        position,
		Normal:          mmath.UNIT_Y_VEC3,
		Uv:              uv,
		DeformType:      model.BDEF1,
		Deform:          model.NewBdef1(boneIndex),
		EdgeFactor:      1.0,
		MaterialIndexes: []int{0},
	})
}

// containsBoneIndex はジョイント配列に対象indexが含まれるか判定する。
func containsBoneIndex(indexes []int, target int) bool {
	for _, index := range indexes {
		if index == target {
			return true
		}
	}
	return false
}

// weightByBoneIndex は対象indexに割り当てられたウェイト合計を返す。
func weightByBoneIndex(indexes []int, weights []float64, target int) float64 {
	maxCount := len(indexes)
	if len(weights) < maxCount {
		maxCount = len(weights)
	}
	total := 0.0
	for i := 0; i < maxCount; i++ {
		if indexes[i] == target {
			total += weights[i]
		}
	}
	return total
}
