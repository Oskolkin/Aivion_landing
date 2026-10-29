package contract

// ValidAnalysisOutcome reports whether outcome is in the frozen allowlist.
func ValidAnalysisOutcome(v string) bool {
	return contains(AllowedAnalysisOutcomesV1, v)
}

// ValidActionType reports whether type is in the frozen allowlist.
func ValidActionType(v string) bool {
	return contains(AllowedActionTypesV1, v)
}

// ValidActionCategory reports whether category is in the frozen allowlist.
func ValidActionCategory(v string) bool {
	return contains(AllowedActionCategoriesV1, v)
}

// ValidActionPriority reports whether priority is in the frozen allowlist.
func ValidActionPriority(v string) bool {
	return contains(AllowedActionPrioritiesV1, v)
}

// ValidExecutionTier reports whether execution_tier is in the frozen allowlist.
func ValidExecutionTier(v string) bool {
	return contains(AllowedExecutionTiersV1, v)
}

// ValidScopeType reports whether scope type is in the frozen allowlist.
func ValidScopeType(v string) bool {
	return contains(AllowedActionScopeTypesV1, v)
}

// ValidConfidence reports whether confidence is in the frozen allowlist.
func ValidConfidence(v string) bool {
	return contains(AllowedActionConfidenceV1, v)
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
