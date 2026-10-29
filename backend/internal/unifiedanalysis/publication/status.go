package publication

// Visibility controls whether a publication is exposed to the seller-facing
// dashboard/actions surface (visible) or only materialized for comparison
// and QA purposes (shadow).
type Visibility string

const (
	VisibilityShadow  Visibility = "shadow"
	VisibilityVisible Visibility = "visible"
)

// IsValid reports whether v is a known visibility value.
func (v Visibility) IsValid() bool {
	switch v {
	case VisibilityShadow, VisibilityVisible:
		return true
	default:
		return false
	}
}

// Status is the lifecycle state of a unified_analysis_publications row.
// Mirrors the chk_unified_analysis_publications_status constraint added by
// migration 000056.
type Status string

const (
	StatusCreated           Status = "created"
	StatusProjecting        Status = "projecting"
	StatusReady             Status = "ready"
	StatusPublishing        Status = "publishing"
	StatusPublished         Status = "published"
	StatusPublicationFailed Status = "publication_failed"
	StatusRolledBack        Status = "rolled_back"
	StatusSuperseded        Status = "superseded"
)

// IsValid reports whether s is a known publication status value.
func (s Status) IsValid() bool {
	switch s {
	case StatusCreated, StatusProjecting, StatusReady, StatusPublishing,
		StatusPublished, StatusPublicationFailed, StatusRolledBack, StatusSuperseded:
		return true
	default:
		return false
	}
}

// IsTerminal reports whether s is a hard-terminal publication status.
func (s Status) IsTerminal() bool {
	switch s {
	case StatusPublished, StatusPublicationFailed, StatusRolledBack, StatusSuperseded:
		return true
	default:
		return false
	}
}

// AnalysisMode is the per-seller rollout mode stored on seller_analysis_settings
// and referenced by unified_analysis_publications.mode.
type AnalysisMode string

const (
	AnalysisModeLegacy  AnalysisMode = "legacy"
	AnalysisModeShadow  AnalysisMode = "shadow"
	AnalysisModeUnified AnalysisMode = "unified"
)

// IsValid reports whether m is a known analysis mode value.
func (m AnalysisMode) IsValid() bool {
	switch m {
	case AnalysisModeLegacy, AnalysisModeShadow, AnalysisModeUnified:
		return true
	default:
		return false
	}
}

// PublicationMode restricts unified_analysis_publications.mode, which (unlike
// seller_analysis_settings.analysis_mode) never takes the "legacy" value —
// a publication row only exists for shadow or unified analysis runs.
type PublicationMode string

const (
	PublicationModeShadow  PublicationMode = "shadow"
	PublicationModeUnified PublicationMode = "unified"
)

// IsValid reports whether m is a known publication mode value.
func (m PublicationMode) IsValid() bool {
	switch m {
	case PublicationModeShadow, PublicationModeUnified:
		return true
	default:
		return false
	}
}

// ForAnalysisMode maps a seller's AnalysisMode to the PublicationMode used
// when creating a unified_analysis_publications row. Legacy sellers never
// reach publication (callers must gate before invoking PublishRun).
func ForAnalysisMode(m AnalysisMode) (PublicationMode, bool) {
	switch m {
	case AnalysisModeShadow:
		return PublicationModeShadow, true
	case AnalysisModeUnified:
		return PublicationModeUnified, true
	default:
		return "", false
	}
}
