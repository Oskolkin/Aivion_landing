package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AISummariesHandler struct {
	pool *pgxpool.Pool
}

func NewAISummariesHandler(pool *pgxpool.Pool) *AISummariesHandler {
	return &AISummariesHandler{pool: pool}
}

type aiSummaryCard struct {
	ID               string          `json:"id,omitempty"`
	SummaryType      string          `json:"summary_type"`
	ScopeType        string          `json:"scope_type"`
	ScopeID          string          `json:"scope_id,omitempty"`
	ScopeName        *string         `json:"scope_name,omitempty"`
	Status           string          `json:"status"`
	Headline         *string         `json:"headline,omitempty"`
	HealthScore      *int32          `json:"health_score,omitempty"`
	ExecutiveSummary *string         `json:"executive_summary,omitempty"`
	SummaryJSON      json.RawMessage `json:"summary_json,omitempty"`
	UpdatedAt        *string         `json:"updated_at,omitempty"`
}

type aiSummariesLatestResponse struct {
	Status              string          `json:"status"`
	Message             string          `json:"message,omitempty"`
	DashboardMetricDate *string         `json:"dashboard_metric_date,omitempty"`
	MetricDate          *string         `json:"metric_date,omitempty"`
	RunStatus           *string         `json:"run_status,omitempty"`
	FailedCategoryCount *int32          `json:"failed_category_count,omitempty"`
	GeneratedAt         *string         `json:"generated_at,omitempty"`
	AccountSummary      *aiSummaryCard  `json:"account_summary,omitempty"`
	CategorySummaries   []aiSummaryCard `json:"category_summaries"`
}

// GetLatest returns the latest stored AI daily summaries for the current seller (async-generated).
func (h *AISummariesHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	seller, ok := auth.SellerAccountFromContext(ctx)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	q := dbgen.New(h.pool)

	dashLatest, err := q.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, seller.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to resolve metric date")
		return
	}
	if !dashLatest.Valid || dashLatest.Time.Year() < 2000 {
		writeJSON(w, http.StatusOK, aiSummariesLatestResponse{
			Status:  "not_enough_data",
			Message: "Данных пока недостаточно для AI-сводки. Она появится после первой загрузки продаж, товаров и остатков.",
		})
		return
	}

	summaryMetricDate, err := q.GetLatestAIDailySummaryMetricDate(ctx, seller.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load summaries")
		return
	}
	dmd := dashLatest.Time.UTC().Format("2006-01-02")
	resp := aiSummariesLatestResponse{
		CategorySummaries:   []aiSummaryCard{},
		DashboardMetricDate: &dmd,
		Status:              "pending",
		Message:             "AI-сводка ещё не готова. Она создаётся автоматически после синхронизации данных.",
	}

	if !summaryMetricDate.Valid {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	md := summaryMetricDate.Time.UTC().Format("2006-01-02")
	resp.MetricDate = &md

	var run *dbgen.AiDailySummaryRun
	if r, err := q.GetAIDailySummaryRunBySellerAndMetricDate(ctx, dbgen.GetAIDailySummaryRunBySellerAndMetricDateParams{
		SellerAccountID: seller.ID,
		MetricDate:      summaryMetricDate,
	}); err == nil {
		run = &r
		rs := run.Status
		resp.RunStatus = &rs
		if run.CategoryFailedCount > 0 {
			f := run.CategoryFailedCount
			resp.FailedCategoryCount = &f
		}
		if run.CompletedAt.Valid {
			g := run.CompletedAt.Time.UTC().Format(time.RFC3339)
			resp.GeneratedAt = &g
		}
	}

	rows, err := q.ListAIDailySummariesForSellerMetricDate(ctx, dbgen.ListAIDailySummariesForSellerMetricDateParams{
		SellerAccountID: seller.ID,
		MetricDate:      summaryMetricDate,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list summaries")
		return
	}

	for _, row := range rows {
		card := aiSummaryCard{
			SummaryType: row.SummaryType,
			ScopeType:   row.ScopeType,
			ScopeID:     row.ScopeID,
			Status:      row.Status,
		}
		if row.ScopeName.Valid {
			s := row.ScopeName.String
			card.ScopeName = &s
		}
		if row.ID.Valid {
			card.ID = uuid.UUID(row.ID.Bytes).String()
		}
		if row.Headline.Valid {
			s := row.Headline.String
			card.Headline = &s
		}
		if row.HealthScore.Valid {
			v := row.HealthScore.Int32
			card.HealthScore = &v
		}
		if row.ExecutiveSummary.Valid {
			s := row.ExecutiveSummary.String
			card.ExecutiveSummary = &s
		}
		if len(row.SummaryJson) > 0 {
			card.SummaryJSON = row.SummaryJson
		}
		if row.UpdatedAt.Valid {
			s := row.UpdatedAt.Time.UTC().Format(time.RFC3339)
			card.UpdatedAt = &s
		}
		if row.SummaryType == "account_daily_summary" {
			resp.AccountSummary = &card
		} else {
			resp.CategorySummaries = append(resp.CategorySummaries, card)
		}
	}

	resp.Message = ""
	if run != nil {
		switch run.Status {
		case "completed":
			resp.Status = "completed"
		case "partial":
			resp.Status = "partial"
		case "failed":
			resp.Status = "failed"
		default:
			resp.Status = "pending"
		}
	} else if len(rows) > 0 {
		resp.Status = "completed"
	}

	writeJSON(w, http.StatusOK, resp)
}
