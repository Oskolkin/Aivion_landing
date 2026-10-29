package pack

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// BuildResult is the output of PackBuilder.Build: the assembled ZIP plus metadata
// needed to store and reference the pack.
type BuildResult struct {
	SchemaVersion   string
	PackID          string
	PackHash        string
	SellerAccountID int64
	MetricDate      string

	Manifest    contract.AnalysisPackManifestV1
	Files       []GeneratedFile
	Limitations []contract.LimitationV1

	ZipBytes []byte

	Validation          ValidationReport
	BusinessConsistency BusinessConsistencyReport

	// Snapshot is the AnalysisDataSnapshot used to build the pack (not serialized to ZIP).
	Snapshot *analysisdata.AnalysisDataSnapshot `json:"-"`
}

// ValidationIssue describes one technical validation failure or warning.
type ValidationIssue struct {
	Code    string
	File    string
	Message string
}

// ValidationReport is the outcome of technical (non-semantic) pack validation.
type ValidationReport struct {
	Valid             bool
	Issues            []ValidationIssue
	CompressedBytes   int64
	UncompressedBytes int64
	TotalRows         int
}
