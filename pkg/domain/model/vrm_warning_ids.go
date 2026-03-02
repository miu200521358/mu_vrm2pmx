// 指示: miu200521358
package model

const (
	// VrmWarningRawExtensionKey は変換時警告ID集合を保持する RawExtensions のキー。
	VrmWarningRawExtensionKey = "MU_VRM2PMX_warnings"
	// VrmLegacyGeneratedToonShadeMapRawExtensionKey は生成toonの shade 色マップを保持する RawExtensions のキー。
	VrmLegacyGeneratedToonShadeMapRawExtensionKey = "MU_VRM2PMX_legacy_generated_toon_shade_map"
	// VrmLegacySpherePriorityMigrationRawExtensionKey は sphere 優先順位の移行観測統計を保持する RawExtensions のキー。
	VrmLegacySpherePriorityMigrationRawExtensionKey = "MU_VRM2PMX_legacy_sphere_priority_migration"

	// VrmWarningWeightsTruncated は頂点ウェイト切り捨て警告。
	VrmWarningWeightsTruncated = "VrmWarningWeightsTruncated"
	// VrmWarningPrimitiveNoSurface は面未生成警告。
	VrmWarningPrimitiveNoSurface = "VrmWarningPrimitiveNoSurface"
	// VrmWarningUnsupportedMaterialFeature は材質機能未対応警告。
	VrmWarningUnsupportedMaterialFeature = "VrmWarningUnsupportedMaterialFeature"
	// VrmWarningToonTextureGenerationFailed は toon 生成失敗警告。
	VrmWarningToonTextureGenerationFailed = "VrmWarningToonTextureGenerationFailed"
	// VrmWarningSphereTextureSourceMissing は sphere 候補の参照不足警告。
	VrmWarningSphereTextureSourceMissing = "VrmWarningSphereTextureSourceMissing"
	// VrmWarningSphereTextureGenerationFailed は sphere 生成失敗警告。
	VrmWarningSphereTextureGenerationFailed = "VrmWarningSphereTextureGenerationFailed"
	// VrmWarningEmissiveIgnoredBySpherePriority は sphere 優先順位で emissive が不採用になった警告。
	VrmWarningEmissiveIgnoredBySpherePriority = "VrmWarningEmissiveIgnoredBySpherePriority"
	// VrmWarningSpherePriorityMigrationDiff は移行観測で新旧 sphere 採用候補が不一致だった警告。
	VrmWarningSpherePriorityMigrationDiff = "VrmWarningSpherePriorityMigrationDiff"
	// VrmWarningSpherePriorityRollbackRecommended は移行観測でロールバック推奨閾値を超えた警告。
	VrmWarningSpherePriorityRollbackRecommended = "VrmWarningSpherePriorityRollbackRecommended"
	// VrmWarningTextureTransformApprox は textureTransform 近似警告。
	VrmWarningTextureTransformApprox = "VrmWarningTextureTransformApprox"
	// VrmWarningMaterialBindNotConvertible は material bind 変換不可警告。
	VrmWarningMaterialBindNotConvertible = "VrmWarningMaterialBindNotConvertible"
	// VrmWarningGravityDirectionUnsupported は重力方向未対応警告。
	VrmWarningGravityDirectionUnsupported = "VrmWarningGravityDirectionUnsupported"
	// VrmWarningSpringParamClamped は spring パラメータ clamp 警告。
	VrmWarningSpringParamClamped = "VrmWarningSpringParamClamped"
)
