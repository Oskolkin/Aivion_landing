package dashboardfacts

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func buildAdEfficiencyFacts(snap *analysisdata.AnalysisDataSnapshot) AdEfficiencyFacts {
	dto, err := analytics.BuildAdEfficiencyDashboardFromSnapshot(snap)
	if err != nil {
		return AdEfficiencyFacts{Status: "not_enough_data"}
	}
	if !dto.HasData {
		return AdEfficiencyFacts{Status: "not_enough_data"}
	}
	status := "available"
	switch dto.WidgetStatus {
	case "partial":
		status = "partial"
	case "not_enough_data":
		status = "not_enough_data"
	case "source_unavailable", "error":
		status = "source_not_available"
	}

	facts := AdEfficiencyFacts{
		Status:           status,
		HighDRRCampaigns: mapAdCampaignRows(dto.HighDRRCampaigns),
		NoOrderCampaigns: mapAdCampaignRows(dto.NoOrderCampaigns),
		Points:           mapLegacyPoints(dto.LegacyPoints),
		Trend7D:          mapTrendPoints(dto.Trend7D),
	}
	if dto.Period.MetricDate != "" {
		facts.Period = &AdEfficiencyPeriod{
			MetricDate:            dto.Period.MetricDate,
			DateFrom:              dto.Period.DateFrom,
			DateTo:                dto.Period.DateTo,
			CurrentCompletedDate:  dto.Period.CurrentCompletedDate,
			PreviousCompletedDate: dto.Period.PreviousCompletedDate,
			Current7dDateFrom:     dto.Period.Current7dDateFrom,
			Current7dDateTo:       dto.Period.Current7dDateTo,
			Previous7dDateFrom:    dto.Period.Previous7dDateFrom,
			Previous7dDateTo:      dto.Period.Previous7dDateTo,
			SampleDays:            dto.Period.SampleDays,
			PreviousSampleDays:    dto.Period.PreviousSampleDays,
		}
	}
	facts.Summary = &AdEfficiencySummary{
		CurrentDRRPct:              dto.Summary.CurrentDRRPct,
		PreviousDRRPct:             dto.Summary.PreviousDRRPct,
		AverageDRR7dPct:            dto.Summary.AverageDRR7dPct,
		DeltaVsPreviousPp:          dto.Summary.DeltaVsPreviousPp,
		DeltaVsAverage7dPp:         dto.Summary.DeltaVsAverage7dPp,
		PreviousDRR7dPct:           dto.Summary.PreviousDRR7dPct,
		DeltaVsPrevious7dPp:        dto.Summary.DeltaVsPrevious7dPp,
		TotalSpendToday:            dto.Summary.TotalSpendToday,
		TotalRevenueToday:          dto.Summary.TotalRevenueToday,
		TotalStoreRevenueToday:     dto.Summary.TotalStoreRevenueToday,
		TotalOrdersToday:           dto.Summary.TotalOrdersToday,
		TotalSpend7d:               dto.Summary.TotalSpend7d,
		TotalAttributedRevenue7d:   dto.Summary.TotalAttributedRevenue7d,
		TotalStoreRevenue7d:        dto.Summary.TotalStoreRevenue7d,
		CampaignsHighDRRCount:      dto.Summary.CampaignsHighDRRCount,
		CampaignsNoOrdersCount:     dto.Summary.CampaignsNoOrdersCount,
		HighDRRSpend:               dto.Summary.HighDRRSpend,
		SpendWithoutOrders:         dto.Summary.SpendWithoutOrders,
		SpendWithoutOrdersDateFrom: dto.Summary.SpendWithoutOrdersDateFrom,
		SpendWithoutOrdersDateTo:   dto.Summary.SpendWithoutOrdersDateTo,
		HighDRRThresholdPct:        dto.Summary.HighDRRThresholdPct,
	}
	return facts
}

func mapAdCampaignRows(rows []analytics.AdCampaignDisplayRowDTO) []AdCampaignDisplayRow {
	out := make([]AdCampaignDisplayRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdCampaignDisplayRow{
			CampaignExternalID: row.CampaignExternalID,
			CampaignName:       row.CampaignName,
			DRRPct:             row.DRRPct,
			Spend:              row.Spend,
			Orders:             row.Orders,
			Revenue:            row.Revenue,
			ReasonCode:         row.ReasonCode,
		})
	}
	return out
}

func mapLegacyPoints(rows []analytics.AdEfficiencyLegacyPointDTO) []AdEfficiencyPoint {
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

func mapTrendPoints(rows []analytics.AdEfficiencyTrendPointDTO) []AdEfficiencyTrendPoint {
	out := make([]AdEfficiencyTrendPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdEfficiencyTrendPoint{
			Date:        row.Date,
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

func countAdPointsWithData(points []AdEfficiencyPoint) int {
	n := 0
	for _, p := range points {
		if p.Spend > 0 || p.Revenue > 0 || p.Orders > 0 {
			n++
		}
	}
	return n
}
