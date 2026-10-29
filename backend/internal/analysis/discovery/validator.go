package discovery

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type ValidateInput struct {
	Output  OutputV1
	Index   PackEntityIndex
	Handles FactHandleMapV1
	Menu    []EvidenceMenuItem
	Budget  OutputBudgetProfile
}

func Validate(in ValidateInput) ValidationReportV1 {
	rep := ValidationReportV1{SchemaVersion: ValidationReportSchemaV1, Valid: true, Errors: []string{}}
	b := in.Budget
	if b.Version == "" {
		b = CurrentOutputBudget()
	}
	out := in.Output
	switch out.DiscoveryState {
	case StateNoMeaningfulThreads:
		if len(out.Threads) != 0 {
			rep.fail("NO_MEANINGFUL_THREADS requires threads=[]")
		}
	case StateThreadsFound:
		if len(out.Threads) < 1 {
			rep.fail("THREADS_FOUND requires at least one thread")
		}
	default:
		rep.fail("invalid discovery_state")
	}
	if len(out.Threads) > b.MaxThreads {
		rep.fail("max_threads exceeded")
	}
	menuKeys := map[string]EvidenceMenuItem{}
	for _, m := range in.Menu {
		menuKeys[m.Key] = m
	}
	handles := in.Handles.ByHandle()
	type evidKey struct{ typ, sk, sid, rk, rid string }
	totalEvid := 0
	for i, t := range out.Threads {
		prefix := fmt.Sprintf("thread[%d]", i)
		if err := validatePrimary(in.Index, t.PrimaryEntity); err != "" {
			rep.fail(prefix + " primary: " + err)
		}
		if len(t.RelatedEntities) > b.MaxRelatedEntitiesPerThread {
			rep.fail(prefix + " related entity budget")
		}
		for _, rel := range t.RelatedEntities {
			if err := validatePresent(in.Index, rel); err != "" {
				rep.fail(prefix + " related: " + err)
			}
		}
		if utf8.RuneCountInString(t.Observation) > b.MaxObservationChars {
			rep.fail(prefix + " observation length")
		}
		if utf8.RuneCountInString(t.InvestigationFocus) > b.MaxInvestigationFocusChars {
			rep.fail(prefix + " investigation_focus length")
		}
		if len(t.BasisFactHandles) < 1 {
			rep.fail(prefix + " requires basis_fact_handles")
		}
		if len(t.BasisFactHandles) > b.MaxBasisFactsPerThread {
			rep.fail(prefix + " basis fact budget")
		}
		usable := false
		for _, h := range t.BasisFactHandles {
			e, ok := handles[h]
			if !ok {
				rep.fail(prefix + " unknown handle " + h)
				continue
			}
			if e.AnalyticalUsability == UsabilityReady || (e.AnalyticalUsability == UsabilityPartial && allowedHandlePartial(e)) {
				usable = true
			}
		}
		if len(t.BasisFactHandles) > 0 && !usable {
			rep.fail(prefix + " STALE-only grounding is invalid")
		}
		if len(t.EvidenceRequests) > b.MaxEvidenceRequestsPerThread {
			rep.fail(prefix + " evidence per thread budget")
		}
		seenEvid := map[evidKey]bool{}
		for _, er := range t.EvidenceRequests {
			totalEvid++
			if utf8.RuneCountInString(er.InvestigationQuestion) > b.MaxInvestigationQuestionChars {
				rep.fail(prefix + " investigation_question length")
			}
			item, ok := menuKeys[er.EvidenceType]
			if !ok {
				rep.fail(prefix + " evidence type not in menu: " + er.EvidenceType)
			} else if !kindIn(item.SubjectKinds, er.Subject.Kind) {
				rep.fail(prefix + " subject kind not allowed for " + er.EvidenceType)
			}
			if err := validatePresent(in.Index, er.Subject); err != "" {
				rep.fail(prefix + " evidence subject: " + err)
			}
			if er.RelatedEntity != nil {
				if !ok {
					rep.fail(prefix + " unknown evidence type")
				} else if len(item.RelatedKinds) == 0 {
					rep.fail(prefix + " related_entity is not supported for evidence type")
				} else if !kindIn(item.RelatedKinds, er.RelatedEntity.Kind) {
					rep.fail(prefix + " unsupported related entity kind")
				}
				if err := validatePresent(in.Index, *er.RelatedEntity); err != "" {
					rep.fail(prefix + " evidence related: " + err)
				}
			}
			k := evidKey{er.EvidenceType, string(er.Subject.Kind), er.Subject.ID, "", ""}
			if er.RelatedEntity != nil {
				k.rk, k.rid = string(er.RelatedEntity.Kind), er.RelatedEntity.ID
			}
			if seenEvid[k] {
				rep.fail("duplicate evidence request")
			}
			seenEvid[k] = true
		}
	}
	if totalEvid > b.MaxTotalEvidenceRequests {
		rep.fail("max_total_evidence_requests exceeded")
	}
	rep.ThreadCount = len(out.Threads)
	rep.EvidenceRequests = totalEvid
	return rep
}

func (r *ValidationReportV1) fail(msg string) {
	r.Valid = false
	r.Errors = append(r.Errors, msg)
}

func validatePrimary(idx PackEntityIndex, e PackEntity) string {
	rec, ok := idx.Lookup(e.Kind, e.ID)
	if !ok || !rec.Present {
		return "entity not in pack"
	}
	if !rec.MayBePrimary {
		return "entity may not be primary"
	}
	if e.Kind == KindCategory && (!rec.Leaf || rec.Unmapped) {
		return "category must be leaf and not UNMAPPED"
	}
	switch e.Kind {
	case KindAccount, KindCategory, KindSKU, KindCampaign:
	default:
		return "forbidden entity kind"
	}
	return ""
}

func validatePresent(idx PackEntityIndex, e PackEntity) string {
	rec, ok := idx.Lookup(e.Kind, e.ID)
	if !ok || !rec.Present {
		return "entity not in pack"
	}
	return ""
}

func kindIn(ks []EntityKind, k EntityKind) bool {
	for _, x := range ks {
		if x == k {
			return true
		}
	}
	return false
}

func allowedHandlePartial(e HandleEntry) bool {
	if e.AnalyticalUsability != UsabilityPartial {
		return false
	}
	p := e.SemanticPath
	return strings.HasPrefix(p, "search.") || strings.HasPrefix(p, "funnel.")
}

func ValidateAuthoritative(in ValidateInput) (ValidatedOutputV1, ValidationReportV1) {
	rep := Validate(in)
	if !rep.Valid {
		return ValidatedOutputV1{}, rep
	}
	return AssignThreadIDs(in.Output), rep
}
