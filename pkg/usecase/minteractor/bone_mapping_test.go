// 指示: miu200521358
package minteractor

import (
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
		model.TOE_T.Left(),
		model.TOE_T.Right(),
		model.HEEL.Left(),
		model.HEEL.Right(),
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
		model.WRIST_TAIL.Left(),
		model.WRIST_TAIL.Right(),
	} {
		if _, err := modelData.Bones.GetByName(name); err != nil {
			t.Fatalf("expected bone %s to exist: %v", name, err)
		}
	}

	if _, err := modelData.Bones.GetByName("leftUpperLeg"); err == nil {
		t.Fatalf("expected leftUpperLeg to be renamed")
	}

	leftToeT, _ := modelData.Bones.GetByName(model.TOE_T.Left())
	leftAnkle, _ := modelData.Bones.GetByName(model.ANKLE.Left())
	if leftToeT.ParentIndex != leftAnkle.Index() {
		t.Fatalf("expected 左つま先先 parent to be 左足首: got=%d want=%d", leftToeT.ParentIndex, leftAnkle.Index())
	}
	center, _ := modelData.Bones.GetByName(model.CENTER.String())
	if center.Position.X != 0.0 || center.Position.Z != 0.0 {
		t.Fatalf("expected センター XZ to be 0: got=(%f,%f)", center.Position.X, center.Position.Z)
	}
	if center.Position.Y != 5.0 {
		t.Fatalf("expected センター Y to be 下半身Yの半分(5.0): got=%f", center.Position.Y)
	}
	groove, _ := modelData.Bones.GetByName(model.GROOVE.String())
	if groove.Position.X != 0.0 || groove.Position.Z != 0.0 {
		t.Fatalf("expected グルーブ XZ to be 0: got=(%f,%f)", groove.Position.X, groove.Position.Z)
	}
	if groove.Position.Y != 7.0 {
		t.Fatalf("expected グルーブ Y to be 下半身Yの7割(7.0): got=%f", groove.Position.Y)
	}

	waist, _ := modelData.Bones.GetByName(model.WAIST.String())
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

	leftAnkleD, _ := modelData.Bones.GetByName(model.ANKLE_D.Left())
	leftToeEx, _ := modelData.Bones.GetByName(model.TOE_EX.Left())
	if leftToeEx.ParentIndex != leftAnkleD.Index() {
		t.Fatalf("expected 左足先EX parent to be 左足首D: got=%d want=%d", leftToeEx.ParentIndex, leftAnkleD.Index())
	}

	leftLegIKParent, _ := modelData.Bones.GetByName(model.LEG_IK_PARENT.Left())
	leftLegIK, _ := modelData.Bones.GetByName(model.LEG_IK.Left())
	if leftLegIK.ParentIndex != leftLegIKParent.Index() {
		t.Fatalf("expected 左足ＩＫ parent to be 左足IK親: got=%d want=%d", leftLegIK.ParentIndex, leftLegIKParent.Index())
	}
	if leftLegIK.Ik == nil {
		t.Fatalf("expected 左足ＩＫ IK setting")
	}
	if leftLegIK.Ik != nil && leftLegIK.Ik.BoneIndex != leftAnkle.Index() {
		t.Fatalf("expected 左足ＩＫ IK target to be 左足首: got=%d want=%d", leftLegIK.Ik.BoneIndex, leftAnkle.Index())
	}

	leftToeIK, _ := modelData.Bones.GetByName(model.TOE_IK.Left())
	if leftToeIK.ParentIndex != leftLegIK.Index() {
		t.Fatalf("expected 左つま先ＩＫ parent to be 左足ＩＫ: got=%d want=%d", leftToeIK.ParentIndex, leftLegIK.Index())
	}
	if leftToeIK.Ik == nil {
		t.Fatalf("expected 左つま先ＩＫ IK setting")
	}
	if leftToeIK.Ik != nil && leftToeIK.Ik.BoneIndex != leftToeT.Index() {
		t.Fatalf("expected 左つま先ＩＫ IK target to be 左つま先先: got=%d want=%d", leftToeIK.Ik.BoneIndex, leftToeT.Index())
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

	return modelData
}
