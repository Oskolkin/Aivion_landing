package aisummaries

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// SummaryRowAudit is a per-row acceptance report for dev/ops tooling.
type SummaryRowAudit struct {
	RunID            string   `json:"run_id"`
	SummaryType      string   `json:"summary_type"`
	ScopeID          string   `json:"scope_id,omitempty"`
	CategoryName     string   `json:"category_name,omitempty"`
	DBStatus         string   `json:"db_status"`
	AcceptanceStatus string   `json:"acceptance_status"`
	HardErrors       []string `json:"hard_errors,omitempty"`
	RepairableErrors []string `json:"repairable_errors,omitempty"`
	BlockFallbacks   []string `json:"block_fallbacks,omitempty"`
	OutputTokens     int      `json:"output_tokens,omitempty"`
	Truncated        bool     `json:"truncated"`
	ErrorMessage     string   `json:"error_message,omitempty"`
}

// LoadSummaryRowAudits lists all summary rows for a metric_date with acceptance diagnostics.
func LoadSummaryRowAudits(ctx context.Context, q *dbgen.Queries, sellerID int64, metricDate pgtype.Date) ([]SummaryRowAudit, error) {
	rows, err := q.ListAIDailySummariesForSellerMetricDate(ctx, dbgen.ListAIDailySummariesForSellerMetricDateParams{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
	})
	if err != nil {
		return nil, err
	}
	out := make([]SummaryRowAudit, 0, len(rows))
	for _, row := range rows {
		audit := SummaryRowAudit{
			SummaryType: row.SummaryType,
			ScopeID:     row.ScopeID,
			DBStatus:    row.Status,
		}
		if row.RunID.Valid {
			audit.RunID = uuid.UUID(row.RunID.Bytes).String()
		}
		if row.ScopeName.Valid {
			audit.CategoryName = row.ScopeName.String
		}
		if row.EstimatedOutputTokens.Valid {
			audit.OutputTokens = int(row.EstimatedOutputTokens.Int32)
		}
		if row.ErrorMessage.Valid {
			audit.ErrorMessage = row.ErrorMessage.String
			audit.Truncated = strings.Contains(audit.ErrorMessage, "openai_output_incomplete_length")
		}

		var payload *SummaryPayload
		if len(row.SummaryJson) > 0 {
			var p SummaryPayload
			if err := json.Unmarshal(row.SummaryJson, &p); err == nil {
				payload = &p
			}
		}

		storedAcceptance, storedNotes := decodeStoredValidationNotes(row.ValidationErrors)
		audit.RepairableErrors = append([]string(nil), storedNotes...)

		if row.Status == "completed" && payload != nil {
			ctxRepair := SummaryRepairContext{
				SummaryType:  row.SummaryType,
				ScopeID:      row.ScopeID,
				CategoryName: audit.CategoryName,
			}
			outcome := ReacceptSummaryPayload(payload, ctxRepair)
			audit.HardErrors = outcome.Repair.HardErrors
			if len(outcome.Repair.RepairableErrors) > 0 {
				audit.RepairableErrors = outcome.Repair.RepairableErrors
			}
			audit.BlockFallbacks = outcome.Repair.FallbackBlocks
			audit.AcceptanceStatus = outcome.ValidationStatus
			if !outcome.Accepted {
				audit.AcceptanceStatus = "failed"
			}
		} else if storedAcceptance != "" {
			audit.AcceptanceStatus = storedAcceptance
		} else {
			audit.AcceptanceStatus = row.Status
		}

		out = append(out, audit)
	}
	return out, nil
}
