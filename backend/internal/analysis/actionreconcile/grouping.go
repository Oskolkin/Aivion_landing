package actionreconcile

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
)

func Group(intents []NewProposalIntent, catalog []actioncatalog.Definition) []GroupIntent {
	byKey := map[string]DefinitionLite{}
	for _, d := range catalog {
		byKey[d.ActionTypeKey] = DefinitionLite{Family: d.ActionFamily, Kind: string(d.TargetKind)}
	}
	buckets := map[string][]NewProposalIntent{}
	order := []string{}
	for _, it := range intents {
		if !it.Visible {
			continue
		}
		ch := ContextHash(it.NormalizedContext)
		k := it.RecommendationRef + "|" + it.ActionType + "|" + string(it.Target.Kind) + "|" + ch
		if _, ok := buckets[k]; !ok {
			order = append(order, k)
		}
		buckets[k] = append(buckets[k], it)
	}
	var out []GroupIntent
	gi := 0
	for _, k := range order {
		mem := buckets[k]
		if len(mem) < 2 {
			continue
		}
		gi++
		refs := make([]string, 0, len(mem))
		for _, m := range mem {
			refs = append(refs, m.LocalRef)
		}
		fam := byKey[mem[0].ActionType].Family
		out = append(out, GroupIntent{
			LocalRef: fmt.Sprintf("G%03d", gi), RecommendationRef: mem[0].RecommendationRef,
			ActionType: mem[0].ActionType, TargetKind: string(mem[0].Target.Kind), Family: fam,
			NormalizedContextHash: ContextHash(mem[0].NormalizedContext), MemberLocalRefs: refs,
			DisplayOrder: mem[0].DisplayOrder, DisplayTitle: mem[0].ActionType,
			DisplaySummary: fmt.Sprintf("%d members", len(mem)), PresentationKey: "ATOMIC_GROUP_V1",
		})
	}
	if out == nil {
		out = []GroupIntent{}
	}
	return out
}

type DefinitionLite struct {
	Family string
	Kind   string
}
