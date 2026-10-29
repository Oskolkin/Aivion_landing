package dashboardhome

import (
	"os"
	"strings"
	"time"
)

// NarrativeStaleDebug is returned in GET /dashboard/home when DASHBOARD_NARRATIVE_DEBUG=1.
type NarrativeStaleDebug struct {
	ExactStaleReason       string   `json:"exact_stale_reason,omitempty"`
	CurrentMetricDate      string   `json:"current_metric_date,omitempty"`
	StoredMetricDate       string   `json:"stored_metric_date,omitempty"`
	CurrentPromptVersion   string   `json:"current_prompt_version,omitempty"`
	StoredPromptVersion    string   `json:"stored_prompt_version,omitempty"`
	CurrentContextHash     string   `json:"current_context_hash,omitempty"`
	StoredContextHash      string   `json:"stored_context_hash,omitempty"`
	CurrentRiskDriversHash string   `json:"current_risk_drivers_hash,omitempty"`
	StoredRiskDriversHash  string   `json:"stored_risk_drivers_hash,omitempty"`
	StoredStatus           string   `json:"stored_status,omitempty"`
	StoredValidationErrors []string `json:"stored_validation_errors,omitempty"`
	SelectedNarrativeRunID string   `json:"selected_narrative_run_id,omitempty"`
	ValidationStatus       string   `json:"validation_status,omitempty"`
}

func shouldExposeNarrativeStaleDebug() bool {
	return strings.TrimSpace(os.Getenv("DASHBOARD_NARRATIVE_DEBUG")) == "1"
}

func buildNarrativeStaleDebug(
	metricDate time.Time,
	narrative *storedDashboardNarrative,
	build dashboardNarrativeBuildResult,
	validationStatus string,
) *NarrativeStaleDebug {
	debug := &NarrativeStaleDebug{
		CurrentMetricDate:      metricDate.UTC().Format("2006-01-02"),
		CurrentPromptVersion:   DashboardNarrativePromptVersion,
		CurrentContextHash:     build.FactualContextHash,
		CurrentRiskDriversHash: build.RiskDriversHash,
		ValidationStatus:       validationStatus,
		ExactStaleReason:       narrativeStaleReason(narrative, build),
	}
	if narrative != nil {
		debug.StoredMetricDate = narrative.MetricDate
		debug.StoredPromptVersion = narrative.PromptVersion
		debug.StoredContextHash = narrative.ContextHash
		debug.StoredRiskDriversHash = narrative.RiskDriversHash
		debug.StoredStatus = narrative.Status
		debug.StoredValidationErrors = append([]string(nil), narrative.ValidationErrors...)
		debug.SelectedNarrativeRunID = narrative.ID
	}
	return debug
}

func narrativeStaleReason(narrative *storedDashboardNarrative, build dashboardNarrativeBuildResult) string {
	if narrative == nil {
		return "missing_narrative_row"
	}
	if narrative.Payload == nil {
		return "missing_validated_payload"
	}
	if narrative.Status != "completed" {
		if len(narrative.ValidationErrors) > 0 {
			return "row_status_not_completed:" + strings.Join(narrative.ValidationErrors, ",")
		}
		return "row_status_not_completed"
	}
	if narrative.PromptVersion != DashboardNarrativePromptVersion {
		return "prompt_version_mismatch"
	}
	if narrative.ContextHash == "" {
		return "empty_stored_hash"
	}
	decision := evaluateNarrativeFreshness(narrative, build)
	if !decision.Fresh {
		return decision.Reason
	}
	return ""
}
