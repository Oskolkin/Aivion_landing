package dashboardhome

import "strings"

const maxSellerFacingFactIDs = 8
const maxRiskSKUExamples = 3

// sanitizeRiskDriversForSellerFacing strips debug grounding from seller-facing API responses.
func sanitizeRiskDriversForSellerFacing(drivers []RiskDriver, resp *Response) []RiskDriver {
	if len(drivers) == 0 {
		return drivers
	}
	out := make([]RiskDriver, 0, len(drivers))
	for _, d := range drivers {
		d.Evidence = sanitizeRiskDriverEvidence(d, resp)
		d.Title = normalizeRiskDriverSellerFacingTitle(d)
		out = append(out, d)
	}
	return out
}

func sanitizeRiskDriverEvidence(driver RiskDriver, resp *Response) map[string]any {
	if driver.Evidence == nil {
		return map[string]any{}
	}
	evidence := make(map[string]any, len(driver.Evidence))
	for k, v := range driver.Evidence {
		if k == "fact_ids" {
			continue
		}
		evidence[k] = v
	}
	if factIDs, ok := driver.Evidence["fact_ids"].([]string); ok && len(factIDs) > 0 {
		evidence["grounding_fact_count"] = len(factIDs)
		if len(factIDs) <= maxSellerFacingFactIDs {
			evidence["fact_ids"] = append([]string(nil), factIDs...)
		}
	} else if factIDsAny, ok := driver.Evidence["fact_ids"].([]any); ok && len(factIDsAny) > 0 {
		evidence["grounding_fact_count"] = len(factIDsAny)
	}
	if driver.ID == "driver-stock" || driver.Category == "stock" {
		evidence["risk_sku_examples"] = buildRiskSKUExamples(resp, maxRiskSKUExamples)
	}
	return compactEvidenceMap(evidence)
}

func buildRiskSKUExamples(resp *Response, limit int) []map[string]any {
	if resp == nil || limit <= 0 {
		return nil
	}
	cards := resp.SKUFocus.StockRisk
	if len(cards) == 0 {
		cards = resp.SKUFocus.OrderDrop
	}
	out := make([]map[string]any, 0, minInt(len(cards), limit))
	for i := 0; i < len(cards) && len(out) < limit; i++ {
		c := cards[i]
		item := map[string]any{
			"name": strings.TrimSpace(c.ProductName),
		}
		if c.SKU != nil {
			item["sku"] = *c.SKU
		}
		if c.CurrentStock != nil {
			item["stock"] = *c.CurrentStock
		}
		if c.CurrentOrders != nil {
			item["orders_7d"] = *c.CurrentOrders
		}
		out = append(out, item)
	}
	return out
}

func compactEvidenceMap(evidence map[string]any) map[string]any {
	if len(evidence) == 0 {
		return map[string]any{}
	}
	return evidence
}

// normalizeCategoryDisplayName replaces generic placeholder category names.
func normalizeCategoryDisplayName(categoryID, name string) string {
	name = strings.TrimSpace(name)
	if name == "" || isGenericCategoryName(name) {
		if alias := categoryDisplayNameAlias(categoryID); alias != "" {
			return alias
		}
	}
	if isGenericCategoryName(name) {
		return name
	}
	return name
}

func isGenericCategoryName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}
	if categoryIDNamePattern.MatchString(name) {
		return true
	}
	if strings.HasPrefix(strings.ToLower(name), "category ") {
		return true
	}
	return false
}

func categoryDisplayNameAlias(categoryID string) string {
	switch strings.TrimSpace(categoryID) {
	case "17990001":
		return "Товары для ванной"
	default:
		return ""
	}
}

// normalizeCategorySummaryCardsAIOnly drops any AI-only category card
// missing a valid category_score before display. This is a read-path
// safety net, not the primary enforcement point: for Unified Analysis
// publications, outputvalidator.validateCategorySummaries and
// bundlevalidator.checkCategoryAnalysis already reject/block a publication
// whose category_summaries entries are incomplete (missing category_score/
// headline) whenever the Pack/facts show categories exist, so a
// successfully published run should never reach this filter with an
// incomplete card. If a card silently disappears here in production, treat
// it as a signal that the publish-time gate was bypassed (e.g. an older
// publication predating that rule) rather than relying on this function to
// catch it going forward.
func normalizeCategorySummaryCardsAIOnly(cards []CategorySummaryCard) []CategorySummaryCard {
	if len(cards) == 0 {
		return cards
	}
	out := make([]CategorySummaryCard, 0, len(cards))
	for _, c := range cards {
		c.CategoryName = normalizeCategoryDisplayName(c.CategoryID, c.CategoryName)
		if c.CategoryScore == nil || *c.CategoryScore < 0 || *c.CategoryScore > categoryScoreMax {
			continue
		}
		c.ScoreMax = categoryScoreMax
		if c.HealthScore == nil {
			v := *c.CategoryScore * 10
			c.HealthScore = &v
		}
		if strings.TrimSpace(c.Severity) == "" || !isCategoryScoreSeverityConsistent(*c.CategoryScore, c.Severity) {
			c.Severity = severityForCategoryScore10(*c.CategoryScore)
		}
		out = append(out, c)
	}
	return out
}

func normalizeCategorySummaryCards(cards []CategorySummaryCard) []CategorySummaryCard {
	if len(cards) == 0 {
		return cards
	}
	out := make([]CategorySummaryCard, 0, len(cards))
	for _, c := range cards {
		c.CategoryName = normalizeCategoryDisplayName(c.CategoryID, c.CategoryName)
		out = append(out, c)
	}
	return finalizeCategorySummaryScores(out)
}

func enrichSKUFocusSlowTurnover(resp *Response) {
	if resp == nil {
		return
	}
	resp.SKUFocus.SlowTurnover = append([]SKUFocusCard(nil), resp.SKUFocus.Surplus...)
}

func reconcileSurplusStockCoverageLabelsV2(resp *Response) {
	if resp == nil {
		return
	}
	slowRisk := len(resp.SKUFocus.SlowTurnover)
	if slowRisk == 0 {
		slowRisk = len(resp.SKUFocus.Surplus)
	}
	for i := range resp.AIDiagnostics.StockCoverage.Segments {
		if resp.AIDiagnostics.StockCoverage.Segments[i].Key == "surplus" {
			resp.AIDiagnostics.StockCoverage.Segments[i].Label = "SURPLUS (Ozon оборачиваемость)"
			break
		}
	}
	if slowRisk > 0 && strings.TrimSpace(resp.AIDiagnostics.StockCoverage.Message) == "" {
		resp.AIDiagnostics.StockCoverage.Message = formatSlowTurnoverStockMessage(slowRisk)
		resp.AIDiagnostics.StockCoverage.SlowTurnoverLink = &StockCoverageSKUFocusLink{
			Label:  "Открыть SKU с риском хранения",
			Target: "sku-focus-slow-turnover",
		}
	}
}

func formatSlowTurnoverStockMessage(count int) string {
	return "Медленная оборачиваемость / риск хранения: " + formatInt(int64(count)) + " SKU с остатками без достаточного спроса."
}
