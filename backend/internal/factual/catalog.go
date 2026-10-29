package factual

import (
	"context"
	"errors"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const skuAsKnownSQL = `SELECT cs.id,cs.ozon_sku,v.provider_product_id,COALESCE(v.offer_id,''),COALESCE(v.name,''),COALESCE(v.provider_status,''),v.catalog_state,v.observed_at,a.seller_category_id,COALESCE(a.mapping_status,'UNKNOWN'),COALESCE(a.category_resolution_status,'NOT_APPLICABLE'),COALESCE(a.product_type_resolution_status,'NOT_APPLICABLE'),a.observed_from,a.observed_to
FROM canonical_skus cs
JOIN LATERAL (SELECT * FROM canonical_sku_versions x WHERE x.canonical_sku_id=cs.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.id DESC LIMIT 1) v ON true
LEFT JOIN LATERAL (SELECT * FROM sku_category_assignments x WHERE x.canonical_sku_id=cs.id AND x.observed_from<=$2 AND (x.observed_to IS NULL OR x.observed_to>$2) ORDER BY x.observed_from DESC,x.id DESC LIMIT 1) a ON true
WHERE cs.seller_account_id=$1 AND cs.first_observed_at<=$2 AND cs.id=$3`

type scanner interface{ Scan(...any) error }

func scanSKU(row scanner) (factualapi.SKU, error) {
	var x factualapi.SKU
	var product, cat pgtype.Int8
	var from, to pgtype.Timestamptz
	e := row.Scan(&x.ID, &x.OzonSKU, &product, &x.OfferID, &x.Name, &x.ProviderStatus, &x.CatalogState, &x.ObservedAt, &cat, &x.Category.MappingStatus, &x.Category.CategoryResolutionStatus, &x.Category.ProductTypeResolutionStatus, &from, &to)
	if product.Valid {
		x.ProviderProductID = &product.Int64
	}
	if cat.Valid {
		x.Category.CategoryID = &cat.Int64
	}
	if from.Valid {
		x.Category.ObservedFrom = from.Time
	}
	if to.Valid {
		x.Category.ObservedTo = &to.Time
	}
	if x.Category.MappingStatus != "MAPPED" {
		x.Category.Limitation = &factualapi.Limitation{Code: "CATEGORY_MAPPING_PARTIAL", Message: "SKU has no resolved historical category at cutoff"}
	}
	return x, e
}
func (s *Service) GetSKU(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.SKU, error) {
	if e := s.validate(c); e != nil {
		return factualapi.SKU{}, e
	}
	x, e := scanSKU(s.pool.QueryRow(ctx, skuAsKnownSQL, c.SellerAccountID(), c.CutoffAt(), id))
	if errors.Is(e, pgx.ErrNoRows) {
		return x, notFound("SKU")
	}
	if e != nil {
		return x, dbError(e)
	}
	x.Meta, e = s.meta(c, "CATALOG.SKU_UNIVERSE", "SKU", x.ID, nil, factHash(x))
	return x, e
}
func (s *Service) BatchGetSKUs(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchSKUResult, error) {
	var out factualapi.BatchSKUResult
	if e := s.validate(c); e != nil {
		return out, e
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	rows, e := s.pool.Query(ctx, `SELECT cs.id,cs.ozon_sku,v.provider_product_id,COALESCE(v.offer_id,''),COALESCE(v.name,''),COALESCE(v.provider_status,''),v.catalog_state,v.observed_at,a.seller_category_id,COALESCE(a.mapping_status,'UNKNOWN'),COALESCE(a.category_resolution_status,'NOT_APPLICABLE'),COALESCE(a.product_type_resolution_status,'NOT_APPLICABLE'),a.observed_from,a.observed_to FROM canonical_skus cs JOIN LATERAL (SELECT * FROM canonical_sku_versions x WHERE x.canonical_sku_id=cs.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.id DESC LIMIT 1)v ON true LEFT JOIN LATERAL(SELECT * FROM sku_category_assignments x WHERE x.canonical_sku_id=cs.id AND x.observed_from<=$2 AND(x.observed_to IS NULL OR x.observed_to>$2)ORDER BY x.observed_from DESC,x.id DESC LIMIT 1)a ON true WHERE cs.seller_account_id=$1 AND cs.first_observed_at<=$2 AND cs.id=ANY($3)`, c.SellerAccountID(), c.CutoffAt(), ids)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	found := map[int64]factualapi.SKU{}
	for rows.Next() {
		x, se := scanSKU(rows)
		if se != nil {
			return out, dbError(se)
		}
		x.Meta, _ = s.meta(c, "CATALOG.SKU_UNIVERSE", "SKU", x.ID, nil, factHash(x))
		found[x.ID] = x
	}
	for _, id := range ids {
		if x, ok := found[id]; ok {
			v := x
			out.Items = append(out.Items, factualapi.BatchSKUItem{ID: id, Status: factualapi.BatchFound, SKU: &v})
		} else {
			out.Items = append(out.Items, factualapi.BatchSKUItem{ID: id, Status: factualapi.BatchNotFound})
		}
	}
	return out, rows.Err()
}
func (s *Service) ListSKUs(ctx context.Context, c factualapi.FactualReadContext, f factualapi.SKUFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.SKU], error) {
	var out factualapi.Page[factualapi.SKU]
	if e := s.validate(c); e != nil {
		return out, e
	}
	n, e := pageSize(p, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListSKUs", f, p)
	if e != nil {
		return out, e
	}
	var cats any = f.CategoryIDs
	if len(f.CategoryIDs) == 0 {
		cats = nil
	}
	rows, e := s.pool.Query(ctx, `SELECT cs.id,cs.ozon_sku,v.provider_product_id,COALESCE(v.offer_id,''),COALESCE(v.name,''),COALESCE(v.provider_status,''),v.catalog_state,v.observed_at,a.seller_category_id,COALESCE(a.mapping_status,'UNKNOWN'),COALESCE(a.category_resolution_status,'NOT_APPLICABLE'),COALESCE(a.product_type_resolution_status,'NOT_APPLICABLE'),a.observed_from,a.observed_to FROM canonical_skus cs JOIN LATERAL(SELECT * FROM canonical_sku_versions x WHERE x.canonical_sku_id=cs.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.id DESC LIMIT 1)v ON true LEFT JOIN LATERAL(SELECT * FROM sku_category_assignments x WHERE x.canonical_sku_id=cs.id AND x.observed_from<=$2 AND(x.observed_to IS NULL OR x.observed_to>$2)ORDER BY x.observed_from DESC,x.id DESC LIMIT 1)a ON true WHERE cs.seller_account_id=$1 AND cs.first_observed_at<=$2 AND cs.id>$3 AND($4='' OR v.catalog_state=$4)AND($5::bigint[] IS NULL OR a.seller_category_id=ANY($5))ORDER BY cs.id LIMIT $6`, c.SellerAccountID(), c.CutoffAt(), last, f.CatalogState, cats, n+1)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		x, se := scanSKU(rows)
		if se != nil {
			return out, dbError(se)
		}
		x.Meta, _ = s.meta(c, "CATALOG.SKU_UNIVERSE", "SKU", x.ID, nil, factHash(x))
		out.Items = append(out.Items, x)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, e = s.next(c, "ListSKUs", h, out.Items[len(out.Items)-1].ID, has)
	}
	return out, e
}

const categoryAsKnownSQL = `SELECT sc.id,pc.provider_category_id,v.name,v.taxonomy_state,parent_pc.provider_category_id,v.is_leaf,v.observed_at FROM seller_categories sc JOIN provider_categories pc ON pc.id=sc.provider_category_id JOIN LATERAL(SELECT * FROM provider_category_versions x WHERE x.provider_category_id=pc.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.id DESC LIMIT 1)v ON true LEFT JOIN provider_categories parent_pc ON parent_pc.id=v.parent_provider_category_id WHERE sc.seller_account_id=$1 AND sc.id=$3 AND sc.first_observed_at<=$2`

func scanCategory(row scanner) (factualapi.Category, error) {
	var x factualapi.Category
	var parent pgtype.Int8
	e := row.Scan(&x.ID, &x.ProviderCategoryID, &x.Name, &x.State, &parent, &x.IsLeaf, &x.ObservedAt)
	if parent.Valid {
		x.ParentProviderCategoryID = &parent.Int64
	}
	x.SellerCategoryID = x.ID
	return x, e
}

func (s *Service) GetCategory(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.Category, error) {
	var x factualapi.Category
	if e := s.validate(c); e != nil {
		return x, e
	}
	x, e := scanCategory(s.pool.QueryRow(ctx, categoryAsKnownSQL, c.SellerAccountID(), c.CutoffAt(), id))
	if errors.Is(e, pgx.ErrNoRows) {
		return x, notFound("category")
	}
	if e != nil {
		return x, dbError(e)
	}
	x.Meta, e = s.meta(c, "CATALOG.CATEGORY_ATTRIBUTION", "CATEGORY", x.ID, nil, factHash(x))
	return x, e
}

func (s *Service) BatchGetCategories(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchCategoryResult, error) {
	var out factualapi.BatchCategoryResult
	if e := s.validate(c); e != nil {
		return out, e
	}
	if len(ids) > s.maxBatch {
		return out, factualapi.NewError(factualapi.CodeInvalidArgument, "batch limit exceeded")
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, e := s.pool.Query(ctx, `SELECT sc.id,pc.provider_category_id,v.name,v.taxonomy_state,parent_pc.provider_category_id,v.is_leaf,v.observed_at FROM seller_categories sc JOIN provider_categories pc ON pc.id=sc.provider_category_id JOIN LATERAL(SELECT * FROM provider_category_versions x WHERE x.provider_category_id=pc.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.id DESC LIMIT 1)v ON true LEFT JOIN provider_categories parent_pc ON parent_pc.id=v.parent_provider_category_id WHERE sc.seller_account_id=$1 AND sc.first_observed_at<=$2 AND sc.id=ANY($3)`, c.SellerAccountID(), c.CutoffAt(), ids)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	found := map[int64]factualapi.Category{}
	for rows.Next() {
		x, se := scanCategory(rows)
		if se != nil {
			return out, dbError(se)
		}
		x.Meta, _ = s.meta(c, "CATALOG.CATEGORY_ATTRIBUTION", "CATEGORY", x.ID, nil, factHash(x))
		found[x.ID] = x
	}
	if e = rows.Err(); e != nil {
		return out, dbError(e)
	}
	for _, id := range ids {
		if x, ok := found[id]; ok {
			v := x
			out.Items = append(out.Items, factualapi.BatchCategoryItem{ID: id, Status: factualapi.BatchFound, Category: &v})
		} else {
			out.Items = append(out.Items, factualapi.BatchCategoryItem{ID: id, Status: factualapi.BatchNotFound})
		}
	}
	return out, nil
}
func (s *Service) ListCategories(ctx context.Context, c factualapi.FactualReadContext, p factualapi.PageRequest) (factualapi.Page[factualapi.Category], error) {
	var out factualapi.Page[factualapi.Category]
	if e := s.validate(c); e != nil {
		return out, e
	}
	n, e := pageSize(p, s.maxPage)
	if e != nil {
		return out, e
	}
	last, h, e := s.cursor(c, "ListCategories", struct{}{}, p)
	if e != nil {
		return out, e
	}
	rows, e := s.pool.Query(ctx, `SELECT sc.id,pc.provider_category_id,v.name,v.taxonomy_state,parent_pc.provider_category_id,v.is_leaf,v.observed_at FROM seller_categories sc JOIN provider_categories pc ON pc.id=sc.provider_category_id JOIN LATERAL(SELECT * FROM provider_category_versions x WHERE x.provider_category_id=pc.id AND x.observed_at<=$2 ORDER BY x.observed_at DESC,x.id DESC LIMIT 1)v ON true LEFT JOIN provider_categories parent_pc ON parent_pc.id=v.parent_provider_category_id WHERE sc.seller_account_id=$1 AND sc.first_observed_at<=$2 AND sc.id>$3 ORDER BY sc.id LIMIT $4`, c.SellerAccountID(), c.CutoffAt(), last, n+1)
	if e != nil {
		return out, dbError(e)
	}
	defer rows.Close()
	for rows.Next() {
		var x factualapi.Category
		var parent pgtype.Int8
		if e = rows.Scan(&x.ID, &x.ProviderCategoryID, &x.Name, &x.State, &parent, &x.IsLeaf, &x.ObservedAt); e != nil {
			return out, dbError(e)
		}
		if parent.Valid {
			x.ParentProviderCategoryID = &parent.Int64
		}
		x.SellerCategoryID = x.ID
		x.Meta, _ = s.meta(c, "CATALOG.CATEGORY_ATTRIBUTION", "CATEGORY", x.ID, nil, factHash(x))
		out.Items = append(out.Items, x)
	}
	has := len(out.Items) > n
	if has {
		out.Items = out.Items[:n]
	}
	if len(out.Items) > 0 {
		out.NextPageToken, e = s.next(c, "ListCategories", h, out.Items[len(out.Items)-1].ID, has)
	}
	return out, e
}
