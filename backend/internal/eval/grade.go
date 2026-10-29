package eval

import (
	"fmt"
	"strings"
)

type GradeResult struct {
	Pass         bool
	CriticalFail bool
	Code         string
	Detail       string
	Metrics      map[string]float64
}

func GradeFactual(c Case, actual map[string]any) GradeResult {
	exp := c.Expected
	if exp.Sales != nil {
		if str(actual["ordered_amount"]) != exp.Sales.OrderedAmount || str(actual["ordered_units"]) != exp.Sales.OrderedUnits {
			return GradeResult{CriticalFail: true, Code: "SALES_MISMATCH", Detail: fmt.Sprintf("got amount=%s units=%s", str(actual["ordered_amount"]), str(actual["ordered_units"]))}
		}
	}
	if exp.DRR != nil {
		if str(actual["account_drr_pct"]) != exp.DRR.AccountDRRPct {
			return GradeResult{CriticalFail: true, Code: "DRR_MISMATCH", Detail: str(actual["account_drr_pct"])}
		}
		if boolVal(actual["used_avg_daily_drr"]) {
			return GradeResult{CriticalFail: true, Code: "DRR_AVG_DAILY_FORBIDDEN"}
		}
		if boolVal(actual["used_attributed_ordered_amount_denom"]) {
			return GradeResult{CriticalFail: true, Code: "DRR_ATTRIBUTED_DENOM_FORBIDDEN"}
		}
	}
	if exp.Stock != nil {
		if exp.Stock.StockZeroDemandPositiveCoverageZero && str(actual["coverage_oos_demand"]) != "0" {
			return GradeResult{CriticalFail: true, Code: "STOCK_COVERAGE_OOS"}
		}
		if exp.Stock.NoDemandCoverageNull && str(actual["coverage_no_demand"]) != "NULL" && str(actual["coverage_no_demand"]) != "NO_DEMAND" {
			return GradeResult{CriticalFail: true, Code: "STOCK_NO_DEMAND"}
		}
		if exp.Stock.StockUnavailableCoverageNull && str(actual["coverage_stock_unavailable"]) != "NULL" {
			return GradeResult{CriticalFail: true, Code: "STOCK_UNAVAILABLE"}
		}
		if exp.Stock.InboundDoesNotIncreaseStock && boolVal(actual["inbound_increased_stock"]) {
			return GradeResult{CriticalFail: true, Code: "INBOUND_AFFECTED_STOCK"}
		}
	}
	if exp.Funnel != nil {
		if exp.Funnel.MissingStageNotInvented && boolVal(actual["missing_stage_invented"]) {
			return GradeResult{CriticalFail: true, Code: "FUNNEL_INVENTED_STAGE"}
		}
		if exp.Funnel.IncompatibleChain != "" && str(actual["incompatible_chain"]) != exp.Funnel.IncompatibleChain {
			return GradeResult{CriticalFail: true, Code: "FUNNEL_INCOMPATIBLE"}
		}
		if exp.Funnel.PremiumUnavailable != "" && str(actual["premium_unavailable"]) != exp.Funnel.PremiumUnavailable {
			return GradeResult{CriticalFail: true, Code: "FUNNEL_PREMIUM"}
		}
		if exp.Funnel.PartialChain != "" && str(actual["partial_chain"]) != exp.Funnel.PartialChain {
			return GradeResult{CriticalFail: true, Code: "FUNNEL_PARTIAL"}
		}
		if exp.Funnel.ValidZeroActivity != "" && str(actual["valid_zero_activity"]) != exp.Funnel.ValidZeroActivity {
			return GradeResult{CriticalFail: true, Code: "FUNNEL_EMPTY"}
		}
		if exp.Funnel.ConversionDeltaPP != "" && str(actual["conversion_delta_pp"]) != exp.Funnel.ConversionDeltaPP {
			return GradeResult{CriticalFail: true, Code: "FUNNEL_DELTA"}
		}
	}
	if has(actual, "unavailable_as_zero") && boolVal(actual["unavailable_as_zero"]) {
		return GradeResult{CriticalFail: true, Code: "UNAVAILABLE_AS_ZERO"}
	}
	if has(actual, "seller_isolation_ok") && !boolVal(actual["seller_isolation_ok"]) {
		return GradeResult{CriticalFail: true, Code: "SELLER_ISOLATION"}
	}
	return GradeResult{Pass: true, Code: "PASS"}
}

func GradeMateriality(labels []MaterialLabel, selected map[string]bool) GradeResult {
	var mustHit, mustTot, hit, tot float64
	for _, l := range labels {
		key := l.Kind + ":" + l.Key
		switch l.Label {
		case LabelMustInclude:
			mustTot++
			tot++
			if selected[key] {
				mustHit++
				hit++
			}
		case LabelShouldInclude:
			tot++
			if selected[key] {
				hit++
			}
		}
	}
	m := map[string]float64{}
	if mustTot > 0 {
		m["must_include_recall"] = mustHit / mustTot
	} else {
		m["must_include_recall"] = 1
	}
	if tot > 0 {
		m["overall_recall"] = hit / tot
	} else {
		m["overall_recall"] = 1
	}
	crit := m["must_include_recall"] < 1
	return GradeResult{Pass: !crit && m["overall_recall"] >= 0.95, CriticalFail: crit, Code: "MATERIALITY", Metrics: m}
}

func GradeDiscovery(exp *DiscoveryExpectation, found []string, duplicates int, unsupportedCritical int) GradeResult {
	if exp == nil {
		return GradeResult{Pass: true, Code: "PASS"}
	}
	if exp.AllowNoMeaningful && len(found) == 0 {
		return GradeResult{Pass: true, Code: "NO_MEANINGFUL_THREADS", Metrics: map[string]float64{"mandatory_recall": 1}}
	}
	var critHit, critTot, manHit, manTot float64
	foundSet := map[string]bool{}
	for _, f := range found {
		foundSet[f] = true
	}
	for _, th := range exp.Threads {
		if th.Class == ThreadMandatory {
			manTot++
			if strings.HasPrefix(th.ID, "critical:") {
				critTot++
			}
			if foundSet[th.ID] || foundSet[strings.TrimPrefix(th.ID, "critical:")] {
				manHit++
				if strings.HasPrefix(th.ID, "critical:") {
					critHit++
				}
			}
		}
		if th.Class == ThreadForbidden && foundSet[th.ID] {
			return GradeResult{CriticalFail: true, Code: "FORBIDDEN_THREAD", Detail: th.ID}
		}
	}
	dupRate := 0.0
	if len(found) > 0 {
		dupRate = float64(duplicates) / float64(len(found))
	}
	m := map[string]float64{"mandatory_recall": 1, "critical_recall": 1, "duplicate_rate": dupRate}
	if manTot > 0 {
		m["mandatory_recall"] = manHit / manTot
	}
	if critTot > 0 {
		m["critical_recall"] = critHit / critTot
	}
	crit := (critTot > 0 && m["critical_recall"] < 1) || unsupportedCritical > 0 || dupRate > 0.05 || m["mandatory_recall"] < 0.90
	return GradeResult{Pass: !crit, CriticalFail: (critTot > 0 && m["critical_recall"] < 1) || unsupportedCritical > 0, Code: "DISCOVERY", Metrics: m}
}

func GradeDeepTraps(traps []string, output string) GradeResult {
	low := strings.ToLower(output)
	forbidden := map[string][]string{
		"ordered_amount_not_revenue":                          {"выручк", "revenue"},
		"finance_without_cogs_forbid_profit":                  {"прибыл", "profit", "маржинальн"},
		"stock_times_price_not_frozen_capital":                {"замороженн", "frozen capital"},
		"search_top_n_forbid_all_queries":                     {"все поисков", "all search queries"},
		"promotion_candidate_not_participant":                 {"участник акци", "promotion participant"},
		"price_change_and_stock_exhausted_no_price_causality": {"из-за цены", "because of price"},
	}
	for _, trap := range traps {
		for _, needle := range forbidden[trap] {
			if needle != "" && strings.Contains(low, needle) {
				return GradeResult{CriticalFail: true, Code: "CRITICAL_SEMANTIC", Detail: trap + ":" + needle}
			}
		}
	}
	return GradeResult{Pass: true, Code: "PASS"}
}

func GradeActions(warranted int, published int, harmful int) GradeResult {
	rate := 1.0
	if published > 0 {
		rate = float64(warranted) / float64(published)
	}
	return GradeResult{
		Pass:         harmful == 0 && rate >= 0.90,
		CriticalFail: harmful > 0,
		Code:         "ACTIONS",
		Metrics:      map[string]float64{"acceptability": rate, "harmful": float64(harmful)},
	}
}

func GradeTrajectory(spec *TrajectorySpec, tools []string) GradeResult {
	if spec == nil {
		return GradeResult{Pass: true, Code: "PASS"}
	}
	have := map[string]bool{}
	for _, t := range tools {
		have[t] = true
	}
	for _, f := range spec.Forbidden {
		if have[f] {
			return GradeResult{CriticalFail: true, Code: "FORBIDDEN_TOOL", Detail: f}
		}
	}
	ok := true
	for _, r := range spec.Required {
		if !have[r] {
			ok = false
			break
		}
	}
	if !ok && len(spec.Equivalent) > 0 {
		for _, alt := range spec.Equivalent {
			match := true
			for _, r := range alt {
				if !have[r] {
					match = false
					break
				}
			}
			if match {
				ok = true
				break
			}
		}
	}
	if !ok {
		return GradeResult{CriticalFail: true, Code: "REQUIRED_TOOL_MISSING"}
	}
	return GradeResult{Pass: true, Code: "PASS"}
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func boolVal(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func has(m map[string]any, k string) bool {
	_, ok := m[k]
	return ok
}
