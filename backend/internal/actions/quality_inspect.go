package actions

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

// ActionQualityReportRow is one action quality inspection result.
type ActionQualityReportRow struct {
	ActionID              int64    `json:"action_id"`
	ActionKey             string   `json:"action_key"`
	Type                  string   `json:"type"`
	Category              string   `json:"category"`
	Title                 string   `json:"title"`
	ObjectActionsCount    int      `json:"object_actions_count"`
	AffectedEntitiesCount int      `json:"affected_entities_count"`
	StepsCount            int      `json:"action_plan_steps_count"`
	ReasonLength          int      `json:"reason_length"`
	MetricsOK             bool     `json:"metrics_ok"`
	HasGenericSteps       bool     `json:"has_generic_steps"`
	EvidenceOK            bool     `json:"evidence_ok"`
	QualityPass           bool     `json:"quality_pass"`
	FailReasons           []string `json:"fail_reasons,omitempty"`
	WouldShowCabinetLevel bool     `json:"would_show_cabinet_level_ui"`
}

// ActionsQualityReport summarizes a published actions run.
type ActionsQualityReport struct {
	RunID                int64                    `json:"run_id"`
	SellerAccountID      int64                    `json:"seller_account_id"`
	MetricDate           string                   `json:"metric_date,omitempty"`
	TotalActions         int                      `json:"total_actions"`
	ObjectBasedActions   int                      `json:"object_based_actions"`
	Passed               int                      `json:"passed"`
	Failed               int                      `json:"failed"`
	MissingObjectActions int                      `json:"missing_object_actions"`
	InsufficientSteps    int                      `json:"insufficient_steps"`
	ShortReasons         int                      `json:"short_reasons"`
	CabinetLevelUI       int                      `json:"cabinet_level_ui_mismatch"`
	Rows                 []ActionQualityReportRow `json:"rows"`
}

// ActionQualityInspectInput is one persisted action row for inspection.
type ActionQualityInspectInput struct {
	ID                    int64
	SellerAccountID       int64
	RunID                 *int64
	ActionKey             string
	Type                  ActionType
	Category              ActionCategory
	Title                 string
	Reason                string
	SourceCandidateIDs    []string
	EvidenceFactIDs       []string
	AffectedEntitiesCount int
	PayloadJSON           []byte
	MetricDate            *time.Time
}

// InspectPublishedActionQuality evaluates persisted action items against structural quality rules.
func InspectPublishedActionQuality(rows []ActionQualityInspectInput) ActionsQualityReport {
	report := ActionsQualityReport{Rows: make([]ActionQualityReportRow, 0, len(rows))}
	for _, row := range rows {
		if report.RunID == 0 && row.RunID != nil {
			report.RunID = *row.RunID
		}
		if report.SellerAccountID == 0 {
			report.SellerAccountID = row.SellerAccountID
		}
		if report.MetricDate == "" && row.MetricDate != nil {
			report.MetricDate = row.MetricDate.Format("2006-01-02")
		}
		inspected := inspectOnePublishedAction(row)
		report.Rows = append(report.Rows, inspected)
		report.TotalActions++
		if isObjectBasedPublishedAction(row.Type, row.AffectedEntitiesCount) {
			report.ObjectBasedActions++
		}
		if inspected.QualityPass {
			report.Passed++
		} else {
			report.Failed++
		}
		for _, reason := range inspected.FailReasons {
			switch reason {
			case "missing_object_actions":
				report.MissingObjectActions++
			case "insufficient_action_plan":
				report.InsufficientSteps++
			case "reason_too_short":
				report.ShortReasons++
			}
		}
		if inspected.WouldShowCabinetLevel {
			report.CabinetLevelUI++
		}
	}
	return report
}

func inspectOnePublishedAction(row ActionQualityInspectInput) ActionQualityReportRow {
	detail, _ := ParseActionPayload(row.PayloadJSON)
	objectCount := 0
	stepsCount := 0
	metricsOK := true
	hasGeneric := false
	if detail != nil {
		objectCount = len(detail.ObjectActions)
		stepsCount = len(detail.ActionPlan.Steps)
		for _, obj := range detail.ObjectActions {
			if countNonEmptyObjectMetrics(objectMetricsToAIFirst(obj.Metrics)) < aiFirstMinObjectActionMetrics {
				metricsOK = false
			}
		}
		for _, step := range detail.ActionPlan.Steps {
			if isGenericActionStep(step.Instruction) {
				hasGeneric = true
			}
		}
	}
	reasonLen := utf8.RuneCountInString(strings.TrimSpace(row.Reason))
	objectBased := isObjectBasedPublishedAction(row.Type, row.AffectedEntitiesCount)
	wouldShowCabinet := objectCount == 0 && row.AffectedEntitiesCount == 0

	var failReasons []string
	if objectBased && objectCount == 0 {
		failReasons = append(failReasons, "missing_object_actions")
	}
	if stepsCount < aiFirstMinActionPlanSteps {
		failReasons = append(failReasons, "insufficient_action_plan")
	}
	if objectBased && reasonLen < aiFirstMinObjectBasedReasonLen {
		failReasons = append(failReasons, "reason_too_short")
	}
	if objectCount > 0 && !metricsOK {
		failReasons = append(failReasons, "insufficient_object_metrics")
	}
	if hasGeneric {
		failReasons = append(failReasons, "generic_action_step")
	}
	evidenceOK := len(row.SourceCandidateIDs) > 0 && len(row.EvidenceFactIDs) > 0
	if !evidenceOK {
		failReasons = append(failReasons, "missing_evidence")
	}
	if objectBased && row.AffectedEntitiesCount > 0 && objectCount == 0 {
		failReasons = append(failReasons, "affected_entities_without_object_actions")
	}
	if wouldShowCabinet && row.AffectedEntitiesCount > 0 {
		failReasons = append(failReasons, "ui_cabinet_level_mismatch")
	}

	return ActionQualityReportRow{
		ActionID:              row.ID,
		ActionKey:             row.ActionKey,
		Type:                  string(row.Type),
		Category:              string(row.Category),
		Title:                 row.Title,
		ObjectActionsCount:    objectCount,
		AffectedEntitiesCount: row.AffectedEntitiesCount,
		StepsCount:            stepsCount,
		ReasonLength:          reasonLen,
		MetricsOK:             metricsOK,
		HasGenericSteps:       hasGeneric,
		EvidenceOK:            evidenceOK,
		QualityPass:           len(failReasons) == 0,
		FailReasons:           failReasons,
		WouldShowCabinetLevel: objectCount == 0 && row.AffectedEntitiesCount > 0,
	}
}

func isObjectBasedPublishedAction(actionType ActionType, affectedCount int) bool {
	switch actionType {
	case ActionTypeFunnelWeakTransition:
		return affectedCount > 0
	case ActionTypeDataQualityIssue:
		return false
	default:
		return affectedCount > 0 || aiFirstRequiresObjectActions(actionType)
	}
}

func objectMetricsToAIFirst(metrics []OperationalMetricRef) []AIFirstObjectMetric {
	out := make([]AIFirstObjectMetric, 0, len(metrics))
	for _, m := range metrics {
		out = append(out, AIFirstObjectMetric{Label: m.Label, Value: m.Value})
	}
	return out
}

// CountAffectedEntitiesFromJSON counts sku+campaign+category refs in affected_entities_json.
func CountAffectedEntitiesFromJSON(raw []byte) int {
	if len(raw) == 0 {
		return 0
	}
	var payload struct {
		SKUs       []json.RawMessage `json:"skus"`
		Campaigns  []json.RawMessage `json:"campaigns"`
		Categories []json.RawMessage `json:"categories"`
		Funnel     *json.RawMessage  `json:"funnel"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0
	}
	n := len(payload.SKUs) + len(payload.Campaigns) + len(payload.Categories)
	if payload.Funnel != nil {
		n++
	}
	return n
}
