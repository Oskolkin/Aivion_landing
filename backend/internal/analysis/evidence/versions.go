package evidence

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"

const (
	CatalogV1                = "evidence_catalog_v1"
	ResolverRegistryV1       = "evidence_resolver_registry_v1"
	RequestNormalizationV1   = "evidence_request_normalization_v1"
	RequestNormalizationV2   = "evidence_request_normalization_v2"
	EnvelopeSchemaV1         = "evidence_envelope_schema_v1"
	BudgetProfileV1          = "evidence_budget_profile_v1"
	FactHandleRegistryV1     = "analysis_fact_handle_registry_v1"
	ExpansionValidationV1    = "evidence_expansion_validation_v1"
	RequestSetSchemaV1       = "evidence_request_set_v1"
	RequestSetSchemaV2       = "evidence_request_set_v2"
	BundleSchemaV1           = "evidence_bundle_v1"
	BundleSchemaV2           = "evidence_bundle_v2"
	ExpansionReportSchemaV1  = "evidence_expansion_report_v1"
	TypeVersionV1            = "evidence_type_v1"
	SourceDiscoveryRequested = "DISCOVERY_REQUESTED"
	SourceContinuityRequired = "CONTINUITY_REQUIRED"
	StatusComplete           = "COMPLETE"
	StatusEmptyValid         = "EMPTY_VALID"
	StatusPartial            = "PARTIAL"
	StatusFailed             = "FAILED"
	CodeIntegrityFailure     = "FACTUAL_UNIVERSE_INTEGRITY_FAILURE"
	CodeBudgetExceeded       = "EVIDENCE_BUDGET_EXCEEDED"
	CodePII                  = "EVIDENCE_PII_FORBIDDEN"
	FormulaWeeklySumV1       = "evidence.flow_weekly_sum_v1"
	FormulaTotalsV1          = "evidence.period_totals_v1"
)

func ExpansionContractSeal() string {
	return "EVIDENCE_EXPANSION#" + canonicaljson.Hash(map[string]any{
		"catalog":               CatalogV1,
		"resolver_registry":     ResolverRegistryV1,
		"request_normalization": RequestNormalizationV1,
		"envelope":              EnvelopeSchemaV1,
		"budget":                BudgetProfileV1,
		"fact_handle_registry":  FactHandleRegistryV1,
		"validation":            ExpansionValidationV1,
	})
}
