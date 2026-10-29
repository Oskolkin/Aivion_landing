package aipipeline

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestIsStalePipelineRelativeToCurrent(t *testing.T) {
	base := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	current := dbgen.AiPipelineRun{
		ID:         10,
		MetricDate: pgtype.Date{Time: base, Valid: true},
		CreatedAt:  pgtype.Timestamptz{Time: base, Valid: true},
	}
	olderSameDay := dbgen.AiPipelineRun{
		ID:         9,
		MetricDate: pgtype.Date{Time: base, Valid: true},
		CreatedAt:  pgtype.Timestamptz{Time: base.Add(-time.Minute), Valid: true},
	}
	if !isStalePipelineRelativeToCurrent(current, olderSameDay) {
		t.Fatal("older same-day pipeline must be stale")
	}
	newerDate := dbgen.AiPipelineRun{
		ID:         11,
		MetricDate: pgtype.Date{Time: base.AddDate(0, 0, 1), Valid: true},
		CreatedAt:  pgtype.Timestamptz{Time: base, Valid: true},
	}
	if isStalePipelineRelativeToCurrent(current, newerDate) {
		t.Fatal("newer metric_date must not be stale")
	}
}
