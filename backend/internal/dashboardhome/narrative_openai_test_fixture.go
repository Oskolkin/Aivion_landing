package dashboardhome

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"

// NewNarrativeServiceWithOpenAIClientForTest wires a minimal narrative service for cross-package tests.
func NewNarrativeServiceWithOpenAIClientForTest(ai *recommendations.OpenAIClient) *NarrativeService {
	return &NarrativeService{ai: ai, openAIModel: "gpt-test"}
}
