package pack

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"

// EntityCounts summarizes SKU/campaign/category coverage for the manifest.
type EntityCounts struct {
	SKUCount      int
	CampaignCount int
	CategoryCount int
}

// BuildManifest assembles manifest.json from the built tables, periods, and identity fields.
// PackID/PackHash are filled in separately once the content hash is computed (see hash.go),
// since the manifest itself is excluded from the content hash.
func BuildManifest(
	sellerAccountID int64,
	metricDate string,
	timezone string,
	currency string,
	periods Periods,
	tables []GeneratedFile,
	counts EntityCounts,
	dataStatus string,
) contract.AnalysisPackManifestV1 {
	tableEntries := make([]contract.AnalysisPackTableManifestV1, 0, len(tables))
	for _, f := range tables {
		if f.Kind != "csv" {
			continue
		}
		tableEntries = append(tableEntries, contract.AnalysisPackTableManifestV1{
			Name:     f.Name,
			File:     f.Name,
			Required: true,
			RowCount: f.RowCount,
			Columns:  contract.AnalysisPackCSVColumnsV1[f.Name],
			SHA256:   f.SHA256,
		})
	}

	return contract.AnalysisPackManifestV1{
		SchemaVersion:   contract.AnalysisPackSchemaVersionV1,
		SellerAccountID: sellerAccountID,
		MetricDate:      metricDate,
		Timezone:        timezone,
		Currency:        currency,
		Periods:         periods.ToContract(),
		Tables:          tableEntries,
		EntityCounts: contract.AnalysisPackEntityCountsV1{
			SKUCount:      counts.SKUCount,
			CampaignCount: counts.CampaignCount,
			CategoryCount: counts.CategoryCount,
		},
		DataStatus: dataStatus,
	}
}

// ResolveDataStatus derives the pack-level data_status label from limitation severities.
func ResolveDataStatus(limitations []contract.LimitationV1) string {
	if len(limitations) == 0 {
		return "available"
	}
	for _, lim := range limitations {
		if lim.Severity == "high" {
			return "partial"
		}
	}
	return "available_with_limitations"
}
