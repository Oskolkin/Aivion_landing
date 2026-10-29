package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/publishedsnapshot"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ActionGenerationRunRecord struct {
	ID                   int64
	SellerAccountID      int64
	MetricDate           *time.Time
	Status               string
	FinalStatus          string
	AIFocusJSON          []byte
	DataQualityNotesJSON []byte
	TopActionKeysJSON    []byte
	AiPipelineRunID      int64
	DataPackHash         string
	GlobalContextHash    string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type ActionItemRecord struct {
	ID                   int64
	SellerAccountID      int64
	RunID                *int64
	ActionKey            string
	Type                 ActionType
	Category             ActionCategory
	Priority             ActionPriority
	Status               ActionStatus
	Title                string
	Summary              string
	Reason               string
	NextStep             string
	ExpectedEffect       string
	EvidenceJSON         []byte
	LimitationsJSON      []byte
	CTAJSON              []byte
	AffectedEntitiesJSON []byte
	EntityGroupRef       string
	SourceCandidateIDs   []byte
	EvidenceFactIDs      []byte
	PayloadJSON          []byte
	MetricDate           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	CheckedAt            *time.Time
}

type ListActionsParams struct {
	SellerAccountID int64
	Status          string
	Category        *ActionCategory
	Priority        *ActionPriority
	MetricDate      *time.Time
	Limit           int
	Offset          int
}

type AdminListGenerationRunsParams struct {
	SellerAccountID *int64
	Status          string
	FinalStatus     string
	MetricDate      *time.Time
	Limit           int
	Offset          int
}

type Repository interface {
	GetLatestRun(ctx context.Context, sellerAccountID int64) (*ActionGenerationRunRecord, error)
	GetLatestPublishableRun(ctx context.Context, sellerAccountID int64) (*ActionGenerationRunRecord, error)
	GetPublishableRunByPipelineID(ctx context.Context, sellerAccountID, pipelineRunID int64) (*ActionGenerationRunRecord, error)
	GetLatestMetricDate(ctx context.Context, sellerAccountID int64) (*time.Time, error)
	ListActionsByRunID(ctx context.Context, runID int64, params ListActionsParams) ([]ActionItemRecord, error)
	ListActions(ctx context.Context, params ListActionsParams) ([]ActionItemRecord, error)
	GetActionByKey(ctx context.Context, sellerAccountID int64, actionKey string) (*ActionItemRecord, error)
	GetActionByID(ctx context.Context, sellerAccountID, actionID int64) (*ActionItemRecord, error)
	GetHistoricalActionByID(ctx context.Context, sellerAccountID, actionID int64) (*ActionItemRecord, error)
	GetActionEntityGroup(ctx context.Context, sellerAccountID, actionsRunID int64, groupKey string) (*ActionEntityGroup, error)
	GetGenerationRunByID(ctx context.Context, runID int64) (*ActionGenerationRunRecord, error)
	SetActionChecked(ctx context.Context, sellerAccountID int64, actionKey string) error
	SetActionOpen(ctx context.Context, sellerAccountID int64, actionKey string) error
	SetActionDismissed(ctx context.Context, sellerAccountID int64, actionKey string) error
}

type AdminRepository interface {
	ListGenerationRunsForAdmin(ctx context.Context, params AdminListGenerationRunsParams) ([]dbgen.ActionGenerationRun, error)
	GetGenerationRunForAdmin(ctx context.Context, runID int64) (*dbgen.ActionGenerationRun, error)
	ListGenerationCategoryRunsForAdmin(ctx context.Context, runID int64) ([]dbgen.ActionGenerationCategoryRun, error)
}

type SQLCRepository struct {
	queries  *dbgen.Queries
	resolver publishedsnapshot.Resolver
}

func NewSQLCRepository(queries *dbgen.Queries) *SQLCRepository {
	return &SQLCRepository{
		queries:  queries,
		resolver: publishedsnapshot.NewSQLCResolver(queries),
	}
}

func (r *SQLCRepository) GetLatestPublishableRun(ctx context.Context, sellerAccountID int64) (*ActionGenerationRunRecord, error) {
	snapshot, err := r.resolver.GetCurrentPublishedSnapshot(ctx, sellerAccountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("resolve current published snapshot: %w", err)
	}
	if err := publishedsnapshot.ValidateSnapshot(snapshot, sellerAccountID); err != nil {
		return nil, fmt.Errorf("invalid published snapshot: %w", err)
	}
	row, err := r.queries.GetActionGenerationRunForAdmin(ctx, snapshot.ActionsRunID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get action generation run by snapshot: %w", err)
	}
	return mapGenerationRun(row), nil
}

func (r *SQLCRepository) GetPublishableRunByPipelineID(ctx context.Context, sellerAccountID, pipelineRunID int64) (*ActionGenerationRunRecord, error) {
	if pipelineRunID <= 0 {
		return nil, nil
	}
	snapshot, err := r.resolver.GetPublishedSnapshotByPipelineID(ctx, sellerAccountID, pipelineRunID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("resolve published snapshot by pipeline: %w", err)
	}
	if err := publishedsnapshot.ValidateSnapshot(snapshot, sellerAccountID); err != nil {
		return nil, fmt.Errorf("invalid published snapshot: %w", err)
	}
	row, err := r.queries.GetActionGenerationRunForAdmin(ctx, snapshot.ActionsRunID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get action generation run by pipeline snapshot: %w", err)
	}
	return mapGenerationRun(row), nil
}

func (r *SQLCRepository) GetLatestRun(ctx context.Context, sellerAccountID int64) (*ActionGenerationRunRecord, error) {
	row, err := r.queries.GetLatestActionGenerationRun(ctx, sellerAccountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest action generation run: %w", err)
	}
	return mapGenerationRun(row), nil
}

func (r *SQLCRepository) GetLatestMetricDate(ctx context.Context, sellerAccountID int64) (*time.Time, error) {
	run, err := r.GetLatestPublishableRun(ctx, sellerAccountID)
	if err != nil {
		return nil, err
	}
	if run == nil || run.MetricDate == nil {
		return nil, nil
	}
	t := *run.MetricDate
	return &t, nil
}

func (r *SQLCRepository) ListActionsByRunID(ctx context.Context, runID int64, params ListActionsParams) ([]ActionItemRecord, error) {
	rows, err := r.queries.ListActionItemsWithUserStateByRunID(ctx, pgtype.Int8{Int64: runID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list action items by run: %w", err)
	}
	out := make([]ActionItemRecord, 0, len(rows))
	for _, row := range rows {
		rec := mapActionItemRunRow(row)
		if params.Category != nil && rec.Category != *params.Category {
			continue
		}
		if params.Priority != nil && rec.Priority != *params.Priority {
			continue
		}
		if params.Status != "" && params.Status != "all" {
			switch params.Status {
			case string(ActionStatusOpen):
				if rec.Status != ActionStatusOpen {
					continue
				}
			case string(ActionStatusChecked):
				if rec.Status != ActionStatusChecked {
					continue
				}
			case string(ActionStatusDismissed):
				if rec.Status != ActionStatusDismissed {
					continue
				}
			}
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *SQLCRepository) ListActions(ctx context.Context, params ListActionsParams) ([]ActionItemRecord, error) {
	rows, err := r.queries.ListActionItemsBySeller(ctx, dbgen.ListActionItemsBySellerParams{
		SellerAccountID: params.SellerAccountID,
		MetricDate:      toDate(params.MetricDate),
		Category:        nullableCategory(params.Category),
		Priority:        nullablePriority(params.Priority),
		Status:          nullableTextValue(params.Status),
		OffsetCount:     normalizeOffset(params.Offset),
		LimitCount:      normalizeLimit(params.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list action items: %w", err)
	}
	out := make([]ActionItemRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapActionItemRow(row))
	}
	return out, nil
}

func (r *SQLCRepository) GetActionByKey(ctx context.Context, sellerAccountID int64, actionKey string) (*ActionItemRecord, error) {
	run, err := r.GetLatestPublishableRun(ctx, sellerAccountID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, ErrActionNotFound
	}
	row, err := r.queries.GetActionItemByKey(ctx, dbgen.GetActionItemByKeyParams{
		SellerAccountID: sellerAccountID,
		ActionKey:       actionKey,
		RunID:           pgtype.Int8{Int64: run.ID, Valid: true},
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrActionNotFound
		}
		return nil, fmt.Errorf("get action by key: %w", err)
	}
	item := mapActionItemKeyRow(row)
	return &item, nil
}

func (r *SQLCRepository) GetActionByID(ctx context.Context, sellerAccountID, actionID int64) (*ActionItemRecord, error) {
	snapshot, err := r.resolver.GetCurrentPublishedSnapshot(ctx, sellerAccountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrActionNotFound
		}
		return nil, fmt.Errorf("resolve current published snapshot: %w", err)
	}
	row, err := r.queries.GetActionItemByIDAndRunIDForSeller(ctx, dbgen.GetActionItemByIDAndRunIDForSellerParams{
		SellerAccountID: sellerAccountID,
		ID:              actionID,
		RunID:           pgtype.Int8{Int64: snapshot.ActionsRunID, Valid: true},
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrActionNotFound
		}
		return nil, fmt.Errorf("get action by id: %w", err)
	}
	item := mapActionItemIDByRunRow(row)
	return &item, nil
}

func (r *SQLCRepository) GetHistoricalActionByID(ctx context.Context, sellerAccountID, actionID int64) (*ActionItemRecord, error) {
	row, err := r.queries.GetHistoricalActionItemByIDForSeller(ctx, dbgen.GetHistoricalActionItemByIDForSellerParams{
		SellerAccountID: sellerAccountID,
		ID:              actionID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrActionNotFound
		}
		return nil, fmt.Errorf("get historical action by id: %w", err)
	}
	item := mapActionItemHistoricalIDRow(row)
	return &item, nil
}

func mapActionItemIDByRunRow(row dbgen.GetActionItemByIDAndRunIDForSellerRow) ActionItemRecord {
	return mapActionItemCommon(
		row.ID, row.SellerAccountID, row.RunID, row.ActionKey,
		row.Type, row.Category, row.Priority, row.UserStatus,
		row.Title, row.Summary, row.Reason, row.NextStep, row.ExpectedEffect,
		row.EvidenceJson, row.LimitationsJson, row.CtaJson, row.AffectedEntitiesJson,
		row.EntityGroupRef, row.SourceCandidateIdsJson, row.EvidenceFactIdsJson,
		row.PayloadJson,
		row.MetricDate, row.CreatedAt, row.UpdatedAt, row.CheckedAt,
	)
}

func mapActionItemHistoricalIDRow(row dbgen.GetHistoricalActionItemByIDForSellerRow) ActionItemRecord {
	return mapActionItemCommon(
		row.ID, row.SellerAccountID, row.RunID, row.ActionKey,
		row.Type, row.Category, row.Priority, row.UserStatus,
		row.Title, row.Summary, row.Reason, row.NextStep, row.ExpectedEffect,
		row.EvidenceJson, row.LimitationsJson, row.CtaJson, row.AffectedEntitiesJson,
		row.EntityGroupRef, row.SourceCandidateIdsJson, row.EvidenceFactIdsJson,
		row.PayloadJson,
		row.MetricDate, row.CreatedAt, row.UpdatedAt, row.CheckedAt,
	)
}

func mapActionItemIDRow(row dbgen.GetActionItemByIDForSellerRow) ActionItemRecord {
	return mapActionItemCommon(
		row.ID, row.SellerAccountID, row.RunID, row.ActionKey,
		row.Type, row.Category, row.Priority, row.UserStatus,
		row.Title, row.Summary, row.Reason, row.NextStep, row.ExpectedEffect,
		row.EvidenceJson, row.LimitationsJson, row.CtaJson, row.AffectedEntitiesJson,
		row.EntityGroupRef, row.SourceCandidateIdsJson, row.EvidenceFactIdsJson,
		row.PayloadJson,
		row.MetricDate, row.CreatedAt, row.UpdatedAt, row.CheckedAt,
	)
}

func (r *SQLCRepository) GetActionEntityGroup(ctx context.Context, sellerAccountID, actionsRunID int64, groupKey string) (*ActionEntityGroup, error) {
	groupKey = strings.TrimSpace(groupKey)
	if groupKey == "" || actionsRunID <= 0 {
		return nil, ErrEntityGroupNotFound
	}
	row, err := r.queries.GetActionEntityGroup(ctx, dbgen.GetActionEntityGroupParams{
		SellerAccountID:       sellerAccountID,
		ActionGenerationRunID: actionsRunID,
		GroupKey:              groupKey,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrEntityGroupNotFound
		}
		return nil, fmt.Errorf("get action entity group: %w", err)
	}
	var entityIDs []string
	if len(row.EntityIdsJson) > 0 {
		_ = json.Unmarshal(row.EntityIdsJson, &entityIDs)
	}
	var summary map[string]any
	if len(row.SummaryJson) > 0 {
		_ = json.Unmarshal(row.SummaryJson, &summary)
	}
	return &ActionEntityGroup{
		GroupKey:   row.GroupKey,
		EntityType: row.EntityType,
		EntityIDs:  entityIDs,
		Summary:    summary,
	}, nil
}

func (r *SQLCRepository) SetActionChecked(ctx context.Context, sellerAccountID int64, actionKey string) error {
	if _, err := r.GetActionByKey(ctx, sellerAccountID, actionKey); err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := r.queries.UpsertActionUserState(ctx, dbgen.UpsertActionUserStateParams{
		SellerAccountID: sellerAccountID,
		ActionKey:       actionKey,
		Status:          string(ActionStatusChecked),
		CheckedAt:       timestamptz(now),
		SnoozedUntil:    pgtype.Timestamptz{},
		DismissedAt:     pgtype.Timestamptz{},
	}); err != nil {
		return fmt.Errorf("set action checked: %w", err)
	}
	return nil
}

func (r *SQLCRepository) SetActionOpen(ctx context.Context, sellerAccountID int64, actionKey string) error {
	if _, err := r.GetActionByKey(ctx, sellerAccountID, actionKey); err != nil {
		return err
	}
	if err := r.queries.UpsertActionUserState(ctx, dbgen.UpsertActionUserStateParams{
		SellerAccountID: sellerAccountID,
		ActionKey:       actionKey,
		Status:          string(ActionStatusOpen),
		CheckedAt:       pgtype.Timestamptz{},
		SnoozedUntil:    pgtype.Timestamptz{},
		DismissedAt:     pgtype.Timestamptz{},
	}); err != nil {
		return fmt.Errorf("set action open: %w", err)
	}
	return nil
}

func (r *SQLCRepository) SetActionDismissed(ctx context.Context, sellerAccountID int64, actionKey string) error {
	if _, err := r.GetActionByKey(ctx, sellerAccountID, actionKey); err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := r.queries.UpsertActionUserState(ctx, dbgen.UpsertActionUserStateParams{
		SellerAccountID: sellerAccountID,
		ActionKey:       actionKey,
		Status:          string(ActionStatusDismissed),
		CheckedAt:       pgtype.Timestamptz{},
		SnoozedUntil:    pgtype.Timestamptz{},
		DismissedAt:     timestamptz(now),
	}); err != nil {
		return fmt.Errorf("set action dismissed: %w", err)
	}
	return nil
}

func (r *SQLCRepository) ListGenerationRunsForAdmin(ctx context.Context, params AdminListGenerationRunsParams) ([]dbgen.ActionGenerationRun, error) {
	rows, err := r.queries.ListActionGenerationRunsForAdmin(ctx, dbgen.ListActionGenerationRunsForAdminParams{
		SellerAccountID: nullableInt8(params.SellerAccountID),
		Status:          nullableTextValue(params.Status),
		FinalStatus:     nullableTextValue(params.FinalStatus),
		MetricDate:      toDate(params.MetricDate),
		LimitCount:      normalizeLimit(params.Limit),
		OffsetCount:     normalizeOffset(params.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list action generation runs for admin: %w", err)
	}
	return rows, nil
}

func (r *SQLCRepository) GetGenerationRunByID(ctx context.Context, runID int64) (*ActionGenerationRunRecord, error) {
	row, err := r.queries.GetActionGenerationRunForAdmin(ctx, runID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrActionNotFound
		}
		return nil, fmt.Errorf("get action generation run: %w", err)
	}
	return mapGenerationRun(row), nil
}

func (r *SQLCRepository) GetGenerationRunForAdmin(ctx context.Context, runID int64) (*dbgen.ActionGenerationRun, error) {
	row, err := r.queries.GetActionGenerationRunForAdmin(ctx, runID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrActionNotFound
		}
		return nil, fmt.Errorf("get action generation run for admin: %w", err)
	}
	return &row, nil
}

func (r *SQLCRepository) ListGenerationCategoryRunsForAdmin(ctx context.Context, runID int64) ([]dbgen.ActionGenerationCategoryRun, error) {
	rows, err := r.queries.ListActionGenerationCategoryRunsForAdmin(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("list action generation category runs for admin: %w", err)
	}
	return rows, nil
}

func mapGenerationRun(row dbgen.ActionGenerationRun) *ActionGenerationRunRecord {
	rec := &ActionGenerationRunRecord{
		ID:                   row.ID,
		SellerAccountID:      row.SellerAccountID,
		MetricDate:           datePtr(row.MetricDate),
		Status:               row.Status,
		AIFocusJSON:          row.AiFocusJson,
		DataQualityNotesJSON: row.DataQualityNotesJson,
		TopActionKeysJSON:    row.TopActionKeysJson,
		CreatedAt:            timestamptzTime(row.CreatedAt),
		UpdatedAt:            timestamptzTime(row.UpdatedAt),
	}
	if row.FinalStatus.Valid {
		rec.FinalStatus = row.FinalStatus.String
	}
	if row.AiPipelineRunID.Valid {
		rec.AiPipelineRunID = row.AiPipelineRunID.Int64
	}
	if row.DataPackHash.Valid {
		rec.DataPackHash = strings.TrimSpace(row.DataPackHash.String)
	}
	if row.GlobalContextHash.Valid {
		rec.GlobalContextHash = strings.TrimSpace(row.GlobalContextHash.String)
	}
	return rec
}

func mapActionItemRow(row dbgen.ListActionItemsBySellerRow) ActionItemRecord {
	return mapActionItemCommon(
		row.ID, row.SellerAccountID, row.RunID, row.ActionKey,
		row.Type, row.Category, row.Priority, row.UserStatus,
		row.Title, row.Summary, row.Reason, row.NextStep, row.ExpectedEffect,
		row.EvidenceJson, row.LimitationsJson, row.CtaJson, row.AffectedEntitiesJson,
		row.EntityGroupRef, row.SourceCandidateIdsJson, row.EvidenceFactIdsJson,
		row.PayloadJson,
		row.MetricDate, row.CreatedAt, row.UpdatedAt, row.CheckedAt,
	)
}

func mapActionItemRunRow(row dbgen.ListActionItemsWithUserStateByRunIDRow) ActionItemRecord {
	return mapActionItemCommon(
		row.ID, row.SellerAccountID, row.RunID, row.ActionKey,
		row.Type, row.Category, row.Priority, row.UserStatus,
		row.Title, row.Summary, row.Reason, row.NextStep, row.ExpectedEffect,
		row.EvidenceJson, row.LimitationsJson, row.CtaJson, row.AffectedEntitiesJson,
		row.EntityGroupRef, row.SourceCandidateIdsJson, row.EvidenceFactIdsJson,
		row.PayloadJson,
		row.MetricDate, row.CreatedAt, row.UpdatedAt, row.CheckedAt,
	)
}

func mapActionItemKeyRow(row dbgen.GetActionItemByKeyRow) ActionItemRecord {
	return mapActionItemCommon(
		row.ID, row.SellerAccountID, row.RunID, row.ActionKey,
		row.Type, row.Category, row.Priority, row.UserStatus,
		row.Title, row.Summary, row.Reason, row.NextStep, row.ExpectedEffect,
		row.EvidenceJson, row.LimitationsJson, row.CtaJson, row.AffectedEntitiesJson,
		row.EntityGroupRef, row.SourceCandidateIdsJson, row.EvidenceFactIdsJson,
		row.PayloadJson,
		row.MetricDate, row.CreatedAt, row.UpdatedAt, row.CheckedAt,
	)
}

func mapActionItemCommon(
	id, sellerAccountID int64,
	runID pgtype.Int8,
	actionKey, typ, category, priority, userStatus string,
	title string, summary pgtype.Text,
	reason, nextStep string, expectedEffect pgtype.Text,
	evidenceJSON, limitationsJSON, ctaJSON, affectedEntitiesJSON []byte,
	entityGroupRef pgtype.Text,
	sourceCandidateIDsJSON, evidenceFactIDsJSON []byte,
	payloadJSON []byte,
	metricDate pgtype.Date,
	createdAt, updatedAt pgtype.Timestamptz,
	checkedAt pgtype.Timestamptz,
) ActionItemRecord {
	var runIDPtr *int64
	if runID.Valid {
		v := runID.Int64
		runIDPtr = &v
	}
	return ActionItemRecord{
		ID:                   id,
		SellerAccountID:      sellerAccountID,
		RunID:                runIDPtr,
		ActionKey:            actionKey,
		Type:                 ActionType(typ),
		Category:             ActionCategory(category),
		Priority:             ActionPriority(priority),
		Status:               ActionStatus(userStatus),
		Title:                title,
		Summary:              textString(summary),
		Reason:               reason,
		NextStep:             nextStep,
		ExpectedEffect:       textString(expectedEffect),
		EvidenceJSON:         evidenceJSON,
		LimitationsJSON:      limitationsJSON,
		CTAJSON:              ctaJSON,
		AffectedEntitiesJSON: affectedEntitiesJSON,
		EntityGroupRef:       textString(entityGroupRef),
		SourceCandidateIDs:   sourceCandidateIDsJSON,
		EvidenceFactIDs:      evidenceFactIDsJSON,
		PayloadJSON:          payloadJSON,
		MetricDate:           datePtr(metricDate),
		CreatedAt:            timestamptzTime(createdAt),
		UpdatedAt:            timestamptzTime(updatedAt),
		CheckedAt:            timestamptzPtr(checkedAt),
	}
}

func toDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{
		Time:  time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
}

func datePtr(v pgtype.Date) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.Date(v.Time.Year(), v.Time.Month(), v.Time.Day(), 0, 0, 0, 0, time.UTC)
	return &t
}

func nullableCategory(v *ActionCategory) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: string(*v), Valid: true}
}

func nullablePriority(v *ActionPriority) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: string(*v), Valid: true}
}

func nullableTextValue(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

func nullableInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func textString(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func timestamptzTime(v pgtype.Timestamptz) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time.UTC()
}

func timestamptzPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

func normalizeLimit(limit int) int32 {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return int32(limit)
}

func normalizeOffset(offset int) int32 {
	if offset < 0 {
		return 0
	}
	return int32(offset)
}
