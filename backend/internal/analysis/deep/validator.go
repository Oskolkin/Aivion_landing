package deep

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidencecatalog"
)

type ValidateInput struct {
	Output   OutputV1
	Payload  PayloadV1
	Snapshot actioncatalog.Snapshot
	Budget   OutputBudgetProfile
}

func Validate(in ValidateInput) ValidationReportV1 {
	var errs []string
	b := in.Budget
	if b.Version == "" {
		b = CurrentOutputBudget()
	}
	p := in.Payload
	out := in.Output
	snap := enrichContinuitySnapshot(p)
	handleOK := map[string]bool{}
	for _, h := range p.FactRegistry {
		handleOK[h.Handle] = true
	}
	entityOK := map[string]EntityContext{}
	mainOK := map[string]bool{}
	for _, e := range p.ThreadEntityContext {
		k := string(e.Entity.Kind) + "|" + e.Entity.ID
		entityOK[k] = e
		if e.Role == RoleMain {
			mainOK[k] = true
		}
	}
	acc := p.AccountContext.Entity
	if acc.ID != "" {
		mainOK[string(acc.Kind)+"|"+acc.ID] = true
	}

	add := func(e string) { errs = append(errs, e) }
	needLen := func(s, name string, n int) {
		if utf8.RuneCountInString(s) > n {
			add(name + "_TOO_LONG")
		}
	}
	needHandles := func(hs []string, name string) {
		if len(hs) > b.MaxFactHandlesPerObject {
			add(name + "_HANDLE_BUDGET")
		}
		for _, h := range hs {
			if !handleOK[h] {
				add(name + "_UNKNOWN_HANDLE:" + h)
			}
		}
	}

	needLen(out.ExecutiveSummary.Headline, "HEADLINE", b.HeadlineChars)
	needLen(out.ExecutiveSummary.Summary, "SUMMARY", b.ExecutiveSummaryChars)
	if len(out.ExecutiveSummary.KeyTakeaways) > b.MaxKeyTakeaways {
		add("TAKEAWAY_BUDGET")
	}
	needHandles(out.ExecutiveSummary.BasisFactHandles, "SUMMARY")
	for _, t := range out.ExecutiveSummary.KeyTakeaways {
		needLen(t.Statement, "TAKEAWAY", b.TakeawayChars)
		if !directionOK(t.Direction) {
			add("TAKEAWAY_DIRECTION")
		}
		if !PrimaryDomainOK(t.PrimaryDomain) {
			add("TAKEAWAY_PRIMARY_DOMAIN")
		}
		needHandles(t.SupportingFactHandles, "TAKEAWAY")
	}

	threadSet := map[string]bool{}
	for _, th := range p.DiscoveryResult.Threads {
		threadSet[th.ThreadRef] = true
	}
	noThreads := p.DiscoveryResult.DiscoveryState == string(discovery.StateNoMeaningfulThreads)
	if noThreads {
		if len(out.ThreadResolutions) != 0 {
			add("NO_MEANINGFUL_THREADS_RESOLUTIONS")
		}
		if len(out.ActionProposals) != 0 {
			add("NO_MEANINGFUL_THREADS_ACTIONS")
		}
	} else {
		if len(out.ThreadResolutions) != len(p.DiscoveryResult.Threads) {
			add("THREAD_COVERAGE")
		}
		seen := map[string]bool{}
		for _, r := range out.ThreadResolutions {
			if !threadSet[r.DiscoveryThreadRef] {
				add("INVALID_THREAD_REF:" + r.DiscoveryThreadRef)
			}
			if seen[r.DiscoveryThreadRef] {
				add("DUPLICATE_THREAD:" + r.DiscoveryThreadRef)
			}
			seen[r.DiscoveryThreadRef] = true
			if !resolutionOK(r.Resolution) {
				add("RESOLUTION_INVALID")
			}
			needLen(r.ResolutionSummary, "THREAD_SUMMARY", b.ThreadResolutionChars)
			needHandles(r.SupportingFactHandles, "THREAD")
			needHandles(r.CounterFactHandles, "THREAD")
			for _, lim := range r.Limitations {
				needLen(lim, "LIMITATION", b.LimitationChars)
			}
		}
		for ref := range threadSet {
			if !seen[ref] {
				add("MISSING_THREAD:" + ref)
			}
		}
	}

	if len(out.Findings) > b.MaxFindings {
		add("FINDING_BUDGET")
	}
	findBy := map[int]Finding{}
	for i, f := range out.Findings {
		if f.Ordinal != i+1 {
			add("FINDING_ORDINAL")
		}
		if _, dup := findBy[f.Ordinal]; dup {
			add("FINDING_ORDINAL_DUP")
		}
		findBy[f.Ordinal] = f
		if !directionOK(f.BusinessDirection) {
			add("FINDING_DIRECTION")
		}
		if !PrimaryDomainOK(f.PrimaryDomain) {
			add("FINDING_PRIMARY_DOMAIN")
		}
		if !OverviewBucketOK(f.OverviewBucket) {
			add("FINDING_OVERVIEW_BUCKET")
		}
		needLen(f.Title, "FINDING_TITLE", b.FindingTitleChars)
		needLen(f.FindingStatement, "FINDING_STATEMENT", b.FindingStatementChars)
		if len(f.RelatedEntities) > b.MaxRelatedEntitiesPerFinding {
			add("RELATED_ENTITY_BUDGET")
		}
		if len(f.SupportingFactHandles) < 1 {
			add("FINDING_UNGROUNDED")
		}
		needHandles(f.SupportingFactHandles, "FINDING")
		needHandles(f.CounterFactHandles, "FINDING")
		pk := string(f.PrimaryEntity.Kind) + "|" + f.PrimaryEntity.ID
		if !mainOK[pk] {
			add("FINDING_PRIMARY_NOT_MAIN")
		}
		if !primaryKindOK(f.PrimaryEntity.Kind) {
			add("FINDING_PRIMARY_KIND")
		}
		switch f.OriginType {
		case "THREAD_DERIVED":
			if len(f.SourceThreadRefs) < 1 {
				add("THREAD_DERIVED_NEEDS_THREAD")
			}
		case "CROSS_THREAD_SYNTHESIS":
			if len(f.SourceThreadRefs) < 2 {
				add("CROSS_THREAD_NEEDS_TWO")
			}
		case "EVIDENCE_EMERGENT":
			if !hasEF(f.SupportingFactHandles) {
				add("EVIDENCE_EMERGENT_NEEDS_EF")
			}
		case "ACCOUNT_CONTEXT":
			if f.PrimaryEntity.Kind != discovery.KindAccount {
				add("ACCOUNT_CONTEXT_PRIMARY")
			}
		default:
			add("ORIGIN_INVALID")
		}
		if noThreads && f.OriginType != "ACCOUNT_CONTEXT" {
			add("NO_MEANINGFUL_FINDING_ORIGIN")
		}
		for _, ref := range f.SourceThreadRefs {
			if !threadSet[ref] {
				add("FINDING_UNKNOWN_THREAD")
			}
		}
		if len(f.RelationshipAssessments) > b.MaxRelationshipsPerFinding {
			add("RELATIONSHIP_BUDGET")
		}
		items := itemViews(p)
		facts := factsByHandle(p)
		for _, ra := range f.RelationshipAssessments {
			if ra.RelationshipBasis == BasisCausalEstimate {
				add(CodeCausalEstimate)
			}
			needLen(ra.RelationshipStatement, "RELATIONSHIP", b.RelationshipStatementChars)
			needLen(ra.DriverOrFactor, "DRIVER", b.DriverOutcomeChars)
			needLen(ra.Outcome, "OUTCOME", b.DriverOutcomeChars)
			if len(ra.AlternativeExplanations) > b.MaxAlternativeExplanations {
				add("ALTERNATIVE_BUDGET")
			}
			needHandles(ra.SupportingFactHandles, "REL")
			needHandles(ra.CounterFactHandles, "REL")
			var support []FactProjection
			for _, h := range ra.SupportingFactHandles {
				if fp, ok := facts[h]; ok {
					support = append(support, fp)
				}
			}
			if err := AllowRelationship(ra.RelationshipBasis, support, items, p.RelationshipClaimPolicy); err != nil {
				add(err.Error())
			}
		}
	}

	if len(out.Recommendations) > b.MaxRecommendations {
		add("RECOMMENDATION_BUDGET")
	}
	recBy := map[int]Recommendation{}
	for i, r := range out.Recommendations {
		if r.Ordinal != i+1 {
			add("RECOMMENDATION_ORDINAL")
		}
		recBy[r.Ordinal] = r
		needLen(r.RecommendationStatement, "RECOMMENDATION", b.RecommendationChars)
		needLen(r.Rationale, "RATIONALE", b.RationaleChars)
		if len(r.SupportFindingOrdinals) < 1 {
			add("RECOMMENDATION_UNGROUNDED")
		}
		for _, fo := range r.SupportFindingOrdinals {
			if _, ok := findBy[fo]; !ok {
				add("RECOMMENDATION_UNKNOWN_FINDING")
			}
		}
		needHandles(r.SupportFactHandles, "RECOMMENDATION")
		if r.Actionability != "ACTIONABLE_NOW" && r.Actionability != "MONITOR" && r.Actionability != "INSUFFICIENT_EVIDENCE" {
			add("ACTIONABILITY_INVALID")
		}
		if noThreads && r.Actionability == "ACTIONABLE_NOW" {
			add("NO_MEANINGFUL_ACTIONABLE")
		}
	}

	if len(out.ActionProposals) > b.MaxActionProposals {
		add("ACTION_BUDGET")
	}
	seenFP := map[string]bool{}
	for i, a := range out.ActionProposals {
		if a.Ordinal != i+1 {
			add("ACTION_ORDINAL")
		}
		rec, ok := recBy[a.RecommendationOrdinal]
		if !ok {
			add("ACTION_UNKNOWN_RECOMMENDATION")
		} else if rec.Actionability != "ACTIONABLE_NOW" {
			add("ACTION_NON_ACTIONABLE_RECOMMENDATION")
		}
		if len(a.FindingOrdinals) < 1 {
			add("ACTION_MISSING_FINDING")
		}
		for _, fo := range a.FindingOrdinals {
			if _, ok := findBy[fo]; !ok {
				add("ACTION_UNKNOWN_FINDING")
			}
		}
		if len(a.FactHandles) < 1 {
			add("ACTION_MISSING_FACTS")
		}
		needHandles(a.FactHandles, "ACTION")
		prop := actioncatalog.Proposal{ActionType: a.ActionType, Target: a.Target, Parameters: a.Parameters}
		if err := actioncatalog.ValidateProposal(prop, p.ActionMenu, snap); err != nil {
			if !continuityProposalAllowed(prop, p.Continuity, snap, p) {
				add("ACTION_CONTRACT:" + err.Error())
			}
		}
		fp := actioncatalog.Fingerprint(prop)
		if seenFP[fp] {
			add("ACTION_DUPLICATE")
		}
		seenFP[fp] = true
		required := 0
		seenSK := map[string]bool{}
		for si, sp := range a.Supports {
			if sp.Ordinal != si+1 && sp.Ordinal != 0 {
				add("SUPPORT_ORDINAL")
			}
			switch sp.Role {
			case "REQUIRED_SUPPORT":
				required++
			case "CONTEXT", "COUNTEREVIDENCE":
			default:
				add("SUPPORT_ROLE_INVALID")
			}
			if strings.TrimSpace(sp.Claim) == "" {
				add("SUPPORT_CLAIM_MISSING")
			}
			if len(sp.FindingOrdinals) < 1 {
				add("SUPPORT_MISSING_FINDING")
			}
			for _, fo := range sp.FindingOrdinals {
				if _, ok := findBy[fo]; !ok {
					add("SUPPORT_UNKNOWN_FINDING")
				}
			}
			if len(sp.FactHandles) < 1 {
				add("SUPPORT_MISSING_FACTS")
			}
			needHandles(sp.FactHandles, "SUPPORT")
			if sp.Role == "REQUIRED_SUPPORT" {
				validateRevalidation(sp.Revalidation, p, add)
			}
			if sp.SupportKey != "" {
				if seenSK[sp.SupportKey] {
					add("SUPPORT_KEY_DUP")
				}
				seenSK[sp.SupportKey] = true
			}
		}
		if required < 1 {
			add("ACTION_MISSING_REQUIRED_SUPPORT")
		}
	}

	if len(out.PreviousActionAssessments) != len(p.Continuity.PreviousActions) {
		add("PREVIOUS_ACTION_COVERAGE")
	}
	seenPrev := map[string]bool{}
	wantPrev := map[string]bool{}
	wantPrevSupports := map[string][]string{}
	for _, pa := range p.Continuity.PreviousActions {
		wantPrev[pa.PreviousActionRef] = true
		keys := make([]string, 0, len(pa.RequiredSupports))
		for _, s := range pa.RequiredSupports {
			keys = append(keys, s.SupportKey)
		}
		wantPrevSupports[pa.PreviousActionRef] = keys
	}
	for _, a := range out.PreviousActionAssessments {
		needLen(a.AssessmentSummary, "PREVIOUS_ASSESSMENT", b.PreviousAssessmentChars)
		needHandles(a.SupportingFactHandles, "PREVIOUS")
		if !assessOK(a.Assessment) {
			add("PREVIOUS_ASSESSMENT_INVALID")
		}
		if seenPrev[a.PreviousActionRef] {
			add("PREVIOUS_ACTION_DUP")
		}
		seenPrev[a.PreviousActionRef] = true
		if len(wantPrev) > 0 && !wantPrev[a.PreviousActionRef] {
			add("PREVIOUS_ACTION_UNKNOWN")
		}
		wantKeys := wantPrevSupports[a.PreviousActionRef]
		if len(a.SupportAssessments) != len(wantKeys) {
			add("PREVIOUS_SUPPORT_COVERAGE")
		}
		seenSA := map[string]bool{}
		for _, sa := range a.SupportAssessments {
			switch sa.Status {
			case "HOLDS", "CHANGED", "NOT_VERIFIABLE":
			default:
				add("PREVIOUS_SUPPORT_STATUS_INVALID")
			}
			if sa.SupportKey != "" {
				if seenSA[sa.SupportKey] {
					add("PREVIOUS_SUPPORT_DUP")
				}
				seenSA[sa.SupportKey] = true
			}
			if sa.Status != "NOT_VERIFIABLE" {
				needHandles(sa.CurrentFactHandles, "PREVIOUS_SUPPORT")
			}
		}
		for _, k := range wantKeys {
			if k != "" && !seenSA[k] {
				add("PREVIOUS_SUPPORT_MISSING")
			}
		}
	}

	blob, _ := jsonBytes(out)
	if containsPII(string(blob)) {
		add("PII_LEAK")
	}

	return ValidationReportV1{SchemaVersion: ValidationReportSchemaV1, Valid: len(errs) == 0, Errors: errs}
}

func AssignBackendRefs(out OutputV1) ValidatedOutputV1 {
	v := ValidatedOutputV1{SchemaVersion: ValidatedOutputSchemaV1, Output: out}
	for i := range out.Findings {
		v.FindingRefs = append(v.FindingRefs, fmt.Sprintf("F%03d", i+1))
	}
	for i := range out.Recommendations {
		v.RecommendationRefs = append(v.RecommendationRefs, fmt.Sprintf("R%03d", i+1))
	}
	for i := range out.ActionProposals {
		apRef := fmt.Sprintf("AP%03d", i+1)
		v.ActionProposalRefs = append(v.ActionProposalRefs, apRef)
		for j := range out.ActionProposals[i].Supports {
			key := fmt.Sprintf("B%03d", j+1)
			out.ActionProposals[i].Supports[j].SupportKey = key
			out.ActionProposals[i].Supports[j].Ordinal = j + 1
			v.SupportRefs = append(v.SupportRefs, apRef+"."+key)
		}
		if out.ActionProposals[i].Supports == nil {
			out.ActionProposals[i].Supports = []ActionSupportBasis{}
		}
	}
	v.Output = out
	if v.FindingRefs == nil {
		v.FindingRefs = []string{}
	}
	if v.RecommendationRefs == nil {
		v.RecommendationRefs = []string{}
	}
	if v.ActionProposalRefs == nil {
		v.ActionProposalRefs = []string{}
	}
	if v.SupportRefs == nil {
		v.SupportRefs = []string{}
	}
	return v
}

func directionOK(s string) bool {
	switch s {
	case "POSITIVE", "NEGATIVE", "MIXED", "NEUTRAL", "UNCERTAIN":
		return true
	}
	return false
}

func resolutionOK(s string) bool {
	switch s {
	case "SUBSTANTIATED", "REFRAMED", "NOT_SUBSTANTIATED", "INCONCLUSIVE":
		return true
	}
	return false
}

func assessOK(s string) bool {
	switch s {
	case "FULLY_SUPPORTED", "PARTIALLY_SUPPORTED", "NOT_SUPPORTED", "UNVERIFIABLE", "SUPERSEDED":
		return true
	}
	return false
}

func validateRevalidation(spec SupportRevalidationSpec, p PayloadV1, add func(string)) {
	if strings.TrimSpace(spec.EvidenceType) == "" {
		add("SUPPORT_REVALIDATION_EVIDENCE_TYPE")
		return
	}
	t, ok := evidencecatalog.ByKey()[spec.EvidenceType]
	if !ok {
		add("SUPPORT_REVALIDATION_UNKNOWN_TYPE")
		return
	}
	if spec.Subject.ID == "" || spec.Subject.Kind == "" {
		add("SUPPORT_REVALIDATION_SUBJECT")
		return
	}
	if !entityInPack(spec.Subject, p) {
		add("SUPPORT_REVALIDATION_SUBJECT_OUTSIDE_UNIVERSE")
	}
	if spec.RelatedEntity != nil {
		if spec.RelatedEntity.ID == "" || spec.RelatedEntity.Kind == "" {
			add("SUPPORT_REVALIDATION_RELATED")
		} else if !entityInPack(*spec.RelatedEntity, p) {
			add("SUPPORT_REVALIDATION_RELATED_OUTSIDE_UNIVERSE")
		}
	}
	_ = t
}

func entityInPack(e EntityRef, p PayloadV1) bool {
	k := string(e.Kind) + "|" + e.ID
	for _, ec := range p.ThreadEntityContext {
		if string(ec.Entity.Kind)+"|"+ec.Entity.ID == k {
			return true
		}
	}
	if string(p.AccountContext.Entity.Kind)+"|"+p.AccountContext.Entity.ID == k {
		return true
	}
	for _, it := range p.EvidenceRegistry {
		if string(it.Subject.Kind)+"|"+it.Subject.ID == k {
			return true
		}
	}
	for _, pa := range p.Continuity.PreviousActions {
		if string(pa.Target.Kind)+"|"+pa.Target.ID == k {
			return true
		}
	}
	return false
}

func primaryKindOK(k discovery.EntityKind) bool {
	switch k {
	case discovery.KindAccount, discovery.KindCategory, discovery.KindSKU, discovery.KindCampaign:
		return true
	}
	return false
}

func hasEF(hs []string) bool {
	for _, h := range hs {
		if strings.HasPrefix(h, "ef_") {
			return true
		}
	}
	return false
}

func itemViews(p PayloadV1) []evidenceItemView {
	out := make([]evidenceItemView, 0, len(p.EvidenceRegistry))
	for _, it := range p.EvidenceRegistry {
		out = append(out, evidenceItemView{EvidenceID: it.EvidenceID, EvidenceType: it.EvidenceType, TimeBearing: timeBearing(it)})
	}
	return out
}

func factsByHandle(p PayloadV1) map[string]FactProjection {
	m := map[string]FactProjection{}
	for _, h := range p.FactRegistry {
		m[h.Handle] = h
	}
	return m
}

func jsonBytes(v any) ([]byte, error) {
	return json.Marshal(v)
}

func containsPII(s string) bool {
	low := strings.ToLower(s)
	for _, needle := range []string{"buyer_name", "phone", "email", "delivery_address", "authorization", "api_key", "postgres://", "sk-"} {
		if strings.Contains(low, needle) {
			return true
		}
	}
	return false
}

func continuityProposalAllowed(prop actioncatalog.Proposal, cont ContinuityView, baseSnap actioncatalog.Snapshot, payload PayloadV1) bool {
	snap := enrichContinuitySnapshot(payload)
	for k, v := range baseSnap.MainIDs {
		if v {
			snap.MainIDs[k] = true
		}
	}
	for id, st := range baseSnap.SKU {
		cur := snap.SKU[id]
		if st.CurrentPrice != nil {
			cur.HasPrice = true
			if cur.CurrentPrice == nil {
				cur.CurrentPrice = st.CurrentPrice
			}
			snap.SKU[id] = cur
		}
	}
	for _, pa := range cont.PreviousActions {
		if pa.ActionType != prop.ActionType {
			continue
		}
		if pa.Target.ID != prop.Target.ID {
			continue
		}
		if pa.Target.Kind != "" && pa.Target.Kind != prop.Target.Kind {
			continue
		}
		def, ok := actioncatalog.ByKey()[prop.ActionType]
		if !ok {
			return false
		}
		item := actioncatalog.MenuItemForContinuity(def, snap, string(prop.Target.Kind), prop.Target.ID)
		menu := actioncatalog.Menu{Version: actioncatalog.MenuPolicyV1, Catalog: actioncatalog.CatalogV1, Items: []actioncatalog.MenuItem{item}}
		return actioncatalog.ValidateProposal(prop, menu, snap) == nil
	}
	return false
}

func enrichContinuitySnapshot(p PayloadV1) actioncatalog.Snapshot {
	snap := RebuildSnapshot(p)
	for _, e := range p.ThreadEntityContext {
		if e.Role == RoleMain {
			snap.MainIDs[string(e.Entity.Kind)+"|"+e.Entity.ID] = true
		}
	}
	for _, item := range p.ActionMenu.Items {
		for _, id := range item.EligibleTargetIDs {
			if hint := actioncatalog.PriceHintFromMenu(p.ActionMenu, item.TargetKind, id); hint != nil {
				st := snap.SKU[id]
				st.HasPrice = true
				if st.CurrentPrice == nil {
					st.CurrentPrice = hint
				}
				snap.SKU[id] = st
			}
		}
	}
	for _, pa := range p.Continuity.PreviousActions {
		if pa.ActionType != actioncatalog.TypeSKUAdjustPrice || pa.Target.ID == "" {
			continue
		}
		kind := string(pa.Target.Kind)
		if hint := actioncatalog.PriceHintFromMenu(p.ActionMenu, kind, pa.Target.ID); hint != nil {
			st := snap.SKU[pa.Target.ID]
			st.HasPrice = true
			if st.CurrentPrice == nil {
				st.CurrentPrice = hint
			}
			snap.SKU[pa.Target.ID] = st
		}
	}
	for _, it := range p.EvidenceRegistry {
		if !strings.Contains(it.EvidenceType, "PRICE") || it.Subject.ID == "" {
			continue
		}
		m := observedPriceFromEvidence(it.Data)
		if m == nil {
			continue
		}
		st := snap.SKU[it.Subject.ID]
		st.HasPrice = true
		if st.CurrentPrice == nil {
			st.CurrentPrice = m
		}
		snap.SKU[it.Subject.ID] = st
	}
	return snap
}

func observedPriceFromEvidence(raw json.RawMessage) *actioncatalog.Money {
	if len(raw) == 0 {
		return nil
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil
	}
	obs, ok := top["observations"]
	if !ok {
		return nil
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(obs, &rows); err != nil {
		return nil
	}
	for _, row := range rows {
		if amt, ok := row["observed_price"]; ok {
			var money map[string]json.RawMessage
			if err := json.Unmarshal(amt, &money); err != nil {
				continue
			}
			var a float64
			var cur string
			if v, ok := money["amount"]; ok {
				_ = json.Unmarshal(v, &a)
			}
			if v, ok := money["currency"]; ok {
				_ = json.Unmarshal(v, &cur)
			}
			if a > 0 && cur != "" {
				return &actioncatalog.Money{Amount: a, Currency: cur}
			}
		}
		if v, ok := row["provider_current_price"]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err == nil && s != "" {
				return parseAmtCur(s)
			}
		}
	}
	return nil
}

func SortedErrors(errs []string) []string {
	sort.Strings(errs)
	return errs
}
