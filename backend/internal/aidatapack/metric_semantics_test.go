package aidatapack

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMetricSemanticDefinition_IDCDays(t *testing.T) {
	def, ok := MetricSemanticDefinitionFor("idc_days")
	if !ok {
		t.Fatal("idc_days not in registry")
	}
	if def.CanonicalLabelRU != "Покрытие запасом" {
		t.Fatalf("label=%q", def.CanonicalLabelRU)
	}
}

func TestDetectMetricSemanticMismatch_ForbiddenStorageTerm(t *testing.T) {
	issues := DetectMetricSemanticMismatches("длительный срок хранения — 142 дня", []string{"idc_days"}, "observation")
	if len(issues) == 0 {
		t.Fatal("expected metric_semantic_mismatch")
	}
	if issues[0].MetricCode != "idc_days" {
		t.Fatalf("metric=%q", issues[0].MetricCode)
	}
}

func TestDetectMetricSemanticMismatch_CorrectWording(t *testing.T) {
	issues := DetectMetricSemanticMismatches("покрытие запасом составляет 142 дня", []string{"idc_days"}, "observation")
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestAccountRollupOrdersFactsRun72Shape(t *testing.T) {
	b := NewBuilder(ComputeDataWindows(time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)))
	ctx := &loadContext{
		AccountMetrics: []dbgen.DailyAccountMetric{
			{MetricDate: pgtype.Date{Time: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), Valid: true}, Revenue: pgtype.Numeric{}, OrdersCount: 100},
			{MetricDate: pgtype.Date{Time: time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), Valid: true}, Revenue: pgtype.Numeric{}, OrdersCount: 96},
		},
	}
	// populate 14 days of metrics for windows
	for i := 0; i < 14; i++ {
		d := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)
		orders := int32(50)
		if d.After(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
			orders = 96
		}
		ctx.AccountMetrics = append(ctx.AccountMetrics, dbgen.DailyAccountMetric{
			MetricDate: pgtype.Date{Time: d, Valid: true}, OrdersCount: orders,
		})
	}
	table := buildAccountRollup(b, ctx)
	found := map[string]bool{}
	for _, id := range table.FactIDs {
		found[id] = true
	}
	for _, id := range RequiredAccountOrderFactIDs {
		if !found[id] {
			t.Fatalf("missing fact %s", id)
		}
	}
}
