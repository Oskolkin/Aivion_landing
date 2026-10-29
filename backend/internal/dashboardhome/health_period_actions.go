package dashboardhome

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/healthstatus"
)

// applyHealthStatusFromScore overwrites AI severity with deterministic bands
// whenever a numeric health_score is present.
func applyHealthStatusFromScore(resp *Response) {
	if resp == nil {
		return
	}
	acc := &resp.AIDiagnostics.AccountSummary
	st := healthstatus.FromScore(acc.HealthScore)
	if st.Code == "" {
		return
	}
	acc.Severity = st.Severity
	acc.HealthStatusCode = st.Code
	acc.HealthStatusLabel = st.Label
	acc.HealthStatusExplain = st.Explanation
}

// applyPeriodContextFromMetricDate fills structured period metadata for home UI.
func applyPeriodContextFromMetricDate(resp *Response) {
	if resp == nil {
		return
	}
	metricDate := strings.TrimSpace(resp.MetricDate)
	if metricDate == "" {
		metricDate = strings.TrimSpace(resp.EffectiveMetricDate)
	}
	day, err := time.Parse("2006-01-02", metricDate)
	if err != nil {
		return
	}
	prevDay := day.AddDate(0, 0, -1)
	weekStart := day.AddDate(0, 0, -6)
	prevWeekEnd := weekStart.AddDate(0, 0, -1)
	prevWeekStart := prevWeekEnd.AddDate(0, 0, -6)
	ruDay := formatRUDayMonth(day)
	ruPrev := formatRUDayMonth(prevDay)
	ruWeek := formatRUDateRange(weekStart, day)
	ruPrevWeek := formatRUDateRange(prevWeekStart, prevWeekEnd)
	captionPrimary := fmt.Sprintf("Карточки: %s | Изменение: к %s | Графики: %s", ruDay, ruPrev, ruWeek)
	captionSecondary := fmt.Sprintf("AI-анализ: %s | Сравнение: %s", ruWeek, ruPrevWeek)
	resp.PeriodContext = &PeriodContext{
		ValuePeriod:      metricDate,
		ComparisonPeriod: prevDay.Format("2006-01-02"),
		TrendPeriod:      "7d",
		CurrentRange:     fmt.Sprintf("%s/%s", weekStart.Format("2006-01-02"), day.Format("2006-01-02")),
		PreviousRange:    fmt.Sprintf("%s/%s", prevWeekStart.Format("2006-01-02"), prevWeekEnd.Format("2006-01-02")),
		CaptionPrimary:   captionPrimary,
		CaptionSecondary: captionSecondary,
		// Keep caption as two-line join for older clients; UI prefers primary/secondary.
		Caption: captionPrimary + "\n" + captionSecondary,
	}
}

func formatRUDayMonth(t time.Time) string {
	months := []string{
		"", "января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}
	return fmt.Sprintf("%d %s", t.Day(), months[int(t.Month())])
}

// formatRUDateRange renders compact ranges: "2–8 июля" or "25 июня–1 июля".
func formatRUDateRange(start, end time.Time) string {
	months := []string{
		"", "января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря",
	}
	if start.Month() == end.Month() && start.Year() == end.Year() {
		return fmt.Sprintf("%d–%d %s", start.Day(), end.Day(), months[int(end.Month())])
	}
	return fmt.Sprintf("%d %s–%d %s", start.Day(), months[int(start.Month())], end.Day(), months[int(end.Month())])
}

// actionDetailHref builds the canonical Actions detail route for a top action.
func actionDetailHref(actionKey string, actionID int64) string {
	key := strings.TrimSpace(actionKey)
	if key == "" && actionID > 0 {
		key = fmt.Sprintf("%d", actionID)
	}
	if key == "" {
		return "/app/actions"
	}
	return "/app/actions?" + url.Values{"action": {key}}.Encode()
}

func isLegacySellerPulseHref(href string) bool {
	href = strings.TrimSpace(href)
	switch href {
	case "/app/advertising", "/app/products", "/app/stocks-replenishment",
		"/app/critical-skus", "/app/recommendations":
		return true
	default:
		return strings.HasPrefix(href, "/app/products?") ||
			strings.HasPrefix(href, "/app/advertising?")
	}
}
