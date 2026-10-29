package actionreconcile

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

func Plan(in PlanInput) (PlanV1, ReportV1, error) {
	rep := ReportV1{SchemaVersion: ReportSchemaV1, Errors: []string{}}
	add := func(s string) { rep.Errors = append(rep.Errors, s) }

	if in.Continuity.BasePublicationID != nil && in.BasePublicationID != nil && *in.Continuity.BasePublicationID != *in.BasePublicationID {
		add("BASE_PUBLICATION_MISMATCH")
	}
	if in.SellerID != 0 && in.Continuity.Mode == "" && in.Continuity.ContinuityContractVersion == "" {
		add("CONTINUITY_MISSING")
	}

	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	assessBy := map[string]deep.PreviousActionAssessment{}
	for _, a := range in.Validated.Output.PreviousActionAssessments {
		if _, ok := assessBy[a.PreviousActionRef]; ok {
			add("PREVIOUS_ASSESSMENT_DUP")
		}
		assessBy[a.PreviousActionRef] = a
	}
	for _, pa := range in.Continuity.PreviousActions {
		if _, ok := assessBy[pa.PreviousActionRef]; !ok {
			add("PREVIOUS_ASSESSMENT_MISSING:" + pa.PreviousActionRef)
		}
	}

	sigState := map[string]PersistedSignature{}
	for _, s := range in.Snapshot.Signatures {
		sigState[s.SignatureHash] = s
	}

	intents := make([]NewProposalIntent, 0, len(in.Validated.Output.ActionProposals))
	seenSig := map[string]int{}
	promoJoinLeave := map[string]string{}

	for i, p := range in.Validated.Output.ActionProposals {
		ctx := NormalizedContext(p.ActionType, p.Parameters)
		priceHint := CurrentPriceHint(p)
		if priceHint == nil {
			priceHint = actioncatalog.PriceHintFromMenu(in.Menu, string(p.Target.Kind), p.Target.ID)
		}
		intent, err := IntentSignature(p.ActionType, p.Parameters, priceHint)
		if err != nil {
			add(err.Error())
			intent = "UNSPECIFIED"
		}
		sig := HashSignature(SignatureInput{
			SellerID: in.SellerID, ActionType: p.ActionType, Target: p.Target,
			NormalizedContext: ctx, IntentSignature: intent,
		})
		if prev, ok := seenSig[sig]; ok {
			add(fmt.Sprintf("DUPLICATE_SIGNATURE:AP%03d,AP%03d", prev, i+1))
		}
		seenSig[sig] = i + 1

		if p.ActionType == actioncatalog.TypeSKUJoinPromotion || p.ActionType == actioncatalog.TypeSKULeavePromotion {
			key := string(p.Target.Kind) + "|" + p.Target.ID + "|" + ctx["promotion_ref"]
			if other, ok := promoJoinLeave[key]; ok && other != p.ActionType {
				add("CONFLICT_JOIN_LEAVE:" + key)
			}
			promoJoinLeave[key] = p.ActionType
		}

		recRef := ""
		if p.RecommendationOrdinal > 0 && p.RecommendationOrdinal <= len(in.Validated.RecommendationRefs) {
			recRef = in.Validated.RecommendationRefs[p.RecommendationOrdinal-1]
		} else {
			recRef = fmt.Sprintf("R%03d", p.RecommendationOrdinal)
		}
		findRefs := make([]string, 0, len(p.FindingOrdinals))
		for _, fo := range p.FindingOrdinals {
			if fo > 0 && fo <= len(in.Validated.FindingRefs) {
				findRefs = append(findRefs, in.Validated.FindingRefs[fo-1])
			}
		}

		st := sigState[sig]
		disp := "VISIBLE_NEW"
		visible := true
		openNew := true
		exist := ""
		if st.SuppressedUntil != nil && st.SuppressedUntil.After(now) {
			disp = "SUPPRESSED_USER_DISMISSED"
			visible = false
			openNew = st.ActiveUserState == "DONE" || st.ActiveCaseRef == ""
			exist = st.ActiveCaseRef
			if st.ActiveUserState != "DONE" && st.ActiveCaseRef != "" {
				openNew = false
			}
		} else if st.ActiveCaseRef != "" && st.ActiveUserState == "DONE" {
			disp = "VISIBLE_NEW"
			openNew = true
		} else if st.ActiveCaseRef != "" && st.ActiveSystem == "ACTIVE" && st.ActiveUserState == "OPEN" {
			disp = "REAFFIRMED_EXISTING"
			openNew = false
			exist = st.ActiveCaseRef
		}

		intents = append(intents, NewProposalIntent{
			LocalRef: fmt.Sprintf("NP%03d", i+1), DeepOrdinal: p.Ordinal, ActionType: p.ActionType,
			Target: p.Target, NormalizedContext: ctx, IntentSignature: intent, SignatureHash: sig,
			Parameters: p.Parameters, RecommendationRef: recRef, FindingRefs: findRefs, FactHandles: p.FactHandles,
			Supports: p.Supports, Disposition: disp, ExistingCaseRef: exist, OpenNewOccurrence: openNew,
			Visible: visible, DisplayOrder: i + 1,
		})
	}

	if len(intents) != len(in.Validated.Output.ActionProposals) {
		add("DISPOSITION_COUNT_MISMATCH")
	}

	prevOut := make([]PreviousOutcome, 0, len(in.Continuity.PreviousActions))
	for _, pa := range in.Continuity.PreviousActions {
		ass := assessBy[pa.PreviousActionRef]
		sys := ""
		ev := ""
		openNew := false
		link := ""
		switch ass.Assessment {
		case "FULLY_SUPPORTED":
			if pa.UserState == "DONE" {
				sys = "KEEP_DONE_OPEN_NEW"
				openNew = true
				ev = "RECURRED"
			} else {
				sys = "REAFFIRM"
				ev = "REAFFIRMED"
			}
			for _, np := range intents {
				if np.SignatureHash == pa.SignatureHash {
					link = np.LocalRef
					break
				}
			}
		case "PARTIALLY_SUPPORTED":
			sys = "BASIS_CHANGED"
			ev = "BASIS_CHANGED"
		case "NOT_SUPPORTED":
			sys = "RESOLVED"
			ev = "RESOLVED"
		case "UNVERIFIABLE":
			sys = "SUSPENDED"
			ev = "SUSPENDED"
		case "SUPERSEDED":
			sys = "SUPERSEDED"
			ev = "SUPERSEDED"
		default:
			add("UNKNOWN_ASSESSMENT:" + ass.Assessment)
		}
		if len(ass.SupportAssessments) != len(pa.RequiredSupports) {
			add("PREVIOUS_SUPPORT_COVERAGE:" + pa.PreviousActionRef)
		}
		prevOut = append(prevOut, PreviousOutcome{
			PreviousActionRef: pa.PreviousActionRef, CaseRef: pa.CaseRef, ProposalRef: pa.ProposalRef,
			SignatureHash: pa.SignatureHash, DeepAssessment: ass.Assessment, SystemTransition: sys,
			OpenNewOccurrence: openNew, LinkedNewLocalRef: link, SupportAssessments: ass.SupportAssessments,
			SystemEvent: ev,
		})
	}

	groups := Group(intents, in.Catalog)
	plan := PlanV1{
		SchemaVersion: PlanSchemaV1, IdentityPolicyVersion: IdentityPolicyV1,
		ReconciliationPolicy: ReconciliationPolicyV1, VisibilityPolicy: VisibilityPolicyV1,
		UserSuppressionPolicy: UserSuppressionPolicyV1, RecurrencePolicy: RecurrencePolicyV1 + ":" + RecurrenceDisabledUntilCalibrated,
		GroupingPolicy: GroupingPolicyV1, SellerAccountID: in.SellerID, AnalysisRunID: in.RunID,
		BasePublicationID: in.BasePublicationID, ValidatedDeepHash: in.ValidatedHash,
		ContinuityContextHash: in.ContinuityHash, NewProposalIntents: intents, PreviousOutcomes: prevOut, Groups: groups,
	}
	rep.Valid = len(rep.Errors) == 0
	sort.Strings(rep.Errors)
	if !rep.Valid {
		return plan, rep, fmt.Errorf("actionreconcile: %s", strings.Join(rep.Errors, ","))
	}
	return plan, rep, nil
}

func PlanHash(p PlanV1) string { return canonicaljson.Hash(p) }
