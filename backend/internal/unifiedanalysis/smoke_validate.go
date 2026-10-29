package unifiedanalysis

import (
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

// SmokeValidationInput configures smoke output validation.
type SmokeValidationInput struct {
	PackID        string
	SchemaVersion string
}

// ValidateSmokeOutput checks completed provider smoke output for pack linkage signals.
func ValidateSmokeOutput(outputText string, in SmokeValidationInput) error {
	text := strings.ToLower(strings.TrimSpace(outputText))
	if text == "" {
		return errorf(CodeSmokeValidationFailed, "smoke output is empty")
	}

	packID := strings.TrimSpace(in.PackID)
	if packID != "" && strings.Contains(text, strings.ToLower(packID)) {
		return nil
	}

	schema := strings.TrimSpace(in.SchemaVersion)
	if schema == "" {
		schema = contract.AnalysisPackSchemaVersionV1
	}
	if strings.Contains(text, strings.ToLower(schema)) {
		return nil
	}
	if strings.Contains(text, "schema_version") && strings.Contains(text, "pack_id") {
		return nil
	}
	return errorf(CodeSmokeValidationFailed, "smoke output missing pack_id or schema_version")
}

// ValidateCompletedProviderResponse enforces stage-3 smoke acceptance criteria.
func ValidateCompletedProviderResponse(resp *provider.ProviderResponse, containerID string, in SmokeValidationInput) error {
	if resp == nil {
		return errorf(CodeSmokeValidationFailed, "provider response is nil")
	}
	if !provider.HasCodeInterpreterCall(resp) {
		return errorf(CodeSmokeValidationFailed, "provider response missing code interpreter call")
	}
	if !provider.CallsMatchContainer(resp, containerID) {
		return errorf(CodeSmokeValidationFailed, "code interpreter container mismatch")
	}
	return ValidateSmokeOutput(resp.OutputText, in)
}

// validateProviderCompletion applies smoke text checks for smoke profile.
// Analysis profile requires Code Interpreter usage AND a non-empty assistant
// structured output message before advancing to output_validating.
func validateProviderCompletion(resp *provider.ProviderResponse, run *Run) error {
	if resp == nil {
		return errorf(CodeSmokeValidationFailed, "provider response is nil")
	}
	if strings.TrimSpace(resp.Refusal) != "" {
		return errorf(CodeProviderRefusal, "provider refused the request")
	}
	if !provider.HasCodeInterpreterCall(resp) {
		return errorf(CodeSmokeValidationFailed, "provider response missing code interpreter call")
	}
	if run != nil && !provider.CallsMatchContainer(resp, run.ProviderContainerID) {
		return errorf(CodeSmokeValidationFailed, "code interpreter container mismatch")
	}
	if run != nil && isAnalysisPromptVersion(run.PromptVersion) {
		if resp.AssistantMessageCount <= 0 || strings.TrimSpace(resp.OutputText) == "" {
			return errorf(CodeStructuredOutputMissing, "structured output text is empty")
		}
		return nil
	}
	in := SmokeValidationInput{}
	if run != nil {
		in.PackID = run.PackID
		in.SchemaVersion = run.DataPackSchemaVersion
	}
	return ValidateSmokeOutput(resp.OutputText, in)
}
