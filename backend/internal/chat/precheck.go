package chat

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatexport"
)

// PrecheckRoute classifies a user question before OpenAI is called.
type PrecheckRoute string

const (
	PrecheckRouteSupportedChat          PrecheckRoute = "supported_chat"
	PrecheckRouteExportRequestCandidate PrecheckRoute = "export_request_candidate"
	PrecheckRouteExportExecute          PrecheckRoute = "export_execute"
	PrecheckRouteUnsupported            PrecheckRoute = "unsupported"
	PrecheckRouteClarificationNeeded    PrecheckRoute = "clarification_needed"
)

// Precheck unsupported / controlled reason codes stored in traces.
const (
	PrecheckReasonUnsupportedTopic        = "unsupported_topic"
	PrecheckReasonUnknownSKU              = "unknown_sku"
	PrecheckReasonUnknownOfferID          = "unknown_offer_id"
	PrecheckReasonUnknownCampaign         = "unknown_campaign"
	PrecheckReasonExportNotImplemented    = "export_not_implemented"
	PrecheckReasonFormatNotSupported      = "format_not_supported"
	PrecheckReasonExportClarification     = "clarification_needed"
	PrecheckReasonExportTypeUnsupported   = "export_type_not_supported"
	PrecheckReasonExportSourceUnavailable = "unsupported_by_source"
	PrecheckReasonArbitrarySQL            = "arbitrary_sql_refused"
	PrecheckReasonDateRangeTooLarge       = "date_range_too_large"
	PrecheckReasonClarificationNeeded     = "clarification_needed"
)

// EntityResolver validates seller-specific identifiers referenced in chat questions.
type EntityResolver interface {
	ProductExistsBySKU(ctx context.Context, sellerAccountID int64, sku int64) (bool, error)
	ProductExistsByOfferID(ctx context.Context, sellerAccountID int64, offerID string) (bool, error)
	AdCampaignExistsByExternalID(ctx context.Context, sellerAccountID int64, campaignExternalID int64) (bool, error)
}

// PrecheckDecision is the outcome of the lightweight pre-check.
type PrecheckDecision struct {
	Route              PrecheckRoute
	UnsupportedReason  string
	Message            string
	SuggestedRephrases []string
	OpenAICalled       bool
	IsSupported        bool
	DetectedKeywords   []string
	DetectedEntities   map[string]string
	ExportRequest      *chatexport.ParsedRequest
}

// AsOfDateResolver resolves the default analytics as-of date for exports.
type AsOfDateResolver interface {
	ResolveAsOfDate(ctx context.Context, sellerAccountID int64, requested *time.Time) (time.Time, error)
}

// Prechecker performs a conservative gate before OpenAI calls.
type Prechecker struct {
	entities  EntityResolver
	asOfDates AsOfDateResolver
}

func NewPrechecker(entities EntityResolver) *Prechecker {
	return &Prechecker{entities: entities}
}

func NewPrecheckerWithAsOf(entities EntityResolver, asOf AsOfDateResolver) *Prechecker {
	return &Prechecker{entities: entities, asOfDates: asOf}
}

func (p *Prechecker) Evaluate(ctx context.Context, sellerAccountID int64, message string, asOfDate *time.Time) (PrecheckDecision, error) {
	normalized := normalizePrecheckText(message)
	decision := PrecheckDecision{
		Route:            PrecheckRouteSupportedChat,
		OpenAICalled:     true,
		IsSupported:      true,
		DetectedEntities: map[string]string{},
	}

	if sellerAccountID <= 0 {
		return decision, nil
	}

	hasMarketplace := detectMarketplaceKeyword(normalized)
	hasConversation := detectSellerConversationPhrase(normalized)
	hasAction := detectActionKeyword(normalized)
	hasExport := detectExportKeyword(normalized)
	hasUnsupportedTopic := detectClearlyUnsupportedTopic(normalized)

	keywords := collectMatchedKeywords(normalized, hasMarketplace, hasConversation, hasAction, hasExport, hasUnsupportedTopic)
	decision.DetectedKeywords = keywords

	if p.entities != nil {
		if sku, ok := extractSKUCandidate(message); ok {
			decision.DetectedEntities["sku"] = fmt.Sprintf("%d", sku)
			exists, err := p.entities.ProductExistsBySKU(ctx, sellerAccountID, sku)
			if err != nil {
				return decision, err
			}
			if exists {
				return decision, nil
			}
			return controlledDecision(
				PrecheckReasonUnknownSKU,
				fmt.Sprintf(unknownSKUMessage, sku),
				unknownSKUSuggestedRephrases(),
				decision.DetectedKeywords,
				decision.DetectedEntities,
			), nil
		}
		if offerID, ok := extractOfferIDCandidate(message); ok {
			decision.DetectedEntities["offer_id"] = offerID
			exists, err := p.entities.ProductExistsByOfferID(ctx, sellerAccountID, offerID)
			if err != nil {
				return decision, err
			}
			if exists {
				return decision, nil
			}
			return controlledDecision(
				PrecheckReasonUnknownOfferID,
				fmt.Sprintf(unknownOfferIDMessage, offerID),
				unknownSKUSuggestedRephrases(),
				decision.DetectedKeywords,
				decision.DetectedEntities,
			), nil
		}
		if campaignID, ok := extractCampaignIDCandidate(message); ok {
			decision.DetectedEntities["campaign_external_id"] = fmt.Sprintf("%d", campaignID)
			exists, err := p.entities.AdCampaignExistsByExternalID(ctx, sellerAccountID, campaignID)
			if err != nil {
				return decision, err
			}
			if exists {
				return decision, nil
			}
			return controlledDecision(
				PrecheckReasonUnknownCampaign,
				fmt.Sprintf(unknownCampaignMessage, fmt.Sprintf("%d", campaignID)),
				unknownCampaignSuggestedRephrases(),
				decision.DetectedKeywords,
				decision.DetectedEntities,
			), nil
		}
	}

	if hasExport {
		return p.evaluateExportRequest(ctx, sellerAccountID, message, asOfDate, decision.DetectedKeywords, decision.DetectedEntities)
	}

	if hasMarketplace || hasConversation {
		return decision, nil
	}

	if hasAction && !hasUnsupportedTopic {
		if isVeryShortAmbiguous(normalized) {
			return clarificationDecision(decision.DetectedKeywords, decision.DetectedEntities), nil
		}
		return decision, nil
	}

	if hasUnsupportedTopic && !hasMarketplace && !hasConversation && !hasAction {
		return controlledDecision(
			PrecheckReasonUnsupportedTopic,
			unsupportedTopicMessage,
			unsupportedTopicSuggestedRephrases(),
			decision.DetectedKeywords,
			decision.DetectedEntities,
		), nil
	}

	if isVeryShortAmbiguous(normalized) {
		return clarificationDecision(decision.DetectedKeywords, decision.DetectedEntities), nil
	}

	// Ambiguous but potentially marketplace-related — prefer passing to OpenAI.
	return decision, nil
}

func (p *Prechecker) evaluateExportRequest(
	ctx context.Context,
	sellerAccountID int64,
	message string,
	asOfDate *time.Time,
	keywords []string,
	entities map[string]string,
) (PrecheckDecision, error) {
	resolveAsOf := func() (time.Time, error) {
		if p.asOfDates == nil {
			return time.Now().UTC().Truncate(24 * time.Hour), nil
		}
		return p.asOfDates.ResolveAsOfDate(ctx, sellerAccountID, asOfDate)
	}
	parsed := chatexport.ParseExportRequest(message, asOfDate, resolveAsOf)
	if parsed.Reason == "" && parsed.ExportType != "" {
		return PrecheckDecision{
			Route:            PrecheckRouteExportExecute,
			OpenAICalled:     false,
			IsSupported:      true,
			DetectedKeywords: append(keywords, "export_execute"),
			DetectedEntities: entities,
			ExportRequest:    &parsed,
		}, nil
	}
	reason := parsed.Reason
	msg := parsed.Message
	suggestions := exportSuggestedRephrases()
	switch reason {
	case chatexport.ReasonFormatNotSupported:
		reason = PrecheckReasonFormatNotSupported
	case chatexport.ReasonExportClarification:
		reason = PrecheckReasonExportClarification
	case chatexport.ReasonExportTypeUnsupported:
		reason = PrecheckReasonExportTypeUnsupported
	case chatexport.ReasonExportSourceUnavailable:
		reason = PrecheckReasonExportSourceUnavailable
	case chatexport.ReasonArbitrarySQL:
		reason = PrecheckReasonArbitrarySQL
		suggestions = nil
	case chatexport.ReasonDateRangeTooLarge:
		reason = PrecheckReasonDateRangeTooLarge
	default:
		reason = PrecheckReasonExportClarification
		if msg == "" {
			msg = exportClarificationMessage
		}
	}
	return PrecheckDecision{
		Route:              PrecheckRouteExportRequestCandidate,
		UnsupportedReason:  reason,
		Message:            msg,
		SuggestedRephrases: suggestions,
		OpenAICalled:       false,
		IsSupported:        false,
		DetectedKeywords:   keywords,
		DetectedEntities:   entities,
	}, nil
}

func controlledDecision(
	reason, message string,
	suggestions, keywords []string,
	entities map[string]string,
) PrecheckDecision {
	route := PrecheckRouteUnsupported
	if reason == PrecheckReasonExportNotImplemented || reason == PrecheckReasonFormatNotSupported ||
		reason == PrecheckReasonExportClarification || reason == PrecheckReasonExportTypeUnsupported ||
		reason == PrecheckReasonExportSourceUnavailable || reason == PrecheckReasonArbitrarySQL ||
		reason == PrecheckReasonDateRangeTooLarge {
		route = PrecheckRouteExportRequestCandidate
	}
	return PrecheckDecision{
		Route:              route,
		UnsupportedReason:  reason,
		Message:            message,
		SuggestedRephrases: suggestions,
		OpenAICalled:       false,
		IsSupported:        false,
		DetectedKeywords:   keywords,
		DetectedEntities:   entities,
	}
}

func clarificationDecision(keywords []string, entities map[string]string) PrecheckDecision {
	return PrecheckDecision{
		Route:              PrecheckRouteClarificationNeeded,
		UnsupportedReason:  PrecheckReasonClarificationNeeded,
		Message:            clarificationNeededMessage,
		SuggestedRephrases: clarificationSuggestedRephrases(),
		OpenAICalled:       false,
		IsSupported:        false,
		DetectedKeywords:   keywords,
		DetectedEntities:   entities,
	}
}

func (d PrecheckDecision) TracePayload(userMessage string) map[string]any {
	return map[string]any{
		"user_message":        userMessage,
		"precheck_result":     string(d.Route),
		"is_supported":        d.IsSupported,
		"unsupported_reason":  nullablePrecheckReason(d.UnsupportedReason),
		"detected_keywords":   d.DetectedKeywords,
		"detected_entities":   d.DetectedEntities,
		"suggested_rephrases": d.SuggestedRephrases,
		"openai_called":       d.OpenAICalled,
	}
}

func nullablePrecheckReason(reason string) any {
	if strings.TrimSpace(reason) == "" {
		return nil
	}
	return reason
}

func FormatControlledAnswer(message string, suggestions []string) string {
	if len(suggestions) == 0 {
		return message
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(message))
	b.WriteString("\n\nНапример:\n")
	for _, s := range suggestions {
		b.WriteString("- ")
		b.WriteString(s)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func normalizePrecheckText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "ё", "е")
	return collapseSpaces(s)
}

func collapseSpaces(s string) string {
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

func detectMarketplaceKeyword(normalized string) bool {
	for _, kw := range marketplaceKeywords {
		if strings.Contains(normalized, kw) {
			return true
		}
	}
	return false
}

func detectSellerConversationPhrase(normalized string) bool {
	for _, phrase := range sellerConversationPhrases {
		if strings.Contains(normalized, phrase) {
			return true
		}
	}
	return false
}

func detectActionKeyword(normalized string) bool {
	for _, kw := range actionKeywords {
		if strings.Contains(normalized, kw) {
			return true
		}
	}
	return false
}

func detectExportKeyword(normalized string) bool {
	return chatexport.HasExportAction(normalized)
}

func detectClearlyUnsupportedTopic(normalized string) bool {
	for _, kw := range unsupportedTopicKeywords {
		if strings.Contains(normalized, kw) {
			return true
		}
	}
	return false
}

func isVeryShortAmbiguous(normalized string) bool {
	runes := []rune(normalized)
	if len(runes) > 40 {
		return false
	}
	words := strings.Fields(normalized)
	return len(words) <= 3
}

var skuPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])sku\s*[:#]?\s*(\d{3,})`),
	regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])скю\s*[:#]?\s*(\d{3,})`),
	regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])товар\s+(\d{3,})`),
}

var offerIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])offer[_\s-]?id\s*[:=]?\s*([a-zA-Z0-9][a-zA-Z0-9._-]{1,})`),
	regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])артикул\s*[:=]?\s*([a-zA-Z0-9][a-zA-Z0-9._-]{1,})`),
}

var campaignIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])кампани\p{L}*\s+(\d{1,})`),
	regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])campaign\s+(\d{1,})`),
}

func extractSKUCandidate(message string) (int64, bool) {
	for _, re := range skuPatterns {
		m := re.FindStringSubmatch(message)
		if len(m) < 2 {
			continue
		}
		var sku int64
		if _, err := fmt.Sscanf(m[1], "%d", &sku); err == nil && sku > 0 {
			return sku, true
		}
	}
	return 0, false
}

func extractOfferIDCandidate(message string) (string, bool) {
	for _, re := range offerIDPatterns {
		m := re.FindStringSubmatch(message)
		if len(m) >= 2 && strings.TrimSpace(m[1]) != "" {
			return strings.TrimSpace(m[1]), true
		}
	}
	return "", false
}

func extractCampaignIDCandidate(message string) (int64, bool) {
	for _, re := range campaignIDPatterns {
		loc := re.FindStringSubmatchIndex(message)
		if len(loc) < 4 {
			continue
		}
		numStr := strings.TrimSpace(message[loc[2]:loc[3]])
		after := strings.TrimSpace(message[loc[3]:])
		if isLikelyDayPeriodSuffix(after) {
			continue
		}
		var id int64
		if _, err := fmt.Sscanf(numStr, "%d", &id); err == nil && id > 0 {
			return id, true
		}
	}
	return 0, false
}

// isLikelyDayPeriodSuffix returns true when digits after a campaign mention are a day/week/month window, not an ID.
func isLikelyDayPeriodSuffix(after string) bool {
	after = strings.TrimSpace(strings.ToLower(strings.ReplaceAll(after, "ё", "е")))
	if after == "" {
		return false
	}
	periodPrefixes := []string{
		"дн", "дня", "дней", "day", "days",
		"недел", "week",
		"месяц", "month",
		"лет", "год", "year",
		"час", "hour",
	}
	for _, prefix := range periodPrefixes {
		if strings.HasPrefix(after, prefix) {
			return true
		}
	}
	return false
}

func collectMatchedKeywords(
	normalized string,
	hasMarketplace, hasConversation, hasAction, hasExport, hasUnsupported bool,
) []string {
	out := make([]string, 0, 8)
	if hasMarketplace {
		out = append(out, "marketplace")
	}
	if hasConversation {
		out = append(out, "seller_conversation")
	}
	if hasAction {
		out = append(out, "action")
	}
	if hasExport {
		out = append(out, "export")
	}
	if hasUnsupported {
		out = append(out, "unsupported_topic_signal")
	}
	_ = normalized
	return out
}

const (
	unsupportedTopicMessage    = "Я могу помогать только с аналитикой вашего Ozon-кабинета: продажами, заказами, остатками, рекламой, alert'ами, рекомендациями, метриками и выгрузками. Этот вопрос не связан с данными магазина, поэтому я не могу корректно ответить на него здесь."
	unknownSKUMessage          = "Я не нашёл SKU %d в подключённом кабинете. Проверьте номер SKU или спросите по offer_id/названию товара."
	unknownOfferIDMessage      = "Я не нашёл offer_id %s в подключённом кабинете. Проверьте артикул или спросите по SKU/названию товара."
	unknownCampaignMessage     = "Я не нашёл рекламную кампанию %s в подключённом кабинете. Проверьте название или ID кампании."
	clarificationNeededMessage = "Я не до конца понял, что именно нужно проверить. Могу посмотреть продажи, остатки, рекламу, alert'ы или рекомендации. Например:"
	exportClarificationMessage = "Что именно выгрузить: продажи, SKU, остатки, рекламу, alert'ы или рекомендации?"
)

func unsupportedTopicSuggestedRephrases() []string {
	return []string{
		"Какие SKU просели по заказам за последние 7 дней?",
		"Какие товары скоро закончатся?",
		"Какие рекламные кампании имеют высокий ДРР?",
		"Какие alert'ы самые важные сейчас?",
	}
}

func unknownSKUSuggestedRephrases() []string {
	return []string{
		"Покажи проблемные SKU за последние 7 дней.",
		"Какие товары просели по заказам?",
		"Какие товары скоро закончатся?",
	}
}

func unknownCampaignSuggestedRephrases() []string {
	return []string{
		"Какие кампании имеют высокий ДРР?",
		"Что с рекламой за последние 7 дней?",
		"Какие кампании тратят бюджет без заказов?",
	}
}

func clarificationSuggestedRephrases() []string {
	return []string{
		"Что проверить сегодня?",
		"Почему упали заказы за последние 7 дней?",
		"Какие товары с риском остатка?",
		"Какие кампании имеют высокий ДРР?",
	}
}

func exportSuggestedRephrases() []string {
	return []string{
		"Покажи товары с риском остатка за последние 7 дней.",
		"Какие рекламные кампании имеют высокий ДРР?",
	}
}

var marketplaceKeywords = []string{
	"ozon", "озон", "маркетплейс", "магазин", "кабинет", "селлер",
	"продаж", "заказ", "выручк", "оборот", "товар", "sku", "артикул", "offer_id",
	"карточк", "остат", "склад", "fbo", "fbs", "фбо", "фбс", "поставк", "хранен",
	"излиш", "дефицит", "реклам", "кампани", "дрр", "drr", "roas", "cpo", "ctr", "cpc",
	"метрик", "аналитик", "воронк", "показ", "клик", "корзин", "конверси",
	"алерт", "alert", "рекомендац", "отчет", "отчёт", "выгруз", "экспорт", "excel", "xlsx", "csv",
}

var sellerConversationPhrases = []string{
	"что у меня не так", "почему все просело", "почему всё просело", "где проблема",
	"где я теряю деньги", "что проверить", "какие проблемы", "какие слабые места",
	"что делать", "какие товары проблемные", "что с кабинетом", "почему мало заказов",
	"почему не покупают", "почему клиенты не покупают", "что улучшить", "где просадка", "что просело",
}

var actionKeywords = []string{
	"проанализируй", "покажи", "объясни", "сравни", "найди", "проверь", "оцени", "расскажи",
	"почему", "что делать", "что проверить", "дай рекомендации", "сформируй вывод",
	"подготовь отчет", "подготовь отчёт",
}

var exportKeywords = []string{
	"выгрузи", "экспортируй", "сделай excel", "сделай xlsx", "сделай csv",
	"скачать", "скачай", "выгрузка", "подготовь файл", "сформируй файл", "дай файл",
	"таблицу в excel", "таблица csv", "подготовь xlsx", "подготовь csv",
}

var unsupportedTopicKeywords = []string{
	"погод", "рецепт", "борщ", "фильм", "сериал", "музык", "стих", "песн",
	"президент", "политик", "истори", "спорт", "футбол", "хоккей",
	"курс валют", "биткоин", "крипт", "программирован", "python", "javascript",
	"математик", "перевод", "английск", "немецк", "медицин", "здоровь", "юрист",
}
