package dashboardhome

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
)

func estimateSynthesisInputTokens(input DashboardSynthesisInput) (int, error) {
	return aitokens.EstimateJSONTokens(input)
}

func maxDashboardSynthesisInputTokens() int {
	return aitokens.MaxInputTokensPerRequest()
}
