package factual

import (
	"context"
	"errors"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) GetOrder(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.Order, error) {
	var x factualapi.Order
	if e := s.validate(c); e != nil {
		return x, e
	}
	e := s.pool.QueryRow(ctx, `SELECT o.id,o.ozon_order_id,COALESCE(v.provider_order_number,''),o.first_observed_at FROM sales_orders o JOIN LATERAL(SELECT provider_order_number FROM sales_posting_versions x WHERE x.canonical_order_id=o.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE o.seller_account_id=$1 AND o.id=$2 AND o.first_observed_at<=$3`, c.SellerAccountID(), id, c.CutoffAt()).Scan(&x.ID, &x.OzonOrderID, &x.OrderNumber, &x.FirstObservedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return x, notFound("order")
	}
	if e != nil {
		return x, dbError(e)
	}
	x.Meta, e = s.meta(c, "LIFECYCLE.ORDERS", "ORDER", x.ID, nil, factHash(x))
	return x, e
}
func (s *Service) GetPosting(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.Posting, error) {
	var x factualapi.Posting
	if e := s.validate(c); e != nil {
		return x, e
	}
	var order pgtype.Int8
	e := s.pool.QueryRow(ctx, `SELECT p.id,p.posting_number,v.fulfillment_scheme,v.order_resolution_status,v.canonical_order_id,v.version_no,v.provider_status,COALESCE(v.provider_substatus,''),v.observed_at,v.source_received_at FROM sales_postings p JOIN LATERAL(SELECT * FROM sales_posting_versions x WHERE x.canonical_posting_id=p.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE p.seller_account_id=$1 AND p.id=$2 AND p.first_observed_at<=$3`, c.SellerAccountID(), id, c.CutoffAt()).Scan(&x.ID, &x.PostingNumber, &x.FulfillmentScheme, &x.OrderResolutionStatus, &order, &x.VersionNo, &x.ProviderStatus, &x.ProviderSubstatus, &x.ObservedAt, &x.SourceReceivedAt)
	if order.Valid {
		x.CanonicalOrderID = &order.Int64
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return x, notFound("posting")
	}
	if e != nil {
		return x, dbError(e)
	}
	rows, e := s.pool.Query(ctx, `SELECT pp.canonical_sku_id,COALESCE(pp.source_sku,0),pp.sku_resolution_status,COALESCE(pp.offer_id,''),COALESCE(pp.product_name,''),pp.quantity,pp.unit_price::text,pp.currency_code FROM sales_posting_products pp JOIN sales_posting_versions v ON v.id=pp.posting_version_id WHERE v.canonical_posting_id=$1 AND v.version_no=$2 ORDER BY pp.line_no`, x.ID, x.VersionNo)
	if e != nil {
		return x, dbError(e)
	}
	for rows.Next() {
		var p factualapi.PostingProduct
		var sku pgtype.Int8
		var amount string
		if e = rows.Scan(&sku, &p.SourceSKU, &p.ResolutionStatus, &p.OfferID, &p.Name, &p.Quantity, &amount, &p.UnitPrice.CurrencyCode); e != nil {
			rows.Close()
			return x, dbError(e)
		}
		if sku.Valid {
			p.CanonicalSKUID = &sku.Int64
		}
		p.UnitPrice.Amount, _ = factualapi.ParseDecimal(amount)
		x.Products = append(x.Products, p)
	}
	rows.Close()
	rels, e := s.pool.Query(ctx, `SELECT rv.parent_posting_id,r.parent_posting_number,rv.resolution_status FROM sales_posting_relations r JOIN LATERAL(SELECT parent_posting_id,resolution_status FROM sales_posting_relation_versions x WHERE x.posting_relation_id=r.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.revision_no DESC,x.id DESC LIMIT 1)rv ON true WHERE r.seller_account_id=$1 AND r.child_posting_id=$2 AND r.first_observed_at<=$3 ORDER BY r.id`, c.SellerAccountID(), x.ID, c.CutoffAt())
	if e != nil {
		return x, dbError(e)
	}
	for rels.Next() {
		var r factualapi.SplitRelation
		var parent pgtype.Int8
		if e = rels.Scan(&parent, &r.ParentPostingNumber, &r.ResolutionStatus); e != nil {
			rels.Close()
			return x, dbError(e)
		}
		if parent.Valid {
			r.ParentPostingID = &parent.Int64
		}
		x.Splits = append(x.Splits, r)
	}
	rels.Close()
	x.Meta, e = s.meta(c, "LIFECYCLE.POSTINGS", "POSTING", x.ID, nil, factHash(x))
	return x, e
}
func (s *Service) ListPostings(ctx context.Context, c factualapi.FactualReadContext, f factualapi.PostingFilter, req factualapi.PageRequest) (factualapi.Page[factualapi.Posting], error) {
	var out factualapi.Page[factualapi.Posting]
	if e := s.validate(c); e != nil {
		return out, e
	}
	n, e := pageSize(req, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListPostings", f, req)
	if e != nil {
		return out, e
	}
	rows, e := s.pool.Query(ctx, `SELECT p.id FROM sales_postings p JOIN LATERAL(SELECT id,provider_status,canonical_order_id,fulfillment_scheme,observed_at FROM sales_posting_versions x WHERE x.canonical_posting_id=p.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE p.seller_account_id=$1 AND p.first_observed_at<=$2 AND p.id>$3 AND($4::bigint IS NULL OR v.canonical_order_id=$4)AND($5='' OR v.fulfillment_scheme=$5)AND($6='' OR v.provider_status=$6)AND($7::bigint IS NULL OR EXISTS(SELECT 1 FROM sales_posting_products pp WHERE pp.posting_version_id=v.id AND pp.canonical_sku_id=$7))ORDER BY p.id LIMIT $8`, c.SellerAccountID(), c.CutoffAt(), last, f.OrderID, f.FulfillmentScheme, f.ProviderStatus, f.SKUID, n+1)
	if e != nil {
		return out, dbError(e)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return out, dbError(e)
		}
		ids = append(ids, id)
	}
	rows.Close()
	has := len(ids) > n
	if has {
		ids = ids[:n]
	}
	out.Items, e = s.batchPostings(ctx, c, ids)
	if e != nil {
		return out, e
	}
	if len(ids) > 0 {
		out.NextPageToken, e = s.next(c, "ListPostings", h, ids[len(ids)-1], has)
	}
	return out, e
}
func (s *Service) ListLifecycleEvents(ctx context.Context, c factualapi.FactualReadContext, f factualapi.LifecycleEventFilter, req factualapi.PageRequest) (factualapi.Page[factualapi.LifecycleEvent], error) {
	var out factualapi.Page[factualapi.LifecycleEvent]
	if e := s.validate(c); e != nil {
		return out, e
	}
	n, e := pageSize(req, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListLifecycleEvents", f, req)
	if e != nil {
		return out, e
	}
	var types, qualities any = f.EventTypes, f.TimeQualities
	if len(f.EventTypes) == 0 {
		types = nil
	}
	if len(f.TimeQualities) == 0 {
		qualities = nil
	}
	var from, to any
	if f.BusinessPeriod != nil {
		from, to = f.BusinessPeriod.From, f.BusinessPeriod.To
	}
	rows, e := s.pool.Query(ctx, `SELECT id,event_type,time_quality,canonical_order_id,canonical_posting_id,canonical_sku_id,canonical_return_id,effective_at,provider_date,business_date,effective_window_from_at,effective_window_to_at,observed_at FROM sales_lifecycle_events WHERE seller_account_id=$1 AND observed_at<=$2 AND id>$3 AND($4::text[] IS NULL OR event_type=ANY($4))AND($5::text[] IS NULL OR time_quality=ANY($5))AND($6::bigint IS NULL OR canonical_order_id=$6)AND($7::bigint IS NULL OR canonical_posting_id=$7)AND($8::bigint IS NULL OR canonical_sku_id=$8)AND($9::date IS NULL OR business_date BETWEEN $9 AND $10)ORDER BY id LIMIT $11`, c.SellerAccountID(), c.CutoffAt(), last, types, qualities, f.OrderID, f.PostingID, f.SKUID, from, to, n+1)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var x factualapi.LifecycleEvent
		var order, posting, sku, ret pgtype.Int8
		var effective, wf, wt pgtype.Timestamptz
		var pd, bd pgtype.Date
		if e = rows.Scan(&x.ID, &x.EventType, &x.TimeQuality, &order, &posting, &sku, &ret, &effective, &pd, &bd, &wf, &wt, &x.ObservedAt); e != nil {
			return out, dbError(e)
		}
		if order.Valid {
			x.CanonicalOrderID = &order.Int64
		}
		if posting.Valid {
			x.CanonicalPostingID = &posting.Int64
		}
		if sku.Valid {
			x.CanonicalSKUID = &sku.Int64
		}
		if ret.Valid {
			x.CanonicalReturnID = &ret.Int64
		}
		if effective.Valid {
			x.EffectiveAt = &effective.Time
		}
		if pd.Valid {
			x.ProviderDate = &pd.Time
		}
		if bd.Valid {
			x.BusinessDate = &bd.Time
		}
		if wf.Valid {
			x.WindowFrom = &wf.Time
		}
		if wt.Valid {
			x.WindowTo = &wt.Time
		}
		x.Meta, _ = s.meta(c, "LIFECYCLE.EVENTS", "LIFECYCLE_EVENT", x.ID, nil, factHash(x))
		out.Items = append(out.Items, x)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, e = s.next(c, "ListLifecycleEvents", h, out.Items[len(out.Items)-1].ID, has)
	}
	return out, e
}
func (s *Service) GetReturn(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.Return, error) {
	var x factualapi.Return
	if e := s.validate(c); e != nil {
		return x, e
	}
	var order, posting, sku pgtype.Int8
	var rd, vc pgtype.Timestamptz
	var amount string
	e := s.pool.QueryRow(ctx, `SELECT r.id,r.provider_return_id,e.version_no,e.canonical_order_id,e.canonical_posting_id,e.canonical_sku_id,COALESCE(e.provider_type,''),COALESCE(e.provider_schema,''),COALESCE(e.return_reason_name,''),COALESCE(e.provider_visual_status,''),COALESCE(e.provider_visual_status_sys_name,''),e.quantity,e.unit_price::text,e.currency_code,e.provider_return_date,e.provider_visual_change_at,e.observed_at,e.source_received_at FROM sales_returns r JOIN LATERAL(SELECT * FROM sales_return_events x WHERE x.sales_return_id=r.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)e ON true WHERE r.seller_account_id=$1 AND r.id=$2 AND r.first_observed_at<=$3`, c.SellerAccountID(), id, c.CutoffAt()).Scan(&x.ID, &x.ProviderReturnID, &x.VersionNo, &order, &posting, &sku, &x.ProviderType, &x.ProviderSchema, &x.Reason, &x.VisualStatus, &x.VisualStatusSystemName, &x.Quantity, &amount, &x.UnitPrice.CurrencyCode, &rd, &vc, &x.ObservedAt, &x.SourceReceivedAt)
	if order.Valid {
		x.CanonicalOrderID = &order.Int64
	}
	if posting.Valid {
		x.CanonicalPostingID = &posting.Int64
	}
	if sku.Valid {
		x.CanonicalSKUID = &sku.Int64
	}
	if rd.Valid {
		x.ProviderReturnDate = &rd.Time
	}
	if vc.Valid {
		x.VisualChangeAt = &vc.Time
	}
	x.UnitPrice.Amount, _ = factualapi.ParseDecimal(amount)
	if errors.Is(e, pgx.ErrNoRows) {
		return x, notFound("return")
	}
	if e != nil {
		return x, dbError(e)
	}
	x.Meta, e = s.meta(c, "LIFECYCLE.RETURNS", "RETURN", x.ID, nil, factHash(x))
	return x, e
}
func (s *Service) ListReturns(ctx context.Context, c factualapi.FactualReadContext, f factualapi.ReturnFilter, req factualapi.PageRequest) (factualapi.Page[factualapi.Return], error) {
	var out factualapi.Page[factualapi.Return]
	if e := s.validate(c); e != nil {
		return out, e
	}
	n, e := pageSize(req, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListReturns", f, req)
	if e != nil {
		return out, e
	}
	rows, e := s.pool.Query(ctx, `SELECT r.id FROM sales_returns r JOIN LATERAL(SELECT canonical_order_id,canonical_posting_id,canonical_sku_id FROM sales_return_events x WHERE x.sales_return_id=r.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)e ON true WHERE r.seller_account_id=$1 AND r.first_observed_at<=$2 AND r.id>$3 AND($4::bigint IS NULL OR e.canonical_posting_id=$4)AND($5::bigint IS NULL OR e.canonical_order_id=$5)AND($6::bigint IS NULL OR e.canonical_sku_id=$6)ORDER BY r.id LIMIT $7`, c.SellerAccountID(), c.CutoffAt(), last, f.PostingID, f.OrderID, f.SKUID, n+1)
	if e != nil {
		return out, dbError(e)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	has := len(ids) > n
	if has {
		ids = ids[:n]
	}
	out.Items, e = s.batchReturns(ctx, c, ids)
	if e != nil {
		return out, e
	}
	if len(ids) > 0 {
		out.NextPageToken, e = s.next(c, "ListReturns", h, ids[len(ids)-1], has)
	}
	return out, e
}

func (s *Service) batchPostings(ctx context.Context, c factualapi.FactualReadContext, ids []int64) ([]factualapi.Posting, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, e := s.pool.Query(ctx, `SELECT p.id,p.posting_number,v.id,v.fulfillment_scheme,v.order_resolution_status,v.canonical_order_id,v.version_no,v.provider_status,COALESCE(v.provider_substatus,''),v.observed_at,v.source_received_at FROM sales_postings p JOIN LATERAL(SELECT * FROM sales_posting_versions x WHERE x.canonical_posting_id=p.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)v ON true WHERE p.seller_account_id=$1 AND p.id=ANY($2) ORDER BY p.id`, c.SellerAccountID(), ids, c.CutoffAt())
	if e != nil {
		return nil, dbError(e)
	}
	defer rows.Close()
	items := make([]factualapi.Posting, 0, len(ids))
	byVersion := map[int64]int{}
	for rows.Next() {
		var x factualapi.Posting
		var versionID int64
		var order pgtype.Int8
		if e = rows.Scan(&x.ID, &x.PostingNumber, &versionID, &x.FulfillmentScheme, &x.OrderResolutionStatus, &order, &x.VersionNo, &x.ProviderStatus, &x.ProviderSubstatus, &x.ObservedAt, &x.SourceReceivedAt); e != nil {
			return nil, dbError(e)
		}
		if order.Valid {
			x.CanonicalOrderID = &order.Int64
		}
		byVersion[versionID] = len(items)
		items = append(items, x)
	}
	rows.Close()
	versions := make([]int64, 0, len(byVersion))
	for id := range byVersion {
		versions = append(versions, id)
	}
	products, e := s.pool.Query(ctx, `SELECT posting_version_id,canonical_sku_id,COALESCE(source_sku,0),sku_resolution_status,COALESCE(offer_id,''),COALESCE(product_name,''),quantity,unit_price::text,currency_code FROM sales_posting_products WHERE posting_version_id=ANY($1) ORDER BY posting_version_id,line_no`, versions)
	if e != nil {
		return nil, dbError(e)
	}
	for products.Next() {
		var vid int64
		var p factualapi.PostingProduct
		var sku pgtype.Int8
		var amount string
		if e = products.Scan(&vid, &sku, &p.SourceSKU, &p.ResolutionStatus, &p.OfferID, &p.Name, &p.Quantity, &amount, &p.UnitPrice.CurrencyCode); e != nil {
			products.Close()
			return nil, dbError(e)
		}
		if sku.Valid {
			p.CanonicalSKUID = &sku.Int64
		}
		p.UnitPrice.Amount, _ = factualapi.ParseDecimal(amount)
		items[byVersion[vid]].Products = append(items[byVersion[vid]].Products, p)
	}
	products.Close()
	rels, e := s.pool.Query(ctx, `SELECT r.child_posting_id,rv.parent_posting_id,r.parent_posting_number,rv.resolution_status FROM sales_posting_relations r JOIN LATERAL(SELECT parent_posting_id,resolution_status FROM sales_posting_relation_versions x WHERE x.posting_relation_id=r.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.revision_no DESC,x.id DESC LIMIT 1)rv ON true WHERE r.seller_account_id=$1 AND r.child_posting_id=ANY($2) ORDER BY r.child_posting_id,r.id`, c.SellerAccountID(), ids, c.CutoffAt())
	if e != nil {
		return nil, dbError(e)
	}
	byID := map[int64]int{}
	for i := range items {
		byID[items[i].ID] = i
	}
	for rels.Next() {
		var child int64
		var r factualapi.SplitRelation
		var parent pgtype.Int8
		if e = rels.Scan(&child, &parent, &r.ParentPostingNumber, &r.ResolutionStatus); e != nil {
			rels.Close()
			return nil, dbError(e)
		}
		if parent.Valid {
			r.ParentPostingID = &parent.Int64
		}
		items[byID[child]].Splits = append(items[byID[child]].Splits, r)
	}
	rels.Close()
	for i := range items {
		items[i].Meta, e = s.meta(c, "LIFECYCLE.POSTINGS", "POSTING", items[i].ID, nil, factHash(items[i]))
		if e != nil {
			return nil, e
		}
	}
	return items, nil
}

func (s *Service) batchReturns(ctx context.Context, c factualapi.FactualReadContext, ids []int64) ([]factualapi.Return, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, e := s.pool.Query(ctx, `SELECT r.id,r.provider_return_id,e.version_no,e.canonical_order_id,e.canonical_posting_id,e.canonical_sku_id,COALESCE(e.provider_type,''),COALESCE(e.provider_schema,''),COALESCE(e.return_reason_name,''),COALESCE(e.provider_visual_status,''),COALESCE(e.provider_visual_status_sys_name,''),e.quantity,e.unit_price::text,e.currency_code,e.provider_return_date,e.provider_visual_change_at,e.observed_at,e.source_received_at FROM sales_returns r JOIN LATERAL(SELECT * FROM sales_return_events x WHERE x.sales_return_id=r.id AND x.observed_at<=$3 ORDER BY x.observed_at DESC,x.version_no DESC,x.id DESC LIMIT 1)e ON true WHERE r.seller_account_id=$1 AND r.id=ANY($2) ORDER BY r.id`, c.SellerAccountID(), ids, c.CutoffAt())
	if e != nil {
		return nil, dbError(e)
	}
	defer rows.Close()
	out := make([]factualapi.Return, 0, len(ids))
	for rows.Next() {
		var x factualapi.Return
		var order, posting, sku pgtype.Int8
		var rd, vc pgtype.Timestamptz
		var amount string
		if e = rows.Scan(&x.ID, &x.ProviderReturnID, &x.VersionNo, &order, &posting, &sku, &x.ProviderType, &x.ProviderSchema, &x.Reason, &x.VisualStatus, &x.VisualStatusSystemName, &x.Quantity, &amount, &x.UnitPrice.CurrencyCode, &rd, &vc, &x.ObservedAt, &x.SourceReceivedAt); e != nil {
			return nil, dbError(e)
		}
		if order.Valid {
			x.CanonicalOrderID = &order.Int64
		}
		if posting.Valid {
			x.CanonicalPostingID = &posting.Int64
		}
		if sku.Valid {
			x.CanonicalSKUID = &sku.Int64
		}
		if rd.Valid {
			x.ProviderReturnDate = &rd.Time
		}
		if vc.Valid {
			x.VisualChangeAt = &vc.Time
		}
		x.UnitPrice.Amount, _ = factualapi.ParseDecimal(amount)
		x.Meta, e = s.meta(c, "LIFECYCLE.RETURNS", "RETURN", x.ID, nil, factHash(x))
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, nil
}
