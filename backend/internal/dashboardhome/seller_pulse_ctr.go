package dashboardhome

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

const sellerPulseCTRNeutralPPThreshold = 0.15

func computeAccountCTRPercent(impressions, productViews int64) (*float64, bool) {
	if impressions <= 0 {
		return nil, false
	}
	v := math.Round((float64(productViews)/float64(impressions))*10000) / 100
	return &v, true
}

func pgInt8Value(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

func buildCTRMetricsByDate(history []dbgen.ProductAnalyticsDaily) map[string]dailyRatioPoint {
	out := make(map[string]dailyRatioPoint)
	for _, row := range filterAccountProductAnalyticsForFunnel(history) {
		impressions := pgInt8Value(row.Impressions)
		views := pgInt8Value(row.ProductViews)
		ctr, ok := computeAccountCTRPercent(impressions, views)
		if !ok || ctr == nil || !row.MetricDate.Valid {
			continue
		}
		key := row.MetricDate.Time.UTC().Format("2006-01-02")
		out[key] = dailyRatioPoint{value: *ctr, hasData: true}
	}
	return out
}

func formatCTRPercentValue(pct float64) string {
	return fmt.Sprintf("%.1f%%", pct)
}

func formatCTRDeltaDisplay(deltaPP float64) string {
	sign := ""
	if deltaPP > 0 {
		sign = "+"
	} else if deltaPP < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.1f п.п. к вчера", sign, deltaPP)
}

func buildCTRKPI(
	currentRow dbgen.ProductAnalyticsDaily,
	prevRow *dbgen.ProductAnalyticsDaily,
	sparkline []SparklinePoint,
) (KPICard, bool) {
	impressions := pgInt8Value(currentRow.Impressions)
	views := pgInt8Value(currentRow.ProductViews)
	ctrToday, ok := computeAccountCTRPercent(impressions, views)
	if !ok || ctrToday == nil {
		return KPICard{}, false
	}
	kpi := KPICard{
		Key:         "ctr",
		Label:       "CTR",
		Value:       formatCTRPercentValue(*ctrToday),
		Sparkline7D: sparkline,
		Hint:        "Просмотры карточек / показы",
	}
	if prevRow != nil {
		prevImp := pgInt8Value(prevRow.Impressions)
		prevViews := pgInt8Value(prevRow.ProductViews)
		if ctrPrev, prevOK := computeAccountCTRPercent(prevImp, prevViews); prevOK && ctrPrev != nil {
			deltaPP := math.Round((*ctrToday-*ctrPrev)*100) / 100
			kpi.Delta = &KPICardDelta{
				Abs:     deltaPP,
				Label:   "к вчера",
				Unit:    "percentage_point",
				Display: formatCTRDeltaDisplay(deltaPP),
			}
		}
	}
	return kpi, true
}

func (s *Service) loadAccountCTRSparkline(
	ctx context.Context,
	sellerID int64,
	from, to time.Time,
) []SparklinePoint {
	historyRows, err := s.q.ListProductAnalyticsDailyBySellerAndDateRange(ctx, dbgen.ListProductAnalyticsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      dateValue(from),
		MetricDate_2:    dateValue(to),
	})
	if err != nil {
		return nil
	}
	return buildRatioSparklineSeries(from, to, buildCTRMetricsByDate(historyRows))
}
