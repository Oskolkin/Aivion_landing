package universe

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/resolve"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

const (
	ManifestVersionV1       = "factual_universe_manifest_v1"
	UniversePolicyVersionV1 = "factual_universe_policy_v1"
)

type Source struct {
	Domain                     string              `json:"domain"`
	FactFamily                 string              `json:"fact_family"`
	SourceBindingKey           string              `json:"source_binding_key"`
	SourceState                string              `json:"source_state"`
	LastAcceptedDomainRunID    *int64              `json:"last_accepted_domain_run_id"`
	ContractKey                string              `json:"source_contract_key,omitempty"`
	ContractVersion            string              `json:"source_contract_version,omitempty"`
	ContractHash               string              `json:"source_contract_hash,omitempty"`
	SourceEndpointFamily       string              `json:"source_endpoint_family,omitempty"`
	SemanticMappingVersion     string              `json:"semantic_mapping_version,omitempty"`
	SourceRevisionHash         string              `json:"source_revision_hash,omitempty"`
	SemanticMappingHash        string              `json:"semantic_mapping_hash,omitempty"`
	SourceFinalizedAt          *time.Time          `json:"source_finalized_at,omitempty"`
	SourceHash                 string              `json:"source_hash"`
	BindingHash                string              `json:"binding_hash"`
	WatermarkHash              string              `json:"watermark_hash,omitempty"`
	CoverageFrom               *canonicaljson.Date `json:"coverage_from,omitempty"`
	CoverageTo                 *canonicaljson.Date `json:"coverage_to,omitempty"`
	SnapshotAsOf               *time.Time          `json:"snapshot_as_of,omitempty"`
	LatestObservedAt           *time.Time          `json:"latest_observed_at,omitempty"`
	LatestCompleteBusinessDate *canonicaljson.Date `json:"latest_complete_business_date,omitempty"`
	LatestMatureBusinessDate   *canonicaljson.Date `json:"latest_mature_business_date,omitempty"`
	LatestCompleteSnapshotAt   *time.Time          `json:"latest_complete_snapshot_at,omitempty"`
}

type Capability struct {
	Key, Scope, FinalState, ReasonCode                          string
	ContractState, AccessState, SourceState, SemanticState      string
	CoverageState, TemporalState, HistoryState, PopulationState string
	PolicyVersion                                               string
	AssessmentID                                                *int64 `json:"assessment_id"`
	AssessedAt                                                  time.Time
	AssessmentHash                                              string `json:"assessment_hash,omitempty"`
}

type DQSummary struct {
	Domain                string `json:"domain"`
	FactFamily            string `json:"fact_family"`
	DQPolicyVersion       string `json:"dq_policy_version"`
	DQResultHash          string `json:"dq_result_hash"`
	HighestSeverity       string `json:"highest_severity"`
	BlockingIssueCount    int64  `json:"blocking_issue_count"`
	LimitationIssueCount  int64  `json:"limitation_issue_count"`
	WarningCount          int64  `json:"warning_count"`
	CanonicalIssueSetHash string `json:"canonical_issue_set_hash"`
	DQRunID               *int64 `json:"dq_run_id"`
}

type EntitySet struct {
	EntityKind  string   `json:"entity_kind"`
	SetHash     string   `json:"set_hash"`
	MemberCount int64    `json:"member_count"`
	Members     []string `json:"members,omitempty"`
}

type Limitation struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

type Manifest struct {
	Version               string                    `json:"version"`
	UniversePolicyVersion string                    `json:"universe_policy_version"`
	AnalysisDate          canonicaljson.Date        `json:"analysis_date"`
	CutoffAt              time.Time                 `json:"cutoff_at"`
	ResolvedContextHash   string                    `json:"resolved_context_hash,omitempty"`
	Policies              factualapi.PolicyVersions `json:"policies"`
	Sources               []Source                  `json:"sources"`
	Capabilities          []Capability              `json:"capabilities"`
	DQ                    []DQSummary               `json:"dq_summaries"`
	EntitySets            []EntitySet               `json:"entity_sets"`
	Limitations           []Limitation              `json:"limitations,omitempty"`
}

type Input struct {
	SellerID            int64
	RunID               int64
	StageID             *int64
	ResolvedContextHash string
	Resolved            *resolve.Manifest
	EntitySets          []EntitySet
	Limitations         []Limitation
}

type Binding struct {
	ID            string
	SellerID      int64
	Hash, Context string
	Manifest      Manifest
	Record        analysispg.UniverseRecord
}

func (b Binding) SealedBindingHash() string { return b.Hash }
func (b Binding) SealedUniverseID() string  { return b.ID }

type Sealer struct {
	Repo  *analysispg.Repository
	Store artifacts.ArtifactStore
}

func ManifestFromResolved(resolved resolve.Manifest, resolvedHash string, entitySets []EntitySet, limitations []Limitation) Manifest {
	m := Manifest{
		Version: ManifestVersionV1, UniversePolicyVersion: UniversePolicyVersionV1,
		AnalysisDate: resolved.AnalysisDate, CutoffAt: resolved.CutoffAt, ResolvedContextHash: resolvedHash,
		Policies: resolved.Policies, Limitations: append([]Limitation(nil), limitations...),
	}
	for _, s := range resolved.Sources {
		m.Sources = append(m.Sources, Source{
			Domain: s.Domain, FactFamily: s.FactFamily, SourceBindingKey: s.SourceBindingKey, SourceState: s.SourceState,
			LastAcceptedDomainRunID: s.LastAcceptedDomainRunID, ContractKey: s.SourceContractKey, ContractVersion: s.SourceContractVersion,
			ContractHash: s.SourceContractHash, SourceEndpointFamily: s.SourceEndpointFamily,
			SemanticMappingVersion: s.SemanticMappingVersion, SourceRevisionHash: s.SourceRevisionHash,
			SemanticMappingHash: s.SemanticMappingHash, SourceHash: s.SourceHash, BindingHash: s.BindingHash,
			WatermarkHash: s.WatermarkHash, SourceFinalizedAt: s.SourceFinalizedAt, CoverageFrom: s.CoverageFrom, CoverageTo: s.CoverageTo, SnapshotAsOf: s.SnapshotAsOf,
			LatestObservedAt: s.LatestObservedAt, LatestCompleteBusinessDate: s.LatestCompleteBusinessDate,
			LatestMatureBusinessDate: s.LatestMatureBusinessDate, LatestCompleteSnapshotAt: s.LatestCompleteSnapshotAt,
		})
	}
	for _, c := range resolved.Capabilities {
		m.Capabilities = append(m.Capabilities, Capability{
			Key: c.Key, Scope: c.Scope, FinalState: c.FinalState, ReasonCode: c.ReasonCode,
			ContractState: c.ContractState, AccessState: c.AccessState, SourceState: c.SourceState, SemanticState: c.SemanticState,
			CoverageState: c.CoverageState, TemporalState: c.TemporalState, HistoryState: c.HistoryState, PopulationState: c.PopulationState,
			PolicyVersion: c.PolicyVersion, AssessmentID: c.AssessmentID, AssessedAt: c.AssessedAt, AssessmentHash: c.AssessmentHash,
		})
	}
	for _, d := range resolved.DQ {
		m.DQ = append(m.DQ, DQSummary{
			Domain: d.Domain, FactFamily: d.FactFamily, DQPolicyVersion: d.DQPolicyVersion, DQResultHash: d.DQResultHash,
			HighestSeverity: d.HighestSeverity, BlockingIssueCount: d.BlockingIssueCount, LimitationIssueCount: d.LimitationIssueCount,
			WarningCount: d.WarningCount, CanonicalIssueSetHash: d.CanonicalIssueSetHash, DQRunID: d.DQRunID,
		})
	}
	m.EntitySets = append([]EntitySet(nil), entitySets...)
	normalizeManifest(&m)
	return m
}

func normalizeManifest(m *Manifest) {
	sort.Slice(m.Sources, func(i, j int) bool {
		a, b := m.Sources[i], m.Sources[j]
		if a.Domain != b.Domain {
			return a.Domain < b.Domain
		}
		if a.FactFamily != b.FactFamily {
			return a.FactFamily < b.FactFamily
		}
		return a.SourceBindingKey < b.SourceBindingKey
	})
	sort.Slice(m.Capabilities, func(i, j int) bool {
		return m.Capabilities[i].Key+"/"+m.Capabilities[i].Scope < m.Capabilities[j].Key+"/"+m.Capabilities[j].Scope
	})
	sort.Slice(m.DQ, func(i, j int) bool {
		if m.DQ[i].Domain != m.DQ[j].Domain {
			return m.DQ[i].Domain < m.DQ[j].Domain
		}
		return m.DQ[i].FactFamily < m.DQ[j].FactFamily
	})
	sort.Slice(m.EntitySets, func(i, j int) bool { return m.EntitySets[i].EntityKind < m.EntitySets[j].EntityKind })
	sort.Slice(m.Limitations, func(i, j int) bool { return m.Limitations[i].Code < m.Limitations[j].Code })
	for i := range m.EntitySets {
		sort.Strings(m.EntitySets[i].Members)
	}
}

// SemanticHashInput strips provenance IDs that must not affect universe hash.
func SemanticHashInput(m Manifest) Manifest {
	out := m
	out.Sources = append([]Source(nil), m.Sources...)
	out.Capabilities = append([]Capability(nil), m.Capabilities...)
	out.DQ = append([]DQSummary(nil), m.DQ...)
	out.EntitySets = make([]EntitySet, len(m.EntitySets))
	for i, e := range m.EntitySets {
		// Members live in MinIO only; set_hash+count are the semantic identity in PG.
		out.EntitySets[i] = EntitySet{EntityKind: e.EntityKind, SetHash: e.SetHash, MemberCount: e.MemberCount}
	}
	for i := range out.Sources {
		out.Sources[i].LastAcceptedDomainRunID = nil
		// WatermarkHash and full provenance fields remain in semantic hash (Prompt 9.2 PG columns).
	}
	for i := range out.Capabilities {
		out.Capabilities[i].AssessmentID = nil
	}
	for i := range out.DQ {
		out.DQ[i].DQRunID = nil
	}
	return out
}

func (s Sealer) Seal(ctx context.Context, in Input) (Binding, error) {
	if in.Resolved == nil {
		return Binding{}, fmt.Errorf("universe: SEAL_UNIVERSE requires accepted RESOLVED_CONTEXT")
	}
	m := ManifestFromResolved(*in.Resolved, in.ResolvedContextHash, in.EntitySets, in.Limitations)
	semantic := SemanticHashInput(m)
	body, err := canonicaljson.Marshal(m)
	if err != nil {
		return Binding{}, err
	}
	hash := canonicaljson.Hash(semantic)
	a, err := s.Store.PutImmutable(ctx, artifacts.Location{SellerID: in.SellerID, RunID: in.RunID, Stage: "SEAL_UNIVERSE", Kind: artifacts.FactualUniverseManifest}, body)
	if err != nil {
		return Binding{}, err
	}
	var artifactID *int64
	if in.RunID > 0 {
		id, addErr := s.Repo.AddArtifact(ctx, in.RunID, in.StageID, string(artifacts.FactualUniverseManifest), a.Key, a.Hash, a.Size)
		if addErr != nil {
			return Binding{}, addErr
		}
		artifactID = &id
	}
	p := m.Policies
	sealIn := analysispg.UniverseSealInput{
		Universe: analysispg.UniverseRecord{
			SellerID: in.SellerID, AnalysisDate: time.Time(m.AnalysisDate), CutoffAt: m.CutoffAt,
			ManifestHash: hash, ContextHash: hash, ObjectKey: a.Key, ManifestJSON: body,
			ManifestArtifactID: artifactID, ManifestVersion: ManifestVersionV1, TemporalVersion: p.Temporal,
			CapabilityVersion: p.Capability, DQVersion: p.DQ, APIVersion: p.API, UniversePolicyVersion: UniversePolicyVersionV1,
		},
	}
	for _, src := range m.Sources {
		// Persist exact resolved provenance 1:1 with MinIO. Never invent Domain/"v1"
		// defaults — empty string is valid for NOT NULL columns and preserves integrity.
		row := analysispg.UniverseSource{
			Domain: src.Domain, FactFamily: src.FactFamily, SourceBindingKey: src.SourceBindingKey, SourceState: src.SourceState,
			LastAcceptedDomainRunID: src.LastAcceptedDomainRunID,
			SourceContractKey:       src.ContractKey, SourceContractVersion: src.ContractVersion,
			SourceContractHash:   hexOrHash(src.ContractHash, src.Domain+"|"+src.ContractKey+"|"+src.ContractVersion),
			SemanticMappingHash:  optionalHex(src.SemanticMappingHash),
			SourceEndpointFamily: src.SourceEndpointFamily, SemanticMappingVersion: src.SemanticMappingVersion,
			SourceRevisionHash: optionalHex(src.SourceRevisionHash), WatermarkHash: optionalHex(src.WatermarkHash),
			SourceFinalizedAt: src.SourceFinalizedAt,
			SourceHash:        hexOrHash(src.SourceHash, src.Domain+"|source"), BindingHash: hexOrHash(src.BindingHash, src.Domain+"|binding"),
			SnapshotAsOf: src.SnapshotAsOf, LatestObservedAt: src.LatestObservedAt, LatestCompleteSnapshotAt: src.LatestCompleteSnapshotAt,
		}
		if src.CoverageFrom != nil {
			t := time.Time(*src.CoverageFrom)
			row.CoverageFrom = &t
		}
		if src.CoverageTo != nil {
			t := time.Time(*src.CoverageTo)
			row.CoverageTo = &t
		}
		if src.LatestCompleteBusinessDate != nil {
			t := time.Time(*src.LatestCompleteBusinessDate)
			row.LatestCompleteBusinessDate = &t
		}
		if src.LatestMatureBusinessDate != nil {
			t := time.Time(*src.LatestMatureBusinessDate)
			row.LatestMatureBusinessDate = &t
		}
		sealIn.Sources = append(sealIn.Sources, row)
	}
	for _, cap := range m.Capabilities {
		sealIn.Capabilities = append(sealIn.Capabilities, analysispg.UniverseCapability{
			CapabilityKey: cap.Key, Scope: cap.Scope, FinalState: cap.FinalState, ReasonCode: cap.ReasonCode,
			ContractState: cap.ContractState, AccessState: cap.AccessState, SourceState: cap.SourceState, SemanticState: cap.SemanticState,
			CoverageState: cap.CoverageState, TemporalState: cap.TemporalState, HistoryState: cap.HistoryState, PopulationState: cap.PopulationState,
			PolicyVersion: nz(cap.PolicyVersion, p.Capability), AssessmentID: cap.AssessmentID,
			AssessmentHash: nz(cap.AssessmentHash, factualapi.StableHash(cap.Key, cap.Scope, cap.FinalState, cap.SemanticState, cap.TemporalState)),
			AssessedAt:     cap.AssessedAt,
		})
	}
	for _, dq := range m.DQ {
		sealIn.DQ = append(sealIn.DQ, analysispg.UniverseDQSummary{
			Domain: dq.Domain, FactFamily: dq.FactFamily, DQPolicyVersion: dq.DQPolicyVersion, DQResultHash: dq.DQResultHash,
			HighestSeverity: dq.HighestSeverity, BlockingIssueCount: dq.BlockingIssueCount, LimitationIssueCount: dq.LimitationIssueCount,
			WarningCount: dq.WarningCount, CanonicalIssueSetHash: dq.CanonicalIssueSetHash, SummaryJSON: []byte("{}"), DQRunID: dq.DQRunID,
		})
	}
	for _, es := range m.EntitySets {
		sealIn.EntitySets = append(sealIn.EntitySets, analysispg.UniverseEntitySet{EntityKind: es.EntityKind, SetHash: es.SetHash, MemberCount: es.MemberCount})
	}
	u, err := s.Repo.PutUniverse(ctx, sealIn)
	if err != nil {
		return Binding{}, err
	}
	return Binding{ID: u.ID, SellerID: u.SellerID, Hash: u.ManifestHash, Context: u.ContextHash, Manifest: m, Record: u}, nil
}

func (s Sealer) LoadValidated(ctx context.Context, id string, seller int64) (Binding, error) {
	u, err := s.Repo.GetUniverse(ctx, id)
	if err != nil {
		return Binding{}, err
	}
	if u.SellerID != seller {
		return Binding{}, factualapi.NewError(factualapi.CodeContextSellerMismatch, "sealed universe seller mismatch")
	}
	body, err := s.Store.Get(ctx, u.ObjectKey)
	if err != nil {
		return Binding{}, err
	}
	contentHash := artifacts.HashBytes(body)
	if err = s.Store.Verify(ctx, artifacts.Artifact{Key: u.ObjectKey, Hash: contentHash, Size: int64(len(body))}); err != nil {
		return Binding{}, factualapi.NewError(factualapi.CodeUniverseIntegrityFailure, "sealed universe artifact verify failed")
	}
	var m Manifest
	if err = json.Unmarshal(body, &m); err != nil {
		return Binding{}, factualapi.NewError(factualapi.CodeUniverseIntegrityFailure, "sealed universe manifest decode failed")
	}
	artifactSemantic := canonicaljson.Hash(SemanticHashInput(m))
	if artifactSemantic != u.ManifestHash {
		return Binding{}, factualapi.NewError(factualapi.CodeUniverseIntegrityFailure, "sealed universe manifest hash mismatch")
	}
	dbManifest := manifestFromRecord(u, m)
	dbSemantic := canonicaljson.Hash(SemanticHashInput(dbManifest))
	if dbSemantic != u.ManifestHash {
		return Binding{}, factualapi.NewError(factualapi.CodeUniverseIntegrityFailure, "DB children diverge from MinIO manifest")
	}
	return Binding{ID: u.ID, SellerID: u.SellerID, Hash: u.ManifestHash, Context: u.ContextHash, Manifest: m, Record: u}, nil
}

func childrenMatch(u analysispg.UniverseRecord, m Manifest) bool {
	if len(u.Sources) != len(m.Sources) || len(u.Capabilities) != len(m.Capabilities) || len(u.DQ) != len(m.DQ) || len(u.EntitySets) != len(m.EntitySets) {
		return false
	}
	for i := range m.Sources {
		if u.Sources[i].Domain != m.Sources[i].Domain || u.Sources[i].FactFamily != m.Sources[i].FactFamily ||
			u.Sources[i].SourceState != m.Sources[i].SourceState || u.Sources[i].BindingHash != m.Sources[i].BindingHash {
			return false
		}
	}
	for i := range m.Capabilities {
		if u.Capabilities[i].CapabilityKey != m.Capabilities[i].Key || u.Capabilities[i].FinalState != m.Capabilities[i].FinalState ||
			u.Capabilities[i].SemanticState != m.Capabilities[i].SemanticState || u.Capabilities[i].TemporalState != m.Capabilities[i].TemporalState {
			return false
		}
	}
	for i := range m.DQ {
		if u.DQ[i].Domain != m.DQ[i].Domain || u.DQ[i].CanonicalIssueSetHash != m.DQ[i].CanonicalIssueSetHash {
			return false
		}
	}
	for i := range m.EntitySets {
		if u.EntitySets[i].EntityKind != m.EntitySets[i].EntityKind || u.EntitySets[i].SetHash != m.EntitySets[i].SetHash ||
			u.EntitySets[i].MemberCount != m.EntitySets[i].MemberCount {
			return false
		}
	}
	return true
}

func manifestFromRecord(u analysispg.UniverseRecord, base Manifest) Manifest {
	m := Manifest{
		Version: base.Version, UniversePolicyVersion: base.UniversePolicyVersion,
		AnalysisDate: base.AnalysisDate, CutoffAt: base.CutoffAt, ResolvedContextHash: base.ResolvedContextHash,
		Policies: base.Policies, Limitations: append([]Limitation(nil), base.Limitations...),
	}
	for _, s := range u.Sources {
		src := Source{
			Domain: s.Domain, FactFamily: s.FactFamily, SourceBindingKey: s.SourceBindingKey, SourceState: s.SourceState,
			LastAcceptedDomainRunID: s.LastAcceptedDomainRunID, ContractKey: s.SourceContractKey, ContractVersion: s.SourceContractVersion,
			ContractHash: s.SourceContractHash, SemanticMappingHash: s.SemanticMappingHash, SourceHash: s.SourceHash, BindingHash: s.BindingHash,
			SourceEndpointFamily: s.SourceEndpointFamily, SemanticMappingVersion: s.SemanticMappingVersion,
			SourceRevisionHash: s.SourceRevisionHash, WatermarkHash: s.WatermarkHash, SourceFinalizedAt: s.SourceFinalizedAt,
			SnapshotAsOf: s.SnapshotAsOf, LatestObservedAt: s.LatestObservedAt, LatestCompleteSnapshotAt: s.LatestCompleteSnapshotAt,
		}
		if s.CoverageFrom != nil {
			d := canonicaljson.Date(*s.CoverageFrom)
			src.CoverageFrom = &d
		}
		if s.CoverageTo != nil {
			d := canonicaljson.Date(*s.CoverageTo)
			src.CoverageTo = &d
		}
		if s.LatestCompleteBusinessDate != nil {
			d := canonicaljson.Date(*s.LatestCompleteBusinessDate)
			src.LatestCompleteBusinessDate = &d
		}
		if s.LatestMatureBusinessDate != nil {
			d := canonicaljson.Date(*s.LatestMatureBusinessDate)
			src.LatestMatureBusinessDate = &d
		}
		m.Sources = append(m.Sources, src)
	}
	for _, c := range u.Capabilities {
		m.Capabilities = append(m.Capabilities, Capability{
			Key: c.CapabilityKey, Scope: c.Scope, FinalState: c.FinalState, ReasonCode: c.ReasonCode,
			ContractState: c.ContractState, AccessState: c.AccessState, SourceState: c.SourceState, SemanticState: c.SemanticState,
			CoverageState: c.CoverageState, TemporalState: c.TemporalState, HistoryState: c.HistoryState, PopulationState: c.PopulationState,
			PolicyVersion: c.PolicyVersion, AssessmentID: c.AssessmentID, AssessedAt: c.AssessedAt, AssessmentHash: c.AssessmentHash,
		})
	}
	for _, d := range u.DQ {
		m.DQ = append(m.DQ, DQSummary{
			Domain: d.Domain, FactFamily: d.FactFamily, DQPolicyVersion: d.DQPolicyVersion, DQResultHash: d.DQResultHash,
			HighestSeverity: d.HighestSeverity, BlockingIssueCount: d.BlockingIssueCount, LimitationIssueCount: d.LimitationIssueCount,
			WarningCount: d.WarningCount, CanonicalIssueSetHash: d.CanonicalIssueSetHash, DQRunID: d.DQRunID,
		})
	}
	for _, e := range u.EntitySets {
		es := EntitySet{EntityKind: e.EntityKind, SetHash: e.SetHash, MemberCount: e.MemberCount}
		for _, me := range base.EntitySets {
			if me.EntityKind == e.EntityKind {
				es.Members = append([]string(nil), me.Members...)
				break
			}
		}
		m.EntitySets = append(m.EntitySets, es)
	}
	normalizeManifest(&m)
	return m
}

func NewSealedContext(u analysispg.UniverseRecord, binding Binding) (factualapi.FactualReadContext, error) {
	spec := factualapi.ContextSpec{
		SellerAccountID: u.SellerID, AnalysisDate: u.AnalysisDate, CutoffAt: u.CutoffAt,
		Mode: factualapi.ModeSealedUniverse,
		Policies: factualapi.PolicyVersions{
			Temporal: u.TemporalVersion, Capability: u.CapabilityVersion, DQ: u.DQVersion, API: u.APIVersion,
			SalesMaturity: factualapi.CurrentPolicies().SalesMaturity, StockFreshness: factualapi.CurrentPolicies().StockFreshness,
			PriceFreshness: factualapi.CurrentPolicies().PriceFreshness, AdvertisingMaturity: factualapi.CurrentPolicies().AdvertisingMaturity,
			AttributionAggregation: factualapi.CurrentPolicies().AttributionAggregation,
		},
	}
	if binding.Manifest.Policies.SalesMaturity != "" {
		spec.Policies = binding.Manifest.Policies
	}
	for _, src := range u.Sources {
		var runID int64
		if src.LastAcceptedDomainRunID != nil {
			runID = *src.LastAcceptedDomainRunID
		}
		var finalizedAt *time.Time
		if src.SourceFinalizedAt != nil {
			x := src.SourceFinalizedAt.UTC()
			finalizedAt = &x
		}
		spec.Domains = append(spec.Domains, factualapi.DomainBinding{
			Domain: src.Domain, FactFamily: src.FactFamily, DomainRunID: runID, FinalizedAt: finalizedAt,
			ContractKey: src.SourceContractKey, ContractVersion: src.SourceContractVersion, ContractHash: src.SourceContractHash,
			CoverageFrom: src.CoverageFrom, CoverageTo: src.CoverageTo, WatermarkHash: src.WatermarkHash,
		})
	}
	for _, cap := range u.Capabilities {
		var aid int64
		if cap.AssessmentID != nil {
			aid = *cap.AssessmentID
		}
		spec.Capabilities = append(spec.Capabilities, factualapi.CapabilityBinding{
			CapabilityKey: cap.CapabilityKey, Scope: cap.Scope, FinalState: cap.FinalState, ReasonCode: cap.ReasonCode,
			AssessmentID: aid, AssessedAt: cap.AssessedAt,
			ContractState: cap.ContractState, AccessState: cap.AccessState, SourceState: cap.SourceState, SemanticState: cap.SemanticState,
			CoverageState: cap.CoverageState, TemporalState: cap.TemporalState, HistoryState: cap.HistoryState, PopulationState: cap.PopulationState,
		})
	}
	return factualapi.NewSealedUniverseContext(spec, binding)
}

func nz(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
func hexOrHash(v, seed string) string {
	if looksHex64(v) {
		return v
	}
	return factualapi.StableHash(seed, v)
}
func optionalHex(v string) string {
	if v == "" {
		return ""
	}
	return hexOrHash(v, v)
}
func looksHex64(v string) bool {
	if len(v) != 64 {
		return false
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
