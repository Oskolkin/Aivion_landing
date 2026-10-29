package deep

import (
	"os"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

type PackBudgetProfile struct {
	Version        string `json:"version"`
	MaxInputTokens int    `json:"max_input_tokens"`
	SafetyMargin   int    `json:"safety_margin"`
	Estimator      string `json:"estimator"`
}

func CurrentPackBudget() PackBudgetProfile {
	maxIn := aitokens.MaxInputTokensPerRequest()
	if v := strings.TrimSpace(os.Getenv("OPENAI_DEEP_MAX_INPUT_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxIn = n
		}
	}
	return PackBudgetProfile{
		Version: PackBudgetV1, MaxInputTokens: maxIn, SafetyMargin: 200,
		Estimator: "aitokens.EstimateBytesTokens_len_div_4",
	}
}

func (p PackBudgetProfile) EffectiveMaxInputTokens() int {
	maxOut := CurrentModelPolicy().MaxOutputTokens
	n := p.MaxInputTokens - p.SafetyMargin
	if maxOut > 0 {
		n -= maxOut
	}
	if n < 1 {
		return 1
	}
	return n
}

type OutputBudgetProfile struct {
	Version                      string `json:"version"`
	MaxKeyTakeaways              int    `json:"max_key_takeaways"`
	MaxFindings                  int    `json:"max_findings"`
	MaxRelationshipsPerFinding   int    `json:"max_relationships_per_finding"`
	MaxRecommendations           int    `json:"max_recommendations"`
	MaxActionProposals           int    `json:"max_action_proposals"`
	MaxFactHandlesPerObject      int    `json:"max_fact_handles_per_object"`
	MaxRelatedEntitiesPerFinding int    `json:"max_related_entities_per_finding"`
	MaxAlternativeExplanations   int    `json:"max_alternative_explanations"`
	HeadlineChars                int    `json:"headline_chars"`
	ExecutiveSummaryChars        int    `json:"executive_summary_chars"`
	TakeawayChars                int    `json:"takeaway_chars"`
	ThreadResolutionChars        int    `json:"thread_resolution_chars"`
	FindingTitleChars            int    `json:"finding_title_chars"`
	FindingStatementChars        int    `json:"finding_statement_chars"`
	RelationshipStatementChars   int    `json:"relationship_statement_chars"`
	DriverOutcomeChars           int    `json:"driver_outcome_chars"`
	RecommendationChars          int    `json:"recommendation_chars"`
	RationaleChars               int    `json:"rationale_chars"`
	PreviousAssessmentChars      int    `json:"previous_assessment_chars"`
	LimitationChars              int    `json:"limitation_chars"`
	AlternativeExplanationChars  int    `json:"alternative_explanation_chars"`
}

func CurrentOutputBudget() OutputBudgetProfile {
	return OutputBudgetProfile{
		Version:         OutputBudgetV1,
		MaxKeyTakeaways: 8, MaxFindings: 20, MaxRelationshipsPerFinding: 4,
		MaxRecommendations: 24, MaxActionProposals: 24, MaxFactHandlesPerObject: 24,
		MaxRelatedEntitiesPerFinding: 20, MaxAlternativeExplanations: 5,
		HeadlineChars: 300, ExecutiveSummaryChars: 1800, TakeawayChars: 700,
		ThreadResolutionChars: 1200, FindingTitleChars: 300, FindingStatementChars: 1400,
		RelationshipStatementChars: 1200, DriverOutcomeChars: 500,
		RecommendationChars: 1400, RationaleChars: 1800, PreviousAssessmentChars: 1200,
		LimitationChars: 700, AlternativeExplanationChars: 800,
	}
}

func (p OutputBudgetProfile) Hash() string { return canonicaljson.Hash(p) }

type ModelPolicy struct {
	Version         string `json:"version"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	Verbosity       string `json:"verbosity,omitempty"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
}

func CurrentModelPolicy() ModelPolicy {
	model := strings.TrimSpace(os.Getenv("OPENAI_DEEP_MODEL"))
	if model == "" {
		model = strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	}
	if model == "" {
		model = "gpt-5.4"
	}
	maxOut := DefaultMaxOutputTokens()
	if v := strings.TrimSpace(os.Getenv("OPENAI_DEEP_MAX_OUTPUT_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxOut = n
		}
	}
	timeout := 180
	if v := strings.TrimSpace(os.Getenv("OPENAI_DEEP_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = n
		}
	}
	return ModelPolicy{
		Version: ModelPolicyV1, Model: model,
		ReasoningEffort: strings.TrimSpace(os.Getenv("OPENAI_DEEP_REASONING_EFFORT")),
		Verbosity:       strings.TrimSpace(os.Getenv("OPENAI_DEEP_VERBOSITY")),
		MaxOutputTokens: maxOut, TimeoutSeconds: timeout,
	}
}

type AIAttemptPolicy struct {
	Version          string `json:"version"`
	MaxModelAttempts int    `json:"max_model_attempts"`
}

func CurrentAttemptPolicy() AIAttemptPolicy {
	return AIAttemptPolicy{Version: AIAttemptPolicyV1, MaxModelAttempts: 2}
}
