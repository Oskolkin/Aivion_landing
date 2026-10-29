package ozonenrichmentsync

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon/performance"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	scopeP0                   = "p0"
	sourceSellerAPI           = "seller_api"
	sourcePerformanceAPI      = "performance_api"
	statusAvailable           = "available"
	statusError               = "error"
	statusUnauthorized        = "unauthorized"
	statusForbidden           = "forbidden"
	statusSkipped             = "skipped"
	runStatusCompleted        = "completed"
	runStatusCompletedPartial = "completed_partial"
	runStatusFailed           = "failed"

	productInfoChunkSize     = 1000
	relatedSKUChunkSize      = 200
	analyticsStocksChunkSize = 100
	campaignPageSize         = 200
	campaignProductsPageSize = 500
	statsLookbackDays        = 13
)

type SyncInput struct {
	SellerAccountID int64
	MetricDate      time.Time
	Scope           string
	Force           bool
}

type SyncResult struct {
	RunID                   int64
	Status                  string
	ProductInfoCount        int32
	RelatedSKUCount         int32
	StockAnalyticsCount     int32
	CampaignCount           int32
	CampaignDailyStatsCount int32
	CampaignObjectsCount    int32
	CampaignProductsCount   int32
	SellerAPIAvailable      bool
	PerformanceAPIAvailable bool
	Errors                  []string
	Warnings                []string
}

type diagnosticEntry struct {
	Source  string `json:"source"`
	Message string `json:"message"`
}

func normalizeScope(scope string) string {
	scope = strings.TrimSpace(strings.ToLower(scope))
	if scope == "" {
		return scopeP0
	}
	return scope
}

func dedupeMethods(roles ozon.RolesResponse) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, role := range roles.Roles {
		for _, method := range role.Methods {
			method = strings.TrimSpace(method)
			if method == "" {
				continue
			}
			if _, ok := seen[method]; ok {
				continue
			}
			seen[method] = struct{}{}
			out = append(out, method)
		}
	}
	return out
}

func classifySellerAPIStatus(err error) (status string, code string, message string) {
	if err == nil {
		return statusAvailable, "", ""
	}
	if errors.Is(err, ozon.ErrInvalidCredentials) {
		return statusUnauthorized, "unauthorized", sanitizeErrorMessage(err.Error())
	}
	var providerErr *ozon.ProviderError
	if errors.As(err, &providerErr) {
		switch providerErr.StatusCode {
		case http.StatusUnauthorized:
			return statusUnauthorized, "unauthorized", sanitizeErrorMessage(providerErr.Error())
		case http.StatusForbidden:
			return statusForbidden, "forbidden", sanitizeErrorMessage(providerErr.Error())
		default:
			return statusError, strconv.Itoa(providerErr.StatusCode), sanitizeErrorMessage(providerErr.Error())
		}
	}
	return statusError, "error", sanitizeErrorMessage(err.Error())
}

func sanitizeErrorMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	replacers := []string{"Api-Key", "api-key", "Client-Id", "client-id", "Bearer", "bearer", "token"}
	for _, needle := range replacers {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(needle)) {
			return "provider request failed"
		}
	}
	if len(msg) > 500 {
		return msg[:500]
	}
	return msg
}

func parseDecimalString(v string) pgtype.Numeric {
	v = strings.TrimSpace(v)
	if v == "" {
		return pgtype.Numeric{Valid: false}
	}
	v = strings.ReplaceAll(v, " ", "")
	v = strings.ReplaceAll(v, ",", ".")
	var n pgtype.Numeric
	if err := n.Scan(v); err != nil {
		return pgtype.Numeric{Valid: false}
	}
	return n
}

func parseDecimalIndex(v string) pgtype.Numeric {
	return parseDecimalString(v)
}

func nullableText(v string) pgtype.Text {
	if strings.TrimSpace(v) == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: strings.TrimSpace(v), Valid: true}
}

func nullableInt64(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: v, Valid: true}
}

func nullableInt32(v int32) pgtype.Int4 {
	if v == 0 {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: v, Valid: true}
}

func nullableBool(v bool) pgtype.Bool {
	return pgtype.Bool{Bool: v, Valid: true}
}

func nullableBoolPtr(v bool, set bool) pgtype.Bool {
	if !set {
		return pgtype.Bool{Valid: false}
	}
	return pgtype.Bool{Bool: v, Valid: true}
}

func nullableNumericString(v string) pgtype.Numeric {
	return parseDecimalString(v)
}

func nullableNumericFromFloat(v float64) pgtype.Numeric {
	if v == 0 {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(v, 'f', -1, 64))
	return n
}

func nullableInt32FromFloat(v float64) pgtype.Int4 {
	if v == 0 {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: int32(v), Valid: true}
}

func pgDate(v time.Time) pgtype.Date {
	normalized := time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
	return pgtype.Date{Time: normalized, Valid: true}
}

func pgTimestamptz(v time.Time) pgtype.Timestamptz {
	if v.IsZero() {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: v.UTC(), Valid: true}
}

func parseOzonTimestamp(v string) pgtype.Timestamptz {
	v = strings.TrimSpace(v)
	if v == "" {
		return pgtype.Timestamptz{Valid: false}
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, v); err == nil {
			return pgTimestamptz(t)
		}
	}
	return pgtype.Timestamptz{Valid: false}
}

func mustJSON(v any) []byte {
	if v == nil {
		return []byte("null")
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return raw
}

func primaryImageURL(images ozon.FlexibleStringList) string {
	if len(images) == 0 {
		return ""
	}
	return strings.TrimSpace(images[0])
}

func campaignName(c performance.Campaign) string {
	if strings.TrimSpace(c.CampaignName) != "" {
		return c.CampaignName
	}
	return "campaign_" + strconv.FormatInt(c.CampaignExternalID, 10)
}

func resolveProductLinkID(sku int64, productsBySKU map[int64]int64) int64 {
	if id, ok := productsBySKU[sku]; ok && id > 0 {
		return id
	}
	return sku
}

func statsDateRange(metricDate time.Time) (time.Time, time.Time) {
	end := time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, 0, -statsLookbackDays)
	return start, end
}

func finalizeRunStatus(errorsCount int, warningsCount int, importedAny bool) string {
	if errorsCount > 0 {
		if importedAny {
			return runStatusCompletedPartial
		}
		return runStatusFailed
	}
	if warningsCount > 0 {
		return runStatusCompletedPartial
	}
	return runStatusCompleted
}
