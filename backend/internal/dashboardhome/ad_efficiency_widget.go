package dashboardhome

import (
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

const (
	adEfficiencyCTALabel = "Открыть рекламные кампании"
	adEfficiencyCTAHref  = "/app/advertising"
)

func mapAdEfficiencyDashboardDTO(dto analytics.AdEfficiencyDashboardDTO) AdEfficiencyWidget {
	widget := AdEfficiencyWidget{
		WidgetBase: WidgetBase{Status: mapAdEfficiencyWidgetStatus(dto.WidgetStatus)},
		CTA: &AdEfficiencyCTA{
			Label: adEfficiencyCTALabel,
			Href:  adEfficiencyCTAHref,
		},
	}
	if !dto.HasData {
		widget.WidgetBase = WidgetBase{
			Status:  WidgetNotEnoughData,
			Message: "Рекламные кампании пока недоступны или не синхронизированы.",
		}
		return widget
	}
	widget.Period = adEfficiencyPeriodPtr(dto.Period)
	widget.Summary = adEfficiencySummaryPtr(dto.Summary)
	widget.Trend7D = mapAdEfficiencyTrendPoints(dto.Trend7D)
	widget.HighDRRCampaigns = mapAdCampaignDisplayRows(dto.HighDRRCampaigns)
	widget.NoOrderCampaigns = mapAdCampaignDisplayRows(dto.NoOrderCampaigns)
	widget.Points = mapAdEfficiencyLegacyPoints(dto.LegacyPoints)
	if dto.WidgetStatus == "partial" {
		widget.Message = "Часть рекламных метрик за выбранный период недоступна."
	}
	return widget
}

func adEfficiencyPeriodPtr(period analytics.AdEfficiencyPeriodDTO) *AdEfficiencyPeriod {
	if !isPopulatedAdEfficiencyPeriod(period) {
		return nil
	}
	return &AdEfficiencyPeriod{
		MetricDate:            period.MetricDate,
		DateFrom:              period.DateFrom,
		DateTo:                period.DateTo,
		CurrentCompletedDate:  period.CurrentCompletedDate,
		PreviousCompletedDate: period.PreviousCompletedDate,
		Current7dDateFrom:     period.Current7dDateFrom,
		Current7dDateTo:       period.Current7dDateTo,
		Previous7dDateFrom:    period.Previous7dDateFrom,
		Previous7dDateTo:      period.Previous7dDateTo,
		SampleDays:            period.SampleDays,
		PreviousSampleDays:    period.PreviousSampleDays,
	}
}

func adEfficiencySummaryPtr(summary analytics.AdEfficiencySummaryDTO) *AdEfficiencySummary {
	return &AdEfficiencySummary{
		CurrentDRRPct:              summary.CurrentDRRPct,
		PreviousDRRPct:             summary.PreviousDRRPct,
		AverageDRR7dPct:            summary.AverageDRR7dPct,
		DeltaVsPreviousPp:          summary.DeltaVsPreviousPp,
		DeltaVsAverage7dPp:         summary.DeltaVsAverage7dPp,
		PreviousDRR7dPct:           summary.PreviousDRR7dPct,
		DeltaVsPrevious7dPp:        summary.DeltaVsPrevious7dPp,
		TotalSpendToday:            summary.TotalSpendToday,
		TotalRevenueToday:          summary.TotalRevenueToday,
		TotalStoreRevenueToday:     summary.TotalStoreRevenueToday,
		TotalOrdersToday:           summary.TotalOrdersToday,
		TotalSpend7d:               summary.TotalSpend7d,
		TotalAttributedRevenue7d:   summary.TotalAttributedRevenue7d,
		TotalStoreRevenue7d:        summary.TotalStoreRevenue7d,
		CampaignsHighDRRCount:      summary.CampaignsHighDRRCount,
		CampaignsNoOrdersCount:     summary.CampaignsNoOrdersCount,
		HighDRRSpend:               summary.HighDRRSpend,
		SpendWithoutOrders:         summary.SpendWithoutOrders,
		SpendWithoutOrdersDateFrom: summary.SpendWithoutOrdersDateFrom,
		SpendWithoutOrdersDateTo:   summary.SpendWithoutOrdersDateTo,
		HighDRRThresholdPct:        summary.HighDRRThresholdPct,
	}
}

func isPopulatedAdEfficiencyPeriod(period analytics.AdEfficiencyPeriodDTO) bool {
	return strings.TrimSpace(period.MetricDate) != "" &&
		strings.TrimSpace(period.DateFrom) != "" &&
		strings.TrimSpace(period.DateTo) != ""
}

func mapAdEfficiencyWidgetStatus(status string) WidgetStatus {
	switch status {
	case "available":
		return WidgetAvailable
	case "partial":
		return WidgetPartial
	case "source_unavailable":
		return WidgetSourceNotAvailable
	case "error":
		return WidgetError
	default:
		return WidgetNotEnoughData
	}
}

func mapAdEfficiencyTrendPoints(rows []analytics.AdEfficiencyTrendPointDTO) []AdEfficiencyTrendPoint {
	out := make([]AdEfficiencyTrendPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdEfficiencyTrendPoint{
			Date:        row.Date,
			Label:       row.Label,
			Spend:       row.Spend,
			Revenue:     row.Revenue,
			Orders:      row.Orders,
			DRRPct:      row.DRRPct,
			IsToday:     row.IsToday,
			IsYesterday: row.IsYesterday,
			DataStatus:  row.DataStatus,
		})
	}
	return out
}

func mapAdCampaignDisplayRows(rows []analytics.AdCampaignDisplayRowDTO) []AdCampaignDisplayRow {
	out := make([]AdCampaignDisplayRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdCampaignDisplayRow{
			CampaignExternalID: row.CampaignExternalID,
			CampaignName:       row.CampaignName,
			DRRPct:             row.DRRPct,
			Spend:              row.Spend,
			Orders:             row.Orders,
			Revenue:            row.Revenue,
			ROAS:               row.ROAS,
			CPO:                row.CPO,
			CTR:                row.CTR,
			CPC:                row.CPC,
			ClickToOrderRate:   row.ClickToOrderRate,
			ReasonCode:         row.ReasonCode,
		})
	}
	return out
}

func mapAdEfficiencyLegacyPoints(rows []analytics.AdEfficiencyLegacyPointDTO) []AdEfficiencyPoint {
	out := make([]AdEfficiencyPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdEfficiencyPoint{
			CampaignExternalID: row.CampaignExternalID,
			CampaignName:       row.CampaignName,
			Spend:              row.Spend,
			DRR:                row.DRR,
			Revenue:            row.Revenue,
			Orders:             row.Orders,
			EfficiencySignal:   row.EfficiencySignal,
		})
	}
	return out
}

func emptyAdEfficiencyWidget(message string) AdEfficiencyWidget {
	if message == "" {
		message = "Рекламные кампании пока недоступны или не синхронизированы."
	}
	return AdEfficiencyWidget{
		WidgetBase: WidgetBase{
			Status:  WidgetNotEnoughData,
			Message: message,
		},
		CTA: &AdEfficiencyCTA{Label: adEfficiencyCTALabel, Href: adEfficiencyCTAHref},
	}
}

func errorAdEfficiencyWidget(message string) AdEfficiencyWidget {
	if message == "" {
		message = "Не удалось загрузить рекламную эффективность."
	}
	return AdEfficiencyWidget{
		WidgetBase: WidgetBase{
			Status:  WidgetError,
			Message: message,
		},
		CTA: &AdEfficiencyCTA{Label: adEfficiencyCTALabel, Href: adEfficiencyCTAHref},
	}
}
