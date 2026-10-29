package pack

import (
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// Periods are the three rolling analysis windows used across Analysis Pack v1 tables.
//
//	current_7d  = metricDate - 6 days ... metricDate
//	previous_7d = metricDate - 13 days ... metricDate - 7 days
//	trend       = metricDate - 29 days ... metricDate (matches AnalysisPackDailyHistoryDaysV1)
type Periods struct {
	Current7D  analysisdata.DateRange
	Previous7D analysisdata.DateRange
	Trend      analysisdata.DateRange
}

// ComputePackPeriods derives the frozen v1 rolling windows from metricDate (UTC date-only).
func ComputePackPeriods(metricDate time.Time) Periods {
	d := analysisdata.DateOnlyUTC(metricDate)
	return Periods{
		Current7D: analysisdata.DateRange{
			From: analysisdata.FormatDate(d.AddDate(0, 0, -6)),
			To:   analysisdata.FormatDate(d),
		},
		Previous7D: analysisdata.DateRange{
			From: analysisdata.FormatDate(d.AddDate(0, 0, -13)),
			To:   analysisdata.FormatDate(d.AddDate(0, 0, -7)),
		},
		Trend: analysisdata.DateRange{
			From: analysisdata.FormatDate(d.AddDate(0, 0, -(contract.AnalysisPackDailyHistoryDaysV1 - 1))),
			To:   analysisdata.FormatDate(d),
		},
	}
}

// ToContract renders Periods as the frozen manifest.json periods block.
func (p Periods) ToContract() contract.AnalysisPackPeriodsV1 {
	return contract.AnalysisPackPeriodsV1{
		Current7D:  contract.ContractDateRange{From: p.Current7D.From, To: p.Current7D.To},
		Previous7D: contract.ContractDateRange{From: p.Previous7D.From, To: p.Previous7D.To},
		Trend:      contract.ContractDateRange{From: p.Trend.From, To: p.Trend.To},
	}
}

// dateRangeInclusive returns every YYYY-MM-DD date string in [from, to] ascending.
func dateRangeInclusive(from, to string) []string {
	start := analysisdata.ParseDate(from)
	end := analysisdata.ParseDate(to)
	if end.Before(start) {
		return nil
	}
	out := make([]string, 0, int(end.Sub(start).Hours()/24)+1)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		out = append(out, analysisdata.FormatDate(d))
	}
	return out
}

// inRange reports whether dateStr (YYYY-MM-DD) falls within [r.From, r.To] inclusive.
func inRange(r analysisdata.DateRange, dateStr string) bool {
	return dateStr >= r.From && dateStr <= r.To
}
