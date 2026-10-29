package stockanalyticssync

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/rawpayloads"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const analyticsStocksBatchSize = 100

type Service struct {
	queries     *dbgen.Queries
	ozonService *ozon.Service
	ozonClient  *ozon.Client
	rawPayloads *rawpayloads.Service
}

type RunInput struct {
	SellerAccountID int64
	ImportJobID     int64
	SnapshotDate    time.Time
}

type RunResult struct {
	RecordsReceived int32
	RecordsImported int32
}

func NewService(
	db *pgxpool.Pool,
	ozonService *ozon.Service,
	rawPayloads *rawpayloads.Service,
) *Service {
	return &Service{
		queries:     dbgen.New(db),
		ozonService: ozonService,
		ozonClient:  ozon.NewClient(),
		rawPayloads: rawPayloads,
	}
}

func (s *Service) Run(ctx context.Context, input RunInput) (RunResult, error) {
	creds, err := s.ozonService.GetDecryptedCredentials(ctx, input.SellerAccountID)
	if err != nil {
		return RunResult{}, fmt.Errorf("get decrypted ozon credentials: %w", err)
	}

	snapshotDate := input.SnapshotDate.UTC()
	if snapshotDate.IsZero() {
		snapshotDate = time.Now().UTC()
	}
	snapshotDate = time.Date(snapshotDate.Year(), snapshotDate.Month(), snapshotDate.Day(), 0, 0, 0, 0, time.UTC)

	products, err := s.queries.ListProductsBySellerAccountID(ctx, input.SellerAccountID)
	if err != nil {
		return RunResult{}, fmt.Errorf("list products for analytics stocks: %w", err)
	}

	skuStrings := make([]string, 0, len(products))
	for _, p := range products {
		if !p.Sku.Valid || p.Sku.Int64 <= 0 {
			continue
		}
		skuStrings = append(skuStrings, strconv.FormatInt(p.Sku.Int64, 10))
	}
	if len(skuStrings) == 0 {
		return RunResult{}, nil
	}

	received := int32(0)
	imported := int32(0)
	pgDate := pgtype.Date{Time: snapshotDate, Valid: true}

	for start := 0; start < len(skuStrings); start += analyticsStocksBatchSize {
		end := start + analyticsStocksBatchSize
		if end > len(skuStrings) {
			end = len(skuStrings)
		}
		batch := skuStrings[start:end]

		resp, err := s.ozonClient.ListAnalyticsStocks(ctx, creds.ClientID, creds.APIKey, ozon.AnalyticsStocksRequest{
			SKUs: batch,
		})
		if err != nil {
			return RunResult{}, fmt.Errorf("fetch analytics stocks batch start=%d: %w", start, err)
		}

		var rawPayloadID pgtype.Int8
		if s.rawPayloads != nil && input.ImportJobID > 0 {
			saved, saveErr := s.rawPayloads.Save(ctx, rawpayloads.SaveInput{
				SellerAccountID: input.SellerAccountID,
				ImportJobID:     input.ImportJobID,
				Domain:          "stocks",
				Source:          "ozon.v1.analytics.stocks",
				RequestKey:      fmt.Sprintf("analytics_stocks:%s:%d", snapshotDate.Format("2006-01-02"), start),
				Body:            resp.Raw,
			})
			if saveErr == nil && saved.ID > 0 {
				rawPayloadID = pgtype.Int8{Int64: saved.ID, Valid: true}
			}
		}

		received += int32(len(resp.Data))
		for _, item := range resp.Data {
			if item.SKU <= 0 {
				continue
			}
			warehouseID := item.WarehouseID
			if _, err := s.queries.UpsertStockAnalyticsSnapshot(ctx, dbgen.UpsertStockAnalyticsSnapshotParams{
				SellerAccountID:              input.SellerAccountID,
				SnapshotDate:                 pgDate,
				Sku:                          item.SKU,
				OfferID:                      nullableText(item.OfferID),
				ProductName:                  nullableText(item.Name),
				WarehouseID:                  warehouseID,
				WarehouseName:                nullableText(item.WarehouseName),
				ClusterID:                    item.ClusterID,
				AvailableStockCount:          item.AvailableStockCount,
				ExcessStockCount:             item.ExcessStockCount,
				Ads:                          nullableNumericFromFloat(item.ADS),
				Idc:                          nullableInt32FromFloat(item.IDC),
				TurnoverGrade:                nullableText(item.TurnoverGrade),
				TurnoverGradeCluster:         nullableText(item.TurnoverGradeCluster),
				DaysWithoutSales:             nullableInt32(item.DaysWithoutSales),
				RequestedStockCount:          nullableInt32(item.RequestedStockCount),
				TransitStockCount:            nullableInt32(item.TransitStockCount),
				ValidStockCount:              nullableInt32(item.ValidStockCount),
				WaitingDocsStockCount:        nullableInt32(item.WaitingDocsStockCount),
				ReturnFromCustomerStockCount: nullableInt32(item.ReturnFromCustomerStockCount),
				ReturnToSellerStockCount:     nullableInt32(item.ReturnToSellerStockCount),
				ItemTagsJson:                 jsonTags(item.ItemTags),
				RawPayloadID:                 rawPayloadID,
			}); err != nil {
				return RunResult{}, fmt.Errorf("upsert stock analytics snapshot sku=%d: %w", item.SKU, err)
			}
			imported++
		}
	}

	return RunResult{RecordsReceived: received, RecordsImported: imported}, nil
}

func nullableText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: v, Valid: true}
}

func nullableInt32FromFloat(v float64) pgtype.Int4 {
	if v == 0 {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: int32(v), Valid: true}
}

func nullableInt32(v int32) pgtype.Int4 {
	if v == 0 {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: v, Valid: true}
}

func nullableNumericFromFloat(v float64) pgtype.Numeric {
	if v == 0 {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%f", v))
	return n
}

func jsonTags(tags []string) []byte {
	raw, err := json.Marshal(tags)
	if err != nil || len(raw) == 0 {
		return []byte("[]")
	}
	return raw
}
