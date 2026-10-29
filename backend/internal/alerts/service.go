package alerts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo Repository
}

type RunSalesAlertsSummary struct {
	SellerAccountID int64
	AsOfDate        time.Time
	PreviousDate    time.Time
	GeneratedAlerts int
	UpsertedAlerts  int
	SkippedRules    int
}

type RunStockAlertsSummary struct {
	SellerAccountID int64
	AsOfDate        time.Time
	GeneratedAlerts int
	UpsertedAlerts  int
	SkippedRules    int
}

type RunAdvertisingAlertsSummary struct {
	SellerAccountID int64
	AsOfDate        time.Time
	DateFrom        time.Time
	DateTo          time.Time
	GeneratedAlerts int
	UpsertedAlerts  int
	SkippedRules    int
}

type RunPriceEconomicsAlertsSummary struct {
	SellerAccountID int64
	AsOfDate        time.Time
	GeneratedAlerts int
	UpsertedAlerts  int
	SkippedRules    int
}

type RunForAccountSummary struct {
	SellerAccountID      int64
	AsOfDate             time.Time
	RunID                int64
	Status               RunStatus
	Sales                RunSalesAlertsSummary
	Stock                RunStockAlertsSummary
	Advertising          RunAdvertisingAlertsSummary
	PriceEconomics       RunPriceEconomicsAlertsSummary
	TotalGeneratedAlerts int
	TotalUpsertedAlerts  int
	TotalSkippedRules    int
	StartedAt            time.Time
	FinishedAt           time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) UpsertAlertFromRuleResult(ctx context.Context, sellerAccountID int64, result RuleResult) (Alert, error) {
	input := RuleResultToUpsertInput(sellerAccountID, result)
	return s.repo.UpsertAlert(ctx, input)
}

func (s *Service) ListOpenAlerts(ctx context.Context, sellerAccountID int64, limit int, offset int) ([]Alert, error) {
	return s.repo.ListOpenAlerts(ctx, sellerAccountID, limit, offset)
}

func (s *Service) ListAlerts(ctx context.Context, sellerAccountID int64, filter ListFilter) ([]Alert, error) {
	return s.repo.ListAlertsFiltered(ctx, sellerAccountID, filter)
}

func (s *Service) GetSummary(ctx context.Context, sellerAccountID int64) (Summary, error) {
	openTotal, err := s.repo.CountOpenAlerts(ctx, sellerAccountID)
	if err != nil {
		return Summary{}, err
	}
	bySeverity, err := s.repo.CountOpenAlertsBySeverity(ctx, sellerAccountID)
	if err != nil {
		return Summary{}, err
	}
	byGroup, err := s.repo.CountOpenAlertsByGroup(ctx, sellerAccountID)
	if err != nil {
		return Summary{}, err
	}
	latestRun, err := s.repo.GetLatestRun(ctx, sellerAccountID)
	var latestRunPtr *AlertRun
	if err == nil {
		latestRunPtr = &latestRun
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Summary{}, err
	}
	return Summary{
		OpenTotal:  openTotal,
		BySeverity: bySeverity,
		ByGroup:    byGroup,
		LatestRun:  latestRunPtr,
	}, nil
}

func (s *Service) ResolveAlert(ctx context.Context, sellerAccountID int64, alertID int64) (Alert, error) {
	return s.repo.ResolveAlert(ctx, sellerAccountID, alertID)
}

func (s *Service) DismissAlert(ctx context.Context, sellerAccountID int64, alertID int64) (Alert, error) {
	return s.repo.DismissAlert(ctx, sellerAccountID, alertID)
}

func (s *Service) CreateRun(ctx context.Context, sellerAccountID int64, runType RunType) (AlertRun, error) {
	return s.repo.CreateRun(ctx, sellerAccountID, runType)
}

func (s *Service) CompleteRun(ctx context.Context, input CompleteRunInput) (AlertRun, error) {
	return s.repo.CompleteRun(ctx, input)
}

func (s *Service) FailRun(ctx context.Context, sellerAccountID int64, runID int64, errorMessage string) (AlertRun, error) {
	return s.repo.FailRun(ctx, sellerAccountID, runID, errorMessage)
}

func (s *Service) GetLatestRun(ctx context.Context, sellerAccountID int64) (AlertRun, error) {
	return s.repo.GetLatestRun(ctx, sellerAccountID)
}

func (s *Service) RunSalesAlerts(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (RunSalesAlertsSummary, error) {
	asOf := asOfDate.UTC().Truncate(24 * time.Hour)
	previous := asOf.AddDate(0, 0, -1)

	currentAccount, err := s.repo.GetDailyAccountMetricByDate(ctx, sellerAccountID, asOf)
	if err != nil {
		return RunSalesAlertsSummary{}, fmt.Errorf("get current account metric: %w", err)
	}
	previousAccount, err := s.repo.GetDailyAccountMetricByDate(ctx, sellerAccountID, previous)
	if err != nil {
		return RunSalesAlertsSummary{}, fmt.Errorf("get previous account metric: %w", err)
	}
	currentSKUs, err := s.repo.ListDailySKUMetricsByDate(ctx, sellerAccountID, asOf)
	if err != nil {
		return RunSalesAlertsSummary{}, fmt.Errorf("list current sku metrics: %w", err)
	}
	previousSKUs, err := s.repo.ListDailySKUMetricsByDate(ctx, sellerAccountID, previous)
	if err != nil {
		return RunSalesAlertsSummary{}, fmt.Errorf("list previous sku metrics: %w", err)
	}

	previousByProduct := make(map[int64]SKUDailyMetric, len(previousSKUs))
	for _, sku := range previousSKUs {
		previousByProduct[sku.OzonProductID] = sku
	}

	eval := EvaluateSalesRules(SalesRuleEvaluationInput{
		SellerAccountID:  sellerAccountID,
		AsOfDate:         asOf,
		PreviousDate:     previous,
		CurrentAccount:   currentAccount,
		PreviousAccount:  previousAccount,
		CurrentSKUs:      currentSKUs,
		PreviousSKUsByID: previousByProduct,
	})

	upserted := 0
	for _, result := range eval.RuleResults {
		if _, err := s.UpsertAlertFromRuleResult(ctx, sellerAccountID, result); err != nil {
			return RunSalesAlertsSummary{}, fmt.Errorf("upsert sales alert type=%s: %w", result.AlertType, err)
		}
		upserted++
	}

	return RunSalesAlertsSummary{
		SellerAccountID: sellerAccountID,
		AsOfDate:        asOf,
		PreviousDate:    previous,
		GeneratedAlerts: len(eval.RuleResults),
		UpsertedAlerts:  upserted,
		SkippedRules:    eval.Skipped,
	}, nil
}

func (s *Service) RunStockAlerts(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (RunStockAlertsSummary, error) {
	asOf := asOfDate.UTC().Truncate(24 * time.Hour)
	skuMetrics, err := s.repo.ListDailySKUMetricsByDate(ctx, sellerAccountID, asOf)
	if err != nil {
		return RunStockAlertsSummary{}, fmt.Errorf("list sku metrics by date: %w", err)
	}
	importance, err := s.buildSKUImportanceForWindow(ctx, sellerAccountID, asOf)
	if err != nil {
		return RunStockAlertsSummary{}, err
	}
	fboStorageRisk, err := s.buildFBOStorageRiskBySKU(ctx, sellerAccountID, asOf)
	if err != nil {
		return RunStockAlertsSummary{}, err
	}

	eval := EvaluateStockRules(StockRuleEvaluationInput{
		SellerAccountID:        sellerAccountID,
		AsOfDate:               formatDate(asOf),
		SKUMetrics:             skuMetrics,
		SKUImportanceByProduct: importance,
		FBOStorageRiskBySKU:    fboStorageRisk,
	})

	upserted := 0
	for _, result := range eval.RuleResults {
		if _, err := s.UpsertAlertFromRuleResult(ctx, sellerAccountID, result); err != nil {
			return RunStockAlertsSummary{}, fmt.Errorf("upsert stock alert type=%s: %w", result.AlertType, err)
		}
		upserted++
	}

	return RunStockAlertsSummary{
		SellerAccountID: sellerAccountID,
		AsOfDate:        asOf,
		GeneratedAlerts: len(eval.RuleResults),
		UpsertedAlerts:  upserted,
		SkippedRules:    eval.Skipped,
	}, nil
}

func (s *Service) RunAdvertisingAlerts(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (RunAdvertisingAlertsSummary, error) {
	dateTo := asOfDate.UTC().Truncate(24 * time.Hour)
	dateFrom := dateTo.AddDate(0, 0, -6)

	campaigns, err := s.repo.ListAdCampaignSummariesByDateRange(ctx, sellerAccountID, dateFrom, dateTo)
	if err != nil {
		return RunAdvertisingAlertsSummary{}, fmt.Errorf("list ad campaign summaries: %w", err)
	}

	eval := EvaluateAdvertisingRules(AdvertisingRuleEvaluationInput{
		SellerAccountID:   sellerAccountID,
		DateFrom:          formatDate(dateFrom),
		DateTo:            formatDate(dateTo),
		AsOfDate:          formatDate(dateTo),
		CampaignSummaries: campaigns,
	})

	upserted := 0
	for _, result := range eval.RuleResults {
		if _, err := s.UpsertAlertFromRuleResult(ctx, sellerAccountID, result); err != nil {
			return RunAdvertisingAlertsSummary{}, fmt.Errorf("upsert advertising alert type=%s: %w", result.AlertType, err)
		}
		upserted++
	}

	return RunAdvertisingAlertsSummary{
		SellerAccountID: sellerAccountID,
		AsOfDate:        dateTo,
		DateFrom:        dateFrom,
		DateTo:          dateTo,
		GeneratedAlerts: len(eval.RuleResults),
		UpsertedAlerts:  upserted,
		SkippedRules:    eval.Skipped,
	}, nil
}

func (s *Service) RunPriceEconomicsAlerts(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (RunPriceEconomicsAlertsSummary, error) {
	asOf := asOfDate.UTC().Truncate(24 * time.Hour)
	if PriceEconomicsGenerationDisabled {
		return RunPriceEconomicsAlertsSummary{
			SellerAccountID: sellerAccountID,
			AsOfDate:        asOf,
		}, nil
	}

	products, err := s.repo.ListProductsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return RunPriceEconomicsAlertsSummary{}, fmt.Errorf("list products: %w", err)
	}
	effectiveConstraints, err := s.repo.ListSKUEffectiveConstraintsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return RunPriceEconomicsAlertsSummary{}, fmt.Errorf("list sku effective constraints: %w", err)
	}
	skuMetrics, err := s.repo.ListDailySKUMetricsByDate(ctx, sellerAccountID, asOf)
	if err != nil {
		return RunPriceEconomicsAlertsSummary{}, fmt.Errorf("list daily sku metrics for price context: %w", err)
	}
	importance, err := s.buildSKUImportanceForWindow(ctx, sellerAccountID, asOf)
	if err != nil {
		return RunPriceEconomicsAlertsSummary{}, err
	}

	constraintsByProduct := make(map[int64]int, len(effectiveConstraints))
	for idx, c := range effectiveConstraints {
		constraintsByProduct[c.OzonProductID] = idx
	}
	skuMetricsByProduct := make(map[int64]SKUDailyMetric, len(skuMetrics))
	for _, m := range skuMetrics {
		skuMetricsByProduct[m.OzonProductID] = m
	}

	contexts := make([]ProductPricingContext, 0, len(products))
	for _, p := range products {
		ctxItem := ProductPricingContext{
			SellerAccountID: p.SellerAccountID,
			OzonProductID:   p.OzonProductID,
			SKU:             int8Ptr(p.Sku),
			OfferID:         textPtr(p.OfferID),
			ProductName:     p.Name,
			ReferencePrice:  numericFloat64Ptr(p.ReferencePrice),
		}
		if idx, ok := constraintsByProduct[p.OzonProductID]; ok {
			ec := effectiveConstraints[idx]
			ctxItem.HasEffectiveConstraint = true
			ctxItem.EffectiveMinPrice = numericFloat64Ptr(ec.EffectiveMinPrice)
			ctxItem.EffectiveMaxPrice = numericFloat64Ptr(ec.EffectiveMaxPrice)
			ctxItem.ImpliedCost = numericFloat64Ptr(ec.ImpliedCost)
			source := ec.ResolvedFromScopeType
			ctxItem.ConstraintSource = &source
			ruleID := ec.RuleID
			ctxItem.ConstraintRuleID = &ruleID
			if ctxItem.SKU == nil {
				ctxItem.SKU = int8Ptr(ec.Sku)
			}
			if ctxItem.OfferID == nil {
				ctxItem.OfferID = textPtr(ec.OfferID)
			}
		}
		if metric, ok := skuMetricsByProduct[p.OzonProductID]; ok {
			ctxItem.RevenueForPeriod = metric.Revenue
			ctxItem.OrdersForPeriod = metric.OrdersCount
		}
		contexts = append(contexts, ctxItem)
	}

	eval := EvaluatePriceEconomicsRules(PriceRuleEvaluationInput{
		SellerAccountID:        sellerAccountID,
		AsOfDate:               formatDate(asOf),
		Products:               contexts,
		SKUImportanceByProduct: importance,
	})

	upserted := 0
	for _, result := range eval.RuleResults {
		if _, err := s.UpsertAlertFromRuleResult(ctx, sellerAccountID, result); err != nil {
			return RunPriceEconomicsAlertsSummary{}, fmt.Errorf("upsert price alert type=%s: %w", result.AlertType, err)
		}
		upserted++
	}

	return RunPriceEconomicsAlertsSummary{
		SellerAccountID: sellerAccountID,
		AsOfDate:        asOf,
		GeneratedAlerts: len(eval.RuleResults),
		UpsertedAlerts:  upserted,
		SkippedRules:    eval.Skipped,
	}, nil
}

func (s *Service) RunForAccount(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (RunForAccountSummary, error) {
	return s.RunForAccountWithType(ctx, sellerAccountID, asOfDate, RunTypeManual)
}

func (s *Service) RunForAccountWithType(ctx context.Context, sellerAccountID int64, asOfDate time.Time, runType RunType) (RunForAccountSummary, error) {
	asOf := asOfDate.UTC().Truncate(24 * time.Hour)
	startedAt := time.Now().UTC()

	run, err := s.CreateRun(ctx, sellerAccountID, runType)
	if err != nil {
		return RunForAccountSummary{}, fmt.Errorf("create alert run: %w", err)
	}
	if !run.StartedAt.IsZero() {
		startedAt = run.StartedAt
	}

	sales, err := s.RunSalesAlerts(ctx, sellerAccountID, asOf)
	if err != nil {
		return RunForAccountSummary{}, s.failRunAndWrap(ctx, sellerAccountID, run.ID, err, "run sales alerts")
	}
	stock, err := s.RunStockAlerts(ctx, sellerAccountID, asOf)
	if err != nil {
		return RunForAccountSummary{}, s.failRunAndWrap(ctx, sellerAccountID, run.ID, err, "run stock alerts")
	}
	advertising, err := s.RunAdvertisingAlerts(ctx, sellerAccountID, asOf)
	if err != nil {
		return RunForAccountSummary{}, s.failRunAndWrap(ctx, sellerAccountID, run.ID, err, "run advertising alerts")
	}
	// Price/economics alert generation is disabled for MVP (see PriceEconomicsGenerationDisabled).
	price := RunPriceEconomicsAlertsSummary{
		SellerAccountID: sellerAccountID,
		AsOfDate:        asOf,
	}

	totalUpserted := sales.UpsertedAlerts + stock.UpsertedAlerts + advertising.UpsertedAlerts
	totalGenerated := sales.GeneratedAlerts + stock.GeneratedAlerts + advertising.GeneratedAlerts
	totalSkipped := sales.SkippedRules + stock.SkippedRules + advertising.SkippedRules

	complete, err := s.CompleteRun(ctx, CompleteRunInput{
		RunID:            run.ID,
		SellerAccountID:  sellerAccountID,
		SalesAlertsCount: int32(sales.UpsertedAlerts),
		StockAlertsCount: int32(stock.UpsertedAlerts),
		AdAlertsCount:    int32(advertising.UpsertedAlerts),
		PriceAlertsCount: int32(price.UpsertedAlerts),
		TotalAlertsCount: int32(totalUpserted),
	})
	if err != nil {
		return RunForAccountSummary{}, fmt.Errorf("complete alert run id=%d: %w", run.ID, err)
	}

	finishedAt := time.Now().UTC()
	if complete.FinishedAt != nil {
		finishedAt = *complete.FinishedAt
	}

	// Stale-alert auto-resolve is intentionally deferred for MVP.
	return RunForAccountSummary{
		SellerAccountID:      sellerAccountID,
		AsOfDate:             asOf,
		RunID:                run.ID,
		Status:               complete.Status,
		Sales:                sales,
		Stock:                stock,
		Advertising:          advertising,
		PriceEconomics:       price,
		TotalGeneratedAlerts: totalGenerated,
		TotalUpsertedAlerts:  totalUpserted,
		TotalSkippedRules:    totalSkipped,
		StartedAt:            startedAt,
		FinishedAt:           finishedAt,
	}, nil
}

func (s *Service) buildFBOStorageRiskBySKU(ctx context.Context, sellerAccountID int64, asOf time.Time) (FBOStorageRiskBySKU, error) {
	rows, err := s.repo.ListStockAnalyticsSnapshotsBySellerAndDate(ctx, sellerAccountID, asOf)
	if err != nil {
		return nil, fmt.Errorf("list stock analytics snapshots: %w", err)
	}
	if len(rows) == 0 {
		return FBOStorageRiskBySKU{}, nil
	}
	products, err := s.repo.ListProductsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list products for fbo storage risk: %w", err)
	}
	productBySKU := make(map[int64]productRef, len(products))
	for _, p := range products {
		if !p.Sku.Valid || p.Sku.Int64 <= 0 {
			continue
		}
		ref := productRef{OzonProductID: p.OzonProductID, Name: p.Name}
		if p.OfferID.Valid {
			offer := p.OfferID.String
			ref.OfferID = &offer
		}
		productBySKU[p.Sku.Int64] = ref
	}
	return BuildFBOStorageRiskBySKU(rows, productBySKU), nil
}

func (s *Service) buildSKUImportanceForWindow(ctx context.Context, sellerAccountID int64, asOf time.Time) (SKUImportanceByProduct, error) {
	dateFrom := asOf.AddDate(0, 0, -(keySKUWindowDays - 1))
	rows, err := s.repo.ListDailySKUMetricsByDateRange(ctx, sellerAccountID, dateFrom, asOf)
	if err != nil {
		return nil, fmt.Errorf("list sku metrics for key sku window: %w", err)
	}
	return BuildSKUImportanceFromDailyMetrics(rows), nil
}

func (s *Service) failRunAndWrap(ctx context.Context, sellerAccountID int64, runID int64, sourceErr error, stage string) error {
	failMsg := sourceErr.Error()
	if _, failErr := s.FailRun(ctx, sellerAccountID, runID, failMsg); failErr != nil {
		return fmt.Errorf("%s: %w (also failed to mark run as failed: %v)", stage, sourceErr, failErr)
	}
	return fmt.Errorf("%s: %w", stage, sourceErr)
}
