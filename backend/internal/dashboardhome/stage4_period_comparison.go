package dashboardhome

import (
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// periodComparisonFact is a neutral factual delta without backend semantic grouping.
type periodComparisonFact struct {
	ID            string `json:"id"`
	Category      string `json:"category"`
	Label         string `json:"label"`
	CurrentValue  string `json:"current_value,omitempty"`
	PreviousValue string `json:"previous_value,omitempty"`
	DeltaAbs      string `json:"delta_abs,omitempty"`
	DeltaPct      string `json:"delta_pct,omitempty"`
	Note          string `json:"note,omitempty"`
}

type dashboardNarrativePeriodComparison struct {
	CurrentMetricDate  string                 `json:"current_metric_date"`
	PreviousMetricDate string                 `json:"previous_metric_date,omitempty"`
	Facts              []periodComparisonFact `json:"facts"`
	AllowedNumbers     []string               `json:"allowed_numbers,omitempty"`
}

func buildDataPackPeriodComparison(pack *aidatapack.SellerAIDataPack) dashboardNarrativePeriodComparison {
	out := dashboardNarrativePeriodComparison{
		CurrentMetricDate: pack.SellerContext.MetricDate,
		Facts:             []periodComparisonFact{},
	}
	ar := pack.Tables.AccountRollup
	if ar.DataStatus != "available" {
		return out
	}
	if cur, ok := ar.CurrentDay["revenue"]; ok {
		fact := periodComparisonFact{
			ID: "account.revenue", Category: "sales", Label: "Выручка",
			CurrentValue: fmt.Sprint(cur),
		}
		if prev, ok := ar.Yesterday["revenue"]; ok {
			fact.PreviousValue = fmt.Sprint(prev)
		}
		if deltas, ok := ar.Deltas["revenue_last_7d_vs_previous_7d_pct"]; ok {
			fact.DeltaPct = fmt.Sprint(deltas)
		}
		out.Facts = append(out.Facts, fact)
		registerPeriodComparisonNumbers(&out, fact)
	}
	if cur, ok := ar.CurrentDay["orders_count"]; ok {
		fact := periodComparisonFact{
			ID: "account.orders", Category: "sales", Label: "Заказы",
			CurrentValue: fmt.Sprint(cur),
		}
		if prev, ok := ar.Yesterday["orders_count"]; ok {
			fact.PreviousValue = fmt.Sprint(prev)
		}
		if deltas, ok := ar.Deltas["orders_last_7d_vs_previous_7d_pct"]; ok {
			fact.DeltaPct = fmt.Sprint(deltas)
		}
		out.Facts = append(out.Facts, fact)
		registerPeriodComparisonNumbers(&out, fact)
	}
	if cur, ok := ar.CurrentDay["cancels_count"]; ok {
		fact := periodComparisonFact{
			ID: "account.cancels", Category: "sales", Label: "Отмены",
			CurrentValue: fmt.Sprint(cur),
		}
		if prev, ok := ar.Yesterday["cancels_count"]; ok {
			fact.PreviousValue = fmt.Sprint(prev)
		}
		out.Facts = append(out.Facts, fact)
		registerPeriodComparisonNumbers(&out, fact)
	}
	return out
}

func registerPeriodComparisonNumbers(ctx *dashboardNarrativePeriodComparison, fact periodComparisonFact) {
	for _, v := range []string{fact.CurrentValue, fact.PreviousValue, fact.DeltaAbs, fact.DeltaPct} {
		v = strings.TrimSpace(v)
		if v != "" {
			ctx.AllowedNumbers = append(ctx.AllowedNumbers, v)
		}
	}
}

func buildWhatChangedValidationBundleFromPeriodComparison(
	comparison dashboardNarrativePeriodComparison,
	allowedNames map[string]struct{},
) whatChangedValidationBundle {
	nums := map[string]struct{}{}
	for _, n := range comparison.AllowedNumbers {
		for _, token := range expandAllowedNumberTokens(n) {
			if token != "" {
				nums[token] = struct{}{}
			}
		}
	}
	return whatChangedValidationBundle{
		Context: dashboardNarrativeWhatChangedContext{
			CurrentMetricDate:  comparison.CurrentMetricDate,
			PreviousMetricDate: comparison.PreviousMetricDate,
			AllowedNumbers:     comparison.AllowedNumbers,
			PeriodComparison:   comparison,
		},
		AllowedNumbers: nums,
		AllowedNames:   allowedNames,
	}
}

func buildDashboardNarrativeContextMinimalForAIOnly(
	metricDate time.Time,
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	recs []dbgen.Recommendation,
	alertRows []dbgen.Alert,
	previousSnapshot *accountMetricsSnapshot,
) dashboardNarrativeBuildResult {
	_ = dto
	_ = recs
	_ = alertRows
	metricDateStr := metricDate.UTC().Format("2006-01-02")
	periodComparison := dashboardNarrativePeriodComparison{
		CurrentMetricDate: metricDateStr,
		Facts:             []periodComparisonFact{},
	}
	if previousSnapshot != nil {
		periodComparison.PreviousMetricDate = previousSnapshot.MetricDate
	}
	// Unified publication home apply does not rebuild the Seller AI DataPack.
	// Period-over-period facts must still be present so AI what_changed with
	// improved/worsened items can pass validation (empty Facts incorrectly
	// forces the improved group to the empty "не выявлено" template).
	if resp != nil {
		fromPulse := periodComparisonFromSellerPulse(metricDateStr, resp.SellerPulse)
		if len(fromPulse.Facts) > 0 {
			if periodComparison.PreviousMetricDate == "" {
				periodComparison.PreviousMetricDate = fromPulse.PreviousMetricDate
			}
			periodComparison.Facts = append(periodComparison.Facts, fromPulse.Facts...)
			periodComparison.AllowedNumbers = append(periodComparison.AllowedNumbers, fromPulse.AllowedNumbers...)
		}
	}
	bundle := buildWhatChangedValidationBundleFromPeriodComparison(periodComparison, map[string]struct{}{})
	return dashboardNarrativeBuildResult{
		Context: dashboardNarrativeContext{
			MetricDate:         periodComparison.CurrentMetricDate,
			WhatChangedContext: bundle.Context,
		},
		WhatChanged: bundle,
	}
}

// periodComparisonFromSellerPulse builds neutral period-comparison facts from
// already-projected Seller Pulse KPI deltas (frozen unified dashboard path).
func periodComparisonFromSellerPulse(metricDate string, pulse SellerPulse) dashboardNarrativePeriodComparison {
	out := dashboardNarrativePeriodComparison{
		CurrentMetricDate: strings.TrimSpace(metricDate),
		Facts:             []periodComparisonFact{},
	}
	for _, kpi := range pulse.KPIs {
		key := strings.TrimSpace(kpi.Key)
		value := strings.TrimSpace(kpi.Value)
		if key == "" || value == "" {
			continue
		}
		if kpi.Delta == nil {
			continue
		}
		fact := periodComparisonFact{
			ID:           "seller_pulse." + key,
			Category:     "sales",
			Label:        firstNonEmpty(strings.TrimSpace(kpi.Label), key),
			CurrentValue: value,
			DeltaAbs:     fmt.Sprint(kpi.Delta.Abs),
		}
		if kpi.Delta.Pct != nil {
			fact.DeltaPct = fmt.Sprint(*kpi.Delta.Pct)
		}
		out.Facts = append(out.Facts, fact)
		registerPeriodComparisonNumbers(&out, fact)
	}
	return out
}
