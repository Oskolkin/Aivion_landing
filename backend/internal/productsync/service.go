package productsync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/descriptioncategories"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/rawpayloads"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncstate"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ozonProductAPI interface {
	ListProducts(ctx context.Context, clientID, apiKey string, req ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error)
	ListProductsInfo(ctx context.Context, clientID, apiKey string, req ozon.ProductInfoListRequest) (*ozon.TypedResponse[ozon.ProductInfoListResult], error)
}

type Service struct {
	pool            *pgxpool.Pool
	queries         *dbgen.Queries
	ozonService     *ozon.Service
	productAPI      ozonProductAPI
	rawPayloads     *rawpayloads.Service
	cursorService   *syncstate.SyncCursorService
	defaultPageSize int
	maxPagesPerRun  int
}

func NewService(
	db *pgxpool.Pool,
	ozonService *ozon.Service,
	rawPayloads *rawpayloads.Service,
) *Service {
	return &Service{
		pool:            db,
		queries:         dbgen.New(db),
		ozonService:     ozonService,
		productAPI:      ozon.NewClient(),
		rawPayloads:     rawPayloads,
		cursorService:   syncstate.NewSyncCursorService(db),
		defaultPageSize: 1000,
		maxPagesPerRun:  1000,
	}
}

type RunInput struct {
	SellerAccountID int64
	ImportJobID     int64
	SourceCursor    string
	// SyncModeFull (default) and SyncModeIncremental both paginate until the list ends from SourceCursor.
	SyncMode string
}

type RunResult struct {
	RecordsReceived int32
	RecordsImported int32
	RecordsFailed   int32
	PagesFetched    int32
	// NextCursorValue is passed to import job completion. Empty when the sync cursor was advanced
	// after each page inside Run (products domain) to avoid double AdvanceCursor.
	NextCursorValue string
	// FinalPageLastID is the effective Ozon last_id after the last processed page (for logs/metrics).
	FinalPageLastID string
}

func (s *Service) Run(ctx context.Context, input RunInput) (RunResult, error) {
	mode := strings.TrimSpace(input.SyncMode)
	if mode == "" {
		mode = SyncModeFull
	}
	if mode != SyncModeFull && mode != SyncModeIncremental {
		return RunResult{}, fmt.Errorf("unsupported products sync mode: %s", input.SyncMode)
	}

	creds, err := s.ozonService.GetDecryptedCredentials(ctx, input.SellerAccountID)
	if err != nil {
		return RunResult{}, fmt.Errorf("get decrypted ozon credentials: %w", err)
	}

	var (
		requestLastID  = strings.TrimSpace(input.SourceCursor)
		totalReceived  int32
		totalImported  int32
		pagesFetched   int32
		finalEffective string
	)

	for {
		if int(pagesFetched) >= s.maxPagesPerRun {
			return RunResult{
				RecordsReceived: totalReceived,
				RecordsImported: totalImported,
				RecordsFailed:   0,
				PagesFetched:    pagesFetched,
				NextCursorValue: "",
				FinalPageLastID: finalEffective,
			}, errMaxPagesExceeded(requestLastID, pagesFetched, totalReceived, totalImported)
		}
		pagesFetched++

		req := ozon.ListProductsRequest{
			Limit:  s.defaultPageSize,
			LastID: requestLastID,
		}

		resp, err := s.productAPI.ListProducts(ctx, creds.ClientID, creds.APIKey, req)
		if err != nil {
			return RunResult{}, fmt.Errorf("fetch products from ozon (page %d): %w", pagesFetched, err)
		}

		items := resp.Data.Items
		effectiveLast := effectiveListLastID(items, resp.Data.LastID)

		requestKey := buildRequestKey(pagesFetched, requestLastID)
		if _, err := s.rawPayloads.Save(ctx, rawpayloads.SaveInput{
			SellerAccountID: input.SellerAccountID,
			ImportJobID:     input.ImportJobID,
			Domain:          "products",
			Source:          "ozon.v3.product.list",
			RequestKey:      requestKey,
			Body:            resp.Raw,
		}); err != nil {
			return RunResult{}, fmt.Errorf("save raw products payload page %d: %w", pagesFetched, err)
		}

		productIDs := make([]int64, 0, len(items))
		for _, item := range items {
			if item.ID > 0 {
				productIDs = append(productIDs, item.ID)
			}
		}

		detailsByID := make(map[int64]ozon.ProductInfoListItem, len(productIDs))
		if len(productIDs) > 0 {
			detailsResp, err := s.productAPI.ListProductsInfo(ctx, creds.ClientID, creds.APIKey, ozon.ProductInfoListRequest{
				ProductID: productIDs,
			})
			if err != nil {
				return RunResult{}, fmt.Errorf("fetch products details from ozon (page %d): %w", pagesFetched, err)
			}
			for _, detail := range detailsResp.Data.Items {
				detailsByID[detail.ID] = detail
			}
		}

		pageImported := int32(0)
		for _, item := range items {
			detail := detailsByID[item.ID]
			name := item.Name
			if detail.Name != "" {
				name = detail.Name
			}
			offerID := item.OfferID
			if detail.OfferID != "" {
				offerID = detail.OfferID
			}
			sku := item.SKU
			if detail.SKU != 0 {
				sku = detail.SKU
			}
			rawSubset, err := json.Marshal(item)
			if err != nil {
				return RunResult{}, fmt.Errorf("marshal product raw subset: %w", err)
			}

			updatedAtRaw := item.UpdatedAt
			if detail.UpdatedAt != "" {
				updatedAtRaw = detail.UpdatedAt
			}
			sourceUpdatedAt, err := parseOptionalRFC3339(updatedAtRaw)
			if err != nil {
				return RunResult{}, fmt.Errorf("parse product updated_at: %w", err)
			}

			primaryImageURL := ozon.PickPrimaryImageURL(detail.PrimaryImage, detail.Images)
			if _, err := s.queries.UpsertProduct(ctx, dbgen.UpsertProductParams{
				SellerAccountID:       input.SellerAccountID,
				OzonProductID:         item.ID,
				OfferID:               nullableText(offerID),
				Sku:                   nullableInt64(sku),
				Name:                  fallbackName(name),
				Status:                nullableText(resolveStatus(item)),
				ReferencePrice:        nullableNumeric(detail.Price),
				OldPrice:              nullableNumeric(detail.OldPrice),
				OzonMinPrice:          nullableNumeric(detail.MinPrice),
				DescriptionCategoryID: nullableInt64(detail.DescriptionCategoryID),
				IsArchived:            item.IsArchived || item.Archived || detail.IsArchived || detail.Archived,
				RawAttributes:         rawSubset,
				PrimaryImageUrl:       nullableText(primaryImageURL),
				ImageUrls:             ozon.ImageURLsJSON(detail.Images),
				SourceUpdatedAt:       sourceUpdatedAt,
			}); err != nil {
				return RunResult{}, fmt.Errorf("upsert product ozon_product_id=%d: %w", item.ID, err)
			}
			pageImported++
		}

		totalReceived += int32(len(items))
		totalImported += pageImported

		if s.cursorService != nil && effectiveLast != "" {
			if _, err := s.cursorService.AdvanceCursor(ctx, input.SellerAccountID, "products", effectiveLast); err != nil {
				return RunResult{}, fmt.Errorf("advance products cursor after page %d: %w", pagesFetched, err)
			}
		}

		finalEffective = effectiveLast

		if !anotherProductsPage(len(items), s.defaultPageSize, requestLastID, effectiveLast) {
			break
		}
		requestLastID = effectiveLast
	}

	// Refresh global description category dictionary in the background (throttled; must not block product import).
	go func(clientID, apiKey string, dbpool *pgxpool.Pool) {
		ctx2, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		treeClient := ozon.NewClient()
		_, _ = descriptioncategories.SyncGlobalIfStale(ctx2, dbpool, treeClient, clientID, apiKey, 24*time.Hour)
	}(creds.ClientID, creds.APIKey, s.pool)

	return RunResult{
		RecordsReceived: totalReceived,
		RecordsImported: totalImported,
		RecordsFailed:   0,
		PagesFetched:    pagesFetched,
		NextCursorValue: "",
		FinalPageLastID: finalEffective,
	}, nil
}

func buildRequestKey(page int32, requestLastID string) string {
	if requestLastID == "" {
		return fmt.Sprintf("ozon.v3.product.list:page:%d:last_id:", page)
	}
	return fmt.Sprintf("ozon.v3.product.list:page:%d:last_id:%s", page, requestLastID)
}

func resolveStatus(item ozon.ProductItem) string {
	if item.Status != "" {
		return item.Status
	}
	return item.State
}

func fallbackName(v string) string {
	if v == "" {
		return "unnamed product"
	}
	return v
}

func nullableText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{
		String: v,
		Valid:  true,
	}
}

func nullableInt64(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{
		Int64: v,
		Valid: true,
	}
}

func nullableNumeric(v string) pgtype.Numeric {
	if v == "" {
		return pgtype.Numeric{Valid: false}
	}
	var parsed pgtype.Numeric
	if err := parsed.Scan(v); err != nil {
		return pgtype.Numeric{Valid: false}
	}
	return parsed
}

func parseOptionalRFC3339(v string) (pgtype.Timestamptz, error) {
	if v == "" {
		return pgtype.Timestamptz{Valid: false}, nil
	}

	parsed, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return pgtype.Timestamptz{}, err
	}

	return pgtype.Timestamptz{
		Time:  parsed,
		Valid: true,
	}, nil
}
