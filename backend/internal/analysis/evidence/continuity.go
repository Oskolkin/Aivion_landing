package evidence

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidencecatalog"
)

type continuityRawRequest struct {
	CaseRef     string
	ProposalRef string
	SupportKey  string
	Spec        continuity.SupportRevalidationSpec
}

func CollectContinuityRequests(ctx continuity.ContinuityContextV1) []continuityRawRequest {
	out := make([]continuityRawRequest, 0)
	for _, pa := range ctx.PreviousActions {
		for _, sp := range pa.RequiredSupports {
			out = append(out, continuityRawRequest{
				CaseRef:     pa.CaseRef,
				ProposalRef: pa.ProposalRef,
				SupportKey:  sp.SupportKey,
				Spec:        sp.Revalidation,
			})
		}
	}
	return out
}

func CountRequiredContinuitySupports(ctx continuity.ContinuityContextV1) int {
	n := 0
	for _, pa := range ctx.PreviousActions {
		n += len(pa.RequiredSupports)
	}
	return n
}

func NormalizeAllRequests(universeHash string, v discovery.ValidatedOutputV1, ctx continuity.ContinuityContextV1) ([]NormalizedEvidenceRequest, int, error) {
	rawDiscovery := CollectRawRequests(v)
	rawContinuity := CollectContinuityRequests(ctx)
	rawN := len(rawDiscovery) + len(rawContinuity)

	byKey := map[string]*NormalizedEvidenceRequest{}
	order := make([]string, 0)
	cat := evidencecatalog.ByKey()

	for _, r := range rawDiscovery {
		def, ok := cat[r.Req.EvidenceType]
		if !ok || !def.EnabledForDiscoveryV1 {
			return nil, rawN, Integrity("unknown or disabled evidence type " + r.Req.EvidenceType)
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
				ConsumerRefs: []EvidenceConsumerRef{},
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
		th := r.ThreadID
		n.ConsumerRefs = append(n.ConsumerRefs, EvidenceConsumerRef{
			Source: SourceDiscoveryRequested, ThreadRef: &th,
		})
	}

	for _, r := range rawContinuity {
		if strings.TrimSpace(r.Spec.EvidenceType) == "" {
			return nil, rawN, Integrity("continuity support missing revalidation evidence_type")
		}
		def, ok := cat[r.Spec.EvidenceType]
		if !ok {
			return nil, rawN, Integrity("unknown evidence type " + r.Spec.EvidenceType)
		}
		subj := canonicalEntity(r.Spec.Subject)
		var rel *EntityRef
		if r.Spec.RelatedEntity != nil {
			e := canonicalEntity(*r.Spec.RelatedEntity)
			rel = &e
		}
		key := RequestKey(universeHash, r.Spec.EvidenceType, def.Version(), subj, rel)
		n, exists := byKey[key]
		if !exists {
			cp := NormalizedEvidenceRequest{
				RequestKey: key, EvidenceType: r.Spec.EvidenceType, EvidenceVersion: def.Version(),
				Subject: subj, Related: rel, Source: SourceContinuityRequired, ThreadRefs: []ThreadEvidenceRef{},
				ConsumerRefs: []EvidenceConsumerRef{},
			}
			byKey[key] = &cp
			n = byKey[key]
			order = append(order, key)
		}
		caseRef, propRef, sk := r.CaseRef, r.ProposalRef, r.SupportKey
		n.ConsumerRefs = append(n.ConsumerRefs, EvidenceConsumerRef{
			Source:              SourceContinuityRequired,
			PreviousCaseRef:     &caseRef,
			PreviousProposalRef: &propRef,
			SupportKey:          &sk,
		})
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
	return out, rawN, nil
}

func ValidateContinuityCoverage(ctx continuity.ContinuityContextV1, rels []ContinuitySupportEvidenceRelation) error {
	want := CountRequiredContinuitySupports(ctx)
	if want == 0 {
		return nil
	}
	if len(rels) != want {
		return Integrity(fmt.Sprintf("continuity_support_evidence_coverage %d != %d", len(rels), want))
	}
	seen := map[string]bool{}
	for _, pa := range ctx.PreviousActions {
		for _, sp := range pa.RequiredSupports {
			k := pa.CaseRef + "|" + sp.SupportKey
			found := false
			for _, r := range rels {
				if r.PreviousCaseRef == pa.CaseRef && r.SupportKey == sp.SupportKey {
					if r.EvidenceID == "" {
						return Integrity("continuity support evidence_id missing")
					}
					found = true
					break
				}
			}
			if !found {
				return Integrity("continuity support evidence missing for " + k)
			}
			if seen[k] {
				return Integrity("continuity support duplicate " + k)
			}
			seen[k] = true
		}
	}
	return nil
}

func BuildContinuityRelations(ctx continuity.ContinuityContextV1, norm []NormalizedEvidenceRequest, items []EvidenceItem) ([]ContinuitySupportEvidenceRelation, error) {
	byID := map[string]EvidenceItem{}
	for _, it := range items {
		byID[it.EvidenceID] = it
	}
	rels := make([]ContinuitySupportEvidenceRelation, 0, CountRequiredContinuitySupports(ctx))
	for _, pa := range ctx.PreviousActions {
		for _, sp := range pa.RequiredSupports {
			evID, err := continuityEvidenceID(norm, pa.CaseRef, pa.ProposalRef, sp.SupportKey)
			if err != nil {
				return nil, err
			}
			if _, ok := byID[evID]; !ok {
				return nil, Integrity("continuity evidence item missing " + evID)
			}
			rels = append(rels, ContinuitySupportEvidenceRelation{
				PreviousCaseRef:     pa.CaseRef,
				PreviousProposalRef: pa.ProposalRef,
				SupportKey:          sp.SupportKey,
				EvidenceID:          evID,
			})
		}
	}
	return rels, nil
}

func continuityEvidenceID(norm []NormalizedEvidenceRequest, caseRef, proposalRef, supportKey string) (string, error) {
	for _, n := range norm {
		for _, c := range n.ConsumerRefs {
			if c.Source != SourceContinuityRequired {
				continue
			}
			if c.PreviousCaseRef != nil && *c.PreviousCaseRef == caseRef &&
				c.PreviousProposalRef != nil && *c.PreviousProposalRef == proposalRef &&
				c.SupportKey != nil && *c.SupportKey == supportKey {
				return EvidenceID(n.Ordinal), nil
			}
		}
	}
	return "", Integrity("continuity consumer not found for " + caseRef + "/" + supportKey)
}
