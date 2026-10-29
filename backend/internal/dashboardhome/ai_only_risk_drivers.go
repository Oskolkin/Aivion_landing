package dashboardhome

import (
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func riskDriversForAIOnlyApply(
	build dashboardNarrativeBuildResult,
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	recs []dbgen.Recommendation,
	alertRows []dbgen.Alert,
) []RiskDriver {
	v8, ok := build.AIContext.(dashboardNarrativeContextV8)
	if !ok || len(v8.RiskDrivers) == 0 {
		return nil
	}
	drivers := riskDriversFromNarrativeViews(v8.RiskDrivers)
	dailyOrders := dataPackDailyOrdersCount(v8)
	if dailyOrders > 0 {
		drivers = normalizeAIOnlyOrdersEvidence(drivers, dailyOrders, dto)
	}
	// Merge deterministic stock/funnel/ad widgets not represented in datapack drivers.
	legacy := buildRiskDrivers(time.Now(), dto, resp, recs, alertRows)
	drivers = mergeMissingAIOnlyDrivers(drivers, legacy)
	return drivers
}

func riskDriversFromNarrativeViews(views []dashboardNarrativeRiskDriverView) []RiskDriver {
	out := make([]RiskDriver, 0, len(views))
	for _, v := range views {
		categoryLabel := strings.TrimSpace(v.CategoryLabel)
		if categoryLabel == "" {
			categoryLabel = riskDriverCategoryLabel(v.Category)
		}
		out = append(out, RiskDriver{
			ID:            v.ID,
			Category:      v.Category,
			CategoryLabel: categoryLabel,
			Severity:      v.Severity,
			ImpactScore:   v.ImpactScore,
			Title:         v.Title,
			Reason:        v.Reason,
			Evidence:      v.Evidence,
		})
	}
	return out
}

func dataPackDailyOrdersCount(v8 dashboardNarrativeContextV8) int64 {
	if account, ok := v8.Account["current_day"].(map[string]any); ok {
		if n, ok := intFromAny(account["orders_count"]); ok {
			return n
		}
	}
	return 0
}

func normalizeAIOnlyOrdersEvidence(drivers []RiskDriver, dailyOrders int64, dto analytics.DashboardMetricsDTO) []RiskDriver {
	out := make([]RiskDriver, 0, len(drivers))
	for _, d := range drivers {
		switch d.ID {
		case "driver-sales-revenue", driverAccountSales, "driver-cancellations":
			if d.Evidence == nil {
				d.Evidence = map[string]any{}
			}
			d.Evidence["orders_today"] = dailyOrders
			d.Evidence["orders_semantic"] = "daily_account_orders_count"
			if d.ID == "driver-cancellations" {
				cancels := dto.Account.CancelsToday
				d.Evidence["cancels_today"] = cancels
				if dailyOrders > 0 {
					rate := float64(cancels) / float64(dailyOrders) * 100
					d.Evidence["cancellation_rate"] = rate
					d.Reason = fmt.Sprintf(
						"Отмены заняли %.1f%% от заказов аккаунта за день: %d отмен при %d заказах аккаунта.",
						rate, cancels, dailyOrders,
					)
				}
			}
		}
		out = append(out, d)
	}
	return out
}

func mergeMissingAIOnlyDrivers(primary, supplemental []RiskDriver) []RiskDriver {
	byID := map[string]RiskDriver{}
	order := make([]string, 0, len(primary)+len(supplemental))
	for _, d := range primary {
		byID[d.ID] = d
		order = append(order, d.ID)
	}
	for _, d := range supplemental {
		switch d.ID {
		case "driver-stock", "driver-funnel", "driver-advertising", driverSlowTurnover:
			if _, exists := byID[d.ID]; !exists {
				byID[d.ID] = d
				order = append(order, d.ID)
			}
		}
	}
	out := make([]RiskDriver, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

func buildAIRiskDriversFromHealthReasons(
	payload *dashboardNarrativePayload,
	build dashboardNarrativeBuildResult,
	fallback []RiskDriver,
) []RiskDriver {
	if payload == nil || len(payload.HealthReasons) == 0 {
		return fallback
	}
	byID := map[string]RiskDriver{}
	for _, d := range fallback {
		byID[d.ID] = d
	}
	out := make([]RiskDriver, 0, len(payload.HealthReasons))
	for _, reason := range payload.HealthReasons {
		base, ok := byID[reason.DriverID]
		if !ok {
			base = RiskDriver{
				ID: reason.DriverID, Category: reason.Category,
				CategoryLabel: riskDriverCategoryLabel(reason.Category),
				Severity:      "medium", ImpactScore: 50,
			}
		}
		if reason.Title != "" {
			base.Title = reason.Title
		}
		if reason.Reason != "" {
			base.Reason = reason.Reason
		}
		if reason.Category != "" {
			base.Category = reason.Category
			base.CategoryLabel = riskDriverCategoryLabel(reason.Category)
		}
		out = append(out, base)
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
