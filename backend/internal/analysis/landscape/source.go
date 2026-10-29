package landscape

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"time"
)

// CoreDataSource supplies batch-shaped Landscape Core inputs.
// Implementations must not call Ozon/OpenAI; only sealed Factual reads.
type CoreDataSource interface {
	LoadCoreFeed(ctx context.Context, sealed factualapi.FactualReadContext, universeID, universeHash string, profiles CoreProfileBundleV3) (CoreFeed, error)
}

// DetailDataSource supplies bounded supporting context for selected entities.
type DetailDataSource interface {
	LoadSKUDetailExtras(ctx context.Context, sealed factualapi.FactualReadContext, skuIDs []int64, profile ExecutableProfileV3) (map[int64]SKUDetailExtras, error)
	LoadCampaignDetailExtras(ctx context.Context, sealed factualapi.FactualReadContext, campaignIDs []int64, profile ExecutableProfileV3) (map[int64]CampaignDetailExtras, error)
	LoadCategoryDetailExtras(ctx context.Context, sealed factualapi.FactualReadContext, categoryIDs []int64, profile ExecutableProfileV3) (map[int64]CategoryDetailExtras, error)
}

// CoreFeed is the batch payload assembled for one Core LandscapeSet.
type CoreFeed struct {
	DomainWindows      []DomainWindow
	GlobalCapabilities []CapabilityFlag
	GlobalLimitations  []Limitation
	Account            AccountLandscape
	Categories         []CategoryLandscape
	SKUs               []SKULandscape
	Campaigns          []CampaignLandscape
}

type SKUDetailExtras struct {
	Funnel           *SKUFunnelBlock
	CampaignContext  SupportingContext[CampaignContextItem]
	PromotionContext SupportingContext[PromotionContextItem]
	QueryContext     SupportingContext[SearchQueryContextItem]
}

type CategoryDetailExtras struct {
	Funnel     *CategoryFunnelBlock
	Search     *CategorySearchBlock
	Promotions *CategoryPromotionsBlock
}

type CampaignDetailExtras struct {
	SKUContext       SupportingContext[SKUAdsContextItem]
	AssignedSKUs     []int64
	AssignedTotal    int64
	ContextTruncated bool
	Stocks           map[int64]factualapi.SKUStock
}

// Ensure TruncatedList metadata is self-consistent after a slice.
func ApplyTruncation[T any](items []T, totalKnown int64, limit int, sortBasis, semanticScope string) TruncatedList[T] {
	if limit < 0 {
		limit = 0
	}
	if totalKnown < int64(len(items)) {
		totalKnown = int64(len(items))
	}
	out := TruncatedList[T]{
		TotalKnown: totalKnown, SortBasis: sortBasis, SemanticScope: semanticScope, Items: []T{},
	}
	if limit == 0 {
		out.Returned = 0
		out.Truncated = totalKnown > 0
		return out
	}
	n := len(items)
	if n > limit {
		n = limit
	}
	out.Items = append([]T(nil), items[:n]...)
	out.Returned = int64(n)
	out.Truncated = totalKnown > out.Returned
	return out
}

func emptySupportingContext[T any](sortBasis, semanticScope string) SupportingContext[T] {
	return SupportingContext[T]{
		State:         BlockReadyEmpty,
		Items:         []T{},
		TotalCount:    0,
		ReturnedCount: 0,
		Truncated:     false,
		SortBasis:     sortBasis,
		SemanticScope: semanticScope,
	}
}

func unavailableSupportingContext[T any](sortBasis, semanticScope, code string) SupportingContext[T] {
	return SupportingContext[T]{
		State:         BlockUnavailable,
		Items:         nil,
		TotalCount:    0,
		ReturnedCount: 0,
		Truncated:     false,
		SortBasis:     sortBasis,
		SemanticScope: semanticScope,
		Limitations:   []Limitation{{Code: code, Message: "supporting context unavailable"}},
	}
}

func supportingFromTruncation[T any](tl TruncatedList[T]) SupportingContext[T] {
	state := BlockReady
	if len(tl.Items) == 0 && tl.TotalKnown == 0 {
		state = BlockReadyEmpty
	}
	return SupportingContext[T]{
		State:         state,
		Items:         tl.Items,
		TotalCount:    tl.TotalKnown,
		ReturnedCount: tl.Returned,
		Truncated:     tl.Truncated,
		SortBasis:     tl.SortBasis,
		SemanticScope: tl.SemanticScope,
	}
}

func supportingWithPeriod[T any](tl TruncatedList[T], windowRef string, period *DateRange) SupportingContext[T] {
	out := supportingFromTruncation(tl)
	out.WindowRef = windowRef
	out.Period = period
	return out
}

func supportingWithAsOf[T any](tl TruncatedList[T], windowRef string, asOf *time.Time) SupportingContext[T] {
	out := supportingFromTruncation(tl)
	out.WindowRef = windowRef
	out.AsOf = cloneTimePtr(asOf)
	return out
}

func periodToDateRange(p factualapi.Period) *DateRange {
	return &DateRange{
		From: canonicaljson.Date(p.From),
		To:   canonicaljson.Date(p.To),
	}
}

func truncatedFromSupporting[T any](sc SupportingContext[T]) *TruncatedList[T] {
	if sc.State == BlockUnavailable || sc.State == BlockNotApplicable || sc.State == BlockUnknown {
		return nil
	}
	tl := TruncatedList[T]{
		Items:         sc.Items,
		TotalKnown:    sc.TotalCount,
		Returned:      sc.ReturnedCount,
		Truncated:     sc.Truncated,
		SortBasis:     sc.SortBasis,
		SemanticScope: sc.SemanticScope,
	}
	return &tl
}

func ApplySupportingTruncation[T any](sc SupportingContext[T], limit int, sortBasis, semanticScope string) SupportingContext[T] {
	if sc.State == BlockUnavailable || sc.State == BlockNotApplicable || sc.State == BlockUnknown {
		return sc
	}
	tl := ApplyTruncation(sc.Items, sc.TotalCount, limit, nz(sc.SortBasis, sortBasis), nz(sc.SemanticScope, semanticScope))
	out := supportingFromTruncation(tl)
	out.WindowRef = sc.WindowRef
	out.Period = cloneDateRange(sc.Period)
	out.AsOf = cloneTimePtr(sc.AsOf)
	out.Limitations = append(out.Limitations, sc.Limitations...)
	return out
}
