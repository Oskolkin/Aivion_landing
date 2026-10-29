package productanalyticssync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/rawpayloads"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	SourceStatusAvailable          = "available"
	SourceStatusNotEnoughData      = "not_enough_data"
	SourceStatusSourceAccessDenied = "source_access_denied"
	SourceStatusPremiumRequired    = "premium_required"
	SourceStatusError              = "error"
)

// analyticsDataFetcher is implemented by ozon.Client (tests may inject a mock).
type analyticsDataFetcher interface {
	FetchAnalyticsData(ctx context.Context, clientID, apiKey string, req ozon.AnalyticsDataRequest) (*ozon.AnalyticsDataResponse, error)
}

// Service syncs account-level product funnel metrics from Ozon POST /v1/analytics/data.
type Service struct {
	pool        *pgxpool.Pool
	q           *dbgen.Queries
	client      analyticsDataFetcher
	ozon        *ozon.Service
	rawPayloads *rawpayloads.Service
}

func NewService(pool *pgxpool.Pool, ozonSvc *ozon.Service) *Service {
	return NewServiceWithRawPayloads(pool, ozonSvc, nil)
}

func NewServiceWithRawPayloads(pool *pgxpool.Pool, ozonSvc *ozon.Service, rawPayloadsSvc *rawpayloads.Service) *Service {
	return &Service{
		pool:        pool,
		q:           dbgen.New(pool),
		client:      ozon.NewClient(),
		ozon:        ozonSvc,
		rawPayloads: rawPayloadsSvc,
	}
}

// NewServiceWithClient allows injecting a mock Ozon client in tests.
func NewServiceWithClient(pool *pgxpool.Pool, ozonSvc *ozon.Service, client analyticsDataFetcher) *Service {
	return &Service{
		pool:   pool,
		q:      dbgen.New(pool),
		client: client,
		ozon:   ozonSvc,
	}
}

// FunnelSyncOutcome is the normalized result of one account-level funnel sync attempt.
type FunnelSyncOutcome struct {
	SourceStatus  string
	SourceMessage string
	Impressions   *int64
	ProductViews  *int64
	AddToCart     *int64
	OrdersCount   *int64
	Completed     *int64
	Revenue       *float64
	RawBody       []byte
}

// SyncFunnelForSeller loads Ozon credentials and upserts funnel data for metricDate.
// Never fails the caller on 401/403/402 — those are persisted as source_status.
func (s *Service) SyncFunnelForSeller(ctx context.Context, sellerAccountID int64, metricDate time.Time) error {
	if s.ozon == nil {
		return nil
	}
	creds, err := s.ozon.GetDecryptedCredentials(ctx, sellerAccountID)
	if err != nil {
		return nil
	}
	outcome, persistErr := s.syncAccountFunnelForDate(ctx, creds.ClientID, creds.APIKey, metricDate)
	if persistErr != nil {
		return persistErr
	}
	return s.persistOutcome(ctx, sellerAccountID, metricDate, outcome)
}

// SyncFunnelForSellerRange syncs account-level funnel for each day in [fromDate, toDate] inclusive.
// Per-date technical/provider statuses are persisted as source_status without aborting the full range.
func (s *Service) SyncFunnelForSellerRange(ctx context.Context, sellerAccountID int64, fromDate, toDate time.Time) error {
	return syncFunnelDateRange(ctx, fromDate, toDate, func(day time.Time) error {
		return s.SyncFunnelForSeller(ctx, sellerAccountID, day)
	})
}

func syncFunnelDateRange(ctx context.Context, fromDate, toDate time.Time, syncDate func(day time.Time) error) error {
	if toDate.Before(fromDate) {
		return nil
	}
	start := calendarDayUTC(fromDate)
	end := calendarDayUTC(toDate)
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		if err := syncDate(day); err != nil {
			return err
		}
	}
	return nil
}

// SyncAccountFunnelForDate fetches Ozon analytics/data and upserts product_analytics_daily.
func (s *Service) SyncAccountFunnelForDate(ctx context.Context, sellerAccountID int64, clientID, apiKey string, metricDate time.Time) error {
	outcome, err := s.syncAccountFunnelForDate(ctx, clientID, apiKey, metricDate)
	if err != nil {
		return err
	}
	return s.persistOutcome(ctx, sellerAccountID, metricDate, outcome)
}

func (s *Service) syncAccountFunnelForDate(ctx context.Context, clientID, apiKey string, metricDate time.Time) (FunnelSyncOutcome, error) {
	dateStr := metricDate.UTC().Format("2006-01-02")
	req := ozon.AnalyticsDataRequest{
		DateFrom:  dateStr,
		DateTo:    dateStr,
		Metrics:   ozon.AccountFunnelMetrics,
		Dimension: []string{"day"},
		Limit:     1,
		Offset:    0,
	}

	resp, err := s.client.FetchAnalyticsData(ctx, clientID, apiKey, req)
	if err != nil {
		if outcome, ok := outcomeFromProviderError(err); ok {
			return outcome, nil
		}
		return FunnelSyncOutcome{
			SourceStatus:  SourceStatusError,
			SourceMessage: "Не удалось загрузить данные воронки продаж. Повторите синхронизацию позже.",
		}, nil
	}

	impressions, productViews, addToCart, orders, completed, revenue := ozon.MapAccountFunnelTotals(resp.Totals)
	if !hasAnyFunnelMetric(impressions, productViews, addToCart, orders, completed) {
		return FunnelSyncOutcome{
			SourceStatus:  SourceStatusNotEnoughData,
			SourceMessage: "Ozon analytics/data вернул пустые метрики воронки за день.",
			RawBody:       resp.Raw,
		}, nil
	}

	return FunnelSyncOutcome{
		SourceStatus: SourceStatusAvailable,
		Impressions:  impressions,
		ProductViews: productViews,
		AddToCart:    addToCart,
		OrdersCount:  orders,
		Completed:    completed,
		Revenue:      revenue,
		RawBody:      resp.Raw,
	}, nil
}

func outcomeFromProviderError(err error) (FunnelSyncOutcome, bool) {
	if errors.Is(err, ozon.ErrInvalidCredentials) {
		return FunnelSyncOutcome{
			SourceStatus:  SourceStatusSourceAccessDenied,
			SourceMessage: "Данные воронки недоступны для текущего кабинета или тарифа Ozon.",
		}, true
	}
	var pe *ozon.ProviderError
	if errors.As(err, &pe) {
		switch pe.StatusCode {
		case 401, 403:
			return FunnelSyncOutcome{
				SourceStatus:  SourceStatusSourceAccessDenied,
				SourceMessage: "Данные воронки недоступны для текущего кабинета или тарифа Ozon.",
			}, true
		case 402:
			return FunnelSyncOutcome{
				SourceStatus:  SourceStatusPremiumRequired,
				SourceMessage: "Данные воронки доступны на расширенном тарифе Ozon (Premium).",
			}, true
		}
	}
	return FunnelSyncOutcome{}, false
}

func hasAnyFunnelMetric(vals ...*int64) bool {
	for _, v := range vals {
		if v != nil {
			return true
		}
	}
	return false
}

func (s *Service) persistOutcome(ctx context.Context, sellerAccountID int64, metricDate time.Time, outcome FunnelSyncOutcome) error {
	rawPayloadID, err := s.persistRawPayload(ctx, sellerAccountID, metricDate, outcome.RawBody)
	if err != nil {
		return err
	}
	_, err = s.q.UpsertProductAnalyticsDaily(ctx, dbgen.UpsertProductAnalyticsDailyParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgtype.Date{Time: metricDate.UTC().Truncate(24 * time.Hour), Valid: true},
		ScopeType:       "account",
		ScopeID:         "",
		Impressions:     int8FromPtr(outcome.Impressions),
		ProductViews:    int8FromPtr(outcome.ProductViews),
		AddToCart:       int8FromPtr(outcome.AddToCart),
		OrdersCount:     int8FromPtr(outcome.OrdersCount),
		CompletedOrders: int8FromPtr(outcome.Completed),
		Revenue:         numericFromFloat(outcome.Revenue),
		RawPayloadID:    rawPayloadID,
		SourceStatus:    textFromString(outcome.SourceStatus),
		SourceMessage:   textFromString(outcome.SourceMessage),
	})
	return err
}

func (s *Service) persistRawPayload(ctx context.Context, sellerAccountID int64, metricDate time.Time, body []byte) (pgtype.Int8, error) {
	if s.rawPayloads == nil || len(body) == 0 {
		return pgtype.Int8{Valid: false}, nil
	}
	rawRow, err := s.rawPayloads.Save(ctx, rawpayloads.SaveInput{
		SellerAccountID: sellerAccountID,
		ImportJobID:     0,
		Domain:          "product_analytics_daily",
		Source:          "ozon.v1.analytics.data",
		RequestKey:      fmt.Sprintf("seller:%d:date:%s:scope:account", sellerAccountID, calendarDayUTC(metricDate).Format("2006-01-02")),
		Body:            body,
	})
	if err != nil {
		return pgtype.Int8{}, fmt.Errorf("save raw analytics/data payload: %w", err)
	}
	return pgtype.Int8{Int64: rawRow.ID, Valid: true}, nil
}

func calendarDayUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func int8FromPtr(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func numericFromFloat(v *float64) pgtype.Numeric {
	if v == nil {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.2f", *v))
	return n
}

func textFromString(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

// MapProviderErrorToWidgetStatus maps Ozon errors to dashboard widget statuses.
func MapProviderErrorToWidgetStatus(err error) (status string, message string) {
	if outcome, ok := outcomeFromProviderError(err); ok {
		return outcome.SourceStatus, outcome.SourceMessage
	}
	return SourceStatusError, "Не удалось загрузить данные воронки."
}
