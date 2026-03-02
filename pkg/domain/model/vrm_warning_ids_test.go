package model

import "testing"

func TestVrmWarningIDsAreNonEmptyAndUnique(t *testing.T) {
	if VrmWarningRawExtensionKey != "MU_VRM2PMX_warnings" {
		t.Fatalf("raw extension key mismatch: got=%s want=%s", VrmWarningRawExtensionKey, "MU_VRM2PMX_warnings")
	}
	if VrmLegacyGeneratedToonShadeMapRawExtensionKey != "MU_VRM2PMX_legacy_generated_toon_shade_map" {
		t.Fatalf(
			"toon shade map key mismatch: got=%s want=%s",
			VrmLegacyGeneratedToonShadeMapRawExtensionKey,
			"MU_VRM2PMX_legacy_generated_toon_shade_map",
		)
	}
	if VrmLegacySpherePriorityMigrationRawExtensionKey != "MU_VRM2PMX_legacy_sphere_priority_migration" {
		t.Fatalf(
			"sphere migration key mismatch: got=%s want=%s",
			VrmLegacySpherePriorityMigrationRawExtensionKey,
			"MU_VRM2PMX_legacy_sphere_priority_migration",
		)
	}

	warningIDs := []string{
		VrmWarningWeightsTruncated,
		VrmWarningPrimitiveNoSurface,
		VrmWarningUnsupportedMaterialFeature,
		VrmWarningToonTextureGenerationFailed,
		VrmWarningSphereTextureSourceMissing,
		VrmWarningSphereTextureGenerationFailed,
		VrmWarningEmissiveIgnoredBySpherePriority,
		VrmWarningSpherePriorityMigrationDiff,
		VrmWarningSpherePriorityRollbackRecommended,
		VrmWarningTextureTransformApprox,
		VrmWarningMaterialBindNotConvertible,
		VrmWarningGravityDirectionUnsupported,
		VrmWarningSpringParamClamped,
	}

	seen := map[string]struct{}{}
	for _, warningID := range warningIDs {
		if warningID == "" {
			t.Fatalf("warning id should not be empty")
		}
		if _, exists := seen[warningID]; exists {
			t.Fatalf("warning id should be unique: %s", warningID)
		}
		seen[warningID] = struct{}{}
	}
}
