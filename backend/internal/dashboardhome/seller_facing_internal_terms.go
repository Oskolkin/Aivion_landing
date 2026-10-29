package dashboardhome

import (
	"regexp"
	"strings"
)

var (
	internalMetricNamePatterns = []struct {
		pattern *regexp.Regexp
		replace func(string) string
	}{
		{
			pattern: regexp.MustCompile(`(?i)\bad_efficiency_drr\b`),
			replace: func(match string) string {
				return "ДРР рекламы"
			},
		},
		{
			pattern: regexp.MustCompile(`(?i)\bdaily_account_orders_count\b`),
			replace: func(string) string { return "количество заказов" },
		},
		{
			pattern: regexp.MustCompile(`(?i)\bdaily_account_metrics\b`),
			replace: func(string) string { return "данные продаж" },
		},
		{
			pattern: regexp.MustCompile(`(?i)\bperiod[_\s-]?comparison\b`),
			replace: func(string) string { return "сравнение с предыдущим периодом" },
		},
		{
			pattern: regexp.MustCompile(`(?i)\baccount_drr\b`),
			replace: func(string) string { return "ДРР в общей выручке" },
		},
		{
			pattern: regexp.MustCompile(`(?i)\bcampaign_drr\b`),
			replace: func(string) string { return "ДРР кампании" },
		},
	}
	internalMetricLeakPattern = regexp.MustCompile(`(?i)\b(ad_efficiency_drr|daily_account_orders_count|daily_account_metrics|period[_\s-]?comparison|account_drr|campaign_drr)\b`)
)

func replaceInternalMetricNamesInText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	for _, rule := range internalMetricNamePatterns {
		text = rule.pattern.ReplaceAllStringFunc(text, rule.replace)
	}
	// Campaign context: "ДРР рекламы" near campaign wording → ДРР кампании
	lower := strings.ToLower(text)
	if strings.Contains(lower, "кампани") && strings.Contains(lower, "дрр рекламы") {
		re := regexp.MustCompile(`(?i)ДРР рекламы`)
		text = re.ReplaceAllString(text, "ДРР кампании")
	}
	return text
}

func containsInternalMetricName(text string) bool {
	return internalMetricLeakPattern.MatchString(strings.TrimSpace(text))
}

func sanitizeSellerFacingInternalTerms(text string) string {
	return replaceInternalMetricNamesInText(cleanupSellerFacingText(text))
}
