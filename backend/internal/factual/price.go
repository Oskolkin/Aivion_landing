package factual

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const priceSelect = `SELECT p.canonical_sku_id,p.id,p.observed_at,p.snapshot_at,p.currency_code,p.provider_current_price::text,p.current_price_state,p.seller_promo_price::text,p.customer_price::text,p.old_reference_price::text,p.minimum_allowed_price::text,p.retail_price::text,p.vat_rate::text,p.auto_action_enabled,COALESCE(p.price_index_color,''),p.price_index_json,p.promotion_context_json,p.semantic_mapping_status,p.reconciliation_status FROM price_snapshots p`

type priceScanner interface{ Scan(...any) error }

func scanPrice(row priceScanner) (factualapi.SKUPrice, error) {
	var x factualapi.SKUPrice
	var current, promo, customer, old, min, retail, vat pgtype.Text
	var auto pgtype.Bool
	var index, promotion []byte
	e := row.Scan(&x.CanonicalSKUID, &x.SnapshotID, &x.ObservedAt, &x.SnapshotAt, &x.CurrencyCode, &current, &x.ProviderCurrentPrice.State, &promo, &customer, &old, &min, &retail, &vat, &auto, &x.Index.Color, &index, &promotion, &x.SemanticMappingStatus, &x.ReconciliationStatus)
	if e != nil {
		return x, e
	}
	x.ProviderCurrentPrice = moneyValue(current, x.CurrencyCode, string(x.ProviderCurrentPrice.State))
	x.SellerPromoPrice = moneyValue(promo, x.CurrencyCode, "NOT_SET")
	x.CustomerPrice = moneyValue(customer, x.CurrencyCode, "PREMIUM_ENRICHMENT_UNAVAILABLE")
	x.OldReferencePrice = moneyValue(old, x.CurrencyCode, "NOT_SET")
	x.MinimumAllowedPrice = moneyValue(min, x.CurrencyCode, "NOT_SET")
	x.RetailPrice = moneyValue(retail, x.CurrencyCode, "NOT_SET")
	if vat.Valid {
		if d, e := factualapi.ParseDecimal(vat.String); e == nil {
			x.VATRate = factualapi.Known(d)
		} else {
			x.VATRate = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "INVALID_VAT")
		}
	} else {
		x.VATRate = factualapi.Missing[factualapi.Decimal](factualapi.ValueUnknown, "VAT_UNAVAILABLE")
	}
	if auto.Valid {
		x.AutoActionEnabled = factualapi.Known(auto.Bool)
	} else {
		x.AutoActionEnabled = factualapi.Missing[bool](factualapi.ValueUnknown, "AUTO_ACTION_STATE_UNAVAILABLE")
	}
	x.Index.Details = decodeExactJSONMap(index)
	x.PromotionContext = decodeExactJSONMap(promotion)
	return x, nil
}

func decodeExactJSONMap(raw []byte) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var nested map[string]any
	if err := dec.Decode(&nested); err != nil {
		return map[string]string{"raw": string(raw)}
	}
	out := map[string]string{}
	for k, v := range nested {
		out[k] = exactJSONScalar(v)
	}
	return out
}

func exactJSONScalar(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		return strconv.FormatBool(x)
	case map[string]any, []any:
		b, _ := json.Marshal(x)
		return string(b)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

func moneyValue(v pgtype.Text, currency, reason string) factualapi.Value[factualapi.Money] {
	if !v.Valid || v.String == "" {
		state := factualapi.ValueUnknown
		if reason == "NOT_SET" {
			state = factualapi.ValueNotApplicable
		}
		return factualapi.Missing[factualapi.Money](state, reason)
	}
	d, e := factualapi.ParseDecimal(v.String)
	if e != nil {
		return factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "INVALID_PRICE_DECIMAL")
	}
	return factualapi.Known(factualapi.Money{Amount: d, CurrencyCode: currency})
}
func (s *Service) GetSKUPrice(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.SKUPrice, error) {
	return s.getSKUPriceAt(ctx, c, id, c.CutoffAt())
}
func (s *Service) getSKUPriceAt(ctx context.Context, c factualapi.FactualReadContext, id int64, at time.Time) (factualapi.SKUPrice, error) {
	var x factualapi.SKUPrice
	if e := s.validate(c); e != nil {
		return x, e
	}
	if at.After(c.CutoffAt()) {
		at = c.CutoffAt()
	}
	x, e := scanPrice(s.pool.QueryRow(ctx, priceSelect+` WHERE p.seller_account_id=$1 AND p.canonical_sku_id=$2 AND p.observed_at<=$3 ORDER BY p.observed_at DESC,p.id DESC LIMIT 1`, c.SellerAccountID(), id, at))
	if errors.Is(e, pgx.ErrNoRows) {
		return x, notFound("SKU price")
	}
	if e != nil {
		return x, dbError(e)
	}
	x.Meta, e = s.meta(c, "PRICE.CORE_STATE", "PRICE_OBSERVATION", x.SnapshotID, nil, factHash(x))
	return x, e
}
func (s *Service) GetSKUPriceAsOf(ctx context.Context, c factualapi.FactualReadContext, id int64, at time.Time) (factualapi.SKUPrice, error) {
	return s.getSKUPriceAt(ctx, c, id, at)
}
func (s *Service) BatchGetSKUPrice(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchSKUPriceResult, error) {
	var out factualapi.BatchSKUPriceResult
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	rows, e := s.pool.Query(ctx, `SELECT canonical_sku_id,id,observed_at,snapshot_at,currency_code,provider_current_price::text,current_price_state,seller_promo_price::text,customer_price::text,old_reference_price::text,minimum_allowed_price::text,retail_price::text,vat_rate::text,auto_action_enabled,COALESCE(price_index_color,''),price_index_json,promotion_context_json,semantic_mapping_status,reconciliation_status FROM(SELECT DISTINCT ON(canonical_sku_id)* FROM price_snapshots WHERE seller_account_id=$1 AND canonical_sku_id=ANY($2)AND observed_at<=$3 ORDER BY canonical_sku_id,observed_at DESC,id DESC)p`, c.SellerAccountID(), ids, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	found := map[int64]factualapi.SKUPrice{}
	for rows.Next() {
		x, se := scanPrice(rows)
		if se != nil {
			rows.Close()
			return out, dbError(se)
		}
		x.Meta, _ = s.meta(c, "PRICE.CORE_STATE", "PRICE_OBSERVATION", x.SnapshotID, nil, factHash(x))
		found[x.CanonicalSKUID] = x
	}
	rows.Close()
	for _, id := range ids {
		if x, ok := found[id]; ok {
			v := x
			out.Items = append(out.Items, factualapi.BatchSKUPriceItem{ID: id, Status: factualapi.BatchFound, Price: &v})
		} else {
			out.Items = append(out.Items, factualapi.BatchSKUPriceItem{ID: id, Status: factualapi.BatchNotFound})
		}
	}
	return out, nil
}
func (s *Service) ListSKUPrices(ctx context.Context, c factualapi.FactualReadContext, f factualapi.PriceFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.SKUPrice], error) {
	var out factualapi.Page[factualapi.SKUPrice]
	n, e := pageSize(p, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListSKUPrices", f, p)
	if e != nil {
		return out, e
	}
	var categories any = f.CategoryIDs
	if len(f.CategoryIDs) == 0 {
		categories = nil
	}
	rows, e := s.pool.Query(ctx, `SELECT canonical_sku_id,id,observed_at,snapshot_at,currency_code,provider_current_price::text,current_price_state,seller_promo_price::text,customer_price::text,old_reference_price::text,minimum_allowed_price::text,retail_price::text,vat_rate::text,auto_action_enabled,COALESCE(price_index_color,''),price_index_json,promotion_context_json,semantic_mapping_status,reconciliation_status FROM(SELECT DISTINCT ON(p.canonical_sku_id)p.* FROM price_snapshots p WHERE p.seller_account_id=$1 AND p.canonical_sku_id>$2 AND p.observed_at<=$3 AND($4='' OR p.currency_code=$4)AND($5::bigint[] IS NULL OR EXISTS(SELECT 1 FROM sku_category_assignments a WHERE a.canonical_sku_id=p.canonical_sku_id AND a.seller_category_id=ANY($5)AND a.observed_from<=$3 AND(a.observed_to IS NULL OR a.observed_to>$3)))ORDER BY p.canonical_sku_id,p.observed_at DESC,p.id DESC)p ORDER BY canonical_sku_id LIMIT $6`, c.SellerAccountID(), last, c.CutoffAt(), f.CurrencyCode, categories, n+1)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		x, se := scanPrice(rows)
		if se != nil {
			return out, dbError(se)
		}
		x.Meta, _ = s.meta(c, "PRICE.CORE_STATE", "PRICE_OBSERVATION", x.SnapshotID, nil, factHash(x))
		out.Items = append(out.Items, x)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, e = s.next(c, "ListSKUPrices", h, out.Items[len(out.Items)-1].CanonicalSKUID, has)
	}
	return out, e
}
func (s *Service) GetSKUPriceHistory(ctx context.Context, c factualapi.FactualReadContext, id int64, p factualapi.PageRequest) (factualapi.Page[factualapi.SKUPrice], error) {
	var out factualapi.Page[factualapi.SKUPrice]
	n, e := pageSize(p, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "GetSKUPriceHistory", id, p)
	if e != nil {
		return out, e
	}
	rows, e := s.pool.Query(ctx, priceSelect+` WHERE p.seller_account_id=$1 AND p.canonical_sku_id=$2 AND p.observed_at<=$3 AND p.id>$4 ORDER BY p.id LIMIT $5`, c.SellerAccountID(), id, c.CutoffAt(), last, n+1)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		x, se := scanPrice(rows)
		if se != nil {
			return out, dbError(se)
		}
		x.Meta, _ = s.meta(c, "PRICE.HISTORY", "PRICE_OBSERVATION", x.SnapshotID, nil, factHash(x))
		out.Items = append(out.Items, x)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, e = s.next(c, "GetSKUPriceHistory", h, out.Items[len(out.Items)-1].SnapshotID, has)
	}
	return out, e
}

func (s *Service) getPriceObservation(ctx context.Context, c factualapi.FactualReadContext, observationID int64) (factualapi.SKUPrice, error) {
	var x factualapi.SKUPrice
	x, e := scanPrice(s.pool.QueryRow(ctx, priceSelect+` WHERE p.seller_account_id=$1 AND p.id=$2 AND p.observed_at<=$3`, c.SellerAccountID(), observationID, c.CutoffAt()))
	if errors.Is(e, pgx.ErrNoRows) {
		return x, notFound("price observation")
	}
	if e != nil {
		return x, dbError(e)
	}
	x.Meta, e = s.meta(c, "PRICE.HISTORY", "PRICE_OBSERVATION", x.SnapshotID, nil, factHash(x))
	return x, e
}
func (s *Service) GetSKUPriceIndex(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.PriceIndex, error) {
	x, e := s.GetSKUPrice(ctx, c, id)
	return x.Index, e
}

func (s *Service) BatchGetSKUPriceLastObservedChange(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchSKUPriceLastObservedChangeResult, error) {
	var out factualapi.BatchSKUPriceLastObservedChangeResult
	if e := s.validate(c); e != nil {
		return out, e
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, e := s.pool.Query(ctx, `WITH requested AS(SELECT id,ord FROM unnest($2::bigint[])WITH ORDINALITY q(id,ord)),eligible AS(SELECT r.id,r.ord,cs.id found FROM requested r LEFT JOIN canonical_skus cs ON cs.id=r.id AND cs.seller_account_id=$1 AND cs.first_observed_at<=$3),ranked AS(SELECT canonical_sku_id,id,observed_at,currency_code,provider_current_price::text,ROW_NUMBER()OVER(PARTITION BY canonical_sku_id ORDER BY observed_at DESC,id DESC)rn FROM price_snapshots WHERE seller_account_id=$1 AND canonical_sku_id=ANY($2)AND observed_at<=$3),pairs AS(SELECT r1.canonical_sku_id,r1.id current_id,r1.observed_at,r1.currency_code,r1.provider_current_price current_price,r2.provider_current_price previous_price FROM ranked r1 LEFT JOIN ranked r2 ON r2.canonical_sku_id=r1.canonical_sku_id AND r2.rn=2 WHERE r1.rn=1)SELECT e.id,e.found,p.canonical_sku_id,p.current_id,p.observed_at,p.currency_code,p.current_price,p.previous_price FROM eligible e LEFT JOIN pairs p ON p.canonical_sku_id=e.found ORDER BY e.ord`, c.SellerAccountID(), ids, c.CutoffAt())
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	foundSet := map[int64]bool{}
	changes := map[int64]*factualapi.SKUPriceLastObservedChange{}
	for rows.Next() {
		var id int64
		var found, skuID, currentID pgtype.Int8
		var observedAt pgtype.Timestamptz
		var currency, currentPrice, previousPrice pgtype.Text
		if e = rows.Scan(&id, &found, &skuID, &currentID, &observedAt, &currency, &currentPrice, &previousPrice); e != nil {
			return out, dbError(e)
		}
		if found.Valid {
			foundSet[id] = true
		}
		if !found.Valid || !skuID.Valid || !currentID.Valid {
			continue
		}
		cur := moneyValue(currentPrice, currency.String, "CURRENT_PRICE_UNAVAILABLE")
		prev := moneyValue(previousPrice, currency.String, "PREVIOUS_PRICE_UNAVAILABLE")
		if cur.State != factualapi.ValueKnown || cur.Value == nil {
			continue
		}
		if prev.State != factualapi.ValueKnown || prev.Value == nil {
			continue
		}
		if cur.Value.Amount.String() == prev.Value.Amount.String() {
			continue
		}
		ch := factualapi.SKUPriceLastObservedChange{
			CanonicalSKUID: skuID.Int64,
			CurrentPrice:   cur,
			PreviousPrice:  prev,
			ObservedAt:     observedAt.Time,
		}
		ch.Meta, e = s.meta(c, "PRICE.HISTORY", "PRICE_OBSERVATION", currentID.Int64, nil, factHash(ch))
		if e != nil {
			return out, e
		}
		changes[id] = &ch
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	for _, id := range ids {
		if !foundSet[id] {
			out.Items = append(out.Items, factualapi.BatchSKUPriceLastObservedChangeItem{ID: id, Status: factualapi.BatchNotFound})
			continue
		}
		if ch, ok := changes[id]; ok {
			v := *ch
			out.Items = append(out.Items, factualapi.BatchSKUPriceLastObservedChangeItem{ID: id, Status: factualapi.BatchFound, Change: &v})
		} else {
			out.Items = append(out.Items, factualapi.BatchSKUPriceLastObservedChangeItem{ID: id, Status: factualapi.BatchNotFound})
		}
	}
	return out, nil
}
