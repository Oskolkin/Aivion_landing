package aidatapack

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

func numericToFloat(n pgtype.Numeric) float64 {
	if v := numericToFloatPtr(n); v != nil {
		return *v
	}
	return 0
}

func numericToFloatPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return nil
	}
	if math.IsNaN(v.Float64) || math.IsInf(v.Float64, 0) {
		return nil
	}
	value := round2(v.Float64)
	return &value
}

func int8ToString(v pgtype.Int8) string {
	if !v.Valid {
		return ""
	}
	return strconv.FormatInt(v.Int64, 10)
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

func textValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func pctDelta(current, previous float64) (abs float64, pct *float64) {
	abs = round2(current - previous)
	if previous > 0 {
		p := round2((current - previous) / previous * 100)
		pct = &p
	}
	return abs, pct
}

func safeRatioPct(numerator, denominator float64) *float64 {
	if denominator <= 0 {
		return nil
	}
	v := round2(numerator / denominator * 100)
	return &v
}

func safeRatio(numerator, denominator float64) *float64 {
	if denominator <= 0 {
		return nil
	}
	v := round4(numerator / denominator)
	return &v
}

func nullableFloat(v *float64) any {
	if v == nil {
		return nil
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil
	}
	return *v
}

func sumAccountMetrics(rows []accountMetricDay) accountMetricDay {
	var out accountMetricDay
	for _, r := range rows {
		out.Revenue += r.Revenue
		out.Orders += r.Orders
		out.UnitsSold += r.UnitsSold
		out.Returns += r.Returns
		out.Cancels += r.Cancels
	}
	if out.Orders > 0 {
		aov := round2(out.Revenue / float64(out.Orders))
		out.AOV = &aov
	}
	return out
}

type accountMetricDay struct {
	Revenue   float64
	Orders    int64
	UnitsSold int64
	Returns   int64
	Cancels   int64
	AOV       *float64
}

type skuMetricDay struct {
	Revenue   float64
	Orders    int64
	UnitsSold int64
	Stock     *int32
	DaysCover *float64
}

func aggregateAdMetrics(rows []adMetricDay) adMetricAgg {
	var out adMetricAgg
	for _, r := range rows {
		out.Impressions += r.Impressions
		out.Clicks += r.Clicks
		out.Spend += r.Spend
		out.Orders += r.Orders
		out.Revenue += r.Revenue
	}
	out.CTRPct = safeRatioPct(float64(out.Clicks), float64(out.Impressions))
	out.CPC = safeRatio(out.Spend, float64(out.Clicks))
	out.CPO = safeRatio(out.Spend, float64(out.Orders))
	out.ROAS = safeRatio(out.Revenue, out.Spend)
	out.DRRPct = safeRatioPct(out.Spend, out.Revenue)
	if out.Spend > 0 && out.Revenue == 0 {
		out.SpendWithoutRevenue = true
	}
	if out.Spend > 0 && out.Orders == 0 {
		out.SpendWithoutOrders = true
	}
	return out
}

type adMetricDay struct {
	Impressions int64
	Clicks      int64
	Spend       float64
	Orders      int64
	Revenue     float64
}

type adMetricAgg struct {
	Impressions         int64
	Clicks              int64
	Spend               float64
	Orders              int64
	Revenue             float64
	CTRPct              *float64
	CPC                 *float64
	CPO                 *float64
	ROAS                *float64
	DRRPct              *float64
	SpendWithoutOrders  bool
	SpendWithoutRevenue bool
}

func adAggToWindow(a adMetricAgg) CampaignMetricsWindow {
	return CampaignMetricsWindow{
		Impressions:         a.Impressions,
		Clicks:              a.Clicks,
		Spend:               round2(a.Spend),
		Orders:              a.Orders,
		Revenue:             round2(a.Revenue),
		CTRPct:              a.CTRPct,
		CPC:                 a.CPC,
		CPO:                 a.CPO,
		ROAS:                a.ROAS,
		DRRPct:              a.DRRPct,
		SpendWithoutOrders:  a.SpendWithoutOrders,
		SpendWithoutRevenue: a.SpendWithoutRevenue,
	}
}

// turnoverGradePriority — lower index = more critical.
var turnoverGradePriority = []string{
	"DEFICIT",
	"WAITING_FOR_SUPPLY",
	"NO_SALES",
	"RESTRICTED_NO_SALES",
	"SURPLUS",
	"POPULAR",
	"ACTUAL",
	"COLLECTING_DATA",
	"TURNOVER_GRADE_NONE",
	"UNSPECIFIED",
}

func mostCriticalTurnoverGrade(grades []string) string {
	best := ""
	bestIdx := len(turnoverGradePriority)
	for _, g := range grades {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		idx := len(turnoverGradePriority)
		for i, candidate := range turnoverGradePriority {
			if candidate == g {
				idx = i
				break
			}
		}
		if idx < bestIdx {
			bestIdx = idx
			best = g
		}
	}
	return best
}

func skuEntityID(sku string) string {
	return "sku:" + sku
}

func campaignEntityID(externalID string) string {
	return "campaign:" + externalID
}

func categoryEntityID(id string) string {
	return "category:" + id
}

func warehouseEntityID(id string) string {
	return "warehouse:" + id
}

func skuGroupEntityID(productID string) string {
	return "sku_group:" + productID
}

func normalizeFactID(raw string) (string, error) {
	id := strings.TrimSpace(strings.ToLower(raw))
	if id == "" {
		return "", fmt.Errorf("fact_id is empty")
	}
	if strings.Contains(id, ":") {
		return "", fmt.Errorf("fact_id must not contain ':'")
	}
	for _, r := range id {
		if r >= 'а' && r <= 'я' || r >= 'А' && r <= 'Я' || r == 'ё' || r == 'Ё' {
			return "", fmt.Errorf("fact_id must not contain cyrillic characters")
		}
		if r == ' ' {
			return "", fmt.Errorf("fact_id must not contain spaces")
		}
	}
	return id, nil
}
