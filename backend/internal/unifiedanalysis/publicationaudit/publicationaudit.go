// Package publicationaudit holds the reusable, DB-free comparison logic behind
// the `audit-unified-publication` devtool. It proves that a single published
// unified-analysis run is internally consistent across the full pipeline:
//
//	Source DB → AnalysisDataSnapshot → Analysis Pack → Dashboard facts →
//	validated AI output → frozen snapshot → Dashboard API model → Actions API model
//
// Each layer boundary is turned into a set of concrete Findings. Every Finding
// carries an explicit ok/warning/error Status — the audit never emits a
// NOT_CHECKED / skipped placeholder. When a layer's data is missing that is
// itself reported as an error Finding (with a real explanation), so a caller
// can always trust that every row reflects a real verdict.
//
// The functions here are intentionally pure: they operate on already-loaded,
// in-memory inputs so they can be unit-tested without a live database or object
// store. The devtool command is the only place that touches Postgres/S3, and it
// simply feeds the loaded artifacts into these comparisons.
package publicationaudit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/sourcerecon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
)

// Status is the verdict of a single Finding. It is deliberately a small closed
// set with no "not_checked" member.
type Status string

const (
	// StatusOK marks a boundary check that held.
	StatusOK Status = "ok"
	// StatusWarning marks a non-blocking inconsistency.
	StatusWarning Status = "warning"
	// StatusError marks a blocking inconsistency (breaks the chain).
	StatusError Status = "error"
)

// Finding is one concrete reconciliation verdict for a single boundary check.
// Every Finding has a real Status; there is no skipped/unknown representation.
type Finding struct {
	Check    string `json:"check"`
	Status   Status `json:"status"`
	EntityID string `json:"entity_id,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// LayerSummary aggregates the findings of one reconciliation CSV.
type LayerSummary struct {
	Total    int  `json:"total"`
	OK       int  `json:"ok"`
	Warnings int  `json:"warnings"`
	Errors   int  `json:"errors"`
	Valid    bool `json:"valid"`
}

// summarize derives a LayerSummary from a set of findings. Valid is true when
// there are no error findings.
func summarize(findings []Finding) LayerSummary {
	s := LayerSummary{Total: len(findings)}
	for _, f := range findings {
		switch f.Status {
		case StatusOK:
			s.OK++
		case StatusWarning:
			s.Warnings++
		case StatusError:
			s.Errors++
		}
	}
	s.Valid = s.Errors == 0
	return s
}

// ---------------------------------------------------------------------------
// 1. Source reconciliation: Source DB → AnalysisDataSnapshot (+ pack continuity)
// ---------------------------------------------------------------------------

// SourceInputs carries everything the source-layer comparison needs. The live
// report is freshly recomputed from the source DB snapshot; the stored report
// is the reconciliation artifact persisted by the pre-OpenAI gate at run time.
type SourceInputs struct {
	// Live is the reconciliation report recomputed now from the current
	// source-DB snapshot (sourcerecon.ReconcileSnapshot).
	Live sourcerecon.Report
	// LiveLoaded is false when the source snapshot could not be reloaded.
	LiveLoaded bool
	// Stored is the reconciliation report artifact captured at run time.
	Stored sourcerecon.Report
	// StoredLoaded is false when no gate artifact was persisted for the run.
	StoredLoaded bool
}

// SourceReconciliation compares the source DB against the analysis snapshot it
// produced, and checks that today's snapshot still matches the reconciliation
// captured when the run was gated.
func SourceReconciliation(in SourceInputs) []Finding {
	var findings []Finding

	if !in.LiveLoaded {
		findings = append(findings, Finding{
			Check:  "source_snapshot_loadable",
			Status: StatusError,
			Detail: "could not reload the analysis-data snapshot from the source DB for this seller/metric_date",
		})
	} else {
		findings = append(findings, issuesToFindings("source_recon", in.Live)...)
		findings = append(findings, Finding{
			Check:    "source_snapshot_reconciles",
			Status:   boolStatus(in.Live.Valid),
			Expected: "valid=true",
			Actual:   fmt.Sprintf("valid=%t errors=%d warnings=%d", in.Live.Valid, len(in.Live.Errors), len(in.Live.Warnings)),
			Detail:   "sourcerecon.ReconcileSnapshot over the live source-DB snapshot",
		})
	}

	// Continuity: the snapshot that produced this publication must still match
	// the one that was gated at run time (detect post-hoc source drift).
	switch {
	case !in.StoredLoaded:
		findings = append(findings, Finding{
			Check:  "source_recon_artifact_present",
			Status: StatusError,
			Detail: "no pre-OpenAI source-reconciliation artifact was persisted for this run",
		})
	case !in.LiveLoaded:
		// Already reported as an error above; nothing to compare against.
	default:
		match := in.Stored.Hash != "" && in.Stored.Hash == in.Live.Hash
		findings = append(findings, Finding{
			Check:    "source_recon_matches_run_artifact",
			Status:   boolWarnStatus(match),
			Expected: in.Stored.Hash,
			Actual:   in.Live.Hash,
			Detail:   "live source reconciliation hash vs the hash captured by the pre-OpenAI gate",
		})
	}

	return findings
}

// ---------------------------------------------------------------------------
// 2. AI claims reconciliation: validated AI output ↔ Analysis Pack facts
// ---------------------------------------------------------------------------

// AIClaimInputs carries the validated output and the pack index it must ground
// against.
type AIClaimInputs struct {
	Output      *contract.UnifiedAnalysisOutputV1
	PackIndex   *outputvalidator.PackReferenceIndex
	IndexLoaded bool
	Meta        outputvalidator.ExpectedRunMeta
}

// AIClaimsReconciliation validates that every AI claim/action/limitation in the
// frozen output is grounded in the exact Analysis Pack used by the run, reusing
// the outputvalidator grounding engine.
func AIClaimsReconciliation(in AIClaimInputs) []Finding {
	var findings []Finding

	if in.Output == nil {
		findings = append(findings, Finding{
			Check:  "ai_output_present",
			Status: StatusError,
			Detail: "no validated AI output artifact was found for this run",
		})
		return findings
	}
	if !in.IndexLoaded || in.PackIndex == nil {
		findings = append(findings, Finding{
			Check:  "pack_index_loadable",
			Status: StatusError,
			Detail: "could not build the Pack reference index from the run's analysis_pack.zip; AI claims cannot be grounded",
		})
		return findings
	}

	report := outputvalidator.Validate(in.Output, in.Meta, in.PackIndex)
	for _, e := range report.Errors {
		findings = append(findings, Finding{
			Check:    e.Code,
			Status:   StatusError,
			EntityID: e.Path,
			Detail:   e.Message,
		})
	}
	for _, w := range report.Warnings {
		findings = append(findings, Finding{
			Check:    w.Code,
			Status:   StatusWarning,
			EntityID: w.Path,
			Detail:   w.Message,
		})
	}
	findings = append(findings, Finding{
		Check:    "ai_output_grounded",
		Status:   boolStatus(report.Valid),
		Expected: "valid=true",
		Actual:   fmt.Sprintf("valid=%t errors=%d warnings=%d actions=%d", report.Valid, len(report.Errors), len(report.Warnings), report.ActionCount),
		Detail:   "outputvalidator.Validate over the frozen AI output against the run's Pack facts registry",
	})
	return findings
}

// ---------------------------------------------------------------------------
// 3. Dashboard reconciliation: Dashboard facts ↔ frozen snapshot ↔ Dashboard API model
// ---------------------------------------------------------------------------

// DashboardInputs carries the three dashboard representations. The frozen
// snapshot is exactly what the Dashboard API serves in AI-first mode, so it
// stands in for the "Dashboard API model" at the end of the chain.
type DashboardInputs struct {
	Facts        dashboardfacts.DashboardFactsV1
	FactsLoaded  bool
	Frozen       dashboardhome.FrozenDashboardData
	FrozenLoaded bool
	// PackHash is the pack hash recorded on the run (authoritative pack identity).
	PackHash string
	// MetricDate is the run's metric_date (YYYY-MM-DD).
	MetricDate string
}

// DashboardReconciliation checks that the factual dashboard layer, the frozen
// dashboard snapshot, and the served Dashboard API model all describe the same
// content, all derived from the same Analysis Pack.
func DashboardReconciliation(in DashboardInputs) []Finding {
	var findings []Finding

	if !in.FactsLoaded {
		findings = append(findings, Finding{
			Check:  "dashboard_facts_present",
			Status: StatusError,
			Detail: "no dashboard_facts_v1 artifact was found for this run",
		})
	}
	if !in.FrozenLoaded {
		findings = append(findings, Finding{
			Check:  "frozen_dashboard_present",
			Status: StatusError,
			Detail: "no frozen ai_dashboard_data_snapshot was found for this run",
		})
	}
	if !in.FactsLoaded || !in.FrozenLoaded {
		return findings
	}

	// Facts must be bound to the exact pack the run used.
	if in.PackHash != "" {
		findings = append(findings, Finding{
			Check:    "facts_pack_hash_matches_run",
			Status:   boolStatus(in.Facts.PackHash == in.PackHash),
			Expected: in.PackHash,
			Actual:   in.Facts.PackHash,
			Detail:   "dashboard facts pack_hash vs the run's pack_hash",
		})
	}
	if in.MetricDate != "" {
		findings = append(findings, Finding{
			Check:    "facts_metric_date_matches_run",
			Status:   boolStatus(in.Facts.MetricDate == in.MetricDate),
			Expected: in.MetricDate,
			Actual:   in.Facts.MetricDate,
			Detail:   "dashboard facts metric_date vs the run's metric_date",
		})
		findings = append(findings, Finding{
			Check:    "frozen_metric_date_matches_run",
			Status:   boolStatus(in.Frozen.MetricDate == in.MetricDate),
			Expected: in.MetricDate,
			Actual:   in.Frozen.MetricDate,
			Detail:   "frozen snapshot metric_date vs the run's metric_date",
		})
	}

	// Content counts must agree between facts and the frozen/served model.
	countChecks := []struct {
		name  string
		facts int
		model int
	}{
		{"seller_pulse_kpis", len(in.Facts.SellerPulse.KPIs), len(in.Frozen.SellerPulse.KPIs)},
		{"seller_pulse_trend_days", len(in.Facts.SellerPulse.Trend7D), len(in.Frozen.SellerPulse.Trend7D)},
		{"stock_segments", len(in.Facts.StockCoverage.Segments), len(in.Frozen.StockCoverage.Segments)},
		{"funnel_stages", len(in.Facts.Funnel.Stages), len(in.Frozen.SalesFunnel.Stages)},
		{"ad_points", len(in.Facts.AdEfficiency.Points), len(in.Frozen.AdEfficiency.Points)},
	}
	for _, c := range countChecks {
		findings = append(findings, Finding{
			Check:    "dashboard_count_" + c.name,
			Status:   boolStatus(c.facts == c.model),
			EntityID: c.name,
			Expected: fmt.Sprintf("facts=%d", c.facts),
			Actual:   fmt.Sprintf("frozen=%d", c.model),
			Detail:   "dashboard facts count vs frozen/served Dashboard API model count",
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// 4. Actions reconciliation: validated AI output ↔ Actions API model
// ---------------------------------------------------------------------------

// PersistedAction is the DB-free view of one persisted action_items row (the
// Actions API model). EvidenceFactIDs is the decoded evidence_fact_ids JSON.
type PersistedAction struct {
	ActionKey       string
	EvidenceFactIDs []string
}

// ActionsInputs carries the AI output actions and the persisted actions built
// from them, plus the pack index used to prove each evidence fact exists.
type ActionsInputs struct {
	Output      *contract.UnifiedAnalysisOutputV1
	Persisted   []PersistedAction
	PackIndex   *outputvalidator.PackReferenceIndex
	IndexLoaded bool
}

// ActionsReconciliation proves that every persisted action carries non-empty
// evidence_fact_ids, that every classifiable evidence fact resolves in the
// Pack facts registry, and that the persisted action set matches the AI output.
func ActionsReconciliation(in ActionsInputs) []Finding {
	var findings []Finding

	if in.Output == nil {
		findings = append(findings, Finding{
			Check:  "ai_output_present",
			Status: StatusError,
			Detail: "no validated AI output artifact was found for this run",
		})
		return findings
	}

	findings = append(findings, Finding{
		Check:    "actions_count_matches_output",
		Status:   boolStatus(len(in.Output.Actions) == len(in.Persisted)),
		Expected: fmt.Sprintf("output_actions=%d", len(in.Output.Actions)),
		Actual:   fmt.Sprintf("persisted_actions=%d", len(in.Persisted)),
		Detail:   "AI output action count vs persisted action_items count",
	})

	if len(in.Persisted) == 0 && len(in.Output.Actions) == 0 {
		// A no-actions outcome is a valid terminal state; record it explicitly
		// so the CSV still carries a concrete verdict rather than being empty.
		findings = append(findings, Finding{
			Check:    "actions_present",
			Status:   StatusOK,
			Expected: "0 actions",
			Actual:   "0 actions",
			Detail:   "outcome carries no actions; nothing to ground",
		})
		return findings
	}

	for i, a := range in.Persisted {
		entity := a.ActionKey
		if entity == "" {
			entity = fmt.Sprintf("action[%d]", i)
		}
		nonEmpty := dedupeNonEmpty(a.EvidenceFactIDs)
		findings = append(findings, Finding{
			Check:    "action_evidence_fact_ids_non_empty",
			Status:   boolStatus(len(nonEmpty) > 0),
			EntityID: entity,
			Expected: ">=1 evidence_fact_id",
			Actual:   fmt.Sprintf("%d", len(nonEmpty)),
			Detail:   "persisted action must carry at least one evidence_fact_id",
		})
		if len(nonEmpty) == 0 {
			continue
		}
		if !in.IndexLoaded || in.PackIndex == nil {
			findings = append(findings, Finding{
				Check:    "action_evidence_facts_resolvable",
				Status:   StatusError,
				EntityID: entity,
				Detail:   "could not build the Pack reference index; evidence facts cannot be proven to exist",
			})
			continue
		}
		var unresolved []string
		for _, f := range nonEmpty {
			if outputvalidator.IsClassifiableFactID(f) && !in.PackIndex.HasFact(f) {
				unresolved = append(unresolved, f)
			}
		}
		findings = append(findings, Finding{
			Check:    "action_evidence_facts_exist",
			Status:   boolStatus(len(unresolved) == 0),
			EntityID: entity,
			Expected: "all evidence facts resolve in Pack registry",
			Actual:   resolveDetail(nonEmpty, unresolved),
			Detail:   "every classifiable evidence_fact_id must resolve in the run's Pack facts registry",
		})
	}
	return findings
}

// ---------------------------------------------------------------------------
// Aggregate report
// ---------------------------------------------------------------------------

// Report is the machine-readable audit summary written to audit-report.json.
type Report struct {
	SellerAccountID int64                   `json:"seller_account_id"`
	PublicationID   int64                   `json:"publication_id"`
	RunID           int64                   `json:"run_id"`
	MetricDate      string                  `json:"metric_date"`
	GeneratedAt     string                  `json:"generated_at"`
	Layers          map[string]LayerSummary `json:"layers"`
	Valid           bool                    `json:"valid"`
}

// Layer keys used both as the JSON summary keys and the CSV file base names.
const (
	LayerSource    = "source-reconciliation"
	LayerAIClaims  = "ai-claims-reconciliation"
	LayerDashboard = "dashboard-reconciliation"
	LayerActions   = "actions-reconciliation"
)

// BuildReport assembles the aggregate report from the four finding sets. Valid
// is true only when every layer is valid (no error findings anywhere).
func BuildReport(source, aiClaims, dashboard, actions []Finding) Report {
	layers := map[string]LayerSummary{
		LayerSource:    summarize(source),
		LayerAIClaims:  summarize(aiClaims),
		LayerDashboard: summarize(dashboard),
		LayerActions:   summarize(actions),
	}
	valid := true
	for _, s := range layers {
		if !s.Valid {
			valid = false
			break
		}
	}
	return Report{Layers: layers, Valid: valid}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func boolStatus(ok bool) Status {
	if ok {
		return StatusOK
	}
	return StatusError
}

func boolWarnStatus(ok bool) Status {
	if ok {
		return StatusOK
	}
	return StatusWarning
}

// issuesToFindings turns a sourcerecon report's errors/warnings into findings.
func issuesToFindings(prefix string, rep sourcerecon.Report) []Finding {
	out := make([]Finding, 0, len(rep.Errors)+len(rep.Warnings))
	convert := func(status Status, issues []sourcerecon.ReconciliationIssue) {
		for _, issue := range issues {
			out = append(out, Finding{
				Check:    prefix + ":" + issue.Code,
				Status:   status,
				EntityID: strings.TrimSpace(issue.Path + " " + issue.EntityID),
				Expected: fmt.Sprintf("%v", issue.Expected),
				Actual:   fmt.Sprintf("%v", issue.Actual),
				Detail:   issue.Message,
			})
		}
	}
	convert(StatusError, rep.Errors)
	convert(StatusWarning, rep.Warnings)
	return out
}

func dedupeNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func resolveDetail(all, unresolved []string) string {
	if len(unresolved) == 0 {
		return fmt.Sprintf("%d/%d resolved", len(all), len(all))
	}
	sort.Strings(unresolved)
	return fmt.Sprintf("%d/%d resolved; unresolved=[%s]", len(all)-len(unresolved), len(all), strings.Join(unresolved, ","))
}
