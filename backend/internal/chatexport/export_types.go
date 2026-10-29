package chatexport

import "time"

const (
	FormatCSV  = "csv"
	FormatXLSX = "xlsx"
	FormatPDF  = "pdf"
	FormatDOCX = "docx"
	FormatPPTX = "pptx"
	FormatTXT  = "txt"
	FormatMD   = "md"
	FormatJSON = "json"
	FormatPNG  = "png"
	FormatJPEG = "jpeg"
	FormatZIP  = "zip"

	MaxExportRows      = 10_000
	MaxExportDateRange = 90

	MIMETypeCSV  = "text/csv; charset=utf-8"
	MIMETypeXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

// Export type allowlist.
const (
	TypeSalesSummary         = "sales_summary"
	TypeSKUPerformance       = "sku_performance"
	TypeCriticalSKU          = "critical_sku"
	TypeStockRisks           = "stock_risks"
	TypeStockSurplus         = "stock_surplus"
	TypeAdvertisingCampaigns = "advertising_campaigns"
	TypeCampaignPerformance  = "campaign_performance"
	TypeAlerts               = "alerts"
	TypeRecommendations      = "recommendations"
	TypeOrdersByStatus       = "orders_by_status"
)

// Reason codes for controlled responses.
const (
	ReasonFormatNotSupported      = "format_not_supported"
	ReasonExportClarification     = "clarification_needed"
	ReasonExportTypeUnsupported   = "export_type_not_supported"
	ReasonExportSourceUnavailable = "unsupported_by_source"
	ReasonArbitrarySQL            = "arbitrary_sql_refused"
	ReasonDateRangeTooLarge       = "date_range_too_large"
)

type ExportTypeSpec struct {
	ExportType   string
	Implemented  bool
	MaxRows      int
	DefaultSort  string
	FileNameStem string
	SheetName    string
}

func AllExportTypeSpecs() []ExportTypeSpec {
	return []ExportTypeSpec{
		{TypeSalesSummary, true, MaxExportRows, "date_desc", "sales_summary", "sales_summary"},
		{TypeSKUPerformance, true, MaxExportRows, "revenue_desc", "sku_performance", "sku_performance"},
		{TypeCriticalSKU, true, MaxExportRows, "problem_score_desc", "critical_sku", "critical_sku"},
		{TypeStockRisks, true, MaxExportRows, "risk_desc", "stock_risks", "stock_risks"},
		{TypeStockSurplus, true, MaxExportRows, "excess_desc", "stock_surplus", "stock_surplus"},
		{TypeAdvertisingCampaigns, true, MaxExportRows, "spend_desc", "advertising_campaigns", "advertising"},
		{TypeCampaignPerformance, true, MaxExportRows, "spend_desc", "campaign_performance", "campaigns"},
		{TypeAlerts, true, MaxExportRows, "created_at_desc", "alerts", "alerts"},
		{TypeRecommendations, true, MaxExportRows, "created_at_desc", "recommendations", "recommendations"},
		{TypeOrdersByStatus, true, MaxExportRows, "count_desc", "orders_by_status", "orders"},
	}
}

func LookupExportType(exportType string) (ExportTypeSpec, bool) {
	for _, spec := range AllExportTypeSpecs() {
		if spec.ExportType == exportType {
			return spec, true
		}
	}
	return ExportTypeSpec{}, false
}

func IsAllowedFormat(format string) bool {
	switch format {
	case FormatCSV, FormatXLSX, FormatPDF, FormatDOCX, FormatPPTX, FormatTXT, FormatMD, FormatJSON, FormatPNG, FormatJPEG, FormatZIP:
		return true
	default:
		return false
	}
}

type ParsedRequest struct {
	ExportType string
	Format     string
	DateFrom   time.Time
	DateTo     time.Time
	Filters    map[string]any
	Reason     string
	Message    string
}

type ExportTable struct {
	Columns []string
	Rows    [][]string
}

type Result struct {
	AttachmentID string
	FileName     string
	MimeType     string
	FileSize     int64
	StorageKey   string
	RowCount     int
}
