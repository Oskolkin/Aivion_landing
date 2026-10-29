package pack

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// GenerationValidationReport is the machine-checkable gate that must pass
// before any OpenAI / provider call. It wraps technical ZIP validation and
// adds generation-specific identity checks so a bad pack never consumes
// provider tokens.
type GenerationValidationReport struct {
	Valid    bool              `json:"valid"`
	PackID   string            `json:"pack_id,omitempty"`
	PackHash string            `json:"pack_hash,omitempty"`
	Issues   []ValidationIssue `json:"issues,omitempty"`
	Zip      ValidationReport  `json:"zip"`
}

// ValidateAnalysisPackForGeneration validates ZIP bytes immediately before a
// provider call. On any failure the caller must not upload the pack, must not
// start a provider response, and must mark the run failed without spending
// tokens.
//
// expectedPackID / expectedPackHash, when non-empty, must match the ZIP
// identity (manifest.pack_id and recomputed content hash). Empty expected
// values skip the corresponding identity cross-check.
func ValidateAnalysisPackForGeneration(zipBytes []byte, expectedPackID, expectedPackHash string, opts BuildOptions) (GenerationValidationReport, error) {
	v := NewValidator(opts)
	zipReport, err := v.ValidateZip(zipBytes)
	if err != nil {
		return GenerationValidationReport{}, err
	}
	out := GenerationValidationReport{
		Valid:  zipReport.Valid,
		Zip:    zipReport,
		Issues: append([]ValidationIssue{}, zipReport.Issues...),
	}
	if !zipReport.Valid {
		return out, nil
	}

	manifest, mErr := ReadManifestFromZip(zipBytes)
	if mErr != nil {
		out.Valid = false
		out.Issues = append(out.Issues, ValidationIssue{
			Code:    "manifest_unreadable",
			File:    "manifest.json",
			Message: mErr.Error(),
		})
		return out, nil
	}
	out.PackID = manifest.PackID

	contentHash, hashErr := ComputeContentHashFromZip(zipBytes)
	if hashErr != nil {
		out.Valid = false
		out.Issues = append(out.Issues, ValidationIssue{
			Code:    "pack_hash_recompute_failed",
			File:    "",
			Message: hashErr.Error(),
		})
		return out, nil
	}
	out.PackHash = contentHash

	if expectedPackID != "" && manifest.PackID != expectedPackID {
		out.Valid = false
		out.Issues = append(out.Issues, ValidationIssue{
			Code:    "pack_id_mismatch",
			File:    "manifest.json",
			Message: fmt.Sprintf("manifest pack_id=%q expected=%q", manifest.PackID, expectedPackID),
		})
	}
	if expectedPackHash != "" && contentHash != expectedPackHash {
		out.Valid = false
		out.Issues = append(out.Issues, ValidationIssue{
			Code:    "pack_hash_mismatch",
			File:    "",
			Message: fmt.Sprintf("recomputed pack_hash=%q expected=%q", contentHash, expectedPackHash),
		})
	}
	if manifest.SellerAccountID <= 0 {
		out.Valid = false
		out.Issues = append(out.Issues, ValidationIssue{
			Code:    "seller_id_invalid",
			File:    "manifest.json",
			Message: "manifest seller_account_id must be positive",
		})
	}
	if manifest.MetricDate == "" {
		out.Valid = false
		out.Issues = append(out.Issues, ValidationIssue{
			Code:    "metric_date_missing",
			File:    "manifest.json",
			Message: "manifest metric_date is required",
		})
	}
	return out, nil
}

// ComputeContentHashFromZip recomputes the pack content hash from ZIP entries
// using the same algorithm as ComputeContentHash (manifest excluded).
func ComputeContentHashFromZip(zipBytes []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return "", fmt.Errorf("unifiedanalysis/pack: open zip: %w", err)
	}
	files := make(map[string]GeneratedFile, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("unifiedanalysis/pack: open %s: %w", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return "", fmt.Errorf("unifiedanalysis/pack: read %s: %w", f.Name, err)
		}
		files[f.Name] = GeneratedFile{
			Name:   f.Name,
			SHA256: fileSHA256(content),
		}
	}
	return ComputeContentHash(contract.AnalysisPackSchemaVersionV1, files)
}
