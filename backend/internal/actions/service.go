package actions

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysissource"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	repo           Repository
	sourceResolver analysissource.Resolver
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// NewServiceWithSourceResolver wires the authoritative analysis-source selector
// so ListActions reads the same publication/run bundle as Dashboard.
func NewServiceWithSourceResolver(repo Repository, sourceResolver analysissource.Resolver) *Service {
	return &Service{repo: repo, sourceResolver: sourceResolver}
}

func (s *Service) ListActions(ctx context.Context, sellerAccountID int64, filters ListActionsFilters) (*ActionsResponse, error) {
	var source analysissource.CurrentAnalysisSource
	var currentRun *ActionGenerationRunRecord
	var err error

	if s.sourceResolver != nil {
		source, err = s.sourceResolver.ResolveCurrent(ctx, sellerAccountID)
		if err != nil {
			return nil, err
		}
		if source.Source == analysissource.SourceNone || source.PipelineRunID <= 0 {
			resp := notGeneratedActionsResponse()
			resp.AnalysisSource = analysissource.SourceNone
			resp.FallbackUsed = source.FallbackUsed
			resp.FallbackReason = source.FallbackReason
			resp.PublicationID = source.PublicationID
			resp.MetricDate = source.MetricDate
			return resp, nil
		}
		currentRun, err = s.repo.GetPublishableRunByPipelineID(ctx, sellerAccountID, source.PipelineRunID)
		if err != nil {
			return nil, err
		}
		if currentRun == nil {
			resp := notGeneratedActionsResponse()
			resp.AnalysisSource = source.Source
			resp.PublicationID = source.PublicationID
			resp.PipelineRunID = source.PipelineRunID
			resp.MetricDate = source.MetricDate
			resp.FallbackUsed = source.FallbackUsed
			resp.FallbackReason = joinActionsReason(source.FallbackReason, "actions_run_missing_for_pipeline")
			return resp, nil
		}
	} else {
		currentRun, err = s.repo.GetLatestPublishableRun(ctx, sellerAccountID)
		if err != nil {
			return nil, err
		}
		if currentRun == nil {
			return notGeneratedActionsResponse(), nil
		}
	}

	metricDate := currentRun.MetricDate
	if filters.MetricDate != "" && filters.MetricDate != "latest" {
		parsed, perr := time.Parse("2006-01-02", filters.MetricDate)
		if perr != nil {
			return nil, ErrInvalidMetricDate
		}
		t := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		metricDate = &t
	}

	if metricDate != nil && currentRun.MetricDate != nil &&
		metricDate.UTC().Format("2006-01-02") != currentRun.MetricDate.UTC().Format("2006-01-02") {
		return notGeneratedActionsResponse(), nil
	}

	if IsEmptyActionFinalStatus(currentRun.FinalStatus) {
		resp := emptyPublishableRunResponse(currentRun)
		annotateActionsFromSource(sellerAccountID, source, currentRun, resp)
		return resp, nil
	}

	counterItems, err := s.repo.ListActionsByRunID(ctx, currentRun.ID, ListActionsParams{
		SellerAccountID: sellerAccountID,
		Status:          "all",
		Limit:           10000,
	})
	if err != nil {
		return nil, err
	}

	listItems, err := s.repo.ListActionsByRunID(ctx, currentRun.ID, ListActionsParams{
		SellerAccountID: sellerAccountID,
		Status:          filters.Status,
		Category:        filters.Category,
		Priority:        filters.Priority,
		Limit:           filters.Limit,
		Offset:          filters.Offset,
	})
	if err != nil {
		return nil, err
	}

	actions := make([]ActionItem, 0, len(listItems))
	for _, rec := range listItems {
		item, mapErr := mapRecordToActionItem(rec)
		if mapErr != nil {
			log.Printf("actions: skip item %q: %v", rec.ActionKey, mapErr)
			continue
		}
		actions = append(actions, item)
	}

	resp := &ActionsResponse{
		Status:           string(ActionResponseStatusAvailable),
		Counters:         computeCounters(counterItems),
		Actions:          actions,
		DataQualityNotes: []string{},
	}
	if currentRun.MetricDate != nil {
		resp.MetricDate = currentRun.MetricDate.Format("2006-01-02")
	}
	if currentRun.AiPipelineRunID > 0 {
		resp.PipelineRunID = currentRun.AiPipelineRunID
	}
	annotateActionsFromSource(sellerAccountID, source, currentRun, resp)
	if notes := parseStringSliceJSON(currentRun.DataQualityNotesJSON); len(notes) > 0 {
		resp.DataQualityNotes = notes
	}
	if focus := parseAIFocusJSON(currentRun.AIFocusJSON); focus != nil {
		resp.AIFocus = focus
	}
	return resp, nil
}

func annotateActionsFromSource(sellerAccountID int64, source analysissource.CurrentAnalysisSource, run *ActionGenerationRunRecord, resp *ActionsResponse) {
	if resp == nil {
		return
	}
	if source.Source != "" {
		resp.AnalysisSource = source.Source
		resp.PublicationID = source.PublicationID
		resp.FallbackUsed = source.FallbackUsed
		resp.FallbackReason = source.FallbackReason
		if source.PipelineRunID > 0 {
			resp.PipelineRunID = source.PipelineRunID
		}
		if source.MetricDate != "" {
			resp.MetricDate = source.MetricDate
		}
	} else if run != nil {
		if strings.HasPrefix(strings.TrimSpace(run.GlobalContextHash), "unified:") {
			resp.AnalysisSource = analysissource.SourceUnified
		} else {
			resp.AnalysisSource = analysissource.SourceLegacy
		}
	}
	log.Printf("actions source selected seller_id=%d selected_source=%s publication_id=%d pipeline_run_id=%d metric_date=%s actions=%d fallback_used=%v fallback_reason=%s",
		sellerAccountID, resp.AnalysisSource, resp.PublicationID, resp.PipelineRunID, resp.MetricDate, len(resp.Actions), resp.FallbackUsed, resp.FallbackReason)
}

func joinActionsReason(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "; " + b
	}
}

func (s *Service) GetAction(ctx context.Context, sellerAccountID int64, actionKey string) (*ActionItem, error) {
	rec, err := s.repo.GetActionByKey(ctx, sellerAccountID, actionKey)
	if err != nil {
		return nil, err
	}
	item, err := mapRecordToActionItem(*rec)
	if err != nil {
		return nil, err
	}
	s.attachActionSourceTrace(ctx, &item, rec)
	return &item, nil
}

func (s *Service) GetHistoricalActionByID(ctx context.Context, sellerAccountID int64, actionID int64) (*ActionItem, error) {
	rec, err := s.repo.GetHistoricalActionByID(ctx, sellerAccountID, actionID)
	if err != nil {
		return nil, err
	}
	item, err := mapRecordToActionItem(*rec)
	if err != nil {
		return nil, err
	}
	s.attachActionSourceTrace(ctx, &item, rec)
	return &item, nil
}

func (s *Service) GetActionByID(ctx context.Context, sellerAccountID int64, actionID int64) (*ActionItem, error) {
	rec, err := s.repo.GetActionByID(ctx, sellerAccountID, actionID)
	if err != nil {
		return nil, err
	}
	item, err := mapRecordToActionItem(*rec)
	if err != nil {
		return nil, err
	}
	s.attachActionSourceTrace(ctx, &item, rec)
	return &item, nil
}

func (s *Service) attachActionSourceTrace(ctx context.Context, item *ActionItem, rec *ActionItemRecord) {
	if item == nil || rec == nil || rec.RunID == nil || *rec.RunID <= 0 {
		return
	}
	item.RunID = *rec.RunID
	run, runErr := s.repo.GetGenerationRunByID(ctx, *rec.RunID)
	if runErr != nil || run == nil {
		return
	}
	trace := &ActionSourceTrace{
		PipelineRunID:     run.AiPipelineRunID,
		ActionsRunID:      run.ID,
		DataPackHash:      run.DataPackHash,
		GlobalContextHash: run.GlobalContextHash,
		MetricDate:        "",
	}
	if run.MetricDate != nil {
		trace.MetricDate = run.MetricDate.UTC().Format("2006-01-02")
	}
	if s.sourceResolver != nil {
		if source, err := s.sourceResolver.ResolveCurrent(ctx, rec.SellerAccountID); err == nil && source.Source == analysissource.SourceUnified {
			trace.PublicationID = source.PublicationID
			if source.PipelineRunID == run.AiPipelineRunID {
				trace.PipelineRunID = source.PipelineRunID
				if source.MetricDate != "" {
					trace.MetricDate = source.MetricDate
				}
			}
		}
	}
	item.Source = trace
}

func (s *Service) CheckAction(ctx context.Context, sellerAccountID int64, actionKey string) error {
	return s.repo.SetActionChecked(ctx, sellerAccountID, actionKey)
}

func (s *Service) UncheckAction(ctx context.Context, sellerAccountID int64, actionKey string) error {
	return s.repo.SetActionOpen(ctx, sellerAccountID, actionKey)
}

// DismissAction marks an action as irrelevant ("Неактуально") for the seller.
func (s *Service) DismissAction(ctx context.Context, sellerAccountID int64, actionKey string) error {
	return s.repo.SetActionDismissed(ctx, sellerAccountID, actionKey)
}

func (s *Service) ExportAction(ctx context.Context, sellerAccountID int64, actionKey, format string) ([]byte, string, string, error) {
	if format == "" {
		format = "xlsx"
	}
	if format != "xlsx" && format != "csv" {
		return nil, "", "", ErrUnsupportedExportFormat
	}
	item, err := s.GetAction(ctx, sellerAccountID, actionKey)
	if err != nil {
		return nil, "", "", err
	}
	if !supportsActionExport(item.Type) {
		return nil, "", "", ErrUnsupportedActionExport
	}
	entityCtx := resolveExportEntityContext(ctx, s.repo, sellerAccountID, *item)
	var payload []byte
	var mime string
	if format == "csv" {
		payload, mime, err = renderActionExportCSV(*item, entityCtx)
	} else {
		payload, mime, err = renderActionExportXLSX(*item, entityCtx)
	}
	if err != nil {
		return nil, "", "", err
	}
	filename := safeExportFilename(*item, format)
	return payload, mime, filename, nil
}

func resolveExportEntityContext(ctx context.Context, repo Repository, sellerAccountID int64, item ActionItem) *ExportEntityContext {
	if strings.TrimSpace(item.EntityGroupRef) == "" {
		return nil
	}
	group, err := repo.GetActionEntityGroup(ctx, sellerAccountID, item.RunID, item.EntityGroupRef)
	if err != nil {
		if errors.Is(err, ErrEntityGroupNotFound) && hasAffectedEntitiesPreview(item.AffectedEntities) {
			return &ExportEntityContext{
				MissingFullList: true,
				Warning:         "Полный список объектов недоступен",
			}
		}
		return nil
	}
	return &ExportEntityContext{
		EntityType: group.EntityType,
		EntityIDs:  group.EntityIDs,
		Summary:    group.Summary,
		Source:     "entity_group",
	}
}

// TODO: Dashboard Top 3 should use this after Actions Engine stabilization.
func (s *Service) ListTopOpenActionsForDashboard(ctx context.Context, sellerAccountID int64, metricDate *time.Time, limit int) ([]ActionItem, error) {
	if limit <= 0 {
		limit = 3
	}
	if limit > 20 {
		limit = 20
	}
	resolvedMetricDate := metricDate
	if resolvedMetricDate == nil {
		latest, err := s.repo.GetLatestMetricDate(ctx, sellerAccountID)
		if err != nil {
			return nil, err
		}
		resolvedMetricDate = latest
	}
	items, err := s.repo.ListActions(ctx, ListActionsParams{
		SellerAccountID: sellerAccountID,
		Status:          string(ActionStatusOpen),
		MetricDate:      resolvedMetricDate,
		Limit:           200,
		Offset:          0,
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		pi := priorityOrder(items[i].Priority)
		pj := priorityOrder(items[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return items[i].ActionKey < items[j].ActionKey
	})
	out := make([]ActionItem, 0, limit)
	for _, rec := range items {
		item, mapErr := mapRecordToActionItem(rec)
		if mapErr != nil {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *Service) ListGenerationRunsForAdmin(ctx context.Context, filters AdminListGenerationRunsFilters) ([]AdminGenerationRun, error) {
	adminRepo, ok := s.repo.(AdminRepository)
	if !ok {
		return nil, ErrActionNotFound
	}
	metricDate, err := parseMetricDateFilter(strings.TrimSpace(filters.MetricDate))
	if err != nil {
		return nil, ErrInvalidMetricDate
	}
	rows, err := adminRepo.ListGenerationRunsForAdmin(ctx, AdminListGenerationRunsParams{
		SellerAccountID: filters.SellerAccountID,
		Status:          strings.TrimSpace(filters.Status),
		FinalStatus:     strings.TrimSpace(filters.FinalStatus),
		MetricDate:      metricDate,
		Limit:           filters.Limit,
		Offset:          filters.Offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]AdminGenerationRun, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapAdminGenerationRun(row))
	}
	return out, nil
}

func (s *Service) GetGenerationRunDetailForAdmin(ctx context.Context, runID int64) (*AdminGenerationRun, []AdminGenerationCategoryRun, error) {
	adminRepo, ok := s.repo.(AdminRepository)
	if !ok {
		return nil, nil, ErrActionNotFound
	}
	run, err := adminRepo.GetGenerationRunForAdmin(ctx, runID)
	if err != nil {
		return nil, nil, err
	}
	rows, err := adminRepo.ListGenerationCategoryRunsForAdmin(ctx, runID)
	if err != nil {
		return nil, nil, err
	}
	categoryRuns := make([]AdminGenerationCategoryRun, 0, len(rows))
	for _, row := range rows {
		categoryRuns = append(categoryRuns, mapAdminGenerationCategoryRun(row))
	}
	mapped := mapAdminGenerationRun(*run)
	return &mapped, categoryRuns, nil
}

func (s *Service) resolveMetricDate(ctx context.Context, sellerAccountID int64, raw string) (*time.Time, error) {
	switch raw {
	case "", "latest":
		latest, err := s.repo.GetLatestMetricDate(ctx, sellerAccountID)
		if err != nil {
			return nil, err
		}
		return latest, nil
	default:
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return nil, ErrInvalidMetricDate
		}
		t := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		return &t, nil
	}
}

func parseMetricDateFilter(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	t := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
	return &t, nil
}

func mapAdminGenerationRun(row dbgen.ActionGenerationRun) AdminGenerationRun {
	out := AdminGenerationRun{
		RunID:                row.ID,
		SellerAccountID:      row.SellerAccountID,
		MetricDate:           formatDate(row.MetricDate),
		Status:               row.Status,
		FinalStatus:          textString(row.FinalStatus),
		ContextHash:          textString(row.ContextHash),
		AcceptedCount:        int(row.AcceptedCount),
		RepairedCount:        int(row.RepairedCount),
		DroppedCount:         int(row.DroppedCount),
		FallbackCount:        int(row.FallbackCount),
		CreatedAt:            formatTime(timestamptzTime(row.CreatedAt)),
		UpdatedAt:            formatTime(timestamptzTime(row.UpdatedAt)),
		AIFocusJSON:          row.AiFocusJson,
		TopActionKeysJSON:    row.TopActionKeysJson,
		DataQualityNotesJSON: row.DataQualityNotesJson,
		DataPackHash:         textString(row.DataPackHash),
		GlobalContextHash:    textString(row.GlobalContextHash),
	}
	if row.AiPipelineRunID.Valid {
		out.AiPipelineRunID = row.AiPipelineRunID.Int64
	}
	return out
}

func mapAdminGenerationCategoryRun(row dbgen.ActionGenerationCategoryRun) AdminGenerationCategoryRun {
	return AdminGenerationCategoryRun{
		Domain:              row.Domain,
		Status:              row.Status,
		Model:               textString(row.Model),
		AcceptedCount:       int(row.AcceptedCount),
		RepairedCount:       int(row.RepairedCount),
		DroppedCount:        int(row.DroppedCount),
		ValidationJSON:      row.ValidationJson,
		FinalValidationJSON: row.FinalValidationJson,
		RawResponseJSON:     row.RawResponseJson,
		RepairResponseJSON:  row.RepairResponseJson,
		CreatedAt:           formatTime(timestamptzTime(row.CreatedAt)),
		UpdatedAt:           formatTime(timestamptzTime(row.UpdatedAt)),
	}
}

func formatDate(v pgtype.Date) string {
	if !v.Valid {
		return ""
	}
	return v.Time.UTC().Format("2006-01-02")
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func emptyActionsResponse() *ActionsResponse {
	return &ActionsResponse{
		Status: string(ActionResponseStatusEmpty),
		Counters: ActionCounters{
			TotalOpen:   0,
			Critical:    0,
			Important:   0,
			Watch:       0,
			Stock:       0,
			SKU:         0,
			Advertising: 0,
			Funnel:      0,
			Sales:       0,
			Checked:     0,
		},
		Actions:          []ActionItem{},
		DataQualityNotes: []string{emptyActionsNote},
	}
}

func notGeneratedActionsResponse() *ActionsResponse {
	return &ActionsResponse{
		Status:           string(ActionResponseStatusNotGenerated),
		Counters:         ActionCounters{},
		Actions:          []ActionItem{},
		DataQualityNotes: []string{notGeneratedActionsNote},
	}
}

func emptyPublishableRunResponse(run *ActionGenerationRunRecord) *ActionsResponse {
	resp := emptyActionsResponse()
	if run != nil && run.MetricDate != nil {
		resp.MetricDate = run.MetricDate.Format("2006-01-02")
	}
	if run != nil {
		if notes := parseStringSliceJSON(run.DataQualityNotesJSON); len(notes) > 0 {
			resp.DataQualityNotes = notes
		} else {
			resp.DataQualityNotes = []string{emptyActionsNote}
		}
		if focus := parseAIFocusJSON(run.AIFocusJSON); focus != nil {
			resp.AIFocus = focus
		}
	}
	return resp
}

func computeCounters(items []ActionItemRecord) ActionCounters {
	var counters ActionCounters
	for _, item := range items {
		switch item.Status {
		case ActionStatusOpen:
			counters.TotalOpen++
			switch item.Priority {
			case ActionPriorityCritical:
				counters.Critical++
			case ActionPriorityImportant:
				counters.Important++
			case ActionPriorityWatch:
				counters.Watch++
			}
		case ActionStatusChecked:
			counters.Checked++
		case ActionStatusDismissed:
			counters.Dismissed++
		}
		switch item.Category {
		case ActionCategoryStock:
			if item.Status == ActionStatusOpen {
				counters.Stock++
			}
		case ActionCategorySlowTurnover:
			if item.Status == ActionStatusOpen {
				counters.SlowTurnover++
			}
		case ActionCategorySKU:
			if item.Status == ActionStatusOpen {
				counters.SKU++
			}
		case ActionCategoryAdvertising:
			if item.Status == ActionStatusOpen {
				counters.Advertising++
			}
		case ActionCategoryFunnel:
			if item.Status == ActionStatusOpen {
				counters.Funnel++
			}
		case ActionCategorySales:
			if item.Status == ActionStatusOpen {
				counters.Sales++
			}
		case ActionCategoryCategory:
			if item.Status == ActionStatusOpen {
				counters.CategoryCount++
			}
		}
	}
	return counters
}

func mapRecordToActionItem(rec ActionItemRecord) (ActionItem, error) {
	evidence, err := parseEvidenceJSON(rec.EvidenceJSON)
	if err != nil {
		return ActionItem{}, err
	}
	limitations, err := parseStringSliceJSONErr(rec.LimitationsJSON)
	if err != nil {
		return ActionItem{}, err
	}
	ctas, err := parseCTAJSON(rec.CTAJSON)
	if err != nil {
		return ActionItem{}, err
	}
	affected, err := parseAffectedEntitiesJSON(rec.AffectedEntitiesJSON)
	if err != nil {
		return ActionItem{}, err
	}
	sourceCandidateIDs, err := parseStringSliceJSONErr(rec.SourceCandidateIDs)
	if err != nil {
		return ActionItem{}, err
	}
	evidenceFactIDs, err := parseStringSliceJSONErr(rec.EvidenceFactIDs)
	if err != nil {
		return ActionItem{}, err
	}

	operationalDetail, aiFirstPayload := parseActionPayload(rec.PayloadJSON)

	item := ActionItem{
		ID:                 rec.ID,
		ActionKey:          rec.ActionKey,
		Type:               rec.Type,
		Category:           rec.Category,
		Priority:           rec.Priority,
		Status:             rec.Status,
		Title:              rec.Title,
		Summary:            rec.Summary,
		Reason:             rec.Reason,
		NextStep:           rec.NextStep,
		ExpectedEffect:     rec.ExpectedEffect,
		Evidence:           evidence,
		Limitations:        limitations,
		CTAs:               ctas,
		AffectedEntities:   affected,
		SourceCandidateIDs: sourceCandidateIDs,
		EvidenceFactIDs:    evidenceFactIDs,
		EntityGroupRef:     rec.EntityGroupRef,
		OperationalDetail:  operationalDetail,
		CreatedAt:          rec.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          rec.UpdatedAt.Format(time.RFC3339),
	}
	if aiFirstPayload != nil {
		item.UrgencyMarker = aiFirstPayload.UrgencyMarker
		item.AffectedObjectsSummary = aiFirstPayload.AffectedObjectsSummary
		item.AffectedObjectsTotal = aiFirstPayload.AffectedObjectsTotal
		item.AffectedObjectsDisplayed = aiFirstPayload.AffectedObjectsDisplayed
		item.AffectedObjectsTruncated = aiFirstPayload.AffectedObjectsTruncated
		item.OperationalGoal = aiFirstPayload.OperationalGoal
		item.ExecutionTier = aiFirstPayload.ExecutionTier
		item.CompletionCriteria = aiFirstPayload.CompletionCriteria
		item.CompletionConditions = aiFirstPayload.CompletionConditions
		item.VisibleLimitations = aiFirstPayload.VisibleLimitations
	}
	if rec.MetricDate != nil {
		item.MetricDate = rec.MetricDate.Format("2006-01-02")
	}
	if rec.CheckedAt != nil {
		v := rec.CheckedAt.Format(time.RFC3339)
		item.CheckedAt = &v
	}
	if item.Evidence == nil {
		item.Evidence = []ActionEvidence{}
	}
	if item.CTAs == nil {
		item.CTAs = []ActionCTA{}
	}
	// Authoritative affected count is the validated entity list length — never
	// trust a stale affected_objects_total from payload when they disagree.
	entityCount := countAffectedEntityRefs(item.AffectedEntities)
	if item.OperationalDetail != nil {
		if objectN := len(item.OperationalDetail.ObjectActions); objectN > entityCount {
			entityCount = objectN
		}
	}
	if entityCount > 0 {
		item.AffectedObjectsTotal = entityCount
		if item.AffectedObjectsDisplayed <= 0 || item.AffectedObjectsDisplayed > entityCount {
			item.AffectedObjectsDisplayed = entityCount
		}
	}
	return item, nil
}

func countAffectedEntityRefs(entities ActionAffectedEntities) int {
	n := len(entities.SKUs) + len(entities.Campaigns)
	if entities.Funnel != nil {
		n++
	}
	return n
}

func parseEvidenceJSON(raw []byte) ([]ActionEvidence, error) {
	if len(raw) == 0 {
		return []ActionEvidence{}, nil
	}
	var out []ActionEvidence
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseCTAJSON(raw []byte) ([]ActionCTA, error) {
	if len(raw) == 0 {
		return []ActionCTA{}, nil
	}
	var out []ActionCTA
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseAffectedEntitiesJSON(raw []byte) (ActionAffectedEntities, error) {
	if len(raw) == 0 {
		return ActionAffectedEntities{}, nil
	}
	var out ActionAffectedEntities
	if err := json.Unmarshal(raw, &out); err != nil {
		return ActionAffectedEntities{}, err
	}
	return out, nil
}

func parseStringSliceJSON(raw []byte) []string {
	out, err := parseStringSliceJSONErr(raw)
	if err != nil {
		log.Printf("actions: invalid string slice json: %v", err)
		return nil
	}
	return out
}

func parseStringSliceJSONErr(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseAIFocusJSON(raw []byte) *ActionsAIFocus {
	if len(raw) == 0 {
		return nil
	}
	var focus ActionsAIFocus
	if err := json.Unmarshal(raw, &focus); err != nil {
		log.Printf("actions: invalid ai_focus json: %v", err)
		return nil
	}
	return &focus
}

func priorityOrder(priority ActionPriority) int {
	switch priority {
	case ActionPriorityCritical:
		return 0
	case ActionPriorityImportant:
		return 1
	case ActionPriorityWatch:
		return 2
	default:
		return 3
	}
}
