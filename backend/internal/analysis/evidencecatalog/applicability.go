package evidencecatalog

import (
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

type MenuItem struct {
	Key          string       `json:"key"`
	SubjectKinds []EntityKind `json:"subject_kinds"`
	RelatedKinds []EntityKind `json:"related_kinds"`
	Description  string       `json:"description"`
}

func BuildMenu(caps map[string]string, kinds map[EntityKind]bool, universeDomains map[string]bool, menuPolicy string) []MenuItem {
	items := make([]MenuItem, 0)
	for _, t := range Catalog() {
		if !t.EnabledForDiscoveryV1 {
			continue
		}
		if !kindApplicable(t, kinds) {
			continue
		}
		if !capabilityOK(t, caps) {
			continue
		}
		if !domainOK(t, universeDomains) {
			continue
		}
		related := t.AllowedRelatedKinds
		if related == nil {
			related = []EntityKind{}
		}
		items = append(items, MenuItem{
			Key: t.Key, SubjectKinds: append([]EntityKind(nil), t.AllowedSubjectKinds...),
			RelatedKinds: append([]EntityKind(nil), related...), Description: t.Description,
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return items
}

func MenuHash(items []MenuItem, menuPolicy string) string {
	return canonicaljson.Hash(map[string]any{
		"policy":  menuPolicy,
		"catalog": CatalogV1,
		"items":   items,
	})
}

func kindApplicable(t Type, kinds map[EntityKind]bool) bool {
	for _, k := range t.AllowedSubjectKinds {
		if kinds[k] {
			return true
		}
	}
	return false
}

func capabilityOK(t Type, caps map[string]string) bool {
	if len(t.RequiredCapabilities) == 0 {
		return true
	}
	ok := func(key string) bool {
		st := strings.ToUpper(strings.TrimSpace(caps[key]))
		return st == "READY" || st == "PARTIAL" || st == "STALE"
	}
	switch t.CapabilityMatch {
	case MatchANY:
		for _, k := range t.RequiredCapabilities {
			if ok(k) {
				return true
			}
		}
		return false
	default:
		for _, k := range t.RequiredCapabilities {
			if !ok(k) {
				return false
			}
		}
		return true
	}
}

func domainOK(t Type, domains map[string]bool) bool {
	if len(domains) == 0 {
		return true
	}
	d := strings.ToLower(t.Domain)
	aliases := map[string][]string{
		"sales": {"sales"}, "lifecycle": {"sales"}, "returns": {"sales"},
		"stock": {"stock"}, "price": {"price"}, "advertising": {"advertising"},
		"funnel": {"funnel"}, "search": {"search"}, "finance": {"finance"},
		"promotions": {"promotions"}, "account_health": {"account_health", "account-health"},
		"supply": {"supply"}, "catalog": {"catalog"},
	}
	for _, a := range aliases[d] {
		if domains[a] {
			return true
		}
	}
	return domains[d]
}
