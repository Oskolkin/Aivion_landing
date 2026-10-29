package analytics

import (
	"context"
	"fmt"
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StocksViewService builds current stocks table view for analytics.
// It is read-only and separated from ingestion/runtime concerns.
type StocksViewService struct {
	queries *dbgen.Queries
}

func NewStocksViewService(db *pgxpool.Pool) *StocksViewService {
	return &StocksViewService{
		queries: dbgen.New(db),
	}
}

type CurrentStockWarehouseRow struct {
	WarehouseExternalID string  `json:"warehouse_external_id"`
	TotalStock          int32   `json:"total_stock"`
	ReservedStock       int32   `json:"reserved_stock"`
	AvailableStock      int32   `json:"available_stock"`
	SnapshotAt          *string `json:"snapshot_at"`
}

type CurrentStockProductRow struct {
	OzonProductID int64   `json:"ozon_product_id"`
	OfferID       *string `json:"offer_id"`
	SKU           *int64  `json:"sku"`
	ProductName   *string `json:"product_name"`

	WarehouseCount int32   `json:"warehouse_count"`
	TotalStock     int32   `json:"total_stock"`
	ReservedStock  int32   `json:"reserved_stock"`
	AvailableStock int32   `json:"available_stock"`
	SnapshotAt     *string `json:"snapshot_at"`

	Warehouses []CurrentStockWarehouseRow `json:"warehouses"`
}

func (s *StocksViewService) ListCurrentStocksBySellerAccount(ctx context.Context, sellerAccountID int64) ([]CurrentStockProductRow, error) {
	if _, err := s.queries.GetSellerAccountByID(ctx, sellerAccountID); err != nil {
		return nil, fmt.Errorf("get seller account: %w", err)
	}

	summaries, err := s.queries.ListCurrentStockProductSummariesBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list stock product summaries: %w", err)
	}

	warehouses, err := s.queries.ListCurrentStockWarehouseRowsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list stock warehouse rows: %w", err)
	}

	warehouseMap := make(map[int64][]CurrentStockWarehouseRow, len(summaries))
	for _, row := range warehouses {
		warehouseMap[row.OzonProductID] = append(warehouseMap[row.OzonProductID], CurrentStockWarehouseRow{
			WarehouseExternalID: row.WarehouseExternalID,
			TotalStock:          row.TotalStock,
			ReservedStock:       row.ReservedStock,
			AvailableStock:      row.AvailableStock,
			SnapshotAt:          timestamptzToRFC3339(row.SnapshotAt),
		})
	}
	for productID := range warehouseMap {
		sort.Slice(warehouseMap[productID], func(i, j int) bool {
			return warehouseMap[productID][i].WarehouseExternalID < warehouseMap[productID][j].WarehouseExternalID
		})
	}

	result := make([]CurrentStockProductRow, 0, len(summaries))
	for _, row := range summaries {
		result = append(result, CurrentStockProductRow{
			OzonProductID:  row.OzonProductID,
			OfferID:        textPtr(row.OfferID),
			SKU:            int8Ptr(row.Sku),
			ProductName:    textPtr(row.ProductName),
			WarehouseCount: row.WarehouseCount,
			TotalStock:     row.TotalStock,
			ReservedStock:  row.ReservedStock,
			AvailableStock: row.AvailableStock,
			SnapshotAt:     timestamptzToRFC3339(row.SnapshotAt),
			Warehouses:     warehouseMap[row.OzonProductID],
		})
	}

	return result, nil
}
