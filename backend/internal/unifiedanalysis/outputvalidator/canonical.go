package outputvalidator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// CanonicalJSON marshals output with stable struct field order and a trailing LF.
func CanonicalJSON(out *contract.UnifiedAnalysisOutputV1) ([]byte, error) {
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", ""); err != nil {
		// Compact is fine; re-encode via map is unnecessary because Go struct order is stable.
		_ = err
	}
	compact := append(append([]byte(nil), raw...), '\n')
	return compact, nil
}

// OutputHash returns SHA256 hex of canonical JSON bytes.
func OutputHash(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}
