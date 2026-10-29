package dashboardhome

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

const (
	sellerPulseNeutralPctThreshold  = 2.0
	sellerPulseDRRWarningThreshold  = 20.0
	sellerPulseDRRCriticalThreshold = 35.0
)

var sellerPulseAllowedMetricRefs = map[string]struct{}{
	"revenue":                      {},
	"orders":                       {},
	"ctr":                          {},
	"average_order_value":          {},
	"ad_spend_share_total_revenue": {},
	"drr":                          {},
	"risks":                        {},
}

var sellerPulseAllowedKinds = map[string]string{
	"action":          "/app/actions",
	"advertising":     "/app/actions",
	"stock":           "/app/actions",
	"sku":             "/app/actions",
	"funnel":          "/app/actions",
	"recommendations": "/app/actions",
	"monitoring":      "/app/actions",
	"products":        "/app/actions",
	"slow_turnover":   "/app/actions",
}

var sellerPulseAllowedHrefs = map[string]struct{}{
	"/app/actions":   {},
	"/app/dashboard": {},
}

type sellerPulseKPIState string

const (
	sellerPulseStatePositive    sellerPulseKPIState = "positive"
	sellerPulseStateNegative    sellerPulseKPIState = "negative"
	sellerPulseStateNeutral     sellerPulseKPIState = "neutral"
	sellerPulseStateWarning     sellerPulseKPIState = "warning"
	sellerPulseStateCritical    sellerPulseKPIState = "critical"
	sellerPulseStateNoData      sellerPulseKPIState = "no_data"
	sellerPulseStateUnavailable sellerPulseKPIState = "unavailable"
)

type sellerPulseKPIFact struct {
	Key          string
	Label        string
	Value        string
	Present      bool
	State        sellerPulseKPIState
	DeltaAbs     float64
	DeltaPct     *float64
	DeltaLabel   string
	HasDelta     bool
	HasSparkline bool
}

type sellerPulseNarrativeValidationContext struct {
	KPIByKey      map[string]sellerPulseKPIFact
	CandidateByID map[string]dashboardNarrativeCandidate
	AllowedHrefs  map[string]struct{}
}

func buildSellerPulseContextMap(
	metricDate string,
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	alertRows []dbgen.Alert,
	actionPool []TopActionItem,
) map[string]any {
	facts := buildSellerPulseKPIFacts(resp.SellerPulse.KPIs)
	kpiView := make([]map[string]any, 0, len(facts))
	for _, key := range []string{"revenue", "orders", "ctr", "drr", "risks"} {
		fact, ok := facts[key]
		if !ok {
			continue
		}
		item := map[string]any{
			"key":     fact.Key,
			"label":   fact.Label,
			"present": fact.Present,
			"state":   fact.State,
		}
		if fact.Value != "" {
			item["value"] = fact.Value
		}
		if fact.HasDelta {
			delta := map[string]any{"abs": fact.DeltaAbs, "label": fact.DeltaLabel}
			if fact.DeltaPct != nil {
				delta["pct"] = *fact.DeltaPct
			}
			item["delta"] = delta
		}
		item["has_sparkline"] = fact.HasSparkline
		kpiView = append(kpiView, item)
	}

	topAlerts := make([]map[string]any, 0, minInt(len(alertRows), 5))
	for _, row := range alertRows {
		if row.Severity != "critical" && row.Severity != "high" {
			continue
		}
		topAlerts = append(topAlerts, map[string]any{
			"id":       row.ID,
			"severity": row.Severity,
			"title":    cleanupSellerFacingText(row.Title),
			"message":  cleanupSellerFacingText(row.Message),
		})
		if len(topAlerts) >= 5 {
			break
		}
	}

	candidates := make([]map[string]any, 0, minInt(len(actionPool), 6))
	for _, action := range actionPool {
		candidates = append(candidates, map[string]any{
			"id":        action.ID,
			"category":  action.Category,
			"title":     cleanupSellerFacingText(action.Title),
			"reason":    cleanupSellerFacingText(action.Reason),
			"next_step": cleanupSellerFacingText(action.NextStep),
		})
	}

	allowedKindKeys := make([]string, 0, len(sellerPulseAllowedKinds))
	for kind := range sellerPulseAllowedKinds {
		allowedKindKeys = append(allowedKindKeys, kind)
	}
	sort.Strings(allowedKindKeys)
	allowedKinds := make([]map[string]any, 0, len(allowedKindKeys))
	for _, kind := range allowedKindKeys {
		allowedKinds = append(allowedKinds, map[string]any{"kind": kind, "href": sellerPulseAllowedKinds[kind]})
	}

	funnelWeakSpot := map[string]any{
		"status":               resp.AIDiagnostics.SalesFunnel.Status,
		"primary_drop_stage":   resp.AIDiagnostics.FunnelAnalysis.PrimaryDropStage,
		"primary_drop_message": resp.AIDiagnostics.FunnelAnalysis.PrimaryDropMessage,
	}

	return map[string]any{
		"metric_date": metricDate,
		"kpis":        kpiView,
		"account": map[string]any{
			"revenue_today":               dto.Account.RevenueToday,
			"orders_today":                dto.Account.OrdersToday,
			"orders_yesterday":            dto.Account.OrdersYesterday,
			"account_ad_spend_to_revenue": valueOrZero(dto.Account.AccountAdSpendToRevenue),
		},
		"top_alerts":                  topAlerts,
		"top_action_candidates":       candidates,
		"funnel_weak_spot":            funnelWeakSpot,
		"open_high_or_critical_risks": countCriticalHighRisksFromKPIs(resp.SellerPulse.KPIs),
		"data_quality_notes":          stringSliceOrEmpty(resp.DataQualityNotes),
		"allowed_quick_action_kinds":  allowedKinds,
		"allowed_metric_refs":         []string{"revenue", "orders", "ctr", "drr", "risks"},
	}
}

func buildSellerPulseNarrativeValidationContext(
	kpis []KPICard,
	build dashboardNarrativeBuildResult,
) sellerPulseNarrativeValidationContext {
	return sellerPulseNarrativeValidationContext{
		KPIByKey:      buildSellerPulseKPIFacts(kpis),
		CandidateByID: build.CandidateByID,
		AllowedHrefs:  sellerPulseAllowedHrefs,
	}
}

func buildSellerPulseKPIFacts(kpis []KPICard) map[string]sellerPulseKPIFact {
	byKey := make(map[string]KPICard, len(kpis))
	for _, kpi := range kpis {
		byKey[kpi.Key] = kpi
	}
	out := make(map[string]sellerPulseKPIFact, 5)
	for _, key := range []string{"revenue", "orders", "ctr", "drr", "risks"} {
		kpi, present := byKey[key]
		out[key] = resolveSellerPulseKPIFact(key, kpi, present)
	}
	return out
}

func resolveSellerPulseKPIFact(key string, kpi KPICard, present bool) sellerPulseKPIFact {
	label := sellerPulseMetricLabel(key)
	if present && strings.TrimSpace(kpi.Label) != "" {
		label = strings.TrimSpace(kpi.Label)
	}
	if key == "risks" && present {
		label = "Риски"
	}
	fact := sellerPulseKPIFact{
		Key:     key,
		Label:   label,
		Present: present,
		State:   sellerPulseStateNoData,
	}
	if !present {
		if key == "risks" {
			fact.State = sellerPulseStateUnavailable
		}
		return fact
	}
	fact.Value = strings.TrimSpace(kpi.Value)
	fact.HasSparkline = len(kpi.Sparkline7D) >= 2
	if fact.Value == "" {
		return fact
	}
	if kpi.Delta != nil {
		fact.HasDelta = true
		fact.DeltaAbs = kpi.Delta.Abs
		fact.DeltaPct = kpi.Delta.Pct
		fact.DeltaLabel = strings.TrimSpace(kpi.Delta.Label)
	}
	switch key {
	case "drr":
		fact.State = sellerPulseDRRState(kpi, fact)
	case "risks":
		fact.State = sellerPulseRisksState(kpi)
	default:
		fact.State = sellerPulseGrowthState(key, fact)
	}
	return fact
}

func sellerPulseGrowthState(key string, fact sellerPulseKPIFact) sellerPulseKPIState {
	if !fact.HasDelta {
		return sellerPulseStateNeutral
	}
	significant := false
	if fact.DeltaPct != nil && math.Abs(*fact.DeltaPct) >= sellerPulseNeutralPctThreshold {
		significant = true
	}
	if key == "orders" || key == "risks" {
		significant = significant || math.Abs(fact.DeltaAbs) >= 1
	}
	if key == "revenue" {
		significant = significant || math.Abs(fact.DeltaAbs) >= 1
	}
	if key == "ctr" {
		significant = significant || math.Abs(fact.DeltaAbs) >= sellerPulseCTRNeutralPPThreshold
	}
	if !significant {
		return sellerPulseStateNeutral
	}
	improved := fact.DeltaAbs > 0 || (fact.DeltaPct != nil && *fact.DeltaPct > 0)
	if improved {
		return sellerPulseStatePositive
	}
	return sellerPulseStateNegative
}

func sellerPulseDRRState(kpi KPICard, fact sellerPulseKPIFact) sellerPulseKPIState {
	pct := parseSellerPulsePercent(kpi.Value)
	if pct == nil {
		return sellerPulseStateNoData
	}
	if fact.HasDelta && sellerPulseGrowthState("drr", fact) == sellerPulseStateNegative {
		if *pct >= sellerPulseDRRCriticalThreshold {
			return sellerPulseStateCritical
		}
		return sellerPulseStateWarning
	}
	if *pct >= sellerPulseDRRCriticalThreshold {
		return sellerPulseStateCritical
	}
	if *pct >= sellerPulseDRRWarningThreshold {
		return sellerPulseStateWarning
	}
	return sellerPulseStatePositive
}

func sellerPulseRisksState(kpi KPICard) sellerPulseKPIState {
	count := parseSellerPulseInt(kpi.Value)
	if count == nil {
		return sellerPulseStateNoData
	}
	if *count == 0 {
		return sellerPulseStatePositive
	}
	if *count >= 3 {
		return sellerPulseStateCritical
	}
	return sellerPulseStateWarning
}

func applySellerPulseNarrativeToResponse(resp *Response, build *dashboardNarrativeBuildResult) {
	if len(resp.SellerPulse.KPIs) == 0 {
		resp.SellerPulse.Narrative = &SellerPulseNarrative{
			Status:   "not_enough_data",
			Tone:     "neutral",
			Headline: "Данных пока недостаточно для полноценной оценки динамики.",
			Summary:  "Блок обновится после синхронизации метрик.",
			ImportantNotes: []SellerPulseImportantNote{
				{Text: "Данных пока недостаточно для полноценного анализа динамики."},
				{Text: "Блок обновится после завершения синхронизации метрик."},
			},
			QuickActions: []SellerPulseQuickAction{
				{Label: "Дождаться завершения синхронизации"},
				{Label: "Обновить главную после загрузки данных", Kind: "monitoring", Href: "/app/dashboard"},
			},
		}
		return
	}

	fallback := buildSellerPulseNarrativeFallback(resp)
	stored := resp.AIDiagnostics.AccountSummary.storedNarrative
	if dashboardAIOnlyEnabled() {
		if stored != nil && stored.Payload != nil && stored.Payload.SellerPulseNarrative != nil {
			ready := cloneSellerPulseNarrative(stored.Payload.SellerPulseNarrative)
			ready.Status = "available"
			ready.DataQualityNote = ""
			resp.SellerPulse.Narrative = ready
			return
		}
		resp.SellerPulse.Narrative = &SellerPulseNarrative{
			Status:   "not_available",
			Tone:     "neutral",
			Headline: "AI-сводка пульса недоступна.",
			Summary:  "Для режима AI-only требуется полный payload seller_pulse_narrative.",
		}
		return
	}
	if stored == nil {
		fallback.Status = "pending"
		fallback.DataQualityNote = "AI-сводка блока формируется после синхронизации метрик."
		resp.SellerPulse.Narrative = fallback
		return
	}
	if stored.Status != "completed" {
		fallback.Status = "pending"
		fallback.DataQualityNote = "AI-сводка блока формируется после синхронизации метрик."
		resp.SellerPulse.Narrative = fallback
		return
	}
	if build == nil || !narrativeFreshForCurrentContext(stored, *build) {
		fallback.Status = "stale"
		fallback.DataQualityNote = "Сводка устарела и обновится после следующей синхронизации."
		resp.SellerPulse.Narrative = fallback
		return
	}
	if stored.Payload == nil || stored.Payload.SellerPulseNarrative == nil {
		fallback.Status = "fallback"
		fallback.DataQualityNote = "Показана базовая сводка по метрикам без AI-интерпретации."
		recordNarrativeBlockFallback("seller_pulse_narrative")
		resp.SellerPulse.Narrative = fallback
		return
	}
	ctx := buildSellerPulseNarrativeValidationContext(resp.SellerPulse.KPIs, *build)
	if errs := validateSellerPulseNarrativePayload(stored.Payload.SellerPulseNarrative, ctx); len(errs) > 0 {
		fallback.Status = "fallback"
		fallback.DataQualityNote = "Показана базовая сводка по метрикам без AI-интерпретации."
		recordNarrativeBlockFallback("seller_pulse_narrative")
		resp.SellerPulse.Narrative = fallback
		return
	}
	ready := cloneSellerPulseNarrative(stored.Payload.SellerPulseNarrative)
	ready.Status = "ready"
	resp.SellerPulse.Narrative = ready
}

func buildSellerPulseNarrativeFallback(resp *Response) *SellerPulseNarrative {
	facts := buildSellerPulseKPIFacts(resp.SellerPulse.KPIs)
	headline, tone, chips := buildSellerPulseFallbackSummary(facts, resp.Status)
	notes := buildSellerPulseFallbackNotes(facts, resp.Status)
	actions := buildSellerPulseFallbackQuickActions(facts, resp)
	summary := headline
	if len(notes) > 0 {
		summary = notes[0].Text
	}
	return &SellerPulseNarrative{
		Status:         "fallback",
		Tone:           tone,
		Headline:       headline,
		Summary:        summary,
		ImportantNotes: notes,
		QuickActions:   actions,
		Chips:          chips,
	}
}

func buildSellerPulseFallbackSummary(facts map[string]sellerPulseKPIFact, status GlobalStatus) (string, string, []SellerPulseChip) {
	stats := sellerPulseStats(facts)
	revenue := facts["revenue"]
	orders := facts["orders"]
	drr := facts["drr"]
	risks := facts["risks"]
	chips := make([]SellerPulseChip, 0, 2)

	if status == StatusNotEnoughData || stats.noData+stats.unavailable >= 4 {
		return "Данных пока недостаточно для полноценной оценки динамики. Блок обновится после синхронизации.",
			"neutral", chips
	}
	if stats.critical > 0 {
		chips = appendSellerPulseSignalChips(chips, facts, 2)
		return "Есть критичные зоны: проверьте показатели с наибольшим влиянием на результат дня.",
			"critical", chips
	}
	if revenue.State == sellerPulseStateNegative || orders.State == sellerPulseStateNegative {
		chips = appendSellerPulseSignalChips(chips, facts, 2)
		return "Есть ухудшение по продажам: проверьте динамику заказов, воронку и товары с наибольшим вкладом в просадку.",
			"attention", chips
	}
	if drr.State == sellerPulseStateWarning || drr.State == sellerPulseStateCritical {
		chips = appendSellerPulseSignalChips(chips, facts, 2)
		return "Рекламная эффективность требует внимания: проверьте кампании с высоким ДРР и расходом без заказов.",
			"attention", chips
	}
	if risks.State == sellerPulseStateWarning || risks.State == sellerPulseStateCritical {
		chips = appendSellerPulseSignalChips(chips, facts, 2)
		return "Есть товары с повышенным риском: стоит проверить SKU с дефицитом или критичными сигналами.",
			"attention", chips
	}
	if stats.positive >= 2 && stats.negative == 0 && stats.warning == 0 {
		return "Кабинет выглядит стабильно: ключевые показатели без критичных ухудшений.",
			"stable", chips
	}
	if stats.noData >= 2 {
		return "Часть метрик ещё формируется. Доступные показатели можно использовать для первичного контроля.",
			"neutral", chips
	}
	return "Ключевые показатели в пределах ожидаемой динамики. Продолжайте мониторинг продаж и рисков.",
		"stable", chips
}

func buildSellerPulseFallbackNotes(facts map[string]sellerPulseKPIFact, status GlobalStatus) []SellerPulseImportantNote {
	notes := make([]SellerPulseImportantNote, 0, 4)
	push := func(text string, tone string, refs ...string) {
		if len(notes) >= 4 {
			return
		}
		for _, note := range notes {
			if note.Text == text {
				return
			}
		}
		notes = append(notes, SellerPulseImportantNote{Text: text, Tone: tone, MetricRefs: refs})
	}
	stats := sellerPulseStats(facts)
	if status == StatusNotEnoughData || stats.noData+stats.unavailable >= 4 {
		push("Данных пока недостаточно для полноценного анализа динамики.", "neutral")
		push("Блок обновится после завершения синхронизации метрик.", "neutral")
		return notes
	}
	revenue := facts["revenue"]
	orders := facts["orders"]
	ctr := facts["ctr"]
	drr := facts["drr"]
	risks := facts["risks"]
	if revenue.State == sellerPulseStateNegative {
		push("Выручка снизилась относительно предыдущего дня — стоит проверить товары с наибольшим вкладом.", "warning", "revenue")
	} else if revenue.State == sellerPulseStatePositive {
		push("Выручка растёт относительно предыдущего дня.", "positive", "revenue")
	}
	if orders.State == sellerPulseStateNegative {
		push("Заказов стало меньше — имеет смысл проверить спрос и воронку продаж.", "warning", "orders")
	} else if orders.State == sellerPulseStatePositive {
		push("Количество заказов увеличилось.", "positive", "orders")
	}
	if ctr.State == sellerPulseStateNegative {
		push("CTR снизился — меньше показов переходят в просмотры карточек.", "warning", "ctr")
	} else if ctr.State == sellerPulseStatePositive {
		push("CTR вырос — больше показов переходят в просмотры карточек.", "positive", "ctr")
	}
	if drr.State == sellerPulseStateWarning || drr.State == sellerPulseStateCritical {
		push("ДРР повышен — проверьте рекламные кампании и расход без заказов.", "warning", "drr")
	} else if drr.State == sellerPulseStatePositive {
		push("Рекламная эффективность в комфортном диапазоне.", "positive", "drr")
	} else if drr.State == sellerPulseStateNoData {
		push("Рекламные данные пока недоступны для оценки ДРР.", "neutral", "drr")
	}
	if risks.State == sellerPulseStateWarning || risks.State == sellerPulseStateCritical {
		push("Есть открытые риски высокой важности — стоит разобрать приоритетные сигналы.", "critical", "risks")
	} else if risks.State == sellerPulseStatePositive {
		push("Критичных рисков по открытым сигналам не обнаружено.", "positive", "risks")
	}
	if len(notes) == 0 {
		push("Критичных ухудшений по доступным метрикам не видно.", "neutral")
	}
	if stats.noData > 0 || stats.unavailable > 0 {
		push("Часть метрик ограничена по данным — выводы основаны только на доступном срезе.", "neutral")
	}
	return notes
}

func buildSellerPulseFallbackQuickActions(facts map[string]sellerPulseKPIFact, resp *Response) []SellerPulseQuickAction {
	actions := make([]SellerPulseQuickAction, 0, 4)
	push := func(label, kind, href, priority string) {
		if len(actions) >= 4 {
			return
		}
		for _, action := range actions {
			if action.Label == label {
				return
			}
		}
		actions = append(actions, SellerPulseQuickAction{
			Label:    label,
			Kind:     kind,
			Href:     href,
			Priority: priority,
		})
	}
	stats := sellerPulseStats(facts)
	if resp.Status == StatusNotEnoughData || stats.noData+stats.unavailable >= 4 {
		push("Дождаться завершения синхронизации", "monitoring", "", "medium")
		push("Обновить главную после загрузки данных", "monitoring", "/app/dashboard", "low")
		return actions
	}
	for _, top := range resp.AIDiagnostics.TopActions {
		if len(actions) >= 4 {
			break
		}
		href := actionDetailHref(top.ActionKey, top.ActionID)
		push(cleanupSellerFacingText(top.Title), "action", href, top.Priority)
	}
	if len(actions) == 0 {
		push("Открыть раздел действий", "action", "/app/actions", "medium")
	}
	return actions
}

func isAllowedSellerPulseHref(href string, allowed map[string]struct{}) bool {
	href = strings.TrimSpace(href)
	if href == "" {
		return false
	}
	if _, ok := allowed[href]; ok {
		return true
	}
	if strings.HasPrefix(href, "/app/actions?") || href == "/app/actions" {
		return true
	}
	return false
}

func sellerPulseKindForTopAction(category string) (string, string) {
	_ = category
	return "action", "/app/actions"
}

func validateSellerPulseNarrativePayload(n *SellerPulseNarrative, ctx sellerPulseNarrativeValidationContext) []string {
	var errs []string
	if n == nil {
		return []string{"nil_seller_pulse_narrative"}
	}
	if strings.TrimSpace(n.Headline) == "" {
		errs = append(errs, "seller_pulse_missing_headline")
	}
	if strings.TrimSpace(n.Summary) == "" {
		errs = append(errs, "seller_pulse_missing_summary")
	}
	if len(n.ImportantNotes) == 0 {
		errs = append(errs, "seller_pulse_missing_important_notes")
	}
	if len(n.QuickActions) == 0 {
		errs = append(errs, "seller_pulse_missing_quick_actions")
	}
	for i, note := range n.ImportantNotes {
		if strings.TrimSpace(note.Text) == "" {
			errs = append(errs, fmt.Sprintf("seller_pulse_note_%d_empty", i))
		}
		for _, ref := range note.MetricRefs {
			if _, ok := sellerPulseAllowedMetricRefs[ref]; !ok {
				errs = append(errs, fmt.Sprintf("seller_pulse_note_%d_unknown_metric_ref_%s", i, ref))
			}
		}
	}
	for i, action := range n.QuickActions {
		if strings.TrimSpace(action.Label) == "" {
			errs = append(errs, fmt.Sprintf("seller_pulse_action_%d_empty_label", i))
		}
		if action.Kind != "" {
			if _, ok := sellerPulseAllowedKinds[action.Kind]; !ok {
				errs = append(errs, fmt.Sprintf("seller_pulse_action_%d_unknown_kind", i))
			}
		}
		if action.Href != "" {
			if !isAllowedSellerPulseHref(action.Href, ctx.AllowedHrefs) {
				errs = append(errs, fmt.Sprintf("seller_pulse_action_%d_unknown_href", i))
			}
		}
	}
	for i, chip := range n.Chips {
		if strings.TrimSpace(chip.Label) == "" {
			errs = append(errs, fmt.Sprintf("seller_pulse_chip_%d_empty", i))
		}
	}
	return compactStringSlice(errs)
}

func normalizeSellerPulseNarrativePayload(n *SellerPulseNarrative) {
	if n == nil {
		return
	}
	n.Status = strings.TrimSpace(n.Status)
	n.Tone = strings.TrimSpace(n.Tone)
	n.Headline = cleanupSellerFacingText(n.Headline)
	n.Summary = cleanupSellerFacingText(n.Summary)
	n.DataQualityNote = cleanupSellerFacingText(n.DataQualityNote)
	if n.KeyPoints == nil {
		n.KeyPoints = []SellerPulseImportantNote{}
	}
	for i := range n.KeyPoints {
		n.KeyPoints[i].Title = cleanupSellerFacingText(n.KeyPoints[i].Title)
		n.KeyPoints[i].Text = cleanupSellerFacingText(n.KeyPoints[i].Text)
		n.KeyPoints[i].Tone = strings.TrimSpace(n.KeyPoints[i].Tone)
		n.KeyPoints[i].MetricRefs = compactStringSlice(n.KeyPoints[i].MetricRefs)
	}
	if n.ImportantNotes == nil {
		n.ImportantNotes = []SellerPulseImportantNote{}
	}
	for i := range n.ImportantNotes {
		n.ImportantNotes[i].Title = cleanupSellerFacingText(n.ImportantNotes[i].Title)
		n.ImportantNotes[i].Text = cleanupSellerFacingText(n.ImportantNotes[i].Text)
		n.ImportantNotes[i].Tone = strings.TrimSpace(n.ImportantNotes[i].Tone)
		n.ImportantNotes[i].MetricRefs = compactStringSlice(n.ImportantNotes[i].MetricRefs)
	}
	if n.QuickActions == nil {
		n.QuickActions = []SellerPulseQuickAction{}
	}
	for i := range n.QuickActions {
		n.QuickActions[i].Label = cleanupSellerFacingText(n.QuickActions[i].Label)
		n.QuickActions[i].Description = cleanupSellerFacingText(n.QuickActions[i].Description)
		n.QuickActions[i].Priority = normalizeTopActionPriority(n.QuickActions[i].Priority)
		n.QuickActions[i].Kind = strings.ToLower(strings.TrimSpace(n.QuickActions[i].Kind))
		n.QuickActions[i].Href = strings.TrimSpace(n.QuickActions[i].Href)
	}
	if n.Chips == nil {
		n.Chips = []SellerPulseChip{}
	}
	for i := range n.Chips {
		n.Chips[i].Label = cleanupSellerFacingText(n.Chips[i].Label)
		n.Chips[i].Tone = strings.TrimSpace(n.Chips[i].Tone)
	}
}

func cloneSellerPulseNarrative(src *SellerPulseNarrative) *SellerPulseNarrative {
	if src == nil {
		return nil
	}
	dst := *src
	dst.KeyPoints = append([]SellerPulseImportantNote(nil), src.KeyPoints...)
	dst.ImportantNotes = append([]SellerPulseImportantNote(nil), src.ImportantNotes...)
	dst.QuickActions = append([]SellerPulseQuickAction(nil), src.QuickActions...)
	dst.Chips = append([]SellerPulseChip(nil), src.Chips...)
	return &dst
}

type sellerPulseStatCounts struct {
	critical, negative, warning, positive, noData, unavailable int
}

func sellerPulseStats(facts map[string]sellerPulseKPIFact) sellerPulseStatCounts {
	var stats sellerPulseStatCounts
	for _, fact := range facts {
		switch fact.State {
		case sellerPulseStateCritical:
			stats.critical++
		case sellerPulseStateNegative:
			stats.negative++
		case sellerPulseStateWarning:
			stats.warning++
		case sellerPulseStatePositive:
			stats.positive++
		case sellerPulseStateNoData:
			stats.noData++
		case sellerPulseStateUnavailable:
			stats.unavailable++
		}
	}
	return stats
}

func appendSellerPulseSignalChips(chips []SellerPulseChip, facts map[string]sellerPulseKPIFact, limit int) []SellerPulseChip {
	for _, key := range []string{"revenue", "orders", "drr", "risks"} {
		if len(chips) >= limit {
			break
		}
		fact := facts[key]
		if fact.State != sellerPulseStateNegative && fact.State != sellerPulseStateWarning && fact.State != sellerPulseStateCritical {
			continue
		}
		label := fmt.Sprintf("%s: %s", fact.Label, sellerPulseStateLabel(fact.State))
		chips = append(chips, SellerPulseChip{Label: label, Tone: sellerPulseChipTone(fact.State)})
	}
	return chips
}

func sellerPulseMetricLabel(key string) string {
	switch key {
	case "revenue":
		return "Выручка"
	case "orders":
		return "Заказы"
	case "ctr":
		return "CTR"
	case "drr":
		return "ДРР"
	case "risks":
		return "Риски"
	default:
		return key
	}
}

func sellerPulseStateLabel(state sellerPulseKPIState) string {
	switch state {
	case sellerPulseStatePositive:
		return "рост"
	case sellerPulseStateNegative:
		return "снижение"
	case sellerPulseStateWarning:
		return "требует внимания"
	case sellerPulseStateCritical:
		return "критично"
	default:
		return "стабильно"
	}
}

func sellerPulseChipTone(state sellerPulseKPIState) string {
	switch state {
	case sellerPulseStatePositive:
		return "positive"
	case sellerPulseStateNegative, sellerPulseStateCritical:
		return "critical"
	case sellerPulseStateWarning:
		return "warning"
	default:
		return "neutral"
	}
}

func parseSellerPulsePercent(value string) *float64 {
	value = strings.TrimSpace(strings.ReplaceAll(value, "%", ""))
	value = strings.ReplaceAll(value, ",", ".")
	value = strings.ReplaceAll(value, " ", "")
	if value == "" {
		return nil
	}
	var parsed float64
	if _, err := fmt.Sscanf(value, "%f", &parsed); err != nil {
		return nil
	}
	return &parsed
}

func parseSellerPulseInt(value string) *int {
	value = strings.TrimSpace(strings.ReplaceAll(value, " ", ""))
	if value == "" {
		return nil
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return nil
	}
	return &parsed
}

func countCriticalHighRisksFromKPIs(kpis []KPICard) int {
	for _, kpi := range kpis {
		if kpi.Key != "risks" {
			continue
		}
		if count := parseSellerPulseInt(kpi.Value); count != nil {
			return *count
		}
	}
	return 0
}
