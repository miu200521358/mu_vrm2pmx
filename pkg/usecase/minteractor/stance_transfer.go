// 指示: miu200521358
package minteractor

import (
	"math"
	"sort"
	"strings"

	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
)

const (
	astanceRightArmRollDegree     = 35.0
	astanceLeftArmRollDegree      = -35.0
	astanceRightThumb0YawDegree   = 8.0
	astanceRightThumb1YawDegree   = 24.0
	astanceLeftThumb0YawDegree    = -8.0
	astanceLeftThumb1YawDegree    = -24.0
	astanceAxisEpsilon            = 1e-8
	astanceMinimumBdef2BoneCount  = 2
	astanceMinimumBdef4BoneCount  = 4
	astanceTstanceUpDownTolerance = 10.0
	astanceTstanceSideTolerance   = 30.0
)

// astanceBoneTransform はAスタンス補正後のボーン姿勢を表す。
type astanceBoneTransform struct {
	Position mmath.Vec3
	Rotation mmath.Quaternion
}

// applyAstanceBeforeViewer はTスタンスと判定できるモデルをAスタンスへ補正する。
func applyAstanceBeforeViewer(modelData *ModelData) error {
	if !shouldApplyAstance(modelData) {
		return nil
	}
	if modelData.Bones == nil || modelData.Bones.Len() == 0 {
		return nil
	}

	originalPositions := collectOriginalBonePositions(modelData.Bones)
	transformedBones := collectAstanceTransformedBones(modelData.Bones, originalPositions)
	if len(transformedBones) == 0 {
		return nil
	}

	applyAstanceBonePositions(modelData.Bones, transformedBones)
	updateAstanceBoneLocalAxes(modelData.Bones, transformedBones)
	applyAstanceVertices(modelData, originalPositions, transformedBones)

	return nil
}

// shouldApplyAstance は姿勢判定に基づくAスタンス補正適用可否を返す。
func shouldApplyAstance(modelData *ModelData) bool {
	if modelData == nil || modelData.Bones == nil || modelData.Bones.Len() == 0 {
		return false
	}
	return isAstanceTargetTstance(modelData.Bones)
}

// isAstanceTargetTstance は左右腕がTスタンス相当か判定する。
func isAstanceTargetTstance(bones *model.BoneCollection) bool {
	if bones == nil {
		return false
	}

	leftArm, leftArmExists := getBoneByName(bones, model.ARM.Left())
	leftElbow, leftElbowExists := getBoneByName(bones, model.ELBOW.Left())
	rightArm, rightArmExists := getBoneByName(bones, model.ARM.Right())
	rightElbow, rightElbowExists := getBoneByName(bones, model.ELBOW.Right())
	if !leftArmExists || !leftElbowExists || !rightArmExists || !rightElbowExists {
		return false
	}

	leftVector := leftElbow.Position.Subed(leftArm.Position)
	rightVector := rightElbow.Position.Subed(rightArm.Position)
	if !isAstanceTargetArmVector(leftVector) || !isAstanceTargetArmVector(rightVector) {
		return false
	}
	if leftVector.X*rightVector.X >= 0 {
		return false
	}
	return true
}

// isAstanceTargetArmVector は片腕ベクトルがTスタンス相当か判定する。
func isAstanceTargetArmVector(armVector mmath.Vec3) bool {
	length := armVector.Length()
	if length <= astanceAxisEpsilon {
		return false
	}

	upDownRatio := clampAstanceValue(armVector.Y/length, -1.0, 1.0)
	upDownDegree := mmath.RadToDeg(math.Abs(math.Asin(upDownRatio)))
	if upDownDegree > astanceTstanceUpDownTolerance {
		return false
	}

	sideLength := math.Hypot(armVector.X, armVector.Z)
	if sideLength <= astanceAxisEpsilon {
		return false
	}
	sideDegree := mmath.RadToDeg(math.Abs(math.Atan2(math.Abs(armVector.Z), math.Abs(armVector.X))))
	return sideDegree <= astanceTstanceSideTolerance
}

// clampAstanceValue はmin-maxで値をクランプする。
func clampAstanceValue(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// collectOriginalBonePositions は補正前ボーン位置をindex単位で退避する。
func collectOriginalBonePositions(bones *model.BoneCollection) map[int]mmath.Vec3 {
	positions := map[int]mmath.Vec3{}
	if bones == nil {
		return positions
	}
	for _, bone := range bones.Values() {
		if bone == nil {
			continue
		}
		positions[bone.Index()] = bone.Position
	}
	return positions
}

// collectAstanceTransformedBones はAスタンス適用後のボーン位置と回転を収集する。
func collectAstanceTransformedBones(
	bones *model.BoneCollection,
	originalPositions map[int]mmath.Vec3,
) map[int]astanceBoneTransform {
	transformedBones := map[int]astanceBoneTransform{}
	if bones == nil || len(originalPositions) == 0 {
		return transformedBones
	}

	rootBone, rootExists := getBoneByName(bones, model.UPPER.String())
	if !rootExists {
		return transformedBones
	}
	rootPosition, rootPositionExists := originalPositions[rootBone.Index()]
	if !rootPositionExists {
		return transformedBones
	}

	childrenByParent := collectBoneChildrenByParent(bones)
	traverseAstanceBoneHierarchy(
		bones,
		rootBone.Index(),
		astanceBoneTransform{
			Position: rootPosition,
			Rotation: mmath.NewQuaternion(),
		},
		originalPositions,
		childrenByParent,
		transformedBones,
	)

	return transformedBones
}

// collectBoneChildrenByParent は親indexごとの子index一覧を構築する。
func collectBoneChildrenByParent(bones *model.BoneCollection) map[int][]int {
	childrenByParent := map[int][]int{}
	if bones == nil {
		return childrenByParent
	}
	for _, bone := range bones.Values() {
		if bone == nil || bone.ParentIndex < 0 {
			continue
		}
		childrenByParent[bone.ParentIndex] = append(childrenByParent[bone.ParentIndex], bone.Index())
	}
	for parentIndex := range childrenByParent {
		sort.Ints(childrenByParent[parentIndex])
	}
	return childrenByParent
}

// traverseAstanceBoneHierarchy は上半身起点でAスタンス姿勢を階層伝播計算する。
func traverseAstanceBoneHierarchy(
	bones *model.BoneCollection,
	boneIndex int,
	currentTransform astanceBoneTransform,
	originalPositions map[int]mmath.Vec3,
	childrenByParent map[int][]int,
	transformedBones map[int]astanceBoneTransform,
) {
	if bones == nil || transformedBones == nil {
		return
	}
	bone, err := bones.Get(boneIndex)
	if err != nil || bone == nil {
		return
	}
	if _, exists := originalPositions[boneIndex]; !exists {
		return
	}

	updatedRotation := currentTransform.Rotation
	localRotation := resolveAstanceLocalRotation(bone.Name())
	updatedRotation = updatedRotation.Muled(localRotation)
	resolvedTransform := astanceBoneTransform{
		Position: currentTransform.Position,
		Rotation: updatedRotation,
	}
	transformedBones[boneIndex] = resolvedTransform

	for _, childIndex := range childrenByParent[boneIndex] {
		childPos, childPosExists := originalPositions[childIndex]
		parentPos, parentPosExists := originalPositions[boneIndex]
		if !childPosExists || !parentPosExists {
			continue
		}
		childRelative := childPos.Subed(parentPos)
		childTransform := astanceBoneTransform{
			Position: currentTransform.Position.Added(updatedRotation.MulVec3(childRelative)),
			Rotation: updatedRotation,
		}
		traverseAstanceBoneHierarchy(
			bones,
			childIndex,
			childTransform,
			originalPositions,
			childrenByParent,
			transformedBones,
		)
	}
}

// resolveAstanceLocalRotation はAスタンス補正対象ボーンのローカル回転を返す。
func resolveAstanceLocalRotation(boneName string) mmath.Quaternion {
	switch boneName {
	case model.ARM.Right():
		return mmath.NewQuaternionFromDegrees(0, 0, astanceRightArmRollDegree)
	case model.ARM.Left():
		return mmath.NewQuaternionFromDegrees(0, 0, astanceLeftArmRollDegree)
	case model.THUMB0.Right():
		return mmath.NewQuaternionFromDegrees(0, astanceRightThumb0YawDegree, 0)
	case model.THUMB1.Right():
		return mmath.NewQuaternionFromDegrees(0, astanceRightThumb1YawDegree, 0)
	case model.THUMB0.Left():
		return mmath.NewQuaternionFromDegrees(0, astanceLeftThumb0YawDegree, 0)
	case model.THUMB1.Left():
		return mmath.NewQuaternionFromDegrees(0, astanceLeftThumb1YawDegree, 0)
	default:
		return mmath.NewQuaternion()
	}
}

// applyAstanceBonePositions は補正後ボーン位置をモデルへ反映する。
func applyAstanceBonePositions(bones *model.BoneCollection, transformedBones map[int]astanceBoneTransform) {
	if bones == nil || len(transformedBones) == 0 {
		return
	}
	for boneIndex, transformedBone := range transformedBones {
		bone, err := bones.Get(boneIndex)
		if err != nil || bone == nil {
			continue
		}
		bone.Position = transformedBone.Position
	}
}

// updateAstanceBoneLocalAxes はAスタンス補正後のローカル軸を再計算する。
func updateAstanceBoneLocalAxes(bones *model.BoneCollection, transformedBones map[int]astanceBoneTransform) {
	if bones == nil || len(transformedBones) == 0 {
		return
	}
	targetIndexes := make([]int, 0, len(transformedBones))
	for boneIndex := range transformedBones {
		targetIndexes = append(targetIndexes, boneIndex)
	}
	sort.Ints(targetIndexes)

	for _, boneIndex := range targetIndexes {
		bone, err := bones.Get(boneIndex)
		if err != nil || bone == nil {
			continue
		}
		updateAstanceArmLocalAxes(bones, bone)
		updateAstanceTwistAxes(bones, bone)
		updateAstanceFingerLocalAxes(bones, bone)
	}
}

// updateAstanceArmLocalAxes は肩・腕・ひじ・手首のローカル軸を更新する。
func updateAstanceArmLocalAxes(bones *model.BoneCollection, bone *model.Bone) {
	if bones == nil || bone == nil {
		return
	}
	armName, elbowName, wristName, fingerName, sideExists := resolveAstanceSideBoneNames(bone.Name())
	if !sideExists {
		return
	}

	switch bone.Name() {
	case model.SHOULDER.Right(), model.SHOULDER.Left():
		armBone, armExists := getBoneByName(bones, armName)
		if !armExists {
			return
		}
		axisX := armBone.Position.Subed(bone.Position)
		axisZ := axisX.Cross(mmath.UNIT_Y_NEG_VEC3)
		assignAstanceLocalAxes(bone, axisX, axisZ)
	case model.ARM.Right(), model.ARM.Left():
		elbowBone, elbowExists := getBoneByName(bones, elbowName)
		if !elbowExists {
			return
		}
		axisX := elbowBone.Position.Subed(bone.Position)
		axisZ := axisX.Cross(mmath.UNIT_Y_NEG_VEC3)
		assignAstanceLocalAxes(bone, axisX, axisZ)
	case model.ELBOW.Right(), model.ELBOW.Left():
		wristBone, wristExists := getBoneByName(bones, wristName)
		if !wristExists {
			return
		}
		axisX := wristBone.Position.Subed(bone.Position)
		axisZ := mmath.UNIT_Y_NEG_VEC3.Cross(axisX)
		assignAstanceLocalAxes(bone, axisX, axisZ)
	case model.WRIST.Right(), model.WRIST.Left():
		fingerBone, fingerExists := getBoneByName(bones, fingerName)
		if !fingerExists {
			return
		}
		axisX := fingerBone.Position.Subed(bone.Position)
		axisZ := axisX.Cross(mmath.UNIT_Y_NEG_VEC3)
		assignAstanceLocalAxes(bone, axisX, axisZ)
	}
}

// updateAstanceTwistAxes は腕捩・手捩の固定軸とローカル軸を更新する。
func updateAstanceTwistAxes(bones *model.BoneCollection, bone *model.Bone) {
	if bones == nil || bone == nil {
		return
	}
	armName, elbowName, wristName, _, sideExists := resolveAstanceSideBoneNames(bone.Name())
	if !sideExists {
		return
	}

	switch bone.Name() {
	case model.ARM_TWIST.Right(), model.ARM_TWIST.Left():
		armBone, armExists := getBoneByName(bones, armName)
		elbowBone, elbowExists := getBoneByName(bones, elbowName)
		if !armExists || !elbowExists {
			return
		}
		axisX := elbowBone.Position.Subed(armBone.Position)
		axisZ := axisX.Cross(mmath.UNIT_Y_NEG_VEC3)
		assignAstanceFixedAxisAndLocalAxes(bone, axisX, axisZ)
	case model.WRIST_TWIST.Right(), model.WRIST_TWIST.Left():
		elbowBone, elbowExists := getBoneByName(bones, elbowName)
		wristBone, wristExists := getBoneByName(bones, wristName)
		if !elbowExists || !wristExists {
			return
		}
		axisX := wristBone.Position.Subed(elbowBone.Position)
		axisZ := axisX.Cross(mmath.UNIT_Y_NEG_VEC3)
		assignAstanceFixedAxisAndLocalAxes(bone, axisX, axisZ)
	}
}

// updateAstanceFingerLocalAxes は指ボーンのローカル軸を更新する。
func updateAstanceFingerLocalAxes(bones *model.BoneCollection, bone *model.Bone) {
	if bones == nil || bone == nil {
		return
	}
	if !strings.Contains(bone.Name(), "指") {
		return
	}
	if bone.TailIndex < 0 || bone.ParentIndex < 0 {
		return
	}

	tailBone, tailErr := bones.Get(bone.TailIndex)
	parentBone, parentErr := bones.Get(bone.ParentIndex)
	if tailErr != nil || parentErr != nil || tailBone == nil || parentBone == nil {
		return
	}

	axisX := tailBone.Position.Subed(parentBone.Position)
	axisZ := axisX.Cross(mmath.UNIT_Y_NEG_VEC3)
	assignAstanceLocalAxes(bone, axisX, axisZ)
}

// resolveAstanceSideBoneNames は左右別の参照ボーン名を解決する。
func resolveAstanceSideBoneNames(
	boneName string,
) (string, string, string, string, bool) {
	switch {
	case strings.Contains(boneName, "右"):
		return model.ARM.Right(), model.ELBOW.Right(), model.WRIST.Right(), model.MIDDLE1.Right(), true
	case strings.Contains(boneName, "左"):
		return model.ARM.Left(), model.ELBOW.Left(), model.WRIST.Left(), model.MIDDLE1.Left(), true
	default:
		return "", "", "", "", false
	}
}

// assignAstanceLocalAxes は正規化済みローカル軸を設定する。
func assignAstanceLocalAxes(bone *model.Bone, axisX mmath.Vec3, axisZ mmath.Vec3) {
	if bone == nil {
		return
	}
	normalizedAxisX := axisX.Normalized()
	normalizedAxisZ := axisZ.Normalized()
	if normalizedAxisX.Length() <= astanceAxisEpsilon || normalizedAxisZ.Length() <= astanceAxisEpsilon {
		return
	}
	bone.LocalAxisX = normalizedAxisX
	bone.LocalAxisZ = normalizedAxisZ
}

// assignAstanceFixedAxisAndLocalAxes は固定軸とローカル軸を同時に設定する。
func assignAstanceFixedAxisAndLocalAxes(bone *model.Bone, axisX mmath.Vec3, axisZ mmath.Vec3) {
	if bone == nil {
		return
	}
	normalizedAxisX := axisX.Normalized()
	if normalizedAxisX.Length() <= astanceAxisEpsilon {
		return
	}
	bone.FixedAxis = normalizedAxisX
	assignAstanceLocalAxes(bone, normalizedAxisX, axisZ)
}

// applyAstanceVertices はAスタンス補正後の頂点位置と法線を再計算する。
func applyAstanceVertices(
	modelData *ModelData,
	originalPositions map[int]mmath.Vec3,
	transformedBones map[int]astanceBoneTransform,
) {
	if modelData == nil || modelData.Vertices == nil {
		return
	}
	if len(originalPositions) == 0 || len(transformedBones) == 0 {
		return
	}

	for _, vertex := range modelData.Vertices.Values() {
		if vertex == nil || vertex.Deform == nil {
			continue
		}
		originalVertexPos := vertex.Position
		originalVertexNormal := vertex.Normal
		switch vertex.DeformType {
		case model.BDEF1:
			applyAstanceBdef1Vertex(vertex, originalVertexPos, originalVertexNormal, originalPositions, transformedBones)
		case model.BDEF2:
			applyAstanceBdef2Vertex(vertex, originalVertexPos, originalVertexNormal, originalPositions, transformedBones)
		case model.BDEF4:
			applyAstanceBdef4Vertex(vertex, originalVertexPos, originalVertexNormal, originalPositions, transformedBones)
		}
	}
}

// applyAstanceBdef1Vertex はBDEF1頂点へAスタンス補正を適用する。
func applyAstanceBdef1Vertex(
	vertex *model.Vertex,
	originalVertexPos mmath.Vec3,
	originalVertexNormal mmath.Vec3,
	originalPositions map[int]mmath.Vec3,
	transformedBones map[int]astanceBoneTransform,
) {
	if vertex == nil || vertex.Deform == nil {
		return
	}
	indexes := vertex.Deform.Indexes()
	if len(indexes) == 0 {
		return
	}
	boneTransform, exists := transformedBones[indexes[0]]
	if !exists {
		return
	}
	bonePos, posExists := originalPositions[indexes[0]]
	if !posExists {
		return
	}
	vertex.Position = transformAstancePositionByBone(boneTransform, bonePos, originalVertexPos)
	vertex.Normal = transformAstanceNormalByBone(boneTransform, originalVertexNormal)
}

// applyAstanceBdef2Vertex はBDEF2頂点へAスタンス補正を適用する。
func applyAstanceBdef2Vertex(
	vertex *model.Vertex,
	originalVertexPos mmath.Vec3,
	originalVertexNormal mmath.Vec3,
	originalPositions map[int]mmath.Vec3,
	transformedBones map[int]astanceBoneTransform,
) {
	if vertex == nil || vertex.Deform == nil {
		return
	}
	indexes := vertex.Deform.Indexes()
	weights := vertex.Deform.Weights()
	if len(indexes) < astanceMinimumBdef2BoneCount || len(weights) < astanceMinimumBdef2BoneCount {
		return
	}
	transform0, exists0 := transformedBones[indexes[0]]
	transform1, exists1 := transformedBones[indexes[1]]
	if !exists0 || !exists1 {
		return
	}
	bonePos0, posExists0 := originalPositions[indexes[0]]
	bonePos1, posExists1 := originalPositions[indexes[1]]
	if !posExists0 || !posExists1 {
		return
	}

	weight0 := weights[0]
	weight1 := weights[1]

	transformedPos0 := transformAstancePositionByBone(transform0, bonePos0, originalVertexPos)
	transformedPos1 := transformAstancePositionByBone(transform1, bonePos1, originalVertexPos)
	vertex.Position = transformedPos0.MuledScalar(weight0).Added(transformedPos1.MuledScalar(weight1))

	transformedNormal0 := transformAstanceNormalByBone(transform0, originalVertexNormal)
	transformedNormal1 := transformAstanceNormalByBone(transform1, originalVertexNormal)
	mergedNormal := transformedNormal0.MuledScalar(weight0).Added(transformedNormal1.MuledScalar(weight1))
	if mergedNormal.Length() > astanceAxisEpsilon {
		vertex.Normal = mergedNormal.Normalized()
	}
}

// applyAstanceBdef4Vertex はBDEF4頂点へAスタンス補正を適用する。
func applyAstanceBdef4Vertex(
	vertex *model.Vertex,
	originalVertexPos mmath.Vec3,
	originalVertexNormal mmath.Vec3,
	originalPositions map[int]mmath.Vec3,
	transformedBones map[int]astanceBoneTransform,
) {
	if vertex == nil || vertex.Deform == nil {
		return
	}
	indexes := vertex.Deform.Indexes()
	weights := vertex.Deform.Weights()
	if len(indexes) < astanceMinimumBdef4BoneCount || len(weights) < astanceMinimumBdef4BoneCount {
		return
	}

	transformedPos := mmath.ZERO_VEC3
	transformedNormal := mmath.ZERO_VEC3
	for idx := 0; idx < astanceMinimumBdef4BoneCount; idx++ {
		boneIndex := indexes[idx]
		boneTransform, transformExists := transformedBones[boneIndex]
		bonePos, posExists := originalPositions[boneIndex]
		if !transformExists || !posExists {
			return
		}
		weight := weights[idx]
		transformedPos = transformedPos.Added(
			transformAstancePositionByBone(boneTransform, bonePos, originalVertexPos).MuledScalar(weight),
		)
		transformedNormal = transformedNormal.Added(
			transformAstanceNormalByBone(boneTransform, originalVertexNormal).MuledScalar(weight),
		)
	}

	vertex.Position = transformedPos
	if transformedNormal.Length() > astanceAxisEpsilon {
		vertex.Normal = transformedNormal.Normalized()
	}
}

// transformAstancePositionByBone はボーン姿勢で頂点位置を変換する。
func transformAstancePositionByBone(
	boneTransform astanceBoneTransform,
	originalBonePosition mmath.Vec3,
	originalVertexPosition mmath.Vec3,
) mmath.Vec3 {
	relative := originalVertexPosition.Subed(originalBonePosition)
	return boneTransform.Position.Added(boneTransform.Rotation.MulVec3(relative))
}

// transformAstanceNormalByBone はボーン回転で法線を変換する。
func transformAstanceNormalByBone(boneTransform astanceBoneTransform, normal mmath.Vec3) mmath.Vec3 {
	transformed := boneTransform.Rotation.MulVec3(normal)
	if transformed.Length() <= astanceAxisEpsilon {
		return normal
	}
	return transformed.Normalized()
}
