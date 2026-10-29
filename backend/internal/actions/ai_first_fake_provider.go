package actions

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// NewDeterministicAIFirstFakeClient returns a deterministic dev/test AI client
// for ACTIONS_AI_FIRST flow without external OpenAI calls.
func NewDeterministicAIFirstFakeClient() *FakeActionAIClient {
	return &FakeActionAIClient{
		ComposeAIFirstComposeFn: func(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstComposeOutput, *AIRawResponse, error) {
			out := buildDeterministicAIFirstV2ComposeOutput(input)
			return &out, &AIRawResponse{
				Model: "fake-ai-first-deterministic", RawText: "deterministic fixture",
				InputTokens: 512, OutputTokens: 128,
			}, nil
		},
		ComposeObjectDetailFn: func(ctx context.Context, batch ObjectDetailBatchInput) (*ObjectDetailBatchOutput, *AIRawResponse, error) {
			return buildDeterministicObjectDetailOutput(batch), &AIRawResponse{
				Model: "fake-object-detail", InputTokens: 64, OutputTokens: 32,
			}, nil
		},
		ComposeAIFirstFn: func(ctx context.Context, input ActionsAIFirstInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			if !input.IsV2CandidateInput() {
				out := buildDeterministicAIFirstOutput(input)
				return &out, &AIRawResponse{
					Model: "fake-ai-first-deterministic", RawText: "deterministic fixture",
					InputTokens: 512, OutputTokens: 128,
				}, nil
			}
			compose := buildDeterministicAIFirstV2ComposeOutput(input)
			legacy := composeOutputToLegacyOutput(&compose)
			legacy.Actions = make([]AIFirstDraftAction, 0, len(compose.Actions))
			for _, a := range compose.Actions {
				legacy.Actions = append(legacy.Actions, baseActionToDraft(AIFirstBaseAction{AIFirstComposeAction: a}))
			}
			return legacy, &AIRawResponse{
				Model: "fake-ai-first-deterministic", RawText: "deterministic fixture",
				InputTokens: 512, OutputTokens: 128,
			}, nil
		},
		RetryAIFirstFn: func(ctx context.Context, input AIFirstRetryInput) (*ActionsAIFirstOutput, *AIRawResponse, error) {
			if input.Input.IsV2CandidateInput() {
				return nil, nil, fmt.Errorf("%s: retry is not supported for actions v2", ActionsErrorValidationFailed)
			}
			out := buildDeterministicAIFirstOutput(input.Input)
			return &out, &AIRawResponse{
				Model:   "fake-ai-first-deterministic-retry",
				RawText: "deterministic fixture retry",
			}, nil
		},
	}
}

func buildDeterministicObjectDetailOutput(batch ObjectDetailBatchInput) *ObjectDetailBatchOutput {
	out := &ObjectDetailBatchOutput{
		ActionKey:  batch.Action.ActionKey,
		BatchIndex: batch.Batch.BatchIndex,
	}
	for _, ent := range batch.Entities {
		out.ObjectActions = append(out.ObjectActions, ObjectDetailObjectAction{
			EntityRef:       ent.EntityRef,
			Recommendation:  "Выполнить рекомендацию по объекту на основании доступных фактов.",
			Reason:          "Объект входит в authoritative scope Action.",
			Steps:           []string{"Проверить текущие показатели.", "Применить рекомендацию."},
			MetricRefs:      firstFactRefs(ent.AvailableFacts, 2),
			DataSufficiency: dataSufficiencyLabel(len(ent.AvailableFacts)),
		})
	}
	return out
}

func firstFactRefs(facts []ObjectDetailFactInput, max int) []string {
	out := make([]string, 0, max)
	for _, f := range facts {
		if len(out) >= max {
			break
		}
		out = append(out, f.FactRef)
	}
	return out
}

func dataSufficiencyLabel(factCount int) string {
	if factCount >= 2 {
		return "sufficient"
	}
	return "insufficient"
}

func buildDeterministicAIFirstV2Output(input ActionsAIFirstInput) ActionsAIFirstOutput {
	compose := buildDeterministicAIFirstV2ComposeOutput(input)
	out := composeOutputToLegacyOutput(&compose)
	out.Actions = make([]AIFirstDraftAction, 0, len(compose.Actions))
	for _, a := range compose.Actions {
		draft := baseActionToDraft(AIFirstBaseAction{AIFirstComposeAction: a})
		draft.SourceCandidateIDs = append([]string(nil), a.SourceCandidateIDs...)
		out.Actions = append(out.Actions, draft)
	}
	return *out
}

func buildDeterministicAIFirstV2ComposeOutput(input ActionsAIFirstInput) ActionsAIFirstComposeOutput {
	if len(input.ActionCandidates) == 0 {
		return ActionsAIFirstComposeOutput{
			SchemaVersion: input.SchemaVersion,
			Actions:       []AIFirstComposeAction{},
			Notes:         []string{"deterministic v2 empty"},
		}
	}
	groups := groupCandidatesForFake(input.ActionCandidates)
	sort.SliceStable(groups, func(i, j int) bool {
		pi := priorityOrder(groups[i].highestPriority())
		pj := priorityOrder(groups[j].highestPriority())
		if pi != pj {
			return pi < pj
		}
		if groups[i].actionType != groups[j].actionType {
			return groups[i].actionType < groups[j].actionType
		}
		return groups[i].firstCandidateID() < groups[j].firstCandidateID()
	})
	if len(groups) > 6 {
		groups = groups[:6]
	}

	actions := make([]AIFirstComposeAction, 0, len(groups))
	categories := make([]ActionCategory, 0, len(groups))
	for _, group := range groups {
		action := mergeFakeCandidateGroupCompose(input, group)
		actions = append(actions, action)
		categories = append(categories, action.Category)
	}

	focusFacts := collectUniqueFactIDsFromComposeActions(actions)
	startWith := ""
	if len(actions) > 0 {
		startWith = firstComposeStep(actions)
	}
	return ActionsAIFirstComposeOutput{
		SchemaVersion: input.SchemaVersion,
		Focus: &ActionsAIFirstFocus{
			Headline:        "Фокус: приоритетные действия по candidates",
			Summary:         "Синтез из локальных action candidates с учётом Global Context.",
			PriorityOrder:   categories,
			StartWith:       startWith,
			Limitations:     append([]string(nil), input.DataLimitations...),
			EvidenceFactIDs: focusFacts,
		},
		Actions: actions,
		Notes:   []string{"Deterministic fixture provider for AI-first v2 compose."},
	}
}

func firstComposeStep(actions []AIFirstComposeAction) string {
	for _, a := range actions {
		if len(a.Steps) > 0 {
			return a.Steps[0]
		}
		if a.NextStep != "" {
			return a.NextStep
		}
	}
	return ""
}

func collectUniqueFactIDsFromComposeActions(actions []AIFirstComposeAction) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, a := range actions {
		for _, id := range a.EvidenceFactIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

type fakeCandidateGroup struct {
	key        string
	actionType ActionType
	candidates []AIFirstActionCandidate
}

func (g fakeCandidateGroup) highestPriority() ActionPriority {
	best := ActionPriorityWatch
	for _, c := range g.candidates {
		if priorityOrder(c.LocalPriority) < priorityOrder(best) {
			best = c.LocalPriority
		}
	}
	return best
}

func (g fakeCandidateGroup) firstCandidateID() string {
	if len(g.candidates) == 0 {
		return ""
	}
	ids := make([]string, 0, len(g.candidates))
	for _, c := range g.candidates {
		ids = append(ids, c.CandidateID)
	}
	sort.Strings(ids)
	return ids[0]
}

func groupCandidatesForFake(candidates []AIFirstActionCandidate) []fakeCandidateGroup {
	byKey := map[string][]AIFirstActionCandidate{}
	order := make([]string, 0)
	for _, c := range candidates {
		key := fakeCandidateCompatibilityKey(c)
		if _, ok := byKey[key]; !ok {
			order = append(order, key)
		}
		byKey[key] = append(byKey[key], c)
	}
	sort.Strings(order)
	out := make([]fakeCandidateGroup, 0, len(order))
	for _, key := range order {
		group := byKey[key]
		sort.SliceStable(group, func(i, j int) bool {
			return group[i].CandidateID < group[j].CandidateID
		})
		out = append(out, fakeCandidateGroup{
			key:        key,
			actionType: group[0].Type,
			candidates: group,
		})
	}
	return out
}

// fakeCandidateCompatibilityKey groups candidates with the same action type,
// normalized recommended action and normalized problem text.
func fakeCandidateCompatibilityKey(c AIFirstActionCandidate) string {
	return string(c.Type) + "|" + normalizeFakeGroupText(c.RecommendedAction) + "|" + normalizeFakeGroupText(c.Problem)
}

func normalizeFakeGroupText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.Join(strings.Fields(s), " ")
}

func mergeFakeCandidateGroupCompose(input ActionsAIFirstInput, group fakeCandidateGroup) AIFirstComposeAction {
	draft := mergeFakeCandidateGroup(input, group)
	scope := draft.ScopeType
	if scope == "" {
		if len(draft.AffectedEntities) > 0 {
			scope = AIFirstActionScopeEntity
		} else {
			scope = AIFirstActionScopeAccount
		}
	}
	steps := make([]string, 0, len(draft.ActionPlan))
	for _, s := range draft.ActionPlan {
		if instr := strings.TrimSpace(s.Instruction); instr != "" {
			steps = append(steps, instr)
		}
	}
	actionKey := strings.TrimSpace(draft.OperationalGoal)
	if actionKey == "" {
		actionKey = string(draft.Type) + "_" + group.firstCandidateID()
	}
	return AIFirstComposeAction{
		ActionKey: actionKey, Type: draft.Type, Category: draft.Category,
		Priority: draft.Priority, ScopeType: scope, OperationalGoal: draft.OperationalGoal,
		SourceCandidateIDs: draft.SourceCandidateIDs, Title: draft.Title,
		TitleTemplate: draft.TitleTemplate, Summary: draft.Summary, Reason: draft.Reason,
		NextStep: draft.NextStep, Steps: steps, ActionPlan: draft.ActionPlan,
		ExpectedEffect: draft.ExpectedEffect, EvidenceFactIDs: draft.EvidenceFactIDs,
		CompletionCriteria: draft.CompletionCriteria, CompletionConditions: draft.CompletionConditions,
		ObjectDetailBrief:  "Сформируй индивидуальную рекомендацию для каждого объекта на основании available_facts.",
		VisibleLimitations: draft.VisibleLimitations,
	}
}

func mergeFakeCandidateGroup(input ActionsAIFirstInput, group fakeCandidateGroup) AIFirstDraftAction {
	sourceIDs := make([]string, 0, len(group.candidates))
	problems := make([]string, 0, len(group.candidates))
	recommendations := make([]string, 0, len(group.candidates))
	why := make([]string, 0, len(group.candidates))
	entityByKey := map[string]AIFirstAffectedEntityRef{}
	factSeen := map[string]struct{}{}
	evidenceFacts := make([]string, 0)
	objectActions := make([]AIFirstObjectAction, 0)

	for _, c := range group.candidates {
		sourceIDs = append(sourceIDs, c.CandidateID)
		if p := strings.TrimSpace(c.Problem); p != "" {
			problems = append(problems, p)
		}
		if r := strings.TrimSpace(c.RecommendedAction); r != "" {
			recommendations = append(recommendations, r)
		}
		if w := strings.TrimSpace(c.WhyActionable); w != "" {
			why = append(why, w)
		}
		for _, ent := range c.AffectedEntities {
			key := ent.EntityType + "|" + ent.EntityID
			existing, ok := entityByKey[key]
			if !ok {
				existing = ent
			} else {
				existing.FactIDs = mergeStringIDs(existing.FactIDs, ent.FactIDs)
				if existing.Name == "" {
					existing.Name = ent.Name
				}
			}
			entityByKey[key] = existing
			metrics := fakeMetricsForEntity(input, ent, c.EvidenceFactIDs)
			objectActions = append(objectActions, AIFirstObjectAction{
				EntityType:        ent.EntityType,
				EntityID:          ent.EntityID,
				Name:              resolveEntityName(input, ent),
				RecommendedAction: c.RecommendedAction,
				Reason:            c.Problem,
				EvidenceFactIDs:   append([]string(nil), ent.FactIDs...),
				Metrics:           metrics,
			})
		}
		for _, factID := range c.EvidenceFactIDs {
			factID = strings.TrimSpace(factID)
			if factID == "" {
				continue
			}
			if _, ok := factSeen[factID]; ok {
				continue
			}
			factSeen[factID] = struct{}{}
			evidenceFacts = append(evidenceFacts, factID)
		}
	}
	sort.Strings(sourceIDs)
	sort.Strings(evidenceFacts)

	affected := make([]AIFirstAffectedEntityRef, 0, len(entityByKey))
	keys := make([]string, 0, len(entityByKey))
	for k := range entityByKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ent := entityByKey[k]
		if ent.Name == "" {
			ent.Name = resolveEntityName(input, ent)
		}
		affected = append(affected, ent)
	}

	actionType := group.actionType
	category, _ := ActionCategoryForType(actionType)
	priority := mapLocalPriorityToGlobal(group.highestPriority())
	title := fakeV2Title(actionType, problems)
	reason := fakeLongReason(problems, why)
	firstStep := recommendations[0]
	if firstStep == "" {
		firstStep = "Открыть список затронутых объектов и начать с приоритетного объекта."
	}

	return AIFirstDraftAction{
		Type:               actionType,
		Category:           category,
		Priority:           priority,
		ScopeType:          AIFirstActionScopeEntity,
		OperationalGoal:    fakeOperationalGoal(actionType),
		Title:              title,
		TitleTemplate:      title + " {object_count} SKU",
		Summary:            recommendations[0],
		Reason:             reason,
		NextStep:           firstStep,
		ExpectedEffect:     "Снижение локального риска по объединённым candidates.",
		EvidenceFactIDs:    evidenceFacts,
		SourceCandidateIDs: sourceIDs,
		AffectedEntities:   affected,
		ActionPlan:         fakeDetailedActionPlan(firstStep, actionType),
		CompletionCriteria: "Подтверждено выполнение рекомендации по объединённым candidates и проверена динамика через 7 дней.",
		VisibleLimitations: append([]string(nil), input.DataLimitations...),
	}
}

func fakeOperationalGoal(actionType ActionType) string {
	switch actionType {
	case ActionTypeStockReplenishment:
		return "replenish_stock"
	case ActionTypeAdHighDRRReview:
		return "stop_ad_waste"
	default:
		return string(actionType)
	}
}

func fakeLongReason(problems, why []string) string {
	reason := strings.Join(uniqueNonEmpty(problems), "; ")
	if reason == "" {
		reason = strings.Join(uniqueNonEmpty(why), "; ")
	}
	reason = strings.TrimSpace(reason + " " +
		"По evidence_fact_ids из source candidates видны конкретные метрики по затронутым объектам. " +
		"Нужно обработать приоритетные объекты и проверить динамику заказов, остатков или расхода через 7 дней.")
	if len([]rune(reason)) < aiFirstMinObjectBasedReasonLen {
		reason += strings.Repeat(" Приоритет — объектам с наибольшим риском по доступным фактам.", 3)
	}
	return reason
}

func fakeDetailedActionPlan(firstStep string, actionType ActionType) []AIFirstActionStep {
	switch actionType {
	case ActionTypeAdHighDRRReview, ActionTypeAdBudgetReallocation, ActionTypeAdSpendNoOrders, ActionTypeAdHighDRR:
		return []AIFirstActionStep{
			{Title: "Проверить кампании", Instruction: "Открыть кампании с высоким DRR и расходом без заказов за 7 дней.", ExpectedResult: "Определены неэффективные кампании."},
			{Title: "Снизить ставки", Instruction: "Снизить ставки или ограничить бюджет по кампаниям с перерасходом.", ExpectedResult: "Расход кампаний снижен."},
			{Title: "Перераспределить бюджет", Instruction: "Перераспределить бюджет в более эффективные кампании.", ExpectedResult: "Бюджет перераспределён."},
			{Title: "Проверить результат", Instruction: "Через 7 дней проверить DRR, расход и заказы по кампаниям.", ExpectedResult: "Зафиксирована динамика рекламы."},
		}
	default:
		return []AIFirstActionStep{
			{Title: "Открыть объекты", Instruction: firstStep, ExpectedResult: "Определён порядок обработки объектов."},
			{Title: "Проверить карточки", Instruction: "По каждому SKU проверить цену, фото, название, отзывы и видимость карточки.", ExpectedResult: "Выявлены проблемы контента."},
			{Title: "Стимулировать спрос", Instruction: "Для объектов с нормальной карточкой запустить акцию, скидку или перераспределить рекламное внимание.", ExpectedResult: "Запущены меры стимулирования."},
			{Title: "Проверить динамику", Instruction: "Через 7 дней проверить заказы и остатки по каждому объекту из списка.", ExpectedResult: "Зафиксирована динамика продаж."},
		}
	}
}

func fakeMetricsForEntity(input ActionsAIFirstInput, ent AIFirstAffectedEntityRef, evidence []string) []AIFirstObjectMetric {
	metrics := make([]AIFirstObjectMetric, 0, 4)
	addFromFact := func(factID string) {
		factID = strings.TrimSpace(factID)
		if factID == "" {
			return
		}
		fact, ok := input.factByID(factID)
		if !ok {
			return
		}
		label := strings.TrimSpace(fact.Metric)
		if label == "" {
			label = factID
		}
		value := strings.TrimSpace(fact.FormattedValue)
		if value == "" {
			value = fmt.Sprint(fact.Value)
		}
		metrics = append(metrics, AIFirstObjectMetric{Label: label, Value: value, Unit: fact.Unit})
	}
	for _, id := range ent.FactIDs {
		addFromFact(id)
	}
	for _, id := range evidence {
		addFromFact(id)
	}
	if len(metrics) >= aiFirstMinObjectActionMetrics {
		return metrics
	}
	switch strings.ToLower(strings.TrimSpace(ent.EntityType)) {
	case "campaign":
		return []AIFirstObjectMetric{{Label: "DRR", Value: "высокий"}, {Label: "Расход 7д", Value: "перерасход"}}
	default:
		return []AIFirstObjectMetric{{Label: "Остаток", Value: "дефицит"}, {Label: "Заказы 7д", Value: "0"}}
	}
}

func fakeV2Title(actionType ActionType, problems []string) string {
	switch actionType {
	case ActionTypeStockReplenishment:
		return "Пополнить SKU по объединённым stock candidates"
	case ActionTypeAdHighDRRReview:
		return "Снизить DRR по объединённым advertising candidates"
	default:
		if len(problems) > 0 {
			return problems[0]
		}
		return string(actionType) + ": действие по candidates"
	}
}

func mapLocalPriorityToGlobal(p ActionPriority) ActionPriority {
	switch p {
	case ActionPriorityCritical:
		return ActionPriorityCritical
	case ActionPriorityImportant:
		return ActionPriorityImportant
	default:
		return ActionPriorityWatch
	}
}

func resolveEntityName(input ActionsAIFirstInput, ent AIFirstAffectedEntityRef) string {
	if strings.TrimSpace(ent.Name) != "" {
		return ent.Name
	}
	if ce, ok := input.EntityCatalog[ent.EntityID]; ok && strings.TrimSpace(ce.Name) != "" {
		return ce.Name
	}
	return ent.EntityID
}

func mergeStringIDs(a, b []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(a)+len(b))
	for _, id := range append(append([]string(nil), a...), b...) {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func uniqueNonEmpty(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func collectUniqueFactIDsFromActions(actions []AIFirstDraftAction) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, a := range actions {
		for _, id := range a.EvidenceFactIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func buildDeterministicAIFirstOutput(input ActionsAIFirstInput) ActionsAIFirstOutput {
	skuEntityID, skuName := firstSKUEntity(input)
	campaignEntityID, campaignName := firstCampaignEntity(input)
	skuFactID, campaignFactID := firstFacts(input)
	return ActionsAIFirstOutput{
		SchemaVersion: input.SchemaVersion,
		Focus: &ActionsAIFirstFocus{
			Headline:        "Фокус: пополнение SKU и контроль рекламы",
			Summary:         "Сначала закрываем риск out-of-stock на SKU, затем снижаем DRR в дорогой кампании.",
			PriorityOrder:   []ActionCategory{ActionCategoryStock, ActionCategoryAdvertising},
			StartWith:       "Проверьте поставку SKU из первого действия.",
			Limitations:     []string{"Атрибуция рекламы доступна только на уровне кампаний."},
			EvidenceFactIDs: []string{skuFactID, campaignFactID},
		},
		Actions: []AIFirstDraftAction{
			{
				Type:            ActionTypeStockReplenishment,
				Category:        ActionCategoryStock,
				Priority:        ActionPriorityCritical,
				Title:           "Пополнить SKU с риском out-of-stock",
				Summary:         "Приоритетные SKU рискуют остаться без остатка в ближайшие дни.",
				Reason:          fakeLongReason([]string{"DOC ниже целевого окна и фиксируется падение доступного остатка по SKU."}, nil),
				NextStep:        "Согласуйте поставку по SKU из списка объектных действий.",
				ExpectedEffect:  "Снижается риск потери выручки из-за отсутствия товара, доступность SKU восстанавливается.",
				EvidenceFactIDs: []string{skuFactID},
				CTAKeys:         []string{ActionCTAAskAI, ActionCTAMarkChecked},
				AffectedEntities: []AIFirstAffectedEntityRef{
					{EntityType: "sku", EntityID: skuEntityID, Name: skuName, FactIDs: []string{skuFactID}},
				},
				UrgencyMarker:          AIFirstUrgencyUrgent,
				AffectedObjectsSummary: "Показан SKU с максимальным риском out-of-stock.",
				ObjectActions: []AIFirstObjectAction{
					{
						EntityType:        "sku",
						EntityID:          skuEntityID,
						Name:              skuName,
						Metrics:           []AIFirstObjectMetric{{Label: "DOC", Value: "низкий"}, {Label: "Остаток", Value: "дефицит"}},
						RecommendedAction: "Добавить SKU в ближайшую поставку и проверить безопасный запас.",
						Reason:            "Низкий запас при текущем темпе продаж.",
						EvidenceFactIDs:   []string{skuFactID},
					},
				},
				ActionPlan: []AIFirstActionStep{
					{Title: "Проверить остатки", Instruction: "Сверьте доступный остаток и темп заказов по SKU.", ExpectedResult: "Подтверждён дефицит по SKU."},
					{Title: "Запланировать поставку", Instruction: "Создайте поставку с безопасным запасом.", ExpectedResult: "SKU включён в поставку."},
					{Title: "Проверить поступление", Instruction: "Через 7 дней проверьте остаток и заказы по SKU после поставки.", ExpectedResult: "Доступность SKU восстановлена."},
				},
				CompletionCriteria: "SKU переведён в безопасный запас и подтверждён к поставке.",
				VisibleLimitations: []string{"Срок приемки поставки зависит от внешней логистики."},
			},
			{
				Type:            ActionTypeAdHighDRRReview,
				Category:        ActionCategoryAdvertising,
				Priority:        ActionPriorityImportant,
				Title:           "Снизить DRR в кампании с перерасходом",
				Summary:         "Кампания показывает повышенный DRR относительно допустимого уровня.",
				Reason:          fakeLongReason([]string{"Расходы растут быстрее выручки, эффективность кампании снижается."}, nil),
				NextStep:        "Снизьте ставки и отключите неэффективные группы объявлений.",
				ExpectedEffect:  "Снижается DRR кампании и высвобождается бюджет для более эффективных кампаний.",
				EvidenceFactIDs: []string{campaignFactID},
				CTAKeys:         []string{ActionCTAAskAI, ActionCTAMarkChecked},
				AffectedEntities: []AIFirstAffectedEntityRef{
					{EntityType: "campaign", EntityID: campaignEntityID, Name: campaignName, FactIDs: []string{campaignFactID}},
				},
				UrgencyMarker:          AIFirstUrgencyWatch,
				AffectedObjectsSummary: "Показана 1 кампания с наибольшим DRR.",
				ObjectActions: []AIFirstObjectAction{
					{
						EntityType:        "campaign",
						EntityID:          campaignEntityID,
						Name:              campaignName,
						Metrics:           []AIFirstObjectMetric{{Label: "DRR", Value: "высокий"}, {Label: "Расход", Value: "перерасход"}},
						RecommendedAction: "Снизить ставки и остановить сегменты без заказов.",
						Reason:            "Кампания тратит бюджет с низкой отдачей.",
						EvidenceFactIDs:   []string{campaignFactID},
					},
				},
				ActionPlan: []AIFirstActionStep{
					{Title: "Проверить сегменты", Instruction: "Найдите сегменты кампании с расходом без заказов.", ExpectedResult: "Определены неэффективные сегменты."},
					{Title: "Перенастроить ставки", Instruction: "Снизьте ставки и перераспределите бюджет.", ExpectedResult: "DRR кампании снижается."},
					{Title: "Проверить результат", Instruction: "Через 7 дней проверьте DRR, расход и заказы по кампании.", ExpectedResult: "Зафиксирована динамика рекламы."},
				},
				CompletionCriteria: "DRR кампании снижен до целевого диапазона и стабилен.",
				VisibleLimitations: []string{"Атрибуция и эффективность рекламы доступны на уровне кампании, не SKU."},
			},
		},
		Notes: []string{"Deterministic fixture provider for AI-first stage stabilization."},
	}
}

func firstSKUEntity(input ActionsAIFirstInput) (string, string) {
	pack, err := input.requireDataPack()
	if err != nil || pack == nil {
		return "sku:unknown", "SKU (fixture)"
	}
	ids := make([]string, 0, len(pack.EntityRegistry.SKUs))
	for id := range pack.EntityRegistry.SKUs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return "sku:unknown", "SKU (fixture)"
	}
	sku := pack.EntityRegistry.SKUs[ids[0]]
	name := strings.TrimSpace(sku.Name)
	if name == "" {
		name = strings.TrimSpace(sku.SKU)
	}
	if name == "" {
		name = ids[0]
	}
	return ids[0], name
}

func firstCampaignEntity(input ActionsAIFirstInput) (string, string) {
	pack, err := input.requireDataPack()
	if err != nil || pack == nil {
		return "campaign:unknown", "Campaign (fixture)"
	}
	ids := make([]string, 0, len(pack.EntityRegistry.Campaigns))
	for id := range pack.EntityRegistry.Campaigns {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return "campaign:unknown", "Campaign (fixture)"
	}
	campaign := pack.EntityRegistry.Campaigns[ids[0]]
	name := strings.TrimSpace(campaign.Name)
	if name == "" {
		name = strings.TrimSpace(campaign.CampaignExternalID)
	}
	if name == "" {
		name = ids[0]
	}
	return ids[0], name
}

func firstFacts(input ActionsAIFirstInput) (string, string) {
	pack, err := input.requireDataPack()
	if err != nil || pack == nil {
		return "account.last_7d.revenue", "account.last_7d.revenue"
	}
	skuFactID := ""
	campaignFactID := ""
	if len(pack.Tables.SKURollups) > 0 && len(pack.Tables.SKURollups[0].FactIDs) > 0 {
		skuFactID = strings.TrimSpace(pack.Tables.SKURollups[0].FactIDs[0])
	}
	if len(pack.Tables.CampaignRollups) > 0 && len(pack.Tables.CampaignRollups[0].FactIDs) > 0 {
		campaignFactID = strings.TrimSpace(pack.Tables.CampaignRollups[0].FactIDs[0])
	}
	if skuFactID == "" || campaignFactID == "" {
		allFactIDs := make([]string, 0, len(pack.FactRegistry))
		for factID := range pack.FactRegistry {
			allFactIDs = append(allFactIDs, factID)
		}
		sort.Strings(allFactIDs)
		for _, factID := range allFactIDs {
			if skuFactID == "" && strings.HasPrefix(factID, "sku.") {
				skuFactID = factID
			}
			if campaignFactID == "" && strings.HasPrefix(factID, "campaign.") {
				campaignFactID = factID
			}
		}
		if skuFactID == "" && len(allFactIDs) > 0 {
			skuFactID = allFactIDs[0]
		}
		if campaignFactID == "" && len(allFactIDs) > 0 {
			campaignFactID = allFactIDs[0]
		}
	}
	if skuFactID == "" {
		skuFactID = "account.last_7d.revenue"
	}
	if campaignFactID == "" {
		campaignFactID = skuFactID
	}
	return skuFactID, campaignFactID
}
