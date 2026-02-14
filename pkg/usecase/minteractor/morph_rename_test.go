// 指示: miu200521358
package minteractor

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/miu200521358/mlib_go/pkg/domain/model"
	"github.com/miu200521358/mlib_go/pkg/domain/model/vrm"
	"github.com/miu200521358/mlib_go/pkg/infra/base/mlogging"
	"github.com/miu200521358/mlib_go/pkg/shared/base/logging"
)

// morphRenameProgressCollector はモーフrename進捗イベント収集を表す。
type morphRenameProgressCollector struct {
	events []PrepareProgressEvent
}

// ReportPrepareProgress は進捗イベントを収集する。
func (c *morphRenameProgressCollector) ReportPrepareProgress(event PrepareProgressEvent) {
	if c == nil {
		return
	}
	c.events = append(c.events, event)
}

// findEventIndex は指定種別イベントの先頭indexを返す。
func (c *morphRenameProgressCollector) findEventIndex(target PrepareProgressEventType) int {
	if c == nil {
		return -1
	}
	for idx, event := range c.events {
		if event.Type == target {
			return idx
		}
	}
	return -1
}

// findEventByType は指定種別イベントを返す。
func (c *morphRenameProgressCollector) findEventByType(target PrepareProgressEventType) (PrepareProgressEvent, bool) {
	if c == nil {
		return PrepareProgressEvent{}, false
	}
	for _, event := range c.events {
		if event.Type == target {
			return event, true
		}
	}
	return PrepareProgressEvent{}, false
}

// TestApplyMorphRenameOnlyBeforeViewerRenamesAndReportsProgress は名称移植と進捗通知を検証する。
func TestApplyMorphRenameOnlyBeforeViewerRenamesAndReportsProgress(t *testing.T) {
	modelData := model.NewPmxModel()
	appendMorphForRenameTest(modelData, "Fcl_MTH_Small", model.MORPH_PANEL_SYSTEM, "Fcl_MTH_Small_en")
	appendMorphForRenameTest(modelData, "mouthPucker", model.MORPH_PANEL_SYSTEM, "mouthPucker_en")
	appendMorphForRenameTest(modelData, "UnknownMorph", model.MORPH_PANEL_OTHER_LOWER_RIGHT, "UnknownMorph_en")

	reporter := &morphRenameProgressCollector{}
	summary := applyMorphRenameOnlyBeforeViewer(modelData, reporter)

	if summary.Targets != 3 {
		t.Fatalf("targets mismatch: got=%d want=3", summary.Targets)
	}
	if summary.Mappings != len(morphRenameOnlyRules) {
		t.Fatalf("mappings mismatch: got=%d want=%d", summary.Mappings, len(morphRenameOnlyRules))
	}
	if summary.Processed != 3 {
		t.Fatalf("processed mismatch: got=%d want=3", summary.Processed)
	}
	if summary.Renamed != 2 {
		t.Fatalf("renamed mismatch: got=%d want=2", summary.Renamed)
	}
	if summary.Unchanged != 1 {
		t.Fatalf("unchanged mismatch: got=%d want=1", summary.Unchanged)
	}
	if summary.NotFound != len(morphRenameOnlyRules)-2 {
		t.Fatalf("notFound mismatch: got=%d want=%d", summary.NotFound, len(morphRenameOnlyRules)-2)
	}

	renamedSmall, err := modelData.Morphs.GetByName("うー")
	if err != nil || renamedSmall == nil {
		t.Fatalf("renamed morph うー not found: err=%v", err)
	}
	if renamedSmall.Panel != model.MORPH_PANEL_LIP_UPPER_RIGHT {
		t.Fatalf("panel mismatch for うー: got=%d want=%d", renamedSmall.Panel, model.MORPH_PANEL_LIP_UPPER_RIGHT)
	}
	if renamedSmall.EnglishName != "うー" {
		t.Fatalf("english name mismatch for うー: got=%s want=うー", renamedSmall.EnglishName)
	}

	renamedPucker, err := modelData.Morphs.GetByName("うう")
	if err != nil || renamedPucker == nil {
		t.Fatalf("renamed morph うう not found: err=%v", err)
	}
	if renamedPucker.Panel != model.MORPH_PANEL_LIP_UPPER_RIGHT {
		t.Fatalf("panel mismatch for うう: got=%d want=%d", renamedPucker.Panel, model.MORPH_PANEL_LIP_UPPER_RIGHT)
	}
	if renamedPucker.EnglishName != "うう" {
		t.Fatalf("english name mismatch for うう: got=%s want=うう", renamedPucker.EnglishName)
	}

	if _, err := modelData.Morphs.GetByName("Fcl_MTH_Small"); err == nil {
		t.Fatalf("source morph Fcl_MTH_Small should be renamed")
	}
	if _, err := modelData.Morphs.GetByName("mouthPucker"); err == nil {
		t.Fatalf("source morph mouthPucker should be renamed")
	}

	unknown, err := modelData.Morphs.GetByName("UnknownMorph")
	if err != nil || unknown == nil {
		t.Fatalf("unknown morph should stay: err=%v", err)
	}
	if unknown.EnglishName != "UnknownMorph_en" {
		t.Fatalf("unknown english name should stay: got=%s", unknown.EnglishName)
	}

	plannedIdx := reporter.findEventIndex(PrepareProgressEventTypeMorphRenamePlanned)
	processedIdx := reporter.findEventIndex(PrepareProgressEventTypeMorphRenameProcessed)
	completedIdx := reporter.findEventIndex(PrepareProgressEventTypeMorphRenameCompleted)
	if plannedIdx < 0 || processedIdx < 0 || completedIdx < 0 {
		t.Fatalf("morph rename events should be reported: planned=%d processed=%d completed=%d", plannedIdx, processedIdx, completedIdx)
	}
	if !(plannedIdx < processedIdx && processedIdx < completedIdx) {
		t.Fatalf("morph rename event order mismatch: planned=%d processed=%d completed=%d", plannedIdx, processedIdx, completedIdx)
	}

	plannedEvent, ok := reporter.findEventByType(PrepareProgressEventTypeMorphRenamePlanned)
	if !ok || plannedEvent.MorphCount != 3 {
		t.Fatalf("planned event MorphCount mismatch: %+v", plannedEvent)
	}
	processedEvent, ok := reporter.findEventByType(PrepareProgressEventTypeMorphRenameProcessed)
	if !ok || processedEvent.MorphCount != 3 {
		t.Fatalf("processed event MorphCount mismatch: %+v", processedEvent)
	}
}

// TestApplyMorphRenameOnlyBeforeViewerKeepsNameOnConflict は名前衝突時に継続することを検証する。
func TestApplyMorphRenameOnlyBeforeViewerKeepsNameOnConflict(t *testing.T) {
	modelData := model.NewPmxModel()
	appendMorphForRenameTest(modelData, "うー", model.MORPH_PANEL_OTHER_LOWER_RIGHT, "already_u")
	appendMorphForRenameTest(modelData, "Fcl_MTH_Small", model.MORPH_PANEL_SYSTEM, "small_en")

	summary := applyMorphRenameOnlyBeforeViewer(modelData, &morphRenameProgressCollector{})

	if summary.Targets != 2 {
		t.Fatalf("targets mismatch: got=%d want=2", summary.Targets)
	}
	if summary.Renamed != 1 {
		t.Fatalf("renamed mismatch: got=%d want=1", summary.Renamed)
	}
	if summary.Unchanged != 1 {
		t.Fatalf("unchanged mismatch: got=%d want=1", summary.Unchanged)
	}

	sourceMorph, err := modelData.Morphs.GetByName("Fcl_MTH_Small")
	if err != nil || sourceMorph == nil {
		t.Fatalf("source morph should remain on conflict: err=%v", err)
	}
	if sourceMorph.Panel != model.MORPH_PANEL_LIP_UPPER_RIGHT {
		t.Fatalf("panel should still be updated on conflict: got=%d want=%d", sourceMorph.Panel, model.MORPH_PANEL_LIP_UPPER_RIGHT)
	}
	if sourceMorph.EnglishName != "small_en" {
		t.Fatalf("english name should remain when rename failed: got=%s want=small_en", sourceMorph.EnglishName)
	}
}

// TestApplyMorphRenameOnlyBeforeViewerDoesNotOutputDebugDetailAtInfo はINFOレベルで詳細DEBUGが出ないことを検証する。
func TestApplyMorphRenameOnlyBeforeViewerDoesNotOutputDebugDetailAtInfo(t *testing.T) {
	logger := mlogging.NewLogger(nil)
	logger.SetLevel(logging.LOG_LEVEL_INFO)
	logger.MessageBuffer().Clear()
	prevLogger := logging.DefaultLogger()
	logging.SetDefaultLogger(logger)
	t.Cleanup(func() {
		logging.SetDefaultLogger(prevLogger)
	})

	modelData := model.NewPmxModel()
	appendMorphForRenameTest(modelData, "Fcl_MTH_Small", model.MORPH_PANEL_SYSTEM, "Fcl_MTH_Small_en")
	appendMorphForRenameTest(modelData, "UnknownMorph", model.MORPH_PANEL_OTHER_LOWER_RIGHT, "UnknownMorph_en")
	applyMorphRenameOnlyBeforeViewer(modelData, &morphRenameProgressCollector{})

	lines := logger.MessageBuffer().Lines()
	for _, line := range lines {
		if strings.Contains(line, "モーフ名称一覧:") || strings.Contains(line, "モーフ名称変換詳細:") {
			t.Fatalf("debug detail should not be output at info level: line=%s", line)
		}
	}
}

// TestApplyMorphRenameOnlyBeforeViewerOutputsModelMorphListAtDebug はDEBUGレベルでモデルモーフ一覧が出ることを検証する。
func TestApplyMorphRenameOnlyBeforeViewerOutputsModelMorphListAtDebug(t *testing.T) {
	logger := mlogging.NewLogger(nil)
	logger.SetLevel(logging.LOG_LEVEL_DEBUG)
	logger.MessageBuffer().Clear()
	prevLogger := logging.DefaultLogger()
	logging.SetDefaultLogger(logger)
	t.Cleanup(func() {
		logging.SetDefaultLogger(prevLogger)
	})

	modelData := model.NewPmxModel()
	appendMorphForRenameTest(modelData, "Fcl_MTH_Small", model.MORPH_PANEL_SYSTEM, "Fcl_MTH_Small_en")
	appendMorphForRenameTest(modelData, "UnknownMorph", model.MORPH_PANEL_OTHER_LOWER_RIGHT, "UnknownMorph_en")
	applyMorphRenameOnlyBeforeViewer(modelData, &morphRenameProgressCollector{})

	lines := logger.MessageBuffer().Lines()
	hasModelListStart := false
	hasModelListEntry := false
	hasRenameDetail := false
	hasRuleCoverageDetail := false
	for _, line := range lines {
		if strings.Contains(line, "モーフ名称一覧開始: count=2") {
			hasModelListStart = true
		}
		if strings.Contains(line, "モーフ名称一覧: index=0 name=Fcl_MTH_Small") {
			hasModelListEntry = true
		}
		if strings.Contains(line, "モーフ名称変換詳細: index=0 source=Fcl_MTH_Small target=うー") {
			hasRenameDetail = true
		}
		if strings.Contains(line, "モーフ名称変換マッピング詳細:") {
			hasRuleCoverageDetail = true
		}
	}
	if !hasModelListStart {
		t.Fatal("model morph list start debug log missing")
	}
	if !hasModelListEntry {
		t.Fatal("model morph list entry debug log missing")
	}
	if !hasRenameDetail {
		t.Fatal("rename detail debug log missing")
	}
	if hasRuleCoverageDetail {
		t.Fatal("rule coverage debug log should not be output")
	}
}

// TestApplyMorphRenameOnlyBeforeViewerSkipsWhenVrmExpressionsDefined はVRM表情定義がある場合にrename-onlyを実行しないことを検証する。
func TestApplyMorphRenameOnlyBeforeViewerSkipsWhenVrmExpressionsDefined(t *testing.T) {
	modelData := model.NewPmxModel()
	appendMorphForRenameTest(modelData, "Fcl_MTH_Small", model.MORPH_PANEL_SYSTEM, "Fcl_MTH_Small_en")
	modelData.VrmData = vrm.NewVrmData()
	rawExpression, err := json.Marshal(map[string]any{
		"expressions": map[string]any{
			"custom": map[string]any{
				"Fcl_MTH_Small": map[string]any{
					"morphTargetBinds": []any{},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to build vrm expression json: %v", err)
	}
	modelData.VrmData.RawExtensions["VRMC_vrm"] = rawExpression

	reporter := &morphRenameProgressCollector{}
	summary := applyMorphRenameOnlyBeforeViewer(modelData, reporter)
	if summary.Processed != 0 {
		t.Fatalf("processed should be zero when skipped: got=%d", summary.Processed)
	}
	if summary.Renamed != 0 {
		t.Fatalf("renamed should be zero when skipped: got=%d", summary.Renamed)
	}

	if _, err := modelData.Morphs.GetByName("うー"); err == nil {
		t.Fatal("rename-only should be skipped, but うー exists")
	}
	originalMorph, err := modelData.Morphs.GetByName("Fcl_MTH_Small")
	if err != nil || originalMorph == nil {
		t.Fatalf("original morph should remain: err=%v", err)
	}
	if originalMorph.EnglishName != "Fcl_MTH_Small_en" {
		t.Fatalf("english name should remain: got=%s", originalMorph.EnglishName)
	}

	plannedEvent, ok := reporter.findEventByType(PrepareProgressEventTypeMorphRenamePlanned)
	if !ok || plannedEvent.MorphCount != 0 {
		t.Fatalf("planned event should be zero on skip: %+v", plannedEvent)
	}
	if _, ok := reporter.findEventByType(PrepareProgressEventTypeMorphRenameCompleted); !ok {
		t.Fatal("completed event should be reported on skip")
	}
}

// appendMorphForRenameTest はテスト用モーフを追加する。
func appendMorphForRenameTest(modelData *ModelData, name string, panel model.MorphPanel, englishName string) int {
	morphData := &model.Morph{
		Panel:     panel,
		MorphType: model.MORPH_TYPE_VERTEX,
	}
	morphData.SetName(name)
	morphData.EnglishName = englishName
	if modelData == nil || modelData.Morphs == nil {
		return -1
	}
	return modelData.Morphs.AppendRaw(morphData)
}
