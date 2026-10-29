package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	DefaultMaxContextBytes    = 50 * 1024
	DefaultMaxTextLength      = 1500
	DefaultMaxItemsPerSection = 20
	FactContextVersionV1      = "stage_10_fact_context_v1"
)

var forbiddenContextKeys = []string{
	"raw_ai_response", "raw_planner_response", "raw_answer_response", "api_key", "token", "authorization",
	"password", "secret", "session_token", "cookie", "raw_payload", "raw_ozon_payload", "raw_response",
	"sql", "query", "database", "table",
}

type ContextAssembler struct {
	MaxContextBytes    int
	MaxTextLength      int
	MaxItemsPerSection int
}

type sanitizeLimits struct {
	maxTextLength      int
	maxItemsPerSection int
}

func NewContextAssembler() *ContextAssembler {
	return NewContextAssemblerWithLimits(DefaultMaxContextBytes, DefaultMaxItemsPerSection)
}

// NewContextAssemblerWithLimits sets byte and per-section item caps (0 falls back to defaults).
func NewContextAssemblerWithLimits(maxContextBytes, maxItemsPerSection int) *ContextAssembler {
	mb := maxContextBytes
	if mb <= 0 {
		mb = DefaultMaxContextBytes
	}
	mi := maxItemsPerSection
	if mi <= 0 {
		mi = DefaultMaxItemsPerSection
	}
	return &ContextAssembler{
		MaxContextBytes:    mb,
		MaxTextLength:      DefaultMaxTextLength,
		MaxItemsPerSection: mi,
	}
}

func (a *ContextAssembler) Assemble(input AssembleContextInput) (*FactContext, error) {
	if strings.TrimSpace(input.Question) == "" {
		return nil, errors.New("question is required")
	}
	if strings.TrimSpace(string(input.Plan.Intent)) == "" {
		return nil, errors.New("plan intent is required")
	}

	maxBytes := a.MaxContextBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxContextBytes
	}
	maxText := a.MaxTextLength
	if maxText <= 0 {
		maxText = DefaultMaxTextLength
	}
	maxItems := a.MaxItemsPerSection
	if maxItems <= 0 {
		maxItems = DefaultMaxItemsPerSection
	}
	limits := sanitizeLimits{maxTextLength: maxText, maxItemsPerSection: maxItems}

	facts := FactContextFacts{
		Dashboard:             map[string]any{},
		Advertising:           map[string]any{},
		Recommendations:       []map[string]any{},
		RecommendationDetails: []map[string]any{},
		Alerts:                []map[string]any{},
		PriceEconomicsRisks:   []map[string]any{},
		CriticalSKUs:          []map[string]any{},
		StockRisks:            []map[string]any{},
		SKUMetrics:            []map[string]any{},
		SKUContexts:           []map[string]any{},
		CampaignContexts:      []map[string]any{},
		ABCAnalysis:           map[string]any{},
		Other:                 []map[string]any{},
	}
	ctx := &FactContext{
		ContextVersion: FactContextVersionV1,
		GeneratedAt:    time.Now().UTC(),
		Question:       strings.TrimSpace(input.Question),
		Intent:         input.Plan.Intent,
		Language:       strings.TrimSpace(input.Language),
		AsOfDate:       input.AsOfDate,
		Seller: SellerContext{
			SellerAccountID: input.SellerAccountID,
			AccountName:     input.SellerName,
			SecretsIncluded: false,
		},
		ToolPlan:               input.Plan,
		ToolResults:            make([]ToolResult, 0, len(input.ToolResults)),
		Facts:                  facts,
		RelatedAlerts:          []FactAlertReference{},
		RelatedRecommendations: []FactRecommendationReference{},
		Assumptions:            dedupeStrings(input.Plan.Assumptions),
		Limitations:            []string{},
		Freshness:              map[string]any{},
		ContextStats: FactContextStats{
			ToolResultsCount: len(input.ToolResults),
		},
	}

	alertSeen := map[int64]struct{}{}
	recommendSeen := map[int64]struct{}{}

	for _, r := range input.ToolResults {
		sanitizedData := sanitizeContextValue(r.Data, limits)
		sanitizedArgs := map[string]any{}
		if argsMap, ok := sanitizeContextValue(r.Args, limits).(map[string]any); ok {
			sanitizedArgs = argsMap
		}
		copyResult := ToolResult{
			Name:        r.Name,
			Args:        sanitizedArgs,
			Data:        sanitizedData,
			Error:       r.Error,
			Limitations: dedupeStrings(append([]string(nil), r.Limitations...)),
		}
		ctx.ToolResults = append(ctx.ToolResults, copyResult)

		if copyResult.Error != nil && strings.TrimSpace(*copyResult.Error) != "" {
			ctx.ContextStats.FailedToolsCount++
			ctx.Limitations = append(ctx.Limitations, fmt.Sprintf("Tool %s failed: %s", copyResult.Name, strings.TrimSpace(*copyResult.Error)))
		}
		ctx.Limitations = append(ctx.Limitations, copyResult.Limitations...)
		a.routeToolResult(ctx, copyResult, alertSeen, recommendSeen)
	}

	ctx.Limitations = dedupeStrings(ctx.Limitations)
	if len(ctx.Facts.Dashboard) == 0 && len(ctx.Facts.Recommendations) == 0 && len(ctx.Facts.Alerts) == 0 &&
		len(ctx.Facts.CriticalSKUs) == 0 && len(ctx.Facts.StockRisks) == 0 && len(ctx.Facts.SKUMetrics) == 0 &&
		len(ctx.Facts.SKUContexts) == 0 && len(ctx.Facts.CampaignContexts) == 0 && len(ctx.Facts.ABCAnalysis) == 0 &&
		len(ctx.Facts.PriceEconomicsRisks) == 0 && len(ctx.Facts.RecommendationDetails) == 0 && len(ctx.Facts.Advertising) == 0 {
		ctx.Limitations = append(ctx.Limitations, "No factual data was available for this question.")
	}
	ctx.Limitations = dedupeStrings(ctx.Limitations)
	ctx.ContextStats.TotalItemsIncluded = countFactItems(ctx.Facts)
	ctx.ContextStats.EstimatedContextBytes = estimateJSONSize(ctx)

	if ctx.ContextStats.EstimatedContextBytes > maxBytes {
		a.shrinkContext(ctx)
		ctx.ContextStats.Truncated = true
		ctx.ContextStats.TruncationReason = "max_context_bytes_exceeded"
		ctx.Limitations = dedupeStrings(append(ctx.Limitations, "Context was truncated to fit size limits."))
		ctx.ContextStats.TotalItemsIncluded = countFactItems(ctx.Facts)
		ctx.ContextStats.EstimatedContextBytes = estimateJSONSize(ctx)
	}

	sort.Slice(ctx.RelatedAlerts, func(i, j int) bool { return ctx.RelatedAlerts[i].ID < ctx.RelatedAlerts[j].ID })
	sort.Slice(ctx.RelatedRecommendations, func(i, j int) bool { return ctx.RelatedRecommendations[i].ID < ctx.RelatedRecommendations[j].ID })
	ctx.ContextTruncated = ctx.ContextStats.Truncated
	ctx.ContextTruncationReason = ctx.ContextStats.TruncationReason
	return ctx, nil
}

func (a *ContextAssembler) routeToolResult(ctx *FactContext, result ToolResult, alertSeen map[int64]struct{}, recommendSeen map[int64]struct{}) {
	asMap, _ := result.Data.(map[string]any)
	switch result.Name {
	case ToolGetDashboardSummary:
		ctx.Facts.Dashboard = asMap
		ctx.Freshness["dashboard"] = pickFreshness(asMap)
	case ToolGetOpenRecommendations:
		items := extractItems(asMap)
		ctx.Facts.Recommendations = append(ctx.Facts.Recommendations, items...)
		for _, item := range items {
			addRecommendationRef(ctx, item, recommendSeen)
		}
	case ToolGetRecommendationDetail:
		ctx.Facts.RecommendationDetails = append(ctx.Facts.RecommendationDetails, asMap)
		if rec, ok := asMap["recommendation"].(map[string]any); ok {
			addRecommendationRef(ctx, rec, recommendSeen)
		}
		for _, a := range extractAnyMaps(asMap["related_alerts"]) {
			addAlertRef(ctx, a, alertSeen)
		}
	case ToolGetOpenAlerts, ToolGetAlertsByGroup:
		items := extractItems(asMap)
		ctx.Facts.Alerts = append(ctx.Facts.Alerts, items...)
		for _, item := range items {
			addAlertRef(ctx, item, alertSeen)
		}
	case ToolGetPriceEconomicsRisks:
		items := extractItems(asMap)
		ctx.Facts.PriceEconomicsRisks = append(ctx.Facts.PriceEconomicsRisks, items...)
		for _, item := range items {
			addAlertRef(ctx, item, alertSeen)
		}
	case ToolGetCriticalSKUs:
		ctx.Facts.CriticalSKUs = append(ctx.Facts.CriticalSKUs, extractItems(asMap)...)
	case ToolGetStockRisks:
		ctx.Facts.StockRisks = append(ctx.Facts.StockRisks, extractItems(asMap)...)
	case ToolGetAdvertisingAnalytics:
		ctx.Facts.Advertising = asMap
		ctx.Freshness["advertising"] = pickFreshness(asMap)
	case ToolGetSKUMetrics:
		ctx.Facts.SKUMetrics = append(ctx.Facts.SKUMetrics, extractItems(asMap)...)
		ctx.Freshness["sku_metrics"] = pickFreshness(asMap)
	case ToolGetSKUContext:
		ctx.Facts.SKUContexts = append(ctx.Facts.SKUContexts, asMap)
		for _, a := range extractAnyMaps(asMap["alerts"]) {
			addAlertRef(ctx, a, alertSeen)
		}
		for _, rec := range extractAnyMaps(asMap["recommendations"]) {
			addRecommendationRef(ctx, rec, recommendSeen)
		}
	case ToolGetCampaignContext:
		ctx.Facts.CampaignContexts = append(ctx.Facts.CampaignContexts, asMap)
		for _, a := range extractAnyMaps(asMap["alerts"]) {
			addAlertRef(ctx, a, alertSeen)
		}
		for _, rec := range extractAnyMaps(asMap["recommendations"]) {
			addRecommendationRef(ctx, rec, recommendSeen)
		}
	case ToolRunABCAnalysis:
		ctx.Facts.ABCAnalysis = asMap
		ctx.Freshness["abc_analysis"] = pickFreshness(asMap)
	default:
		ctx.Facts.Other = append(ctx.Facts.Other, asMap)
		ctx.Limitations = append(ctx.Limitations, fmt.Sprintf("Tool result from unknown tool was not included: %s", result.Name))
	}
}

func sanitizeContextValue(v any, limits sanitizeLimits) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, raw := range t {
			if isForbiddenKey(k) {
				continue
			}
			sv := sanitizeContextValue(raw, limits)
			if sv == nil {
				continue
			}
			out[k] = sv
		}
		return out
	case []map[string]any:
		maxItems := minContextInt(len(t), limits.maxItemsPerSection)
		out := make([]map[string]any, 0, maxItems)
		for _, item := range t[:maxItems] {
			if cast, ok := sanitizeContextValue(item, limits).(map[string]any); ok {
				out = append(out, cast)
			}
		}
		return out
	case []any:
		maxItems := minContextInt(len(t), limits.maxItemsPerSection)
		out := make([]any, 0, maxItems)
		for _, item := range t[:maxItems] {
			out = append(out, sanitizeContextValue(item, limits))
		}
		return out
	case string:
		if len(t) <= limits.maxTextLength {
			return t
		}
		return t[:limits.maxTextLength] + "...[truncated]"
	case int, int32, int64, float32, float64, bool, nil, ChatIntent:
		return t
	default:
		// Keep data JSON-friendly.
		raw, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		var out any
		if err := json.Unmarshal(raw, &out); err != nil {
			return string(raw)
		}
		return sanitizeContextValue(out, limits)
	}
}

func isForbiddenKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, bad := range forbiddenContextKeys {
		if strings.Contains(k, bad) {
			return true
		}
	}
	return false
}

func extractAnyMaps(v any) []map[string]any {
	switch t := v.(type) {
	case []map[string]any:
		return t
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, item := range t {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return []map[string]any{}
	}
}

func extractItems(data map[string]any) []map[string]any {
	if data == nil {
		return []map[string]any{}
	}
	if raw, ok := data["items"]; ok {
		return extractAnyMaps(raw)
	}
	return []map[string]any{}
}

func addAlertRef(ctx *FactContext, src map[string]any, seen map[int64]struct{}) {
	id, ok := readInt64(src["id"])
	if !ok {
		return
	}
	if _, exists := seen[id]; exists {
		return
	}
	seen[id] = struct{}{}
	ref := FactAlertReference{
		ID:            id,
		AlertType:     readString(src["alert_type"]),
		Group:         firstNonEmpty(readString(src["alert_group"]), readString(src["group"])),
		Severity:      readString(src["severity"]),
		Urgency:       readString(src["urgency"]),
		EntityType:    readString(src["entity_type"]),
		EntitySKU:     readInt64Ptr(src["entity_sku"]),
		EntityOfferID: readStringPtr(src["entity_offer_id"]),
		Title:         readString(src["title"]),
	}
	ctx.RelatedAlerts = append(ctx.RelatedAlerts, ref)
}

func addRecommendationRef(ctx *FactContext, src map[string]any, seen map[int64]struct{}) {
	id, ok := readInt64(src["id"])
	if !ok {
		return
	}
	if _, exists := seen[id]; exists {
		return
	}
	seen[id] = struct{}{}
	ref := FactRecommendationReference{
		ID:                 id,
		RecommendationType: readString(src["recommendation_type"]),
		PriorityLevel:      readString(src["priority_level"]),
		Urgency:            readString(src["urgency"]),
		ConfidenceLevel:    readString(src["confidence_level"]),
		EntityType:         readString(src["entity_type"]),
		EntitySKU:          readInt64Ptr(src["entity_sku"]),
		EntityOfferID:      readStringPtr(src["entity_offer_id"]),
		Title:              readString(src["title"]),
		RecommendedAction:  readString(src["recommended_action"]),
	}
	ctx.RelatedRecommendations = append(ctx.RelatedRecommendations, ref)
}

func pickFreshness(data map[string]any) map[string]any {
	keys := []string{"as_of_date", "as_of_date_source", "data_freshness", "last_successful_update", "generated_at", "date_from", "date_to", "period"}
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := data[k]; ok {
			out[k] = v
		}
	}
	return out
}

func readString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func readStringPtr(v any) *string {
	s := readString(v)
	if s == "" {
		return nil
	}
	return &s
}

func readInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case float64:
		if math.Mod(t, 1) == 0 {
			return int64(t), true
		}
		return 0, false
	default:
		return 0, false
	}
}

func readInt64Ptr(v any) *int64 {
	if id, ok := readInt64(v); ok {
		return &id
	}
	return nil
}

func dedupeStrings(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || slices.Contains(out, item) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func estimateJSONSize(v any) int {
	raw, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return len(raw)
}

func countFactItems(f FactContextFacts) int {
	total := 0
	if len(f.Dashboard) > 0 {
		total++
	}
	if len(f.Advertising) > 0 {
		total++
	}
	if len(f.ABCAnalysis) > 0 {
		total++
	}
	total += len(f.Recommendations)
	total += len(f.RecommendationDetails)
	total += len(f.Alerts)
	total += len(f.PriceEconomicsRisks)
	total += len(f.CriticalSKUs)
	total += len(f.StockRisks)
	total += len(f.SKUMetrics)
	total += len(f.SKUContexts)
	total += len(f.CampaignContexts)
	total += len(f.Other)
	return total
}

func (a *ContextAssembler) shrinkContext(ctx *FactContext) {
	ctx.Facts.Recommendations = shrinkSlice(ctx.Facts.Recommendations, 5)
	ctx.Facts.Alerts = shrinkSlice(ctx.Facts.Alerts, 10)
	ctx.Facts.SKUMetrics = shrinkSlice(ctx.Facts.SKUMetrics, 10)
	ctx.Facts.CriticalSKUs = shrinkSlice(ctx.Facts.CriticalSKUs, 10)
	ctx.Facts.StockRisks = shrinkSlice(ctx.Facts.StockRisks, 10)
	ctx.Facts.PriceEconomicsRisks = shrinkSlice(ctx.Facts.PriceEconomicsRisks, 10)
	ctx.Facts.CampaignContexts = shrinkSlice(ctx.Facts.CampaignContexts, 5)
	ctx.Facts.SKUContexts = shrinkSlice(ctx.Facts.SKUContexts, 5)
	if campaigns, ok := ctx.Facts.Advertising["campaigns"].([]any); ok && len(campaigns) > 10 {
		ctx.Facts.Advertising["campaigns"] = campaigns[:10]
	}
	if rows, ok := ctx.Facts.ABCAnalysis["rows"].([]any); ok && len(rows) > 20 {
		ctx.Facts.ABCAnalysis["rows"] = rows[:20]
	}
	ctx.ToolResults = shrinkToolResults(ctx.ToolResults, 10)
}

func shrinkSlice[T any](items []T, n int) []T {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func shrinkToolResults(items []ToolResult, n int) []ToolResult {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func minContextInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
