package dashboardhome

import "strings"

// UnifiedPublicationAdapterPromptVersion is stamped on ai_dashboard_narratives
// written by Stage 5 publication (must match publication/adapter.Version).
const UnifiedPublicationAdapterPromptVersion = "unified_publication_adapter_v1"

// IsUnifiedPublicationPromptVersion reports whether a stored narrative was
// projected by the Unified Analysis publication adapter rather than the
// legacy dashboard_narrative_v9 generator.
func IsUnifiedPublicationPromptVersion(promptVersion string) bool {
	return strings.TrimSpace(promptVersion) == UnifiedPublicationAdapterPromptVersion
}
