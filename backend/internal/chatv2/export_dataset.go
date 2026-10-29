package chatv2

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type DatasetResolver struct {
	API factualapi.API
}

type ResolvedDataset struct {
	Rows          [][]string
	PeriodFrom    time.Time
	PeriodTo      time.Time
	CanonicalSpec ExportDatasetRef
}

var exportColumns = []string{
	"entity_kind", "entity_id", "entity_label", "metric", "value", "state", "reason_code",
}

var supportedExportKinds = map[string]struct{}{
	"":            {},
	"SALES":       {},
	"SKU":         {},
	"SKU_METRICS": {},
}

var supportedExportPeriods = map[string]factualapi.StandardPeriod{
	"":             factualapi.PeriodCurrent7D,
	"last_7d":      factualapi.PeriodCurrent7D,
	"current_7d":   factualapi.PeriodCurrent7D,
	"last_30d":     factualapi.PeriodTrend30D,
	"trend_30d":    factualapi.PeriodTrend30D,
	"analysis_day": factualapi.PeriodAnalysisDay,
	"previous_day": factualapi.PeriodPreviousDay,
}

var supportedExportFilterFields = map[string]struct{}{
	"sku_id":           {},
	"canonical_sku_id": {},
}

func (r *DatasetResolver) Resolve(ctx context.Context, frozen FrozenTurnContext, spec *ExportDatasetRef) (ResolvedDataset, error) {
	if r == nil || r.API == nil {
		return ResolvedDataset{}, ErrExportFailed
	}
	if spec == nil {
		return ResolvedDataset{}, ErrExportFailed
	}
	if !frozen.HasFactual {
		return ResolvedDataset{}, ErrExportFailed
	}
	kind := strings.ToUpper(strings.TrimSpace(spec.Kind))
	if _, ok := supportedExportKinds[kind]; !ok {
		return ResolvedDataset{}, fmt.Errorf("%w: kind %q", ErrUnsupportedExportDataset, spec.Kind)
	}
	std, err := exportPeriod(spec.Period)
	if err != nil {
		return ResolvedDataset{}, err
	}
	resolved, err := factualapi.ResolvePeriod(frozen.Factual, std, nil)
	if err != nil {
		return ResolvedDataset{}, err
	}
	skuIDs, err := exportSKUScope(frozen, spec)
	if err != nil {
		return ResolvedDataset{}, err
	}
	rows, err := r.resolveSKUMetrics(ctx, frozen, skuIDs, std, nil)
	if err != nil {
		return ResolvedDataset{}, err
	}
	canon := *spec
	if canon.Kind == "" {
		canon.Kind = "SKU_METRICS"
	}
	if canon.Period == "" {
		canon.Period = "last_7d"
	}
	return ResolvedDataset{
		Rows: rows, PeriodFrom: resolved.From, PeriodTo: resolved.To, CanonicalSpec: canon,
	}, nil
}

func exportPeriod(period string) (factualapi.StandardPeriod, error) {
	p := strings.ToLower(strings.TrimSpace(period))
	std, ok := supportedExportPeriods[p]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedExportPeriod, period)
	}
	return std, nil
}

func exportSKUScope(frozen FrozenTurnContext, spec *ExportDatasetRef) ([]int64, error) {
	seen := map[int64]struct{}{}
	var ids []int64
	add := func(id int64) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, f := range spec.Filters {
		field := strings.ToLower(strings.TrimSpace(f.Field))
		if _, ok := supportedExportFilterFields[field]; !ok {
			return nil, fmt.Errorf("%w: filter field %q", ErrUnsupportedExportDataset, f.Field)
		}
		if id, err := strconv.ParseInt(strings.TrimSpace(f.Value), 10, 64); err == nil {
			add(id)
		}
	}
	for _, e := range frozen.ReferencedEntities {
		if strings.EqualFold(e.Kind, "SKU") {
			if id, err := strconv.ParseInt(strings.TrimSpace(e.ID), 10, 64); err == nil {
				add(id)
			}
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("export requires sku scope")
	}
	if len(ids) > 50 {
		ids = ids[:50]
	}
	return ids, nil
}

func (r *DatasetResolver) resolveSKUMetrics(ctx context.Context, frozen FrozenTurnContext, skuIDs []int64, std factualapi.StandardPeriod, explicit *factualapi.Period) ([][]string, error) {
	demand, err := r.API.BatchGetSKUDemand(ctx, frozen.Factual, skuIDs, std, explicit)
	if err != nil {
		return nil, fmt.Errorf("BatchGetSKUDemand: %w", err)
	}
	stock, err := r.API.BatchGetSKUStock(ctx, frozen.Factual, skuIDs)
	if err != nil {
		return nil, fmt.Errorf("BatchGetSKUStock: %w", err)
	}
	catalog, err := r.API.BatchGetSKUs(ctx, frozen.Factual, skuIDs)
	if err != nil {
		return nil, fmt.Errorf("BatchGetSKUs: %w", err)
	}
	labels := map[int64]string{}
	for _, it := range catalog.Items {
		if it.SKU != nil {
			labels[it.ID] = fmt.Sprintf("%s (%d)", it.SKU.Name, it.SKU.OzonSKU)
		}
	}
	stockByID := map[int64]factualapi.SKUStock{}
	for _, it := range stock.Items {
		if it.Stock != nil {
			stockByID[it.ID] = *it.Stock
		}
	}
	var rows [][]string
	for _, it := range demand.Items {
		label := labels[it.ID]
		if label == "" {
			label = strconv.FormatInt(it.ID, 10)
		}
		if it.Demand == nil {
			rows = append(rows, exportRow("SKU", it.ID, label, "ordered_units", "", string(factualapi.ValueUnknown), "demand_unavailable"))
			rows = append(rows, exportRow("SKU", it.ID, label, "ordered_amount", "", string(factualapi.ValueUnknown), "demand_unavailable"))
		} else {
			v, st, reason := valueExport(it.Demand.Demand.OrderedUnits)
			rows = append(rows, exportRow("SKU", it.ID, label, "ordered_units", v, st, reason))
			v, st, reason = valueExportMoney(it.Demand.Demand.OrderedAmount)
			rows = append(rows, exportRow("SKU", it.ID, label, "ordered_amount", v, st, reason))
		}
		if st, ok := stockByID[it.ID]; ok {
			v, state, reason := valueExport(st.SellableUnits)
			rows = append(rows, exportRow("SKU", it.ID, label, "sellable_stock", v, state, reason))
		} else {
			rows = append(rows, exportRow("SKU", it.ID, label, "sellable_stock", "", string(factualapi.ValueUnknown), "stock_unavailable"))
		}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("export produced no factual rows")
	}
	return rows, nil
}

func exportRow(kind string, id int64, label, metric, value, state, reason string) []string {
	return []string{kind, strconv.FormatInt(id, 10), label, metric, value, state, reason}
}

func valueExport(v factualapi.Value[int64]) (value, state, reason string) {
	state = string(v.State)
	reason = v.ReasonCode
	if v.State == factualapi.ValueKnown && v.Value != nil {
		value = strconv.FormatInt(*v.Value, 10)
	}
	return value, state, reason
}

func valueExportMoney(v factualapi.Value[factualapi.Money]) (value, state, reason string) {
	state = string(v.State)
	reason = v.ReasonCode
	if v.State == factualapi.ValueKnown && v.Value != nil {
		value = v.Value.Amount.String()
	}
	return value, state, reason
}
