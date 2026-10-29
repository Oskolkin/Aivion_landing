package evidence

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidencecatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func parseID(e EntityRef) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(e.ID), 10, 64)
	if err != nil || id <= 0 {
		return 0, Integrity("invalid entity id " + e.ID)
	}
	return id, nil
}

func marshalData(v any) (json.RawMessage, error) {
	if v == nil {
		return json.RawMessage("{}"), nil
	}
	b, err := canonicaljson.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func knownInt(v factualapi.Value[int64]) *int64 {
	if v.State != factualapi.ValueKnown || v.Value == nil {
		return nil
	}
	x := *v.Value
	return &x
}

func knownMoney(v factualapi.Value[factualapi.Money]) *string {
	if v.State != factualapi.ValueKnown || v.Value == nil {
		return nil
	}
	s := v.Value.Amount.String()
	return &s
}

func failResult(req NormalizedEvidenceRequest, code, msg, readiness string) MaterializationResult {
	if readiness == "" {
		readiness = "FAILED"
	}
	return MaterializationResult{
		Request: req, Status: StatusFailed, Readiness: readiness,
		Limitations: []Limitation{{Code: code, Message: msg}},
		Data:        map[string]string{"error": code},
	}
}

func emptyResult(req NormalizedEvidenceRequest, temporal TemporalScope, note string) MaterializationResult {
	return MaterializationResult{
		Request: req, Status: StatusEmptyValid, Readiness: "READY", Temporal: temporal,
		Coverage: Coverage{Notes: []string{note}}, Data: map[string]any{"observations": []any{}},
	}
}

type FlowHistoryDTO struct {
	Grain             string          `json:"grain"`
	Daily             []FlowDailyDTO  `json:"daily_30d"`
	WeeklyContext     []FlowWeeklyDTO `json:"weekly_context"`
	Current7          *FlowTotalsDTO  `json:"current7,omitempty"`
	Previous7         *FlowTotalsDTO  `json:"previous7,omitempty"`
	Totals30D         *FlowTotalsDTO  `json:"totals_30d,omitempty"`
	NotCanonicalSales bool            `json:"attributed_sales_are_not_canonical_demand,omitempty"`
}

type FlowDailyDTO struct {
	BusinessDate  string  `json:"business_date"`
	OrderedUnits  *int64  `json:"ordered_units,omitempty"`
	OrderedAmount *string `json:"ordered_amount,omitempty"`
	CurrencyCode  string  `json:"currency_code,omitempty"`
	FactRef       string  `json:"fact_ref,omitempty"`
}

type FlowWeeklyDTO struct {
	WeekStart      string   `json:"week_start"`
	WeekEnd        string   `json:"week_end"`
	OrderedUnits   *int64   `json:"ordered_units,omitempty"`
	OrderedAmount  *string  `json:"ordered_amount,omitempty"`
	CurrencyCode   string   `json:"currency_code,omitempty"`
	InputFactRefs  []string `json:"input_fact_refs"`
	FormulaKey     string   `json:"formula_key"`
	FormulaVersion string   `json:"formula_version"`
}

type FlowTotalsDTO struct {
	OrderedUnits  *int64   `json:"ordered_units,omitempty"`
	OrderedAmount *string  `json:"ordered_amount,omitempty"`
	CurrencyCode  string   `json:"currency_code,omitempty"`
	InputFactRefs []string `json:"input_fact_refs"`
	FormulaKey    string   `json:"formula_key"`
}

func compressDemand(points []factualapi.DemandPoint, analysisDate time.Time) (FlowHistoryDTO, []DeclaredFact, string) {
	dailyCut := analysisDate.AddDate(0, 0, -29)
	dto := FlowHistoryDTO{Grain: "observed_business_date", Daily: []FlowDailyDTO{}, WeeklyContext: []FlowWeeklyDTO{}}
	facts := []DeclaredFact{}
	readiness := "READY"
	type weekKey struct{ start, end time.Time }
	weeks := map[string]*FlowWeeklyDTO{}
	var c7, p7, t30 FlowTotalsDTO
	c7.FormulaKey, p7.FormulaKey, t30.FormulaKey = FormulaTotalsV1, FormulaTotalsV1, FormulaTotalsV1
	prevFrom, prevTo := analysisDate.AddDate(0, 0, -13), analysisDate.AddDate(0, 0, -7)
	curFrom := analysisDate.AddDate(0, 0, -6)
	for _, pt := range points {
		d := dateOnly(pt.BusinessDate)
		if pt.OrderedAmount.State != factualapi.ValueKnown || pt.OrderedUnits.State != factualapi.ValueKnown {
			if pt.Meta.Readiness == "PARTIAL" || pt.Meta.Readiness == "STALE" {
				readiness = worseReady(readiness, pt.Meta.Readiness)
			}
			continue
		}
		day := FlowDailyDTO{
			BusinessDate: d.Format("2006-01-02"), OrderedUnits: knownInt(pt.OrderedUnits),
			OrderedAmount: knownMoney(pt.OrderedAmount), FactRef: pt.Meta.FactRef,
		}
		if pt.OrderedAmount.Value != nil {
			day.CurrencyCode = pt.OrderedAmount.Value.CurrencyCode
		}
		if pt.Meta.FactRef != "" {
			facts = append(facts, DeclaredFact{SemanticPath: "sales.demand.day." + day.BusinessDate, FactRefs: []string{pt.Meta.FactRef}})
		}
		if !d.Before(dailyCut) {
			dto.Daily = append(dto.Daily, day)
		} else {
			ws := weekStart(d)
			we := ws.AddDate(0, 0, 6)
			k := ws.Format("2006-01-02")
			w := weeks[k]
			if w == nil {
				w = &FlowWeeklyDTO{WeekStart: k, WeekEnd: we.Format("2006-01-02"), FormulaKey: FormulaWeeklySumV1, FormulaVersion: FormulaWeeklySumV1, InputFactRefs: []string{}}
				weeks[k] = w
			}
			addFlow(w, pt)
			if pt.Meta.FactRef != "" {
				w.InputFactRefs = append(w.InputFactRefs, pt.Meta.FactRef)
			}
		}
		if !d.Before(curFrom) && !d.After(analysisDate) {
			addTotals(&c7, pt)
		}
		if !d.Before(prevFrom) && !d.After(prevTo) {
			addTotals(&p7, pt)
		}
		if !d.Before(dailyCut) && !d.After(analysisDate) {
			addTotals(&t30, pt)
		}
		if pt.Meta.Readiness == "PARTIAL" || pt.Meta.Readiness == "STALE" {
			readiness = worseReady(readiness, pt.Meta.Readiness)
		}
	}
	keys := make([]string, 0, len(weeks))
	for k := range weeks {
		keys = append(keys, k)
	}
	sortStrings(keys)
	for _, k := range keys {
		w := *weeks[k]
		dto.WeeklyContext = append(dto.WeeklyContext, w)
		facts = append(facts, DeclaredFact{
			SemanticPath: "sales.demand.week." + w.WeekStart,
			Derivation:   &discovery.Derivation{FormulaKey: FormulaWeeklySumV1, FormulaVersion: FormulaWeeklySumV1, InputFactRefs: w.InputFactRefs},
		})
	}
	if c7.OrderedUnits != nil || c7.OrderedAmount != nil {
		dto.Current7 = &c7
	}
	if p7.OrderedUnits != nil || p7.OrderedAmount != nil {
		dto.Previous7 = &p7
	}
	if t30.OrderedUnits != nil || t30.OrderedAmount != nil {
		dto.Totals30D = &t30
	}
	return dto, facts, readiness
}

func addFlow(w *FlowWeeklyDTO, pt factualapi.DemandPoint) {
	if pt.OrderedUnits.Value != nil {
		v := int64(0)
		if w.OrderedUnits != nil {
			v = *w.OrderedUnits
		}
		v += *pt.OrderedUnits.Value
		w.OrderedUnits = &v
	}
	if pt.OrderedAmount.Value != nil {
		if w.OrderedAmount == nil {
			s := pt.OrderedAmount.Value.Amount.String()
			w.OrderedAmount = &s
			w.CurrencyCode = pt.OrderedAmount.Value.CurrencyCode
		}
	}
}

func addTotals(t *FlowTotalsDTO, pt factualapi.DemandPoint) {
	if pt.OrderedUnits.Value != nil {
		v := int64(0)
		if t.OrderedUnits != nil {
			v = *t.OrderedUnits
		}
		v += *pt.OrderedUnits.Value
		t.OrderedUnits = &v
	}
	if pt.Meta.FactRef != "" {
		t.InputFactRefs = append(t.InputFactRefs, pt.Meta.FactRef)
	}
	if pt.OrderedAmount.Value != nil && t.OrderedAmount == nil {
		s := pt.OrderedAmount.Value.Amount.String()
		t.OrderedAmount = &s
		t.CurrencyCode = pt.OrderedAmount.Value.CurrencyCode
	}
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func weekStart(d time.Time) time.Time {
	wd := int(d.Weekday())
	if wd == 0 {
		wd = 7
	}
	return d.AddDate(0, 0, -(wd - 1))
}

func worseReady(a, b string) string {
	rank := func(s string) int {
		switch s {
		case "READY":
			return 0
		case "STALE":
			return 1
		case "PARTIAL":
			return 2
		default:
			return 3
		}
	}
	if rank(b) > rank(a) {
		return b
	}
	return a
}

func sortStrings(in []string) {
	for i := 0; i < len(in); i++ {
		for j := i + 1; j < len(in); j++ {
			if in[j] < in[i] {
				in[i], in[j] = in[j], in[i]
			}
		}
	}
}

func temporalFor(t evidencecatalog.Type, rc factualapi.FactualReadContext) TemporalScope {
	sc := TemporalScope{ProfileKey: t.TemporalProfileKey, To: rc.AnalysisDate().Format("2006-01-02")}
	switch t.TemporalProfileKey {
	case evidencecatalog.TemporalFlowHistory30D90D:
		sc.From = rc.AnalysisDate().AddDate(0, 0, -89).Format("2006-01-02")
	case evidencecatalog.TemporalEventHistory30D:
		sc.From = rc.AnalysisDate().AddDate(0, 0, -29).Format("2006-01-02")
	case evidencecatalog.TemporalCurrentSnapshot:
		sc.From = cutoffRFC(rc)
		sc.To = cutoffRFC(rc)
	}
	return sc
}

func boundList[T any](items []T, max int, sortBasis, scope string, total *int) ([]T, TruncationMeta) {
	tr := TruncationMeta{Returned: len(items), SortBasis: sortBasis, SemanticScope: scope, TotalKnown: total}
	if max > 0 && len(items) > max {
		tr.Truncated = true
		tr.Returned = max
		if total == nil {
			n := len(items)
			tr.TotalKnown = &n
		}
		return items[:max], tr
	}
	tr.Returned = len(items)
	return items, tr
}

func pageTrunc[T any](p factualapi.Page[T], returned int, sortBasis, scope string) TruncationMeta {
	tr := TruncationMeta{Returned: returned, SortBasis: sortBasis, SemanticScope: scope, Truncated: p.NextPageToken != ""}
	return tr
}
