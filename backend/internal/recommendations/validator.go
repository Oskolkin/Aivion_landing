package recommendations

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type OutputValidator struct {
	MaxRecommendations int
	MaxTextLength      int
}

type ValidationResult struct {
	ValidRecommendations    []ValidatedRecommendation `json:"valid_recommendations"`
	RejectedRecommendations []RejectedRecommendation  `json:"rejected_recommendations"`
	TotalRecommendations    int                       `json:"total_recommendations"`
	NormalizedTypesCount    int                       `json:"normalized_types_count,omitempty"`
	NormalizedTypes         []TypeNormalizationEntry  `json:"normalized_types,omitempty"`
}

type ValidatedRecommendation struct {
	Recommendation       AIRecommendationCandidate `json:"recommendation"`
	Warnings             []string                  `json:"warnings,omitempty"`
	FinalConfidenceLevel string                    `json:"final_confidence_level"`
}

type RejectedRecommendation struct {
	Index              int    `json:"index"`
	Reason             string `json:"reason"`
	RecommendationType string `json:"recommendation_type,omitempty"`
	Raw                any    `json:"raw,omitempty"`
}

type AIRecommendationCandidate struct {
	RecommendationType string         `json:"recommendation_type"`
	Horizon            string         `json:"horizon"`
	EntityType         string         `json:"entity_type"`
	EntityID           *string        `json:"entity_id,omitempty"`
	EntitySKU          *int64         `json:"entity_sku,omitempty"`
	EntityOfferID      *string        `json:"entity_offer_id,omitempty"`
	Title              string         `json:"title"`
	WhatHappened       string         `json:"what_happened"`
	WhyItMatters       string         `json:"why_it_matters"`
	RecommendedAction  string         `json:"recommended_action"`
	ExpectedEffect     *string        `json:"expected_effect,omitempty"`
	PriorityScore      float64        `json:"priority_score"`
	PriorityLevel      string         `json:"priority_level"`
	Urgency            string         `json:"urgency"`
	ConfidenceLevel    string         `json:"confidence_level"`
	SupportingMetrics  map[string]any `json:"supporting_metrics,omitempty"`
	Constraints        map[string]any `json:"constraints,omitempty"`
	ConstraintsChecked map[string]any `json:"constraints_checked,omitempty"`
	SupportingAlertIDs []int64        `json:"supporting_alert_ids,omitempty"`
	RelatedAlertTypes  []string       `json:"related_alert_types,omitempty"`
}

type recommendationsEnvelope struct {
	Recommendations []AIRecommendationCandidate `json:"recommendations"`
}

var (
	priceActionRegex = regexp.MustCompile(`(?i)(снизить цену|понизить цену|уменьшить цену|поднять цену|повысить цену|reduce price|lower price|increase price|raise price)`)
	priceDownRegex   = regexp.MustCompile(`(?i)(снизить цену|понизить цену|уменьшить цену|reduce price|lower price)`)
	increaseAdsRegex = regexp.MustCompile(`(?i)(увеличить рекламу|усилить рекламу|увеличить бюджет|повысить бюджет|increase ad|increase ads|increase budget|scale campaign)`)
	numberRegex      = regexp.MustCompile(`\d+(?:[.,]\d+)?`)
)

func NewOutputValidator() *OutputValidator {
	return &OutputValidator{
		MaxRecommendations: 50,
		MaxTextLength:      2000,
	}
}

func (v *OutputValidator) Validate(output *GenerateRecommendationsOutput, ctx *AIRecommendationContext) (*ValidationResult, error) {
	if output == nil {
		return nil, fmt.Errorf("output is required")
	}
	if strings.TrimSpace(output.Content) == "" {
		return nil, fmt.Errorf("output content is empty")
	}
	if v.MaxRecommendations <= 0 {
		v.MaxRecommendations = 50
	}
	if v.MaxTextLength <= 0 {
		v.MaxTextLength = 2000
	}

	candidates, raws, err := parseCandidates(output.Content)
	if err != nil {
		return nil, fmt.Errorf("parse ai output json: %w", err)
	}
	if len(candidates) > v.MaxRecommendations {
		candidates = candidates[:v.MaxRecommendations]
		raws = raws[:v.MaxRecommendations]
	}

	indexes := buildContextIndexes(ctx)

	result := &ValidationResult{
		ValidRecommendations:    make([]ValidatedRecommendation, 0, len(candidates)),
		RejectedRecommendations: make([]RejectedRecommendation, 0),
		TotalRecommendations:    len(candidates),
	}
	for i, c := range candidates {
		norm, warnings, reason, typeNorm := v.validateOne(c, indexes)
		if typeNorm != nil {
			result.NormalizedTypes = append(result.NormalizedTypes, TypeNormalizationEntry{
				Index:     i,
				Original:  typeNorm.Original,
				Canonical: typeNorm.Canonical,
			})
		}
		if reason != "" {
			rej := RejectedRecommendation{
				Index:  i,
				Reason: reason,
				Raw:    raws[i],
			}
			if rt := extractRecommendationTypeFromRaw(raws[i]); rt != "" {
				rej.RecommendationType = rt
			} else if strings.TrimSpace(c.RecommendationType) != "" {
				rej.RecommendationType = strings.TrimSpace(c.RecommendationType)
			}
			result.RejectedRecommendations = append(result.RejectedRecommendations, rej)
			continue
		}
		result.ValidRecommendations = append(result.ValidRecommendations, ValidatedRecommendation{
			Recommendation:       norm,
			Warnings:             warnings,
			FinalConfidenceLevel: downgradeConfidence(norm.ConfidenceLevel, len(warnings)),
		})
	}
	result.NormalizedTypesCount = len(result.NormalizedTypes)
	return result, nil
}

type normalizedTypeChange struct {
	Original  string
	Canonical string
}

type contextIndexes struct {
	alertIDs                    map[int64]struct{}
	alertTypeByID               map[int64]string
	alertTypes                  map[string]struct{}
	skus                        map[int64]struct{}
	offerIDs                    map[string]struct{}
	productIDs                  map[string]struct{}
	campaignIDs                 map[string]struct{}
	minPriceBySKU               map[int64]float64
	maxPriceBySKU               map[int64]float64
	lowStockSKUs                map[int64]struct{}
	hasStockData                bool
	hasAdData                   bool
	hasPriceData                bool
	hasMissingConstraintsSignal bool
}

func (v *OutputValidator) validateOne(in AIRecommendationCandidate, idx contextIndexes) (AIRecommendationCandidate, []string, string, *normalizedTypeChange) {
	out := in
	originalType := strings.TrimSpace(out.RecommendationType)
	out.RecommendationType = originalType
	out.Horizon = strings.TrimSpace(out.Horizon)
	out.EntityType = strings.TrimSpace(out.EntityType)
	out.Title = strings.TrimSpace(out.Title)
	out.WhatHappened = strings.TrimSpace(out.WhatHappened)
	out.WhyItMatters = strings.TrimSpace(out.WhyItMatters)
	out.RecommendedAction = strings.TrimSpace(out.RecommendedAction)
	out.PriorityLevel = strings.TrimSpace(out.PriorityLevel)
	out.Urgency = strings.TrimSpace(out.Urgency)
	out.ConfidenceLevel = strings.TrimSpace(out.ConfidenceLevel)
	out.RelatedAlertTypes = dedupeStringsTrimmed(out.RelatedAlertTypes)
	out.SupportingAlertIDs = dedupeInt64(out.SupportingAlertIDs)
	out.Title = capString(out.Title, 200)
	out.WhatHappened = capString(out.WhatHappened, 1000)
	out.WhyItMatters = capString(out.WhyItMatters, 1000)
	out.RecommendedAction = capString(out.RecommendedAction, 1500)
	if out.ExpectedEffect != nil {
		s := capString(*out.ExpectedEffect, 1000)
		out.ExpectedEffect = &s
	}
	if len(out.Constraints) == 0 && len(out.ConstraintsChecked) > 0 {
		out.Constraints = out.ConstraintsChecked
	}
	if len(out.ConstraintsChecked) == 0 && len(out.Constraints) > 0 {
		out.ConstraintsChecked = out.Constraints
	}
	warnings := make([]string, 0)

	var typeNorm *normalizedTypeChange
	if out.RecommendationType == "" {
		return out, warnings, "recommendation_type is required", nil
	}
	canonical, changed, ok := NormalizeRecommendationType(out.RecommendationType)
	if !ok {
		return out, warnings, fmt.Sprintf("recommendation_type is not allowed: %s", originalType), nil
	}
	if changed {
		typeNorm = &normalizedTypeChange{Original: originalType, Canonical: canonical}
	}
	out.RecommendationType = canonical
	if !IsActiveRecommendationType(canonical) {
		return out, warnings, fmt.Sprintf("recommendation_type is disabled for MVP: %s", canonical), typeNorm
	}
	if !oneOf(out.Horizon, "short_term", "medium_term", "long_term") {
		return out, warnings, "invalid horizon", typeNorm
	}
	if !oneOf(out.EntityType, "account", "sku", "product", "campaign", "pricing_constraint") {
		return out, warnings, "invalid entity_type", typeNorm
	}
	if out.Title == "" || out.WhatHappened == "" || out.WhyItMatters == "" || out.RecommendedAction == "" {
		return out, warnings, "title/what_happened/why_it_matters/recommended_action are required", typeNorm
	}
	if len(out.Title) > v.MaxTextLength || len(out.WhatHappened) > v.MaxTextLength || len(out.WhyItMatters) > v.MaxTextLength || len(out.RecommendedAction) > v.MaxTextLength {
		return out, warnings, "one or more text fields exceed max length", typeNorm
	}
	if !oneOf(out.PriorityLevel, "low", "medium", "high", "critical") {
		return out, warnings, "invalid priority_level", typeNorm
	}
	if !oneOf(out.Urgency, "low", "medium", "high", "immediate") {
		return out, warnings, "invalid urgency", typeNorm
	}
	if !oneOf(out.ConfidenceLevel, "low", "medium", "high") {
		return out, warnings, "invalid confidence_level", typeNorm
	}
	if math.IsNaN(out.PriorityScore) || out.PriorityScore < 0 || out.PriorityScore > 100 {
		return out, warnings, "priority_score must be in range [0,100]", typeNorm
	}
	if len(out.SupportingMetrics) == 0 {
		return out, warnings, "supporting_metrics is required and must be non-empty object", typeNorm
	}
	if len(out.Constraints) == 0 {
		return out, warnings, "constraints_checked is required and must be non-empty object", typeNorm
	}
	if len(idx.alertIDs) > 0 && len(out.SupportingAlertIDs) == 0 {
		return out, warnings, "supporting_alert_ids is required when open alerts exist in context", typeNorm
	}

	switch out.EntityType {
	case "account":
		// account-level recommendation can omit concrete identifiers.
	case "sku":
		if out.EntitySKU == nil && emptyStringPtr(out.EntityOfferID) && emptyStringPtr(out.EntityID) {
			return out, warnings, "sku entity must include entity_sku, entity_offer_id, or entity_id", typeNorm
		}
	case "product", "campaign", "pricing_constraint":
		if emptyStringPtr(out.EntityID) {
			return out, warnings, "entity_id is required for selected entity_type", typeNorm
		}
	}
	if !entityExistsInContext(out, idx) {
		return out, warnings, "entity does not exist in provided context", typeNorm
	}

	if len(idx.alertIDs) > 0 {
		for _, id := range out.SupportingAlertIDs {
			if _, ok := idx.alertIDs[id]; !ok {
				return out, warnings, "supporting_alert_ids contain unknown alert id", typeNorm
			}
		}
	}
	if len(out.RelatedAlertTypes) > 0 {
		for _, t := range out.RelatedAlertTypes {
			if _, ok := idx.alertTypes[t]; !ok {
				return out, warnings, "related_alert_types contain unknown alert type", typeNorm
			}
		}
		if len(out.SupportingAlertIDs) > 0 {
			for _, id := range out.SupportingAlertIDs {
				if expected, ok := idx.alertTypeByID[id]; ok && !containsString(out.RelatedAlertTypes, expected) {
					return out, warnings, "related_alert_types mismatch with supporting_alert_ids", typeNorm
				}
			}
		}
	}

	combinedText := strings.ToLower(strings.Join([]string{
		out.Title, out.WhatHappened, out.WhyItMatters, out.RecommendedAction, derefString(out.ExpectedEffect),
	}, " "))
	hasPriceAction := priceActionRegex.MatchString(combinedText)
	hasPriceDecrease := priceDownRegex.MatchString(combinedText)
	if hasPriceAction {
		if !idx.hasPriceData {
			warnings = append(warnings, "price action without pricing context")
		}
		if !constraintBool(out.Constraints, "pricing_checked") {
			warnings = append(warnings, "pricing_checked is missing")
		}
		prices := extractPrices(combinedText)
		if len(prices) > 0 && out.EntitySKU != nil {
			minPrice, hasMin := idx.minPriceBySKU[*out.EntitySKU]
			maxPrice, hasMax := idx.maxPriceBySKU[*out.EntitySKU]
			for _, p := range prices {
				if hasMin && p < minPrice {
					return out, warnings, "suggested price is below effective_min_price", typeNorm
				}
				if hasMax && p > maxPrice {
					return out, warnings, "suggested price is above effective_max_price", typeNorm
				}
			}
		}
	}

	if hasPriceDecrease {
		expectedMargin, hasExpectedMargin := numberFromMetrics(out.SupportingMetrics, "expected_margin")
		hasMarginRiskSignal := containsString(out.RelatedAlertTypes, "margin_risk_at_current_price")
		if hasExpectedMargin && expectedMargin < 0 {
			return out, warnings, "expected_margin is below 0", typeNorm
		}
		if hasExpectedMargin && expectedMargin < 0.05 {
			return out, warnings, "expected_margin is below 0.05", typeNorm
		}
		if hasMarginRiskSignal || (hasExpectedMargin && expectedMargin < 0.10) {
			if !constraintBool(out.Constraints, "margin_checked") {
				return out, warnings, "margin_checked must be true for price decrease under margin risk", typeNorm
			}
		}
	}

	if increaseAdsRegex.MatchString(combinedText) {
		lowStock := false
		if out.EntitySKU != nil {
			_, lowStock = idx.lowStockSKUs[*out.EntitySKU]
		}
		if lowStock || containsString(out.RelatedAlertTypes, "stock_oos_risk") || containsString(out.RelatedAlertTypes, "stock_low_coverage") || containsString(out.RelatedAlertTypes, "ad_budget_on_low_stock_sku") {
			return out, warnings, "cannot increase ads for low-stock sku", typeNorm
		}
	}

	if reason := evidenceCheck(out, idx); reason != "" {
		return out, warnings, reason, typeNorm
	}

	if isStockOrAdOrPrice(out.RecommendationType) {
		flag := expectedCheckFlag(out.RecommendationType)
		if flag != "" && !constraintBool(out.Constraints, flag) {
			warnings = append(warnings, flag+" is missing")
		}
	}
	return out, warnings, "", typeNorm
}

func oneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

func emptyStringPtr(v *string) bool {
	return v == nil || strings.TrimSpace(*v) == ""
}

func buildContextIndexes(ctx *AIRecommendationContext) contextIndexes {
	idx := contextIndexes{
		alertIDs:      map[int64]struct{}{},
		alertTypeByID: map[int64]string{},
		alertTypes:    map[string]struct{}{},
		skus:          map[int64]struct{}{},
		offerIDs:      map[string]struct{}{},
		productIDs:    map[string]struct{}{},
		campaignIDs:   map[string]struct{}{},
		minPriceBySKU: map[int64]float64{},
		maxPriceBySKU: map[int64]float64{},
		lowStockSKUs:  map[int64]struct{}{},
	}
	if ctx == nil {
		return idx
	}
	for _, a := range ctx.Alerts.TopOpen {
		idx.alertIDs[a.ID] = struct{}{}
		idx.alertTypeByID[a.ID] = a.AlertType
		idx.alertTypes[a.AlertType] = struct{}{}
		if a.EntitySKU != nil {
			idx.skus[*a.EntitySKU] = struct{}{}
			if a.AlertType == "stock_oos_risk" || a.AlertType == "stock_low_coverage" {
				idx.lowStockSKUs[*a.EntitySKU] = struct{}{}
			}
		}
		if a.EntityOfferID != nil {
			idx.offerIDs[strings.TrimSpace(*a.EntityOfferID)] = struct{}{}
		}
		if a.EntityID != nil {
			id := strings.TrimSpace(*a.EntityID)
			idx.productIDs[id] = struct{}{}
			idx.campaignIDs[id] = struct{}{}
			if a.AlertType == "missing_pricing_constraints_for_key_sku" {
				idx.hasMissingConstraintsSignal = true
			}
		}
		if strings.HasPrefix(a.AlertType, "price_") || strings.Contains(a.AlertType, "margin_risk") {
			idx.hasPriceData = true
		}
		if strings.HasPrefix(a.AlertType, "ad_") {
			idx.hasAdData = true
		}
		if strings.HasPrefix(a.AlertType, "stock_") {
			idx.hasStockData = true
		}
	}
	for _, sku := range ctx.Merchandising.TopRevenueSKUs {
		idx.skus[sku.OzonProductID] = struct{}{}
		idx.productIDs[strconv.FormatInt(sku.OzonProductID, 10)] = struct{}{}
		if sku.SKU != nil {
			idx.skus[*sku.SKU] = struct{}{}
		}
		if sku.OfferID != nil {
			idx.offerIDs[strings.TrimSpace(*sku.OfferID)] = struct{}{}
		}
	}
	for _, sku := range ctx.Merchandising.LowStockSKUs {
		idx.skus[sku.OzonProductID] = struct{}{}
		idx.lowStockSKUs[sku.OzonProductID] = struct{}{}
		if sku.SKU != nil {
			idx.skus[*sku.SKU] = struct{}{}
			idx.lowStockSKUs[*sku.SKU] = struct{}{}
		}
		if sku.DaysOfCover != nil && *sku.DaysOfCover <= 7 && sku.SKU != nil {
			idx.lowStockSKUs[*sku.SKU] = struct{}{}
		}
		if sku.StockAvailable <= 0 && sku.SKU != nil {
			idx.lowStockSKUs[*sku.SKU] = struct{}{}
		}
	}
	for _, p := range ctx.Pricing.TopConstrainedSKUs {
		idx.hasPriceData = true
		idx.productIDs[strconv.FormatInt(p.OzonProductID, 10)] = struct{}{}
		if p.SKU != nil {
			idx.skus[*p.SKU] = struct{}{}
			if p.EffectiveMinPrice != nil {
				idx.minPriceBySKU[*p.SKU] = *p.EffectiveMinPrice
			}
			if p.EffectiveMaxPrice != nil {
				idx.maxPriceBySKU[*p.SKU] = *p.EffectiveMaxPrice
			}
		}
		if p.OfferID != nil {
			idx.offerIDs[strings.TrimSpace(*p.OfferID)] = struct{}{}
		}
	}
	for _, c := range ctx.Advertising.TopCampaigns {
		idx.hasAdData = true
		idx.campaignIDs[strconv.FormatInt(c.CampaignExternalID, 10)] = struct{}{}
	}
	return idx
}

func entityExistsInContext(rec AIRecommendationCandidate, idx contextIndexes) bool {
	switch rec.EntityType {
	case "account":
		return true
	case "sku":
		if rec.EntitySKU != nil {
			if _, ok := idx.skus[*rec.EntitySKU]; ok {
				return true
			}
		}
		if rec.EntityOfferID != nil {
			if _, ok := idx.offerIDs[strings.TrimSpace(*rec.EntityOfferID)]; ok {
				return true
			}
		}
		if rec.EntityID != nil {
			if _, ok := idx.productIDs[strings.TrimSpace(*rec.EntityID)]; ok {
				return true
			}
		}
		return false
	case "product", "pricing_constraint":
		if rec.EntityID == nil {
			return false
		}
		_, ok := idx.productIDs[strings.TrimSpace(*rec.EntityID)]
		return ok
	case "campaign":
		if rec.EntityID == nil {
			return false
		}
		_, ok := idx.campaignIDs[strings.TrimSpace(*rec.EntityID)]
		return ok
	default:
		return false
	}
}

func evidenceCheck(rec AIRecommendationCandidate, idx contextIndexes) string {
	switch rec.RecommendationType {
	case "replenish_sku":
		if !idx.hasStockData && len(idx.lowStockSKUs) == 0 {
			return "replenish_sku requires stock evidence"
		}
	case "review_ad_spend", "pause_or_reduce_ads":
		if !idx.hasAdData {
			return rec.RecommendationType + " requires advertising evidence"
		}
	case "avoid_ads_for_low_stock_sku":
		if !idx.hasStockData && len(idx.lowStockSKUs) == 0 && !idx.hasAdData {
			return "avoid_ads_for_low_stock_sku requires stock or advertising evidence"
		}
	case "review_price_margin":
		if !idx.hasPriceData {
			return "price recommendation requires pricing evidence"
		}
	case "review_price_floor":
		if !idx.hasPriceData && !idx.hasMissingConstraintsSignal {
			return "review_price_floor requires pricing or missing-constraints evidence"
		}
	}
	return ""
}

func expectedCheckFlag(recType string) string {
	switch recType {
	case "replenish_sku", "avoid_ads_for_low_stock_sku", "discount_overstock":
		return "stock_checked"
	case "review_ad_spend", "pause_or_reduce_ads":
		return "ads_checked"
	case "review_price_margin", "review_price_floor":
		return "pricing_checked"
	default:
		return ""
	}
}

func extractRecommendationTypeFromRaw(raw any) string {
	switch v := raw.(type) {
	case map[string]any:
		if s, ok := v["recommendation_type"].(string); ok {
			return strings.TrimSpace(s)
		}
	case AIRecommendationCandidate:
		return strings.TrimSpace(v.RecommendationType)
	}
	return ""
}

func isStockOrAdOrPrice(recType string) bool {
	return expectedCheckFlag(recType) != ""
}

func constraintBool(m map[string]any, key string) bool {
	if len(m) == 0 {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func numberFromMetrics(m map[string]any, key string) (float64, bool) {
	if len(m) == 0 {
		return 0, false
	}
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		n, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(t), ",", "."), 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func extractPrices(text string) []float64 {
	matches := numberRegex.FindAllString(text, -1)
	out := make([]float64, 0, len(matches))
	for _, m := range matches {
		n, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", "."), 64)
		if err == nil {
			out = append(out, n)
		}
	}
	return out
}

func capString(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func dedupeInt64(in []int64) []int64 {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(in))
	out := make([]int64, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func dedupeStringsTrimmed(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func downgradeConfidence(initial string, warningCount int) string {
	if warningCount <= 0 {
		return initial
	}
	switch initial {
	case "high":
		return "medium"
	case "medium":
		return "low"
	default:
		return "low"
	}
}

func containsString(in []string, target string) bool {
	for _, v := range in {
		if v == target {
			return true
		}
	}
	return false
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
