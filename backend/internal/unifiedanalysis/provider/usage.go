package provider

// SumUsage returns aggregate token usage across responses.
func SumUsage(responses ...*ProviderResponse) ProviderUsage {
	var total ProviderUsage
	for _, resp := range responses {
		if resp == nil {
			continue
		}
		total.InputTokens += resp.Usage.InputTokens
		total.OutputTokens += resp.Usage.OutputTokens
		total.TotalTokens += resp.Usage.TotalTokens
	}
	return total
}

// CountCodeInterpreterCalls returns the total number of Code Interpreter calls.
func CountCodeInterpreterCalls(resp *ProviderResponse) int {
	if resp == nil {
		return 0
	}
	return len(resp.CodeInterpreterCalls)
}

// IsTerminalStatus reports whether a provider response reached a final state.
func IsTerminalStatus(status ProviderResponseStatus) bool {
	switch status {
	case ProviderResponseStatusCompleted,
		ProviderResponseStatusFailed,
		ProviderResponseStatusIncomplete,
		ProviderResponseStatusCancelled:
		return true
	default:
		return false
	}
}

// IsActiveStatus reports whether polling should continue.
func IsActiveStatus(status ProviderResponseStatus) bool {
	switch status {
	case ProviderResponseStatusQueued, ProviderResponseStatusInProgress:
		return true
	default:
		return false
	}
}
