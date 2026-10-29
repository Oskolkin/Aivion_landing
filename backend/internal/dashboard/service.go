package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actionview"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/projection"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo *analysispg.Repository
}

func NewService(repo *analysispg.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Home(ctx context.Context, sellerID int64) (HomeResponse, error) {
	pub, err := s.repo.GetCurrentPublication(ctx, sellerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return HomeResponse{
			State: "PRE_ANALYSIS",
			PreAnalysis: &PreAnalysis{
				ReasonCode: "NO_CURRENT_PUBLICATION",
				MessageKey: "dashboard.pre_analysis.no_current_publication",
			},
		}, nil
	}
	if err != nil {
		return HomeResponse{}, err
	}
	ovRaw, err := s.repo.GetProjection(ctx, pub.ID, projection.KeyOverview)
	if err != nil {
		return HomeResponse{State: "FAILED", Error: &ProductError{ReasonCode: "PROJECTION_CORRUPT", MessageKey: "dashboard.error.projection_corrupt"}}, nil
	}
	fnRaw, err := s.repo.GetProjection(ctx, pub.ID, projection.KeyFunnel)
	if err != nil {
		return HomeResponse{State: "FAILED", Error: &ProductError{ReasonCode: "PROJECTION_CORRUPT", MessageKey: "dashboard.error.projection_corrupt"}}, nil
	}
	stRaw, err := s.repo.GetProjection(ctx, pub.ID, projection.KeyStock)
	if err != nil {
		return HomeResponse{State: "FAILED", Error: &ProductError{ReasonCode: "PROJECTION_CORRUPT", MessageKey: "dashboard.error.projection_corrupt"}}, nil
	}
	adRaw, err := s.repo.GetProjection(ctx, pub.ID, projection.KeyAdvertising)
	if err != nil {
		return HomeResponse{State: "FAILED", Error: &ProductError{ReasonCode: "PROJECTION_CORRUPT", MessageKey: "dashboard.error.projection_corrupt"}}, nil
	}
	var ov projection.OverviewV1DTO
	var fn projection.FunnelV1DTO
	var st, ad projection.DomainPageDTO
	if json.Unmarshal(ovRaw, &ov) != nil || json.Unmarshal(fnRaw, &fn) != nil || json.Unmarshal(stRaw, &st) != nil || json.Unmarshal(adRaw, &ad) != nil {
		return HomeResponse{State: "FAILED", Error: &ProductError{ReasonCode: "PROJECTION_CORRUPT", MessageKey: "dashboard.error.projection_corrupt"}}, nil
	}
	groups, err := s.repo.ListActionGroupsOverlay(ctx, sellerID, pub.ID)
	if err != nil {
		return HomeResponse{}, err
	}
	focus := make([]FocusGroup, 0)
	for _, g := range groups {
		if g.ActiveMemberCount == 0 {
			continue
		}
		focus = append(focus, FocusGroup{
			ActionViewID:   actionview.EncodeGroup(g.ID.String()),
			DisplayOrder:   g.DisplayOrder,
			DisplayTitle:   g.DisplayTitle,
			DisplaySummary: g.Summary,
			ActionType:     g.ActionType,
			Family:         g.Family,
			ActiveMembers:  g.ActiveMemberCount,
		})
		if len(focus) >= 5 {
			break
		}
	}
	pid := strconv.FormatInt(pub.ID, 10)
	return HomeResponse{
		State: "READY",
		Publication: &PublicationMeta{
			ID: pid, AnalysisDate: pub.AnalysisDate.UTC().Format("2006-01-02"),
			AnalysisCutoffAt: pub.CutoffAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			PublishedAt:      pub.PublishedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		},
		Periods: map[string]any{
			"current_7d":  map[string]string{"key": "current_7d"},
			"previous_7d": map[string]string{"key": "previous_7d"},
			"trend_30d":   map[string]string{"key": "trend_30d"},
		},
		Limitations: ov.Limitations,
		Overview:    &ov,
		Funnel:      &fn,
		Stock:       &st,
		Advertising: &ad,
		Focus:       focus,
	}, nil
}

func (s *Service) StockSKUs(ctx context.Context, sellerID, publicationID int64, cursor, band string, noSales *bool, limit int) (RowsResponse, error) {
	return s.listRows(ctx, sellerID, publicationID, "publication_stock_sku_rows", "sku_id", cursor, band, noSales, limit)
}

func (s *Service) AdCampaigns(ctx context.Context, sellerID, publicationID int64, cursor string, limit int) (RowsResponse, error) {
	return s.listRows(ctx, sellerID, publicationID, "publication_ad_campaign_rows", "campaign_id", cursor, "", nil, limit)
}

func (s *Service) listRows(ctx context.Context, sellerID, publicationID int64, table, idCol, cursor, band string, noSales *bool, limit int) (RowsResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	page, err := s.repo.ListPinnedRows(ctx, sellerID, publicationID, table, idCol, cursor, band, noSales, limit+1)
	if errors.Is(err, pgx.ErrNoRows) {
		return RowsResponse{}, err
	}
	if err != nil {
		return RowsResponse{}, err
	}
	hasMore := len(page) > limit
	if hasMore {
		page = page[:limit]
	}
	items := make([]map[string]any, 0, len(page))
	next := ""
	for _, row := range page {
		var m map[string]any
		_ = json.Unmarshal(row.JSON, &m)
		if m == nil {
			m = map[string]any{}
		}
		items = append(items, m)
		next = row.ID
	}
	return RowsResponse{
		PublicationID: strconv.FormatInt(publicationID, 10),
		Items:         items,
		NextCursor:    next,
		HasMore:       hasMore,
	}, nil
}
