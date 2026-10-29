package chat

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type AnswerValidator struct{}

func NewAnswerValidator() *AnswerValidator {
	return &AnswerValidator{}
}

func (v *AnswerValidator) Validate(answer *ChatAnswer, ctx *FactContext) (*AnswerValidationResult, error) {
	_ = v
	if answer == nil {
		return nil, errors.New("answer is required")
	}
	result := &AnswerValidationResult{IsValid: true, Errors: []string{}, Warnings: []string{}}
	finalConfidence := answer.ConfidenceLevel
	trimmedAnswer := strings.TrimSpace(answer.Answer)
	trimmedSummary := strings.TrimSpace(answer.Summary)
	if trimmedAnswer == "" {
		result.Errors = append(result.Errors, "answer text is empty")
	}
	if trimmedSummary == "" {
		result.Errors = append(result.Errors, "summary is empty")
	}
	switch finalConfidence {
	case ConfidenceLevelLow, ConfidenceLevelMedium, ConfidenceLevelHigh:
	default:
		result.Errors = append(result.Errors, "invalid confidence level")
		finalConfidence = ConfidenceLevelLow
	}
	if len(answer.SupportingFacts) == 0 {
		result.Errors = append(result.Errors, "supporting facts are required")
	}

	allText := collectAnswerText(answer)
	forbiddenAutoActionClaims := []string{
		"я изменил цену", "цена изменена", "я снизил цену", "я повысил цену", "я остановил кампанию", "кампания остановлена",
		"я запустил кампанию", "я изменил бюджет", "бюджет изменён", "я создал поставку", "поставка создана", "я закрыл alert",
		"alert закрыт", "я принял рекомендацию", "рекомендация принята", "я обновил данные в ozon", "я отправил запрос в ozon",
		"i changed the price", "price has been changed", "i lowered the price", "i increased the price", "i stopped the campaign",
		"campaign has been stopped", "i launched the campaign", "i changed the budget", "budget has been changed",
		"i created a replenishment", "i closed the alert", "i accepted the recommendation", "i updated ozon",
	}
	forbiddenDBOrOzonClaims := []string{
		"я сходил в базу", "я запросил базу данных", "я выполнил sql", "select *", "я проверил ozon напрямую", "я обратился в ozon",
		"i queried the database", "i ran sql", "i checked ozon directly",
	}
	forbiddenSecretMarkers := []string{"sk-", "openai_api_key", "api_key", "authorization: bearer", "password", "secret", "raw_payload", "raw_ai_response", "sql query"}

	for _, t := range allText {
		lower := strings.ToLower(t)
		if stringsContainsAny(lower, forbiddenAutoActionClaims) {
			result.Errors = append(result.Errors, "answer contains forbidden auto-action claim")
		}
		if stringsContainsAny(lower, forbiddenDBOrOzonClaims) {
			result.Errors = append(result.Errors, "answer contains forbidden direct database/ozon access claim")
		}
		if stringsContainsAny(lower, forbiddenSecretMarkers) {
			result.Errors = append(result.Errors, "answer contains forbidden secret/raw marker")
		}
	}

	alertIDsInContext := map[int64]struct{}{}
	recommendIDsInContext := map[int64]struct{}{}
	if ctx != nil {
		for _, a := range ctx.RelatedAlerts {
			alertIDsInContext[a.ID] = struct{}{}
		}
		for _, r := range ctx.RelatedRecommendations {
			recommendIDsInContext[r.ID] = struct{}{}
		}
	}
	for _, id := range answer.RelatedAlertIDs {
		if id <= 0 {
			result.Errors = append(result.Errors, "related alert id must be > 0")
			continue
		}
		if ctx == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("related alert id not found in context: %d", id))
			continue
		}
		if _, ok := alertIDsInContext[id]; !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("related alert id not found in context: %d", id))
		}
	}
	for _, id := range answer.RelatedRecommendationIDs {
		if id <= 0 {
			result.Errors = append(result.Errors, "related recommendation id must be > 0")
			continue
		}
		if ctx == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("related recommendation id not found in context: %d", id))
			continue
		}
		if _, ok := recommendIDsInContext[id]; !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("related recommendation id not found in context: %d", id))
		}
	}

	allowedSources := map[string]struct{}{
		"dashboard": {}, "recommendation": {}, "alert": {}, "critical_sku": {}, "stock_risk": {}, "advertising": {},
		"price_economics": {}, "sku_metrics": {}, "sku_context": {}, "campaign_context": {}, "abc_analysis": {},
		"tool_result": {}, "freshness": {}, "limitation": {}, "action_context": {}, "attachment": {},
	}
	for i, sf := range answer.SupportingFacts {
		if strings.TrimSpace(sf.Fact) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("supporting fact %d is empty", i+1))
		}
		source := strings.TrimSpace(strings.ToLower(sf.Source))
		if source == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("supporting fact %d source is empty", i+1))
			continue
		}
		if _, ok := allowedSources[source]; !ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf("unknown supporting fact source: %s", sf.Source))
		}
		if sf.ID != nil && ctx != nil {
			if source == "alert" {
				if _, ok := alertIDsInContext[*sf.ID]; !ok {
					result.Errors = append(result.Errors, fmt.Sprintf("supporting fact alert id not found in context: %d", *sf.ID))
				}
			}
			if source == "recommendation" {
				if _, ok := recommendIDsInContext[*sf.ID]; !ok {
					result.Errors = append(result.Errors, fmt.Sprintf("supporting fact recommendation id not found in context: %d", *sf.ID))
				}
			}
		}
	}

	if ctx != nil {
		mentionedAlertIDs, mentionedRecommendationIDs := extractMentionedEntityIDs(allText)
		for _, id := range mentionedAlertIDs {
			if _, ok := alertIDsInContext[id]; !ok {
				result.Errors = append(result.Errors, fmt.Sprintf("text references alert id not found in context: %d", id))
			}
		}
		for _, id := range mentionedRecommendationIDs {
			if _, ok := recommendIDsInContext[id]; !ok {
				result.Errors = append(result.Errors, fmt.Sprintf("text references recommendation id not found in context: %d", id))
			}
		}
	}

	hasFacts := factContextHasFacts(ctx)
	isUnsupportedIntent := ctx != nil && ctx.Intent == ChatIntentUnsupported
	if !hasFacts {
		if !isUnsupportedIntent && finalConfidence != ConfidenceLevelLow {
			result.Errors = append(result.Errors, "answer confidence must be low when no facts are available")
		}
		if len(answer.Limitations) == 0 {
			result.Errors = append(result.Errors, "answer must include limitations when no facts are available")
		}
		if len(answer.SupportingFacts) > 0 {
			hasLimitationSource := false
			for _, sf := range answer.SupportingFacts {
				if strings.EqualFold(strings.TrimSpace(sf.Source), "limitation") {
					hasLimitationSource = true
					break
				}
			}
			if !hasLimitationSource {
				result.Errors = append(result.Errors, "supporting facts must include limitation source when no facts are available")
			}
		}
	}
	if isUnsupportedIntent {
		if len(answer.Limitations) == 0 {
			result.Errors = append(result.Errors, "unsupported answer must include limitations")
		}
		hasLimitationSource := false
		for _, sf := range answer.SupportingFacts {
			if strings.EqualFold(strings.TrimSpace(sf.Source), "limitation") {
				hasLimitationSource = true
				break
			}
		}
		if !hasLimitationSource {
			result.Errors = append(result.Errors, "unsupported answer must include limitation supporting fact")
		}
	}

	if ctx != nil && len(ctx.Limitations) > 0 {
		if len(answer.Limitations) == 0 && !containsLimitationWording(strings.ToLower(trimmedAnswer)) {
			result.Warnings = append(result.Warnings, "context limitations were not reflected in answer")
			finalConfidence = downgradeConfidence(finalConfidence)
		}
		if hasCriticalLimitation(ctx.Limitations) {
			if stringsContainsAny(strings.ToLower(strings.Join(ctx.Limitations, " ")), []string{"no factual data"}) {
				finalConfidence = ConfidenceLevelLow
				result.Warnings = append(result.Warnings, "critical no-data limitation forces low confidence")
			} else {
				finalConfidence = downgradeConfidence(finalConfidence)
			}
		}
		if limitationMentionsAdvertisingUnavailable(ctx.Limitations) && answerMentionsAdSpecifics(allText) {
			result.Warnings = append(result.Warnings, "answer may overstate advertising precision despite context limitations")
			finalConfidence = downgradeConfidence(finalConfidence)
		}
		if limitationMentionsApproximateCategory(ctx.Limitations) && answerClaimsExactCategory(allText) {
			result.Warnings = append(result.Warnings, "answer claims exact category insight despite approximate category limitations")
			finalConfidence = downgradeConfidence(finalConfidence)
		}
	}
	if ctx != nil && ctx.ContextStats.Truncated {
		result.Warnings = append(result.Warnings, "context is truncated")
		finalConfidence = downgradeConfidence(finalConfidence)
	}
	if ctx != nil && ctx.ContextStats.FailedToolsCount > 0 {
		result.Warnings = append(result.Warnings, "some tools failed while building context")
		finalConfidence = downgradeConfidence(finalConfidence)
	}

	result.Errors = dedupeValidationMessages(result.Errors)
	result.Warnings = dedupeValidationMessages(result.Warnings)
	result.IsValid = len(result.Errors) == 0
	result.FinalConfidenceLevel = finalConfidence
	return result, nil
}

func downgradeConfidence(level ConfidenceLevel) ConfidenceLevel {
	switch level {
	case ConfidenceLevelHigh:
		return ConfidenceLevelMedium
	case ConfidenceLevelMedium:
		return ConfidenceLevelLow
	default:
		return ConfidenceLevelLow
	}
}

func factContextHasFacts(ctx *FactContext) bool {
	if ctx == nil {
		return false
	}
	f := ctx.Facts
	return len(f.Dashboard) > 0 ||
		len(f.Recommendations) > 0 ||
		len(f.RecommendationDetails) > 0 ||
		len(f.Alerts) > 0 ||
		len(f.CriticalSKUs) > 0 ||
		len(f.StockRisks) > 0 ||
		len(f.Advertising) > 0 ||
		len(f.PriceEconomicsRisks) > 0 ||
		len(f.SKUMetrics) > 0 ||
		len(f.SKUContexts) > 0 ||
		len(f.CampaignContexts) > 0 ||
		len(f.ABCAnalysis) > 0 ||
		len(f.Other) > 0 ||
		ctx.Action != nil ||
		len(ctx.RelatedAlerts) > 0 ||
		len(ctx.RelatedRecommendations) > 0
}

// normalizeAnswerForValidation repairs common model shape issues so honest
// no-data answers are not replaced with controlled failure messages.
func normalizeAnswerForValidation(answer *ChatAnswer, ctx *FactContext) {
	if answer == nil {
		return
	}
	if strings.TrimSpace(answer.Answer) == "" {
		answer.Answer = "Недостаточно данных в кабинете для точного ответа."
	}
	if strings.TrimSpace(answer.Summary) == "" {
		answer.Summary = truncateStreamSummary(answer.Answer, 240)
	}
	if !factContextHasFacts(ctx) {
		limitation := "No factual data was available for this question."
		if ctx != nil {
			for _, l := range ctx.Limitations {
				if strings.TrimSpace(l) != "" {
					limitation = strings.TrimSpace(l)
					break
				}
			}
		}
		answer.ConfidenceLevel = ConfidenceLevelLow
		if len(answer.Limitations) == 0 {
			answer.Limitations = []string{limitation}
		}
		hasLimitationSource := false
		for _, sf := range answer.SupportingFacts {
			if strings.EqualFold(strings.TrimSpace(sf.Source), "limitation") {
				hasLimitationSource = true
				break
			}
		}
		if !hasLimitationSource {
			answer.SupportingFacts = append(answer.SupportingFacts, SupportingFact{
				Source: "limitation",
				Fact:   limitation,
			})
		}
		return
	}
	if len(answer.SupportingFacts) == 0 {
		answer.SupportingFacts = []SupportingFact{{
			Source: "tool_result",
			Fact:   "Ответ основан на фактическом контексте кабинета.",
		}}
	}
	switch answer.ConfidenceLevel {
	case ConfidenceLevelLow, ConfidenceLevelMedium, ConfidenceLevelHigh:
	default:
		answer.ConfidenceLevel = ConfidenceLevelMedium
	}
}

func collectAnswerText(answer *ChatAnswer) []string {
	out := []string{answer.Answer, answer.Summary}
	for _, sf := range answer.SupportingFacts {
		out = append(out, sf.Fact)
	}
	for _, l := range answer.Limitations {
		out = append(out, l)
	}
	return out
}

func extractMentionedEntityIDs(texts []string) ([]int64, []int64) {
	alertRe := regexp.MustCompile(`(?i)(alert|алерт)\s*#?\s*(\d+)`)
	recRe := regexp.MustCompile(`(?i)(recommendation|рекомендац(?:ия|ии))\s*#?\s*(\d+)`)
	alertIDs := []int64{}
	recIDs := []int64{}
	for _, text := range texts {
		for _, m := range alertRe.FindAllStringSubmatch(text, -1) {
			id, _ := strconv.ParseInt(m[2], 10, 64)
			if id > 0 {
				alertIDs = append(alertIDs, id)
			}
		}
		for _, m := range recRe.FindAllStringSubmatch(text, -1) {
			id, _ := strconv.ParseInt(m[2], 10, 64)
			if id > 0 {
				recIDs = append(recIDs, id)
			}
		}
	}
	return uniqueInt64(alertIDs), uniqueInt64(recIDs)
}

func uniqueInt64(items []int64) []int64 {
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func containsLimitationWording(answerLower string) bool {
	return stringsContainsAny(answerLower, []string{"огранич", "недостаточно данных", "limited", "limitation", "недоступ"})
}

func hasCriticalLimitation(limitations []string) bool {
	for _, l := range limitations {
		ll := strings.ToLower(l)
		if stringsContainsAny(ll, []string{"no factual data", "context was truncated", "failed", "unavailable", "not available", "недоступ"}) {
			return true
		}
	}
	return false
}

func limitationMentionsAdvertisingUnavailable(limitations []string) bool {
	for _, l := range limitations {
		ll := strings.ToLower(l)
		if stringsContainsAny(ll, []string{"advertising unavailable", "no advertising data", "advertising data is not available", "реклам", "недоступ"}) {
			return true
		}
	}
	return false
}

func limitationMentionsApproximateCategory(limitations []string) bool {
	for _, l := range limitations {
		ll := strings.ToLower(l)
		if stringsContainsAny(ll, []string{"category filtering may be approximate", "category filtering depends", "приблиз", "категор"}) {
			return true
		}
	}
	return false
}

func answerMentionsAdSpecifics(texts []string) bool {
	for _, text := range texts {
		t := strings.ToLower(text)
		if stringsContainsAny(t, []string{"roas", "spend", "campaign", "кампан", "расход"}) {
			return true
		}
	}
	return false
}

func answerClaimsExactCategory(texts []string) bool {
	for _, text := range texts {
		t := strings.ToLower(text)
		if stringsContainsAny(t, []string{"точно по катег", "exactly by category", "exact category"}) {
			return true
		}
	}
	return false
}

func stringsContainsAny(text string, phrases []string) bool {
	for _, phrase := range phrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func dedupeValidationMessages(messages []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(messages))
	for _, m := range messages {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}
