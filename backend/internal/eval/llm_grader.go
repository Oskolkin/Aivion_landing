package eval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

const LLMGraderKey = "offline_llm_rubric"
const LLMGraderVersion = "llm_grader_v1"
const SemanticVersion = "semantic_rubric_v1"
const DeterministicVersion = "deterministic_v1"

type SemanticScores struct {
	Groundedness         float64 `json:"groundedness"`
	FactualCorrectness   float64 `json:"factual_correctness"`
	Relevance            float64 `json:"relevance"`
	MaterialSignificance float64 `json:"material_significance"`
	CausalRestraint      float64 `json:"causal_restraint"`
	NonDuplication       float64 `json:"non_duplication"`
	Clarity              float64 `json:"clarity"`
}

type SemanticGrade struct {
	Groundedness         float64  `json:"groundedness"`
	FactualCorrectness   float64  `json:"factual_correctness"`
	Relevance            float64  `json:"relevance"`
	MaterialSignificance float64  `json:"material_significance"`
	CausalRestraint      float64  `json:"causal_restraint"`
	NonDuplication       float64  `json:"non_duplication"`
	Clarity              float64  `json:"clarity"`
	CriticalFailure      bool     `json:"critical_failure"`
	Reasons              []string `json:"reasons"`
}

func GradeSemantic(_ Case, _ string, scores SemanticScores, numericFactRefsValid bool) GradeResult {
	if !numericFactRefsValid {
		return GradeResult{CriticalFail: true, Code: "FACTREF_INVALID", Detail: "LLM grader cannot override deterministic FactRef failure"}
	}
	if scores.Groundedness < 0.5 || scores.FactualCorrectness < 0.5 {
		return GradeResult{CriticalFail: true, Code: "SEMANTIC_FAIL", Metrics: semanticMetrics(scores)}
	}
	return GradeResult{Pass: true, Code: "SEMANTIC", Metrics: semanticMetrics(scores)}
}

func semanticMetrics(scores SemanticScores) map[string]float64 {
	return map[string]float64{
		"groundedness":          scores.Groundedness,
		"factual_correctness":   scores.FactualCorrectness,
		"relevance":             scores.Relevance,
		"material_significance": scores.MaterialSignificance,
		"causal_restraint":      scores.CausalRestraint,
		"non_duplication":       scores.NonDuplication,
		"clarity":               scores.Clarity,
	}
}

type LLMGrader struct {
	Model         string
	RubricVersion string
	Prompt        string
	Client        *openaix.ResponsesClient
}

func (g LLMGrader) Identity() (key, version, promptHash, model, rubric string) {
	sum := sha256.Sum256([]byte(g.Prompt))
	return LLMGraderKey, LLMGraderVersion, hex.EncodeToString(sum[:]), g.Model, g.RubricVersion
}

func (g LLMGrader) Grade(artifact []byte) (GradeResult, error) {
	return g.GradeContext(context.Background(), LLMGradeInput{Candidate: string(artifact), NumericFactRefsValid: true})
}

type LLMGradeInput struct {
	Facts                string
	Rubric               string
	Candidate            string
	MandatoryThemes      []string
	ForbiddenConclusions []string
	NumericFactRefsValid bool
}

func (g LLMGrader) GradeContext(ctx context.Context, in LLMGradeInput) (GradeResult, error) {
	if !in.NumericFactRefsValid {
		return GradeResult{CriticalFail: true, Code: "FACTREF_INVALID", Detail: "LLM grader cannot override deterministic FactRef failure"}, nil
	}
	if len(in.Candidate) == 0 && len(in.Facts) == 0 {
		return GradeResult{Pass: false, Code: "EMPTY_ARTIFACT"}, fmt.Errorf("empty artifact")
	}
	if g.Client == nil {
		return GradeResult{Pass: false, Code: "LLM_GRADER_NO_CLIENT"}, fmt.Errorf("LLM grader requires openaix.ResponsesClient")
	}
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"groundedness":          map[string]any{"type": "number"},
			"factual_correctness":   map[string]any{"type": "number"},
			"relevance":             map[string]any{"type": "number"},
			"material_significance": map[string]any{"type": "number"},
			"causal_restraint":      map[string]any{"type": "number"},
			"non_duplication":       map[string]any{"type": "number"},
			"clarity":               map[string]any{"type": "number"},
			"critical_failure":      map[string]any{"type": "boolean"},
			"reasons":               map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"required": []string{"groundedness", "factual_correctness", "relevance", "material_significance", "causal_restraint", "non_duplication", "clarity", "critical_failure", "reasons"},
	}
	payload, _ := json.Marshal(map[string]any{
		"frozen_facts": in.Facts, "rubric": firstNonEmpty(in.Rubric, g.Prompt), "candidate": in.Candidate,
		"mandatory_themes": in.MandatoryThemes, "forbidden_conclusions": in.ForbiddenConclusions,
	})
	out, err := g.Client.Create(ctx, openaix.ResponsesRequest{
		Model: firstNonEmpty(g.Model, ResolveConfiguredModels().Discovery.Configured), Instructions: "Score the candidate against frozen facts. JSON only.",
		Input: string(payload), Tools: []any{}, MaxOutputTokens: 800, Truncation: "disabled",
		Text: openaix.TextFormat{Format: openaix.JSONSchemaFormat{Type: "json_schema", Name: "aivion_semantic_grade_v1", Strict: true, Schema: schema}},
	})
	if err != nil {
		return GradeResult{Pass: false, Code: "LLM_GRADER_PROVIDER"}, err
	}
	var grade SemanticGrade
	if e := json.Unmarshal([]byte(out.OutputText), &grade); e != nil {
		return GradeResult{Pass: false, Code: "LLM_GRADER_PARSE"}, e
	}
	scores := SemanticScores{
		Groundedness: grade.Groundedness, FactualCorrectness: grade.FactualCorrectness, Relevance: grade.Relevance,
		MaterialSignificance: grade.MaterialSignificance, CausalRestraint: grade.CausalRestraint,
		NonDuplication: grade.NonDuplication, Clarity: grade.Clarity,
	}
	res := GradeSemantic(Case{}, in.Candidate, scores, true)
	if grade.CriticalFailure {
		res.CriticalFail = true
		res.Pass = false
		res.Code = "LLM_CRITICAL"
	}
	return res, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
