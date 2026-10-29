package factual

import (
	"context"
	"errors"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type supplyScanner interface{ Scan(...any) error }

const supplySelect = `SELECT s.id,s.supply_order_id,s.provider_supply_id,v.version_no,v.provider_state_code,v.canonical_state,v.state_mapping_status,v.storage_location_id,v.observed_at FROM supplies s JOIN LATERAL(SELECT * FROM supply_versions x WHERE x.supply_id=s.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true`

func scanSupply(row supplyScanner) (factualapi.InboundSupply, error) {
	var out factualapi.InboundSupply
	var location pgtype.Int8
	err := row.Scan(&out.ID, &out.SupplyOrderID, &out.ProviderSupplyID, &out.VersionNo, &out.ProviderState, &out.CanonicalState, &out.StateMappingStatus, &location, &out.ObservedAt)
	if location.Valid {
		v := location.Int64
		out.StorageLocationID = &v
	}
	return out, err
}
func (s *Service) GetSupplyOrder(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.SupplyOrder, error) {
	var out factualapi.SupplyOrder
	if err := s.validate(c); err != nil {
		return out, err
	}
	var location pgtype.Int8
	var from, to pgtype.Timestamptz
	err := s.pool.QueryRow(ctx, `SELECT o.id,o.provider_order_id,v.version_no,v.provider_state_code,v.canonical_state,v.state_mapping_status,v.dropoff_location_id,v.timeslot_from,v.timeslot_to,v.observed_at FROM supply_orders o JOIN LATERAL(SELECT * FROM supply_order_versions x WHERE x.supply_order_id=o.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE o.seller_account_id=$1 AND o.id=$3 AND o.first_observed_at<=$2`, c.SellerAccountID(), c.CutoffAt(), id).Scan(&out.ID, &out.ProviderOrderID, &out.VersionNo, &out.ProviderState, &out.CanonicalState, &out.StateMappingStatus, &location, &from, &to, &out.ObservedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, notFound("supply order")
	}
	if err != nil {
		return out, dbError(err)
	}
	if location.Valid {
		v := location.Int64
		out.DropoffLocationID = &v
	}
	if from.Valid {
		out.TimeslotFrom = &from.Time
	}
	if to.Valid {
		out.TimeslotTo = &to.Time
	}
	out.Meta, err = s.meta(c, "SUPPLY.STATE", "SUPPLY_ORDER", id, nil, factHash(out))
	return out, err
}
func (s *Service) GetSupply(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.InboundSupply, error) {
	var out factualapi.InboundSupply
	if err := s.validate(c); err != nil {
		return out, err
	}
	out, err := scanSupply(s.pool.QueryRow(ctx, supplySelect+` WHERE s.seller_account_id=$1 AND s.id=$3 AND s.first_observed_at<=$2`, c.SellerAccountID(), c.CutoffAt(), id))
	if errors.Is(err, pgx.ErrNoRows) {
		return out, notFound("supply")
	}
	if err != nil {
		return out, dbError(err)
	}
	out.Meta, err = s.meta(c, "SUPPLY.STATE", "SUPPLY", id, nil, factHash(out))
	return out, err
}
func (s *Service) ListSupplies(ctx context.Context, c factualapi.FactualReadContext, filter factualapi.SupplyFilter, page factualapi.PageRequest) (factualapi.Page[factualapi.InboundSupply], error) {
	var out factualapi.Page[factualapi.InboundSupply]
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "ListSupplies", filter, page)
	if err != nil {
		return out, err
	}
	var states any = filter.CanonicalStates
	if len(filter.CanonicalStates) == 0 {
		states = nil
	}
	rows, err := s.pool.Query(ctx, supplySelect+` WHERE s.seller_account_id=$1 AND s.first_observed_at<=$2 AND s.id>$3 AND($4::text[] IS NULL OR v.canonical_state=ANY($4))ORDER BY s.id LIMIT $5`, c.SellerAccountID(), c.CutoffAt(), last, states, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		item, e := scanSupply(rows)
		if e != nil {
			return out, e
		}
		item.Meta, _ = s.meta(c, "SUPPLY.STATE", "SUPPLY", item.ID, nil, factHash(item))
		out.Items = append(out.Items, item)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, err = s.next(c, "ListSupplies", hash, out.Items[len(out.Items)-1].ID, has)
	}
	return out, err
}
func (s *Service) ListSupplyPlannedItems(ctx context.Context, c factualapi.FactualReadContext, supplyID int64, page factualapi.PageRequest) (factualapi.Page[factualapi.SupplyPlannedItem], error) {
	var out factualapi.Page[factualapi.SupplyPlannedItem]
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "ListSupplyPlannedItems", supplyID, page)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `WITH bundle AS(SELECT * FROM supply_bundle_versions WHERE seller_account_id=$1 AND supply_id=$2 AND observed_at<=$3 ORDER BY observed_at DESC,version_no DESC,id DESC LIMIT 1)SELECT i.id,b.supply_id,COALESCE(b.provider_bundle_id,''),b.version_no,i.canonical_sku_id,i.source_sku,i.source_product_id,COALESCE(i.source_offer_id,''),i.sku_resolution_status,i.declared_units,b.completeness_status,b.observed_at FROM bundle b JOIN supply_planned_items i ON i.supply_bundle_version_id=b.id WHERE i.id>$4 ORDER BY i.id LIMIT $5`, c.SellerAccountID(), supplyID, c.CutoffAt(), last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var x factualapi.SupplyPlannedItem
		var canonical, sku, product, units pgtype.Int8
		if err = rows.Scan(&x.ID, &x.SupplyID, &x.ProviderBundleID, &x.BundleVersionNo, &canonical, &sku, &product, &x.SourceOfferID, &x.ResolutionStatus, &units, &x.PlanCompleteness, &x.ObservedAt); err != nil {
			return out, err
		}
		assignIDs(&x.CanonicalSKUID, &x.SourceProductID, &x.SourceSKU, canonical, product, sku)
		x.DeclaredUnits = intValue(units, "DECLARED_UNITS_UNAVAILABLE")
		out.Items = append(out.Items, x)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, err = s.next(c, "ListSupplyPlannedItems", hash, out.Items[len(out.Items)-1].ID, has)
	}
	return out, err
}

func (s *Service) GetSupplyStateHistory(ctx context.Context, c factualapi.FactualReadContext, supplyID int64, page factualapi.PageRequest) (factualapi.Page[factualapi.SupplyStateObservation], error) {
	var out factualapi.Page[factualapi.SupplyStateObservation]
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "GetSupplyStateHistory", supplyID, page)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `SELECT o.id,o.supply_order_id,o.supply_id,o.provider_state_code,o.canonical_state,o.state_mapping_status,o.state_mapping_version,o.observed_at FROM supply_state_observations o WHERE o.seller_account_id=$1 AND o.supply_id=$2 AND o.observed_at<=$3 AND o.id>$4 ORDER BY o.id LIMIT $5`, c.SellerAccountID(), supplyID, c.CutoffAt(), last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var x factualapi.SupplyStateObservation
		var orderID, sid pgtype.Int8
		if err = rows.Scan(&x.ID, &orderID, &sid, &x.ProviderState, &x.CanonicalState, &x.StateMappingStatus, &x.StateMappingVersion, &x.ObservedAt); err != nil {
			return out, err
		}
		if orderID.Valid {
			v := orderID.Int64
			x.SupplyOrderID = &v
		}
		if sid.Valid {
			v := sid.Int64
			x.SupplyID = &v
		}
		x.Meta, _ = s.meta(c, "SUPPLY.HISTORY", "SUPPLY_STATE_OBSERVATION", x.ID, nil, factHash(x))
		out.Items = append(out.Items, x)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, err = s.next(c, "GetSupplyStateHistory", hash, out.Items[len(out.Items)-1].ID, has)
	}
	return out, err
}

func (s *Service) GetSupplyAcceptance(ctx context.Context, c factualapi.FactualReadContext, supplyID int64) (factualapi.SupplyAcceptance, error) {
	var out factualapi.SupplyAcceptance
	if err := s.validate(c); err != nil {
		return out, err
	}
	var amount, currency pgtype.Text
	err := s.pool.QueryRow(ctx, `SELECT id,supply_id,provider_act_id,COALESCE(provider_state,''),total_amount::text,currency_code,completeness_status,observed_at FROM supply_acceptance_acts WHERE seller_account_id=$1 AND supply_id=$2 AND observed_at<=$3 ORDER BY observed_at DESC,id DESC LIMIT 1`, c.SellerAccountID(), supplyID, c.CutoffAt()).Scan(&out.ID, &out.SupplyID, &out.ProviderActID, &out.ProviderState, &amount, &currency, &out.CompletenessStatus, &out.ObservedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, notFound("supply acceptance")
	}
	if err != nil {
		return out, dbError(err)
	}
	out.CurrencyCode = textValue(currency)
	out.TotalAmount = moneyValue(amount, out.CurrencyCode, "ACCEPTANCE_AMOUNT_UNAVAILABLE")
	out.Meta, err = s.meta(c, "SUPPLY.ACCEPTANCE_SUMMARY", "SUPPLY_ACCEPTANCE_ACT", out.ID, nil, factHash(out))
	if out.CompletenessStatus == "PARTIAL" {
		out.Meta.Limitations = append(out.Meta.Limitations, factualapi.Limitation{Code: "SUPPLY_ACCEPTANCE_PARTIAL", Message: "acceptance product pages were incomplete"})
	}
	return out, err
}

func (s *Service) ListSupplyAcceptanceItems(ctx context.Context, c factualapi.FactualReadContext, actID int64, page factualapi.PageRequest) (factualapi.Page[factualapi.SupplyAcceptanceItem], error) {
	var out factualapi.Page[factualapi.SupplyAcceptanceItem]
	n, err := pageSize(page, s.maxPage)
	if err != nil {
		return out, err
	}
	last, hash, err := s.cursor(c, "ListSupplyAcceptanceItems", actID, page)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `SELECT i.id,i.supply_acceptance_act_id,i.canonical_sku_id,i.source_sku,COALESCE(i.source_offer_id,''),i.sku_resolution_status,i.accepted_units,i.shortage_units,i.surplus_units,i.amount::text,i.currency_code FROM supply_acceptance_items i JOIN supply_acceptance_acts a ON a.id=i.supply_acceptance_act_id WHERE a.seller_account_id=$1 AND a.id=$2 AND a.observed_at<=$3 AND i.id>$4 ORDER BY i.id LIMIT $5`, c.SellerAccountID(), actID, c.CutoffAt(), last, n+1)
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var x factualapi.SupplyAcceptanceItem
		var canonical, sku, accepted, shortage, surplus pgtype.Int8
		var amount, currency pgtype.Text
		if err = rows.Scan(&x.ID, &x.AcceptanceActID, &canonical, &sku, &x.SourceOfferID, &x.ResolutionStatus, &accepted, &shortage, &surplus, &amount, &currency); err != nil {
			return out, err
		}
		if canonical.Valid {
			v := canonical.Int64
			x.CanonicalSKUID = &v
		}
		if sku.Valid {
			v := sku.Int64
			x.SourceSKU = &v
		}
		x.AcceptedUnits = intValue(accepted, "ACCEPTED_UNITS_UNAVAILABLE")
		x.ShortageUnits = intValue(shortage, "SHORTAGE_UNITS_UNAVAILABLE")
		x.SurplusUnits = intValue(surplus, "SURPLUS_UNITS_UNAVAILABLE")
		x.CurrencyCode = textValue(currency)
		x.Amount = moneyValue(amount, x.CurrencyCode, "ACCEPTANCE_LINE_AMOUNT_UNAVAILABLE")
		out.Items = append(out.Items, x)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, err = s.next(c, "ListSupplyAcceptanceItems", hash, out.Items[len(out.Items)-1].ID, has)
	}
	return out, err
}

func (s *Service) GetSKUInbound(ctx context.Context, c factualapi.FactualReadContext, skuID int64) (factualapi.SKUInbound, error) {
	out := factualapi.SKUInbound{CanonicalSKUID: skuID}
	if err := s.validate(c); err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `WITH current_supply AS(SELECT s.id,s.supply_order_id,v.canonical_state FROM supplies s JOIN LATERAL(SELECT * FROM supply_versions x WHERE x.supply_id=s.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE s.seller_account_id=$1 AND s.first_observed_at<=$3),order_slot AS(SELECT o.id,ov.timeslot_to FROM supply_orders o JOIN LATERAL(SELECT * FROM supply_order_versions x WHERE x.supply_order_id=o.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)ov ON true WHERE o.seller_account_id=$1 AND o.first_observed_at<=$3),latest_bundle AS(SELECT DISTINCT ON(b.supply_id)b.*,CASE WHEN os.timeslot_to IS NOT NULL AND os.timeslot_to<$3::date AND s.canonical_state NOT IN('COMPLETED','CANCELLED','REJECTED')THEN 'OVERDUE' ELSE s.canonical_state END canonical_state FROM supply_bundle_versions b JOIN current_supply s ON s.id=b.supply_id LEFT JOIN order_slot os ON os.id=s.supply_order_id WHERE b.observed_at<=$3 AND s.canonical_state NOT IN('COMPLETED','CANCELLED','REJECTED')ORDER BY b.supply_id,b.observed_at DESC,b.version_no DESC,b.id DESC)SELECT i.id,b.supply_id,COALESCE(b.provider_bundle_id,''),b.version_no,i.source_sku,i.source_product_id,COALESCE(i.source_offer_id,''),i.sku_resolution_status,i.declared_units,b.completeness_status,b.observed_at,COALESCE(b.canonical_state,'')FROM latest_bundle b JOIN supply_planned_items i ON i.supply_bundle_version_id=b.id WHERE i.canonical_sku_id=$2 ORDER BY i.id`, c.SellerAccountID(), skuID, c.CutoffAt())
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	var total int64
	complete := true
	for rows.Next() {
		var x factualapi.SupplyPlannedItem
		var sourceSKU, product, units pgtype.Int8
		var canonicalState string
		if err = rows.Scan(&x.ID, &x.SupplyID, &x.ProviderBundleID, &x.BundleVersionNo, &sourceSKU, &product, &x.SourceOfferID, &x.ResolutionStatus, &units, &x.PlanCompleteness, &x.ObservedAt, &canonicalState); err != nil {
			return out, err
		}
		x.CanonicalState = canonicalState
		x.CanonicalSKUID = &skuID
		if sourceSKU.Valid {
			v := sourceSKU.Int64
			x.SourceSKU = &v
		}
		if product.Valid {
			v := product.Int64
			x.SourceProductID = &v
		}
		x.DeclaredUnits = intValue(units, "DECLARED_UNITS_UNAVAILABLE")
		if x.PlanCompleteness != "COMPLETE" || !units.Valid {
			complete = false
		} else {
			total += units.Int64
		}
		out.Supplies = append(out.Supplies, x)
	}
	if complete {
		out.DeclaredInboundUnits = factualapi.Known(total)
	} else {
		out.DeclaredInboundUnits = factualapi.Missing[int64](factualapi.ValueUnknown, "INBOUND_PLAN_PARTIAL")
	}
	out.Meta, err = s.meta(c, "SUPPLY.BUNDLE", "SKU_INBOUND", skuID, nil, factHash(out))
	if !complete {
		out.Meta.Limitations = append(out.Meta.Limitations, factualapi.Limitation{Code: "INBOUND_PLAN_PARTIAL", Message: "partial bundle pagination prevents a closed declared inbound total"})
	}
	return out, err
}

func (s *Service) BatchGetSKUInbound(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchSKUInboundResult, error) {
	var out factualapi.BatchSKUInboundResult
	if err := s.validate(c); err != nil {
		return out, err
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$3),current_supply AS(SELECT s.id,s.supply_order_id,v.canonical_state FROM supplies s JOIN LATERAL(SELECT * FROM supply_versions x WHERE x.supply_id=s.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE s.seller_account_id=$1 AND s.first_observed_at<=$3),order_slot AS(SELECT o.id,ov.timeslot_to FROM supply_orders o JOIN LATERAL(SELECT * FROM supply_order_versions x WHERE x.supply_order_id=o.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)ov ON true WHERE o.seller_account_id=$1 AND o.first_observed_at<=$3),latest_bundle AS(SELECT DISTINCT ON(b.supply_id)b.*,CASE WHEN os.timeslot_to IS NOT NULL AND os.timeslot_to<$3::date AND s.canonical_state NOT IN('COMPLETED','CANCELLED','REJECTED')THEN 'OVERDUE' ELSE s.canonical_state END canonical_state FROM supply_bundle_versions b JOIN current_supply s ON s.id=b.supply_id LEFT JOIN order_slot os ON os.id=s.supply_order_id WHERE b.observed_at<=$3 AND s.canonical_state NOT IN('COMPLETED','CANCELLED','REJECTED')ORDER BY b.supply_id,b.observed_at DESC,b.version_no DESC,b.id DESC),items AS(SELECT i.id,i.canonical_sku_id,b.supply_id,COALESCE(b.provider_bundle_id,'')provider_bundle_id,b.version_no,i.source_sku,i.source_product_id,COALESCE(i.source_offer_id,'')source_offer_id,i.sku_resolution_status,i.declared_units,b.completeness_status,b.observed_at,COALESCE(b.canonical_state,'')canonical_state FROM latest_bundle b JOIN supply_planned_items i ON i.supply_bundle_version_id=b.id WHERE i.canonical_sku_id=ANY($2))SELECT e.id,e.found,i.id item_id,i.supply_id,i.provider_bundle_id,i.version_no,i.source_sku,i.source_product_id,i.source_offer_id,i.sku_resolution_status,i.declared_units,i.completeness_status,i.observed_at,i.canonical_state FROM eligible e LEFT JOIN items i ON i.canonical_sku_id=e.found ORDER BY e.ord,i.id`, c.SellerAccountID(), ids, c.CutoffAt())
	if err != nil {
		return out, dbError(err)
	}
	defer rows.Close()
	foundSet := map[int64]bool{}
	type inboundAgg struct {
		complete bool
		total    int64
		supplies []factualapi.SupplyPlannedItem
		seenItem map[int64]bool
	}
	byID := map[int64]*inboundAgg{}
	for rows.Next() {
		var id int64
		var found, itemID, supplyID, sourceSKU, product, units pgtype.Int8
		var bundleID, offerID, resolution, completeness, canonicalState pgtype.Text
		var versionNo pgtype.Int8
		var observedAt pgtype.Timestamptz
		if err = rows.Scan(&id, &found, &itemID, &supplyID, &bundleID, &versionNo, &sourceSKU, &product, &offerID, &resolution, &units, &completeness, &observedAt, &canonicalState); err != nil {
			return out, err
		}
		if found.Valid {
			foundSet[id] = true
		}
		if !found.Valid || !itemID.Valid {
			continue
		}
		agg, ok := byID[id]
		if !ok {
			agg = &inboundAgg{complete: true, seenItem: map[int64]bool{}}
			byID[id] = agg
		}
		if agg.seenItem[itemID.Int64] {
			continue
		}
		agg.seenItem[itemID.Int64] = true
		x := factualapi.SupplyPlannedItem{
			ID:               itemID.Int64,
			SupplyID:         supplyID.Int64,
			ProviderBundleID: bundleID.String,
			BundleVersionNo:  versionNo.Int64,
			SourceOfferID:    offerID.String,
			ResolutionStatus: resolution.String,
			PlanCompleteness: completeness.String,
			ObservedAt:       observedAt.Time,
			CanonicalState:   canonicalState.String,
		}
		sku := id
		x.CanonicalSKUID = &sku
		if sourceSKU.Valid {
			v := sourceSKU.Int64
			x.SourceSKU = &v
		}
		if product.Valid {
			v := product.Int64
			x.SourceProductID = &v
		}
		x.DeclaredUnits = intValue(units, "DECLARED_UNITS_UNAVAILABLE")
		if x.PlanCompleteness != "COMPLETE" || !units.Valid {
			agg.complete = false
		} else {
			agg.total += units.Int64
		}
		agg.supplies = append(agg.supplies, x)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	for _, id := range ids {
		if !foundSet[id] {
			out.Items = append(out.Items, factualapi.BatchSKUInboundItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		agg := byID[id]
		if agg == nil {
			agg = &inboundAgg{complete: true}
		}
		v := factualapi.SKUInbound{CanonicalSKUID: id, Supplies: agg.supplies}
		if agg.complete {
			v.DeclaredInboundUnits = factualapi.Known(agg.total)
		} else {
			v.DeclaredInboundUnits = factualapi.Missing[int64](factualapi.ValueUnknown, "INBOUND_PLAN_PARTIAL")
		}
		v.Meta, err = s.meta(c, "SUPPLY.BUNDLE", "SKU_INBOUND", id, nil, factHash(v))
		if err != nil {
			return out, err
		}
		if !agg.complete {
			v.Meta.Limitations = append(v.Meta.Limitations, factualapi.Limitation{Code: "INBOUND_PLAN_PARTIAL", Message: "partial bundle pagination prevents a closed declared inbound total"})
		}
		fact := v
		out.Items = append(out.Items, factualapi.BatchSKUInboundItem{ID: id, Status: factualapi.BatchFound, Inbound: &fact})
	}
	return out, nil
}

func (s *Service) GetSupplyAccountSummary(ctx context.Context, c factualapi.FactualReadContext) (factualapi.SupplyAccountSummary, error) {
	var out factualapi.SupplyAccountSummary
	if err := s.validate(c); err != nil {
		return out, err
	}
	var openSupplies, skuInbound, pendingActs, overdue pgtype.Int8
	err := s.pool.QueryRow(ctx, `WITH current_supply AS(SELECT s.id,s.supply_order_id,v.canonical_state FROM supplies s JOIN LATERAL(SELECT * FROM supply_versions x WHERE x.supply_id=s.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE s.seller_account_id=$1 AND s.first_observed_at<=$2),order_slot AS(SELECT o.id,ov.timeslot_to FROM supply_orders o JOIN LATERAL(SELECT * FROM supply_order_versions x WHERE x.supply_order_id=o.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)ov ON true WHERE o.seller_account_id=$1 AND o.first_observed_at<=$2),open_supply AS(SELECT cs.id,cs.supply_order_id,CASE WHEN os.timeslot_to IS NOT NULL AND os.timeslot_to<$2::date AND cs.canonical_state NOT IN('COMPLETED','CANCELLED','REJECTED')THEN true ELSE false END overdue FROM current_supply cs LEFT JOIN order_slot os ON os.id=cs.supply_order_id WHERE cs.canonical_state NOT IN('COMPLETED','CANCELLED','REJECTED')),latest_bundle AS(SELECT DISTINCT ON(b.supply_id)b.id,b.supply_id FROM supply_bundle_versions b JOIN open_supply os ON os.id=b.supply_id WHERE b.observed_at<=$2 ORDER BY b.supply_id,b.observed_at DESC,b.version_no DESC,b.id DESC),inbound_skus AS(SELECT DISTINCT i.canonical_sku_id FROM latest_bundle b JOIN supply_planned_items i ON i.supply_bundle_version_id=b.id WHERE i.canonical_sku_id IS NOT NULL),latest_act AS(SELECT DISTINCT ON(supply_id)supply_id,provider_state,completeness_status FROM supply_acceptance_acts WHERE seller_account_id=$1 AND observed_at<=$2 ORDER BY supply_id,observed_at DESC,id DESC)SELECT (SELECT count(DISTINCT supply_order_id)FROM open_supply),(SELECT count(*)FROM inbound_skus),(SELECT count(*)FROM latest_act WHERE provider_state ILIKE '%PENDING%' OR completeness_status='PARTIAL'),(SELECT count(*)FROM open_supply WHERE overdue)`, c.SellerAccountID(), c.CutoffAt()).Scan(&openSupplies, &skuInbound, &pendingActs, &overdue)
	if err != nil {
		return out, dbError(err)
	}
	out.OpenSupplyOrderCount = intValue(openSupplies, "OPEN_SUPPLY_ORDER_COUNT_UNAVAILABLE")
	out.SKUWithOpenInboundCount = intValue(skuInbound, "SKU_WITH_OPEN_INBOUND_COUNT_UNAVAILABLE")
	out.PendingAcceptanceActCount = intValue(pendingActs, "PENDING_ACCEPTANCE_ACT_COUNT_UNAVAILABLE")
	out.OverdueSupplyCount = intValue(overdue, "OVERDUE_SUPPLY_COUNT_UNAVAILABLE")
	out.Meta, err = s.meta(c, "SUPPLY.STATE", "SUPPLY_ACCOUNT_SUMMARY", c.SellerAccountID(), nil, factHash(out))
	return out, err
}
