// 指示: miu200521358
package messages

import "testing"

func TestEdgeAcceptanceAndStatsKeysAreDefined(t *testing.T) {
	keys := []string{
		LogMaterialReorderWarnEdgeOffsetTuning,
		LogMaterialReorderInfoEdgeOffsetStats,
		LogVrmInfoMaterialStatsIoModel,
		EdgeAcceptanceJudgePass,
		EdgeAcceptanceJudgeFail,
		EdgeAcceptanceReasonNone,
		EdgeAcceptanceReasonPrimaryP50Below,
		EdgeAcceptanceReasonPrimaryP95Below,
		EdgeAcceptanceReasonPrimaryCoincident,
		EdgeAcceptanceReasonComparisonCurrent,
		EdgeAcceptanceReasonComparisonBaseline,
		EdgeAcceptanceReasonBaselineP50NonPos,
		EdgeAcceptanceReasonComparisonP50Drop,
	}

	seen := map[string]struct{}{}
	for _, key := range keys {
		if key == "" {
			t.Fatalf("key should not be empty")
		}
		if _, exists := seen[key]; exists {
			t.Fatalf("key should be unique: %s", key)
		}
		seen[key] = struct{}{}
	}
}
