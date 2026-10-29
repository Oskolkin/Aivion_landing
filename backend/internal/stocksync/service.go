package stocksync

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/rawpayloads"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockanalyticssync"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	queries       *dbgen.Queries
	ozonService   *ozon.Service
	ozonClient    *ozon.Client
	rawPayloads   *rawpayloads.Service
	analyticsSync *stockanalyticssync.Service
}

type RunInput struct {
	SellerAccountID int64
	ImportJobID     int64
	SourceCursor    string
}

type RunResult struct {
	RecordsReceived int32
	RecordsImported int32
	RecordsFailed   int32
	NextCursorValue string
}

func NewService(
	db *pgxpool.Pool,
	ozonService *ozon.Service,
	rawPayloads *rawpayloads.Service,
) *Service {
	return &Service{
		queries:       dbgen.New(db),
		ozonService:   ozonService,
		ozonClient:    ozon.NewClient(),
		rawPayloads:   rawPayloads,
		analyticsSync: stockanalyticssync.NewService(db, ozonService, rawPayloads),
	}
}

func (s *Service) Run(ctx context.Context, input RunInput) (RunResult, error) {
	creds, err := s.ozonService.GetDecryptedCredentials(ctx, input.SellerAccountID)
	if err != nil {
		return RunResult{}, fmt.Errorf("get decrypted ozon credentials: %w", err)
	}

	snapshotAt := time.Now().UTC().Truncate(time.Second)

	received := int32(0)
	imported := int32(0)
	cursor := ""
	for {
		req := ozon.ListStocksRequest{
			Cursor: cursor,
			Limit:  1000,
		}

		resp, err := s.ozonClient.ListStocks(ctx, creds.ClientID, creds.APIKey, req)
		if err != nil {
			return RunResult{}, fmt.Errorf("fetch stocks from ozon: %w", err)
		}

		if _, err := s.rawPayloads.Save(ctx, rawpayloads.SaveInput{
			SellerAccountID: input.SellerAccountID,
			ImportJobID:     input.ImportJobID,
			Domain:          "stocks",
			Source:          "ozon.v1.product.info.warehouse.stocks",
			RequestKey:      buildStocksRequestKey(req.Cursor),
			Body:            resp.Raw,
		}); err != nil {
			return RunResult{}, fmt.Errorf("save raw stocks payload: %w", err)
		}

		received += int32(len(resp.Data.Stocks))

		for _, item := range resp.Data.Stocks {
			rawSubset, err := json.Marshal(item)
			if err != nil {
				return RunResult{}, fmt.Errorf("marshal stock raw subset: %w", err)
			}

			productExternalID := strconv.FormatInt(item.ProductID, 10)
			warehouseExternalID := strconv.FormatInt(item.WarehouseID, 10)

			available := item.FreeStock
			if available == 0 {
				available = item.Present - item.Reserved
				if available < 0 {
					available = 0
				}
			}

			stockSnapshotAt := snapshotAt
			if item.UpdatedAt != "" {
				if parsed, err := time.Parse(time.RFC3339, item.UpdatedAt); err == nil {
					stockSnapshotAt = parsed.UTC()
				}
			}

			if _, err := s.queries.UpsertStock(ctx, dbgen.UpsertStockParams{
				SellerAccountID:     input.SellerAccountID,
				ProductExternalID:   productExternalID,
				WarehouseExternalID: warehouseExternalID,
				QuantityTotal:       nullableInt32(item.Present),
				QuantityReserved:    nullableInt32(item.Reserved),
				QuantityAvailable:   nullableInt32(available),
				SnapshotAt: pgtype.Timestamptz{
					Time:  stockSnapshotAt,
					Valid: true,
				},
				RawAttributes: rawSubset,
			}); err != nil {
				return RunResult{}, fmt.Errorf(
					"upsert stock product_id=%d warehouse_id=%d: %w",
					item.ProductID,
					item.WarehouseID,
					err,
				)
			}

			imported++
		}

		if !resp.Data.HasNext || resp.Data.Cursor == "" {
			break
		}
		cursor = resp.Data.Cursor
	}

	if s.analyticsSync != nil {
		if _, err := s.analyticsSync.Run(ctx, stockanalyticssync.RunInput{
			SellerAccountID: input.SellerAccountID,
			ImportJobID:     input.ImportJobID,
			SnapshotDate:    snapshotAt,
		}); err != nil {
			return RunResult{}, fmt.Errorf("sync ozon analytics stocks: %w", err)
		}
	}

	return RunResult{
		RecordsReceived: received,
		RecordsImported: imported,
		RecordsFailed:   0,
		NextCursorValue: snapshotAt.Format(time.RFC3339),
	}, nil
}

func buildStocksRequestKey(cursor string) string {
	if cursor == "" {
		return "stocks:warehouse:initial"
	}
	return "stocks:warehouse:cursor:" + cursor
}

func nullableInt32(v int32) pgtype.Int4 {
	return pgtype.Int4{
		Int32: v,
		Valid: true,
	}
}
