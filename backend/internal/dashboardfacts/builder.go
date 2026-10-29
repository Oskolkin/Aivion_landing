package dashboardfacts

import (
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// BuildDashboardFacts assembles immutable dashboard facts from the same snapshot used for the pack.
func BuildDashboardFacts(snapshot *analysisdata.AnalysisDataSnapshot, identity PackIdentity) (DashboardFactsV1, FactsValidationReport, error) {
	if snapshot == nil {
		return DashboardFactsV1{}, FactsValidationReport{}, fmt.Errorf("snapshot is nil")
	}
	if !snapshot.MetricDate.Valid {
		return DashboardFactsV1{}, FactsValidationReport{}, fmt.Errorf("snapshot metric_date is invalid")
	}
	if identity.PackID == "" || identity.PackHash == "" {
		return DashboardFactsV1{}, FactsValidationReport{}, fmt.Errorf("pack identity is incomplete")
	}

	metricDate := analysisdata.DateOnlyUTC(snapshot.MetricDate.Time)
	metricDateKey := analysisdata.FormatDate(metricDate)

	metrics, err := analytics.BuildDashboardMetricsFromSnapshot(snapshot)
	if err != nil {
		return DashboardFactsV1{}, FactsValidationReport{}, err
	}

	catByProduct := buildProductCategoryMap(snapshot)
	skuIndex := buildSKUFactsIndex(snapshot, metrics, metricDate)

	facts := DashboardFactsV1{
		SchemaVersion:    SchemaVersion,
		SellerAccountID:  snapshot.SellerAccountID,
		MetricDate:       metricDateKey,
		PackID:           identity.PackID,
		PackHash:         identity.PackHash,
		DashboardMetrics: metrics,
		SellerPulse:      buildSellerPulseFacts(snapshot, metrics, metricDate),
		StockCoverage:    buildStockCoverageFacts(metrics),
		AdEfficiency:     buildAdEfficiencyFacts(snapshot),
		Funnel:           buildFunnelFacts(snapshot, metricDate),
		SKUFactsIndex:    skuIndex,
		KeySKUCohort:     buildKeySKUCohortFacts(skuIndex),
		Categories:       buildCategoryFacts(snapshot, metrics, metricDate, catByProduct),
		DataCoverage:     buildDataCoverage(snapshot, metricDate, catByProduct),
	}

	facts.ContentCounts = buildContentCounts(facts)

	hash, err := computeFactsHash(facts)
	if err != nil {
		return DashboardFactsV1{}, FactsValidationReport{}, fmt.Errorf("compute facts_hash: %w", err)
	}
	facts.FactsHash = hash

	report := validateDashboardFacts(facts)
	appendSourceReconciliationIssues(&report, snapshot)
	return facts, report, nil
}

func buildDataCoverage(snapshot *analysisdata.AnalysisDataSnapshot, metricDate time.Time, catByProduct map[int64]categoryRef) DataCoverage {
	from7 := metricDate.AddDate(0, 0, -6)
	fromAd := metricDate.AddDate(0, 0, -(analytics.AdEfficiencyDashboardTrendWindowDays - 1))
	currentFunnel := accountFunnelRowForDate(filterAccountProductAnalytics(snapshot.FunnelRows), metricDate.AddDate(0, 0, -1))
	return DataCoverage{
		AccountMetricsDays: len(observedAccountMetricDays(snapshot.AccountMetrics, from7, metricDate)),
		SKUMetricsDays:     countDistinctSKUMetricDates(snapshot.SKUMetrics, from7, metricDate),
		FunnelAvailable:    currentFunnel != nil,
		AdMetricsDays:      countDistinctAdMetricDates(snapshot.AdMetrics, fromAd, metricDate),
		StockSnapshotRows:  len(snapshot.StockSnapshots),
		CategoryCount:      countDistinctSellerCategories(catByProduct),
	}
}

func countDistinctSKUMetricDates(rows []dbgen.DailySkuMetric, from, to time.Time) int {
	fromDate := analysisdata.DateOnlyUTC(from)
	toDate := analysisdata.DateOnlyUTC(to)
	seen := make(map[string]struct{})
	for _, row := range rows {
		if !row.MetricDate.Valid {
			continue
		}
		d := analysisdata.DateOnlyUTC(row.MetricDate.Time)
		if d.Before(fromDate) || d.After(toDate) {
			continue
		}
		seen[d.Format("2006-01-02")] = struct{}{}
	}
	return len(seen)
}

func countDistinctAdMetricDates(rows []dbgen.AdMetricsDaily, from, to time.Time) int {
	fromDate := analysisdata.DateOnlyUTC(from)
	toDate := analysisdata.DateOnlyUTC(to)
	seen := make(map[string]struct{})
	for _, row := range rows {
		if !row.MetricDate.Valid {
			continue
		}
		d := analysisdata.DateOnlyUTC(row.MetricDate.Time)
		if d.Before(fromDate) || d.After(toDate) {
			continue
		}
		seen[d.Format("2006-01-02")] = struct{}{}
	}
	return len(seen)
}
