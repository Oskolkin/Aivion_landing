package metrics

import "strings"

// Bounded Prometheus reason_code values. Raw errors belong in logs/traces/artifacts.
const (
	ReasonSchemaInvalid     = "SCHEMA_INVALID"
	ReasonUnknownFactRef    = "UNKNOWN_FACT_REF"
	ReasonInvalidEntity     = "INVALID_ENTITY"
	ReasonBudgetExceeded    = "BUDGET_EXCEEDED"
	ReasonContinuityMissing = "CONTINUITY_MISSING"
	ReasonActionInvalid     = "ACTION_INVALID"
	ReasonProviderError     = "PROVIDER_ERROR"
	ReasonTimeout           = "TIMEOUT"
	ReasonUnknown           = "UNKNOWN"
)

var reasonNeedles = []struct {
	code string
	keys []string
}{
	{ReasonTimeout, []string{"timeout", "deadline exceeded", "context deadline"}},
	{ReasonBudgetExceeded, []string{"budget", "token limit", "max output"}},
	{ReasonUnknownFactRef, []string{"fact_ref", "factref", "unknown fact", "unknown handle"}},
	{ReasonInvalidEntity, []string{"invalid entity", "unknown sku", "unknown campaign", "entity"}},
	{ReasonSchemaInvalid, []string{"schema", "unmarshal", "json", "required field"}},
	{ReasonContinuityMissing, []string{"continuity"}},
	{ReasonActionInvalid, []string{"action invalid", "action_type", "unsupported action"}},
	{ReasonProviderError, []string{"provider", "openai", "status 4", "status 5"}},
}

// NormalizeReasonCode maps a raw error string to a low-cardinality reason_code.
func NormalizeReasonCode(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ReasonUnknown
	}
	for _, n := range reasonNeedles {
		for _, k := range n.keys {
			if strings.Contains(s, k) {
				return n.code
			}
		}
	}
	return ReasonUnknown
}
