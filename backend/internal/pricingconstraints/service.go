package pricingconstraints

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	queries  *dbgen.Queries
	resolver *Resolver
}

func NewService(queries *dbgen.Queries) *Service {
	repo := NewSQLCRuleRepository(queries)
	return &Service{
		queries:  queries,
		resolver: NewResolver(repo),
	}
}

type UpsertRuleInput struct {
	SellerAccountID        int64
	ScopeTargetKind        *ScopeTargetKind
	ScopeTargetID          *int64
	ScopeTargetCode        *string
	MinPrice               *float64
	MaxPrice               *float64
	ReferenceMarginPercent *float64
	ReferencePrice         *float64
	ImpliedCost            *float64
	IsActive               bool
}

type RecomputeResult struct {
	SellerAccountID      int64
	ProductsScanned      int
	MaterializedCount    int
	NoConstraintsCount   int
	SkippedInvalidInputs int
	ComputedAt           time.Time
}

type RuleSets struct {
	GlobalDefault  *Rule
	CategoryRules  []Rule
	SKUOverrides   []Rule
	TotalRules     int
	ActiveRules    int
	LastRuleUpdate *time.Time
}

type EffectiveConstraintsPage struct {
	Items          []dbgen.SkuEffectiveConstraint
	Total          int
	Limit          int
	Offset         int
	LastComputedAt *time.Time
}

func (s *Service) UpsertGlobalDefault(ctx context.Context, input UpsertRuleInput) (Rule, error) {
	input.ScopeTargetKind = nil
	input.ScopeTargetID = nil
	input.ScopeTargetCode = nil
	return s.upsertRuleByScope(ctx, ScopeTypeGlobalDefault, input)
}

func (s *Service) UpsertCategoryRule(ctx context.Context, input UpsertRuleInput) (Rule, error) {
	kind := ScopeTargetKindCategoryID
	input.ScopeTargetKind = &kind
	return s.upsertRuleByScope(ctx, ScopeTypeCategoryRule, input)
}

func (s *Service) UpsertSKUOverride(ctx context.Context, input UpsertRuleInput) (Rule, error) {
	if input.ScopeTargetKind == nil {
		if input.ScopeTargetCode != nil {
			kind := ScopeTargetKindOfferID
			input.ScopeTargetKind = &kind
		} else {
			kind := ScopeTargetKindProductID
			input.ScopeTargetKind = &kind
		}
	}
	return s.upsertRuleByScope(ctx, ScopeTypeSKUOverride, input)
}

func (s *Service) DeactivateCategoryRuleByID(ctx context.Context, sellerAccountID int64, ruleID int64) (Rule, error) {
	row, err := s.queries.DeactivatePricingConstraintRuleByIDAndScope(ctx, dbgen.DeactivatePricingConstraintRuleByIDAndScopeParams{
		ID:              ruleID,
		SellerAccountID: sellerAccountID,
		ScopeType:       string(ScopeTypeCategoryRule),
	})
	if err != nil {
		return Rule{}, err
	}
	return mapRule(row), nil
}

func (s *Service) DeactivateSKUOverrideByID(ctx context.Context, sellerAccountID int64, ruleID int64) (Rule, error) {
	row, err := s.queries.DeactivatePricingConstraintRuleByIDAndScope(ctx, dbgen.DeactivatePricingConstraintRuleByIDAndScopeParams{
		ID:              ruleID,
		SellerAccountID: sellerAccountID,
		ScopeType:       string(ScopeTypeSKUOverride),
	})
	if err != nil {
		return Rule{}, err
	}
	return mapRule(row), nil
}

func (s *Service) RecomputeEffectiveConstraintsForAccount(ctx context.Context, sellerAccountID int64) (RecomputeResult, error) {
	products, err := s.queries.ListProductsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return RecomputeResult{}, fmt.Errorf("list products by seller account: %w", err)
	}

	if err := s.queries.DeleteSKUEffectiveConstraintsBySellerAccountID(ctx, sellerAccountID); err != nil {
		return RecomputeResult{}, fmt.Errorf("delete existing effective constraints: %w", err)
	}

	result := RecomputeResult{
		SellerAccountID: sellerAccountID,
		ProductsScanned: len(products),
		ComputedAt:      time.Now().UTC(),
	}

	for _, product := range products {
		referencePrice := numericPtr(product.ReferencePrice)
		if referencePrice == nil || *referencePrice <= 0 {
			result.SkippedInvalidInputs++
			continue
		}

		resolved, err := s.resolver.Resolve(ctx, ResolveInput{
			SellerAccountID:       sellerAccountID,
			OzonProductID:         product.OzonProductID,
			SKU:                   int8Ptr(product.Sku),
			OfferID:               textPtr(product.OfferID),
			DescriptionCategoryID: int8Ptr(product.DescriptionCategoryID),
			ReferencePrice:        *referencePrice,
		})
		if err != nil {
			return RecomputeResult{}, fmt.Errorf("resolve product=%d: %w", product.OzonProductID, err)
		}
		if !resolved.HasConstraints {
			result.NoConstraintsCount++
			continue
		}

		record, err := resolved.ToEffectiveConstraintRecord(ResolveInput{
			SellerAccountID: sellerAccountID,
			OzonProductID:   product.OzonProductID,
			SKU:             int8Ptr(product.Sku),
			OfferID:         textPtr(product.OfferID),
		})
		if err != nil {
			return RecomputeResult{}, fmt.Errorf("materialize resolved constraints product=%d: %w", product.OzonProductID, err)
		}

		if _, err := s.queries.UpsertSKUEffectiveConstraint(ctx, dbgen.UpsertSKUEffectiveConstraintParams{
			SellerAccountID:        record.SellerAccountID,
			OzonProductID:          record.OzonProductID,
			Sku:                    ptrToInt8(record.SKU),
			OfferID:                ptrToText(record.OfferID),
			ResolvedFromScopeType:  string(record.ResolvedFromScopeType),
			RuleID:                 record.RuleID,
			EffectiveMinPrice:      ptrToNumeric(record.EffectiveMinPrice),
			EffectiveMaxPrice:      ptrToNumeric(record.EffectiveMaxPrice),
			ReferencePrice:         ptrToNumeric(record.ReferencePrice),
			ReferenceMarginPercent: ptrToNumeric(record.ReferenceMarginPercent),
			ImpliedCost:            ptrToNumeric(record.ImpliedCost),
		}); err != nil {
			return RecomputeResult{}, fmt.Errorf("upsert effective constraints product=%d: %w", product.OzonProductID, err)
		}
		result.MaterializedCount++
	}

	return result, nil
}

func (s *Service) PreviewImpliedCost(referencePrice float64, referenceMarginPercent float64) (float64, error) {
	return ComputeImpliedCost(referencePrice, referenceMarginPercent)
}

func (s *Service) PreviewExpectedMargin(newPrice float64, impliedCost float64) (float64, error) {
	return ComputeExpectedMargin(newPrice, impliedCost)
}

func (s *Service) ListEffectiveConstraintsBySellerAccountID(ctx context.Context, sellerAccountID int64) ([]dbgen.SkuEffectiveConstraint, error) {
	return s.queries.ListSKUEffectiveConstraintsBySellerAccountID(ctx, sellerAccountID)
}

func (s *Service) ListRuleSetsBySellerAccountID(ctx context.Context, sellerAccountID int64) (RuleSets, error) {
	rows, err := s.queries.ListPricingConstraintRulesBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return RuleSets{}, err
	}
	result := RuleSets{
		CategoryRules: make([]Rule, 0),
		SKUOverrides:  make([]Rule, 0),
		TotalRules:    len(rows),
	}
	for _, row := range rows {
		rule := mapRule(row)
		if row.IsActive {
			result.ActiveRules++
		}
		if row.UpdatedAt.Valid {
			t := row.UpdatedAt.Time.UTC()
			if result.LastRuleUpdate == nil || t.After(*result.LastRuleUpdate) {
				result.LastRuleUpdate = &t
			}
		}
		switch rule.ScopeType {
		case ScopeTypeGlobalDefault:
			if result.GlobalDefault == nil || (rule.IsActive && !result.GlobalDefault.IsActive) {
				copied := rule
				result.GlobalDefault = &copied
			}
		case ScopeTypeCategoryRule:
			result.CategoryRules = append(result.CategoryRules, rule)
		case ScopeTypeSKUOverride:
			result.SKUOverrides = append(result.SKUOverrides, rule)
		}
	}
	return result, nil
}

func (s *Service) ListEffectiveConstraintsPage(ctx context.Context, sellerAccountID int64, limit int, offset int) (EffectiveConstraintsPage, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	totalCount, err := s.queries.CountSKUEffectiveConstraintsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return EffectiveConstraintsPage{}, err
	}
	items, err := s.queries.ListSKUEffectiveConstraintsPageBySellerAccountID(ctx, dbgen.ListSKUEffectiveConstraintsPageBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           int32(limit),
		Offset:          int32(offset),
	})
	if err != nil {
		return EffectiveConstraintsPage{}, err
	}
	var lastComputedAt *time.Time
	if len(items) > 0 && items[0].ComputedAt.Valid {
		t := items[0].ComputedAt.Time.UTC()
		lastComputedAt = &t
	}
	return EffectiveConstraintsPage{
		Items:          items,
		Total:          int(totalCount),
		Limit:          limit,
		Offset:         offset,
		LastComputedAt: lastComputedAt,
	}, nil
}

func (s *Service) GetEffectiveConstraintBySellerAndProduct(ctx context.Context, sellerAccountID int64, ozonProductID int64) (dbgen.SkuEffectiveConstraint, error) {
	return s.queries.GetSKUEffectiveConstraintBySellerAndProduct(ctx, dbgen.GetSKUEffectiveConstraintBySellerAndProductParams{
		SellerAccountID: sellerAccountID,
		OzonProductID:   ozonProductID,
	})
}

func (s *Service) GetEffectiveConstraintBySellerAndSKU(ctx context.Context, sellerAccountID int64, sku int64) (dbgen.SkuEffectiveConstraint, error) {
	return s.queries.GetSKUEffectiveConstraintBySellerAndSKU(ctx, dbgen.GetSKUEffectiveConstraintBySellerAndSKUParams{
		SellerAccountID: sellerAccountID,
		Sku:             pgtype.Int8{Int64: sku, Valid: true},
	})
}

func (s *Service) ListProductsBySellerAccountID(ctx context.Context, sellerAccountID int64) ([]dbgen.Product, error) {
	return s.queries.ListProductsBySellerAccountID(ctx, sellerAccountID)
}

func (s *Service) GetProductBySellerAndProductID(ctx context.Context, sellerAccountID int64, ozonProductID int64) (dbgen.Product, error) {
	return s.queries.GetProductBySellerAndOzonProductID(ctx, dbgen.GetProductBySellerAndOzonProductIDParams{
		SellerAccountID: sellerAccountID,
		OzonProductID:   ozonProductID,
	})
}

func (s *Service) upsertRuleByScope(ctx context.Context, scopeType ScopeType, input UpsertRuleInput) (Rule, error) {
	if err := ValidateRuleInput(RuleValidationInput{
		ScopeType:              scopeType,
		ScopeTargetKind:        input.ScopeTargetKind,
		MinPrice:               input.MinPrice,
		MaxPrice:               input.MaxPrice,
		ReferenceMarginPercent: input.ReferenceMarginPercent,
		ReferencePrice:         input.ReferencePrice,
		ImpliedCost:            input.ImpliedCost,
	}); err != nil {
		return Rule{}, err
	}

	existing, err := s.queries.ListPricingConstraintRulesByScope(ctx, dbgen.ListPricingConstraintRulesByScopeParams{
		SellerAccountID: input.SellerAccountID,
		ScopeType:       string(scopeType),
		ScopeTargetKind: nullableTargetKind(input.ScopeTargetKind),
		ScopeTargetID:   nullableInt64(input.ScopeTargetID),
		ScopeTargetCode: nullableText(input.ScopeTargetCode),
	})
	if err != nil {
		return Rule{}, fmt.Errorf("list rules by scope for upsert: %w", err)
	}

	isActive := input.IsActive
	if len(existing) == 0 && !input.IsActive {
		isActive = true
	}

	var winner dbgen.PricingConstraintRule
	if len(existing) > 0 {
		target := existing[0]
		updated, err := s.queries.UpdatePricingConstraintRule(ctx, dbgen.UpdatePricingConstraintRuleParams{
			ID:                     target.ID,
			ScopeType:              string(scopeType),
			ScopeTargetKind:        nullableTargetKind(input.ScopeTargetKind),
			ScopeTargetID:          nullableInt64(input.ScopeTargetID),
			ScopeTargetCode:        nullableText(input.ScopeTargetCode),
			MinPrice:               ptrToNumeric(input.MinPrice),
			MaxPrice:               ptrToNumeric(input.MaxPrice),
			ReferenceMarginPercent: ptrToNumeric(input.ReferenceMarginPercent),
			ReferencePrice:         ptrToNumeric(input.ReferencePrice),
			ImpliedCost:            ptrToNumeric(input.ImpliedCost),
			IsActive:               isActive,
			SellerAccountID:        input.SellerAccountID,
		})
		if err != nil {
			return Rule{}, fmt.Errorf("update winning rule id=%d: %w", target.ID, err)
		}
		winner = updated

		for _, row := range existing[1:] {
			if !row.IsActive {
				continue
			}
			if _, err := s.queries.UpdatePricingConstraintRule(ctx, dbgen.UpdatePricingConstraintRuleParams{
				ID:                     row.ID,
				ScopeType:              row.ScopeType,
				ScopeTargetKind:        row.ScopeTargetKind,
				ScopeTargetID:          row.ScopeTargetID,
				ScopeTargetCode:        row.ScopeTargetCode,
				MinPrice:               row.MinPrice,
				MaxPrice:               row.MaxPrice,
				ReferenceMarginPercent: row.ReferenceMarginPercent,
				ReferencePrice:         row.ReferencePrice,
				ImpliedCost:            row.ImpliedCost,
				IsActive:               false,
				SellerAccountID:        row.SellerAccountID,
			}); err != nil {
				return Rule{}, fmt.Errorf("deactivate duplicate rule id=%d: %w", row.ID, err)
			}
		}
	} else {
		created, err := s.queries.CreatePricingConstraintRule(ctx, dbgen.CreatePricingConstraintRuleParams{
			SellerAccountID:        input.SellerAccountID,
			ScopeType:              string(scopeType),
			ScopeTargetKind:        nullableTargetKind(input.ScopeTargetKind),
			ScopeTargetID:          nullableInt64(input.ScopeTargetID),
			ScopeTargetCode:        nullableText(input.ScopeTargetCode),
			MinPrice:               ptrToNumeric(input.MinPrice),
			MaxPrice:               ptrToNumeric(input.MaxPrice),
			ReferenceMarginPercent: ptrToNumeric(input.ReferenceMarginPercent),
			ReferencePrice:         ptrToNumeric(input.ReferencePrice),
			ImpliedCost:            ptrToNumeric(input.ImpliedCost),
			IsActive:               isActive,
		})
		if err != nil {
			return Rule{}, fmt.Errorf("create rule: %w", err)
		}
		winner = created
	}

	return mapRule(winner), nil
}

func ptrToNumeric(v *float64) pgtype.Numeric {
	if v == nil {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	if err := n.Scan(*v); err != nil {
		return pgtype.Numeric{Valid: false}
	}
	return n
}

func ptrToText(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func ptrToInt8(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}
