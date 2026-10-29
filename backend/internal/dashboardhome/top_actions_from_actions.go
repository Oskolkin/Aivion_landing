package dashboardhome

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/publishedsnapshot"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const topActionsSourceFinal = "actions_final"
const topActionsSourceEmpty = "actions_empty"

type topActionsFromActionsInput struct {
	SellerAccountID int64
	MetricDate      time.Time
	PipelineRunID   int64
	Limit           int
	Fallback        []TopActionItem
}

type topActionsFromActionsResult struct {
	Items    []TopActionItem
	Source   string
	Pipeline *DashboardPipelineInfo
}

func (s *Service) resolveDashboardTopActions(
	ctx context.Context,
	in topActionsFromActionsInput,
) topActionsFromActionsResult {
	if in.Limit <= 0 {
		in.Limit = topActionsLimit
	}
	if fromActions, ok := s.buildTopActionsFromFinalActions(ctx, in); ok {
		return fromActions
	}
	return topActionsFromActionsResult{Items: []TopActionItem{}, Source: "empty"}
}

func (s *Service) buildTopActionsFromFinalActions(
	ctx context.Context,
	in topActionsFromActionsInput,
) (topActionsFromActionsResult, bool) {
	run, pipeline, ok := s.resolveFinalActionsRun(ctx, in.SellerAccountID, in.MetricDate, in.PipelineRunID)
	if !ok || run == nil {
		return topActionsFromActionsResult{}, false
	}
	runID := run.ID
	if actions.IsEmptyActionFinalStatus(textOrEmpty(run.FinalStatus)) {
		pipelineInfo := buildDashboardPipelineInfo(run, pipeline)
		pipelineInfo.TopActionsSource = topActionsSourceEmpty
		return topActionsFromActionsResult{
			Items:    []TopActionItem{},
			Source:   topActionsSourceEmpty,
			Pipeline: pipelineInfo,
		}, true
	}
	rows, err := s.q.ListActionItemsWithUserStateByRunID(ctx, pgtype.Int8{Int64: runID, Valid: true})
	if err != nil || len(rows) == 0 {
		return topActionsFromActionsResult{}, false
	}

	topKeys := parseTopActionKeysJSON(run.TopActionKeysJson)
	selected := SelectDashboardTopActionRows(rows, topKeys, in.Limit)
	items := make([]TopActionItem, 0, len(selected))
	for _, row := range selected {
		item := mapActionRowToTopActionItem(row, *run, pipeline)
		if isValidTopActionFromActions(item) {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return topActionsFromActionsResult{}, false
	}

	pipelineInfo := buildDashboardPipelineInfo(run, pipeline)
	pipelineInfo.TopActionsSource = topActionsSourceFinal
	return topActionsFromActionsResult{
		Items:    items,
		Source:   topActionsSourceFinal,
		Pipeline: pipelineInfo,
	}, true
}

func (s *Service) resolveFinalActionsRun(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	pipelineRunID int64,
) (*dbgen.ActionGenerationRun, *dbgen.AiPipelineRun, bool) {
	var snapshot publishedsnapshot.Snapshot
	var err error
	if pipelineRunID > 0 {
		snapshot, err = s.resolver.GetPublishedSnapshotByPipelineID(ctx, sellerAccountID, pipelineRunID)
	} else {
		snapshot, err = s.resolver.GetCurrentPublishedSnapshot(ctx, sellerAccountID)
	}
	if err != nil || snapshot.ActionsRunID <= 0 {
		return nil, nil, false
	}
	if err := publishedsnapshot.ValidateSnapshot(snapshot, sellerAccountID); err != nil {
		return nil, nil, false
	}
	var pipeline *dbgen.AiPipelineRun
	pipelineRunID = snapshot.PipelineRunID
	if pipelineRunID > 0 {
		row, err := s.q.GetAIPipelineRun(ctx, pipelineRunID)
		if err == nil {
			pipeline = &row
			if row.ActionsRunID.Valid && row.ActionsRunID.Int64 == snapshot.ActionsRunID {
				run, runErr := s.q.GetActionGenerationRunForAdmin(ctx, snapshot.ActionsRunID)
				if runErr == nil {
					return &run, pipeline, true
				}
			}
		}
	}
	run, err := s.q.GetActionGenerationRunForAdmin(ctx, snapshot.ActionsRunID)
	if err != nil {
		return nil, nil, false
	}
	if !metricDate.IsZero() && run.MetricDate.Valid && run.MetricDate.Time.UTC().Format("2006-01-02") != metricDate.UTC().Format("2006-01-02") {
		return nil, nil, false
	}
	if pipeline == nil && run.AiPipelineRunID.Valid && run.AiPipelineRunID.Int64 > 0 {
		if row, pipeErr := s.q.GetAIPipelineRun(ctx, run.AiPipelineRunID.Int64); pipeErr == nil {
			pipeline = &row
		}
	}
	return &run, pipeline, true
}

func textOrEmpty(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return strings.TrimSpace(v.String)
}

func buildDashboardPipelineInfo(run *dbgen.ActionGenerationRun, pipeline *dbgen.AiPipelineRun) *DashboardPipelineInfo {
	out := &DashboardPipelineInfo{
		ActionsRunID: run.ID,
	}
	if run.DataPackHash.Valid {
		out.DataPackHash = strings.TrimSpace(run.DataPackHash.String)
	}
	if run.GlobalContextHash.Valid {
		out.GlobalContextHash = strings.TrimSpace(run.GlobalContextHash.String)
	}
	if pipeline != nil {
		out.PipelineRunID = pipeline.ID
		if strings.TrimSpace(pipeline.DataPackHash) != "" {
			out.DataPackHash = strings.TrimSpace(pipeline.DataPackHash)
		}
		if pipeline.GlobalContextHash.Valid {
			out.GlobalContextHash = strings.TrimSpace(pipeline.GlobalContextHash.String)
		}
		if pipeline.DashboardRunID.Valid {
			if u, err := uuid.FromBytes(pipeline.DashboardRunID.Bytes[:]); err == nil {
				out.DashboardRunID = u.String()
			}
		}
		if pipeline.ActionsRunID.Valid && pipeline.ActionsRunID.Int64 > 0 {
			out.ActionsRunID = pipeline.ActionsRunID.Int64
		}
	} else if run.AiPipelineRunID.Valid {
		out.PipelineRunID = run.AiPipelineRunID.Int64
	}
	return out
}

func mapActionRowToTopActionItem(
	row dbgen.ListActionItemsWithUserStateByRunIDRow,
	run dbgen.ActionGenerationRun,
	pipeline *dbgen.AiPipelineRun,
) TopActionItem {
	priority := actionPriorityToTopActionPriority(row.Priority)
	category := strings.ToLower(strings.TrimSpace(row.Category))
	summary := strings.TrimSpace(pgText(row.Summary))
	expectedEffect := strings.TrimSpace(pgText(row.ExpectedEffect))
	visibleLimitations := parseVisibleLimitationsFromPayload(row.PayloadJson)
	completionCriteria := parseCompletionCriteriaFromPayload(row.PayloadJson)

	item := TopActionItem{
		ID:                 row.ActionKey,
		ActionID:           row.ID,
		ActionKey:          row.ActionKey,
		ActionsRunID:       run.ID,
		Type:               strings.TrimSpace(row.Type),
		Summary:            cleanupSellerFacingText(summary),
		CompletionCriteria: cleanupSellerFacingText(completionCriteria),
		VisibleLimitations: visibleLimitations,
		Source:             topActionsSourceFinal,
		Priority:           priority,
		PriorityLabel:      topActionPriorityLabel(priority),
		Category:           category,
		CategoryLabel:      topActionCategoryLabel(category),
		Title:              cleanupSellerFacingText(row.Title),
		Reason:             cleanupSellerFacingText(row.Reason),
		NextStep:           cleanupSellerFacingText(row.NextStep),
		ExpectedEffect:     cleanupSellerFacingText(expectedEffect),
		SupportingMetrics:  evidenceJSONToSupportingMetrics(row.EvidenceJson),
	}
	if pipeline != nil {
		item.PipelineRunID = pipeline.ID
	} else if run.AiPipelineRunID.Valid {
		item.PipelineRunID = run.AiPipelineRunID.Int64
	}
	return item
}

func isValidTopActionFromActions(item TopActionItem) bool {
	return item.ActionID > 0 &&
		strings.TrimSpace(item.ActionKey) != "" &&
		strings.TrimSpace(item.Source) == topActionsSourceFinal &&
		isValidTopActionItem(item)
}

func isOpenActionUserStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "open":
		return true
	default:
		return false
	}
}

func actionPriorityToTopActionPriority(priority string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "critical", "urgent":
		return "critical"
	case "important", "high":
		return "high"
	case "watch", "low":
		return "low"
	default:
		return "medium"
	}
}

func actionPriorityRankForTopActions(priority string) int {
	switch actionPriorityToTopActionPriority(priority) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}

func sortActionRowsForTopActions(rows []dbgen.ListActionItemsWithUserStateByRunIDRow, topKeys []string) []dbgen.ListActionItemsWithUserStateByRunIDRow {
	order := make(map[string]int, len(topKeys))
	for i, key := range topKeys {
		order[key] = i
	}
	out := append([]dbgen.ListActionItemsWithUserStateByRunIDRow(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool {
		pi := actionPriorityRankForTopActions(out[i].Priority)
		pj := actionPriorityRankForTopActions(out[j].Priority)
		if pi != pj {
			return pi < pj
		}
		oi, okI := order[out[i].ActionKey]
		oj, okJ := order[out[j].ActionKey]
		if okI && okJ && oi != oj {
			return oi < oj
		}
		if okI != okJ {
			return okI
		}
		if out[i].CreatedAt.Time.Equal(out[j].CreatedAt.Time) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Time.Before(out[j].CreatedAt.Time)
	})
	return out
}

// SelectDashboardTopActionRows picks dashboard top-N rows using the same ordering
// rules as GET /api/v1/dashboard/home.
func SelectDashboardTopActionRows(
	rows []dbgen.ListActionItemsWithUserStateByRunIDRow,
	topKeys []string,
	limit int,
) []dbgen.ListActionItemsWithUserStateByRunIDRow {
	if limit <= 0 {
		limit = topActionsLimit
	}
	selected := make([]dbgen.ListActionItemsWithUserStateByRunIDRow, 0, limit)
	for _, key := range topKeys {
		if len(selected) >= limit {
			break
		}
		row, found := findActionRowByKey(rows, key)
		if !found || !isOpenActionUserStatus(row.UserStatus) {
			continue
		}
		selected = append(selected, row)
	}
	if len(selected) < limit {
		ordered := sortActionRowsForTopActions(rows, topKeys)
		seen := make(map[string]struct{}, len(selected))
		for _, row := range selected {
			seen[row.ActionKey] = struct{}{}
		}
		for _, row := range ordered {
			if len(selected) >= limit {
				break
			}
			if !isOpenActionUserStatus(row.UserStatus) {
				continue
			}
			if _, exists := seen[row.ActionKey]; exists {
				continue
			}
			seen[row.ActionKey] = struct{}{}
			selected = append(selected, row)
		}
	}
	return selected
}

// DashboardTopActionIDsFromRows returns action_id values for dashboard top-N in display order.
func DashboardTopActionIDsFromRows(
	rows []dbgen.ListActionItemsWithUserStateByRunIDRow,
	topActionKeysJSON []byte,
	limit int,
) []int64 {
	selected := SelectDashboardTopActionRows(rows, parseTopActionKeysJSON(topActionKeysJSON), limit)
	out := make([]int64, 0, len(selected))
	for _, row := range selected {
		out = append(out, row.ID)
	}
	return out
}

func findActionRowByKey(rows []dbgen.ListActionItemsWithUserStateByRunIDRow, key string) (dbgen.ListActionItemsWithUserStateByRunIDRow, bool) {
	key = strings.TrimSpace(key)
	for _, row := range rows {
		if row.ActionKey == key {
			return row, true
		}
	}
	return dbgen.ListActionItemsWithUserStateByRunIDRow{}, false
}

func parseTopActionKeysJSON(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func parseVisibleLimitationsFromPayload(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var payload struct {
		VisibleLimitations []string `json:"visible_limitations"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return compactStringSlice(payload.VisibleLimitations)
}

func parseCompletionCriteriaFromPayload(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var payload struct {
		CompletionCriteria string `json:"completion_criteria"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.CompletionCriteria)
}

func evidenceJSONToSupportingMetrics(raw []byte) []TopActionSupportingMetric {
	if len(raw) == 0 {
		return nil
	}
	var evidence []struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &evidence); err != nil {
		return nil
	}
	out := make([]TopActionSupportingMetric, 0, len(evidence))
	for _, item := range evidence {
		label := strings.TrimSpace(item.Label)
		value := strings.TrimSpace(item.Value)
		if label == "" || value == "" {
			continue
		}
		out = append(out, TopActionSupportingMetric{Label: label, Value: value})
	}
	return limitAndDedupeSupportingMetrics(compactSupportingMetrics(out), 6)
}

func limitAndDedupeSupportingMetrics(metrics []TopActionSupportingMetric, limit int) []TopActionSupportingMetric {
	if len(metrics) == 0 || limit <= 0 {
		return metrics
	}
	seen := make(map[string]struct{}, len(metrics))
	out := make([]TopActionSupportingMetric, 0, limit)
	for _, metric := range metrics {
		key := strings.ToLower(strings.TrimSpace(metric.Label)) + "|" + strings.ToLower(strings.TrimSpace(metric.Value))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, metric)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func pgText(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}
