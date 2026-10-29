package readiness

import (
	"testing"
	"time"
)

func TestAccountHealthCapabilitiesAndWatermarksRemainIndependent(t *testing.T) {
	required := map[string]bool{
		"HEALTH.CURRENT_RATINGS": false, "HEALTH.RATING_HISTORY": false,
		"HEALTH.THRESHOLDS": false, "HEALTH.PENALTY_EVENTS": false,
		"HEALTH.FBS_ERROR_INDEX": false, "HEALTH.FBS_POSTING_ERRORS": false,
	}
	for _, key := range CapabilityKeys() {
		if _, ok := required[key]; ok {
			required[key] = true
		}
	}
	for key, found := range required {
		if !found {
			t.Errorf("missing capability %s", key)
		}
	}
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	history := now.Add(-time.Hour)
	index := now.Add(-2 * time.Hour)
	errorsAt := now.Add(-3 * time.Hour)
	service := &Service{}
	watermarks := service.watermarks([]domainEvidence{{domain: "ACCOUNT_HEALTH", runID: 7, finalized: now}}, evidenceStats{healthSnapshotObserved: &now, healthHistoryObserved: &history, healthFBSIndexObserved: &index, healthFBSErrorObserved: &errorsAt}, now)
	seen := map[string]*time.Time{}
	for _, watermark := range watermarks {
		if watermark.Domain == "ACCOUNT_HEALTH" {
			seen[watermark.FactFamily] = watermark.LatestObservedAt
		}
	}
	for _, family := range []string{"CURRENT_SNAPSHOT", "RATING_HISTORY", "FBS_INDEX", "FBS_ERROR_EVENTS"} {
		if seen[family] == nil {
			t.Errorf("missing independent watermark %s", family)
		}
	}
}

func TestProviderCriticalStatusIsNotDataQualitySeverity(t *testing.T) {
	service := &Service{}
	issues := service.issues(nil, evidenceStats{}, time.Now().UTC())
	for _, issue := range issues {
		if issue.Domain == "ACCOUNT_HEALTH" && issue.Code == "CRITICAL" {
			t.Fatal("provider rating status became a DQ issue")
		}
	}
}
