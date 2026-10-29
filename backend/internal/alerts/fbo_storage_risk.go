package alerts

import (
	"math"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	fboStorageRiskSourceMethod     = "/v1/analytics/stocks"
	fboStorageRiskMetricQuality    = "actual_ozon_stock_analytics"
	fboStorageRiskIDCThresholdDays = 120
	fboStorageRiskIDCHighDays      = 180
	fboStorageRiskIDCCriticalDays  = 240
	fboStorageRiskExcessShareHigh  = 0.50
	fboTurnoverGradeSurplus        = "SURPLUS"

	fboReasonTurnoverSurplus  = "turnover_grade_surplus"
	fboReasonExcessPositive   = "excess_stock_count_positive"
	fboReasonIDCGte120        = "idc_gte_120_days"
	fboReasonIDCGte180        = "idc_gte_180_days"
	fboReasonExcessShareGte50 = "excess_share_gte_50_percent"
)

// FBOStorageRiskSKUContext is SKU-aggregated Ozon stock analytics for surplus storage alerts.
type FBOStorageRiskSKUContext struct {
	SKU                   int64
	OzonProductID         int64
	OfferID               *string
	ProductName           *string
	AvailableStockCount   int32
	ExcessStockCount      int32
	ExcessShare           float64
	HasExcessShare        bool
	ADS                   *float64
	IDC                   *int32
	TurnoverGrade         string
	TurnoverGradeCluster  string
	DaysWithoutSales      *int32
	RequestedStockCount   int32
	TransitStockCount     int32
	ValidStockCount       int32
	WaitingDocsStockCount int32
	WarehouseNames        []string
	WarehouseIDs          []int64
}

// FBOStorageRiskBySKU maps sku to aggregated analytics context.
type FBOStorageRiskBySKU map[int64]FBOStorageRiskSKUContext

// BuildFBOStorageRiskBySKU aggregates warehouse-level snapshots to SKU level and maps ozon_product_id.
func BuildFBOStorageRiskBySKU(
	rows []dbgen.StockAnalyticsSnapshot,
	productBySKU map[int64]productRef,
) FBOStorageRiskBySKU {
	bySKU := make(map[int64]*fboStorageRiskAgg)
	for _, row := range rows {
		agg := bySKU[row.Sku]
		if agg == nil {
			agg = &fboStorageRiskAgg{
				sku:            row.Sku,
				warehouseNames: make(map[string]struct{}),
				warehouseIDs:   make(map[int64]struct{}),
			}
			bySKU[row.Sku] = agg
		}
		agg.available += row.AvailableStockCount
		agg.excess += row.ExcessStockCount
		agg.requested += int32FromInt4(row.RequestedStockCount)
		agg.transit += int32FromInt4(row.TransitStockCount)
		agg.valid += int32FromInt4(row.ValidStockCount)
		agg.waitingDocs += int32FromInt4(row.WaitingDocsStockCount)
		if row.WarehouseName.Valid && row.WarehouseName.String != "" {
			agg.warehouseNames[row.WarehouseName.String] = struct{}{}
		}
		if row.WarehouseID != 0 {
			agg.warehouseIDs[row.WarehouseID] = struct{}{}
		}
		if row.Idc.Valid && (!agg.idcSet || row.Idc.Int32 > agg.idc) {
			agg.idc = row.Idc.Int32
			agg.idcSet = true
		}
		if row.DaysWithoutSales.Valid && (!agg.daysWithoutSalesSet || row.DaysWithoutSales.Int32 > agg.daysWithoutSales) {
			agg.daysWithoutSales = row.DaysWithoutSales.Int32
			agg.daysWithoutSalesSet = true
		}
		if row.Ads.Valid {
			if v, ok := numericToFloat64(row.Ads); ok {
				agg.adsSum += v
				agg.adsCount++
			}
		}
		grade := strings.TrimSpace(strings.ToUpper(textValue(row.TurnoverGrade)))
		if grade == fboTurnoverGradeSurplus {
			agg.turnoverGrade = grade
		} else if agg.turnoverGrade == "" && grade != "" && !isUnknownTurnoverGrade(grade) {
			agg.turnoverGrade = grade
		}
		if cluster := strings.TrimSpace(textValue(row.TurnoverGradeCluster)); cluster != "" {
			agg.turnoverGradeCluster = cluster
		}
		if row.OfferID.Valid && agg.offerID == "" {
			agg.offerID = row.OfferID.String
		}
		if row.ProductName.Valid && agg.productName == "" {
			agg.productName = row.ProductName.String
		}
	}

	out := make(FBOStorageRiskBySKU, len(bySKU))
	for sku, agg := range bySKU {
		ctx := FBOStorageRiskSKUContext{
			SKU:                   sku,
			AvailableStockCount:   agg.available,
			ExcessStockCount:      agg.excess,
			TurnoverGrade:         agg.turnoverGrade,
			TurnoverGradeCluster:  agg.turnoverGradeCluster,
			RequestedStockCount:   agg.requested,
			TransitStockCount:     agg.transit,
			ValidStockCount:       agg.valid,
			WaitingDocsStockCount: agg.waitingDocs,
		}
		if ref, ok := productBySKU[sku]; ok {
			ctx.OzonProductID = ref.OzonProductID
			if ref.OfferID != nil {
				ctx.OfferID = ref.OfferID
			}
			if ref.Name != "" && ctx.ProductName == nil {
				name := ref.Name
				ctx.ProductName = &name
			}
		}
		if agg.offerID != "" && ctx.OfferID == nil {
			offer := agg.offerID
			ctx.OfferID = &offer
		}
		if agg.productName != "" {
			name := agg.productName
			ctx.ProductName = &name
		}
		if agg.idcSet {
			idc := agg.idc
			ctx.IDC = &idc
		}
		if agg.daysWithoutSalesSet {
			d := agg.daysWithoutSales
			ctx.DaysWithoutSales = &d
		}
		if agg.adsCount > 0 {
			ads := agg.adsSum / float64(agg.adsCount)
			ctx.ADS = &ads
		}
		if agg.available > 0 && agg.excess > 0 {
			share := float64(agg.excess) / float64(agg.available)
			if !math.IsNaN(share) && !math.IsInf(share, 0) {
				ctx.ExcessShare = share
				ctx.HasExcessShare = true
			}
		}
		for name := range agg.warehouseNames {
			ctx.WarehouseNames = append(ctx.WarehouseNames, name)
		}
		for whID := range agg.warehouseIDs {
			ctx.WarehouseIDs = append(ctx.WarehouseIDs, whID)
		}
		out[sku] = ctx
	}
	return out
}

type productRef struct {
	OzonProductID int64
	OfferID       *string
	Name          string
}

type fboStorageRiskAgg struct {
	sku                  int64
	available            int32
	excess               int32
	idc                  int32
	idcSet               bool
	daysWithoutSales     int32
	daysWithoutSalesSet  bool
	turnoverGrade        string
	turnoverGradeCluster string
	offerID              string
	productName          string
	requested            int32
	transit              int32
	valid                int32
	waitingDocs          int32
	adsSum               float64
	adsCount             int
	warehouseNames       map[string]struct{}
	warehouseIDs         map[int64]struct{}
}

func isUnknownTurnoverGrade(grade string) bool {
	switch grade {
	case "", "UNKNOWN", "UNSPECIFIED", "NONE":
		return true
	default:
		return false
	}
}

func textValue(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func int32FromInt4(v pgtype.Int4) int32 {
	if !v.Valid {
		return 0
	}
	return v.Int32
}

func numericToFloat64(v pgtype.Numeric) (float64, bool) {
	if !v.Valid {
		return 0, false
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return 0, false
	}
	return f.Float64, true
}

type fboStorageRiskEval struct {
	Trigger  bool
	Severity Severity
	Urgency  Urgency
	Reasons  []string
}

func evaluateFBOStorageRiskSignals(ctx FBOStorageRiskSKUContext) fboStorageRiskEval {
	if ctx.AvailableStockCount <= 0 {
		return fboStorageRiskEval{}
	}

	reasons := make([]string, 0, 4)
	surplusGrade := strings.ToUpper(strings.TrimSpace(ctx.TurnoverGrade)) == fboTurnoverGradeSurplus
	if surplusGrade {
		reasons = append(reasons, fboReasonTurnoverSurplus)
	}
	if ctx.ExcessStockCount > 0 {
		reasons = append(reasons, fboReasonExcessPositive)
	}
	idc := int32(0)
	hasIDC := ctx.IDC != nil
	if hasIDC {
		idc = *ctx.IDC
	}
	if hasIDC && idc >= fboStorageRiskIDCThresholdDays {
		reasons = append(reasons, fboReasonIDCGte120)
	}
	if hasIDC && idc >= fboStorageRiskIDCHighDays {
		reasons = append(reasons, fboReasonIDCGte180)
	}
	if ctx.HasExcessShare && ctx.ExcessShare >= fboStorageRiskExcessShareHigh {
		reasons = append(reasons, fboReasonExcessShareGte50)
	}
	if len(reasons) == 0 {
		return fboStorageRiskEval{}
	}

	sev := SeverityMedium
	urg := UrgencyLow
	if (surplusGrade && ctx.ExcessStockCount > 0) ||
		(hasIDC && idc >= fboStorageRiskIDCHighDays) ||
		(ctx.HasExcessShare && ctx.ExcessShare >= fboStorageRiskExcessShareHigh) {
		sev = SeverityHigh
		urg = UrgencyMedium
	}
	if surplusGrade && hasIDC && idc >= fboStorageRiskIDCCriticalDays &&
		ctx.ExcessStockCount > 0 && ctx.AvailableStockCount >= 100 {
		sev = SeverityCritical
		urg = UrgencyMedium
	}

	return fboStorageRiskEval{Trigger: true, Severity: sev, Urgency: urg, Reasons: reasons}
}
