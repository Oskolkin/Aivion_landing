package actions

import (
	"context"
	"fmt"
	"math"
	"time"
)

const funnelWeakTransitionLimitation = "Причина просадки воронки требует проверки цены, карточки, фото, отзывов, остатков и условий доставки."

type funnelStageDef struct {
	key   string
	label string
	val   *int64
	prev  *int64
}

func buildFunnelDomain(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	metricDate time.Time,
	cfg CandidateBuilderConfig,
) (ActionDomainContext, int) {
	domain := ActionDomainContext{
		Domain:     ActionDomainFunnel,
		Status:     DomainStatusEmpty,
		Candidates: []ActionCandidate{},
	}

	bundle, err := repo.GetFunnelData(ctx, sellerAccountID, metricDate)
	if err != nil {
		domain.Status = DomainStatusError
		domain.DataQualityNotes = []string{fmt.Sprintf("Воронка: ошибка загрузки: %v", err)}
		return domain, 0
	}
	if bundle == nil || !bundle.HasAccountScope || bundle.Current == nil {
		domain.Status = DomainStatusSourceNotAvailable
		domain.DataQualityNotes = funnelUnavailableNotes(bundle, sellerAccountID, metricDate)
		return domain, 0
	}

	row := analyzeFunnelWeakTransition(bundle)
	if row == nil {
		domain.DataQualityNotes = []string{
			"Недостаточно данных для определения слабого перехода воронки.",
			fmt.Sprintf("funnel_diagnostics: rows_account_scope_7d=%d, reason=no_significant_negative_transition, date=%s, seller_account_id=%d",
				len(bundle.History7d), metricDate.Format("2006-01-02"), sellerAccountID),
		}
		return domain, 0
	}

	cand, err := buildFunnelWeakTransitionCandidate(ctx, repo, sellerAccountID, metricDate, *row, cfg)
	if err != nil {
		domain.Status = DomainStatusError
		domain.DataQualityNotes = []string{fmt.Sprintf("funnel_weak_transition: %v", err)}
		return domain, 0
	}
	domain.Status = DomainStatusAvailable
	domain.Candidates = []ActionCandidate{*cand}
	return domain, 1
}

// funnelUnavailableNotes explains why the funnel domain is unavailable. The funnel
// is computed on the account-level aggregate row (scope_type='account'); SKU-scoped
// rows can exist while the account aggregate is missing, which is the usual cause.
func funnelUnavailableNotes(bundle *FunnelDataBundle, sellerAccountID int64, metricDate time.Time) []string {
	reason := "no_account_scope_row"
	missing := "account_aggregate_row"
	historyRows := 0
	switch {
	case bundle == nil:
		reason = "bundle_not_loaded"
		missing = "product_analytics_daily"
	case !bundle.HasAccountScope:
		reason = "no_account_scope_row"
		missing = "scope_type=account row for metric_date"
		historyRows = len(bundle.History7d)
	case bundle.Current == nil:
		reason = "current_day_missing"
		missing = "current account row"
		historyRows = len(bundle.History7d)
	}
	return []string{
		"Аналитика воронки (product_analytics_daily) недоступна за дату анализа: нужна агрегирующая строка уровня аккаунта (scope_type=account).",
		fmt.Sprintf("funnel_diagnostics: reason=%s, missing_fields=%s, rows_account_scope_7d=%d, date=%s, seller_account_id=%d",
			reason, missing, historyRows, metricDate.Format("2006-01-02"), sellerAccountID),
	}
}

func analyzeFunnelWeakTransition(bundle *FunnelDataBundle) *FunnelWeakTransitionRow {
	stages := buildFunnelStageDefs(bundle.Current, bundle.Previous)
	if len(stages) < 2 {
		return nil
	}

	bestSeverity := 0.0
	var best *FunnelWeakTransitionRow
	hasBaseline := false

	for i := 1; i < len(stages); i++ {
		prev := stages[i-1]
		cur := stages[i]
		if prev.val == nil || cur.val == nil || *prev.val <= 0 {
			continue
		}
		conv := float64(*cur.val) / float64(*prev.val) * 100
		conv = math.Round(conv*100) / 100

		var convPrev *float64
		var deltaPP *float64
		if prev.prev != nil && cur.prev != nil && *prev.prev > 0 {
			pv := float64(*cur.prev) / float64(*prev.prev) * 100
			pv = math.Round(pv*100) / 100
			convPrev = &pv
			d := conv - pv
			d = math.Round(d*100) / 100
			deltaPP = &d
		}

		var volumeDelta *float64
		if cur.prev != nil && *cur.prev > 0 {
			vd := (float64(*cur.val) - float64(*cur.prev)) / float64(*cur.prev) * 100
			vd = math.Round(vd*100) / 100
			volumeDelta = &vd
		}

		var vs7dPP *float64
		if avg := averageFunnelTransitionConversion(bundle.History7d, prev.key, cur.key); avg != nil {
			v := conv - *avg
			v = math.Round(v*100) / 100
			vs7dPP = &v
		}

		if deltaPP == nil && vs7dPP == nil {
			continue
		}
		severity := 0.0
		if deltaPP != nil && *deltaPP < severity {
			severity = *deltaPP
		}
		if vs7dPP != nil && *vs7dPP < severity {
			severity = *vs7dPP
		}
		if !hasBaseline || severity < bestSeverity {
			hasBaseline = true
			bestSeverity = severity
			transitionKey := prev.key + "_to_" + cur.key
			best = &FunnelWeakTransitionRow{
				TransitionKey:      transitionKey,
				FromStage:          prev.label,
				ToStage:            cur.label,
				FromStageKey:       prev.key,
				ToStageKey:         cur.key,
				ConversionCurrent:  conv,
				ConversionPrevious: convPrev,
				ConversionDeltaPP:  deltaPP,
				VolumeDeltaPct:     volumeDelta,
			}
		}
	}

	if best != nil && hasBaseline && bestSeverity < 0 {
		return best
	}
	return nil
}

func buildFunnelStageDefs(current, previous *FunnelDayMetrics) []funnelStageDef {
	if current == nil {
		return nil
	}
	var prev *FunnelDayMetrics
	if previous != nil {
		prev = previous
	}
	labels := []struct {
		key   string
		label string
		cur   *int64
		prv   *int64
	}{
		{"impressions", "Показы", current.Impressions, metricPtr(prev, "impressions")},
		{"product_views", "Просмотры карточки", current.ProductViews, metricPtr(prev, "product_views")},
		{"add_to_cart", "Добавления в корзину", current.AddToCart, metricPtr(prev, "add_to_cart")},
		{"orders", "Заказанные единицы", current.Orders, metricPtr(prev, "orders")},
	}
	if current.CompletedOrders != nil {
		labels = append(labels, struct {
			key   string
			label string
			cur   *int64
			prv   *int64
		}{"delivered_units", "Доставленные единицы", current.CompletedOrders, metricPtr(prev, "delivered_units")})
	}
	out := make([]funnelStageDef, 0, len(labels))
	for _, l := range labels {
		if l.cur == nil {
			continue
		}
		out = append(out, funnelStageDef{key: l.key, label: l.label, val: l.cur, prev: l.prv})
	}
	return out
}

func metricPtr(day *FunnelDayMetrics, key string) *int64 {
	if day == nil {
		return nil
	}
	switch key {
	case "impressions":
		return day.Impressions
	case "product_views":
		return day.ProductViews
	case "add_to_cart":
		return day.AddToCart
	case "orders":
		return day.Orders
	case "delivered_units":
		return day.CompletedOrders
	default:
		return nil
	}
}

func averageFunnelTransitionConversion(history []FunnelDayMetrics, fromKey, toKey string) *float64 {
	if len(history) == 0 {
		return nil
	}
	total := 0.0
	count := 0
	for i := range history {
		fromVal := metricPtr(&history[i], fromKey)
		toVal := metricPtr(&history[i], toKey)
		if fromVal == nil || toVal == nil || *fromVal <= 0 {
			continue
		}
		total += float64(*toVal) / float64(*fromVal) * 100
		count++
	}
	if count == 0 {
		return nil
	}
	avg := math.Round((total/float64(count))*100) / 100
	return &avg
}

func buildFunnelWeakTransitionCandidate(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	metricDate time.Time,
	row FunnelWeakTransitionRow,
	cfg CandidateBuilderConfig,
) (*ActionCandidate, error) {
	suffix := row.FromStageKey + "_to_" + row.ToStageKey
	id := candidateID(ActionTypeFunnelWeakTransition, metricDate, suffix)

	aggregates := map[string]any{
		"transition_key":     row.TransitionKey,
		"from_stage":         row.FromStage,
		"to_stage":           row.ToStage,
		"conversion_current": row.ConversionCurrent,
	}
	if row.ConversionPrevious != nil {
		aggregates["conversion_previous"] = *row.ConversionPrevious
	}
	if row.ConversionDeltaPP != nil {
		aggregates["conversion_delta_pp"] = *row.ConversionDeltaPP
	}
	if row.VolumeDeltaPct != nil {
		aggregates["volume_delta_pct"] = *row.VolumeDeltaPct
	}

	facts := []ActionCandidateFact{
		{
			FactID: factID(id, "weakest_transition"),
			Kind:   "funnel",
			Label:  "Слабый переход",
			Value:  row.FromStage + " → " + row.ToStage,
		},
	}
	if row.ConversionDeltaPP != nil {
		facts = append(facts, formatCountFact(id, "conversion_delta", "Изменение конверсии", fmt.Sprintf("%.2f", *row.ConversionDeltaPP), "п.п."))
	}
	if row.VolumeDeltaPct != nil {
		facts = append(facts, formatCountFact(id, "volume_delta", "Изменение объёма", fmt.Sprintf("%.2f", *row.VolumeDeltaPct), "%"))
	}

	examples := []ActionCandidateExample{{
		EntityType: "funnel",
		EntityID:   row.TransitionKey,
		Name:       row.FromStage + " → " + row.ToStage,
		Metrics: map[string]any{
			"conversion_current": row.ConversionCurrent,
		},
	}}

	var groupRef *ActionEntityGroupRef
	if len(row.SKUContributorIDs) > 0 {
		groupKey := entityGroupKey(id)
		summary := map[string]any{"transition_key": row.TransitionKey}
		rec, err := repo.UpsertActionEntityGroup(ctx, sellerAccountID, metricDate, groupKey, "sku", row.SKUContributorIDs, summary)
		if err != nil {
			return nil, err
		}
		groupRef = entityGroupRefFromRecord(rec, summary)
	}

	priority := ActionPriorityWatch
	if row.ConversionDeltaPP != nil && *row.ConversionDeltaPP <= -2 {
		priority = ActionPriorityImportant
	}

	return &ActionCandidate{
		CandidateID:    id,
		Domain:         ActionDomainFunnel,
		Type:           ActionTypeFunnelWeakTransition,
		Category:       ActionCategoryFunnel,
		PriorityHint:   priority,
		Summary:        fmt.Sprintf("Слабый переход воронки: %s → %s", row.FromStage, row.ToStage),
		Aggregates:     aggregates,
		Facts:          limitFacts(facts, cfg.MaxFactsPerCandidate),
		TopExamples:    limitExamples(examples, cfg.MaxTopExamplesPerCandidate),
		EntityGroupRef: groupRef,
		AllowedCTAs: []string{
			ActionCTAAskAI,
			ActionCTAMarkChecked,
		},
		Limitations: []string{funnelWeakTransitionLimitation},
		SourceRefs:  []string{"product_analytics_daily"},
	}, nil
}

// GetFunnelWeakTransitionCandidateRow exposes funnel analysis for repository contract.
func GetFunnelWeakTransitionCandidateRow(bundle *FunnelDataBundle) *FunnelWeakTransitionRow {
	return analyzeFunnelWeakTransition(bundle)
}
