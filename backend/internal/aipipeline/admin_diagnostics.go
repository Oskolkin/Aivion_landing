package aipipeline

import (
	"context"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// AdminPipelineDiagnostics is the admin-facing pipeline trace for Dashboard ↔ Actions consistency checks.
type AdminPipelineDiagnostics struct {
	PipelineRunID       int64                   `json:"pipeline_run_id"`
	SellerAccountID     int64                   `json:"seller_account_id"`
	MetricDate          string                  `json:"metric_date,omitempty"`
	DataPackHash        string                  `json:"data_pack_hash,omitempty"`
	GlobalContextHash   string                  `json:"global_context_hash,omitempty"`
	DashboardRunID      string                  `json:"dashboard_run_id,omitempty"`
	ActionsRunID        int64                   `json:"actions_run_id,omitempty"`
	DashboardStatus     string                  `json:"dashboard_status,omitempty"`
	ActionsStatus       string                  `json:"actions_status,omitempty"`
	AIProxy             openaix.ProxySummary    `json:"ai_proxy"`
	DashboardTopActions DashboardTopActionsDiag `json:"dashboard_top_actions"`
	Artifacts           []string                `json:"artifacts"`
	Warnings            []string                `json:"warnings"`
	Usage               AdminUsageDiagnostics   `json:"usage"`
}

type AdminUsageDiagnostics struct {
	Events               []AdminUsageEvent `json:"events"`
	TotalInputTokens     int               `json:"total_input_tokens"`
	TotalOutputTokens    int               `json:"total_output_tokens"`
	TotalTokens          int               `json:"total_tokens"`
	TotalEstimatedCost   *float64          `json:"total_estimated_cost,omitempty"`
	UnknownPriceWarnings []string          `json:"unknown_price_warnings,omitempty"`
}

type AdminUsageEvent struct {
	ID            int64    `json:"id"`
	Component     string   `json:"component"`
	Attempt       int      `json:"attempt"`
	Provider      string   `json:"provider,omitempty"`
	Model         string   `json:"model,omitempty"`
	RequestID     string   `json:"request_id,omitempty"`
	InputTokens   int      `json:"input_tokens"`
	OutputTokens  int      `json:"output_tokens"`
	TotalTokens   int      `json:"total_tokens"`
	EstimatedCost *float64 `json:"estimated_cost,omitempty"`
	Status        string   `json:"status"`
	CreatedAt     string   `json:"created_at"`
}

type DashboardTopActionsDiag struct {
	Source       string  `json:"source,omitempty"`
	ActionIDs    []int64 `json:"action_ids,omitempty"`
	ActionsRunID int64   `json:"actions_run_id,omitempty"`
}

// BuildAdminPipelineDiagnostics assembles admin diagnostics for a pipeline run.
func BuildAdminPipelineDiagnostics(
	run dbgen.AiPipelineRun,
	artifacts []dbgen.AiPipelineArtifact,
	topActions DashboardTopActionsDiag,
) AdminPipelineDiagnostics {
	out := AdminPipelineDiagnostics{
		PipelineRunID:       run.ID,
		SellerAccountID:     run.SellerAccountID,
		DataPackHash:        strings.TrimSpace(run.DataPackHash),
		GlobalContextHash:   pgText(run.GlobalContextHash),
		DashboardStatus:     pgText(run.DashboardStatus),
		ActionsStatus:       pgText(run.ActionsStatus),
		AIProxy:             openaix.ProxySummaryFromEnv(),
		DashboardTopActions: topActions,
		Warnings:            []string{},
	}
	if run.MetricDate.Valid {
		out.MetricDate = run.MetricDate.Time.UTC().Format("2006-01-02")
	}
	if run.DashboardRunID.Valid {
		if u, err := uuid.FromBytes(run.DashboardRunID.Bytes[:]); err == nil {
			out.DashboardRunID = u.String()
		}
	}
	if run.ActionsRunID.Valid {
		out.ActionsRunID = run.ActionsRunID.Int64
	}
	kinds := make([]string, 0, len(artifacts))
	seen := make(map[string]struct{}, len(artifacts))
	for _, art := range artifacts {
		kind := strings.TrimSpace(art.Kind)
		if kind == "" {
			continue
		}
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		kinds = append(kinds, kind)
	}
	out.Artifacts = kinds
	return out
}

// LoadAdminPipelineDiagnostics loads pipeline run, artifacts and optional top action ids.
func LoadAdminPipelineDiagnostics(
	ctx context.Context,
	store PipelineInspectStore,
	pipelineRunID int64,
	topActions DashboardTopActionsDiag,
) (AdminPipelineDiagnostics, error) {
	run, err := store.GetPipelineRun(ctx, pipelineRunID)
	if err != nil {
		return AdminPipelineDiagnostics{}, err
	}
	artifacts, err := store.ListPipelineArtifacts(ctx, pipelineRunID)
	if err != nil {
		return AdminPipelineDiagnostics{}, err
	}
	return BuildAdminPipelineDiagnostics(run, artifacts, topActions), nil
}

// AttachUsageDiagnostics loads ai_usage_events for admin diagnostics.
func AttachUsageDiagnostics(ctx context.Context, q *dbgen.Queries, diag *AdminPipelineDiagnostics) error {
	if q == nil || diag == nil || diag.PipelineRunID <= 0 {
		return nil
	}
	rows, err := q.ListAIUsageEventsByPipelineRun(ctx, pgInt8(diag.PipelineRunID))
	if err != nil {
		return err
	}
	usage := AdminUsageDiagnostics{Events: make([]AdminUsageEvent, 0, len(rows))}
	var totalCost float64
	var hasCost bool
	for _, row := range rows {
		ev := AdminUsageEvent{
			ID:           row.ID,
			Component:    row.Component,
			Attempt:      int(row.Attempt),
			InputTokens:  int(row.InputTokens),
			OutputTokens: int(row.OutputTokens),
			TotalTokens:  int(row.TotalTokens),
			Status:       row.Status,
			CreatedAt:    row.CreatedAt.Time.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if row.Model.Valid {
			ev.Model = row.Model.String
			if row.EstimatedCost.Valid {
				if c, ok := numericToFloat(row.EstimatedCost); ok {
					ev.EstimatedCost = &c
					totalCost += c
					hasCost = true
				}
			} else if row.TotalTokens > 0 {
				est := aicost.EstimateUSD(row.Model.String, int(row.InputTokens), int(row.OutputTokens))
				if !est.Known {
					usage.UnknownPriceWarnings = append(usage.UnknownPriceWarnings, "unknown model price: "+row.Model.String)
				}
			}
		}
		if row.RequestID.Valid {
			ev.RequestID = row.RequestID.String
		}
		usage.Events = append(usage.Events, ev)
		usage.TotalInputTokens += ev.InputTokens
		usage.TotalOutputTokens += ev.OutputTokens
		usage.TotalTokens += ev.TotalTokens
	}
	if hasCost {
		usage.TotalEstimatedCost = &totalCost
	}
	diag.Usage = usage
	return nil
}

func pgInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: true}
}

func numericToFloat(n pgtype.Numeric) (float64, bool) {
	if !n.Valid {
		return 0, false
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0, false
	}
	return f.Float64, true
}
