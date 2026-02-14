// 指示: miu200521358
package minteractor

import (
	"sort"
	"strings"

	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
	"gonum.org/v1/gonum/spatial/r3"
)

const (
	astanceRightArmRollDegree    = 35.0
	astanceLeftArmRollDegree     = -35.0
	astanceRightThumb0YawDegree  = 8.0
	astanceRightThumb1YawDegree  = 24.0
	astanceLeftThumb0YawDegree   = -8.0
	astanceLeftThumb1YawDegree   = -24.0
	astanceAxisEpsilon           = 1e-8
	astanceMinimumBdef2BoneCount = 2
	astanceMinimumBdef4BoneCount = 4
)

// astanceChainBone は上半身起点チェーンの1要素を表す。
type astanceChainBone struct {
	BoneIndex      int
	BoneName       string
	RelativeVector mmath.Vec3
}

// astanceRotationSpec は片側チェーンに適用する回転対象を表す。
type astanceRotationSpec struct {
	ArmBoneName    string
	ArmRotation    mmath.Quaternion
	Thumb0BoneName string
	Thumb0Rotation mmath.Quaternion
	Thumb1BoneName string
	Thumb1Rotation mmath.Quaternion
}

// applyAstanceBeforeViewer はVRoidプロファイル向けにTスタンスをAスタンスへ補正する。
func applyAstanceBeforeViewer(modelData *ModelData) error {
	if !shouldApplyAstance(modelData) {
		return nil
	}
	if modelData.Bones == nil || modelData.Bones.Len() == 0 {
		return nil
	}

	originalPositions := collectOriginalBonePositions(modelData.Bones)
	transformedPositions, transformedMatrices := collectAstanceTransformedBones(modelData.Bones, originalPositions)
	if len(transformedPositions) == 0 || len(transformedMatrices) == 0 {
		return nil
	}

	applyAstanceBonePositions(modelData.Bones, transformedPositions)
	updateAstanceBoneLocalAxes(modelData.Bones, transformedMatrices)
	applyAstanceVertices(modelData, originalPositions, transformedMatrices)

	return nil
}

// shouldApplyAstance はAスタンス補正適用可否を返す。
func shouldApplyAstance(modelData *ModelData) bool {
	if modelData == nil || modelData.VrmData == nil {
		return false
	}
	return modelData.VrmData.Profile == vrm.VRM_PROFILE_VROID
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

// collectAstanceTransformedBones はAスタンス適用後のボーン位置と行列を収集する。
func collectAstanceTransformedBones(
	bones *model.BoneCollection,
	originalPositions map[int]mmath.Vec3,
) (map[int]mmath.Vec3, map[int]mmath.Mat4) {
	transformedPositions := map[int]mmath.Vec3{}
	transformedMatrices := map[int]mmath.Mat4{}
	if bones == nil {
		return transformedPositions, transformedMatrices
	}

	rootBoneName := model.UPPER.String()
	targetBoneNames := buildAstanceTargetBoneNames(bones)
	for _, endBoneName := range targetBoneNames {
		chain, exists := buildAstanceChainFromRoot(bones, endBoneName, rootBoneName, originalPositions)
		if !exists || len(chain) == 0 {
			continue
		}

		rotationSpec := resolveAstanceRotationSpec(chain)
		mat := mmath.NewMat4()
		for _, chainBone := range chain {
			mat.Translate(chainBone.RelativeVector)
			if rotationSpec.ArmBoneName != "" && chainBone.BoneName == rotationSpec.ArmBoneName {
				mat.Rotate(rotationSpec.ArmRotation)
			} else if rotationSpec.Thumb0BoneName != "" && chainBone.BoneName == rotationSpec.Thumb0BoneName {
				mat.Rotate(rotationSpec.Thumb0Rotation)
			} else if rotationSpec.Thumb1BoneName != "" && chainBone.BoneName == rotationSpec.Thumb1BoneName {
				mat.Rotate(rotationSpec.Thumb1Rotation)
			}

			if _, exists := transformedPositions[chainBone.BoneIndex]; exists {
				continue
			}
			transformedPositions[chainBone.BoneIndex] = mat.MulVec3(mmath.ZERO_VEC3)
			transformedMatrices[chainBone.BoneIndex] = mat
		}
	}

	return transformedPositions, transformedMatrices
}

// buildAstanceTargetBoneNames は旧参考実装相当のチェーン終端ボーン名一覧を生成する。
func buildAstanceTargetBoneNames(bones *model.BoneCollection) []string {
	names := []string{model.HEAD.String()}
	names = appendAstanceDirectionTargetBoneNames(names, model.BONE_DIRECTION_RIGHT)
	names = appendAstanceDirectionTargetBoneNames(names, model.BONE_DIRECTION_LEFT)

	if bones != nil {
		for _, bone := range bones.Values() {
			if bone == nil {
				continue
			}
			if strings.Contains(bone.Name(), "装飾_") {
				names = append(names, bone.Name())
			}
		}
	}

	return uniqueBoneNamesInOrder(names)
}

// appendAstanceDirectionTargetBoneNames は左右別の終端ボーン名を追加する。
func appendAstanceDirectionTargetBoneNames(names []string, direction model.BoneDirection) []string {
	result := append([]string(nil), names...)
	switch direction {
	case model.BONE_DIRECTION_RIGHT:
		result = append(result,
			rightThumbTipName,
			rightIndexTipName,
			rightMiddleTipName,
			rightRingTipName,
			rightPinkyTipName,
			"右胸先",
			model.ARM_TWIST1.Right(),
			model.ARM_TWIST2.Right(),
			model.ARM_TWIST3.Right(),
			model.WRIST_TWIST1.Right(),
			model.WRIST_TWIST2.Right(),
			model.WRIST_TWIST3.Right(),
		)
	case model.BONE_DIRECTION_LEFT:
		result = append(result,
			leftThumbTipName,
			leftIndexTipName,
			leftMiddleTipName,
			leftRingTipName,
			leftPinkyTipName,
			"左胸先",
			model.ARM_TWIST1.Left(),
			model.ARM_TWIST2.Left(),
			model.ARM_TWIST3.Left(),
			model.WRIST_TWIST1.Left(),
			model.WRIST_TWIST2.Left(),
			model.WRIST_TWIST3.Left(),
		)
	}
	return result
}

// uniqueBoneNamesInOrder は重複を除去しつつ先頭出現順を維持する。
func uniqueBoneNamesInOrder(names []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		unique = append(unique, trimmed)
	}
	return unique
}

// buildAstanceChainFromRoot は指定終端から指定起点までのチェーンを生成する。
func buildAstanceChainFromRoot(
	bones *model.BoneCollection,
	endBoneName string,
	rootBoneName string,
	originalPositions map[int]mmath.Vec3,
) ([]astanceChainBone, bool) {
	if bones == nil {
		return nil, false
	}
	endBone, exists := getBoneByName(bones, endBoneName)
	if !exists {
		return nil, false
	}

	reversedIndexes := make([]int, 0)
	currentBone := endBone
	foundRoot := false
	for currentBone != nil {
		reversedIndexes = append(reversedIndexes, currentBone.Index())
		if currentBone.Name() == rootBoneName {
			foundRoot = true
			break
		}
		if currentBone.ParentIndex < 0 {
			break
		}
		parentBone, err := bones.Get(currentBone.ParentIndex)
		if err != nil || parentBone == nil {
			break
		}
		currentBone = parentBone
	}
	if !foundRoot {
		return nil, false
	}

	for left, right := 0, len(reversedIndexes)-1; left < right; left, right = left+1, right-1 {
		reversedIndexes[left], reversedIndexes[right] = reversedIndexes[right], reversedIndexes[left]
	}

	chain := make([]astanceChainBone, 0, len(reversedIndexes))
	for idx, boneIndex := range reversedIndexes {
		bone, err := bones.Get(boneIndex)
		if err != nil || bone == nil {
			return nil, false
		}
		bonePos, bonePosExists := originalPositions[boneIndex]
		if !bonePosExists {
			return nil, false
		}
		relativeVector := bonePos
		if idx > 0 {
			parentPos, parentPosExists := originalPositions[reversedIndexes[idx-1]]
			if !parentPosExists {
				return nil, false
			}
			relativeVector = bonePos.Subed(parentPos)
		}
		chain = append(chain, astanceChainBone{
			BoneIndex:      bone.Index(),
			BoneName:       bone.Name(),
			RelativeVector: relativeVector,
		})
	}

	return chain, true
}

// resolveAstanceRotationSpec はチェーンに対応する左右回転条件を返す。
func resolveAstanceRotationSpec(chain []astanceChainBone) astanceRotationSpec {
	hasRight := false
	hasLeft := false
	for _, chainBone := range chain {
		if strings.Contains(chainBone.BoneName, "右") {
			hasRight = true
		}
		if strings.Contains(chainBone.BoneName, "左") {
			hasLeft = true
		}
	}

	if hasRight {
		return astanceRotationSpec{
			ArmBoneName:    model.ARM.Right(),
			ArmRotation:    mmath.NewQuaternionFromDegrees(0, 0, astanceRightArmRollDegree),
			Thumb0BoneName: model.THUMB0.Right(),
			Thumb0Rotation: mmath.NewQuaternionFromDegrees(0, astanceRightThumb0YawDegree, 0),
			Thumb1BoneName: model.THUMB1.Right(),
			Thumb1Rotation: mmath.NewQuaternionFromDegrees(0, astanceRightThumb1YawDegree, 0),
		}
	}
	if hasLeft {
		return astanceRotationSpec{
			ArmBoneName:    model.ARM.Left(),
			ArmRotation:    mmath.NewQuaternionFromDegrees(0, 0, astanceLeftArmRollDegree),
			Thumb0BoneName: model.THUMB0.Left(),
			Thumb0Rotation: mmath.NewQuaternionFromDegrees(0, astanceLeftThumb0YawDegree, 0),
			Thumb1BoneName: model.THUMB1.Left(),
			Thumb1Rotation: mmath.NewQuaternionFromDegrees(0, astanceLeftThumb1YawDegree, 0),
		}
	}
	return astanceRotationSpec{}
}

// applyAstanceBonePositions は補正後ボーン位置をモデルへ反映する。
func applyAstanceBonePositions(bones *model.BoneCollection, transformedPositions map[int]mmath.Vec3) {
	if bones == nil || len(transformedPositions) == 0 {
		return
	}
	for boneIndex, transformedPosition := range transformedPositions {
		bone, err := bones.Get(boneIndex)
		if err != nil || bone == nil {
			continue
		}
		bone.Position = transformedPosition
	}
}

// updateAstanceBoneLocalAxes はAスタンス補正後のローカル軸を再計算する。
func updateAstanceBoneLocalAxes(bones *model.BoneCollection, transformedMatrices map[int]mmath.Mat4) {
	if bones == nil || len(transformedMatrices) == 0 {
		return
	}
	targetIndexes := make([]int, 0, len(transformedMatrices))
	for boneIndex := range transformedMatrices {
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
	transformedMatrices map[int]mmath.Mat4,
) {
	if modelData == nil || modelData.Vertices == nil {
		return
	}
	if len(originalPositions) == 0 || len(transformedMatrices) == 0 {
		return
	}

	for _, vertex := range modelData.Vertices.Values() {
		if vertex == nil || vertex.Deform == nil {
			continue
		}
		switch vertex.DeformType {
		case model.BDEF1:
			applyAstanceBdef1Vertex(vertex, originalPositions, transformedMatrices)
		case model.BDEF2:
			applyAstanceBdef2Vertex(vertex, originalPositions, transformedMatrices)
		case model.BDEF4:
			applyAstanceBdef4Vertex(vertex, originalPositions, transformedMatrices)
		}
	}
}

// applyAstanceBdef1Vertex はBDEF1頂点へAスタンス補正を適用する。
func applyAstanceBdef1Vertex(
	vertex *model.Vertex,
	originalPositions map[int]mmath.Vec3,
	transformedMatrices map[int]mmath.Mat4,
) {
	if vertex == nil || vertex.Deform == nil {
		return
	}
	indexes := vertex.Deform.Indexes()
	if len(indexes) == 0 {
		return
	}
	boneMatrix, exists := transformedMatrices[indexes[0]]
	if !exists {
		return
	}
	bonePos, posExists := originalPositions[indexes[0]]
	if !posExists {
		return
	}
	relativePos := vertex.Position.Subed(bonePos)
	vertex.Position = boneMatrix.MulVec3(relativePos)
	vertex.Normal = transformAstanceNormalByMatrix(boneMatrix, vertex.Normal)
}

// applyAstanceBdef2Vertex はBDEF2頂点へAスタンス補正を適用する。
func applyAstanceBdef2Vertex(
	vertex *model.Vertex,
	originalPositions map[int]mmath.Vec3,
	transformedMatrices map[int]mmath.Mat4,
) {
	if vertex == nil || vertex.Deform == nil {
		return
	}
	indexes := vertex.Deform.Indexes()
	weights := vertex.Deform.Weights()
	if len(indexes) < astanceMinimumBdef2BoneCount || len(weights) < astanceMinimumBdef2BoneCount {
		return
	}
	matrix0, exists0 := transformedMatrices[indexes[0]]
	matrix1, exists1 := transformedMatrices[indexes[1]]
	if !exists0 || !exists1 {
		return
	}
	bonePos0, posExists0 := originalPositions[indexes[0]]
	bonePos1, posExists1 := originalPositions[indexes[1]]
	if !posExists0 || !posExists1 {
		return
	}

	relativePos0 := vertex.Position.Subed(bonePos0)
	relativePos1 := vertex.Position.Subed(bonePos1)
	weight0 := weights[0]
	weight1 := weights[1]

	transformedPos0 := matrix0.MulVec3(relativePos0)
	transformedPos1 := matrix1.MulVec3(relativePos1)
	vertex.Position = transformedPos0.MuledScalar(weight0).Added(transformedPos1.MuledScalar(weight1))

	transformedNormal0 := transformAstanceNormalByMatrix(matrix0, vertex.Normal)
	transformedNormal1 := transformAstanceNormalByMatrix(matrix1, vertex.Normal)
	mergedNormal := transformedNormal0.MuledScalar(weight0).Added(transformedNormal1.MuledScalar(weight1))
	if mergedNormal.Length() > astanceAxisEpsilon {
		vertex.Normal = mergedNormal.Normalized()
	}
}

// applyAstanceBdef4Vertex はBDEF4頂点へAスタンス補正を適用する。
func applyAstanceBdef4Vertex(
	vertex *model.Vertex,
	originalPositions map[int]mmath.Vec3,
	transformedMatrices map[int]mmath.Mat4,
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
		boneMatrix, matrixExists := transformedMatrices[boneIndex]
		bonePos, posExists := originalPositions[boneIndex]
		if !matrixExists || !posExists {
			return
		}
		weight := weights[idx]
		relativePos := vertex.Position.Subed(bonePos)
		transformedPos = transformedPos.Added(boneMatrix.MulVec3(relativePos).MuledScalar(weight))
		transformedNormal = transformedNormal.Added(transformAstanceNormalByMatrix(boneMatrix, vertex.Normal).MuledScalar(weight))
	}

	vertex.Position = transformedPos
	if transformedNormal.Length() > astanceAxisEpsilon {
		vertex.Normal = transformedNormal.Normalized()
	}
}

// transformAstanceNormalByMatrix は平行移動を除いた3x3成分で法線を変換する。
func transformAstanceNormalByMatrix(matrix mmath.Mat4, normal mmath.Vec3) mmath.Vec3 {
	transformed := mmath.Vec3{Vec: r3.Vec{
		X: normal.X*matrix[0] + normal.Y*matrix[4] + normal.Z*matrix[8],
		Y: normal.X*matrix[1] + normal.Y*matrix[5] + normal.Z*matrix[9],
		Z: normal.X*matrix[2] + normal.Y*matrix[6] + normal.Z*matrix[10],
	}}
	if transformed.Length() <= astanceAxisEpsilon {
		return normal
	}
	return transformed.Normalized()
}
