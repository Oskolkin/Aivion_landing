package contract

// PackFilePresenceStable means every listed pack file is always present in the ZIP.
// When a source has no rows, the CSV still exists with header-only content and row_count=0.
// Absence of data is recorded in limitations.json and manifest availability fields.
const PackFilePresenceStable = true

// PackFileDefinition describes a file that belongs to Analysis Pack v1.
type PackFileDefinition struct {
	Name     string
	Required bool   // true => always present in ZIP (may be header-only)
	Kind     string // "json" | "csv"
}

// AnalysisPackFilesV1 is the immutable ordered file list for pack v1.
// Decision (stage-1 freeze): all CSV files are required/present in ZIP.
// Empty sources produce header-only files (row_count=0), not omitted files.
var AnalysisPackFilesV1 = []PackFileDefinition{
	{Name: "manifest.json", Required: true, Kind: "json"},
	{Name: "data_dictionary.json", Required: true, Kind: "json"},
	{Name: "analysis_policy.json", Required: true, Kind: "json"},
	{Name: "limitations.json", Required: true, Kind: "json"},
	{Name: "account_summary.csv", Required: true, Kind: "csv"},
	{Name: "account_daily.csv", Required: true, Kind: "csv"},
	{Name: "sku_summary.csv", Required: true, Kind: "csv"},
	{Name: "sku_daily.csv", Required: true, Kind: "csv"},
	{Name: "campaign_summary.csv", Required: true, Kind: "csv"},
	{Name: "campaign_daily.csv", Required: true, Kind: "csv"},
	{Name: "campaign_sku.csv", Required: true, Kind: "csv"},
	{Name: "product_enrichment.csv", Required: true, Kind: "csv"},
}

// RequiredAnalysisPackFilesV1 returns required pack file names in stable order.
func RequiredAnalysisPackFilesV1() []string {
	out := make([]string, 0, len(AnalysisPackFilesV1))
	for _, f := range AnalysisPackFilesV1 {
		if f.Required {
			out = append(out, f.Name)
		}
	}
	return out
}
