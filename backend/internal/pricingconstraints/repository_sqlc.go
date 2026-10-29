package pricingconstraints

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type RuleRepository interface {
	ListByScope(
		ctx context.Context,
		sellerAccountID int64,
		scopeType ScopeType,
		targetKind *ScopeTargetKind,
		targetID *int64,
		targetCode *string,
	) ([]Rule, error)
}

type SQLCRuleRepository struct {
	queries *dbgen.Queries
}

func NewSQLCRuleRepository(queries *dbgen.Queries) *SQLCRuleRepository {
	return &SQLCRuleRepository{queries: queries}
}

func (r *SQLCRuleRepository) ListByScope(
	ctx context.Context,
	sellerAccountID int64,
	scopeType ScopeType,
	targetKind *ScopeTargetKind,
	targetID *int64,
	targetCode *string,
) ([]Rule, error) {
	rows, err := r.queries.ListPricingConstraintRulesByScope(ctx, dbgen.ListPricingConstraintRulesByScopeParams{
		SellerAccountID: sellerAccountID,
		ScopeType:       string(scopeType),
		ScopeTargetKind: nullableTargetKind(targetKind),
		ScopeTargetID:   nullableInt64(targetID),
		ScopeTargetCode: nullableText(targetCode),
	})
	if err != nil {
		return nil, fmt.Errorf("list pricing constraint rules by scope=%s: %w", scopeType, err)
	}
	out := make([]Rule, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapRule(row))
	}
	return out, nil
}

type EffectiveConstraintRecord struct {
	SellerAccountID        int64
	OzonProductID          int64
	SKU                    *int64
	OfferID                *string
	ResolvedFromScopeType  ScopeType
	RuleID                 int64
	EffectiveMinPrice      *float64
	EffectiveMaxPrice      *float64
	ReferencePrice         *float64
	ReferenceMarginPercent *float64
	ImpliedCost            *float64
	ComputedAt             time.Time
}

func mapRule(row dbgen.PricingConstraintRule) Rule {
	return Rule{
		ID:                     row.ID,
		SellerAccountID:        row.SellerAccountID,
		ScopeType:              ScopeType(row.ScopeType),
		ScopeTargetKind:        targetKindPtr(row.ScopeTargetKind),
		ScopeTargetID:          int8Ptr(row.ScopeTargetID),
		ScopeTargetCode:        textPtr(row.ScopeTargetCode),
		MinPrice:               numericPtr(row.MinPrice),
		MaxPrice:               numericPtr(row.MaxPrice),
		ReferenceMarginPercent: numericPtr(row.ReferenceMarginPercent),
		ReferencePrice:         numericPtr(row.ReferencePrice),
		ImpliedCost:            numericPtr(row.ImpliedCost),
		IsActive:               row.IsActive,
		CreatedAt:              timestamptz(row.CreatedAt),
		UpdatedAt:              timestamptz(row.UpdatedAt),
	}
}

func nullableTargetKind(v *ScopeTargetKind) pgtype.Text {
	if v == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: string(*v), Valid: true}
}

func nullableInt64(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func nullableText(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func int8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	i := v.Int64
	return &i
}

func targetKindPtr(v pgtype.Text) *ScopeTargetKind {
	if !v.Valid {
		return nil
	}
	kind := ScopeTargetKind(v.String)
	return &kind
}

func numericPtr(v pgtype.Numeric) *float64 {
	if !v.Valid {
		return nil
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	x := f.Float64
	return &x
}

func timestamptz(v pgtype.Timestamptz) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time.UTC()
}
