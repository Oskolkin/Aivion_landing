package pack

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// ReadManifestFromZip extracts and decodes manifest.json from a pack ZIP.
func ReadManifestFromZip(zipBytes []byte) (contract.AnalysisPackManifestV1, error) {
	var zero contract.AnalysisPackManifestV1
	if len(zipBytes) == 0 {
		return zero, ErrEmptyZip
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return zero, fmt.Errorf("unifiedanalysis/pack: open zip: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != "manifest.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return zero, fmt.Errorf("unifiedanalysis/pack: open manifest.json: %w", err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return zero, fmt.Errorf("unifiedanalysis/pack: read manifest.json: %w", err)
		}
		var manifest contract.AnalysisPackManifestV1
		if err := json.Unmarshal(content, &manifest); err != nil {
			return zero, fmt.Errorf("unifiedanalysis/pack: decode manifest.json: %w", err)
		}
		return manifest, nil
	}
	return zero, fmt.Errorf("unifiedanalysis/pack: manifest.json missing from zip")
}
