package resolve

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ManifestVersion = "resolved_context_manifest_v1"

// Manifest is the deterministic RESOLVED_CONTEXT artifact. No wall-clock, DB PK
// inventiveness, or provider I/O — only frozen readiness/watermark evidence.
type Manifest struct {
	Version            string                    `json:"version"`
	SellerAccountID    int64                     `json:"seller_account_id"`
	AnalysisDate       canonicaljson.Date        `json:"analysis_date"`
	CutoffAt           time.Time                 `json:"cutoff_at"`
	RunKind            string                    `json:"run_kind"`
	IntentHash         string                    `json:"intent_hash"`
	ContractBundleHash string                    `json:"contract_bundle_hash"`
	Policies           factualapi.PolicyVersions `json:"policies"`
	Sources            []SourceBinding           `json:"sources"`
	Capabilities       []CapabilitySnapshot      `json:"capabilities"`
	DQ                 []DQSnapshot              `json:"dq_summaries"`
	Limitations        []Limitation              `json:"limitations,omitempty"`
}

type SourceBinding struct {
	Domain                     string              `json:"domain"`
	FactFamily                 string              `json:"fact_family"`
	SourceBindingKey           string              `json:"source_binding_key"`
	SourceState                string              `json:"source_state"`
	LastAcceptedDomainRunID    *int64              `json:"last_accepted_domain_run_id"`
	CoverageFrom               *canonicaljson.Date `json:"coverage_from,omitempty"`
	CoverageTo                 *canonicaljson.Date `json:"coverage_to,omitempty"`
	SnapshotAsOf               *time.Time          `json:"snapshot_as_of,omitempty"`
	LatestObservedAt           *time.Time          `json:"latest_observed_at,omitempty"`
	LatestCompleteBusinessDate *canonicaljson.Date `json:"latest_complete_business_date,omitempty"`
	LatestMatureBusinessDate   *canonicaljson.Date `json:"latest_mature_business_date,omitempty"`
	LatestCompleteSnapshotAt   *time.Time          `json:"latest_complete_snapshot_at,omitempty"`
	SourceContractKey          string              `json:"source_contract_key,omitempty"`
	SourceContractVersion      string              `json:"source_contract_version,omitempty"`
	SourceContractHash         string              `json:"source_contract_hash,omitempty"`
	SourceEndpointFamily       string              `json:"source_endpoint_family,omitempty"`
	SemanticMappingVersion     string              `json:"semantic_mapping_version,omitempty"`
	SourceRevisionHash         string              `json:"source_revision_hash,omitempty"`
	SemanticMappingHash        string              `json:"semantic_mapping_hash,omitempty"`
	SourceFinalizedAt          *time.Time          `json:"source_finalized_at,omitempty"`
	WatermarkHash              string              `json:"watermark_hash,omitempty"`
	SourceHash                 string              `json:"source_hash"`
	BindingHash                string              `json:"binding_hash"`
	CapabilityState            string              `json:"capability_state,omitempty"`
	RelatedCapabilityKeys      []string            `json:"related_capability_keys,omitempty"`
	DQState                    string              `json:"dq_state,omitempty"`
}

type CapabilitySnapshot struct {
	Key, Scope, FinalState, ReasonCode                          string
	ContractState, AccessState, SourceState, SemanticState      string
	CoverageState, TemporalState, HistoryState, PopulationState string
	PolicyVersion                                               string
	AssessmentID                                                *int64
	AssessedAt                                                  time.Time
	AssessmentHash                                              string
}

type DQSnapshot struct {
	Domain, FactFamily, DQPolicyVersion, DQResultHash string
	HighestSeverity                                   string
	BlockingIssueCount, LimitationIssueCount          int64
	WarningCount                                      int64
	CanonicalIssueSetHash                             string
	DQRunID                                           *int64
}

type Limitation struct {
	Code, Message string
}

type Input struct {
	SellerID           int64
	AnalysisDate       time.Time
	CutoffAt           time.Time // immutable run cutoff — never time.Now()
	RunKind            string
	IntentHash         string
	ContractBundleHash string
	Policies           factualapi.PolicyVersions
}

type Resolver struct{ Pool *pgxpool.Pool }

func (r Resolver) Resolve(ctx context.Context, in Input) (Manifest, []byte, string, error) {
	if r.Pool == nil {
		return Manifest{}, nil, "", fmt.Errorf("resolve: pool required")
	}
	if in.SellerID <= 0 || in.CutoffAt.IsZero() || in.AnalysisDate.IsZero() {
		return Manifest{}, nil, "", fmt.Errorf("resolve: seller, analysis_date and cutoff required")
	}
	if in.Policies.API == "" {
		in.Policies = factualapi.CurrentPolicies()
	}
	m := Manifest{
		Version: ManifestVersion, SellerAccountID: in.SellerID,
		AnalysisDate: canonicaljson.Date(in.AnalysisDate), CutoffAt: in.CutoffAt.UTC(),
		RunKind: in.RunKind, IntentHash: in.IntentHash, ContractBundleHash: in.ContractBundleHash,
		Policies: in.Policies,
	}
	caps, err := r.loadCapabilities(ctx, in.SellerID, in.CutoffAt)
	if err != nil {
		return Manifest{}, nil, "", err
	}
	m.Capabilities = caps
	sources, err := r.loadSources(ctx, in.SellerID, in.CutoffAt, caps)
	if err != nil {
		return Manifest{}, nil, "", err
	}
	m.Sources = sources
	dq, err := r.loadDQ(ctx, in.SellerID, in.CutoffAt)
	if err != nil {
		return Manifest{}, nil, "", err
	}
	m.DQ = dq
	for _, d := range dq {
		if d.DQPolicyVersion == "NOT_EVALUATED" {
			m.Limitations = append(m.Limitations, Limitation{
				Code: "NO_DQ_RUN", Message: "no DQ evaluation existed at analysis cutoff",
			})
			break
		}
	}
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
	body, err := canonicaljson.Marshal(m)
	if err != nil {
		return Manifest{}, nil, "", err
	}
	return m, body, canonicaljson.Hash(m), nil
}

func (r Resolver) loadSources(ctx context.Context, seller int64, cutoff time.Time, caps []CapabilitySnapshot) ([]SourceBinding, error) {
	rows, err := r.Pool.Query(ctx, `SELECT DISTINCT ON (w.seller_account_id, w.domain, w.fact_family)
		w.domain,w.fact_family,COALESCE(w.source_contract_hash,''),w.state_hash,
		w.last_successful_domain_run_id,w.history_available_from,w.history_available_to,
		w.latest_observed_at,w.latest_complete_business_date,w.latest_mature_business_date,w.latest_complete_snapshot_at,
		COALESCE(r.status,''),
		COALESCE(r.source_contract_key,''),COALESCE(r.source_contract_version,''),
		COALESCE(r.source_endpoint_family,''),COALESCE(r.semantic_mapping_version,''),
		COALESCE(r.semantic_mapping_hash,''),COALESCE(r.source_revision_hash,''),r.finished_at
		FROM domain_watermark_revisions w
		LEFT JOIN collection_domain_runs r ON r.id=w.last_successful_domain_run_id
		WHERE w.seller_account_id=$1 AND w.knowledge_time<=$2
		ORDER BY w.seller_account_id,w.domain,w.fact_family,w.knowledge_time DESC,w.id DESC`, seller, cutoff.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SourceBinding
	for rows.Next() {
		var s SourceBinding
		var run *int64
		var from, to, complete, mature *time.Time
		var observed, snap *time.Time
		var storedContractHash, stateHash, runStatus string
		var contractKey, contractVersion, endpointFamily, mappingVersion, mappingHash, revisionHash string
		var finishedAt *time.Time
		if err = rows.Scan(&s.Domain, &s.FactFamily, &storedContractHash, &stateHash, &run, &from, &to, &observed, &complete, &mature, &snap,
			&runStatus, &contractKey, &contractVersion, &endpointFamily, &mappingVersion, &mappingHash, &revisionHash, &finishedAt); err != nil {
			return nil, err
		}
		s.SourceBindingKey = bindingKey(s.Domain, s.FactFamily)
		s.LastAcceptedDomainRunID = run
		s.SourceContractKey = contractKey
		s.SourceContractVersion = contractVersion
		s.SourceEndpointFamily = endpointFamily
		s.SemanticMappingVersion = mappingVersion
		s.SourceFinalizedAt = finishedAt
		s.SemanticMappingHash = mappingHash
		if s.SemanticMappingHash == "" && mappingVersion != "" {
			s.SemanticMappingHash = factualapi.StableHash("semantic_mapping", mappingVersion)
		}
		s.SourceRevisionHash = revisionHash
		if s.SourceRevisionHash != "" && s.SourceRevisionHash == s.SemanticMappingHash {
			s.SourceRevisionHash = "" // legacy aliased mapping hash
		}
		s.SourceContractHash = resolveContractHash(storedContractHash, contractKey, contractVersion, endpointFamily)
		s.WatermarkHash = stateHash
		s.LatestObservedAt = observed
		s.LatestCompleteSnapshotAt = snap
		s.SnapshotAsOf = snap
		if from != nil {
			d := canonicaljson.Date(*from)
			s.CoverageFrom = &d
		}
		if to != nil {
			d := canonicaljson.Date(*to)
			s.CoverageTo = &d
		}
		if complete != nil {
			d := canonicaljson.Date(*complete)
			s.LatestCompleteBusinessDate = &d
		}
		if mature != nil {
			d := canonicaljson.Date(*mature)
			s.LatestMatureBusinessDate = &d
		}
		related := matchCapabilities(caps, s.Domain, s.FactFamily)
		s.RelatedCapabilityKeys = RelatedCapabilityKeys(s.Domain, s.FactFamily)
		// SourceState comes from watermark/run evidence — never from a picked capability.
		s.SourceState = resolveSourceState(observed, run, runStatus)
		s.SourceHash = factualapi.StableHash(s.Domain, s.FactFamily, s.SourceState, s.SourceContractHash, stateHash,
			s.SourceEndpointFamily, s.SemanticMappingVersion, s.SourceRevisionHash,
			fmt.Sprint(ptrTime(observed)), fmt.Sprint(ptrTime(complete)), fmt.Sprint(ptrTime(mature)), fmt.Sprint(ptrTime(snap)))
		relatedHashParts := make([]string, 0, len(related)*2)
		for _, c := range related {
			relatedHashParts = append(relatedHashParts, c.Key, c.FinalState)
		}
		s.BindingHash = factualapi.StableHash(append([]string{s.SourceBindingKey, s.SourceState, s.SourceHash}, relatedHashParts...)...)
		out = append(out, s)
	}
	return out, rows.Err()
}

func resolveContractHash(stored, key, version, endpointFamily string) string {
	if key != "" || version != "" || endpointFamily != "" {
		return factualapi.DeriveSourceContractHash(key, version, endpointFamily)
	}
	if isHex64(stored) {
		return stored
	}
	return stored
}

func isHex64(v string) bool {
	if len(v) != 64 {
		return false
	}
	for _, c := range v {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func bindingKey(domain, factFamily string) string {
	if factFamily == "" {
		return domain
	}
	if strings.Contains(factFamily, ".") || factFamily == domain {
		return factFamily
	}
	return domain + "." + factFamily
}

// resolveSourceState derives SourceState from watermark/run evidence only.
// Capabilities are independent frozen dimensions and must not drive this field.
func resolveSourceState(observed *time.Time, run *int64, runStatus string) string {
	switch strings.ToUpper(runStatus) {
	case "FAILED", "CANCELLED":
		return "FAILED"
	case "PARTIAL", "UNAVAILABLE":
		return runStatus
	case "SUCCEEDED", "SUCCESS", "COMPLETED":
		if observed != nil {
			return "SUCCESS"
		}
		if run != nil {
			return "SUCCESS"
		}
	}
	if observed == nil && run == nil {
		return "NOT_ATTEMPTED"
	}
	if run == nil {
		return "PARTIAL"
	}
	if observed != nil {
		return "SUCCESS"
	}
	return "PARTIAL"
}

func (r Resolver) loadCapabilities(ctx context.Context, seller int64, cutoff time.Time) ([]CapabilitySnapshot, error) {
	rows, err := r.Pool.Query(ctx, `SELECT DISTINCT ON (capability_key,capability_scope)
		id,capability_key,capability_scope,final_state,contract_state,access_state,source_state,semantic_state,
		coverage_state,temporal_state,history_state,population_state,COALESCE(reason_code,''),policy_version,assessed_at
		FROM business_capability_assessments
		WHERE seller_account_id=$1 AND assessed_at<=$2
		ORDER BY capability_key,capability_scope,assessed_at DESC,id DESC`, seller, cutoff.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CapabilitySnapshot
	for rows.Next() {
		var c CapabilitySnapshot
		var id int64
		if err = rows.Scan(&id, &c.Key, &c.Scope, &c.FinalState, &c.ContractState, &c.AccessState, &c.SourceState, &c.SemanticState,
			&c.CoverageState, &c.TemporalState, &c.HistoryState, &c.PopulationState, &c.ReasonCode, &c.PolicyVersion, &c.AssessedAt); err != nil {
			return nil, err
		}
		if id > 0 {
			c.AssessmentID = &id
		}
		c.AssessmentHash = factualapi.StableHash(c.Key, c.Scope, c.FinalState, c.ContractState, c.AccessState, c.SourceState, c.SemanticState,
			c.CoverageState, c.TemporalState, c.HistoryState, c.PopulationState, c.ReasonCode, c.PolicyVersion)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r Resolver) loadDQ(ctx context.Context, seller int64, cutoff time.Time) ([]DQSnapshot, error) {
	var runID int64
	var policy, resultHash string
	err := r.Pool.QueryRow(ctx, `SELECT id,policy_version,COALESCE(result_hash,repeat('0',64))
		FROM data_quality_runs
		WHERE seller_account_id=$1 AND status='COMPLETED' AND completed_at IS NOT NULL AND completed_at<=$2 AND cutoff_at<=$2
		ORDER BY completed_at DESC,id DESC LIMIT 1`, seller, cutoff.UTC()).Scan(&runID, &policy, &resultHash)
	if err != nil {
		if err == pgx.ErrNoRows {
			return notEvaluatedDQ(), nil
		}
		return nil, err
	}
	rows, err := r.Pool.Query(ctx, `SELECT domain,
		COALESCE(max(CASE severity WHEN 'CRITICAL' THEN 4 WHEN 'ERROR' THEN 3 WHEN 'WARNING' THEN 2 WHEN 'INFO' THEN 1 ELSE 0 END),0),
		count(*) FILTER (WHERE readiness_effect IN ('FAIL_CAPABILITY','FAIL_DOMAIN','BLOCK_ANALYSIS_DATE')),
		count(*) FILTER (WHERE readiness_effect='LIMITATION_ONLY'),
		count(*) FILTER (WHERE severity='WARNING')
		FROM data_quality_issues WHERE data_quality_run_id=$1 AND seller_account_id=$2
		GROUP BY domain ORDER BY domain`, runID, seller)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DQSnapshot
	for rows.Next() {
		var domain string
		var sevRank int
		var blocking, limitation, warning int64
		if err = rows.Scan(&domain, &sevRank, &blocking, &limitation, &warning); err != nil {
			return nil, err
		}
		sev := []string{"NONE", "INFO", "WARNING", "ERROR", "CRITICAL"}[sevRank]
		issueHash := factualapi.StableHash(resultHash, domain, fmt.Sprint(blocking), fmt.Sprint(limitation), fmt.Sprint(warning), sev)
		out = append(out, DQSnapshot{
			Domain: domain, FactFamily: "RUN", DQPolicyVersion: policy, DQResultHash: resultHash,
			HighestSeverity: sev, BlockingIssueCount: blocking, LimitationIssueCount: limitation, WarningCount: warning,
			CanonicalIssueSetHash: issueHash, DQRunID: &runID,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		out = append(out, DQSnapshot{
			Domain: "ACCOUNT", FactFamily: "RUN", DQPolicyVersion: policy, DQResultHash: resultHash,
			HighestSeverity: "NONE", CanonicalIssueSetHash: resultHash, DQRunID: &runID,
		})
	}
	return out, nil
}

func notEvaluatedDQ() []DQSnapshot {
	// Honest NO_DQ_RUN: severity stays schema-legal NONE, but policy/result hashes
	// and fact_family mark NOT_EVALUATED so this is never confused with a clean DQ run.
	h := factualapi.StableHash("NO_DQ_RUN", "NOT_EVALUATED")
	return []DQSnapshot{{
		Domain: "ACCOUNT", FactFamily: "NOT_EVALUATED", DQPolicyVersion: "NOT_EVALUATED",
		DQResultHash: h, HighestSeverity: "NONE", CanonicalIssueSetHash: h,
	}}
}

func ptrTime(v *time.Time) string {
	if v == nil {
		return "NULL"
	}
	return v.UTC().Format(time.RFC3339Nano)
}
