package factual

import (
	"context"
	"errors"
	"math/big"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) stockSnapshotForContext(ctx context.Context, c factualapi.FactualReadContext) (int64, error) {
	binding, ok := c.Binding("STOCK", "STOCK.SNAPSHOT")
	if !ok {
		return 0, nil
	}
	var id int64
	e := s.pool.QueryRow(ctx, `SELECT id FROM stock_snapshots WHERE seller_account_id=$1 AND collection_domain_run_id=$2 AND snapshot_at<=$3 ORDER BY snapshot_at DESC,id DESC LIMIT 1`, c.SellerAccountID(), binding.DomainRunID, c.CutoffAt()).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return 0, nil
	}
	return id, e
}

func (s *Service) fetchStocks(ctx context.Context, c factualapi.FactualReadContext, snapshot int64, ids []int64) (map[int64]factualapi.SKUStock, error) {
	out := map[int64]factualapi.SKUStock{}
	if snapshot == 0 || len(ids) == 0 {
		return out, nil
	}
	rows, e := s.pool.Query(ctx, `SELECT DISTINCT l.canonical_sku_id,ss.id,ss.snapshot_at,ss.time_quality FROM stock_snapshot_lines l JOIN stock_snapshots ss ON ss.id=l.snapshot_id WHERE ss.seller_account_id=$1 AND ss.id=$2 AND l.canonical_sku_id=ANY($3) ORDER BY l.canonical_sku_id`, c.SellerAccountID(), snapshot, ids)
	if e != nil {
		return nil, dbError(e)
	}
	for rows.Next() {
		var x factualapi.SKUStock
		if e = rows.Scan(&x.CanonicalSKUID, &x.SnapshotID, &x.SnapshotAt, &x.TimeQuality); e != nil {
			rows.Close()
			return nil, dbError(e)
		}
		out[x.CanonicalSKUID] = x
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return nil, e
	}
	rows.Close()
	lines, e := s.pool.Query(ctx, `SELECT l.id,l.canonical_sku_id,l.inventory_pool_id,l.inventory_location_id,l.fulfillment_type,p.independence_status,COALESCE(v.provider_name,''),l.sellable_units,l.reserved_units,l.non_sellable_units,l.provider_valid_stock_count,COALESCE(t.provider_ads::text,''),COALESCE(t.provider_idc::text,''),COALESCE(t.provider_turnover_grade,''),COALESCE(l.absence_reason,'')FROM stock_snapshot_lines l JOIN inventory_pools p ON p.id=l.inventory_pool_id LEFT JOIN LATERAL(SELECT provider_name FROM inventory_location_versions x WHERE x.inventory_location_id=l.inventory_location_id AND x.observed_at<=$4 ORDER BY x.observed_at DESC,x.id DESC LIMIT 1)v ON true LEFT JOIN stock_turnover_observations t ON t.stock_snapshot_line_id=l.id WHERE l.seller_account_id=$1 AND l.snapshot_id=$2 AND l.canonical_sku_id=ANY($3)ORDER BY l.canonical_sku_id,l.id`, c.SellerAccountID(), snapshot, ids, c.CutoffAt())
	if e != nil {
		return nil, dbError(e)
	}
	defer lines.Close()
	known := map[int64]bool{}
	ambiguous := map[int64]bool{}
	absent := map[int64]string{}
	for lines.Next() {
		var p factualapi.StockPosition
		var sku int64
		var location pgtype.Int8
		var sellable, reserved, nonSellable, valid pgtype.Int8
		var ads, idc string
		if e = lines.Scan(&p.LineID, &sku, &p.InventoryPoolID, &location, &p.FulfillmentType, &p.PoolIndependenceStatus, &p.LocationName, &sellable, &reserved, &nonSellable, &valid, &ads, &idc, &p.ProviderTurnoverGrade, &p.AbsenceReason); e != nil {
			return nil, dbError(e)
		}
		if location.Valid {
			p.InventoryLocationID = &location.Int64
		}
		if p.AbsenceReason != "" {
			p.SellableUnits = factualapi.Missing[int64](factualapi.ValueUnknown, p.AbsenceReason)
			p.ReservedUnits = factualapi.Missing[int64](factualapi.ValueUnknown, p.AbsenceReason)
			p.NonSellableUnits = factualapi.Missing[int64](factualapi.ValueUnknown, p.AbsenceReason)
			p.ProviderValidStockCount = factualapi.Missing[int64](factualapi.ValueUnknown, p.AbsenceReason)
			p.ProviderADS = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, p.AbsenceReason)
			p.ProviderIDC = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, p.AbsenceReason)
			x := out[sku]
			x.Positions = append(x.Positions, p)
			absent[sku] = p.AbsenceReason
			out[sku] = x
			continue
		}
		p.SellableUnits = intValue(sellable, "SELLABLE_NOT_MAPPED")
		p.ReservedUnits = intValue(reserved, "PROVIDER_FIELD_UNAVAILABLE")
		p.NonSellableUnits = intValue(nonSellable, "PROVIDER_FIELD_UNAVAILABLE")
		p.ProviderValidStockCount = intValue(valid, "PROVIDER_FIELD_UNAVAILABLE")
		p.ProviderADS = decimalValue(ads, "PROVIDER_TURNOVER_UNAVAILABLE")
		p.ProviderIDC = decimalValue(idc, "PROVIDER_TURNOVER_UNAVAILABLE")
		x := out[sku]
		x.Positions = append(x.Positions, p)
		if p.PoolIndependenceStatus == "VERIFIED_INDEPENDENT" && sellable.Valid {
			x.KnownSubtotalUnits += sellable.Int64
			known[sku] = true
		} else if p.PoolIndependenceStatus != "VERIFIED_INDEPENDENT" {
			ambiguous[sku] = true
		}
		out[sku] = x
	}
	if e = lines.Err(); e != nil {
		return nil, e
	}
	for id, x := range out {
		var limitations []factualapi.Limitation
		switch {
		case known[id]:
			x.SellableUnits = factualapi.Known(x.KnownSubtotalUnits)
		case absent[id] != "" && !known[id] && !ambiguous[id]:
			x.SellableUnits = factualapi.Missing[int64](factualapi.ValueUnknown, absent[id])
			limitations = append(limitations, factualapi.Limitation{Code: absent[id], Message: "requested SKU quantity is unknown due to provider absence observation"})
		case ambiguous[id]:
			x.SellableUnits = factualapi.Missing[int64](factualapi.ValueUnknown, "INVENTORY_POOL_OVERLAP_UNVERIFIED")
		default:
			x.SellableUnits = factualapi.Missing[int64](factualapi.ValueUnknown, "SELLABLE_MAPPING_PENDING_ACCEPTANCE")
		}
		if ambiguous[id] {
			limitations = append(limitations, factualapi.Limitation{Code: "INVENTORY_POOL_OVERLAP_UNVERIFIED", Message: "only proven independent inventory pools are included in the known subtotal"})
		}
		x.Meta.Limitations = limitations
		out[id] = x
	}
	return out, nil
}
func intValue(v pgtype.Int8, reason string) factualapi.Value[int64] {
	if v.Valid {
		return factualapi.Known(v.Int64)
	}
	return factualapi.Missing[int64](factualapi.ValueUnknown, reason)
}
func decimalValue(v, reason string) factualapi.Value[factualapi.Decimal] {
	if v == "" {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, reason)
	}
	d, e := factualapi.ParseDecimal(v)
	if e != nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "INVALID_PROVIDER_DECIMAL")
	}
	return factualapi.Known(d)
}

func (s *Service) GetSKUStock(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.SKUStock, error) {
	var out factualapi.SKUStock
	if e := s.validate(c); e != nil {
		return out, e
	}
	snapshot, e := s.stockSnapshotForContext(ctx, c)
	if e != nil {
		return out, dbError(e)
	}
	m, e := s.fetchStocks(ctx, c, snapshot, []int64{id})
	if e != nil {
		return out, e
	}
	x, ok := m[id]
	if !ok {
		return out, notFound("SKU stock")
	}
	limitations := x.Meta.Limitations
	x.Meta, e = s.metaWithAux(c, "STOCK.SKU_CURRENT", "SKU_STOCK", id, x.SnapshotID, nil, factHash(x))
	x.Meta.Limitations = append(x.Meta.Limitations, limitations...)
	return x, e
}
func (s *Service) BatchGetSKUStock(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchSKUStockResult, error) {
	var out factualapi.BatchSKUStockResult
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	snapshot, e := s.stockSnapshotForContext(ctx, c)
	if e != nil {
		return out, dbError(e)
	}
	m, e := s.fetchStocks(ctx, c, snapshot, ids)
	if e != nil {
		return out, e
	}
	for _, id := range ids {
		if x, ok := m[id]; ok {
			limitations := x.Meta.Limitations
			x.Meta, _ = s.metaWithAux(c, "STOCK.SKU_CURRENT", "SKU_STOCK", id, x.SnapshotID, nil, factHash(x))
			x.Meta.Limitations = append(x.Meta.Limitations, limitations...)
			v := x
			out.Items = append(out.Items, factualapi.BatchSKUStockItem{ID: id, Status: factualapi.BatchFound, Stock: &v})
		} else {
			out.Items = append(out.Items, factualapi.BatchSKUStockItem{ID: id, Status: factualapi.BatchNotFound})
		}
	}
	return out, nil
}
func (s *Service) ListSKUStock(ctx context.Context, c factualapi.FactualReadContext, f factualapi.StockFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.SKUStock], error) {
	var out factualapi.Page[factualapi.SKUStock]
	n, e := pageSize(p, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListSKUStock", f, p)
	if e != nil {
		return out, e
	}
	snapshot, e := s.stockSnapshotForContext(ctx, c)
	if e != nil {
		return out, dbError(e)
	}
	var categories any = f.CategoryIDs
	if len(f.CategoryIDs) == 0 {
		categories = nil
	}
	rows, e := s.pool.Query(ctx, `SELECT DISTINCT l.canonical_sku_id FROM stock_snapshot_lines l WHERE l.seller_account_id=$1 AND l.snapshot_id=$2 AND l.canonical_sku_id>$3 AND($4::bigint[] IS NULL OR EXISTS(SELECT 1 FROM sku_category_assignments a WHERE a.canonical_sku_id=l.canonical_sku_id AND a.seller_category_id=ANY($4)AND a.observed_from<=$5 AND(a.observed_to IS NULL OR a.observed_to>$5)))ORDER BY l.canonical_sku_id LIMIT $6`, c.SellerAccountID(), snapshot, last, categories, c.CutoffAt(), n+1)
	if e != nil {
		return out, dbError(e)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return out, e
		}
		ids = append(ids, id)
	}
	rows.Close()
	has := len(ids) > n
	if has {
		ids = ids[:n]
	}
	m, e := s.fetchStocks(ctx, c, snapshot, ids)
	if e != nil {
		return out, e
	}
	for _, id := range ids {
		x := m[id]
		limitations := x.Meta.Limitations
		x.Meta, _ = s.metaWithAux(c, "STOCK.SKU_CURRENT", "SKU_STOCK", id, x.SnapshotID, nil, factHash(x))
		x.Meta.Limitations = append(x.Meta.Limitations, limitations...)
		out.Items = append(out.Items, x)
	}
	if len(ids) > 0 {
		out.NextPageToken, e = s.next(c, "ListSKUStock", h, ids[len(ids)-1], has)
	}
	return out, e
}
func (s *Service) GetStockLocationBreakdown(ctx context.Context, c factualapi.FactualReadContext, id int64) ([]factualapi.StockPosition, error) {
	x, e := s.GetSKUStock(ctx, c, id)
	return x.Positions, e
}
func (s *Service) GetStockSnapshot(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.StockSnapshot, error) {
	var x factualapi.StockSnapshot
	if e := s.validate(c); e != nil {
		return x, e
	}
	e := s.pool.QueryRow(ctx, `SELECT id,snapshot_at,collection_started_at,collection_completed_at,time_quality,requested_sku_universe_hash,completeness_status,expected_sku_count,received_sku_count,position_count,batches_expected,batches_completed,batches_failed FROM stock_snapshots WHERE seller_account_id=$1 AND id=$2 AND snapshot_at<=$3`, c.SellerAccountID(), id, c.CutoffAt()).Scan(&x.ID, &x.SnapshotAt, &x.CollectionStartedAt, &x.CollectionCompletedAt, &x.TimeQuality, &x.UniverseHash, &x.CompletenessStatus, &x.ExpectedSKUCount, &x.ReceivedSKUCount, &x.PositionCount, &x.BatchesExpected, &x.BatchesCompleted, &x.BatchesFailed)
	if errors.Is(e, pgx.ErrNoRows) {
		return x, notFound("stock snapshot")
	}
	if e != nil {
		return x, dbError(e)
	}
	x.Meta, e = s.meta(c, "STOCK.SKU_CURRENT", "STOCK_SNAPSHOT", x.ID, nil, factHash(x))
	return x, e
}
func (s *Service) ListStockSnapshots(ctx context.Context, c factualapi.FactualReadContext, p factualapi.PageRequest) (factualapi.Page[factualapi.StockSnapshot], error) {
	var out factualapi.Page[factualapi.StockSnapshot]
	n, e := pageSize(p, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListStockSnapshots", struct{}{}, p)
	if e != nil {
		return out, e
	}
	rows, e := s.pool.Query(ctx, `SELECT id,snapshot_at,collection_started_at,collection_completed_at,time_quality,requested_sku_universe_hash,completeness_status,expected_sku_count,received_sku_count,position_count,batches_expected,batches_completed,batches_failed FROM stock_snapshots WHERE seller_account_id=$1 AND snapshot_at<=$2 AND id>$3 ORDER BY id LIMIT $4`, c.SellerAccountID(), c.CutoffAt(), last, n+1)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var x factualapi.StockSnapshot
		if e = rows.Scan(&x.ID, &x.SnapshotAt, &x.CollectionStartedAt, &x.CollectionCompletedAt, &x.TimeQuality, &x.UniverseHash, &x.CompletenessStatus, &x.ExpectedSKUCount, &x.ReceivedSKUCount, &x.PositionCount, &x.BatchesExpected, &x.BatchesCompleted, &x.BatchesFailed); e != nil {
			return out, e
		}
		x.Meta, _ = s.meta(c, "STOCK.SKU_CURRENT", "STOCK_SNAPSHOT", x.ID, nil, factHash(x))
		out.Items = append(out.Items, x)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, e = s.next(c, "ListStockSnapshots", h, out.Items[len(out.Items)-1].ID, has)
	}
	return out, e
}
func (s *Service) GetSKUStockHistory(ctx context.Context, c factualapi.FactualReadContext, id int64, p factualapi.PageRequest) (factualapi.Page[factualapi.SKUStock], error) {
	var out factualapi.Page[factualapi.SKUStock]
	n, e := pageSize(p, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "GetSKUStockHistory", id, p)
	if e != nil {
		return out, e
	}
	rows, e := s.pool.Query(ctx, `SELECT DISTINCT snapshot_id FROM stock_snapshot_lines l JOIN stock_snapshots s ON s.id=l.snapshot_id WHERE l.seller_account_id=$1 AND l.canonical_sku_id=$2 AND s.snapshot_at<=$3 AND snapshot_id>$4 ORDER BY snapshot_id LIMIT $5`, c.SellerAccountID(), id, c.CutoffAt(), last, n+1)
	if e != nil {
		return out, dbError(e)
	}
	var snapshots []int64
	for rows.Next() {
		var x int64
		if e = rows.Scan(&x); e != nil {
			rows.Close()
			return out, e
		}
		snapshots = append(snapshots, x)
	}
	rows.Close()
	has := len(snapshots) > n
	if has {
		snapshots = snapshots[:n]
	}
	for _, snapshot := range snapshots {
		m, e := s.fetchStocks(ctx, c, snapshot, []int64{id})
		if e != nil {
			return out, e
		}
		x := m[id]
		x.Meta, _ = s.metaWithAux(c, "STOCK.SKU_CURRENT", "SKU_STOCK", id, x.SnapshotID, nil, factHash(x))
		out.Items = append(out.Items, x)
	}
	if len(snapshots) > 0 {
		out.NextPageToken, e = s.next(c, "GetSKUStockHistory", h, snapshots[len(snapshots)-1], has)
	}
	return out, e
}

func (s *Service) GetStockCoverage(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.StockCoverage, error) {
	var out factualapi.StockCoverage
	stock, e := s.GetSKUStock(ctx, c, id)
	if e != nil {
		return out, e
	}
	out.CanonicalSKUID = id
	out.Stock = stock
	out.FormulaPolicy = "stock_coverage_formula_v1"
	p7 := factualapi.Period{From: c.AnalysisDate().AddDate(0, 0, -6), To: c.AnalysisDate()}
	p30 := factualapi.Period{From: c.AnalysisDate().AddDate(0, 0, -29), To: c.AnalysisDate()}
	d7, e := s.GetSKUDemand(ctx, c, id, "", &p7)
	if e != nil {
		return out, e
	}
	d30, e := s.GetSKUDemand(ctx, c, id, "", &p30)
	if e != nil {
		return out, e
	}
	out.Demand7DFactRef = d7.Demand.Meta.FactRef
	out.Demand30DFactRef = d30.Demand.Meta.FactRef
	out.CoverageDays7D = coverageValue(stock.SellableUnits, d7.Demand.OrderedUnits, 7)
	out.CoverageDays30D = coverageValue(stock.SellableUnits, d30.Demand.OrderedUnits, 30)
	out.Meta, e = s.metaWithAux(c, "STOCK.COVERAGE_7D", "STOCK_COVERAGE", id, stock.SnapshotID, nil, factHash(out))
	return out, e
}

func (s *Service) BatchGetStockCoverage(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchStockCoverageResult, error) {
	var out factualapi.BatchStockCoverageResult
	if e := s.validate(c); e != nil {
		return out, e
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	if len(ids) == 0 {
		return out, nil
	}
	stocks, e := s.BatchGetSKUStock(ctx, c, ids)
	if e != nil {
		return out, e
	}
	p7 := factualapi.Period{From: c.AnalysisDate().AddDate(0, 0, -6), To: c.AnalysisDate()}
	p30 := factualapi.Period{From: c.AnalysisDate().AddDate(0, 0, -29), To: c.AnalysisDate()}
	d7, e := s.BatchGetSKUDemand(ctx, c, ids, "", &p7)
	if e != nil {
		return out, e
	}
	d30, e := s.BatchGetSKUDemand(ctx, c, ids, "", &p30)
	if e != nil {
		return out, e
	}
	demand7 := map[int64]factualapi.BatchSKUDemandItem{}
	for _, item := range d7.Items {
		demand7[item.ID] = item
	}
	demand30 := map[int64]factualapi.BatchSKUDemandItem{}
	for _, item := range d30.Items {
		demand30[item.ID] = item
	}
	for _, item := range stocks.Items {
		if item.Status == factualapi.BatchNotFound || item.Stock == nil {
			out.Items = append(out.Items, factualapi.BatchStockCoverageItem{ID: item.ID, Status: factualapi.BatchNotFound})
			continue
		}
		stock := *item.Stock
		v := factualapi.StockCoverage{
			CanonicalSKUID: item.ID,
			Stock:          stock,
			FormulaPolicy:  "stock_coverage_formula_v1",
		}
		status := factualapi.BatchFound
		if d, ok := demand7[item.ID]; ok && d.Demand != nil {
			v.Demand7DFactRef = d.Demand.Demand.Meta.FactRef
			v.CoverageDays7D = coverageValue(stock.SellableUnits, d.Demand.Demand.OrderedUnits, 7)
			if d.Status == factualapi.BatchLimited {
				status = factualapi.BatchLimited
			}
		} else {
			v.CoverageDays7D = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "DEMAND_UNAVAILABLE")
			status = factualapi.BatchLimited
		}
		if d, ok := demand30[item.ID]; ok && d.Demand != nil {
			v.Demand30DFactRef = d.Demand.Demand.Meta.FactRef
			v.CoverageDays30D = coverageValue(stock.SellableUnits, d.Demand.Demand.OrderedUnits, 30)
			if d.Status == factualapi.BatchLimited {
				status = factualapi.BatchLimited
			}
		} else {
			v.CoverageDays30D = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "DEMAND_UNAVAILABLE")
			status = factualapi.BatchLimited
		}
		v.Meta, e = s.metaWithAux(c, "STOCK.COVERAGE_7D", "STOCK_COVERAGE", item.ID, stock.SnapshotID, nil, factHash(v))
		if e != nil {
			return out, e
		}
		fact := v
		out.Items = append(out.Items, factualapi.BatchStockCoverageItem{ID: item.ID, Status: status, Coverage: &fact})
	}
	return out, nil
}

func coverageValue(stock factualapi.Value[int64], demand factualapi.Value[int64], days int64) factualapi.Value[factualapi.Decimal] {
	if stock.State != factualapi.ValueKnown || stock.Value == nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "STOCK_UNAVAILABLE")
	}
	if demand.State != factualapi.ValueKnown || demand.Value == nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "DEMAND_UNAVAILABLE")
	}
	if *demand.Value == 0 {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueNoDenominator, "NO_DEMAND")
	}
	stockDays := new(big.Int).Mul(big.NewInt(*stock.Value), big.NewInt(days))
	num := new(big.Rat).SetInt(stockDays)
	den := new(big.Rat).SetInt64(*demand.Value)
	num.Quo(num, den)
	v, e := factualapi.ParseDecimal(num.FloatString(6))
	if e != nil {
		return factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "COVERAGE_ARITHMETIC_FAILED")
	}
	return factualapi.Known(v)
}
