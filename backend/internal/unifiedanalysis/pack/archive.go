package pack

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// zipEpoch is the fixed ModTime used for every ZIP entry so byte-identical
// snapshots always produce byte-identical archives (no wall-clock drift).
var zipEpoch = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// BuildZip assembles the 12 pack files into a deterministic ZIP archive in the
// frozen contract.AnalysisPackFilesV1 order, using Deflate compression and a
// fixed ModTime for every entry.
func BuildZip(files map[string]GeneratedFile) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	for _, def := range contract.AnalysisPackFilesV1 {
		f, ok := files[def.Name]
		if !ok {
			return nil, fmt.Errorf("unifiedanalysis/pack: missing file %s for zip", def.Name)
		}
		if err := validateZipEntryName(def.Name); err != nil {
			return nil, err
		}
		hdr := &zip.FileHeader{
			Name:     def.Name,
			Method:   zip.Deflate,
			Modified: zipEpoch,
		}
		entry, err := w.CreateHeader(hdr)
		if err != nil {
			return nil, fmt.Errorf("create zip entry %s: %w", def.Name, err)
		}
		if _, err := entry.Write(f.Content); err != nil {
			return nil, fmt.Errorf("write zip entry %s: %w", def.Name, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}

func validateZipEntryName(name string) error {
	if name == "" || strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("unifiedanalysis/pack: unsafe zip entry name %q", name)
	}
	return nil
}
