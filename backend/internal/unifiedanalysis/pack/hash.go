package pack

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// fileSHA256 returns the lowercase hex-encoded SHA256 of file content.
func fileSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// ComputeContentHash returns the full hex SHA256 pack content hash: it is the
// SHA256 of schema_version followed by each non-manifest file's name and
// content SHA256, in the frozen contract.AnalysisPackFilesV1 order.
// manifest.json is excluded because it embeds per-file hashes and would be
// self-referential.
func ComputeContentHash(schemaVersion string, files map[string]GeneratedFile) (string, error) {
	h := sha256.New()
	h.Write([]byte(schemaVersion))
	for _, def := range contract.AnalysisPackFilesV1 {
		if def.Name == "manifest.json" {
			continue
		}
		f, ok := files[def.Name]
		if !ok {
			return "", fmt.Errorf("unifiedanalysis/pack: missing file %s for content hash", def.Name)
		}
		h.Write([]byte(def.Name))
		h.Write([]byte(f.SHA256))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// BuildPackID renders the frozen v1 pack_id format:
// pack:<seller_account_id>:<metric_date>:<first16(content_hash)>.
func BuildPackID(sellerAccountID int64, metricDate string, contentHash string) string {
	prefixLen := 16
	if len(contentHash) < prefixLen {
		prefixLen = len(contentHash)
	}
	return fmt.Sprintf("pack:%d:%s:%s", sellerAccountID, metricDate, contentHash[:prefixLen])
}
