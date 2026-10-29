package recommendations

import (
	"context"
	"encoding/json"
	"fmt"
)

type CreateRunDiagnosticInput struct {
	RunID                   int64
	SellerAccountID         int64
	OpenAIRequestID         string
	AIModel                 string
	PromptVersion           string
	ContextPayloadSummary   map[string]any
	RawOpenAIResponse       json.RawMessage
	ValidationResultPayload map[string]any
	RejectedItemsPayload    map[string]any
	ErrorStage              string
	ErrorMessage            string
	InputTokens             int
	OutputTokens            int
	EstimatedCost           float64
}

func buildRunDiagnosticInput(
	runID int64,
	sellerAccountID int64,
	contextPayload *AIRecommendationContext,
	aiOutput *GenerateRecommendationsOutput,
	validation *ValidationResult,
	outcome runOutcome,
	savedCount int,
	estCost float64,
) CreateRunDiagnosticInput {
	in := CreateRunDiagnosticInput{
		RunID:                 runID,
		SellerAccountID:       sellerAccountID,
		ContextPayloadSummary: buildContextDiagnosticsSummary(contextPayload),
		RejectedItemsPayload:  map[string]any{"rejected_count": 0, "reasons": []map[string]any{}},
		InputTokens:           0,
		OutputTokens:          0,
		EstimatedCost:         estCost,
	}
	if aiOutput != nil {
		in.OpenAIRequestID = aiOutput.RequestID
		in.AIModel = aiOutput.Model
		in.InputTokens = aiOutput.InputTokens
		in.OutputTokens = aiOutput.OutputTokens
		in.RawOpenAIResponse = buildStoredRawAIResponse(aiOutput)
	}
	if validation != nil {
		rejectedPayload := map[string]any{
			"rejected_count": len(validation.RejectedRecommendations),
			"reasons":        rejectedReasonsSummary(validation.RejectedRecommendations, 30),
		}
		if validation.NormalizedTypesCount > 0 {
			rejectedPayload["normalized_types_count"] = validation.NormalizedTypesCount
			rejectedPayload["normalized_types"] = validation.NormalizedTypes
		}
		in.RejectedItemsPayload = rejectedPayload
	}
	summary := buildContextDiagnosticsSummary(contextPayload)
	if aiOutput != nil {
		summary["raw_ai_response_preview"] = rawAIResponsePreview(in.RawOpenAIResponse, aiOutput.Content, 4000)
	}
	if outcome.FailRun {
		in.ErrorStage = outcome.ErrorStage
		in.ErrorMessage = outcome.ErrorMessage
		summary["error_stage"] = outcome.ErrorStage
		summary["error_message"] = outcome.ErrorMessage
	}
	in.ValidationResultPayload = buildValidationDiagnosticsPayload(contextPayload, validation, outcome, savedCount)
	if len(outcome.Warnings) > 0 {
		summary["warnings"] = outcome.Warnings
	}
	in.ContextPayloadSummary = summary
	return in
}

func persistRunDiagnostic(
	repo serviceRepository,
	ctx context.Context,
	input CreateRunDiagnosticInput,
	promptVersion string,
) error {
	input.PromptVersion = promptVersion
	if input.ContextPayloadSummary == nil {
		input.ContextPayloadSummary = map[string]any{}
	}
	if input.ValidationResultPayload == nil {
		input.ValidationResultPayload = map[string]any{}
	}
	if input.RejectedItemsPayload == nil {
		input.RejectedItemsPayload = map[string]any{}
	}
	if len(input.RawOpenAIResponse) == 0 {
		input.RawOpenAIResponse = json.RawMessage(`{}`)
	}
	if err := repo.CreateRunDiagnostic(ctx, input); err != nil {
		return fmt.Errorf("persist recommendation run diagnostic: %w", err)
	}
	return nil
}
