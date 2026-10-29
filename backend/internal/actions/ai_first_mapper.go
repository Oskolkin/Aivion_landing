package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// BuildAIFirstActionKey builds a stable deterministic key for AI-first actions.
// AI-first actions have no source candidates, so the key is derived from the
// metric date, action type and the affected entity set.
func BuildAIFirstActionKey(metricDate string, actionType ActionType, entityIDs []string) string {
	ids := append([]string(nil), entityIDs...)
	sort.Strings(ids)
	joined := strings.Join(ids, ".")
	base := fmt.Sprintf("%s:%s:ai_first:%s", metricDate, actionType, joined)
	base = strings.ReplaceAll(base, "/", "_")
	if len(base) <= 200 && actionKeyPattern.MatchString(base) {
		return base
	}
	h := sha256.Sum256([]byte(base))
	return fmt.Sprintf("%s:%s:ai_first:%s", metricDate, actionType, hex.EncodeToString(h[:8]))
}

// MapAIFirstActionToItem converts a validated AI-first draft into a persistable
// ActionItem. It never rewrites AI-authored meaning (title/reason/plan/type):
// it only resolves evidence facts, entity names and deterministic CTAs.
func MapAIFirstActionToItem(metricDate string, input ActionsAIFirstInput, action AIFirstDraftAction) ActionItem {
	entityIDs := make([]string, 0, len(action.AffectedEntities))
	for _, ent := range action.AffectedEntities {
		entityIDs = append(entityIDs, ent.EntityID)
	}

	urgency := strings.TrimSpace(action.UrgencyMarker)
	if urgency == "" {
		switch action.Priority {
		case ActionPriorityCritical:
			urgency = AIFirstUrgencyUrgent
		case ActionPriorityWatch:
			urgency = AIFirstUrgencyWatch
		}
	}

	visibleLimitations := append([]string(nil), action.VisibleLimitations...)
	if len(visibleLimitations) == 0 {
		visibleLimitations = append(visibleLimitations, action.Limitations...)
	}

	item := ActionItem{
		ActionKey:                BuildAIFirstActionKey(metricDate, action.Type, entityIDs),
		Type:                     action.Type,
		Category:                 action.Category,
		Priority:                 action.Priority,
		Status:                   ActionStatusOpen,
		Title:                    action.Title,
		Summary:                  action.Summary,
		Reason:                   action.Reason,
		NextStep:                 resolveAIFirstNextStep(action),
		ExpectedEffect:           action.ExpectedEffect,
		Evidence:                 buildAIFirstEvidence(input, action.EvidenceFactIDs),
		Limitations:              visibleLimitations,
		CTAs:                     buildAIFirstCTAs(),
		AffectedEntities:         buildAIFirstAffectedEntities(input, action),
		SourceCandidateIDs:       append([]string(nil), action.SourceCandidateIDs...),
		EvidenceFactIDs:          append([]string(nil), action.EvidenceFactIDs...),
		EntityGroupRef:           "",
		OperationalDetail:        buildAIFirstOperationalDetail(action),
		UrgencyMarker:            urgency,
		AffectedObjectsSummary:   action.AffectedObjectsSummary,
		AffectedObjectsTotal:     actionAffectedObjectsTotal(action),
		AffectedObjectsDisplayed: actionAffectedObjectsDisplayed(action),
		AffectedObjectsTruncated: action.AffectedObjectsTruncated,
		OperationalGoal:          strings.TrimSpace(action.OperationalGoal),
		ScopeType:                string(normalizeAIFirstScopeType(string(action.ScopeType))),
		ObjectActionsCoverage:    mapObjectActionsCoverageToAPI(action.ObjectActionsCoverage),
		CompletionCriteria:       action.CompletionCriteria,
		CompletionConditions:     mapCompletionConditionsToAPI(action.CompletionConditions),
		VisibleLimitations:       visibleLimitations,
		MetricDate:               metricDate,
	}
	return item
}

// MapAIFirstActionToItemWithActionKey maps a draft to ActionItem preserving a fixed action_key.
func MapAIFirstActionToItemWithActionKey(metricDate string, input ActionsAIFirstInput, action AIFirstDraftAction, actionKey string) ActionItem {
	item := MapAIFirstActionToItem(metricDate, input, action)
	if strings.TrimSpace(actionKey) != "" {
		item.ActionKey = actionKey
	}
	return item
}

func resolveAIFirstNextStep(action AIFirstDraftAction) string {
	if strings.TrimSpace(action.NextStep) != "" {
		return action.NextStep
	}
	if len(action.ActionPlan) > 0 {
		return action.ActionPlan[0].Instruction
	}
	return ""
}

// buildAIFirstCTAs returns the deterministic CTA set. The model never
// generates CTAs; the UI renders Ask AI / Mark completed / Mark irrelevant.
func buildAIFirstCTAs() []ActionCTA {
	return []ActionCTA{
		{Type: "ask_ai", Label: "Спросить AI по этому действию", Params: map[string]string{"cta_key": ActionCTAAskAI}},
		{Type: "mark_checked", Label: "Отметить выполненным", Params: map[string]string{"cta_key": ActionCTAMarkChecked}},
		{Type: "mark_irrelevant", Label: "Неактуально", Params: map[string]string{"cta_key": "mark_irrelevant"}},
	}
}

func buildAIFirstEvidence(input ActionsAIFirstInput, factIDs []string) []ActionEvidence {
	out := make([]ActionEvidence, 0, len(factIDs))
	for _, id := range factIDs {
		fact, ok := input.factByID(id)
		if !ok {
			continue
		}
		value := strings.TrimSpace(fact.FormattedValue)
		unit := ""
		if value == "" {
			value = fmt.Sprint(fact.Value)
			unit = fact.Unit
		}
		out = append(out, ActionEvidence{
			Kind:  "metric",
			Label: factEvidenceLabel(fact.Metric, fact.Window),
			Value: value,
			Unit:  unit,
		})
	}
	return out
}

func factEvidenceLabel(metric, window string) string {
	if strings.TrimSpace(window) == "" {
		return metric
	}
	return fmt.Sprintf("%s (%s)", metric, window)
}

func buildAIFirstAffectedEntities(input ActionsAIFirstInput, action AIFirstDraftAction) ActionAffectedEntities {
	var out ActionAffectedEntities
	seen := make(map[string]bool)
	addRef := func(entityType, entityID, name string) {
		key := entityType + "|" + entityID
		if seen[key] {
			return
		}
		seen[key] = true
		switch entityType {
		case "sku":
			ref := ActionSKURef{SKU: entityID, Name: name}
			if input.IsV2CandidateInput() {
				if e, ok := input.EntityCatalog[entityID]; ok {
					ref.SKU = e.SKU
					ref.OfferID = e.OfferID
					if ref.Name == "" {
						ref.Name = e.Name
					}
				}
			}
			pack, err := input.requireDataPack()
			if err == nil && pack != nil {
				if e, ok := pack.EntityRegistry.SKUs[entityID]; ok {
					if ref.SKU == "" {
						ref.SKU = e.SKU
					}
					ref.OfferID = e.OfferID
					if ref.Name == "" {
						ref.Name = e.Name
					}
				}
			}
			out.SKUs = append(out.SKUs, ref)
		case "campaign":
			ref := ActionCampaignRef{CampaignID: entityID, Name: name}
			if input.IsV2CandidateInput() {
				if e, ok := input.EntityCatalog[entityID]; ok {
					ref.CampaignID = e.CampaignExternalID
					if ref.Name == "" {
						ref.Name = e.Name
					}
				}
			}
			pack, err := input.requireDataPack()
			if err == nil && pack != nil {
				if e, ok := pack.EntityRegistry.Campaigns[entityID]; ok {
					ref.CampaignID = e.CampaignExternalID
					if ref.Name == "" {
						ref.Name = e.Name
					}
				}
			}
			out.Campaigns = append(out.Campaigns, ref)
		case "funnel":
			if out.Funnel == nil {
				out.Funnel = &ActionFunnelRef{TransitionKey: entityID}
			}
		}
	}
	for _, ent := range action.AffectedEntities {
		addRef(strings.TrimSpace(ent.EntityType), strings.TrimSpace(ent.EntityID), ent.Name)
	}
	for _, obj := range action.ObjectActions {
		addRef(strings.TrimSpace(obj.EntityType), strings.TrimSpace(obj.EntityID), obj.Name)
	}
	return out
}

// buildAIFirstOperationalDetail projects the AI-first action into the existing
// operational detail contract so the detail UI renders why / plan / objects.
func actionAffectedObjectsTotal(action AIFirstDraftAction) int {
	if action.AffectedObjectsTotal > 0 {
		return action.AffectedObjectsTotal
	}
	return len(action.AffectedEntities)
}

func actionAffectedObjectsDisplayed(action AIFirstDraftAction) int {
	if action.AffectedObjectsDisplayed > 0 {
		return action.AffectedObjectsDisplayed
	}
	return len(action.ObjectActions)
}

func buildAIFirstOperationalDetail(action AIFirstDraftAction) *OperationalDetail {
	if action.OperationalDetail != nil {
		return cloneOperationalDetail(action.OperationalDetail)
	}
	steps := make([]OperationalStep, 0, len(action.ActionPlan))
	for i, step := range action.ActionPlan {
		title := strings.TrimSpace(step.Title)
		if title == "" {
			title = fmt.Sprintf("Шаг %d", i+1)
		}
		steps = append(steps, OperationalStep{
			StepID:         fmt.Sprintf("step_%d", i+1),
			Title:          title,
			Instruction:    step.Instruction,
			ExpectedResult: step.ExpectedResult,
		})
	}
	objects := make([]OperationalObjectAction, 0, len(action.ObjectActions))
	for _, obj := range action.ObjectActions {
		metrics := make([]OperationalMetricRef, 0, len(obj.Metrics))
		for _, m := range obj.Metrics {
			label := firstNonEmptyText(m.DisplayLabel, m.Label)
			value := firstNonEmptyText(m.FormattedValue, m.Value)
			if strings.TrimSpace(m.Unit) != "" && value == m.Value {
				value = strings.TrimSpace(value + " " + m.Unit)
			}
			metrics = append(metrics, OperationalMetricRef{
				Label:          label,
				Value:          value,
				MetricCode:     m.MetricCode,
				DisplayLabel:   label,
				FormattedValue: firstNonEmptyText(m.FormattedValue, value),
			})
		}
		objects = append(objects, OperationalObjectAction{
			EntityType:        obj.EntityType,
			EntityID:          obj.EntityID,
			Name:              obj.Name,
			Metrics:           metrics,
			RecommendedAction: obj.RecommendedAction,
			Reason:            obj.Reason,
		})
	}
	if len(steps) == 0 && len(objects) == 0 {
		return nil
	}
	return &OperationalDetail{
		Status:        "available",
		FormatVersion: OperationalDetailFormatVersion,
		Why: OperationalWhy{
			Headline: action.Title,
			Text:     action.Reason,
		},
		ActionPlan: OperationalActionPlan{
			Headline: "Что сделать",
			Steps:    steps,
		},
		ObjectActions: objects,
	}
}

// BuildAIFocusFromAIFirst maps the AI-first focus into the API focus contract.
// When the AI focus is missing or inconsistent, a deterministic focus is built
// from accepted actions.
func BuildAIFocusFromAIFirst(focus *ActionsAIFirstFocus, focusValid bool, accepted []ActionItem) *ActionsAIFocus {
	if focus != nil && focusValid {
		out := &ActionsAIFocus{
			Status:      "available",
			Source:      "ai_first",
			Headline:    focus.Headline,
			Summary:     focus.Summary,
			Limitations: append([]string(nil), focus.Limitations...),
			StartWith:   focus.StartWith,
		}
		for _, cat := range focus.PriorityOrder {
			out.PriorityOrder = append(out.PriorityOrder, string(cat))
			out.TopCategories = append(out.TopCategories, string(cat))
		}
		return out
	}
	return buildDeterministicAIFirstFocus(accepted)
}

func buildDeterministicAIFirstFocus(accepted []ActionItem) *ActionsAIFocus {
	if len(accepted) == 0 {
		return &ActionsAIFocus{
			Status:   "available",
			Source:   "deterministic",
			Headline: "Явных действий нет",
			Summary:  "По текущим данным Aivion не нашёл действий, требующих внимания, или данных недостаточно для уверенных выводов.",
		}
	}
	categories := make([]string, 0)
	seen := make(map[string]bool)
	for _, item := range accepted {
		c := string(item.Category)
		if !seen[c] {
			seen[c] = true
			categories = append(categories, c)
		}
	}
	return &ActionsAIFocus{
		Status:        "available",
		Source:        "deterministic",
		Headline:      accepted[0].Title,
		Summary:       fmt.Sprintf("Сформировано действий: %d. Начните с самого приоритетного действия.", len(accepted)),
		StartWith:     accepted[0].Title,
		TopCategories: categories,
		PriorityOrder: categories,
	}
}
