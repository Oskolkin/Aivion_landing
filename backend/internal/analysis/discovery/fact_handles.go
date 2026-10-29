package discovery

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type Derivation struct {
	FormulaKey     string   `json:"formula_key"`
	FormulaVersion string   `json:"formula_version"`
	InputFactRefs  []string `json:"input_fact_refs"`
}

type HandleEntry struct {
	Handle              string      `json:"handle"`
	Entity              PackEntity  `json:"entity"`
	SemanticPath        string      `json:"semantic_path"`
	AnalyticalUsability Usability   `json:"analytical_usability"`
	FactRefs            []string    `json:"fact_refs"`
	Derivation          *Derivation `json:"derivation,omitempty"`
}

type FactHandleMapV1 struct {
	SchemaVersion   string        `json:"schema_version"`
	RegistryVersion string        `json:"registry_version"`
	Entries         []HandleEntry `json:"entries"`
}

func (m FactHandleMapV1) Hash() string { return canonicaljson.Hash(m) }

func (m FactHandleMapV1) ByHandle() map[string]HandleEntry {
	out := make(map[string]HandleEntry, len(m.Entries))
	for _, e := range m.Entries {
		out[e.Handle] = e
	}
	return out
}

func (m FactHandleMapV1) Lookup(kind EntityKind, id, path string) (HandleEntry, bool) {
	for _, e := range m.Entries {
		if e.Entity.Kind == kind && e.Entity.ID == id && e.SemanticPath == path {
			return e, true
		}
	}
	return HandleEntry{}, false
}

func formatHandle(i int) string { return fmt.Sprintf("f_%04d", i) }

func requiredSideUsability(meta factualapi.FactMeta) (Usability, bool) {
	switch meta.Readiness {
	case "READY":
		if meta.FactRef == "" {
			return "", false
		}
		return UsabilityReady, true
	case "PARTIAL":
		if meta.FactRef == "" || !allowedGroundingPartial(meta) {
			return "", false
		}
		return UsabilityPartial, true
	case "STALE":
		if meta.FactRef == "" {
			return "", false
		}
		return UsabilityStale, true
	default:
		return "", false
	}
}

func combineUsability(a, b Usability) Usability {
	rank := func(u Usability) int {
		switch u {
		case UsabilityReady:
			return 0
		case UsabilityPartial:
			return 1
		case UsabilityStale:
			return 2
		default:
			return 3
		}
	}
	if rank(b) > rank(a) {
		return b
	}
	return a
}

func comparisonUsability(current, previous factualapi.FactMeta) (Usability, bool) {
	cu, ok := requiredSideUsability(current)
	if !ok {
		return "", false
	}
	pu, ok := requiredSideUsability(previous)
	if !ok {
		return "", false
	}
	return combineUsability(cu, pu), true
}

func singleFactUsability(meta factualapi.FactMeta) (Usability, bool) {
	return requiredSideUsability(meta)
}

func allowedGroundingPartial(meta factualapi.FactMeta) bool {
	for _, l := range meta.Limitations {
		switch l.Code {
		case "EXPECTED_PROVIDER_LAG", "SEARCH_PROVIDER_LAG", "PROVIDER_LAG", "LAG":
			return true
		}
	}
	return false
}

func allowedPartial(u Usability, limitations []factualapi.Limitation) bool {
	if u != UsabilityPartial {
		return u == UsabilityReady
	}
	return allowedGroundingPartial(factualapi.FactMeta{Limitations: limitations})
}

func collectRefs(refs ...string) []string {
	out := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, r := range refs {
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}
