// Package v11validate implements Part 1 structural/semantic checks for
// aivion_unified_analysis_v11. It is intentionally NOT wired into the
// production ValidateOutput path (v10 remains current).
package v11validate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	policyv2 "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/policy/v2"
)

// Issue is one validation problem.
type Issue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Report aggregates validation issues.
type Report struct {
	Valid  bool    `json:"valid"`
	Issues []Issue `json:"issues"`
}

func (r *Report) add(code, path, msg string) {
	r.Valid = false
	r.Issues = append(r.Issues, Issue{Code: code, Path: path, Message: msg})
}

var (
	reClaimID     = regexp.MustCompile(`^c_[a-z0-9_]{3,80}$`)
	reThreshold   = regexp.MustCompile(`^t_[a-z0-9_]{3,80}$`)
	reFindingID   = regexp.MustCompile(`^f_[a-z0-9_]{3,100}$`)
	reActionID    = regexp.MustCompile(`^a_[a-z0-9_]{3,100}$`)
	rePlaceholder = regexp.MustCompile(`\{\{(claim|threshold):([a-z0-9_]+)\.([a-z0-9_]+)\}\}`)
	reDigits      = regexp.MustCompile(`[0-9]+([.,][0-9]+)?`)
)

var allowedClaimParts = map[string]bool{
	"current": true, "previous": true, "delta": true,
	"period_current": true, "period_previous": true,
}
var allowedThresholdParts = map[string]bool{"value": true, "operator": true}

// Validate runs schema-ish decode checks already assumed, then semantic/text/ref rules.
func Validate(out *contract.UnifiedAnalysisOutputV11, policy *policyv2.Policy) Report {
	r := Report{Valid: true}
	if out == nil {
		r.add("nil_output", "$", "output is nil")
		return r
	}
	if out.SchemaVersion != contract.UnifiedAnalysisOutputSchemaVersionV11 {
		r.add("schema_version_mismatch", "schema_version", fmt.Sprintf("got %q", out.SchemaVersion))
	}
	validateIDsAndRefs(out, &r)
	validateTextRules(out, policy, &r)
	validateFindings(out, policy, &r)
	validateActions(out, policy, &r)
	validateSectionRelevance(out, &r)
	return r
}

// ValidateJSON unmarshals and validates.
func ValidateJSON(raw []byte, policy *policyv2.Policy) (Report, *contract.UnifiedAnalysisOutputV11) {
	var out contract.UnifiedAnalysisOutputV11
	if err := json.Unmarshal(raw, &out); err != nil {
		r := Report{Valid: false, Issues: []Issue{{Code: "invalid_json", Path: "$", Message: err.Error()}}}
		return r, nil
	}
	return Validate(&out, policy), &out
}

func validateIDsAndRefs(out *contract.UnifiedAnalysisOutputV11, r *Report) {
	claims := map[string]contract.UnifiedMetricClaimV11{}
	for i, c := range out.MetricClaims {
		path := fmt.Sprintf("metric_claims[%d]", i)
		if !reClaimID.MatchString(c.ClaimID) {
			r.add("claim_invalid_id", path+".claim_id", c.ClaimID)
		}
		if _, dup := claims[c.ClaimID]; dup {
			r.add("claim_duplicate_id", path+".claim_id", c.ClaimID)
		}
		claims[c.ClaimID] = c
	}
	thresholds := map[string]contract.UnifiedThresholdClaimV11{}
	for i, t := range out.ThresholdClaims {
		path := fmt.Sprintf("threshold_claims[%d]", i)
		if !reThreshold.MatchString(t.ClaimID) {
			r.add("threshold_invalid_id", path+".claim_id", t.ClaimID)
		}
		if _, ok := claims[t.MetricClaimID]; !ok {
			r.add("threshold_metric_claim_missing", path+".metric_claim_id", t.MetricClaimID)
		}
		if _, dup := thresholds[t.ClaimID]; dup {
			r.add("threshold_duplicate_id", path+".claim_id", t.ClaimID)
		}
		thresholds[t.ClaimID] = t
	}
	findings := map[string]contract.UnifiedFindingV11{}
	for i, f := range out.Findings {
		path := fmt.Sprintf("findings[%d]", i)
		if !reFindingID.MatchString(f.FindingID) {
			r.add("finding_invalid_id", path+".finding_id", f.FindingID)
		}
		if _, dup := findings[f.FindingID]; dup {
			r.add("finding_duplicate_id", path+".finding_id", f.FindingID)
		}
		findings[f.FindingID] = f
		for _, cid := range f.MetricClaimIDs {
			if _, ok := claims[cid]; !ok {
				r.add("finding_claim_missing", path+".metric_claim_ids", cid)
			}
		}
		for _, tid := range f.ThresholdClaimIDs {
			if _, ok := thresholds[tid]; !ok {
				r.add("finding_threshold_missing", path+".threshold_claim_ids", tid)
			}
		}
	}
	actions := map[string]contract.UnifiedActionV11{}
	for i, a := range out.Actions {
		path := fmt.Sprintf("actions[%d]", i)
		if !reActionID.MatchString(a.AnalysisActionID) {
			r.add("action_invalid_id", path+".analysis_action_id", a.AnalysisActionID)
		}
		if _, dup := actions[a.AnalysisActionID]; dup {
			r.add("action_duplicate_id", path+".analysis_action_id", a.AnalysisActionID)
		}
		actions[a.AnalysisActionID] = a
		if len(a.FindingIDs) == 0 {
			r.add("action_missing_finding", path+".finding_ids", "required at least one")
		}
		for _, fid := range a.FindingIDs {
			if _, ok := findings[fid]; !ok {
				r.add("action_finding_missing", path+".finding_ids", fid)
			}
		}
	}
	checkFindingList := func(path string, ids []string) {
		for _, id := range ids {
			if _, ok := findings[id]; !ok {
				r.add("dashboard_finding_missing", path, id)
			}
		}
	}
	d := out.Dashboard
	checkFindingList("dashboard.account_diagnosis.primary_finding_ids", d.AccountDiagnosis.PrimaryFindingIDs)
	checkFindingList("dashboard.account_diagnosis.positive_finding_ids", d.AccountDiagnosis.PositiveFindingIDs)
	checkFindingList("dashboard.funnel_insight.finding_ids", d.FunnelInsight.FindingIDs)
	checkFindingList("dashboard.stock_insight.primary_finding_ids", d.StockInsight.PrimaryFindingIDs)
	checkFindingList("dashboard.advertising_insight.primary_finding_ids", d.AdvertisingInsight.PrimaryFindingIDs)
	checkFindingList("dashboard.seller_pulse_insight.key_point_finding_ids", d.SellerPulseInsight.KeyPointFindingIDs)
	checkFindingList("dashboard.seller_pulse_insight.important_fact_finding_ids", d.SellerPulseInsight.ImportantFactFindingIDs)
	for _, lever := range d.AccountDiagnosis.ImprovementLevers {
		if _, ok := findings[lever.FindingID]; !ok {
			r.add("dashboard_finding_missing", "dashboard.account_diagnosis.improvement_levers.finding_id", lever.FindingID)
		}
		if lever.LinkedActionID != nil && *lever.LinkedActionID != "" {
			if _, ok := actions[*lever.LinkedActionID]; !ok {
				r.add("dashboard_action_missing", "dashboard.account_diagnosis.improvement_levers.linked_action_id", *lever.LinkedActionID)
			}
		}
	}
}

func validateTextRules(out *contract.UnifiedAnalysisOutputV11, policy *policyv2.Policy, r *Report) {
	_ = policy
	check := func(path, text string) {
		if text == "" {
			return
		}
		if strings.Contains(text, "<") || strings.Contains(text, ">") {
			r.add("html_forbidden", path, "HTML markers not allowed")
		}
		for _, m := range rePlaceholder.FindAllStringSubmatch(text, -1) {
			kind, id, part := m[1], m[2], m[3]
			if kind == "claim" {
				if !allowedClaimParts[part] {
					r.add("placeholder_unknown_part", path, m[0])
				}
				if !strings.HasPrefix(id, "c_") {
					r.add("placeholder_invalid_id", path, id)
				}
			} else {
				if !allowedThresholdParts[part] {
					r.add("placeholder_unknown_part", path, m[0])
				}
				if !strings.HasPrefix(id, "t_") {
					r.add("placeholder_invalid_id", path, id)
				}
			}
		}
		// Strip placeholders then reject remaining business-looking digits.
		stripped := rePlaceholder.ReplaceAllString(text, "")
		if reDigits.MatchString(stripped) && looksLikeBusinessNumber(stripped) {
			r.add("raw_business_number", path, "raw business number in user-facing text")
		}
		for _, tok := range []string{"health_score", "stock_coverage_days_7d", "available_stock_units", "campaign_drr_pct"} {
			if strings.Contains(stripped, tok) {
				r.add("internal_field_name", path, tok)
			}
		}
	}
	for i, f := range out.Findings {
		p := fmt.Sprintf("findings[%d]", i)
		check(p+".title", f.Title)
		check(p+".explanation_template", f.ExplanationTemplate)
		if f.BusinessImpactTemplate != nil {
			check(p+".business_impact_template", *f.BusinessImpactTemplate)
		}
	}
	d := out.Dashboard
	if d.AccountDiagnosis.Headline != nil {
		check("dashboard.account_diagnosis.headline", *d.AccountDiagnosis.Headline)
	}
	if d.AccountDiagnosis.ExecutiveSummaryTemplate != nil {
		check("dashboard.account_diagnosis.executive_summary_template", *d.AccountDiagnosis.ExecutiveSummaryTemplate)
	}
	if d.FunnelInsight.DiagnosisTemplate != nil {
		check("dashboard.funnel_insight.diagnosis_template", *d.FunnelInsight.DiagnosisTemplate)
	}
	if d.StockInsight.DiagnosisTemplate != nil {
		check("dashboard.stock_insight.diagnosis_template", *d.StockInsight.DiagnosisTemplate)
	}
	if d.AdvertisingInsight.DiagnosisTemplate != nil {
		check("dashboard.advertising_insight.diagnosis_template", *d.AdvertisingInsight.DiagnosisTemplate)
	}
	if d.SellerPulseInsight.SummaryTemplate != nil {
		check("dashboard.seller_pulse_insight.summary_template", *d.SellerPulseInsight.SummaryTemplate)
	}
	for i, a := range out.Actions {
		p := fmt.Sprintf("actions[%d]", i)
		check(p+".title", a.Title)
		check(p+".situation_template", a.SituationTemplate)
		check(p+".goal", a.Goal)
		check(p+".expected_effect_template", a.ExpectedEffectTemplate)
		for j, step := range a.Plan {
			check(fmt.Sprintf("%s.plan[%d].instruction", p, j), step.Instruction)
			check(fmt.Sprintf("%s.plan[%d].rationale_template", p, j), step.RationaleTemplate)
			check(fmt.Sprintf("%s.plan[%d].expected_result", p, j), step.ExpectedResult)
		}
	}
}

func looksLikeBusinessNumber(s string) bool {
	// Ignore pure order integers in isolation is hard; treat any digit run as business
	// number in user-facing text (IDs live outside these fields).
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func knownMetricCodes(policy *policyv2.Policy) map[string]bool {
	out := map[string]bool{}
	if policy == nil {
		return out
	}
	for _, name := range policy.FindingTypesSorted() {
		ft, ok := policy.FindingType(name)
		if !ok {
			continue
		}
		for _, m := range ft.RequiredMetricCodes {
			out[m] = true
		}
	}
	for _, th := range policy.Thresholds {
		out[th.MetricCode] = true
	}
	// Common Pack dictionary metrics used by fixtures / claims.
	for _, m := range []string{
		"available_stock_units", "spend_7d", "ad_orders_7d", "orders_7d", "revenue_7d",
		"impression_to_view_pct", "view_to_cart_pct", "cart_to_order_pct", "order_to_delivered_pct",
		"stock_coverage_days_7d", "campaign_drr_pct", "drr_pct",
		"ad_spend_share_of_total_revenue_pct", "average_order_value_7d", "impressions_7d",
	} {
		out[m] = true
	}
	return out
}

func validateFindings(out *contract.UnifiedAnalysisOutputV11, policy *policyv2.Policy, r *Report) {
	metrics := knownMetricCodes(policy)
	for i, c := range out.MetricClaims {
		if len(metrics) > 0 && !metrics[c.MetricCode] {
			r.add("claim_unknown_metric_code", fmt.Sprintf("metric_claims[%d].metric_code", i), c.MetricCode)
		}
	}
	for i, f := range out.Findings {
		path := fmt.Sprintf("findings[%d]", i)
		if policy != nil {
			ft, ok := policy.FindingType(f.FindingType)
			if !ok {
				r.add("finding_unknown_type", path+".finding_type", f.FindingType)
			} else if ft.Domain != f.Domain {
				r.add("finding_domain_mismatch", path+".domain", fmt.Sprintf("%s vs registry %s", f.Domain, ft.Domain))
			}
		}
		if f.Direction == "positive" && f.Severity == "critical" {
			r.add("finding_invalid_severity", path+".severity", "positive finding cannot be critical")
		}
		if f.Direction == "neutral" && f.Severity == "critical" {
			r.add("finding_invalid_severity", path+".severity", "neutral finding cannot be critical")
		}
		if f.Interpretation == "hypothesis" && f.Confidence == "high" {
			r.add("finding_hypothesis_high_confidence", path+".confidence", "hypothesis cannot be high")
		}
		switch f.PrimaryBlock {
		case "account", "what_changed", "category", "funnel", "stock", "advertising", "seller_pulse", "sku_focus":
		default:
			r.add("finding_invalid_primary_block", path+".primary_block", f.PrimaryBlock)
		}
	}
}

func validateActions(out *contract.UnifiedAnalysisOutputV11, policy *policyv2.Policy, r *Report) {
	findings := map[string]contract.UnifiedFindingV11{}
	for _, f := range out.Findings {
		findings[f.FindingID] = f
	}
	seenScopeKey := map[string]string{}
	for i, a := range out.Actions {
		path := fmt.Sprintf("actions[%d]", i)
		if a.Scope.ScopeType == "mixed" {
			r.add("action_mixed_scope", path+".scope.scope_type", "mixed forbidden")
		}
		if policy != nil {
			if _, ok := policy.ActionType(a.Type); !ok {
				r.add("action_unknown_type", path+".type", a.Type)
			}
		}
		if a.Priority == "watch" && a.ExecutionTier == "required" {
			r.add("action_watch_required", path, "watch cannot be required")
		}
		onlyHypothesis := len(a.FindingIDs) > 0
		for _, fid := range a.FindingIDs {
			f, ok := findings[fid]
			if !ok {
				onlyHypothesis = false
				continue
			}
			if f.Interpretation != "hypothesis" {
				onlyHypothesis = false
			}
		}
		if onlyHypothesis && a.ExecutionTier == "required" {
			r.add("action_hypothesis_only_required", path, "required Action cannot rely only on hypothesis")
		}
		if policy != nil {
			for _, t := range a.Completion.Conditions {
				if t.Type == "policy_threshold" && t.PolicyID != nil {
					if _, ok := policy.Threshold(*t.PolicyID); !ok {
						r.add("threshold_unknown_policy", path+".completion", *t.PolicyID)
					}
				}
			}
		}
		for _, eid := range a.Scope.EntityIDs {
			// Same type+entity is a duplicate/conflict; same campaign under any
			// advertising action type is treated as an operational conflict.
			keys := []string{a.Type + "|" + a.Scope.ScopeType + "|" + eid}
			if a.Scope.ScopeType == "campaign" && strings.HasPrefix(a.Type, "ad_") {
				keys = append(keys, "campaign_ops|"+eid)
			}
			for _, key := range keys {
				if prev, ok := seenScopeKey[key]; ok {
					r.add("action_conflict", path, "conflicts with "+prev+" on "+eid)
				} else {
					seenScopeKey[key] = a.AnalysisActionID
				}
			}
		}
	}
	// threshold policy IDs on claims
	if policy != nil {
		for i, t := range out.ThresholdClaims {
			if _, ok := policy.Threshold(t.PolicyID); !ok {
				r.add("threshold_unknown_policy", fmt.Sprintf("threshold_claims[%d].policy_id", i), t.PolicyID)
			}
		}
	}
}

func validateSectionRelevance(out *contract.UnifiedAnalysisOutputV11, r *Report) {
	findings := map[string]contract.UnifiedFindingV11{}
	for _, f := range out.Findings {
		findings[f.FindingID] = f
	}
	for _, id := range out.Dashboard.FunnelInsight.FindingIDs {
		if f, ok := findings[id]; ok && f.Domain == "stock" {
			r.add("section_domain_mismatch", "dashboard.funnel_insight.finding_ids", "stock finding in funnel")
		}
	}
	for _, id := range out.Dashboard.StockInsight.PrimaryFindingIDs {
		if f, ok := findings[id]; ok && f.Domain == "advertising" {
			r.add("section_domain_mismatch", "dashboard.stock_insight.primary_finding_ids", "advertising finding in stock")
		}
	}
	for _, id := range out.Dashboard.AdvertisingInsight.PrimaryFindingIDs {
		if f, ok := findings[id]; ok && f.Domain == "stock" {
			r.add("section_domain_mismatch", "dashboard.advertising_insight.primary_finding_ids", "stock finding in advertising")
		}
	}
}

// ValidateSchemaStrict walks the v11 schema for OpenAI strict-mode invariants.
func ValidateSchemaStrict() error {
	raw, err := contract.SchemaJSONV11()
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	visited := map[string]bool{}
	return walkStrict(doc, doc, "#", visited)
}

func walkStrict(root, node map[string]any, path string, visited map[string]bool) error {
	if ref, ok := node["$ref"].(string); ok {
		if visited[ref] {
			return nil
		}
		visited[ref] = true
		resolved, err := resolve(root, ref)
		if err != nil {
			return err
		}
		return walkStrict(root, resolved, ref, visited)
	}
	if anyOf, ok := node["anyOf"].([]any); ok {
		for i, sub := range anyOf {
			m, ok := sub.(map[string]any)
			if !ok {
				continue
			}
			if err := walkStrict(root, m, fmt.Sprintf("%s/anyOf[%d]", path, i), visited); err != nil {
				return err
			}
		}
	}
	if isObj(node) {
		if node["additionalProperties"] != false {
			return fmt.Errorf("%s: additionalProperties must be false", path)
		}
		props, _ := node["properties"].(map[string]any)
		req, _ := node["required"].([]any)
		reqSet := map[string]bool{}
		for _, r := range req {
			if s, ok := r.(string); ok {
				reqSet[s] = true
			}
		}
		for name, ps := range props {
			if !reqSet[name] {
				return fmt.Errorf("%s: property %q missing from required", path, name)
			}
			if pm, ok := ps.(map[string]any); ok {
				if err := walkStrict(root, pm, path+"/properties/"+name, visited); err != nil {
					return err
				}
			}
		}
		for name := range reqSet {
			if _, ok := props[name]; !ok {
				return fmt.Errorf("%s: required %q has no property", path, name)
			}
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		if err := walkStrict(root, items, path+"/items", visited); err != nil {
			return err
		}
	}
	return nil
}

func isObj(n map[string]any) bool {
	switch t := n["type"].(type) {
	case string:
		return t == "object"
	case []any:
		for _, v := range t {
			if s, ok := v.(string); ok && s == "object" {
				return true
			}
		}
	}
	_, has := n["properties"]
	return has
}

func resolve(root map[string]any, ref string) (map[string]any, error) {
	const prefix = "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		return nil, fmt.Errorf("unsupported ref %s", ref)
	}
	defs, _ := root["$defs"].(map[string]any)
	m, ok := defs[strings.TrimPrefix(ref, prefix)].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing ref %s", ref)
	}
	return m, nil
}
