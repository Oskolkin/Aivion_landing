package pack

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"

// BuildAnalysisPolicy renders analysis_policy.json from the frozen v1 defaults.
func BuildAnalysisPolicy() contract.AnalysisPolicyV1 {
	return contract.DefaultAnalysisPolicyV1()
}
