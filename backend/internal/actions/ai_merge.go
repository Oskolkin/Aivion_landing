package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// MergeAndDedup merges domain actions and removes duplicates.
func MergeAndDedup(actions []ScoredDraftAction) []ScoredDraftAction {
	if len(actions) == 0 {
		return nil
	}

	// Drop duplicate candidate/type/entity_group/domain
	seen := make(map[string]int)
	out := make([]ScoredDraftAction, 0, len(actions))
	for _, a := range actions {
		key := dedupKey(a)
		if idx, ok := seen[key]; ok {
			if specificityScore(a.Action) > specificityScore(out[idx].Action) {
				out[idx] = a
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, a)
	}

	// Drop duplicate actions for same single candidate — keep more specific
	byCandidate := make(map[string]int)
	filtered := make([]ScoredDraftAction, 0, len(out))
	for _, a := range out {
		if len(a.Action.SourceCandidateIDs) != 1 {
			filtered = append(filtered, a)
			continue
		}
		cid := a.Action.SourceCandidateIDs[0]
		if idx, ok := byCandidate[cid]; ok {
			if specificityScore(a.Action) > specificityScore(filtered[idx].Action) {
				filtered[idx] = a
			}
			continue
		}
		byCandidate[cid] = len(filtered)
		filtered = append(filtered, a)
	}

	return filtered
}

func dedupKey(a ScoredDraftAction) string {
	ids := append([]string(nil), a.Action.SourceCandidateIDs...)
	sort.Strings(ids)
	return fmt.Sprintf("%s|%s|%s|%s", a.Domain, a.Action.Type, strings.Join(ids, ","), a.EntityGroupRef)
}

func specificityScore(a DomainAIAction) int {
	score := len(a.EvidenceFactIDs)*2 + len(a.CTAKeys) + len(strings.Fields(a.Title))
	if strings.TrimSpace(a.Summary) != "" {
		score++
	}
	if strings.TrimSpace(a.ExpectedEffect) != "" {
		score++
	}
	if a.OperationalDetail != nil && len(a.OperationalDetail.ActionPlan.Steps) > 0 {
		score += len(a.OperationalDetail.ActionPlan.Steps) * 3
	}
	return score
}

// BuildActionKey creates a stable deterministic action key.
func BuildActionKey(metricDate string, actionType ActionType, sourceCandidateIDs []string, entityGroupRef string) string {
	ids := append([]string(nil), sourceCandidateIDs...)
	sort.Strings(ids)
	joined := strings.Join(ids, ".")
	base := fmt.Sprintf("%s:%s:%s:%s", metricDate, actionType, joined, entityGroupRef)
	base = strings.ReplaceAll(base, "/", "_")
	if len(base) <= 200 && actionKeyPattern.MatchString(base) {
		return base
	}
	h := sha256.Sum256([]byte(base))
	return fmt.Sprintf("%s:%s:%s", metricDate, actionType, hex.EncodeToString(h[:8]))
}

// MapDomainActionToItem converts a validated draft into a final ActionItem (without DB id).
func MapDomainActionToItem(
	metricDate string,
	runID int64,
	draft ScoredDraftAction,
	candidateCtx *ActionCandidateContext,
) ActionItem {
	action := draft.Action
	entityGroupRef := draft.EntityGroupRef

	evidence := buildEvidence(action.EvidenceFactIDs, draft.SourceCandidates)
	ctas := buildCTAs(canonicalCTAKeys(action.CTAKeys, draft.SourceCandidates))
	affected := buildAffectedEntities(draft.SourceCandidates)

	key := BuildActionKey(metricDate, action.Type, action.SourceCandidateIDs, entityGroupRef)
	_ = candidateCtx // reserved for future enrichment

	return ActionItem{
		ActionKey:          key,
		Type:               action.Type,
		Category:           action.Category,
		Priority:           draft.Priority,
		Status:             ActionStatusOpen,
		Title:              action.Title,
		Summary:            action.Summary,
		Reason:             action.Reason,
		NextStep:           action.NextStep,
		ExpectedEffect:     action.ExpectedEffect,
		Evidence:           evidence,
		Limitations:        action.Limitations,
		CTAs:               ctas,
		AffectedEntities:   affected,
		SourceCandidateIDs: append([]string(nil), action.SourceCandidateIDs...),
		EvidenceFactIDs:    append([]string(nil), action.EvidenceFactIDs...),
		EntityGroupRef:     entityGroupRef,
		OperationalDetail:  cloneOperationalDetail(action.OperationalDetail),
		MetricDate:         metricDate,
	}
}

func buildEvidence(factIDs []string, candidates []ActionCandidate) []ActionEvidence {
	factMap := collectFactsFromCandidates(candidates)
	out := make([]ActionEvidence, 0, len(factIDs))
	for _, id := range factIDs {
		f, ok := factMap[id]
		if !ok {
			continue
		}
		out = append(out, ActionEvidence{
			Kind:        f.Kind,
			Label:       f.Label,
			Value:       f.Value,
			Unit:        f.Unit,
			Description: f.Description,
		})
	}
	return out
}

// ctaOrderGroup returns a sort weight so CTAs render consistently:
// navigate/open (0) -> export (1) -> ask_ai (2) -> mark_checked (3) -> other (4).
func ctaOrderGroup(key string) int {
	switch key {
	case ActionCTAOpenSKUList, ActionCTAOpenCampaignList:
		return 0
	case ActionCTAExportStockReplenishment, ActionCTAExportNoSalesSKUs, ActionCTAExportSKUOrdersDrop,
		ActionCTAExportHighDRRCampaigns, ActionCTAExportNoOrderCampaigns:
		return 1
	case ActionCTAAskAI:
		return 2
	case ActionCTAMarkChecked:
		return 3
	default:
		return 4
	}
}

// canonicalCTAKeys produces the final, ordered CTA set for an action. It uses the
// backend allowlist (candidate AllowedCTAs) as the source of truth so every action
// consistently exposes navigate/export/ask_ai/mark_checked when permitted, regardless
// of whether the action came from the AI composer or the deterministic fallback.
// It never introduces a CTA outside the candidate allowlist.
func canonicalCTAKeys(actionKeys []string, candidates []ActionCandidate) []string {
	allowed := unionAllowedCTAs(candidates)
	set := make(map[string]bool)
	keys := make([]string, 0, len(allowed)+len(actionKeys))
	addKey := func(k string) {
		if k == "" || set[k] {
			return
		}
		// Respect the allowlist when we have one; otherwise trust action keys.
		if len(allowed) > 0 && !allowed[k] {
			return
		}
		set[k] = true
		keys = append(keys, k)
	}
	// Prefer the full curated allowlist so navigate + ask_ai are always present.
	for k := range allowed {
		addKey(k)
	}
	// Keep any action-selected keys that are still in the allowlist.
	for _, k := range actionKeys {
		addKey(k)
	}
	// Always offer ask_ai when permitted.
	if len(allowed) == 0 || allowed[ActionCTAAskAI] {
		addKey(ActionCTAAskAI)
	}
	return orderCTAKeys(keys)
}

func orderCTAKeys(keys []string) []string {
	out := append([]string(nil), keys...)
	sort.SliceStable(out, func(i, j int) bool {
		gi, gj := ctaOrderGroup(out[i]), ctaOrderGroup(out[j])
		if gi != gj {
			return gi < gj
		}
		return out[i] < out[j]
	})
	return out
}

func buildCTAs(keys []string) []ActionCTA {
	out := make([]ActionCTA, 0, len(keys))
	for _, k := range keys {
		if cta, ok := ctaKeyToActionCTA(k); ok {
			out = append(out, cta)
		}
	}
	return out
}

func ctaKeyToActionCTA(key string) (ActionCTA, bool) {
	labels := map[string]ActionCTA{
		ActionCTAOpenSKUList:              {Type: "navigate", Label: "Открыть список SKU", Href: "/app/products"},
		ActionCTAOpenCampaignList:         {Type: "navigate", Label: "Открыть кампании", Href: "/app/advertising"},
		ActionCTAExportStockReplenishment: {Type: "export", Label: "Экспорт пополнения", ExportType: "stock_replenishment"},
		ActionCTAExportNoSalesSKUs:        {Type: "export", Label: "Экспорт SKU без продаж", ExportType: "no_sales_skus"},
		ActionCTAExportSKUOrdersDrop:      {Type: "export", Label: "Экспорт падения заказов", ExportType: "sku_orders_drop"},
		ActionCTAExportHighDRRCampaigns:   {Type: "export", Label: "Экспорт кампаний с высоким ДРР", ExportType: "high_drr_campaigns"},
		ActionCTAExportNoOrderCampaigns:   {Type: "export", Label: "Экспорт кампаний без заказов", ExportType: "no_order_campaigns"},
		ActionCTAAskAI:                    {Type: "ask_ai", Label: "Спросить AI"},
		ActionCTAMarkChecked:              {Type: "mark_checked", Label: "Отметить выполненным"},
	}
	cta, ok := labels[key]
	if !ok {
		return ActionCTA{}, false
	}
	cta.Params = map[string]string{"cta_key": key}
	return cta, true
}

func buildAffectedEntities(candidates []ActionCandidate) ActionAffectedEntities {
	var out ActionAffectedEntities
	for _, c := range candidates {
		for _, ex := range c.TopExamples {
			switch ex.EntityType {
			case "sku":
				out.SKUs = append(out.SKUs, ActionSKURef{
					SKU:  ex.EntityID,
					Name: ex.Name,
				})
			case "campaign":
				out.Campaigns = append(out.Campaigns, ActionCampaignRef{
					CampaignID: ex.EntityID,
					Name:       ex.Name,
				})
			}
		}
		if c.Type == ActionTypeFunnelWeakTransition {
			from := stringFromAgg(c.Aggregates, "from_stage")
			to := stringFromAgg(c.Aggregates, "to_stage")
			key := stringFromAgg(c.Aggregates, "transition_key")
			if from != "" || to != "" {
				out.Funnel = &ActionFunnelRef{
					TransitionKey: key,
					FromStage:     from,
					ToStage:       to,
				}
			}
		}
	}
	return out
}

func resolveSourceCandidates(candidateCtx *ActionCandidateContext, action DomainAIAction) []ActionCandidate {
	out := make([]ActionCandidate, 0, len(action.SourceCandidateIDs))
	idSet := make(map[string]bool, len(action.SourceCandidateIDs))
	for _, id := range action.SourceCandidateIDs {
		idSet[id] = true
	}
	for _, d := range candidateCtx.Domains {
		for _, c := range d.Candidates {
			if idSet[c.CandidateID] {
				out = append(out, c)
			}
		}
	}
	return out
}

func entityGroupRefFromCandidates(candidates []ActionCandidate) string {
	for _, c := range candidates {
		if c.EntityGroupRef != nil {
			return c.EntityGroupRef.GroupKey
		}
	}
	return ""
}
