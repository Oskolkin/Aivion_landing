package discovery

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
)

type PackEntityRecord struct {
	Kind            EntityKind
	AIID            string
	CanonicalRef    string
	SellerScoped    bool
	Leaf            bool
	Unmapped        bool
	Present         bool
	MayBePrimary    bool
	MayBeSupporting bool
}

type PackEntityIndex struct {
	ByKey map[string]PackEntityRecord
}

func entityKey(kind EntityKind, id string) string { return string(kind) + ":" + id }

func (idx PackEntityIndex) Lookup(kind EntityKind, id string) (PackEntityRecord, bool) {
	if idx.ByKey == nil {
		return PackEntityRecord{}, false
	}
	r, ok := idx.ByKey[entityKey(kind, id)]
	return r, ok
}

func BuildEntityIndex(detail landscape.LandscapeSet, sel materiality.SelectionArtifact) (PackEntityIndex, error) {
	idx := PackEntityIndex{ByKey: map[string]PackEntityRecord{}}
	selectedCats := selectedIDs(sel.Categories)
	selectedSKUs := selectedIDs(sel.SKUs)
	selectedCamps := selectedIDs(sel.Campaigns)

	idx.ByKey[entityKey(KindAccount, "account")] = PackEntityRecord{
		Kind: KindAccount, AIID: "account", CanonicalRef: "account",
		SellerScoped: true, Present: true, MayBePrimary: true, MayBeSupporting: true,
	}

	for _, c := range detail.Categories {
		id := strconv.FormatInt(c.Identity.CategoryID, 10)
		rec := PackEntityRecord{
			Kind: KindCategory, AIID: id,
			CanonicalRef: fmt.Sprintf("category:%d", c.Identity.CategoryID),
			SellerScoped: true, Leaf: c.Identity.IsLeaf, Unmapped: c.Identity.Unmapped,
			Present: true, MayBeSupporting: true,
		}
		if selectedCats[c.Identity.CategoryID] && c.Identity.IsLeaf && !c.Identity.Unmapped {
			rec.MayBePrimary = true
		}
		idx.ByKey[entityKey(KindCategory, id)] = rec
	}
	for _, s := range detail.SKUs {
		id := strconv.FormatInt(s.Identity.CanonicalSKUID, 10)
		idx.ByKey[entityKey(KindSKU, id)] = PackEntityRecord{
			Kind: KindSKU, AIID: id, CanonicalRef: fmt.Sprintf("sku:%d", s.Identity.CanonicalSKUID),
			SellerScoped: true, Present: true,
			MayBePrimary:    selectedSKUs[s.Identity.CanonicalSKUID],
			MayBeSupporting: true,
		}
	}
	for _, c := range detail.Campaigns {
		id := strconv.FormatInt(c.Identity.CampaignID, 10)
		idx.ByKey[entityKey(KindCampaign, id)] = PackEntityRecord{
			Kind: KindCampaign, AIID: id, CanonicalRef: fmt.Sprintf("campaign:%d", c.Identity.CampaignID),
			SellerScoped: true, Present: true,
			MayBePrimary:    selectedCamps[c.Identity.CampaignID],
			MayBeSupporting: true,
		}
	}
	return idx, nil
}

func selectedIDs(ents []materiality.SelectedEntity) map[int64]bool {
	out := map[int64]bool{}
	for _, e := range ents {
		if e.CanonicalRefID == "" {
			continue
		}
		n, err := strconv.ParseInt(e.CanonicalRefID, 10, 64)
		if err == nil {
			out[n] = true
		}
	}
	return out
}

func SortedIndexKeys(idx PackEntityIndex) []string {
	keys := make([]string, 0, len(idx.ByKey))
	for k := range idx.ByKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func IndexFromPayload(p PayloadV1) PackEntityIndex {
	idx := PackEntityIndex{ByKey: map[string]PackEntityRecord{}}
	put := func(e PackEntity, primary bool) {
		idx.ByKey[entityKey(e.Kind, e.ID)] = PackEntityRecord{
			Kind: e.Kind, AIID: e.ID, CanonicalRef: string(e.Kind) + ":" + e.ID,
			SellerScoped: true, Present: true, MayBePrimary: primary, MayBeSupporting: true,
			Leaf: e.Kind != KindCategory || true,
		}
	}
	put(p.Account.Entity, true)
	for _, c := range p.Categories {
		rec := PackEntityRecord{Kind: KindCategory, AIID: c.Entity.ID, CanonicalRef: "category:" + c.Entity.ID,
			SellerScoped: true, Present: true, MayBePrimary: true, MayBeSupporting: true, Leaf: true}
		idx.ByKey[entityKey(KindCategory, c.Entity.ID)] = rec
	}
	for _, s := range p.SKUs {
		put(s.Entity, true)
	}
	for _, c := range p.Campaigns {
		put(c.Entity, true)
	}
	return idx
}
