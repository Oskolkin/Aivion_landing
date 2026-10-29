package outputextract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

// Error codes for structured output extraction/parsing (mapped by orchestration).
const (
	CodeStructuredOutputMissing      = "structured_output_missing"
	CodeStructuredOutputAmbiguous    = "structured_output_ambiguous"
	CodeStructuredOutputInvalidJSON  = "structured_output_invalid_json"
	CodeStructuredOutputUnknownField = "structured_output_unknown_field"
	CodeProviderRefusal              = "provider_refusal"
	CodeProviderResponseIncomplete   = "provider_response_incomplete"
	CodeProviderFailed               = "provider_failed"
)

// Error is a typed extraction/parse failure with a stable code.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func coded(code, message string) error {
	return &Error{Code: code, Message: message}
}

// CodeOf returns the extraction error code when err is *Error.
func CodeOf(err error) (string, bool) {
	var e *Error
	if errors.As(err, &e) && e != nil {
		return e.Code, true
	}
	return "", false
}

// Extracted holds exact structured JSON bytes taken from a completed provider response.
type Extracted struct {
	JSONBytes []byte
}

// ExtractStructuredOutput pulls exactly one structured JSON payload from a completed response.
func ExtractStructuredOutput(resp *provider.ProviderResponse) (*Extracted, error) {
	if resp == nil {
		return nil, coded(CodeStructuredOutputMissing, "provider response is nil")
	}
	if strings.TrimSpace(resp.Refusal) != "" {
		return nil, coded(CodeProviderRefusal, "provider refused the request")
	}
	if resp.Status == provider.ProviderResponseStatusIncomplete {
		reason := ""
		if resp.IncompleteDetails != nil {
			reason = resp.IncompleteDetails.Reason
		}
		return nil, coded(CodeProviderResponseIncomplete, "provider response incomplete: "+reason)
	}
	if resp.Status != provider.ProviderResponseStatusCompleted {
		return nil, coded(CodeProviderFailed, "provider status is not completed")
	}
	if resp.AssistantMessageCount > 1 {
		return nil, coded(CodeStructuredOutputAmbiguous, "multiple assistant messages in provider output")
	}
	raw := strings.TrimSpace(resp.OutputText)
	raw = strings.TrimPrefix(raw, "\ufeff")
	if raw == "" {
		return nil, coded(CodeStructuredOutputMissing, "structured output text is empty")
	}
	if strings.HasPrefix(raw, "```") {
		return nil, coded(CodeStructuredOutputInvalidJSON, "structured output must not be markdown-fenced")
	}
	b := []byte(raw)
	if !utf8.Valid(b) {
		return nil, coded(CodeStructuredOutputInvalidJSON, "structured output is not valid utf-8")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var probe any
	if err := dec.Decode(&probe); err != nil {
		return nil, coded(CodeStructuredOutputInvalidJSON, "structured output is not valid JSON")
	}
	if dec.More() {
		return nil, coded(CodeStructuredOutputInvalidJSON, "structured output has trailing JSON garbage")
	}
	return &Extracted{JSONBytes: append([]byte(nil), b...)}, nil
}

// ParseOutput decodes extracted JSON into UnifiedAnalysisOutputV1 with unknown fields rejected.
func ParseOutput(raw []byte) (*contract.UnifiedAnalysisOutputV1, error) {
	trimmed := bytes.TrimSpace(raw)
	trimmed = bytes.TrimPrefix(trimmed, []byte("\ufeff"))
	if len(trimmed) == 0 {
		return nil, coded(CodeStructuredOutputMissing, "empty output json")
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var out contract.UnifiedAnalysisOutputV1
	if err := dec.Decode(&out); err != nil {
		msg := err.Error()
		code := CodeStructuredOutputInvalidJSON
		if strings.Contains(msg, "unknown field") {
			code = CodeStructuredOutputUnknownField
		}
		return nil, coded(code, fmt.Sprintf("parse unified output: %v", err))
	}
	if dec.More() {
		return nil, coded(CodeStructuredOutputInvalidJSON, "trailing JSON after unified output")
	}
	return &out, nil
}
