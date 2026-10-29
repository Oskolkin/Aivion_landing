package aipipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type Store struct {
	q *dbgen.Queries
}

func NewStore(q *dbgen.Queries) *Store {
	return &Store{q: q}
}

type CreatePipelineRunParams struct {
	SellerAccountID       int64
	MetricDate            time.Time
	Mode                  string
	Trigger               string
	Status                string
	DataPackSchemaVersion string
	DataPackHash          string
	StartedAt             time.Time
}

func (s *Store) CreatePipelineRun(ctx context.Context, params CreatePipelineRunParams) (int64, error) {
	row, err := s.q.CreateAIPipelineRun(ctx, dbgen.CreateAIPipelineRunParams{
		SellerAccountID:       params.SellerAccountID,
		MetricDate:            toDate(params.MetricDate),
		Mode:                  params.Mode,
		Trigger:               params.Trigger,
		Status:                params.Status,
		DataPackSchemaVersion: nullableText(params.DataPackSchemaVersion),
		DataPackHash:          params.DataPackHash,
		StartedAt:             pgtype.Timestamptz{Time: params.StartedAt.UTC(), Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("create pipeline run: %w", err)
	}
	return row.ID, nil
}

type UpdatePipelineRunParams struct {
	ID                         int64
	Status                     string
	DataPackSchemaVersion      string
	DataPackHash               string
	GlobalContextSchemaVersion string
	GlobalContextHash          string
	GlobalContextStatus        string
	DashboardRunID             string
	ActionsRunID               int64
	DashboardStatus            string
	ActionsStatus              string
	ErrorCode                  string
	ErrorMessage               string
	CompletedAt                *time.Time
	PublishedAt                *time.Time
}

func (s *Store) UpdatePipelineRun(ctx context.Context, params UpdatePipelineRunParams) error {
	var dashboardUUID pgtype.UUID
	if params.DashboardRunID != "" {
		if err := dashboardUUID.Scan(params.DashboardRunID); err != nil {
			return fmt.Errorf("parse dashboard_run_id: %w", err)
		}
	}
	var actionsRunID pgtype.Int8
	if params.ActionsRunID > 0 {
		actionsRunID = pgtype.Int8{Int64: params.ActionsRunID, Valid: true}
	}
	var completedAt pgtype.Timestamptz
	if params.CompletedAt != nil {
		completedAt = pgtype.Timestamptz{Time: params.CompletedAt.UTC(), Valid: true}
	}
	var publishedAt pgtype.Timestamptz
	if params.PublishedAt != nil {
		publishedAt = pgtype.Timestamptz{Time: params.PublishedAt.UTC(), Valid: true}
	}
	_, err := s.q.UpdateAIPipelineRun(ctx, dbgen.UpdateAIPipelineRunParams{
		ID:                         params.ID,
		Status:                     nullableText(params.Status),
		DataPackSchemaVersion:      nullableText(params.DataPackSchemaVersion),
		DataPackHash:               nullableText(params.DataPackHash),
		GlobalContextSchemaVersion: nullableText(params.GlobalContextSchemaVersion),
		GlobalContextHash:          nullableText(params.GlobalContextHash),
		GlobalContextStatus:        nullableText(params.GlobalContextStatus),
		DashboardRunID:             dashboardUUID,
		ActionsRunID:               actionsRunID,
		DashboardStatus:            nullableText(params.DashboardStatus),
		ActionsStatus:              nullableText(params.ActionsStatus),
		ErrorCode:                  nullableText(params.ErrorCode),
		ErrorMessage:               nullableText(params.ErrorMessage),
		CompletedAt:                completedAt,
		PublishedAt:                publishedAt,
	})
	if err != nil {
		return fmt.Errorf("update pipeline run: %w", err)
	}
	return nil
}

type CreateArtifactParams struct {
	PipelineRunID int64
	Kind          string
	Hash          string
	Payload       any
}

func (s *Store) CreateArtifact(ctx context.Context, params CreateArtifactParams) (int64, error) {
	var payloadJSON []byte
	if params.Payload != nil {
		raw, err := json.Marshal(params.Payload)
		if err != nil {
			return 0, fmt.Errorf("marshal artifact payload: %w", err)
		}
		payloadJSON = raw
	}
	row, err := s.q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: params.PipelineRunID,
		Kind:          params.Kind,
		Hash:          nullableText(params.Hash),
		Storage:       "db",
		ObjectKey:     pgtype.Text{},
		PayloadJson:   payloadJSON,
	})
	if err != nil {
		return 0, fmt.Errorf("create pipeline artifact: %w", err)
	}
	return row.ID, nil
}

func (s *Store) ListArtifacts(ctx context.Context, pipelineRunID int64) ([]dbgen.AiPipelineArtifact, error) {
	return s.q.ListAIPipelineArtifacts(ctx, pipelineRunID)
}

func (s *Store) ListPipelineArtifacts(ctx context.Context, pipelineRunID int64) ([]dbgen.AiPipelineArtifact, error) {
	return s.ListArtifacts(ctx, pipelineRunID)
}

func (s *Store) GetPipelineRun(ctx context.Context, id int64) (dbgen.AiPipelineRun, error) {
	return s.q.GetAIPipelineRun(ctx, id)
}

func (s *Store) GetPublishedAIPipelineRunBySellerDateAndDataPackHash(
	ctx context.Context,
	arg dbgen.GetPublishedAIPipelineRunBySellerDateAndDataPackHashParams,
) (dbgen.AiPipelineRun, error) {
	return s.q.GetPublishedAIPipelineRunBySellerDateAndDataPackHash(ctx, arg)
}

func (s *Store) GetDashboardNarrativeHashes(ctx context.Context, narrativeID string, sellerAccountID int64) (dataPackHash, globalContextHash string, err error) {
	var id pgtype.UUID
	if err := id.Scan(narrativeID); err != nil {
		return "", "", fmt.Errorf("parse narrative id: %w", err)
	}
	row, err := s.q.GetAIDashboardNarrativeByID(ctx, dbgen.GetAIDashboardNarrativeByIDParams{
		ID:              id,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		return "", "", err
	}
	return textOrEmpty(row.DataPackHash), textOrEmpty(row.GlobalContextHash), nil
}

func (s *Store) GetActionGenerationRunHashes(ctx context.Context, actionsRunID, sellerAccountID int64) (dataPackHash, globalContextHash string, err error) {
	row, err := s.q.GetActionGenerationRunForAdmin(ctx, actionsRunID)
	if err != nil {
		return "", "", err
	}
	if row.SellerAccountID != sellerAccountID {
		return "", "", fmt.Errorf("actions run seller mismatch")
	}
	return textOrEmpty(row.DataPackHash), textOrEmpty(row.GlobalContextHash), nil
}

func (s *Store) GetActionGenerationRunFinalStatus(ctx context.Context, actionsRunID, sellerAccountID int64) (string, error) {
	row, err := s.q.GetActionGenerationRunForAdmin(ctx, actionsRunID)
	if err != nil {
		return "", err
	}
	if row.SellerAccountID != sellerAccountID {
		return "", fmt.Errorf("actions run seller mismatch")
	}
	if status := strings.TrimSpace(textOrEmpty(row.FinalStatus)); status != "" {
		return status, nil
	}
	return strings.TrimSpace(row.Status), nil
}

func (s *Store) LinkDashboardRun(ctx context.Context, narrativeID string, sellerID, pipelineRunID int64, dataPackHash, globalContextHash string) error {
	var id pgtype.UUID
	if err := id.Scan(narrativeID); err != nil {
		return fmt.Errorf("parse narrative id: %w", err)
	}
	if err := s.q.ClearAIDashboardNarrativePipelineLinkExcept(ctx, dbgen.ClearAIDashboardNarrativePipelineLinkExceptParams{
		AiPipelineRunID: pgtype.Int8{Int64: pipelineRunID, Valid: true},
		ID:              id,
	}); err != nil {
		return fmt.Errorf("clear previous dashboard pipeline link: %w", err)
	}
	_, err := s.q.UpdateAIDashboardNarrativePipelineFields(ctx, dbgen.UpdateAIDashboardNarrativePipelineFieldsParams{
		ID:                id,
		SellerAccountID:   sellerID,
		AiPipelineRunID:   pgtype.Int8{Int64: pipelineRunID, Valid: true},
		DataPackHash:      nullableText(dataPackHash),
		GlobalContextHash: nullableText(globalContextHash),
	})
	return err
}

type PersistFakeDashboardParams struct {
	SellerAccountID   int64
	MetricDate        time.Time
	ForceRegenerate   bool
	DataPackHash      string
	GlobalContextHash string
	PipelineRunID     *int64
	Payload           any
	HealthScore       int32
	Severity          string
}

func (s *Store) PersistFakeDashboardNarrative(ctx context.Context, params PersistFakeDashboardParams) (string, error) {
	if s == nil || s.q == nil {
		return "", fmt.Errorf("store is not configured")
	}
	now := time.Now().UTC()
	row, err := s.q.InsertAIDashboardNarrative(ctx, dbgen.InsertAIDashboardNarrativeParams{
		SellerAccountID: params.SellerAccountID,
		MetricDate:      toDate(params.MetricDate),
		Status:          "running",
		HealthScore:     pgtype.Int4{Int32: params.HealthScore, Valid: true},
		Severity:        pgtype.Text{String: params.Severity, Valid: params.Severity != ""},
		RiskDriversHash: params.GlobalContextHash,
		ContextHash:     params.DataPackHash,
		PromptVersion:   dashboardhome.DashboardNarrativePromptVersion,
		Model:           pgtype.Text{String: fakeDashboardModel, Valid: true},
		StartedAt:       pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return "", fmt.Errorf("insert fake dashboard narrative: %w", err)
	}

	validatedBytes, err := json.Marshal(params.Payload)
	if err != nil {
		return "", fmt.Errorf("marshal fake dashboard payload: %w", err)
	}
	rawResponse, err := json.Marshal(map[string]any{
		"mode":  "fake_dashboard_deterministic",
		"model": fakeDashboardModel,
	})
	if err != nil {
		return "", fmt.Errorf("marshal fake dashboard raw response: %w", err)
	}

	_, err = s.q.CompleteAIDashboardNarrative(ctx, dbgen.CompleteAIDashboardNarrativeParams{
		ID:               row.ID,
		SellerAccountID:  params.SellerAccountID,
		Status:           "completed",
		RawAiResponse:    pgtype.Text{String: string(rawResponse), Valid: true},
		ValidatedPayload: validatedBytes,
		ValidationErrors: []byte("[]"),
		FinishedAt:       pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return "", fmt.Errorf("complete fake dashboard narrative: %w", err)
	}
	if params.PipelineRunID != nil && *params.PipelineRunID > 0 {
		if err := s.persistFakeDashboardDataSnapshot(ctx, *params.PipelineRunID, params); err != nil {
			return "", err
		}
	}
	return row.ID.String(), nil
}

func (s *Store) persistFakeDashboardDataSnapshot(ctx context.Context, pipelineRunID int64, params PersistFakeDashboardParams) error {
	metricDateStr := params.MetricDate.UTC().Format("2006-01-02")
	dto := analytics.DashboardMetricsDTO{
		SellerAccountID: params.SellerAccountID,
		AsOfDate:        metricDateStr,
		Account: analytics.DashboardAccountMetrics{
			RevenueToday: 1000,
			OrdersToday:  5,
		},
	}
	frozen := dashboardhome.NewFrozenDashboardData(metricDateStr, dto, nil)
	payloadHash, err := frozen.CanonicalHash()
	if err != nil {
		return fmt.Errorf("hash fake dashboard data snapshot: %w", err)
	}
	payloadJSON, err := json.Marshal(frozen)
	if err != nil {
		return fmt.Errorf("marshal fake dashboard data snapshot: %w", err)
	}
	now := time.Now().UTC()
	row, err := s.q.InsertAIDashboardDataSnapshot(ctx, dbgen.InsertAIDashboardDataSnapshotParams{
		AiPipelineRunID:   pipelineRunID,
		SellerAccountID:   params.SellerAccountID,
		MetricDate:        toDate(params.MetricDate),
		DataPackHash:      params.DataPackHash,
		GlobalContextHash: params.GlobalContextHash,
		SchemaVersion:     dashboardhome.FrozenDashboardDataSchemaVersion,
		PayloadJson:       payloadJSON,
		PayloadHash:       payloadHash,
		Status:            "completed",
		FinishedAt:        pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("insert fake dashboard data snapshot: %w", err)
	}
	if _, err := s.q.LinkAIPipelineRunDashboardDataSnapshot(ctx, dbgen.LinkAIPipelineRunDashboardDataSnapshotParams{
		ID:                      pipelineRunID,
		DashboardDataSnapshotID: pgtype.Int8{Int64: row.ID, Valid: true},
	}); err != nil {
		return fmt.Errorf("link fake dashboard data snapshot: %w", err)
	}
	return nil
}

func (s *Store) LinkActionsRun(ctx context.Context, actionsRunID, pipelineRunID int64, dataPackHash, globalContextHash string) error {
	if err := s.q.ClearActionGenerationRunPipelineLinkExcept(ctx, dbgen.ClearActionGenerationRunPipelineLinkExceptParams{
		AiPipelineRunID: pgtype.Int8{Int64: pipelineRunID, Valid: true},
		ID:              actionsRunID,
	}); err != nil {
		return fmt.Errorf("clear previous actions pipeline link: %w", err)
	}
	_, err := s.q.UpdateActionGenerationRunPipelineFields(ctx, dbgen.UpdateActionGenerationRunPipelineFieldsParams{
		ID:                actionsRunID,
		AiPipelineRunID:   pgtype.Int8{Int64: pipelineRunID, Valid: true},
		DataPackHash:      nullableText(dataPackHash),
		GlobalContextHash: nullableText(globalContextHash),
	})
	return err
}

func toDate(t time.Time) pgtype.Date {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return pgtype.Date{Time: t, Valid: true}
}

func nullableText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}
