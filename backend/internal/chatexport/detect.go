package chatexport

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// DefaultChatDateRangeDays matches chat.DefaultChatDateRangeDays.
const DefaultChatDateRangeDays = 7

var (
	sqlPattern = regexp.MustCompile(`(?i)(?:^|[^\p{L}])(?:select|insert|update|delete|drop|truncate|alter)\s`)
)

// ParseExportRequest detects export intent from a user message without calling OpenAI.
func ParseExportRequest(message string, asOfDate *time.Time, resolveAsOf func() (time.Time, error)) ParsedRequest {
	normalized := normalizeDetectText(message)
	if !detectExportAction(normalized) {
		return ParsedRequest{Reason: ""}
	}
	if detectArbitrarySQL(normalized) {
		return ParsedRequest{
			Reason:  ReasonArbitrarySQL,
			Message: arbitrarySQLMessage,
		}
	}
	format, formatReason := detectFormat(normalized)
	if formatReason == ReasonFormatNotSupported {
		return ParsedRequest{Reason: formatReason, Message: formatNotSupportedMessage}
	}
	if isAmbiguousProductExport(normalized) {
		return ParsedRequest{Reason: ReasonExportClarification, Message: exportClarificationMessage}
	}
	exportType, typeReason := detectExportType(normalized)
	if typeReason == ReasonExportTypeUnsupported {
		return ParsedRequest{Reason: typeReason, Message: exportTypeUnsupportedMessage}
	}
	if typeReason == ReasonExportClarification {
		return ParsedRequest{Reason: typeReason, Message: exportClarificationMessage}
	}
	spec, ok := LookupExportType(exportType)
	if !ok || !spec.Implemented {
		return ParsedRequest{Reason: ReasonExportSourceUnavailable, Message: exportSourceUnavailableMessage}
	}

	dateTo := time.Now().UTC().Truncate(24 * time.Hour)
	if asOfDate != nil {
		dateTo = asOfDate.UTC().Truncate(24 * time.Hour)
	} else if resolveAsOf != nil {
		if resolved, err := resolveAsOf(); err == nil {
			dateTo = resolved
		}
	}
	dateFrom := dateTo.AddDate(0, 0, -(DefaultChatDateRangeDays - 1))
	if days, ok := extractExplicitDayRange(normalized); ok {
		if days > MaxExportDateRange {
			return ParsedRequest{Reason: ReasonDateRangeTooLarge, Message: dateRangeTooLargeMessage}
		}
		dateFrom = dateTo.AddDate(0, 0, -(days - 1))
	}

	return ParsedRequest{
		ExportType: exportType,
		Format:     format,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		Filters:    map[string]any{},
	}
}

// HasExportAction reports whether normalized text (lowercase) requests a file export.
func HasExportAction(normalized string) bool {
	return detectExportAction(normalized)
}

func detectExportAction(normalized string) bool {
	for _, kw := range exportActionKeywords {
		if strings.Contains(normalized, kw) {
			return true
		}
	}
	return false
}

func detectFormat(normalized string) (string, string) {
	for format, keywords := range artifactFormatKeywords {
		for _, kw := range keywords {
			if strings.Contains(normalized, kw) {
				return format, ""
			}
		}
	}
	for _, kw := range csvFormatKeywords {
		if strings.Contains(normalized, kw) {
			return FormatCSV, ""
		}
	}
	for _, kw := range xlsxFormatKeywords {
		if strings.Contains(normalized, kw) {
			return FormatXLSX, ""
		}
	}
	return FormatXLSX, ""
}

func detectExportType(normalized string) (string, string) {
	if matchesAny(normalized, competitorKeywords) {
		return "", ReasonExportTypeUnsupported
	}
	typeScores := map[string]int{}
	for exportType, keywords := range exportTypeKeywords {
		for _, kw := range keywords {
			if strings.Contains(normalized, kw) {
				typeScores[exportType]++
			}
		}
	}
	bestType := ""
	bestScore := 0
	secondScore := 0
	for t, score := range typeScores {
		if score > bestScore {
			secondScore = bestScore
			bestScore = score
			bestType = t
			continue
		}
		if score > secondScore {
			secondScore = score
		}
	}
	if bestScore == 0 {
		return "", ReasonExportClarification
	}
	if bestScore == secondScore && secondScore > 0 {
		if matchesAny(normalized, stockRiskOnlyKeywords) {
			return TypeStockRisks, ""
		}
		if matchesAny(normalized, []string{"излиш", "surplus", "fbo storage", "платное хранение"}) {
			return TypeStockSurplus, ""
		}
		if matchesAny(normalized, []string{"ключевые sku", "critical sku", "топовые товары"}) {
			return TypeCriticalSKU, ""
		}
		if matchesAny(normalized, []string{"продаж", "выручк", "сводк"}) {
			return TypeSalesSummary, ""
		}
		if matchesAny(normalized, performanceKeywords) {
			return TypeCampaignPerformance, ""
		}
		if typeScores[TypeAdvertisingCampaigns] > 0 || typeScores[TypeCampaignPerformance] > 0 {
			return TypeAdvertisingCampaigns, ""
		}
		return "", ReasonExportClarification
	}
	if bestType == TypeAdvertisingCampaigns && matchesAny(normalized, performanceKeywords) {
		return TypeCampaignPerformance, ""
	}
	return bestType, ""
}

func detectArbitrarySQL(normalized string) bool {
	if sqlPattern.MatchString(normalized) {
		return true
	}
	for _, kw := range sqlRefusalKeywords {
		if strings.Contains(normalized, kw) {
			return true
		}
	}
	return false
}

func extractExplicitDayRange(normalized string) (int, bool) {
	re := regexp.MustCompile(`(\d{1,3})\s*(?:дн|дня|дней|day|days)`)
	m := re.FindStringSubmatch(normalized)
	if len(m) < 2 {
		return 0, false
	}
	days, err := strconv.Atoi(m[1])
	if err != nil || days <= 0 {
		return 0, false
	}
	return days, true
}

func normalizeDetectText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "ё", "е")
	return collapseDetectSpaces(s)
}

func collapseDetectSpaces(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}

func matchesAny(normalized string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(normalized, kw) {
			return true
		}
	}
	return false
}

var exportActionKeywords = []string{
	"выгрузи", "экспортируй", "скачай", "скачать",
	"сделай excel", "сделай xlsx", "сделай csv", "сделай pdf", "сделай файл",
	"подготовь файл", "подготовь xlsx", "подготовь csv",
	"таблицу в excel", "таблица csv",
	"выгрузка", "сформируй файл", "дай файл", "экспорт",
}

// isAmbiguousProductExport returns true when the user asks for a file about "products" without a clear domain.
func isAmbiguousProductExport(normalized string) bool {
	if !detectExportAction(normalized) {
		return false
	}
	if !matchesAny(normalized, []string{"товар", "sku", "скю", "проблемн"}) {
		return false
	}
	if matchesAny(normalized, stockRiskOnlyKeywords) {
		return false
	}
	if matchesAny(normalized, []string{
		"ключевые sku", "critical sku", "топовые", "продаж", "выручк", "сводк",
		"реклам", "кампани", "alert", "алерт", "рекомендац", "заказ", "остат", "излиш", "deficit", "дефицит",
	}) {
		return false
	}
	return true
}

var xlsxFormatKeywords = []string{"excel", "xlsx", "эксель", "эксел"}
var csvFormatKeywords = []string{"csv"}
var artifactFormatKeywords = map[string][]string{
	FormatPDF:  {"pdf"},
	FormatDOCX: {"docx", "word"},
	FormatPPTX: {"pptx", "powerpoint", "презентац"},
	FormatTXT:  {"txt", "текстовый файл"},
	FormatMD:   {"markdown", " md"},
	FormatJSON: {"json"},
	FormatPNG:  {"png"},
	FormatJPEG: {"jpeg", "jpg"},
	FormatZIP:  {"zip", "архив"},
}

var competitorKeywords = []string{"конкурент", "competitor"}

var sqlRefusalKeywords = []string{
	"выполни select", "execute select", "sql запрос", "произвольн sql", "arbitrary sql",
}

var performanceKeywords = []string{"drr", "дрр", "roas", "cpo", "ctr", "cpc", "performance", "эффективност"}

var stockRiskOnlyKeywords = []string{
	"риск остат", "риском остат", "скоро закончатся", "дефицит", "stock risk", "days_of_cover", "хватит на дней",
}

var exportTypeKeywords = map[string][]string{
	TypeSalesSummary: {
		"продаж", "выручк", "заказ", "summary", "сводк", "продажи по дням",
	},
	TypeSKUPerformance: {
		"sku", "скю", "товар", "эффективность товаров", "динамика sku", "падение заказов",
		"проблемные товары",
	},
	TypeCriticalSKU: {
		"ключевые sku", "critical sku", "важные товары", "топовые товары", "key sku",
	},
	TypeStockRisks: {
		"остат", "риск остат", "риском остат", "скоро закончатся", "дефицит", "stock risk", "days_of_cover",
		"хватит на дней", "закончатся",
	},
	TypeStockSurplus: {
		"излиш", "платное хранение", "surplus", "fbo storage", "избыточное хранение", "excess",
		"лежит на складе",
	},
	TypeAdvertisingCampaigns: {
		"реклам", "кампани", "дрр", "drr", "roas", "cpo", "ctr", "cpc",
	},
	TypeCampaignPerformance: {
		"реклам", "кампани", "дрр", "drr", "roas", "cpo", "ctr", "cpc", "performance",
	},
	TypeAlerts: {
		"alert", "алерт", "сигнал",
	},
	TypeRecommendations: {
		"рекомендац", "recommendation", "что делать",
	},
	TypeOrdersByStatus: {
		"статус заказ", "заказы по статусам", "orders by status",
	},
}

const (
	formatNotSupportedMessage      = "Этот формат пока не поддерживается."
	exportClarificationMessage     = "Что именно выгрузить: продажи, SKU, остатки, рекламу, alert'ы или рекомендации?"
	exportTypeUnsupportedMessage   = "Этот тип выгрузки пока недоступен."
	exportSourceUnavailableMessage = "Этот экспорт пока недоступен, потому что для него не подключён источник данных."
	arbitrarySQLMessage            = "Я не могу выполнять произвольные SQL-запросы. Могу подготовить разрешённую выгрузку: продажи, SKU, остатки, рекламу, alert'ы или рекомендации."
	dateRangeTooLargeMessage       = "Слишком широкий период для выгрузки. Максимум 90 дней."
)

// SuccessMessage builds the assistant reply for a completed export.
func SuccessMessage(exportType, format string, rowCount int, days int) string {
	typeLabel := exportTypeHumanLabel(exportType)
	formatLabel := strings.ToUpper(format)
	base := "Я подготовил " + formatLabel + "-файл: " + typeLabel + " за последние " + itoa(days) + " дней."
	if rowCount == 0 {
		return base + " За выбранный период данных нет — в файле только заголовки колонок."
	}
	return base
}

func exportTypeHumanLabel(exportType string) string {
	switch exportType {
	case TypeSalesSummary:
		return "сводка продаж"
	case TypeSKUPerformance:
		return "эффективность SKU"
	case TypeCriticalSKU:
		return "ключевые SKU"
	case TypeStockRisks:
		return "товары с риском остатка"
	case TypeStockSurplus:
		return "излишки на складе"
	case TypeAdvertisingCampaigns, TypeCampaignPerformance:
		return "рекламные кампании"
	case TypeAlerts:
		return "alert'ы"
	case TypeRecommendations:
		return "рекомендации"
	case TypeOrdersByStatus:
		return "заказы по статусам"
	default:
		return exportType
	}
}

func itoa(n int) string {
	if n <= 0 {
		return "7"
	}
	return strconv.Itoa(n)
}
