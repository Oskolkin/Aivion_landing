package devseed

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DevAdEfficiencySeedSource            = "dev_fixture_ad_efficiency"
	DevAdEfficiencyCampaignIDMin   int64 = 900001
	DevAdEfficiencyCampaignIDMax   int64 = 900099
	DevAdEfficiencyTrendWindowDays       = 7
)

type AdEfficiencySeedScenario string

const (
	AdEfficiencyScenarioAvailable          AdEfficiencySeedScenario = "available"
	AdEfficiencyScenarioAvailableAIPending AdEfficiencySeedScenario = "available_ai_pending"
	AdEfficiencyScenarioPartial            AdEfficiencySeedScenario = "partial"
	AdEfficiencyScenarioNotEnoughData      AdEfficiencySeedScenario = "not_enough_data"
	AdEfficiencyScenarioReset              AdEfficiencySeedScenario = "reset"
)

type AdEfficiencySeedOptions struct {
	SellerAccountID int64
	MetricDate      *time.Time
	Scenario        AdEfficiencySeedScenario
	ResetFirst      bool
	WithAINarrative *bool
}

type AdEfficiencySeedResult struct {
	SellerAccountID          int64
	Scenario                 AdEfficiencySeedScenario
	MetricDate               time.Time
	DateFrom                 time.Time
	DateTo                   time.Time
	CampaignsUpserted        int
	DailyMetricsUpserted     int
	HighDRRCampaignsExpected int
	NoOrderCampaignsExpected int
	SampleDaysExpected       int
	MissingDaysExpected      int
	AINarrative              string
	WidgetStatus             string
	HasData                  bool
}

type devAdCampaignKind string

const (
	devAdKindHighDRR  devAdCampaignKind = "high_drr"
	devAdKindNoOrders devAdCampaignKind = "no_orders"
	devAdKindNormal   devAdCampaignKind = "normal"
)

type devAdCampaignDef struct {
	ExternalID int64
	Name       string
	Kind       devAdCampaignKind
}

var devAdEfficiencyTrendDRR = []float64{28, 32, 27, 29, 31, 33, 38}

func devAdEfficiencyCampaignDefs() []devAdCampaignDef {
	return []devAdCampaignDef{
		{900001, "Холодный трафик", devAdKindHighDRR},
		{900002, "Автокампания + категория A", devAdKindHighDRR},
		{900003, "Поиск + брендовые запросы", devAdKindHighDRR},
		{900004, "РСЯ + интересы", devAdKindNoOrders},
		{900005, "Динамический ретаргетинг", devAdKindNoOrders},
		{900006, "Брендовая кампания", devAdKindNormal},
		{900007, "Ретаргетинг — корзина", devAdKindNormal},
		{900008, "Категорийный поиск", devAdKindNormal},
	}
}

// SeedAdEfficiency writes dev-only campaign-level advertising fixtures for dashboard ad_efficiency testing.
func SeedAdEfficiency(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, opts AdEfficiencySeedOptions) (*AdEfficiencySeedResult, error) {
	if err := AssertDevSeedAllowed(); err != nil {
		return nil, err
	}
	if opts.SellerAccountID <= 0 {
		return nil, fmt.Errorf("seller-account-id must be > 0")
	}
	switch opts.Scenario {
	case AdEfficiencyScenarioAvailable, AdEfficiencyScenarioAvailableAIPending, AdEfficiencyScenarioPartial,
		AdEfficiencyScenarioNotEnoughData, AdEfficiencyScenarioReset:
	default:
		return nil, fmt.Errorf("unknown scenario %q", opts.Scenario)
	}

	metricDate, err := resolveAdEfficiencyMetricDate(ctx, pool, opts.SellerAccountID, opts.MetricDate)
	if err != nil {
		return nil, err
	}
	if opts.MetricDate != nil {
		if err := ensureDashboardAnchorForMetricDate(ctx, pool, opts.SellerAccountID, metricDate); err != nil {
			return nil, err
		}
	}

	dateFrom := metricDate.UTC().AddDate(0, 0, -(DevAdEfficiencyTrendWindowDays - 1))
	result := &AdEfficiencySeedResult{
		SellerAccountID:          opts.SellerAccountID,
		Scenario:                 opts.Scenario,
		MetricDate:               metricDate.UTC(),
		DateFrom:                 dateFrom,
		DateTo:                   metricDate.UTC(),
		HighDRRCampaignsExpected: 3,
		NoOrderCampaignsExpected: 2,
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	if opts.ResetFirst || opts.Scenario == AdEfficiencyScenarioReset || opts.Scenario == AdEfficiencyScenarioNotEnoughData {
		if err := deleteDevAdEfficiencyFixtureData(ctx, tx, opts.SellerAccountID); err != nil {
			return nil, err
		}
		if err := dashboardhome.DeleteDevDashboardNarrative(ctx, q, opts.SellerAccountID, metricDate); err != nil {
			return nil, err
		}
	}

	switch opts.Scenario {
	case AdEfficiencyScenarioReset, AdEfficiencyScenarioNotEnoughData:
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		result.AINarrative = "not_enough_data"
		result.WidgetStatus = "not_enough_data"
		return result, nil
	}

	includeDays := devAdEfficiencyDayIndexes(opts.Scenario)
	campaigns, metrics, err := seedDevAdEfficiencyCampaignData(ctx, q, opts.SellerAccountID, metricDate, includeDays)
	if err != nil {
		return nil, err
	}
	result.CampaignsUpserted = campaigns
	result.DailyMetricsUpserted = metrics
	result.SampleDaysExpected = len(includeDays)
	result.MissingDaysExpected = DevAdEfficiencyTrendWindowDays - len(includeDays)

	withAI := opts.Scenario == AdEfficiencyScenarioAvailable
	if opts.WithAINarrative != nil {
		withAI = *opts.WithAINarrative
	}
	needDeleteNarrative := opts.Scenario == AdEfficiencyScenarioAvailableAIPending

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	switch {
	case withAI:
		if err := dashboardhome.UpsertDevAdEfficiencyNarrative(ctx, cfg, pool, opts.SellerAccountID, metricDate, dashboardhome.DevAdEfficiencyNarrativeFixture()); err != nil {
			return nil, fmt.Errorf("seed ad efficiency narrative: %w", err)
		}
		result.AINarrative = "available"
	case needDeleteNarrative:
		q := dbgen.New(pool)
		if err := dashboardhome.DeleteDevDashboardNarrative(ctx, q, opts.SellerAccountID, metricDate); err != nil {
			return nil, err
		}
		result.AINarrative = "pending"
	default:
		result.AINarrative = "pending"
	}

	dto, err := verifyAdEfficiencyDashboard(ctx, pool, opts.SellerAccountID, metricDate)
	if err != nil {
		return nil, err
	}
	result.WidgetStatus = dto.WidgetStatus
	result.HasData = dto.HasData

	return result, nil
}

func devAdEfficiencyDayIndexes(scenario AdEfficiencySeedScenario) []int {
	switch scenario {
	case AdEfficiencyScenarioPartial:
		return []int{3, 4, 5, 6}
	default:
		out := make([]int, DevAdEfficiencyTrendWindowDays)
		for i := range out {
			out[i] = i
		}
		return out
	}
}

func resolveAdEfficiencyMetricDate(ctx context.Context, pool *pgxpool.Pool, sellerID int64, override *time.Time) (time.Time, error) {
	if override != nil {
		return override.UTC().Truncate(24 * time.Hour), nil
	}
	q := dbgen.New(pool)
	latest, err := q.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, sellerID)
	if err != nil {
		return time.Time{}, fmt.Errorf("resolve dashboard metric date: %w", err)
	}
	if !latest.Valid {
		return time.Time{}, fmt.Errorf("dashboard metric date unavailable: run seed-mvp first or pass --metric-date YYYY-MM-DD")
	}
	metricDate := latest.Time.UTC()
	if metricDate.Year() <= 1970 {
		return time.Time{}, fmt.Errorf("dashboard metric date unavailable: run seed-mvp first or pass --metric-date YYYY-MM-DD")
	}
	return metricDate, nil
}

func ensureDashboardAnchorForMetricDate(ctx context.Context, pool *pgxpool.Pool, sellerID int64, metricDate time.Time) error {
	q := dbgen.New(pool)
	_, err := q.UpsertDailyAccountMetric(ctx, dbgen.UpsertDailyAccountMetricParams{
		SellerAccountID: sellerID,
		MetricDate:      pgtype.Date{Time: metricDate.UTC(), Valid: true},
		Revenue:         moneyNV(250_000),
		OrdersCount:     120,
		ReturnsCount:    3,
		CancelCount:     2,
		UnitsSold:       int4NV(140),
	})
	if err != nil {
		return fmt.Errorf("ensure dashboard anchor metric: %w", err)
	}
	return nil
}

func deleteDevAdEfficiencyFixtureData(ctx context.Context, tx pgx.Tx, sellerID int64) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM ad_metrics_daily
		WHERE seller_account_id = $1
		  AND campaign_external_id BETWEEN $2 AND $3
	`, sellerID, DevAdEfficiencyCampaignIDMin, DevAdEfficiencyCampaignIDMax)
	if err != nil {
		return fmt.Errorf("delete dev ad metrics: %w", err)
	}
	_, err = tx.Exec(ctx, `
		DELETE FROM ad_campaigns
		WHERE seller_account_id = $1
		  AND campaign_external_id BETWEEN $2 AND $3
	`, sellerID, DevAdEfficiencyCampaignIDMin, DevAdEfficiencyCampaignIDMax)
	if err != nil {
		return fmt.Errorf("delete dev ad campaigns: %w", err)
	}
	return nil
}

func seedDevAdEfficiencyCampaignData(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	metricDate time.Time,
	dayIndexes []int,
) (campaigns int, metrics int, err error) {
	dateFrom := metricDate.UTC().AddDate(0, 0, -(DevAdEfficiencyTrendWindowDays - 1))
	daySet := make(map[int]struct{}, len(dayIndexes))
	for _, idx := range dayIndexes {
		daySet[idx] = struct{}{}
	}

	defs := devAdEfficiencyCampaignDefs()
	for _, def := range defs {
		raw, _ := json.Marshal(map[string]any{
			"seeded_by": DevAdEfficiencySeedSource,
			"kind":      string(def.Kind),
		})
		if _, err := q.UpsertAdCampaign(ctx, dbgen.UpsertAdCampaignParams{
			SellerAccountID:    sellerID,
			CampaignExternalID: def.ExternalID,
			CampaignName:       def.Name,
			CampaignType:       textNV("search_promo"),
			PlacementType:      textNV("search"),
			Status:             textNV("running"),
			BudgetAmount:       moneyNV(100_000),
			BudgetDaily:        moneyNV(5_000),
			RawAttributes:      raw,
		}); err != nil {
			return 0, 0, fmt.Errorf("upsert campaign %d: %w", def.ExternalID, err)
		}
		campaigns++
	}

	for dayIdx := 0; dayIdx < DevAdEfficiencyTrendWindowDays; dayIdx++ {
		if _, ok := daySet[dayIdx]; !ok {
			continue
		}
		day := dateFrom.AddDate(0, 0, dayIdx)
		alloc := allocateDevAdEfficiencyDay(dayIdx, defs)
		for _, row := range alloc {
			mRaw, _ := json.Marshal(map[string]any{
				"seeded_by": DevAdEfficiencySeedSource,
				"day_index": dayIdx,
			})
			_, err := q.UpsertAdMetricDaily(ctx, dbgen.UpsertAdMetricDailyParams{
				SellerAccountID:    sellerID,
				CampaignExternalID: row.externalID,
				MetricDate:         pgtype.Date{Time: day.UTC(), Valid: true},
				Impressions:        row.impressions,
				Clicks:             row.clicks,
				Spend:              moneyNV(row.spend),
				OrdersCount:        row.orders,
				Revenue:            moneyNV(row.revenue),
				RawAttributes:      mRaw,
			})
			if err != nil {
				return campaigns, metrics, fmt.Errorf("upsert metric %d %s: %w", row.externalID, day.Format("2006-01-02"), err)
			}
			metrics++
		}
	}
	return campaigns, metrics, nil
}

type devAdDayAllocation struct {
	externalID  int64
	spend       float64
	revenue     float64
	orders      int32
	impressions int64
	clicks      int64
}

func allocateDevAdEfficiencyDay(dayIdx int, defs []devAdCampaignDef) []devAdDayAllocation {
	targetDRR := devAdEfficiencyTrendDRR[dayIdx]
	totalSpend := 52_000 + float64(dayIdx)*3_000
	targetRevenue := totalSpend / (targetDRR / 100)

	weights := map[devAdCampaignKind]float64{
		devAdKindHighDRR:  0.42,
		devAdKindNoOrders: 0.18,
		devAdKindNormal:   0.40,
	}
	kindCount := map[devAdCampaignKind]int{
		devAdKindHighDRR:  0,
		devAdKindNoOrders: 0,
		devAdKindNormal:   0,
	}
	for _, def := range defs {
		kindCount[def.Kind]++
	}

	out := make([]devAdDayAllocation, 0, len(defs))
	kindByID := make(map[int64]devAdCampaignKind, len(defs))
	var provisionalRevenue float64
	for _, def := range defs {
		kindByID[def.ExternalID] = def.Kind
		kindSpend := totalSpend * weights[def.Kind] / float64(kindCount[def.Kind])
		var kindRevenue float64
		var orders int32
		switch def.Kind {
		case devAdKindHighDRR:
			ratio := 1.62
			switch def.ExternalID {
			case 900001:
				ratio = 1.21
			case 900002:
				ratio = 1.62
			case 900003:
				ratio = 1.90
			}
			kindRevenue = kindSpend / ratio
			orders = int32(math.Max(1, math.Round(kindRevenue/9000)))
		case devAdKindNoOrders:
			kindRevenue = 0
			orders = 0
		default:
			kindRevenue = kindSpend / 0.45
			orders = int32(math.Max(1, math.Round(kindRevenue/7000)))
		}
		clicks := int64(math.Max(8, math.Round(kindSpend/35)))
		impr := clicks * int64(8+dayIdx%3)
		out = append(out, devAdDayAllocation{
			externalID:  def.ExternalID,
			spend:       mvpRoundMoney(kindSpend),
			revenue:     mvpRoundMoney(kindRevenue),
			orders:      orders,
			impressions: impr,
			clicks:      clicks,
		})
		provisionalRevenue += kindRevenue
	}

	if provisionalRevenue > 0 && targetRevenue > 0 {
		scale := targetRevenue / provisionalRevenue
		var spendSum, revenueSum float64
		for i := range out {
			if kindByID[out[i].externalID] == devAdKindNoOrders {
				continue
			}
			out[i].revenue = mvpRoundMoney(math.Max(500, out[i].revenue*scale))
			if out[i].orders == 0 {
				out[i].orders = 1
			}
		}
		for i := range out {
			spendSum += out[i].spend
			revenueSum += out[i].revenue
		}
		if revenueSum > 0 {
			current := (spendSum / revenueSum) * 100
			delta := targetDRR - current
			for i := len(out) - 1; i >= 0; i-- {
				if kindByID[out[i].externalID] == devAdKindNormal && out[i].revenue > 0 {
					out[i].revenue = mvpRoundMoney(math.Max(1000, out[i].revenue*(1+delta/100)))
					break
				}
			}
		}
	}
	return out
}

func verifyAdEfficiencyDashboard(ctx context.Context, pool *pgxpool.Pool, sellerID int64, metricDate time.Time) (analytics.AdEfficiencyDashboardDTO, error) {
	svc := analytics.NewAdvertisingService(pool)
	return svc.BuildAdEfficiencyDashboard(ctx, sellerID, analytics.AdvertisingQuery{}, metricDate)
}

func PrintAdEfficiencySeedReport(result *AdEfficiencySeedResult) {
	if result == nil {
		return
	}
	fmt.Println("Ad efficiency seed completed.")
	fmt.Printf("seller_id: %d\n", result.SellerAccountID)
	fmt.Printf("scenario: %s\n", result.Scenario)
	fmt.Printf("metric_date: %s\n", result.MetricDate.Format("2006-01-02"))
	fmt.Printf("date_from: %s\n", result.DateFrom.Format("2006-01-02"))
	fmt.Printf("date_to: %s\n", result.DateTo.Format("2006-01-02"))
	fmt.Printf("campaigns_upserted: %d\n", result.CampaignsUpserted)
	fmt.Printf("daily_metrics_upserted: %d\n", result.DailyMetricsUpserted)
	fmt.Printf("high_drr_campaigns_expected: %d\n", result.HighDRRCampaignsExpected)
	fmt.Printf("no_order_campaigns_expected: %d\n", result.NoOrderCampaignsExpected)
	if result.Scenario == AdEfficiencyScenarioPartial {
		fmt.Printf("sample_days_expected: %d\n", result.SampleDaysExpected)
		fmt.Printf("missing_days_expected: %d\n", result.MissingDaysExpected)
	}
	fmt.Printf("ai_narrative: %s\n", result.AINarrative)
	if result.WidgetStatus != "" {
		fmt.Printf("widget_status: %s\n", result.WidgetStatus)
	}
	fmt.Println("next: open /app/dashboard")
}
