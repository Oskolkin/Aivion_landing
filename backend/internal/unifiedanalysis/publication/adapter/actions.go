package adapter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// ProjectedAction is one seller-facing action ready to persist through
// actions.UpsertActionItem, plus the bookkeeping fields needed to populate
// unified_publication_action_map (analysis_action_id, is_top, is_recommended,
// sort_order) once the action_items.id is known.
type ProjectedAction struct {
	AnalysisActionID string
	IsTop            bool
	IsRecommended    bool
	SortOrder        int
	ExecutionTier    string
	Params           actions.UpsertActionItemParams

	// StepCount/CompletionCriteria surface the two action-completeness
	// signals bundlevalidator.checkActionDetailCompleteness needs, without
	// requiring it to decode Params.PayloadJSON.
	StepCount          int
	CompletionCriteria string
}

// EntityFact carries optional Pack-backed metrics for one affected entity.
type EntityFact struct {
	Name                   string
	AvailableStock         *int64
	Orders7D               *int64
	CoverageDays7D         *float64
	InTransitUnits         *int64
	TargetCoverageDays     *float64
	TargetCoverageSource   string
	RecommendedSupplyUnits *int64
	EstimatedStockoutDate  *string // YYYY-MM-DD
	RecommendedOrderDate   *string // YYYY-MM-DD
	SurplusExcessUnits     *int64
	StockRiskClass         string
}

// EntityFactsLookup resolves Pack metrics for an action entity id.
// Nil lookup is allowed — object_actions then carry identity without metrics.
type EntityFactsLookup func(scopeType, entityID string) (EntityFact, bool)

// EntityFactsLookupFromDashboardFacts builds a lookup from the immutable
// dashboard facts SKU index (already loaded for publication). Entity IDs are
// matched by Pack sku_id, ozon_product_id, or offer_id.
func EntityFactsLookupFromDashboardFacts(facts dashboardfacts.DashboardFactsV1) EntityFactsLookup {
	lookup := buildSKUFactsLookup(facts.SKUFactsIndex)
	campaignByID := make(map[string]dashboardfacts.AdCampaignDisplayRow)
	for _, row := range facts.AdEfficiency.HighDRRCampaigns {
		campaignByID[strconv.FormatInt(row.CampaignExternalID, 10)] = row
	}
	for _, row := range facts.AdEfficiency.NoOrderCampaigns {
		campaignByID[strconv.FormatInt(row.CampaignExternalID, 10)] = row
	}
	return func(scopeType, entityID string) (EntityFact, bool) {
		scopeType = strings.ToLower(strings.TrimSpace(scopeType))
		entityID = strings.TrimSpace(entityID)
		if entityID == "" {
			return EntityFact{}, false
		}
		switch scopeType {
		case "sku":
			rawID := entityID
			entityID = stripEntityPrefix(entityID)
			var skuPtr *int64
			if id, err := strconv.ParseInt(entityID, 10, 64); err == nil {
				skuPtr = &id
			}
			offer := entityID
			rec, ok := lookup.find(skuPtr, &offer)
			if !ok && rawID != entityID {
				rawOffer := rawID
				rec, ok = lookup.find(skuPtr, &rawOffer)
			}
			if !ok && skuPtr != nil {
				rec, ok = lookup.byOzonProductID[*skuPtr]
			}
			if !ok {
				return EntityFact{}, false
			}
			stock := int64(rec.StockAvailable)
			orders := rec.Orders7d
			fact := EntityFact{
				AvailableStock: &stock,
				Orders7D:       &orders,
				CoverageDays7D: rec.DaysOfCover,
			}
			// OOS with demand must show coverage=0, not "missing".
			if stock == 0 && orders > 0 && fact.CoverageDays7D == nil {
				zero := 0.0
				fact.CoverageDays7D = &zero
			}
			if fact.CoverageDays7D == nil && orders > 0 {
				fact.CoverageDays7D = stockpolicy.CoverageDays7D(stock, orders)
			}
			if rec.ProductName != nil {
				fact.Name = strings.TrimSpace(*rec.ProductName)
			}
			enrichStockDecisionMetrics(&fact, rec)
			return fact, true
		case "campaign":
			entityID = stripEntityPrefix(entityID)
			row, ok := campaignByID[entityID]
			if !ok {
				return EntityFact{}, false
			}
			orders := row.Orders
			return EntityFact{
				Name:     row.CampaignName,
				Orders7D: &orders,
			}, true
		default:
			return EntityFact{}, false
		}
	}
}

func stripEntityPrefix(entityID string) string {
	id := strings.TrimSpace(entityID)
	if i := strings.IndexByte(id, ':'); i >= 0 {
		prefix := strings.ToLower(id[:i])
		if prefix == "sku" || prefix == "campaign" || prefix == "product" {
			return strings.TrimSpace(id[i+1:])
		}
	}
	return id
}

func enrichStockDecisionMetrics(fact *EntityFact, rec dashboardfacts.SKUFactRecord) {
	if fact == nil || fact.AvailableStock == nil {
		return
	}
	stock := *fact.AvailableStock
	var orders int64
	if fact.Orders7D != nil {
		orders = *fact.Orders7D
	}
	avgDaily := 0.0
	if orders > 0 {
		avgDaily = float64(orders) / 7.0
	}
	inTransit := int64(0)
	fact.InTransitUnits = &inTransit
	target, source := stockpolicy.DefaultTargetCoverageDays()
	fact.TargetCoverageDays = &target
	fact.TargetCoverageSource = source

	if avgDaily > 0 {
		qty := stockpolicy.RecommendedSupplyUnits(stock, inTransit, 0, avgDaily, target)
		fact.RecommendedSupplyUnits = &qty
		asOf := time.Now().UTC().Truncate(24 * time.Hour)
		if stock == 0 {
			d := asOf.Format("2006-01-02")
			fact.EstimatedStockoutDate = &d
			fact.RecommendedOrderDate = &d
		} else if stockout := stockpolicy.EstimatedStockoutDate(asOf, stock, avgDaily); stockout != nil {
			d := stockout.Format("2006-01-02")
			fact.EstimatedStockoutDate = &d
			if order := stockpolicy.RecommendedOrderDate(asOf, stockout, stockpolicy.DefaultLeadTimeDays); order != nil {
				od := order.Format("2006-01-02")
				fact.RecommendedOrderDate = &od
			}
		}
	}

	seg := ""
	if fact.CoverageDays7D != nil {
		seg = stockpolicy.ClassifyCoverageSegment(stockpolicy.Input{
			AvailableStock:  stock,
			HasStockData:    true,
			CoverageDays:    fact.CoverageDays7D,
			Orders7D:        orders,
			HasSalesHistory: true,
		})
	} else if stock == 0 {
		seg = stockpolicy.SegmentOutOfStock
	}
	fact.StockRiskClass = seg
	if seg == stockpolicy.SegmentSurplus && avgDaily > 0 {
		excess := stockpolicy.SurplusExcessUnits(stock, avgDaily, stockpolicy.CoverageSurplusMin)
		fact.SurplusExcessUnits = &excess
	}
	_ = rec
}

// ProjectActions maps the unified output's actions[] into ProjectedAction rows.
// Required-tier actions are sorted before recommended; relative order within
// each tier is preserved. packID is shortened into the action_key so keys stay
// human-scannable while remaining stable across re-publishes of the same run.
//
// facts may be nil; when provided, object_actions are hydrated with Pack metrics
// so the seller UI does not show empty "—" cells for known stock/orders/coverage.
func ProjectActions(out *contract.UnifiedAnalysisOutputV1, runID int64, sellerAccountID int64, packID string, metricDate time.Time, facts EntityFactsLookup) []ProjectedAction {
	if out == nil {
		return nil
	}
	topSet := stringSet(out.Dashboard.TopActionIDs)
	recSet := stringSet(out.Dashboard.RecommendedActionIDs)
	packIDShort := shortPackID(packID)

	result := make([]ProjectedAction, 0, len(out.Actions))
	for i, a := range out.Actions {
		params := buildActionParams(a, runID, sellerAccountID, packIDShort, metricDate, facts)
		result = append(result, ProjectedAction{
			AnalysisActionID:   a.AnalysisActionID,
			IsTop:              topSet[a.AnalysisActionID],
			IsRecommended:      recSet[a.AnalysisActionID],
			SortOrder:          i,
			ExecutionTier:      strings.TrimSpace(a.ExecutionTier),
			Params:             params,
			StepCount:          len(a.Steps),
			CompletionCriteria: a.CompletionCriteria,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		ri := result[i].ExecutionTier == contract.ExecutionTierRequired
		rj := result[j].ExecutionTier == contract.ExecutionTierRequired
		if ri == rj {
			return false
		}
		return ri
	})
	for i := range result {
		result[i].SortOrder = i
	}
	return result
}

func buildActionParams(a contract.UnifiedActionV1, runID, sellerAccountID int64, packIDShort string, metricDate time.Time, facts EntityFactsLookup) actions.UpsertActionItemParams {
	actionType := actions.ActionType(a.Type)
	category := actions.ActionCategory(a.Category)
	priority := actions.ActionPriority(a.Priority)

	affected := projectAffectedEntities(a.Scope)
	entityCount := countAffectedEntities(affected)

	item := actions.ActionItem{
		ActionKey:                ActionKeyFor(packIDShort, a.AnalysisActionID),
		Type:                     actionType,
		Category:                 category,
		Priority:                 priority,
		Status:                   actions.ActionStatusOpen,
		ExecutionTier:            strings.TrimSpace(a.ExecutionTier),
		Title:                    a.Title,
		Summary:                  a.Summary,
		Reason:                   a.Problem,
		NextStep:                 firstStepText(a.Steps),
		ExpectedEffect:           a.ExpectedEffect,
		Limitations:              append([]string{}, a.Limitations...),
		AffectedEntities:         affected,
		AffectedObjectsTotal:     entityCount,
		AffectedObjectsDisplayed: entityCount,
		AffectedObjectsTruncated: false,
		OperationalGoal:          a.OperationalGoal,
		ScopeType:                a.Scope.ScopeType,
		CompletionCriteria:       a.CompletionCriteria,
		CompletionConditions:     projectCompletionConditions(a.CompletionConditions),
		MetricDate:               metricDate.Format("2006-01-02"),
	}
	if entityCount > 0 {
		item.AffectedObjectsSummary = fmt.Sprintf("%d", entityCount)
	}
	item.OperationalDetail = projectOperationalDetail(a, facts)

	evidenceFactIDs := dedupeNonEmpty(a.EvidenceFactIDs)
	sourceSignalIDs := dedupeNonEmpty(a.SourceSignalIDs)
	evidence := buildActionEvidence(a, evidenceFactIDs)
	item.Evidence = evidence

	evidenceJSON, _ := json.Marshal(evidence)
	limitationsJSON, _ := json.Marshal(item.Limitations)
	ctaJSON, _ := json.Marshal([]actions.ActionCTA{})
	affectedEntitiesJSON, _ := json.Marshal(item.AffectedEntities)
	payloadJSON := actions.MarshalActionItemPayloadJSON(item)

	evidenceFactIDsJSON := marshalStringList(evidenceFactIDs)
	sourceSignalIDsJSON := marshalStringList(sourceSignalIDs)

	return actions.UpsertActionItemParams{
		SellerAccountID:      sellerAccountID,
		RunID:                runID,
		ActionKey:            item.ActionKey,
		Type:                 actionType,
		Category:             category,
		Priority:             priority,
		Title:                item.Title,
		Summary:              item.Summary,
		Reason:               item.Reason,
		NextStep:             item.NextStep,
		ExpectedEffect:       item.ExpectedEffect,
		EvidenceJSON:         evidenceJSON,
		LimitationsJSON:      limitationsJSON,
		CTAJSON:              ctaJSON,
		AffectedEntitiesJSON: affectedEntitiesJSON,
		SourceCandidateIDs:   sourceSignalIDsJSON,
		EvidenceFactIDs:      evidenceFactIDsJSON,
		PayloadJSON:          payloadJSON,
		MetricDate:           metricDate,
	}
}

// buildActionEvidence turns the AI-supplied evidence_fact_ids into concrete
// actions.ActionEvidence rows. Each fact_id becomes a fact-reference
// evidence entry; the action's evidence_summary is attached as the
// description of the first entry so the seller-facing card keeps its
// human-readable grounding.
func buildActionEvidence(a contract.UnifiedActionV1, evidenceFactIDs []string) []actions.ActionEvidence {
	if len(evidenceFactIDs) == 0 {
		return []actions.ActionEvidence{}
	}
	summary := strings.TrimSpace(a.EvidenceSummary)
	out := make([]actions.ActionEvidence, 0, len(evidenceFactIDs))
	for i, factID := range evidenceFactIDs {
		ev := actions.ActionEvidence{
			Kind:  "fact",
			Label: factID,
			Value: factID,
		}
		if i == 0 && summary != "" {
			ev.Description = summary
		}
		out = append(out, ev)
	}
	return out
}

func dedupeNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, v := range in {
		v = strings.TrimSpace(v)
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

func marshalStringList(in []string) []byte {
	if len(in) == 0 {
		return []byte("[]")
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return []byte("[]")
	}
	return raw
}

// ActionKeyFor builds the stable action_key for a unified action. It is
// deterministic per (pack, analysis_action_id) so re-publishing the same
// pack/action upserts the existing action_items row instead of duplicating it.
func ActionKeyFor(packIDShort, analysisActionID string) string {
	return fmt.Sprintf("unified:%s:%s", packIDShort, analysisActionID)
}

func shortPackID(packID string) string {
	packID = strings.TrimSpace(packID)
	if packID == "" {
		return "unknown"
	}
	if len(packID) > 12 {
		return packID[:12]
	}
	return packID
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, v := range values {
		out[v] = true
	}
	return out
}

func firstStepText(steps []contract.UnifiedActionStepV1) string {
	if len(steps) == 0 {
		return ""
	}
	first := steps[0]
	if strings.TrimSpace(first.Description) != "" {
		return first.Description
	}
	return first.Title
}

func projectAffectedEntities(scope contract.UnifiedActionScopeV1) actions.ActionAffectedEntities {
	switch scope.ScopeType {
	case "sku":
		skus := make([]actions.ActionSKURef, 0, len(scope.EntityIDs))
		for _, id := range scope.EntityIDs {
			skus = append(skus, actions.ActionSKURef{SKU: id})
		}
		return actions.ActionAffectedEntities{SKUs: skus}
	case "campaign":
		campaigns := make([]actions.ActionCampaignRef, 0, len(scope.EntityIDs))
		for _, id := range scope.EntityIDs {
			campaigns = append(campaigns, actions.ActionCampaignRef{CampaignID: id})
		}
		return actions.ActionAffectedEntities{Campaigns: campaigns}
	default:
		return actions.ActionAffectedEntities{}
	}
}

func projectOperationalDetail(a contract.UnifiedActionV1, facts EntityFactsLookup) *actions.OperationalDetail {
	steps := make([]actions.OperationalStep, 0, len(a.Steps))
	for _, s := range a.Steps {
		steps = append(steps, actions.OperationalStep{
			StepID:         fmt.Sprintf("step-%d", s.Order),
			Title:          s.Title,
			Instruction:    s.Description,
			TargetEntities: targetEntitiesFor(a.Scope.ScopeType, s.EntityIDs),
		})
	}
	objectActions := projectObjectActions(a.Scope, facts)
	whyFacts := importanceFactsFromProblem(a.Problem)
	return &actions.OperationalDetail{
		Status:        "available",
		FormatVersion: actions.OperationalDetailFormatVersion,
		Why: actions.OperationalWhy{
			Headline: a.Title,
			// WhyNow is the seller-facing "Ситуация" body — prefer it over a
			// short summary that often duplicates the title.
			Text:  firstNonEmpty(a.WhyNow, a.Summary),
			Facts: whyFacts,
		},
		ActionPlan: actions.OperationalActionPlan{
			Headline: "План действий",
			Steps:    steps,
		},
		ObjectActions:     objectActions,
		HiddenLimitations: append([]string{}, a.Limitations...),
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func importanceFactsFromProblem(problem string) []actions.OperationalFact {
	problem = strings.TrimSpace(problem)
	if problem == "" {
		return nil
	}
	parts := splitImportanceBullets(problem)
	if len(parts) == 0 {
		return []actions.OperationalFact{{
			FactID: "importance",
			Label:  "",
			Value:  problem,
		}}
	}
	out := make([]actions.OperationalFact, 0, len(parts))
	for i, part := range parts {
		out = append(out, actions.OperationalFact{
			FactID: fmt.Sprintf("importance_%d", i+1),
			Label:  "",
			Value:  part,
		})
	}
	return out
}

// splitImportanceBullets turns multi-sentence / bullet problem text into
// discrete importance facts. Single short paragraphs stay as one fact.
func splitImportanceBullets(text string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	// Explicit bullets / newlines first.
	lines := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '\n' || r == ';'
	})
	var bullets []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "•*-–— \t")
		if line == "" {
			continue
		}
		bullets = append(bullets, line)
	}
	if len(bullets) >= 2 {
		return bullets
	}
	// Fall back to sentence split when the paragraph is long enough.
	if utf8RuneCountApprox(trimmed) < 80 {
		return []string{trimmed}
	}
	sentences := splitSentencesRU(trimmed)
	if len(sentences) >= 2 {
		return sentences
	}
	return []string{trimmed}
}

func splitSentencesRU(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func utf8RuneCountApprox(s string) int {
	return len([]rune(s))
}

func countAffectedEntities(entities actions.ActionAffectedEntities) int {
	n := len(entities.SKUs) + len(entities.Campaigns)
	if entities.Funnel != nil {
		n++
	}
	return n
}

func projectObjectActions(scope contract.UnifiedActionScopeV1, facts EntityFactsLookup) []actions.OperationalObjectAction {
	scopeType := strings.ToLower(strings.TrimSpace(scope.ScopeType))
	if scopeType != "sku" && scopeType != "campaign" {
		return nil
	}
	if len(scope.EntityIDs) == 0 {
		return nil
	}
	out := make([]actions.OperationalObjectAction, 0, len(scope.EntityIDs))
	for _, id := range scope.EntityIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		obj := actions.OperationalObjectAction{
			EntityType:        scopeType,
			EntityID:          id,
			RecommendedAction: "",
			Metrics:           nil,
		}
		if facts != nil {
			if fact, ok := facts(scopeType, id); ok {
				obj.EntityID = stripEntityPrefix(id)
				obj.Metrics = entityMetrics(fact)
				obj.RecommendedAction = recommendedActionForFact(fact)
				name := strings.TrimSpace(fact.Name)
				if name != "" && !strings.HasPrefix(strings.ToLower(name), "sku:") {
					obj.Name = name
				} else {
					obj.Name = "Товар без названия"
				}
			}
		}
		if obj.Name == "" {
			bare := stripEntityPrefix(id)
			obj.EntityID = bare
			obj.Name = "Товар без названия"
		}
		out = append(out, obj)
	}
	return out
}

func entityMetrics(fact EntityFact) []actions.OperationalMetricRef {
	metrics := make([]actions.OperationalMetricRef, 0, 8)
	if fact.AvailableStock != nil {
		metrics = append(metrics, actions.OperationalMetricRef{
			Label:          "Остаток",
			Value:          fmt.Sprintf("%d", *fact.AvailableStock),
			MetricCode:     "available_stock_units",
			DisplayLabel:   "Остаток",
			FormattedValue: fmt.Sprintf("%d шт.", *fact.AvailableStock),
		})
	}
	if fact.Orders7D != nil {
		metrics = append(metrics, actions.OperationalMetricRef{
			Label:          "Продажи за 7 дней",
			Value:          fmt.Sprintf("%d", *fact.Orders7D),
			MetricCode:     "orders_7d",
			DisplayLabel:   "Продажи за 7 дней",
			FormattedValue: fmt.Sprintf("%d", *fact.Orders7D),
		})
	}
	if fact.CoverageDays7D != nil {
		metrics = append(metrics, actions.OperationalMetricRef{
			Label:          "Покрытие",
			Value:          fmt.Sprintf("%.1f", *fact.CoverageDays7D),
			MetricCode:     "stock_coverage_days_7d",
			DisplayLabel:   "Покрытие",
			FormattedValue: fmt.Sprintf("%.1f дн.", *fact.CoverageDays7D),
		})
	}
	if fact.InTransitUnits != nil {
		metrics = append(metrics, actions.OperationalMetricRef{
			Label:          "В пути",
			Value:          fmt.Sprintf("%d", *fact.InTransitUnits),
			MetricCode:     "in_transit_units",
			DisplayLabel:   "В пути",
			FormattedValue: fmt.Sprintf("%d шт.", *fact.InTransitUnits),
		})
	}
	if fact.TargetCoverageDays != nil {
		label := "Целевое покрытие"
		formatted := fmt.Sprintf("%.0f дн.", *fact.TargetCoverageDays)
		if fact.TargetCoverageSource == stockpolicy.TargetSourcePolicyDefault {
			formatted += " (по умолчанию политики)"
		}
		metrics = append(metrics, actions.OperationalMetricRef{
			Label:          label,
			Value:          fmt.Sprintf("%.0f", *fact.TargetCoverageDays),
			MetricCode:     "target_coverage_days",
			DisplayLabel:   label,
			FormattedValue: formatted,
		})
	}
	if fact.RecommendedSupplyUnits != nil {
		metrics = append(metrics, actions.OperationalMetricRef{
			Label:          "Рекомендуется поставить",
			Value:          fmt.Sprintf("%d", *fact.RecommendedSupplyUnits),
			MetricCode:     "recommended_supply_units",
			DisplayLabel:   "Рекомендуется поставить",
			FormattedValue: fmt.Sprintf("%d шт.", *fact.RecommendedSupplyUnits),
		})
	}
	if fact.RecommendedOrderDate != nil && *fact.RecommendedOrderDate != "" {
		metrics = append(metrics, actions.OperationalMetricRef{
			Label:          "Заказать до",
			Value:          *fact.RecommendedOrderDate,
			MetricCode:     "recommended_order_date",
			DisplayLabel:   "Заказать до",
			FormattedValue: *fact.RecommendedOrderDate,
		})
	}
	if fact.EstimatedStockoutDate != nil && *fact.EstimatedStockoutDate != "" {
		metrics = append(metrics, actions.OperationalMetricRef{
			Label:          "До обнуления",
			Value:          *fact.EstimatedStockoutDate,
			MetricCode:     "estimated_stockout_date",
			DisplayLabel:   "До обнуления",
			FormattedValue: *fact.EstimatedStockoutDate,
		})
	}
	if fact.SurplusExcessUnits != nil {
		metrics = append(metrics, actions.OperationalMetricRef{
			Label:          "Избыточное количество",
			Value:          fmt.Sprintf("%d", *fact.SurplusExcessUnits),
			MetricCode:     "surplus_excess_units",
			DisplayLabel:   "Избыточное количество",
			FormattedValue: fmt.Sprintf("%d шт.", *fact.SurplusExcessUnits),
		})
	}
	return metrics
}

func priorityForCoverage(coverage *float64) string {
	if coverage == nil {
		return ""
	}
	switch {
	case *coverage <= 0:
		return "Критический"
	case *coverage <= 3:
		return "Критический"
	case *coverage <= 7:
		return "Срочный"
	default:
		return "Плановый"
	}
}

func recommendedActionForFact(fact EntityFact) string {
	if fact.StockRiskClass == stockpolicy.SegmentSurplus || (fact.SurplusExcessUnits != nil && *fact.SurplusExcessUnits > 0) {
		if fact.SurplusExcessUnits != nil && *fact.SurplusExcessUnits > 0 {
			return fmt.Sprintf("Не пополнять; сократить закупку примерно на %d шт.", *fact.SurplusExcessUnits)
		}
		return "Не пополнять; сократить следующую закупку и оценить акцию или цену"
	}
	if fact.RecommendedSupplyUnits != nil && *fact.RecommendedSupplyUnits > 0 {
		orderBy := ""
		if fact.RecommendedOrderDate != nil && *fact.RecommendedOrderDate != "" {
			orderBy = "; заказать до " + *fact.RecommendedOrderDate
		}
		return fmt.Sprintf("Поставить %d шт.%s", *fact.RecommendedSupplyUnits, orderBy)
	}
	if fact.AvailableStock != nil && *fact.AvailableStock == 0 {
		return "Срочно восстановить остаток"
	}
	return priorityForCoverage(fact.CoverageDays7D)
}

func targetEntitiesFor(scopeType string, entityIDs []string) []actions.OperationalTargetEntity {
	if len(entityIDs) == 0 {
		return nil
	}
	entityType := scopeType
	if entityType == "" {
		entityType = "entity"
	}
	out := make([]actions.OperationalTargetEntity, 0, len(entityIDs))
	for _, id := range entityIDs {
		out = append(out, actions.OperationalTargetEntity{EntityType: entityType, ID: id})
	}
	return out
}

func projectCompletionConditions(in []contract.UnifiedCompletionConditionV1) []actions.ActionCompletionCondition {
	if len(in) == 0 {
		return nil
	}
	out := make([]actions.ActionCompletionCondition, 0, len(in))
	for _, c := range in {
		out = append(out, actions.ActionCompletionCondition{
			MetricCode:  c.MetricField,
			Operator:    c.Operator,
			TargetValue: c.TargetValue,
			// FactID is the baseline fact the condition is measured against.
			// Never conflate it with condition_key (schema v8+).
			FactID:      strings.TrimSpace(c.BaselineFactID),
			PeriodDays:  intOrZero(c.PeriodDays),
			DisplayText: c.Description,
		})
	}
	return out
}

func intOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
