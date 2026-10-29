package deep

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"os"
)

type BuildInput struct {
	SellerAccountID int64
	RunID           int64
	UniverseHash    string
	AnalysisDate    time.Time
	CutoffAt        time.Time
	ContractHash    string

	DiscoveryPackSHA      string
	ValidatedDiscoverySHA string
	EvidenceBundleSHA     string
	FactRegistrySHA       string
	ContinuitySHA         string

	DiscoveryEnvelope discovery.AuditEnvelope
	DiscoveryPayload  discovery.PayloadV1
	Validated         discovery.ValidatedOutputV1
	Bundle            evidence.BundleV1
	Handles           evidence.UnifiedHandleRegistryV1
	Continuity        continuity.ContinuityContextV1
}

type BuiltPack struct {
	PackBytes       []byte
	PayloadBytes    []byte
	MenuBytes       []byte
	PolicyBytes     []byte
	ManifestBytes   []byte
	Pack            PackWireV1
	Payload         PayloadV1
	Menu            actioncatalog.Menu
	Snapshot        actioncatalog.Snapshot
	EstimatedTokens int
}

func BuildPack(in BuildInput) (BuiltPack, error) {
	if err := BindInputs(in); err != nil {
		return BuiltPack{}, err
	}
	pol := CurrentRelationshipPolicy()
	payload, snap, err := buildPayload(in, pol)
	if err != nil {
		return BuiltPack{}, err
	}
	budget := CurrentPackBudget()
	maxTok := budget.EffectiveMaxInputTokens()
	est := estimatePayloadTokens(payload)
	if est > maxTok {
		ObservePack("too_large", 0, est, len(payload.FactRegistry), len(payload.EvidenceRegistry))
		return BuiltPack{}, packErr(CodePackTooLarge, fmt.Sprintf("estimated tokens %d exceed budget %d", est, maxTok))
	}
	payloadHash := HashPayloadCanonical(payload)
	payloadBytes, err := MarshalPayloadCanonical(payload)
	if err != nil {
		return BuiltPack{}, err
	}
	menuHash := actioncatalog.HashMenu(payload.ActionMenu)
	env := AuditEnvelope{
		SchemaVersion: PackArtifactSchemaV1, RunID: in.RunID, SellerAccountID: in.SellerAccountID,
		FactualUniverseHash:      in.UniverseHash,
		DiscoveryPackArtifactSHA: in.DiscoveryPackSHA, ValidatedDiscoveryArtifactSHA: in.ValidatedDiscoverySHA,
		EvidenceBundleArtifactSHA: in.EvidenceBundleSHA, AnalysisFactRegistryArtifactSHA: in.FactRegistrySHA,
		ContinuityArtifactSHA: in.ContinuitySHA, ActionCatalogVersion: actioncatalog.CatalogV1,
		ActionMenuHash: menuHash, RelationshipClaimPolicyVersion: pol.Version,
		DeepPackBudgetVersion: PackBudgetV1, PayloadHash: payloadHash, EstimatedInputTokens: est,
		ContractBundleHash: in.ContractHash,
	}
	wire := PackWireV1{AuditEnvelope: env, Payload: payload}
	packBytes, err := MarshalPackWireCanonical(wire)
	if err != nil {
		return BuiltPack{}, err
	}
	menuBytes, err := canonicaljson.Marshal(payload.ActionMenu)
	if err != nil {
		return BuiltPack{}, err
	}
	polBytes, err := canonicaljson.Marshal(pol)
	if err != nil {
		return BuiltPack{}, err
	}
	req := Request{
		Model: CurrentModelPolicy().Model, Instructions: StableInstructionsV2,
		PayloadJSON: payloadBytes, Schema: OutputJSONSchema(), SchemaName: OutputSchemaNameV4,
		MaxOutputTokens: CurrentModelPolicy().MaxOutputTokens,
		ReasoningEffort: CurrentModelPolicy().ReasoningEffort, Verbosity: CurrentModelPolicy().Verbosity,
	}
	man := RequestManifestV1{
		RequestContractVersion: RequestContractV1, Model: req.Model,
		ReasoningEffort: req.ReasoningEffort, Verbosity: req.Verbosity,
		ModelPolicyVersion: ModelPolicyV1, PromptPolicyVersion: PromptPolicyV4,
		PromptHash:          canonicaljson.Hash(StableInstructionsV2),
		OutputSchemaVersion: OutputSchemaV4, OutputSchemaHash: canonicaljson.Hash(OutputJSONSchema()),
		SchemaName: OutputSchemaNameV4, Strict: true, PackHash: "", PayloadHash: payloadHash,
		ActionMenuHash: menuHash, RelationshipPolicyHash: canonicaljson.Hash(pol),
		RequestHash: HashRequest(req), MaxOutputTokens: req.MaxOutputTokens,
		TruncationMode: "disabled", Store: false, ToolsEnabled: false, PreviousResponseEnabled: false,
		EstimatedInputTokens: est,
	}
	manBytes, err := canonicaljson.Marshal(man)
	if err != nil {
		return BuiltPack{}, err
	}
	ObservePack("ok", len(packBytes), est, len(payload.FactRegistry), len(payload.EvidenceRegistry))
	return BuiltPack{
		PackBytes: packBytes, PayloadBytes: payloadBytes, MenuBytes: menuBytes, PolicyBytes: polBytes,
		ManifestBytes: manBytes, Pack: wire, Payload: payload, Menu: payload.ActionMenu, Snapshot: snap, EstimatedTokens: est,
	}, nil
}

func UnmarshalPackWire(body []byte) (AuditEnvelope, PayloadV1, error) {
	var wire PackWireV1
	if err := json.Unmarshal(body, &wire); err != nil {
		return AuditEnvelope{}, PayloadV1{}, err
	}
	return wire.AuditEnvelope, wire.Payload, nil
}

func BindInputs(in BuildInput) error {
	if in.DiscoveryEnvelope.FactualUniverseHash != "" && in.UniverseHash != "" && in.DiscoveryEnvelope.FactualUniverseHash != in.UniverseHash {
		return BindingMismatch("discovery pack universe")
	}
	if in.Bundle.FactualUniverseHash != "" && in.UniverseHash != "" && in.Bundle.FactualUniverseHash != in.UniverseHash {
		return BindingMismatch("evidence bundle universe")
	}
	date := in.AnalysisDate.UTC().Format("2006-01-02")
	if in.DiscoveryPayload.Context.AnalysisDate != "" && in.DiscoveryPayload.Context.AnalysisDate != date {
		return BindingMismatch("analysis_date")
	}
	cutoff := in.CutoffAt.UTC().Format(time.RFC3339Nano)
	if in.DiscoveryPayload.Context.AnalysisCutoffAt != "" && in.DiscoveryPayload.Context.AnalysisCutoffAt != cutoff &&
		in.DiscoveryPayload.Context.AnalysisCutoffAt != in.CutoffAt.UTC().Format(time.RFC3339) {
		if !sameTime(in.DiscoveryPayload.Context.AnalysisCutoffAt, in.CutoffAt) {
			return BindingMismatch("analysis_cutoff_at")
		}
	}
	if in.Bundle.AnalysisDate != "" && in.Bundle.AnalysisDate != date {
		return BindingMismatch("bundle analysis_date")
	}
	if in.DiscoveryEnvelope.SellerAccountID != 0 && in.SellerAccountID != 0 && in.DiscoveryEnvelope.SellerAccountID != in.SellerAccountID {
		return BindingMismatch("seller")
	}
	idx := discovery.IndexFromPayload(in.DiscoveryPayload)
	for _, th := range in.Validated.Threads {
		if _, ok := idx.Lookup(th.PrimaryEntity.Kind, th.PrimaryEntity.ID); !ok {
			return BindingMismatch("validated thread entity missing from discovery pack")
		}
	}
	_ = in.ValidatedDiscoverySHA
	_ = in.EvidenceBundleSHA
	_ = in.FactRegistrySHA
	_ = in.ContinuitySHA
	_ = in.DiscoveryPackSHA
	return nil
}

func sameTime(s string, t time.Time) bool {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, s); err == nil && parsed.UTC().Equal(t.UTC()) {
			return true
		}
	}
	return false
}

func buildPayload(in BuildInput, pol RelationshipPolicyV1) (PayloadV1, actioncatalog.Snapshot, error) {
	main := mainSet(in.DiscoveryPayload)
	ents := collectEntities(in, main)
	need := packFactHandleNeed(in)
	facts := projectFacts(in.Handles, in.Bundle.Items, pol, need)
	links := threadLinks(in.Bundle)
	contLinks := continuitySupportLinks(in.Bundle)
	threads := make([]ThreadView, 0, len(in.Validated.Threads))
	for _, th := range in.Validated.Threads {
		threads = append(threads, ThreadView{
			ThreadRef: th.ThreadID, PrimaryEntity: th.PrimaryEntity, RelatedEntities: th.RelatedEntities,
			Observation: th.Observation, InvestigationFocus: th.InvestigationFocus, BasisFactHandles: th.BasisFactHandles,
		})
	}
	cont := ContinuityView{Mode: "INITIAL_NO_BASE", PreviousActions: []PreviousAction{}}
	if in.Continuity.ContinuityContractVersion == continuity.ContractFromPublicationV1 {
		cont.Mode = "FROM_PUBLICATION"
		cont.PreviousActions = previousFromContinuity(in.Continuity)
	} else if in.Continuity.ContinuityContractVersion == continuity.ContractInitialNoBaseV1 {
		cont.Mode = "INITIAL_NO_BASE"
	}
	reg := stripEvidence(in.Bundle.Items)
	snap := buildSnapshot(in.DiscoveryPayload, in.Bundle, main)
	snap = enrichSnapPricesFromEvidence(snap, in.Bundle.Items)
	menu := actioncatalog.BuildMenu(snap, mainsFromSet(main, in.DiscoveryPayload))
	menu = ensureFactualPriceMenu(menu, snap, mainsFromSet(main, in.DiscoveryPayload))
	if in.Continuity.ContinuityContractVersion == continuity.ContractFromPublicationV1 {
		menu = ensureContinuityMenuItems(menu, snap, in.Continuity)
	}
	if strings.TrimSpace(os.Getenv("DEEP_FAKE_PRICE_MENU")) == "1" {
		menu = ensureFakePriceMenu(menu, snap, ents)
	}
	p := PayloadV1{
		AnalysisContext: PayloadContext{
			AnalysisDate:     in.DiscoveryPayload.Context.AnalysisDate,
			AnalysisCutoffAt: in.DiscoveryPayload.Context.AnalysisCutoffAt,
			BusinessTimezone: in.DiscoveryPayload.Context.BusinessTimezone,
		},
		DomainWindows:                  in.DiscoveryPayload.Windows,
		CapabilitiesAndLimitations:     in.DiscoveryPayload.Capabilities,
		AccountContext:                 in.DiscoveryPayload.Account,
		DiscoveryResult:                DiscoveryResultView{DiscoveryState: string(in.Validated.DiscoveryState), Threads: threads},
		ThreadEntityContext:            ents,
		EvidenceRegistry:               reg,
		ThreadEvidenceLinks:            links,
		ContinuitySupportEvidenceLinks: contLinks,
		FactRegistry:                   facts,
		RelationshipClaimPolicy:        pol,
		ActionMenu:                     menu,
		Continuity:                     cont,
	}
	return p, snap, nil
}

func mainSet(p discovery.PayloadV1) map[string]EntityRef {
	m := map[string]EntityRef{}
	put := func(e EntityRef) { m[string(e.Kind)+"|"+e.ID] = e }
	put(p.Account.Entity)
	for _, c := range p.Categories {
		put(c.Entity)
	}
	for _, s := range p.SKUs {
		put(s.Entity)
	}
	for _, c := range p.Campaigns {
		put(c.Entity)
	}
	return m
}

func collectEntities(in BuildInput, main map[string]EntityRef) []EntityContext {
	seen := map[string]bool{}
	var out []EntityContext
	add := func(e EntityRef) {
		if e.ID == "" {
			return
		}
		k := string(e.Kind) + "|" + e.ID
		if seen[k] {
			return
		}
		seen[k] = true
		role := RoleSupporting
		if _, ok := main[k]; ok {
			role = RoleMain
		}
		out = append(out, EntityContext{Entity: e, Role: role})
	}
	add(in.DiscoveryPayload.Account.Entity)
	for _, th := range in.Validated.Threads {
		add(th.PrimaryEntity)
		for _, r := range th.RelatedEntities {
			add(r)
		}
	}
	for _, it := range in.Bundle.Items {
		add(it.Subject)
		if it.Related != nil {
			add(*it.Related)
		}
	}
	return out
}

func projectFacts(reg evidence.UnifiedHandleRegistryV1, items []evidence.EvidenceItem, pol RelationshipPolicyV1, need map[string]bool) []FactProjection {
	itemViews := make([]evidenceItemView, 0, len(items))
	for _, it := range items {
		itemViews = append(itemViews, evidenceItemView{EvidenceID: it.EvidenceID, EvidenceType: it.EvidenceType, TimeBearing: timeBearing(it)})
	}
	out := make([]FactProjection, 0, len(reg.Entries))
	for _, h := range reg.Entries {
		if len(need) > 0 && !need[h.Handle] {
			continue
		}
		srcKind, srcRef := "DISCOVERY", h.Handle
		if strings.HasPrefix(h.Handle, "ef_") || h.Origin == "EVIDENCE" {
			srcKind, srcRef = "EVIDENCE", firstNonEmpty(h.EvidenceID, h.Handle)
		}
		fp := FactProjection{
			Handle: h.Handle, Entity: h.Entity, SemanticPath: h.SemanticPath,
			SourceKind: srcKind, SourceRef: srcRef, AnalyticalUsability: "USABLE",
		}
		bases := HandleAllowedBases(fp, itemViews, pol)
		var list []string
		for b := range bases {
			list = append(list, b)
		}
		sort.Strings(list)
		fp.AllowedRelationshipBases = list
		out = append(out, fp)
	}
	return out
}

func timeBearing(it evidence.EvidenceItem) bool {
	if it.TemporalScope.From != "" && it.TemporalScope.To != "" {
		return true
	}
	pol := CurrentRelationshipPolicy()
	for _, b := range pol.EvidenceTypeBases[it.EvidenceType] {
		if b == BasisTemporalAssociation || b == BasisCoMovement {
			return true
		}
	}
	return false
}

func continuitySupportLinks(b evidence.BundleV1) []ContinuitySupportEvidenceLink {
	out := make([]ContinuitySupportEvidenceLink, 0, len(b.ContinuitySupportRelations))
	for _, r := range b.ContinuitySupportRelations {
		out = append(out, ContinuitySupportEvidenceLink{
			PreviousCaseRef:     r.PreviousCaseRef,
			PreviousProposalRef: r.PreviousProposalRef,
			SupportKey:          r.SupportKey,
			EvidenceID:          r.EvidenceID,
		})
	}
	return out
}

func threadLinks(b evidence.BundleV1) []ThreadEvidenceLink {
	by := map[string][]string{}
	for _, rel := range b.ThreadEvidenceRelations {
		by[rel.ThreadID] = append(by[rel.ThreadID], rel.EvidenceID)
	}
	var keys []string
	for th := range by {
		keys = append(keys, th)
	}
	sort.Strings(keys)
	var out []ThreadEvidenceLink
	for _, th := range keys {
		out = append(out, ThreadEvidenceLink{ThreadRef: th, EvidenceRefs: uniqStable(by[th])})
	}
	return out
}

func stripEvidence(items []evidence.EvidenceItem) []evidence.EvidenceItem {
	out := make([]evidence.EvidenceItem, 0, len(items))
	for _, it := range items {
		it.Data = evidence.NormalizeEvidenceDataBytes(it.Data)
		it.Data = stripSecrets(it.Data)
		it.FactHandles = nil
		it.EvidenceTypeVersion = ""
		if len(it.Limitations) == 0 {
			it.Limitations = nil
		}
		if !it.Truncation.Truncated && it.Truncation.Returned == 0 && it.Truncation.SortBasis == "" {
			it.Truncation = evidence.TruncationMeta{}
		}
		out = append(out, it)
	}
	return out
}

func stripSecrets(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	s := string(raw)
	for _, bad := range []string{"password", "authorization", "api_key", "apiKey", "secret", "access_token"} {
		if strings.Contains(strings.ToLower(s), strings.ToLower(bad)) {
			return json.RawMessage(`null`)
		}
	}
	return raw
}

func mainsFromSet(main map[string]EntityRef, p discovery.PayloadV1) []actioncatalog.MainEntity {
	var out []actioncatalog.MainEntity
	for _, e := range main {
		out = append(out, actioncatalog.MainEntity{Ref: e})
	}
	_ = p
	return out
}

func buildSnapshot(p discovery.PayloadV1, bundle evidence.BundleV1, main map[string]EntityRef) actioncatalog.Snapshot {
	snap := actioncatalog.Snapshot{
		Capabilities: map[string]string{},
		SKU:          map[string]actioncatalog.SKUState{},
		Category:     map[string]actioncatalog.CategoryState{},
		Campaign:     map[string]actioncatalog.CampaignState{},
		MainIDs:      map[string]bool{},
	}
	for k := range main {
		snap.MainIDs[k] = true
	}
	for _, c := range p.Capabilities {
		snap.Capabilities[c.Key] = c.Readiness
	}
	var materialSKUs []string
	for _, s := range p.SKUs {
		st := snap.SKU[s.Entity.ID]
		if s.Price != nil && s.Price.ObservedPrice != nil {
			st.HasPrice = true
			st.CurrentPrice = parseMoneyPtr(s.Price.ObservedPrice.Value)
		}
		if s.Stock != nil {
			st.HasStockContext = true
		}
		if s.Sales != nil {
			st.HasDemandContext = true
		}
		if s.Supply != nil && s.Supply.OpenInbound != nil {
			st.HasReplenishmentContext = true
		}
		snap.SKU[s.Entity.ID] = st
		materialSKUs = append(materialSKUs, s.Entity.ID)
	}
	for _, c := range p.Categories {
		snap.Category[c.Entity.ID] = actioncatalog.CategoryState{MaterialSKURefs: append([]string(nil), materialSKUs...)}
	}
	for _, c := range p.Campaigns {
		snap.Campaign[c.Entity.ID] = actioncatalog.CampaignState{}
	}
	for _, it := range bundle.Items {
		applyEvidenceState(&snap, it)
	}
	return snap
}

func applyEvidenceState(snap *actioncatalog.Snapshot, it evidence.EvidenceItem) {
	if it.Subject.Kind != discovery.KindSKU {
		return
	}
	st := snap.SKU[it.Subject.ID]
	switch {
	case strings.Contains(it.EvidenceType, "PRICE"):
		st.HasPrice = true
		if m := priceFromEvidenceData(it.Data); m != nil {
			st.CurrentPrice = m
		}
	case strings.Contains(it.EvidenceType, "STOCK"):
		st.HasStockContext = true
	case strings.Contains(it.EvidenceType, "SALES"):
		st.HasDemandContext = true
	case strings.Contains(it.EvidenceType, "SUPPLY"):
		st.HasReplenishmentContext = true
	}
	snap.SKU[it.Subject.ID] = st
}

func priceFromEvidenceData(raw json.RawMessage) *actioncatalog.Money {
	if len(raw) == 0 {
		return nil
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	if cur, ok := v["current"].(map[string]any); ok {
		return moneyFromAnyMap(cur)
	}
	if obs, ok := v["observations"].([]any); ok && len(obs) > 0 {
		if m, ok := obs[0].(map[string]any); ok {
			if s, ok := m["customer_price"].(string); ok && s != "" {
				if p := parseMoneyPtr(&s); p != nil {
					return p
				}
			}
			if s, ok := m["provider_current_price"].(string); ok && s != "" {
				if p := parseMoneyPtr(&s); p != nil {
					return p
				}
			}
			return moneyFromAnyMap(m)
		}
	}
	if pts, ok := v["points"].([]any); ok && len(pts) > 0 {
		if m, ok := pts[len(pts)-1].(map[string]any); ok {
			return moneyFromAnyMap(m)
		}
	}
	return nil
}

func moneyFromAnyMap(m map[string]any) *actioncatalog.Money {
	amt, ok := floatFromAny(m["amount"])
	if !ok {
		if s, ok := m["price"].(string); ok {
			if p := parseMoneyPtr(&s); p != nil {
				return p
			}
		}
		return nil
	}
	cur := "RUB"
	if c, ok := m["currency"].(string); ok && c != "" {
		cur = c
	}
	return &actioncatalog.Money{Amount: amt, Currency: cur}
}

func floatFromAny(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(strings.ReplaceAll(t, ",", ""), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func parseMoneyPtr(v *string) *actioncatalog.Money {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	fields := strings.Fields(*v)
	if len(fields) == 0 {
		return nil
	}
	amt, err := strconv.ParseFloat(strings.ReplaceAll(fields[0], ",", ""), 64)
	if err != nil {
		return nil
	}
	cur := "RUB"
	if len(fields) > 1 {
		cur = fields[1]
	}
	return &actioncatalog.Money{Amount: amt, Currency: cur}
}

func uniqStable(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if x == "" || seen[x] {
			continue
		}
		seen[x] = true
		out = append(out, x)
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func ensureFakePriceMenu(menu actioncatalog.Menu, snap actioncatalog.Snapshot, ents []EntityContext) actioncatalog.Menu {
	def, ok := actioncatalog.ByKey()[actioncatalog.TypeSKUAdjustPrice]
	if !ok {
		return menu
	}
	for _, e := range ents {
		if e.Role != RoleMain || e.Entity.Kind != discovery.KindSKU || e.Entity.ID == "" {
			continue
		}
		st := snap.SKU[e.Entity.ID]
		if st.CurrentPrice == nil {
			st.HasPrice = true
			st.CurrentPrice = &actioncatalog.Money{Amount: 1000, Currency: "RUB"}
			snap.SKU[e.Entity.ID] = st
		}
		found := false
		for i, it := range menu.Items {
			if it.ActionType != actioncatalog.TypeSKUAdjustPrice {
				continue
			}
			found = true
			if !containsStr(it.EligibleTargetIDs, e.Entity.ID) {
				menu.Items[i].EligibleTargetIDs = append(menu.Items[i].EligibleTargetIDs, e.Entity.ID)
				sort.Strings(menu.Items[i].EligibleTargetIDs)
			}
			break
		}
		if !found {
			menu.Items = append(menu.Items, actioncatalog.MenuItemForContinuity(def, snap, string(e.Entity.Kind), e.Entity.ID))
		}
	}
	sort.Slice(menu.Items, func(i, j int) bool { return menu.Items[i].ActionType < menu.Items[j].ActionType })
	return menu
}

func enrichSnapPricesFromEvidence(snap actioncatalog.Snapshot, items []evidence.EvidenceItem) actioncatalog.Snapshot {
	for _, it := range items {
		if !strings.Contains(it.EvidenceType, "PRICE") || it.Subject.ID == "" {
			continue
		}
		m := priceFromEvidenceData(it.Data)
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

func ensureFactualPriceMenu(menu actioncatalog.Menu, snap actioncatalog.Snapshot, mains []actioncatalog.MainEntity) actioncatalog.Menu {
	def, ok := actioncatalog.ByKey()[actioncatalog.TypeSKUAdjustPrice]
	if !ok {
		return menu
	}
	for _, m := range mains {
		if m.Ref.Kind != discovery.KindSKU {
			continue
		}
		st := snap.SKU[m.Ref.ID]
		if st.CurrentPrice == nil {
			continue
		}
		found := false
		for i, it := range menu.Items {
			if it.ActionType != actioncatalog.TypeSKUAdjustPrice {
				continue
			}
			found = true
			if !containsStr(it.EligibleTargetIDs, m.Ref.ID) {
				menu.Items[i].EligibleTargetIDs = append(menu.Items[i].EligibleTargetIDs, m.Ref.ID)
				sort.Strings(menu.Items[i].EligibleTargetIDs)
			}
			break
		}
		if found {
			continue
		}
		menu.Items = append(menu.Items, actioncatalog.MenuItemForContinuity(def, snap, string(m.Ref.Kind), m.Ref.ID))
	}
	sort.Slice(menu.Items, func(i, j int) bool { return menu.Items[i].ActionType < menu.Items[j].ActionType })
	return menu
}

func ensureContinuityMenuItems(menu actioncatalog.Menu, snap actioncatalog.Snapshot, cont continuity.ContinuityContextV1) actioncatalog.Menu {
	for _, pa := range cont.PreviousActions {
		if pa.ActionType == "" || pa.Target.ID == "" {
			continue
		}
		def, ok := actioncatalog.ByKey()[pa.ActionType]
		if !ok {
			continue
		}
		targetKind := string(pa.Target.Kind)
		if targetKind == "" {
			targetKind = string(def.TargetKind)
		}
		if pa.ActionType == actioncatalog.TypeSKUAdjustPrice {
			st := snap.SKU[pa.Target.ID]
			if st.CurrentPrice == nil {
				st.HasPrice = true
				st.CurrentPrice = &actioncatalog.Money{Amount: 1000, Currency: "RUB"}
				snap.SKU[pa.Target.ID] = st
			}
		}
		foundType := false
		for i, it := range menu.Items {
			if it.ActionType != pa.ActionType {
				continue
			}
			foundType = true
			if !containsStr(it.EligibleTargetIDs, pa.Target.ID) {
				menu.Items[i].EligibleTargetIDs = append(menu.Items[i].EligibleTargetIDs, pa.Target.ID)
				sort.Strings(menu.Items[i].EligibleTargetIDs)
			}
			if pa.ActionType == actioncatalog.TypeSKUAdjustPrice {
				menu.Items[i] = mergePriceMenuItem(menu.Items[i], actioncatalog.MenuItemForContinuity(def, snap, targetKind, pa.Target.ID))
			}
			break
		}
		if foundType {
			continue
		}
		menu.Items = append(menu.Items, actioncatalog.MenuItemForContinuity(def, snap, targetKind, pa.Target.ID))
	}
	sort.Slice(menu.Items, func(i, j int) bool { return menu.Items[i].ActionType < menu.Items[j].ActionType })
	return menu
}

func mergePriceMenuItem(dst, src actioncatalog.MenuItem) actioncatalog.MenuItem {
	seen := map[string]bool{}
	for _, id := range dst.EligibleTargetIDs {
		seen[id] = true
	}
	for _, id := range src.EligibleTargetIDs {
		if !seen[id] {
			dst.EligibleTargetIDs = append(dst.EligibleTargetIDs, id)
			seen[id] = true
		}
	}
	sort.Strings(dst.EligibleTargetIDs)
	seenC := map[string]bool{}
	for _, c := range dst.HardConstraints {
		seenC[c] = true
	}
	for _, c := range src.HardConstraints {
		if !seenC[c] {
			dst.HardConstraints = append(dst.HardConstraints, c)
			seenC[c] = true
		}
	}
	sort.Strings(dst.HardConstraints)
	return dst
}

func containsStr(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func previousFromContinuity(c continuity.ContinuityContextV1) []PreviousAction {
	out := make([]PreviousAction, 0, len(c.PreviousActions))
	for _, a := range c.PreviousActions {
		out = append(out, PreviousAction{
			PreviousActionRef: a.PreviousActionRef, CaseRef: a.CaseRef, ProposalRef: a.ProposalRef,
			SignatureHash: a.SignatureHash, ActionType: a.ActionType, Target: a.Target,
			NormalizedContext: a.NormalizedContext, IntentSignature: a.IntentSignature,
			Parameters:       a.Parameters,
			RequiredSupports: packedSupports(a.RequiredSupports),
			ContextSupports:  packedSupports(a.ContextSupports), CounterSupports: packedSupports(a.CounterSupports),
			SystemState: a.SystemState, UserState: a.UserState, SuppressedUntil: a.SuppressedUntil,
		})
	}
	return out
}

func packedSupports(in []continuity.PackedSupport) []PackedSupport {
	out := make([]PackedSupport, 0, len(in))
	for _, s := range in {
		out = append(out, PackedSupport{
			SupportKey: s.SupportKey, Role: s.Role, Claim: s.Claim,
			FindingRefs: s.FindingRefs, FactHandles: s.FactHandles, EvidenceTypes: s.EvidenceTypes,
			Revalidation: SupportRevalidationSpec{
				EvidenceType:  s.Revalidation.EvidenceType,
				Subject:       s.Revalidation.Subject,
				RelatedEntity: s.Revalidation.RelatedEntity,
			},
		})
	}
	return out
}

func estimatePackTokens(payload []byte) int {
	req := Request{
		Model: CurrentModelPolicy().Model, Instructions: StableInstructionsV2,
		PayloadJSON: payload, Schema: OutputJSONSchema(), SchemaName: OutputSchemaNameV4,
		MaxOutputTokens: CurrentModelPolicy().MaxOutputTokens,
	}
	n, err := EstimateDeepRequestTokens(req)
	if err != nil {
		return aitokens.EstimateBytesTokens(payload) + aitokens.EstimateBytesTokens([]byte(StableInstructionsV2))
	}
	return n
}

func estimatePayloadTokens(payload PayloadV1) int {
	b, err := MarshalPayloadCanonical(payload)
	if err != nil {
		return 0
	}
	return estimatePackTokens(b)
}

func packFactHandleNeed(in BuildInput) map[string]bool {
	need := map[string]bool{}
	for _, th := range in.Validated.Threads {
		for _, h := range th.BasisFactHandles {
			need[h] = true
		}
	}
	bundleEvidence := map[string]bool{}
	for _, it := range in.Bundle.Items {
		bundleEvidence[it.EvidenceID] = true
		for _, fh := range it.FactHandles {
			need[fh] = true
		}
	}
	for _, h := range in.Handles.Entries {
		if bundleEvidence[h.EvidenceID] {
			need[h.Handle] = true
		}
	}
	for _, pa := range in.Continuity.PreviousActions {
		all := append(append(pa.RequiredSupports, pa.ContextSupports...), pa.CounterSupports...)
		for _, sp := range all {
			for _, fh := range sp.FactHandles {
				need[fh] = true
			}
		}
	}
	return need
}

func filterFactRegistry(regs []FactProjection, need map[string]bool) []FactProjection {
	if len(need) == 0 {
		return regs
	}
	out := make([]FactProjection, 0, len(regs))
	for _, f := range regs {
		if need[f.Handle] {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Handle < out[j].Handle })
	return out
}
