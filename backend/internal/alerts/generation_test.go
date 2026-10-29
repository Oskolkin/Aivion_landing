package alerts

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestRunPriceEconomicsAlertsDisabled(t *testing.T) {
	if !PriceEconomicsGenerationDisabled {
		t.Fatal("expected PriceEconomicsGenerationDisabled=true for MVP")
	}
	asOf := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepo{asOf: asOf}
	svc := NewService(repo)
	summary, err := svc.RunPriceEconomicsAlerts(context.Background(), 1, asOf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.GeneratedAlerts != 0 || summary.UpsertedAlerts != 0 {
		t.Fatalf("expected no price alerts, got %+v", summary)
	}
}

func TestRunForAccountDoesNotGeneratePriceAlerts(t *testing.T) {
	asOf := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		asOf: asOf,
		currentMetric: &AccountDailyMetric{
			SellerAccountID: 1,
			MetricDate:      asOf,
			Revenue:         60,
			OrdersCount:     60,
		},
		previousMetric: &AccountDailyMetric{
			SellerAccountID: 1,
			MetricDate:      asOf.AddDate(0, 0, -1),
			Revenue:         100,
			OrdersCount:     100,
		},
	}
	svc := NewService(repo)
	summary, err := svc.RunForAccount(context.Background(), 1, asOf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.PriceEconomics.UpsertedAlerts != 0 || summary.PriceEconomics.GeneratedAlerts != 0 {
		t.Fatalf("expected empty price economics summary, got %+v", summary.PriceEconomics)
	}
}

func TestActiveAndLegacyAlertTypesDisjoint(t *testing.T) {
	for tpe := range activeAlertTypes {
		if IsLegacyAlertType(tpe) {
			t.Fatalf("%s is both active and legacy", tpe)
		}
		if !IsActiveAlertType(tpe) {
			t.Fatalf("IsActiveAlertType(%s) = false", tpe)
		}
	}
	for tpe := range legacyAlertTypes {
		if IsActiveAlertType(tpe) {
			t.Fatalf("%s is both legacy and active", tpe)
		}
		if !IsLegacyAlertType(tpe) {
			t.Fatalf("IsLegacyAlertType(%s) = false", tpe)
		}
	}
}

func TestAlertRiskBucketMutuallyExclusive(t *testing.T) {
	cases := []struct {
		alertType string
		group     string
		want      string
	}{
		{"fbo_surplus_storage_risk", "stock", "surplus"},
		{"stock_critical_sku_low_stock", "stock", "critical_sku"},
		{"stock_low_coverage", "stock", "stock"},
		{"ad_weak_campaign_efficiency", "advertising", "advertising"},
		{"sku_orders_drop", "sales", "sales"},
		{"price_below_min_constraint", "price_economics", "other"},
	}
	for _, tc := range cases {
		got := AlertRiskBucket(AlertType(tc.alertType), AlertGroup(tc.group))
		if got != tc.want {
			t.Fatalf("%s/%s => %q want %q", tc.alertType, tc.group, got, tc.want)
		}
	}
}

// Ensure fakeRepo still satisfies Repository after generation tests.
var _ Repository = (*fakeRepo)(nil)
var _ = dbgen.Product{}
