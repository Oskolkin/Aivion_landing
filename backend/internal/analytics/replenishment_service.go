package analytics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReplenishmentService struct {
	queries      *dbgen.Queries
	stocksViewer *StocksViewService
}

func NewReplenishmentService(db *pgxpool.Pool) *ReplenishmentService {
	return &ReplenishmentService{
		queries:      dbgen.New(db),
		stocksViewer: NewStocksViewService(db),
	}
}

type ReplenishmentSKUItem struct {
	OzonProductID int64   `json:"ozon_product_id"`
	OfferID       *string `json:"offer_id"`
	SKU           *int64  `json:"sku"`
	ProductName   *string `json:"product_name"`

	CurrentTotalStock     int32    `json:"current_total_stock"`
	CurrentReservedStock  int32    `json:"current_reserved_stock"`
	CurrentAvailableStock int32    `json:"current_available_stock"`
	DaysOfCover           *float64 `json:"days_of_cover"`
	SnapshotAt            *string  `json:"snapshot_at"`
	WarehouseCount        int32    `json:"warehouse_count"`

	Importance            float64  `json:"importance"`
	DepletionRisk         string   `json:"depletion_risk"`
	ReplenishmentPriority string   `json:"replenishment_priority"`
	Signals               []string `json:"signals"`
}

type ReplenishmentResult struct {
	SellerAccountID int64                  `json:"seller_account_id"`
	AsOfDate        string                 `json:"as_of_date"`
	LastStockUpdate *string                `json:"last_stock_update"`
	StockSemantics  string                 `json:"stock_semantics"`
	PriorityRule    string                 `json:"priority_rule"`
	Items           []ReplenishmentSKUItem `json:"items"`
}

func (s *ReplenishmentService) ListReplenishmentForSellerAccount(
	ctx context.Context,
	sellerAccountID int64,
	asOfDate time.Time,
) (ReplenishmentResult, error) {
	asOf := normalizeDate(asOfDate)

	if _, err := s.queries.GetSellerAccountByID(ctx, sellerAccountID); err != nil {
		return ReplenishmentResult{}, fmt.Errorf("get seller account: %w", err)
	}

	skuRows, err := s.queries.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(asOf),
		MetricDate_2:    dateValue(asOf),
	})
	if err != nil {
		return ReplenishmentResult{}, fmt.Errorf("list sku metrics for as_of date: %w", err)
	}

	var accountRevenue float64
	skuByProduct := make(map[int64]dbgen.DailySkuMetric, len(skuRows))
	for _, row := range skuRows {
		skuByProduct[row.OzonProductID] = row
		accountRevenue += numericToFloatCritical(row.Revenue)
	}
	accountRevenue = round2Critical(accountRevenue)

	stockRows, err := s.stocksViewer.ListCurrentStocksBySellerAccount(ctx, sellerAccountID)
	if err != nil {
		return ReplenishmentResult{}, fmt.Errorf("list current stocks: %w", err)
	}
	var lastStockUpdate *string
	for _, row := range stockRows {
		lastStockUpdate = maxRFC3339Ptr(lastStockUpdate, row.SnapshotAt)
	}

	// Build union based on stock rows; stocks & replenishment screen is stock-first.
	items := make([]ReplenishmentSKUItem, 0, len(stockRows))
	for _, stock := range stockRows {
		skuMetric, ok := skuByProduct[stock.OzonProductID]
		daysCover := (*float64)(nil)
		revenue := 0.0
		if ok {
			daysCover = numericToFloatPtrCritical(skuMetric.DaysOfCover)
			revenue = numericToFloatCritical(skuMetric.Revenue)
		}

		importance, _ := calcImportance(revenue, accountRevenue, 0)
		depletionRisk, riskSignals := classifyDepletionRisk(stock.AvailableStock, daysCover)
		priority, prioritySignals := classifyReplenishmentPriority(stock.AvailableStock, daysCover, importance)

		signals := append(riskSignals, prioritySignals...)

		items = append(items, ReplenishmentSKUItem{
			OzonProductID:         stock.OzonProductID,
			OfferID:               stock.OfferID,
			SKU:                   stock.SKU,
			ProductName:           stock.ProductName,
			CurrentTotalStock:     stock.TotalStock,
			CurrentReservedStock:  stock.ReservedStock,
			CurrentAvailableStock: stock.AvailableStock,
			DaysOfCover:           daysCover,
			SnapshotAt:            stock.SnapshotAt,
			WarehouseCount:        stock.WarehouseCount,
			Importance:            importance,
			DepletionRisk:         depletionRisk,
			ReplenishmentPriority: priority,
			Signals:               uniqStrings(signals),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if priorityRank(left.ReplenishmentPriority) == priorityRank(right.ReplenishmentPriority) {
			if riskRank(left.depletionRiskSafe()) == riskRank(right.depletionRiskSafe()) {
				if left.Importance == right.Importance {
					return left.OzonProductID < right.OzonProductID
				}
				return left.Importance > right.Importance
			}
			return riskRank(left.depletionRiskSafe()) > riskRank(right.depletionRiskSafe())
		}
		return priorityRank(left.ReplenishmentPriority) > priorityRank(right.ReplenishmentPriority)
	})

	return ReplenishmentResult{
		SellerAccountID: sellerAccountID,
		AsOfDate:        asOf.Format("2006-01-02"),
		LastStockUpdate: lastStockUpdate,
		StockSemantics:  "Current operational snapshot from latest product+warehouse rows in DB; not stock history stream or strict realtime.",
		PriorityRule:    "high: (stock=0 or days_of_cover<=3) and importance>=0.5; medium: days_of_cover<=7 and importance>=0.3; else low.",
		Items:           items,
	}, nil
}

func classifyDepletionRisk(stockAvailable int32, daysOfCover *float64) (string, []string) {
	switch {
	case stockAvailable <= 0:
		return "critical_out_of_stock", []string{"out_of_stock"}
	case daysOfCover != nil && *daysOfCover <= 3:
		return "high_depletion_risk", []string{"days_of_cover_le_3"}
	case stockAvailable <= 3 || (daysOfCover != nil && *daysOfCover <= 7):
		return "medium_depletion_risk", []string{"low_stock_or_cover_le_7"}
	default:
		return "low_depletion_risk", []string{"stock_is_sufficient"}
	}
}

func classifyReplenishmentPriority(stockAvailable int32, daysOfCover *float64, importance float64) (string, []string) {
	isHighStockProblem := stockAvailable == 0 || (daysOfCover != nil && *daysOfCover <= 3)
	if isHighStockProblem && importance >= 0.5 {
		return "high", []string{"priority_high"}
	}

	if daysOfCover != nil && *daysOfCover <= 7 && importance >= 0.3 {
		return "medium", []string{"priority_medium"}
	}

	return "low", []string{"priority_low"}
}

func priorityRank(priority string) int {
	switch priority {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

func riskRank(risk string) int {
	switch risk {
	case "critical_out_of_stock":
		return 4
	case "high_depletion_risk":
		return 3
	case "medium_depletion_risk":
		return 2
	default:
		return 1
	}
}

func (i ReplenishmentSKUItem) depletionRiskSafe() string {
	if i.DepletionRisk == "" {
		return "low_depletion_risk"
	}
	return i.DepletionRisk
}
