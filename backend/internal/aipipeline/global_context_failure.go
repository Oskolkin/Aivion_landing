package aipipeline

import (
	"strings"
)

const (
	GlobalContextFailureTruncatedConfirmed = "global_context_truncated_confirmed"
	GlobalContextFailureTruncatedSuspected = "global_context_truncated_suspected"
	GlobalContextFailureInvalidJSON        = "global_context_invalid_json"
	GlobalContextFailureEmptyResponse      = "global_context_empty_response"
	GlobalContextFailureSchemaInvalid      = "global_context_schema_invalid"
	GlobalContextFailureSemanticInvalid    = "global_context_semantic_invalid"
	GlobalContextFailureProviderError      = "global_context_provider_error"
	GlobalContextFailureTimeout            = "global_context_timeout"
)

// ClassifyGlobalContextFailure maps provider/parse state to a stable failure subreason.
func ClassifyGlobalContextFailure(raw *GlobalContextRawResponse, parseErr error, validationStatus string) string {
	if raw == nil {
		if parseErr != nil {
			return GlobalContextFailureInvalidJSON
		}
		return GlobalContextFailureEmptyResponse
	}
	if strings.TrimSpace(raw.Content) == "" && raw.OutputTokens == 0 {
		return GlobalContextFailureEmptyResponse
	}
	if raw.ProviderStatus == "incomplete" && raw.IncompleteReason == "max_output_tokens" {
		return GlobalContextFailureTruncatedConfirmed
	}
	if raw.OutputTokens > 0 && raw.RequestedMaxOutputTokens > 0 &&
		raw.OutputTokens >= raw.RequestedMaxOutputTokens &&
		raw.IncompleteReason == "" {
		return GlobalContextFailureTruncatedSuspected
	}
	if parseErr != nil || raw.ParseStatus == ParseStatusInvalidJSON {
		if raw.OutputTokens > 0 && raw.RequestedMaxOutputTokens > 0 &&
			raw.OutputTokens < raw.RequestedMaxOutputTokens/2 {
			return GlobalContextFailureInvalidJSON
		}
		if raw.OutputTokens >= raw.RequestedMaxOutputTokens && raw.RequestedMaxOutputTokens > 0 {
			return GlobalContextFailureTruncatedSuspected
		}
		return GlobalContextFailureInvalidJSON
	}
	if validationStatus == "invalid" {
		return GlobalContextFailureSemanticInvalid
	}
	if strings.Contains(strings.ToLower(raw.ProviderError), "timeout") {
		return GlobalContextFailureTimeout
	}
	if raw.ProviderError != "" {
		return GlobalContextFailureProviderError
	}
	return GlobalContextFailureInvalidJSON
}
