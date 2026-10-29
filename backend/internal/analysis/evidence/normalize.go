package evidence

import (
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidencecatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

type rawRequest struct {
	ThreadID string
	Ordinal  int
	Req      discovery.EvidenceRequest
}

func CollectRawRequests(v discovery.ValidatedOutputV1) []rawRequest {
	out := make([]rawRequest, 0)
	for _, t := range v.Threads {
		for i, er := range t.EvidenceRequests {
			out = append(out, rawRequest{ThreadID: t.ThreadID, Ordinal: i, Req: er})
		}
	}
	return out
}

func NormalizeRequests(universeHash string, v discovery.ValidatedOutputV1) ([]NormalizedEvidenceRequest, error) {
	raw := CollectRawRequests(v)
	byKey := map[string]*NormalizedEvidenceRequest{}
	order := make([]string, 0)
	cat := evidencecatalog.ByKey()
	for _, r := range raw {
		def, ok := cat[r.Req.EvidenceType]
		if !ok || !def.EnabledForDiscoveryV1 {
			return nil, Integrity("unknown or disabled evidence type " + r.Req.EvidenceType)
		}
		subj := canonicalEntity(r.Req.Subject)
		var rel *EntityRef
		if r.Req.RelatedEntity != nil {
			e := canonicalEntity(*r.Req.RelatedEntity)
			rel = &e
		}
		key := RequestKey(universeHash, r.Req.EvidenceType, def.Version(), subj, rel)
		n, exists := byKey[key]
		if !exists {
			cp := NormalizedEvidenceRequest{
				RequestKey: key, EvidenceType: r.Req.EvidenceType, EvidenceVersion: def.Version(),
				Subject: subj, Related: rel, Source: SourceDiscoveryRequested, ThreadRefs: []ThreadEvidenceRef{},
			}
			byKey[key] = &cp
			n = byKey[key]
			order = append(order, key)
		}
		q := strings.TrimSpace(r.Req.InvestigationQuestion)
		merged := false
		for i := range n.ThreadRefs {
			if n.ThreadRefs[i].ThreadID == r.ThreadID {
				n.ThreadRefs[i].InvestigationQuestions = appendQuestion(n.ThreadRefs[i].InvestigationQuestions, q)
				merged = true
				break
			}
		}
		if !merged {
			n.ThreadRefs = append(n.ThreadRefs, ThreadEvidenceRef{
				ThreadID: r.ThreadID, InvestigationQuestions: appendQuestion(nil, q),
			})
		}
	}
	typeOrder := map[string]int{}
	for i, t := range evidencecatalog.Catalog() {
		typeOrder[t.Key] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := byKey[order[i]], byKey[order[j]]
		if typeOrder[a.EvidenceType] != typeOrder[b.EvidenceType] {
			return typeOrder[a.EvidenceType] < typeOrder[b.EvidenceType]
		}
		if a.Subject.Kind != b.Subject.Kind {
			return a.Subject.Kind < b.Subject.Kind
		}
		if a.Subject.ID != b.Subject.ID {
			return a.Subject.ID < b.Subject.ID
		}
		ak, aid, bk, bid := "", "", "", ""
		if a.Related != nil {
			ak, aid = string(a.Related.Kind), a.Related.ID
		}
		if b.Related != nil {
			bk, bid = string(b.Related.Kind), b.Related.ID
		}
		if ak != bk {
			return ak < bk
		}
		if aid != bid {
			return aid < bid
		}
		return a.RequestKey < b.RequestKey
	})
	out := make([]NormalizedEvidenceRequest, 0, len(order))
	for i, k := range order {
		n := *byKey[k]
		n.Ordinal = i
		sort.SliceStable(n.ThreadRefs, func(i, j int) bool { return n.ThreadRefs[i].ThreadID < n.ThreadRefs[j].ThreadID })
		out = append(out, n)
	}
	return out, nil
}

func RequestKey(universeHash, evidType, evidVer string, subject EntityRef, related *EntityRef) string {
	rel := any(nil)
	if related != nil {
		rel = canonicalEntity(*related)
	}
	return canonicaljson.Hash(map[string]any{
		"factual_universe_hash": universeHash,
		"evidence_type":         evidType,
		"evidence_type_version": evidVer,
		"subject":               canonicalEntity(subject),
		"related":               rel,
	})
}

func canonicalEntity(e EntityRef) EntityRef {
	return EntityRef{Kind: e.Kind, ID: strings.TrimSpace(e.ID)}
}

func appendQuestion(in []string, q string) []string {
	if q == "" {
		return in
	}
	for _, x := range in {
		if x == q {
			return in
		}
	}
	return append(in, q)
}

func EvidenceID(ordinal int) string {
	return "ev_" + pad3(ordinal+1)
}

func pad3(n int) string {
	s := []byte{'0', '0', '0'}
	for i := 2; n > 0 && i >= 0; i-- {
		s[i] = byte('0' + n%10)
		n /= 10
	}
	return string(s)
}
