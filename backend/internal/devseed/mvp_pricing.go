package devseed

import (
	"context"
	"fmt"
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/pricingconstraints"
	"github.com/jackc/pgx/v5/pgtype"
)

func float64Ptr(v float64) *float64 {
	return &v
}

// mvpPricingOrphanDescriptionCategoryID is outside categoryDescriptionID() range so no category_rule matches.
func mvpPricingOrphanDescriptionCategoryID(sellerAccountID int64) int64 {
	return 17_990_000 + sellerAccountID%7000
}

func productUpsertParamsFromRow(p dbgen.Product) dbgen.UpsertProductParams {
	return dbgen.UpsertProductParams{
		SellerAccountID:       p.SellerAccountID,
		OzonProductID:         p.OzonProductID,
		OfferID:               p.OfferID,
		Sku:                   p.Sku,
		Name:                  p.Name,
		Status:                p.Status,
		ReferencePrice:        p.ReferencePrice,
		OldPrice:              p.OldPrice,
		OzonMinPrice:          p.OzonMinPrice,
		DescriptionCategoryID: p.DescriptionCategoryID,
		IsArchived:            p.IsArchived,
		RawAttributes:         p.RawAttributes,
		SourceUpdatedAt:       p.SourceUpdatedAt,
	}
}

func collectProductsBySegment(products []dbgen.Product) map[mvpCommerceSegment][]dbgen.Product {
	out := make(map[mvpCommerceSegment][]dbgen.Product)
	for _, p := range products {
		seg := productSegmentFromRaw(p.RawAttributes)
		if seg == "" {
			continue
		}
		out[seg] = append(out[seg], p)
	}
	for k := range out {
		sort.Slice(out[k], func(i, j int) bool { return out[k][i].OzonProductID < out[k][j].OzonProductID })
	}
	return out
}

// SeedMVPPricing writes global_default, category_rule, sku_override rows and recomputes sku_effective_constraints
// via pricingconstraints.Service (no manual effective rows).
//
// Shape: 1× global_default; 6× category_rule (kitchen_storage, home_textile, cleaning, bathroom, light_decor,
// home_organization); 10× sku_override (2 leaders, 2 low_stock, 3 overstock, 3 price_risk — within 8–12);
// two products patched to an orphan description_category_id so some SKUs resolve to global_default.
func SeedMVPPricing(ctx context.Context, q *dbgen.Queries, sellerAccountID int64) (*PricingSeedStats, error) {
	stats := &PricingSeedStats{}
	svc := pricingconstraints.NewService(q)

	products, err := q.ListProductsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list products for pricing: %w", err)
	}
	if len(products) == 0 {
		return stats, nil
	}

	bySeg := collectProductsBySegment(products)

	gRef := 1200.0
	gMargin := 0.28
	gImplied, err := pricingconstraints.ComputeImpliedCost(gRef, gMargin)
	if err != nil {
		return nil, err
	}
	if _, err := svc.UpsertGlobalDefault(ctx, pricingconstraints.UpsertRuleInput{
		SellerAccountID:        sellerAccountID,
		MinPrice:               float64Ptr(149),
		MaxPrice:               float64Ptr(999_999),
		ReferenceMarginPercent: float64Ptr(gMargin),
		ReferencePrice:         float64Ptr(gRef),
		ImpliedCost:            float64Ptr(gImplied),
		IsActive:               true,
	}); err != nil {
		return nil, fmt.Errorf("upsert global_default: %w", err)
	}
	stats.Rules++

	type catSpec struct {
		cat    mvpCommerceCategory
		min    float64
		max    float64
		margin float64
		pref   float64
	}
	categorySpecs := []catSpec{
		{catKitchenStorage, 189, 280_000, 0.30, 920},
		{catHomeTextile, 249, 320_000, 0.32, 1400},
		{catCleaning, 159, 200_000, 0.26, 650},
		{catBathroom, 179, 220_000, 0.29, 780},
		{catLightDecor, 219, 350_000, 0.27, 1100},
		{catHomeOrganization, 199, 310_000, 0.28, 880},
	}
	for _, cs := range categorySpecs {
		catID := categoryDescriptionID(cs.cat, sellerAccountID)
		implied, err := pricingconstraints.ComputeImpliedCost(cs.pref, cs.margin)
		if err != nil {
			return nil, err
		}
		cid := catID
		if _, err := svc.UpsertCategoryRule(ctx, pricingconstraints.UpsertRuleInput{
			SellerAccountID:        sellerAccountID,
			ScopeTargetID:          &cid,
			MinPrice:               float64Ptr(cs.min),
			MaxPrice:               float64Ptr(cs.max),
			ReferenceMarginPercent: float64Ptr(cs.margin),
			ReferencePrice:         float64Ptr(cs.pref),
			ImpliedCost:            float64Ptr(implied),
			IsActive:               true,
		}); err != nil {
			return nil, fmt.Errorf("upsert category_rule %s: %w", cs.cat, err)
		}
		stats.Rules++
	}

	taken := make(map[int64]struct{})

	take := func(seg mvpCommerceSegment, n int) []dbgen.Product {
		var xs []dbgen.Product
		for _, p := range bySeg[seg] {
			if _, ok := taken[p.OzonProductID]; ok {
				continue
			}
			xs = append(xs, p)
			if len(xs) >= n {
				break
			}
		}
		for _, p := range xs {
			taken[p.OzonProductID] = struct{}{}
		}
		return xs
	}

	upsertSKUOverride := func(p dbgen.Product, minF, maxF, margin float64) error {
		ref := numericToFloat64(p.ReferencePrice)
		if ref <= 0 {
			return fmt.Errorf("product %d: invalid reference price", p.OzonProductID)
		}
		implied, err := pricingconstraints.ComputeImpliedCost(ref, margin)
		if err != nil {
			return err
		}
		pid := p.OzonProductID
		kind := pricingconstraints.ScopeTargetKindProductID
		if _, err := svc.UpsertSKUOverride(ctx, pricingconstraints.UpsertRuleInput{
			SellerAccountID:        sellerAccountID,
			ScopeTargetKind:        &kind,
			ScopeTargetID:          &pid,
			MinPrice:               float64Ptr(minF),
			MaxPrice:               float64Ptr(maxF),
			ReferenceMarginPercent: float64Ptr(margin),
			ReferencePrice:         float64Ptr(ref),
			ImpliedCost:            float64Ptr(implied),
			IsActive:               true,
		}); err != nil {
			return err
		}
		stats.Rules++
		return nil
	}

	for _, p := range take(segLeaders, 2) {
		ref := numericToFloat64(p.ReferencePrice)
		if err := upsertSKUOverride(p, mvpRoundMoney(ref*0.86), mvpRoundMoney(ref*3.5), 0.34); err != nil {
			return nil, err
		}
	}
	for _, p := range take(segLowStock, 2) {
		ref := numericToFloat64(p.ReferencePrice)
		if err := upsertSKUOverride(p, mvpRoundMoney(ref*0.96), mvpRoundMoney(ref*2.8), 0.30); err != nil {
			return nil, err
		}
	}
	for _, p := range take(segOverstock, 3) {
		ref := numericToFloat64(p.ReferencePrice)
		if err := upsertSKUOverride(p, mvpRoundMoney(ref*0.52), mvpRoundMoney(ref*2.2), 0.24); err != nil {
			return nil, err
		}
	}
	for _, p := range take(segPriceRisk, 3) {
		ref := numericToFloat64(p.ReferencePrice)
		if err := upsertSKUOverride(p, mvpRoundMoney(ref*0.978), mvpRoundMoney(ref*1.25), 0.08); err != nil {
			return nil, err
		}
	}

	orphan := mvpPricingOrphanDescriptionCategoryID(sellerAccountID)
	patched := 0
	patchOrphans := func(match func(dbgen.Product) bool) error {
		for _, p := range products {
			if patched >= 2 {
				return nil
			}
			if _, ok := taken[p.OzonProductID]; ok {
				continue
			}
			if p.IsArchived {
				continue
			}
			if !match(p) {
				continue
			}
			up := productUpsertParamsFromRow(p)
			up.DescriptionCategoryID = pgtype.Int8{Int64: orphan, Valid: true}
			if _, err := q.UpsertProduct(ctx, up); err != nil {
				return fmt.Errorf("orphan category patch product %d: %w", p.OzonProductID, err)
			}
			patched++
		}
		return nil
	}
	if err := patchOrphans(func(p dbgen.Product) bool {
		return productSegmentFromRaw(p.RawAttributes) == segStableTail
	}); err != nil {
		return nil, err
	}
	if patched < 2 {
		if err := patchOrphans(func(dbgen.Product) bool { return true }); err != nil {
			return nil, err
		}
	}
	if patched < 2 {
		return nil, fmt.Errorf("pricing seed: need 2 products for global_default path, patched %d", patched)
	}

	re, err := svc.RecomputeEffectiveConstraintsForAccount(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("recompute effective constraints: %w", err)
	}
	stats.Effective = re.MaterializedCount

	return stats, nil
}

func numericToFloat64(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}
