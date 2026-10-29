package discovery

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"

type PackBudgetProfile struct {
	Version        string `json:"version"`
	MaxInputTokens int    `json:"max_input_tokens"`
	Estimator      string `json:"estimator"`
}

func CurrentPackBudget() PackBudgetProfile {
	return PackBudgetProfile{
		Version:        PackBudgetV2,
		MaxInputTokens: aitokens.MaxInputTokensPerRequest(),
		Estimator:      "aitokens.EstimateBytesTokens_len_div_4",
	}
}

type OutputBudgetProfile struct {
	Version                       string `json:"version"`
	MaxThreads                    int    `json:"max_threads"`
	MaxRelatedEntitiesPerThread   int    `json:"max_related_entities_per_thread"`
	MaxBasisFactsPerThread        int    `json:"max_basis_facts_per_thread"`
	MaxEvidenceRequestsPerThread  int    `json:"max_evidence_requests_per_thread"`
	MaxTotalEvidenceRequests      int    `json:"max_total_evidence_requests"`
	MaxObservationChars           int    `json:"max_observation_chars"`
	MaxInvestigationFocusChars    int    `json:"max_investigation_focus_chars"`
	MaxInvestigationQuestionChars int    `json:"max_investigation_question_chars"`
}

func CurrentOutputBudget() OutputBudgetProfile {
	return OutputBudgetProfile{
		Version:                       OutputBudgetV1,
		MaxThreads:                    10,
		MaxRelatedEntitiesPerThread:   20,
		MaxBasisFactsPerThread:        16,
		MaxEvidenceRequestsPerThread:  5,
		MaxTotalEvidenceRequests:      24,
		MaxObservationChars:           800,
		MaxInvestigationFocusChars:    1200,
		MaxInvestigationQuestionChars: 800,
	}
}

func (p OutputBudgetProfile) Hash() string {
	return hashProfile(p)
}
