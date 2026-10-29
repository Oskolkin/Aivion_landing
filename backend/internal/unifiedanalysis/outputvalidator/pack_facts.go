package outputvalidator

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"sort"
	"strconv"
	"strings"
)

// SKUFactRow holds the sku_summary.csv fields needed for deterministic
// material-domain detection (see ensure_domain_actions.go). It is a
// read-only projection of the Analysis Pack — never invented, never
// modified — used purely to decide which uncovered SKU problems are
// material enough to justify a backfilled Action.
type SKUFactRow struct {
	SKUID         string
	Title         string
	CategoryID    string
	Orders7D      int64
	OrdersPrev7D  int64
	Revenue7D     float64
	RevenuePrev7D float64

	HasStockData   bool
	AvailableStock int64

	AvgDailySales7D  *float64
	AvgDailySales30D *float64
	CoverageDays7D   *float64
	CoverageDays30D  *float64

	StockStatus string

	HasDaysWithoutSales bool
	DaysWithoutSales    int64
}

// CampaignFactRow holds the campaign_summary.csv fields needed for
// deterministic advertising material-domain detection.
type CampaignFactRow struct {
	CampaignID        string
	CampaignName      string
	Spend7D           float64
	Revenue7D         float64
	Orders7D          int64
	DRRPct            *float64
	ConnectedSKUCount int64
}

// SortedSKUIDs returns SKUFacts keys in stable ascending order.
func (idx *PackReferenceIndex) SortedSKUIDs() []string {
	if idx == nil || len(idx.SKUFacts) == 0 {
		return nil
	}
	ids := make([]string, 0, len(idx.SKUFacts))
	for id := range idx.SKUFacts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// SortedCampaignIDs returns CampaignFacts keys in stable ascending order.
func (idx *PackReferenceIndex) SortedCampaignIDs() []string {
	if idx == nil || len(idx.CampaignFacts) == 0 {
		return nil
	}
	ids := make([]string, 0, len(idx.CampaignFacts))
	for id := range idx.CampaignFacts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func indexSKUFactsFromCSV(idx *PackReferenceIndex, f *zip.File) {
	if idx == nil || f == nil {
		return
	}
	raw, err := readZipFile(f)
	if err != nil || len(raw) == 0 {
		return
	}
	header, rows, err := readCSVTable(raw)
	if err != nil || len(rows) == 0 {
		return
	}
	col := csvColumnIndex(header)
	get := func(row []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	if idx.SKUFacts == nil {
		idx.SKUFacts = make(map[string]SKUFactRow)
	}
	for _, row := range rows {
		skuID := get(row, "sku_id")
		if skuID == "" {
			continue
		}
		fact := SKUFactRow{
			SKUID:            skuID,
			Title:            get(row, "title"),
			CategoryID:       get(row, "category_id"),
			Orders7D:         parseInt64Cell(get(row, "orders_7d")),
			OrdersPrev7D:     parseInt64Cell(get(row, "orders_previous_7d")),
			Revenue7D:        parseFloat64Cell(get(row, "revenue_7d")),
			RevenuePrev7D:    parseFloat64Cell(get(row, "revenue_previous_7d")),
			AvgDailySales7D:  parseFloatCellPtr(get(row, "average_daily_sales_7d")),
			AvgDailySales30D: parseFloatCellPtr(get(row, "average_daily_sales_30d")),
			CoverageDays7D:   parseFloatCellPtr(get(row, "stock_coverage_days_7d")),
			CoverageDays30D:  parseFloatCellPtr(get(row, "stock_coverage_days_30d")),
			StockStatus:      get(row, "stock_status"),
		}
		if v := get(row, "available_stock_units"); v != "" {
			fact.HasStockData = true
			fact.AvailableStock = parseInt64Cell(v)
		}
		if v := get(row, "days_without_sales"); v != "" {
			fact.HasDaysWithoutSales = true
			fact.DaysWithoutSales = parseInt64Cell(v)
		}
		idx.SKUFacts[skuID] = fact
	}
}

func indexCampaignFactsFromCSV(idx *PackReferenceIndex, f *zip.File) {
	if idx == nil || f == nil {
		return
	}
	raw, err := readZipFile(f)
	if err != nil || len(raw) == 0 {
		return
	}
	header, rows, err := readCSVTable(raw)
	if err != nil || len(rows) == 0 {
		return
	}
	col := csvColumnIndex(header)
	get := func(row []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	if idx.CampaignFacts == nil {
		idx.CampaignFacts = make(map[string]CampaignFactRow)
	}
	for _, row := range rows {
		campaignID := get(row, "campaign_id")
		if campaignID == "" {
			continue
		}
		fact := CampaignFactRow{
			CampaignID:        campaignID,
			CampaignName:      get(row, "campaign_name"),
			Spend7D:           parseFloat64Cell(get(row, "spend_7d")),
			Revenue7D:         parseFloat64Cell(get(row, "revenue_7d")),
			Orders7D:          parseInt64Cell(get(row, "orders_7d")),
			DRRPct:            parseFloatCellPtr(get(row, "drr_pct")),
			ConnectedSKUCount: parseInt64Cell(get(row, "connected_sku_count")),
		}
		idx.CampaignFacts[campaignID] = fact
	}
}

// readCSVTable parses raw CSV bytes into a header row and data rows,
// copying every cell so callers may retain rows beyond the reader's
// internal buffer lifetime.
func readCSVTable(raw []byte) (header []string, rows [][]string, err error) {
	r := csv.NewReader(bytes.NewReader(raw))
	head, err := r.Read()
	if err != nil {
		return nil, nil, err
	}
	header = append([]string(nil), head...)
	for {
		rec, rerr := r.Read()
		if rerr != nil {
			break
		}
		rows = append(rows, append([]string(nil), rec...))
	}
	return header, rows, nil
}

func csvColumnIndex(header []string) map[string]int {
	col := make(map[string]int, len(header))
	for i, h := range header {
		col[strings.TrimSpace(h)] = i
	}
	return col
}

func parseInt64Cell(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil {
		return 0
	}
	return int64(v)
}

func parseFloat64Cell(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil {
		return 0
	}
	return v
}

func parseFloatCellPtr(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil {
		return nil
	}
	return &v
}
