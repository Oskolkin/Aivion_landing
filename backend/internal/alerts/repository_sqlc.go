package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type Repository interface {
	UpsertAlert(ctx context.Context, input UpsertAlertInput) (Alert, error)
	GetAlertByID(ctx context.Context, sellerAccountID int64, alertID int64) (Alert, error)
	ListOpenAlerts(ctx context.Context, sellerAccountID int64, limit int, offset int) ([]Alert, error)
	ResolveAlert(ctx context.Context, sellerAccountID int64, alertID int64) (Alert, error)
	DismissAlert(ctx context.Context, sellerAccountID int64, alertID int64) (Alert, error)

	CreateRun(ctx context.Context, sellerAccountID int64, runType RunType) (AlertRun, error)
	CompleteRun(ctx context.Context, input CompleteRunInput) (AlertRun, error)
	FailRun(ctx context.Context, sellerAccountID int64, runID int64, errorMessage string) (AlertRun, error)
	GetLatestRun(ctx context.Context, sellerAccountID int64) (AlertRun, error)
	GetDailyAccountMetricByDate(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*AccountDailyMetric, error)
	ListDailySKUMetricsByDate(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]SKUDailyMetric, error)
	ListDailySKUMetricsByDateRange(ctx context.Context, sellerAccountID int64, dateFrom time.Time, dateTo time.Time) ([]SKUDailyMetric, error)
	ListAlertsFiltered(ctx context.Context, sellerAccountID int64, filter ListFilter) ([]Alert, error)
	CountOpenAlertsBySeverity(ctx context.Context, sellerAccountID int64) ([]SeverityCount, error)
	CountOpenAlertsByGroup(ctx context.Context, sellerAccountID int64) ([]GroupCount, error)
	CountOpenAlerts(ctx context.Context, sellerAccountID int64) (int64, error)
	ListAdCampaignSummariesByDateRange(ctx context.Context, sellerAccountID int64, dateFrom time.Time, dateTo time.Time) ([]AdCampaignMetricSummary, error)
	ListAdCampaignSKUMappings(ctx context.Context, sellerAccountID int64) ([]AdCampaignSKUMapping, error)
	ListProductsBySellerAccountID(ctx context.Context, sellerAccountID int64) ([]dbgen.Product, error)
	ListSKUEffectiveConstraintsBySellerAccountID(ctx context.Context, sellerAccountID int64) ([]dbgen.SkuEffectiveConstraint, error)
	ListStockAnalyticsSnapshotsBySellerAndDate(ctx context.Context, sellerAccountID int64, snapshotDate time.Time) ([]dbgen.StockAnalyticsSnapshot, error)
}

type SQLCRepository struct {
	queries *dbgen.Queries
}

func NewSQLCRepository(queries *dbgen.Queries) *SQLCRepository {
	return &SQLCRepository{queries: queries}
}

func (r *SQLCRepository) UpsertAlert(ctx context.Context, input UpsertAlertInput) (Alert, error) {
	payload, err := json.Marshal(normalizeEvidence(input.EvidencePayload))
	if err != nil {
		return Alert{}, fmt.Errorf("marshal alert evidence payload: %w", err)
	}
	row, err := r.queries.UpsertAlert(ctx, dbgen.UpsertAlertParams{
		SellerAccountID: input.SellerAccountID,
		AlertType:       string(input.AlertType),
		AlertGroup:      string(input.AlertGroup),
		EntityType:      string(input.EntityType),
		EntityID:        nullableText(input.EntityID),
		EntitySku:       nullableInt64(input.EntitySKU),
		EntityOfferID:   nullableText(input.EntityOfferID),
		Title:           input.Title,
		Message:         input.Message,
		Severity:        string(input.Severity),
		Urgency:         string(input.Urgency),
		EvidencePayload: payload,
		Fingerprint:     input.Fingerprint,
	})
	if err != nil {
		return Alert{}, fmt.Errorf("upsert alert: %w", err)
	}
	return mapAlert(row)
}

func (r *SQLCRepository) GetAlertByID(ctx context.Context, sellerAccountID int64, alertID int64) (Alert, error) {
	row, err := r.queries.GetAlertByID(ctx, dbgen.GetAlertByIDParams{
		ID:              alertID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		return Alert{}, fmt.Errorf("get alert by id=%d: %w", alertID, err)
	}
	return mapAlert(row)
}

func (r *SQLCRepository) ListOpenAlerts(ctx context.Context, sellerAccountID int64, limit int, offset int) ([]Alert, error) {
	rows, err := r.queries.ListOpenAlertsBySellerAccountID(ctx, dbgen.ListOpenAlertsBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           normalizeLimit(limit),
		Offset:          normalizeOffset(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list open alerts: %w", err)
	}
	out := make([]Alert, 0, len(rows))
	for _, row := range rows {
		alert, mapErr := mapAlert(row)
		if mapErr != nil {
			return nil, mapErr
		}
		out = append(out, alert)
	}
	return out, nil
}

func (r *SQLCRepository) ResolveAlert(ctx context.Context, sellerAccountID int64, alertID int64) (Alert, error) {
	row, err := r.queries.ResolveAlert(ctx, dbgen.ResolveAlertParams{
		ID:              alertID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		return Alert{}, fmt.Errorf("resolve alert id=%d: %w", alertID, err)
	}
	return mapAlert(row)
}

func (r *SQLCRepository) DismissAlert(ctx context.Context, sellerAccountID int64, alertID int64) (Alert, error) {
	row, err := r.queries.DismissAlert(ctx, dbgen.DismissAlertParams{
		ID:              alertID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		return Alert{}, fmt.Errorf("dismiss alert id=%d: %w", alertID, err)
	}
	return mapAlert(row)
}

func (r *SQLCRepository) CreateRun(ctx context.Context, sellerAccountID int64, runType RunType) (AlertRun, error) {
	row, err := r.queries.CreateAlertRun(ctx, dbgen.CreateAlertRunParams{
		SellerAccountID: sellerAccountID,
		RunType:         string(runType),
	})
	if err != nil {
		return AlertRun{}, fmt.Errorf("create alert run: %w", err)
	}
	return mapAlertRun(row), nil
}

func (r *SQLCRepository) CompleteRun(ctx context.Context, input CompleteRunInput) (AlertRun, error) {
	row, err := r.queries.CompleteAlertRun(ctx, dbgen.CompleteAlertRunParams{
		ID:               input.RunID,
		SellerAccountID:  input.SellerAccountID,
		SalesAlertsCount: input.SalesAlertsCount,
		StockAlertsCount: input.StockAlertsCount,
		AdAlertsCount:    input.AdAlertsCount,
		PriceAlertsCount: input.PriceAlertsCount,
		TotalAlertsCount: input.TotalAlertsCount,
	})
	if err != nil {
		return AlertRun{}, fmt.Errorf("complete alert run id=%d: %w", input.RunID, err)
	}
	return mapAlertRun(row), nil
}

func (r *SQLCRepository) FailRun(ctx context.Context, sellerAccountID int64, runID int64, errorMessage string) (AlertRun, error) {
	row, err := r.queries.FailAlertRun(ctx, dbgen.FailAlertRunParams{
		ID:              runID,
		SellerAccountID: sellerAccountID,
		ErrorMessage:    pgtype.Text{String: errorMessage, Valid: errorMessage != ""},
	})
	if err != nil {
		return AlertRun{}, fmt.Errorf("fail alert run id=%d: %w", runID, err)
	}
	return mapAlertRun(row), nil
}

func (r *SQLCRepository) GetLatestRun(ctx context.Context, sellerAccountID int64) (AlertRun, error) {
	row, err := r.queries.GetLatestAlertRunBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return AlertRun{}, fmt.Errorf("get latest alert run by seller account id=%d: %w", sellerAccountID, err)
	}
	return mapAlertRun(row), nil
}

func (r *SQLCRepository) GetDailyAccountMetricByDate(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*AccountDailyMetric, error) {
	date := pgDate(metricDate)
	rows, err := r.queries.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      date,
		MetricDate_2:    date,
	})
	if err != nil {
		return nil, fmt.Errorf("list daily account metrics by date=%s: %w", metricDate.Format("2006-01-02"), err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	return &AccountDailyMetric{
		SellerAccountID: row.SellerAccountID,
		MetricDate:      dateToTime(row.MetricDate),
		Revenue:         numericFloat64(row.Revenue),
		OrdersCount:     row.OrdersCount,
	}, nil
}

func (r *SQLCRepository) ListDailySKUMetricsByDate(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]SKUDailyMetric, error) {
	return r.ListDailySKUMetricsByDateRange(ctx, sellerAccountID, metricDate, metricDate)
}

func (r *SQLCRepository) ListDailySKUMetricsByDateRange(ctx context.Context, sellerAccountID int64, dateFrom time.Time, dateTo time.Time) ([]SKUDailyMetric, error) {
	rows, err := r.queries.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(dateFrom),
		MetricDate_2:    pgDate(dateTo),
	})
	if err != nil {
		return nil, fmt.Errorf("list daily sku metrics range %s..%s: %w",
			dateFrom.Format("2006-01-02"), dateTo.Format("2006-01-02"), err)
	}
	out := make([]SKUDailyMetric, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapDailySKUMetricRow(row))
	}
	return out, nil
}

func mapDailySKUMetricRow(row dbgen.DailySkuMetric) SKUDailyMetric {
	return SKUDailyMetric{
		SellerAccountID: row.SellerAccountID,
		MetricDate:      dateToTime(row.MetricDate),
		OzonProductID:   row.OzonProductID,
		SKU:             int8Ptr(row.Sku),
		OfferID:         textPtr(row.OfferID),
		ProductName:     textPtr(row.ProductName),
		CurrentStock:    row.StockAvailable,
		DaysOfCover:     numericFloat64Ptr(row.DaysOfCover),
		Revenue:         numericFloat64(row.Revenue),
		OrdersCount:     row.OrdersCount,
	}
}

func (r *SQLCRepository) ListAlertsFiltered(ctx context.Context, sellerAccountID int64, filter ListFilter) ([]Alert, error) {
	limit := normalizeLimit(filter.Limit)
	offset := normalizeOffset(filter.Offset)
	rows, err := r.queries.ListAlertsFiltered(ctx, dbgen.ListAlertsFilteredParams{
		SellerAccountID: sellerAccountID,
		Limit:           limit,
		Offset:          offset,
		Status:          nullableAlertStatus(filter.Status),
		AlertGroup:      nullableAlertGroup(filter.Group),
		Severity:        nullableSeverity(filter.Severity),
		EntityType:      nullableEntityType(filter.EntityType),
	})
	if err != nil {
		return nil, fmt.Errorf("list alerts filtered: %w", err)
	}
	out := make([]Alert, 0, len(rows))
	for _, row := range rows {
		item, mapErr := mapAlert(row)
		if mapErr != nil {
			return nil, mapErr
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *SQLCRepository) CountOpenAlertsBySeverity(ctx context.Context, sellerAccountID int64) ([]SeverityCount, error) {
	rows, err := r.queries.CountOpenAlertsBySeverity(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("count open alerts by severity: %w", err)
	}
	out := make([]SeverityCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, SeverityCount{
			Severity: Severity(row.Severity),
			Count:    row.AlertsCount,
		})
	}
	return out, nil
}

func (r *SQLCRepository) CountOpenAlertsByGroup(ctx context.Context, sellerAccountID int64) ([]GroupCount, error) {
	rows, err := r.queries.CountOpenAlertsByGroup(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("count open alerts by group: %w", err)
	}
	out := make([]GroupCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, GroupCount{
			Group: AlertGroup(row.AlertGroup),
			Count: row.AlertsCount,
		})
	}
	return out, nil
}

func (r *SQLCRepository) CountOpenAlerts(ctx context.Context, sellerAccountID int64) (int64, error) {
	count, err := r.queries.CountOpenAlertsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return 0, fmt.Errorf("count open alerts: %w", err)
	}
	return count, nil
}

func (r *SQLCRepository) ListAdCampaignSummariesByDateRange(ctx context.Context, sellerAccountID int64, dateFrom time.Time, dateTo time.Time) ([]AdCampaignMetricSummary, error) {
	rows, err := r.queries.ListAdCampaignSummariesBySellerAndDateRange(ctx, dbgen.ListAdCampaignSummariesBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        pgDate(dateFrom),
		DateTo:          pgDate(dateTo),
	})
	if err != nil {
		return nil, fmt.Errorf("list ad campaign summaries by date range: %w", err)
	}
	out := make([]AdCampaignMetricSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdCampaignMetricSummary{
			SellerAccountID:    row.SellerAccountID,
			CampaignExternalID: row.CampaignExternalID,
			CampaignName:       row.CampaignName,
			CampaignType:       textPtr(row.CampaignType),
			Impressions:        row.ImpressionsTotal,
			Clicks:             row.ClicksTotal,
			Spend:              numericFloat64(row.SpendTotal),
			Orders:             row.OrdersTotal,
			Revenue:            numericFloat64(row.RevenueTotal),
		})
	}
	return out, nil
}

func (r *SQLCRepository) ListAdCampaignSKUMappings(ctx context.Context, sellerAccountID int64) ([]AdCampaignSKUMapping, error) {
	rows, err := r.queries.ListAdCampaignSKUMappingsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list ad campaign sku mappings: %w", err)
	}
	out := make([]AdCampaignSKUMapping, 0, len(rows))
	for _, row := range rows {
		out = append(out, AdCampaignSKUMapping{
			SellerAccountID:    row.SellerAccountID,
			CampaignExternalID: row.CampaignExternalID,
			CampaignName:       textPtr(row.CampaignName),
			OzonProductID:      row.OzonProductID,
			OfferID:            textPtr(row.OfferID),
			SKU:                int8Ptr(row.Sku),
			IsActive:           row.IsActive,
		})
	}
	return out, nil
}

func (r *SQLCRepository) ListProductsBySellerAccountID(ctx context.Context, sellerAccountID int64) ([]dbgen.Product, error) {
	rows, err := r.queries.ListProductsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list products by seller account: %w", err)
	}
	return rows, nil
}

func (r *SQLCRepository) ListStockAnalyticsSnapshotsBySellerAndDate(
	ctx context.Context,
	sellerAccountID int64,
	snapshotDate time.Time,
) ([]dbgen.StockAnalyticsSnapshot, error) {
	rows, err := r.queries.ListStockAnalyticsSnapshotsBySellerAndDate(ctx, dbgen.ListStockAnalyticsSnapshotsBySellerAndDateParams{
		SellerAccountID: sellerAccountID,
		SnapshotDate:    pgDate(snapshotDate),
	})
	if err != nil {
		return nil, fmt.Errorf("list stock analytics snapshots date=%s: %w", snapshotDate.Format("2006-01-02"), err)
	}
	return rows, nil
}

func (r *SQLCRepository) ListSKUEffectiveConstraintsBySellerAccountID(ctx context.Context, sellerAccountID int64) ([]dbgen.SkuEffectiveConstraint, error) {
	rows, err := r.queries.ListSKUEffectiveConstraintsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list sku effective constraints by seller account: %w", err)
	}
	return rows, nil
}

func mapAlert(row dbgen.Alert) (Alert, error) {
	payload := EvidencePayload{}
	if len(row.EvidencePayload) > 0 {
		if err := json.Unmarshal(row.EvidencePayload, &payload); err != nil {
			return Alert{}, fmt.Errorf("unmarshal alert evidence payload: %w", err)
		}
	}
	return Alert{
		ID:              row.ID,
		SellerAccountID: row.SellerAccountID,
		AlertType:       AlertType(row.AlertType),
		AlertGroup:      AlertGroup(row.AlertGroup),
		EntityType:      EntityType(row.EntityType),
		EntityID:        textPtr(row.EntityID),
		EntitySKU:       int8Ptr(row.EntitySku),
		EntityOfferID:   textPtr(row.EntityOfferID),
		Title:           row.Title,
		Message:         row.Message,
		Severity:        Severity(row.Severity),
		Urgency:         Urgency(row.Urgency),
		Status:          AlertStatus(row.Status),
		EvidencePayload: payload,
		Fingerprint:     row.Fingerprint,
		FirstSeenAt:     timestamptz(row.FirstSeenAt),
		LastSeenAt:      timestamptz(row.LastSeenAt),
		ResolvedAt:      timestamptzPtr(row.ResolvedAt),
		CreatedAt:       timestamptz(row.CreatedAt),
		UpdatedAt:       timestamptz(row.UpdatedAt),
	}, nil
}

func mapAlertRun(row dbgen.AlertRun) AlertRun {
	return AlertRun{
		ID:               row.ID,
		SellerAccountID:  row.SellerAccountID,
		RunType:          RunType(row.RunType),
		Status:           RunStatus(row.Status),
		StartedAt:        timestamptz(row.StartedAt),
		FinishedAt:       timestamptzPtr(row.FinishedAt),
		SalesAlertsCount: row.SalesAlertsCount,
		StockAlertsCount: row.StockAlertsCount,
		AdAlertsCount:    row.AdAlertsCount,
		PriceAlertsCount: row.PriceAlertsCount,
		TotalAlertsCount: row.TotalAlertsCount,
		ErrorMessage:     textPtr(row.ErrorMessage),
		CreatedAt:        timestamptz(row.CreatedAt),
	}
}

func nullableText(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func nullableInt64(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func int8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	i := v.Int64
	return &i
}

func timestamptz(v pgtype.Timestamptz) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time.UTC()
}

func timestamptzPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

func normalizeLimit(limit int) int32 {
	if limit <= 0 {
		return 20
	}
	if limit > 200 {
		return 200
	}
	return int32(limit)
}

func normalizeOffset(offset int) int32 {
	if offset < 0 {
		return 0
	}
	return int32(offset)
}

func pgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t.UTC(), Valid: true}
}

func dateToTime(v pgtype.Date) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time.UTC()
}

func numericFloat64(v pgtype.Numeric) float64 {
	if !v.Valid {
		return 0
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func numericFloat64Ptr(v pgtype.Numeric) *float64 {
	if !v.Valid {
		return nil
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	out := f.Float64
	return &out
}

func nullableAlertStatus(v *AlertStatus) pgtype.Text {
	if v == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: string(*v), Valid: true}
}

func nullableAlertGroup(v *AlertGroup) pgtype.Text {
	if v == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: string(*v), Valid: true}
}

func nullableSeverity(v *Severity) pgtype.Text {
	if v == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: string(*v), Valid: true}
}

func nullableEntityType(v *EntityType) pgtype.Text {
	if v == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: string(*v), Valid: true}
}
