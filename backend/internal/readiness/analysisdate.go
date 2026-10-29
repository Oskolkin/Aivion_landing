package readiness

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func lastCompletedMoscow(cutoff time.Time) time.Time {
	return factualapi.LastCompletedBusinessDate(cutoff)
}
func capByKey(caps []Capability, key, scope string) (Capability, bool) {
	for _, c := range caps {
		if c.Key == key && c.Scope == scope {
			return c, true
		}
	}
	return Capability{}, false
}
func (s *Service) resolveDates(ctx context.Context, seller, run int64, cutoff time.Time, st evidenceStats, caps []Capability, watermarks []Watermark) ([]Candidate, *time.Time, error) {
	if stop := analysisDateHardStop(st); stop != nil {
		id, e := s.persistCandidate(ctx, seller, run, cutoff, stop, watermarks)
		stop.EvaluationID = id
		return []Candidate{*stop}, nil, e
	}
	catalog, _ := capByKey(caps, "CATALOG.SKU_UNIVERSE", "HISTORICAL_QUERY")
	account, _ := capByKey(caps, "SALES.ACCOUNT_DEMAND", "HISTORICAL_QUERY")
	sku, _ := capByKey(caps, "SALES.SKU_DEMAND", "HISTORICAL_QUERY")
	planned, selected := selectAnalysisDateCandidates(cutoff, st, catalog, account, sku)
	var out []Candidate
	for i := range planned {
		c := planned[i]
		id, e := s.persistCandidate(ctx, seller, run, cutoff, &c, watermarks)
		if e != nil {
			return nil, nil, e
		}
		c.EvaluationID = id
		out = append(out, c)
	}
	if selected != nil {
		return out, selected, nil
	}
	if len(out) == 0 {
		c := Candidate{Status: GateInsufficient, BlockingReasons: []string{"INSUFFICIENT_ANALYSIS_HISTORY"}}
		id, e := s.persistCandidate(ctx, seller, run, cutoff, &c, watermarks)
		c.EvaluationID = id
		return []Candidate{c}, nil, e
	}
	return out, nil, nil
}

func analysisDateHardStop(st evidenceStats) *Candidate {
	if st.skuCount == 0 {
		return &Candidate{Status: GateEmpty, BlockingReasons: []string{"EMPTY_SKU_UNIVERSE"}}
	}
	if st.maxAccountDate == nil {
		return &Candidate{Status: GateInsufficient, BlockingReasons: []string{"NO_SALES_HISTORY"}}
	}
	return nil
}

// selectAnalysisDateCandidates walks latest completed business days and
// evaluates each candidate day D on its own Core readiness. Historical depth
// is not a hard Analysis Date gate (Physical Implementation Plan §172).
func selectAnalysisDateCandidates(cutoff time.Time, st evidenceStats, catalog, account, sku Capability) ([]Candidate, *time.Time) {
	max := *st.maxAccountDate
	complete := lastCompletedMoscow(cutoff)
	if max.After(complete) {
		max = complete
	}
	var out []Candidate
	earliest := max.AddDate(0, 0, -maxInt(st.salesDays+30, 60))
	for d := max; !d.Before(earliest); d = d.AddDate(0, 0, -1) {
		reasons := candidateDayBlockingReasons(d, st, catalog, account, sku)
		status := GateBlocked
		if len(reasons) == 0 {
			status = GateReady
		}
		candidateDate := d
		c := Candidate{Date: &candidateDate, Status: status, Selected: status == GateReady, BlockingReasons: reasons}
		out = append(out, c)
		if c.Selected {
			return out, &candidateDate
		}
		if len(out) >= maxInt(st.salesDays, 1) {
			break
		}
	}
	return out, nil
}

func candidateDayBlockingReasons(d time.Time, st evidenceStats, catalog, account, sku Capability) []string {
	var reasons []string
	if catalog.FinalState != "READY" {
		reasons = append(reasons, "CATALOG_CORE_CAPABILITY_"+catalog.FinalState)
	}
	if !coreSalesCapabilityEligible(account) {
		reasons = append(reasons, "ACCOUNT_DEMAND_CAPABILITY_"+account.FinalState)
	}
	if !coreSalesCapabilityEligible(sku) {
		reasons = append(reasons, "SKU_DEMAND_CAPABILITY_"+sku.FinalState)
	}
	if catalog.SemanticState != "VALIDATED" || account.SemanticState != "VALIDATED" || sku.SemanticState != "VALIDATED" {
		reasons = append(reasons, "SOURCE_SEMANTICS_PENDING_ACCEPTANCE")
	}
	k := d.Format("2006-01-02")
	if !st.salesDates[k] {
		reasons = append(reasons, "MISSING_DAY:"+k)
		return reasons
	}
	status, ok := st.reconciliation[k]
	if !ok {
		reasons = append(reasons, "MISSING_DAY:"+k)
		return reasons
	}
	if status != "PASS" && status != "EXPLAINED_GAP" {
		reasons = append(reasons, "RECONCILIATION_"+status+":"+k)
	}
	if !st.matureDates[k] {
		reasons = append(reasons, "SALES_NOT_MATURE:"+k)
	}
	return reasons
}

func coreSalesCapabilityEligible(c Capability) bool {
	if c.FinalState == "READY" {
		return true
	}
	if c.FinalState != "PARTIAL" {
		return false
	}
	switch c.ReasonCode {
	case "SALES_RECONCILIATION_EXPLAINED_GAP", "INSUFFICIENT_HISTORY":
		return true
	default:
		return false
	}
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func (s *Service) persistCandidate(ctx context.Context, seller, run int64, cutoff time.Time, c *Candidate, watermarks []Watermark) (int64, error) {
	reasons, _ := json.Marshal(c.BlockingReasons)
	date := ""
	if c.Date != nil {
		date = c.Date.Format("2006-01-02")
	}
	salesSnapshot, catalogSnapshot := watermarkSnapshots(watermarks)
	hash := factualapi.StableHash(fmt.Sprint(seller), fmt.Sprint(run), date, c.Status, fmt.Sprint(c.Selected), string(reasons), cutoff.Format(time.RFC3339Nano), string(salesSnapshot), string(catalogSnapshot))
	var id int64
	pol := factualapi.CurrentPolicies()
	e := s.pool.QueryRow(ctx, `WITH inserted AS(INSERT INTO analysis_readiness_evaluations(seller_account_id,data_quality_run_id,candidate_analysis_date,evaluated_at,cutoff_at,core_gate_status,selected,blocking_reasons_json,temporal_policy_version,dq_policy_version,capability_policy_version,sales_watermark_snapshot_json,catalog_watermark_snapshot_json,evaluation_hash)VALUES($1,$2,$3,$4,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)ON CONFLICT DO NOTHING RETURNING id)SELECT id FROM inserted UNION ALL SELECT id FROM analysis_readiness_evaluations WHERE data_quality_run_id=$2 AND candidate_analysis_date IS NOT DISTINCT FROM $3 LIMIT 1`, seller, run, c.Date, cutoff, c.Status, c.Selected, reasons, factualapi.TemporalPolicyV1, pol.DQ, pol.Capability, salesSnapshot, catalogSnapshot, hash).Scan(&id)
	return id, e
}
func watermarkSnapshots(watermarks []Watermark) ([]byte, []byte) {
	sales := map[string]Watermark{}
	catalog := map[string]Watermark{}
	for _, w := range watermarks {
		switch w.Domain {
		case "SALES_DEMAND":
			sales[w.FactFamily] = w
		case "CATALOG":
			catalog[w.FactFamily] = w
		}
	}
	salesJSON, _ := json.Marshal(sales)
	catalogJSON, _ := json.Marshal(catalog)
	return salesJSON, catalogJSON
}
func (s *Service) buildContext(r Result, ds []domainEvidence) (factualapi.FactualReadContext, error) {
	domains := make([]factualapi.DomainBinding, 0, len(r.Watermarks))
	for _, w := range r.Watermarks {
		d, ok := domainByName(ds, w.Domain)
		if !ok {
			continue
		}
		var runID int64
		if w.DomainRunID != nil {
			runID = *w.DomainRunID
		}
		var finalizedAt *time.Time
		if !d.finishedAt.IsZero() {
			x := d.finishedAt.UTC()
			finalizedAt = &x
		}
		domains = append(domains, factualapi.DomainBinding{
			Domain: w.Domain, FactFamily: w.FactFamily, DomainRunID: runID, FinalizedAt: finalizedAt,
			ContractKey: d.contractKey, ContractVersion: d.contractVersion, ContractHash: w.SourceContractHash,
			CoverageFrom: w.HistoryFrom, CoverageTo: w.HistoryTo, WatermarkHash: w.StateHash, DQRunID: r.DQRunID,
		})
	}
	caps := make([]factualapi.CapabilityBinding, 0, len(r.Capabilities))
	for _, c := range r.Capabilities {
		caps = append(caps, factualapi.CapabilityBinding{CapabilityKey: c.Key, Scope: c.Scope, FinalState: c.FinalState, ReasonCode: c.ReasonCode, AssessmentID: c.ID, AssessedAt: c.AssessedAt})
	}
	return factualapi.NewReadContext(factualapi.ContextSpec{SellerAccountID: r.SellerAccountID, AnalysisDate: *r.AnalysisDate, CutoffAt: r.CutoffAt, Mode: factualapi.ModeLatestStable, Policies: r.Policies, Domains: domains, Capabilities: caps})
}
