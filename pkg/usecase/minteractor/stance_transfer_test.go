// 指示: miu200521358
package minteractor

import (
	"math"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/domain/mmath"
	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
	"gonum.org/v1/gonum/spatial/r3"
)

func TestApplyAstanceBeforeViewerTransformsArmAndThumbForTstance(t *testing.T) {
	modelData := newBoneMappingTargetModel()
	modelData.VrmData.Profile = vrm.VRM_PROFILE_STANDARD

	if err := applyHumanoidBoneMappingAfterReorder(modelData); err != nil {
		t.Fatalf("bone mapping failed: %v", err)
	}
	setAstanceTestTstanceArms(t, modelData)

	rightArm, rightArmExists := getBoneByName(modelData.Bones, model.ARM.Right())
	leftArm, leftArmExists := getBoneByName(modelData.Bones, model.ARM.Left())
	rightElbow, rightElbowExists := getBoneByName(modelData.Bones, model.ELBOW.Right())
	leftElbow, leftElbowExists := getBoneByName(modelData.Bones, model.ELBOW.Left())
	rightThumb0, rightThumb0Exists := getBoneByName(modelData.Bones, model.THUMB0.Right())
	leftThumb0, leftThumb0Exists := getBoneByName(modelData.Bones, model.THUMB0.Left())
	if !rightArmExists || !leftArmExists || !rightElbowExists || !leftElbowExists || !rightThumb0Exists || !leftThumb0Exists {
		t.Fatalf("required mapped bones are missing")
	}

	rightArmBefore := rightArm.Position
	leftArmBefore := leftArm.Position
	rightElbowBefore := rightElbow.Position
	leftElbowBefore := leftElbow.Position
	rightThumb0Before := rightThumb0.Position
	leftThumb0Before := leftThumb0.Position

	rightVertexIndex := appendAstanceTestVertex(modelData, mmath.Vec3{Vec: r3.Vec{X: -1.3, Y: 14.3, Z: 0.0}}, rightArm.Index())
	leftVertexIndex := appendAstanceTestVertex(modelData, mmath.Vec3{Vec: r3.Vec{X: 1.3, Y: 14.3, Z: 0.0}}, leftArm.Index())
	rightVertexBefore := mustGetVertex(t, modelData, rightVertexIndex).Position
	leftVertexBefore := mustGetVertex(t, modelData, leftVertexIndex).Position

	if err := applyAstanceBeforeViewer(modelData); err != nil {
		t.Fatalf("apply astance failed: %v", err)
	}

	rightArmAfter := rightArm.Position
	leftArmAfter := leftArm.Position
	rightElbowAfter := rightElbow.Position
	leftElbowAfter := leftElbow.Position
	rightThumb0After := rightThumb0.Position
	leftThumb0After := leftThumb0.Position

	if !rightArmAfter.NearEquals(rightArmBefore, 1e-6) {
		t.Fatalf("right arm pivot should keep position: before=%v after=%v", rightArmBefore, rightArmAfter)
	}
	if !leftArmAfter.NearEquals(leftArmBefore, 1e-6) {
		t.Fatalf("left arm pivot should keep position: before=%v after=%v", leftArmBefore, leftArmAfter)
	}
	rightElbowDeltaX := rightElbowAfter.X - rightElbowBefore.X
	leftElbowDeltaX := leftElbowAfter.X - leftElbowBefore.X
	if math.Abs(rightElbowDeltaX) <= 1e-6 || math.Abs(leftElbowDeltaX) <= 1e-6 {
		t.Fatalf("elbow x delta is too small: right=%f left=%f", rightElbowDeltaX, leftElbowDeltaX)
	}
	if rightElbowDeltaX*leftElbowDeltaX >= 0 {
		t.Fatalf("elbow x delta should have opposite sign: right=%f left=%f", rightElbowDeltaX, leftElbowDeltaX)
	}

	if rightThumb0After.NearEquals(rightThumb0Before, 1e-6) {
		t.Fatalf("right thumb0 should be transformed: before=%v after=%v", rightThumb0Before, rightThumb0After)
	}
	if leftThumb0After.NearEquals(leftThumb0Before, 1e-6) {
		t.Fatalf("left thumb0 should be transformed: before=%v after=%v", leftThumb0Before, leftThumb0After)
	}

	rightVertexAfter := mustGetVertex(t, modelData, rightVertexIndex)
	leftVertexAfter := mustGetVertex(t, modelData, leftVertexIndex)
	if rightVertexAfter.Position.NearEquals(rightVertexBefore, 1e-6) {
		t.Fatalf("right arm weighted vertex should move: before=%v after=%v", rightVertexBefore, rightVertexAfter.Position)
	}
	if leftVertexAfter.Position.NearEquals(leftVertexBefore, 1e-6) {
		t.Fatalf("left arm weighted vertex should move: before=%v after=%v", leftVertexBefore, leftVertexAfter.Position)
	}
	if math.Abs(rightVertexAfter.Normal.Length()-1.0) > 1e-6 {
		t.Fatalf("right vertex normal should be normalized: normal=%v length=%f", rightVertexAfter.Normal, rightVertexAfter.Normal.Length())
	}
	if math.Abs(leftVertexAfter.Normal.Length()-1.0) > 1e-6 {
		t.Fatalf("left vertex normal should be normalized: normal=%v length=%f", leftVertexAfter.Normal, leftVertexAfter.Normal.Length())
	}
}

func TestApplyAstanceBeforeViewerSkipsWhenNotTstance(t *testing.T) {
	modelData := newBoneMappingTargetModel()
	modelData.VrmData.Profile = vrm.VRM_PROFILE_STANDARD

	if err := applyHumanoidBoneMappingAfterReorder(modelData); err != nil {
		t.Fatalf("bone mapping failed: %v", err)
	}

	rightArm, rightArmExists := getBoneByName(modelData.Bones, model.ARM.Right())
	if !rightArmExists {
		t.Fatalf("required mapped bone is missing: %s", model.ARM.Right())
	}
	rightArmBefore := rightArm.Position

	rightVertexIndex := appendAstanceTestVertex(modelData, mmath.Vec3{Vec: r3.Vec{X: -1.3, Y: 14.3, Z: 0.0}}, rightArm.Index())
	rightVertexBefore := mustGetVertex(t, modelData, rightVertexIndex).Position

	if err := applyAstanceBeforeViewer(modelData); err != nil {
		t.Fatalf("apply astance failed: %v", err)
	}

	if !rightArm.Position.NearEquals(rightArmBefore, 1e-6) {
		t.Fatalf("right arm should not change for non-t stance: before=%v after=%v", rightArmBefore, rightArm.Position)
	}
	rightVertexAfter := mustGetVertex(t, modelData, rightVertexIndex)
	if !rightVertexAfter.Position.NearEquals(rightVertexBefore, 1e-6) {
		t.Fatalf("vertex should not change for non-t stance: before=%v after=%v", rightVertexBefore, rightVertexAfter.Position)
	}
}

func TestApplyAstanceBeforeViewerSkipsNonTstanceEvenVroidProfile(t *testing.T) {
	modelData := newBoneMappingTargetModel()
	modelData.VrmData.Profile = vrm.VRM_PROFILE_VROID

	if err := applyHumanoidBoneMappingAfterReorder(modelData); err != nil {
		t.Fatalf("bone mapping failed: %v", err)
	}

	rightArm, rightArmExists := getBoneByName(modelData.Bones, model.ARM.Right())
	if !rightArmExists {
		t.Fatalf("required mapped bone is missing: %s", model.ARM.Right())
	}
	rightArmBefore := rightArm.Position

	if err := applyAstanceBeforeViewer(modelData); err != nil {
		t.Fatalf("apply astance failed: %v", err)
	}

	if !rightArm.Position.NearEquals(rightArmBefore, 1e-6) {
		t.Fatalf("right arm should not change for non-t stance even vroid profile: before=%v after=%v", rightArmBefore, rightArm.Position)
	}
}

// setAstanceTestTstanceArms は左右腕を水平姿勢へ調整する。
func setAstanceTestTstanceArms(t *testing.T, modelData *ModelData) {
	t.Helper()
	if modelData == nil || modelData.Bones == nil {
		t.Fatalf("model or bones is nil")
	}

	leftArm, leftArmExists := getBoneByName(modelData.Bones, model.ARM.Left())
	leftElbow, leftElbowExists := getBoneByName(modelData.Bones, model.ELBOW.Left())
	leftWrist, leftWristExists := getBoneByName(modelData.Bones, model.WRIST.Left())
	rightArm, rightArmExists := getBoneByName(modelData.Bones, model.ARM.Right())
	rightElbow, rightElbowExists := getBoneByName(modelData.Bones, model.ELBOW.Right())
	rightWrist, rightWristExists := getBoneByName(modelData.Bones, model.WRIST.Right())
	if !leftArmExists || !leftElbowExists || !leftWristExists || !rightArmExists || !rightElbowExists || !rightWristExists {
		t.Fatalf("required mapped arm bones are missing")
	}

	leftUpperLength := leftElbow.Position.Subed(leftArm.Position).Length()
	leftLowerLength := leftWrist.Position.Subed(leftElbow.Position).Length()
	rightUpperLength := rightElbow.Position.Subed(rightArm.Position).Length()
	rightLowerLength := rightWrist.Position.Subed(rightElbow.Position).Length()

	leftElbow.Position = leftArm.Position.Added(mmath.Vec3{Vec: r3.Vec{X: leftUpperLength, Y: 0, Z: 0}})
	leftWrist.Position = leftElbow.Position.Added(mmath.Vec3{Vec: r3.Vec{X: leftLowerLength, Y: 0, Z: 0}})
	rightElbow.Position = rightArm.Position.Added(mmath.Vec3{Vec: r3.Vec{X: -rightUpperLength, Y: 0, Z: 0}})
	rightWrist.Position = rightElbow.Position.Added(mmath.Vec3{Vec: r3.Vec{X: -rightLowerLength, Y: 0, Z: 0}})
}

// appendAstanceTestVertex は指定ボーンBDEF1の検証頂点を追加してindexを返す。
func appendAstanceTestVertex(modelData *ModelData, position mmath.Vec3, boneIndex int) int {
	return modelData.Vertices.AppendRaw(&model.Vertex{
		Position:   position,
		Normal:     mmath.UNIT_Y_VEC3,
		Uv:         mmath.ZERO_VEC2,
		DeformType: model.BDEF1,
		Deform:     model.NewBdef1(boneIndex),
		EdgeFactor: 1.0,
	})
}

// mustGetVertex は指定indexの頂点を取得し、取得失敗時はテストを中断する。
func mustGetVertex(t *testing.T, modelData *ModelData, index int) *model.Vertex {
	t.Helper()
	if modelData == nil || modelData.Vertices == nil {
		t.Fatalf("model or vertices is nil")
	}
	vertex, err := modelData.Vertices.Get(index)
	if err != nil || vertex == nil {
		t.Fatalf("vertex not found: index=%d err=%v", index, err)
	}
	return vertex
}
