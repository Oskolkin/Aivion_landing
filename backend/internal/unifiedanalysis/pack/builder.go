package pack

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// PackBuilder assembles a Unified Analysis Pack v1 ZIP from a shared AnalysisDataSnapshot.
type PackBuilder struct {
	opts BuildOptions
}

// NewPackBuilder creates a PackBuilder with the given options (defaults filled where zero).
func NewPackBuilder(opts BuildOptions) *PackBuilder {
	return &PackBuilder{opts: opts.withDefaults()}
}

// Build assembles every pack file from snapshot and returns the complete BuildResult,
// including the assembled ZIP bytes. It does not perform technical validation or storage;
// see Validator and PackStorage / Service for those steps.
func (b *PackBuilder) Build(snapshot *analysisdata.AnalysisDataSnapshot) (*BuildResult, error) {
	if snapshot == nil {
		return nil, ErrNilSnapshot
	}
	if snapshot.SellerAccountID <= 0 {
		return nil, ErrInvalidSellerAccountID
	}
	if !snapshot.MetricDate.Valid {
		return nil, fmt.Errorf("unifiedanalysis/pack: snapshot.MetricDate is not valid")
	}

	metricDate := analysisdata.FormatDate(snapshot.MetricDate.Time)
	periods := ComputePackPeriods(snapshot.MetricDate.Time)
	idx := BuildIndexes(snapshot)
	facts := ComputeAllSKUFacts(idx, periods, metricDate)
	limitations := newLimitationCollector()

	business := ValidateBusinessConsistency(idx, facts, periods, metricDate)

	tables := []csvTable{
		BuildAccountSummaryTable(idx, periods, facts, snapshot.SellerAccountID, metricDate, limitations),
		BuildAccountDailyTable(idx, periods, snapshot.SellerAccountID),
		BuildSKUSummaryTable(idx, facts, limitations),
		BuildSKUDailyTable(idx),
		BuildCampaignSummaryTable(idx, periods),
		BuildCampaignDailyTable(idx),
		BuildCampaignSKUTable(idx, limitations),
		BuildProductEnrichmentTable(idx, limitations),
	}

	files := make(map[string]GeneratedFile, len(contract.AnalysisPackFilesV1))
	orderedCSV := make([]GeneratedFile, 0, len(tables))
	for _, t := range tables {
		content, err := renderCSV(t)
		if err != nil {
			return nil, err
		}
		gf := GeneratedFile{
			Name:     t.Name,
			Content:  content,
			SHA256:   fileSHA256(content),
			RowCount: len(t.Rows),
			Kind:     "csv",
		}
		files[t.Name] = gf
		orderedCSV = append(orderedCSV, gf)
	}

	limitationDocs := limitations.Build()
	coverageDocs := BuildDailyCoverageMetadata(idx, periods)

	dictionary := BuildDataDictionary()
	if err := putJSONFile(files, "data_dictionary.json", dictionary); err != nil {
		return nil, err
	}
	policy := BuildAnalysisPolicy()
	if err := putJSONFile(files, "analysis_policy.json", policy); err != nil {
		return nil, err
	}
	limitationsDoc := BuildLimitationsDocument(limitationDocs, coverageDocs)
	if err := putJSONFile(files, "limitations.json", limitationsDoc); err != nil {
		return nil, err
	}

	dataStatus := ResolveDataStatus(limitationDocs)
	manifest := BuildManifest(
		snapshot.SellerAccountID,
		metricDate,
		b.opts.Timezone,
		b.opts.Currency,
		periods,
		orderedCSV,
		EntityCounts{
			SKUCount:      len(idx.SKUUniverse),
			CampaignCount: len(idx.CampaignUniverse),
			CategoryCount: len(idx.CategoryByID),
		},
		dataStatus,
	)
	manifest.Diagnostics = &contract.AnalysisPackDiagnosticsV1{
		TotalDiscoveredSKU:    idx.Eligibility.TotalDiscovered,
		AnalysisEligibleSKU:   idx.Eligibility.AnalysisEligible,
		ExcludedTechnicalSKU:  idx.Eligibility.ExcludedTechnical,
		ExcludedHistoricalSKU: idx.Eligibility.ExcludedHistorical,
		ExcludedIncompleteSKU: idx.Eligibility.ExcludedIncomplete + idx.Eligibility.ExcludedNoProduct + idx.Eligibility.ExcludedNoBusinessSource,
		ExcludedArchivedSKU:   idx.Eligibility.ExcludedArchived,
		BusinessConsistencyOK: !business.HasErrors(),
		BusinessErrorCount:    len(business.Errors),
		BusinessWarningCount:  len(business.Warnings),
	}

	contentHash, err := ComputeContentHash(contract.AnalysisPackSchemaVersionV1, files)
	if err != nil {
		return nil, err
	}
	packID := BuildPackID(snapshot.SellerAccountID, metricDate, contentHash)
	manifest.PackID = packID

	if err := putJSONFile(files, "manifest.json", manifest); err != nil {
		return nil, err
	}

	zipBytes, err := BuildZip(files)
	if err != nil {
		return nil, err
	}

	orderedFiles := make([]GeneratedFile, 0, len(contract.AnalysisPackFilesV1))
	for _, def := range contract.AnalysisPackFilesV1 {
		orderedFiles = append(orderedFiles, files[def.Name])
	}

	return &BuildResult{
		SchemaVersion:       contract.AnalysisPackSchemaVersionV1,
		PackID:              packID,
		PackHash:            contentHash,
		SellerAccountID:     snapshot.SellerAccountID,
		MetricDate:          metricDate,
		Manifest:            manifest,
		Files:               orderedFiles,
		Limitations:         limitationDocs,
		ZipBytes:            zipBytes,
		BusinessConsistency: business,
	}, nil
}

func putJSONFile(files map[string]GeneratedFile, name string, doc any) error {
	content, err := marshalJSONDoc(name, doc)
	if err != nil {
		return err
	}
	files[name] = GeneratedFile{
		Name:    name,
		Content: content,
		SHA256:  fileSHA256(content),
		Kind:    "json",
	}
	return nil
}
