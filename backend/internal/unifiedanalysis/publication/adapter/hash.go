package adapter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// CanonicalJSON marshals v to JSON with deterministically sorted object keys
// so structurally-equal projections hash identically regardless of Go struct
// field ordering quirks or map iteration order.
func CanonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	normalized := normalizeJSONValue(decoded)
	out, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func normalizeJSONValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(t))
		for _, k := range keys {
			out[k] = normalizeJSONValue(t[k])
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = normalizeJSONValue(t[i])
		}
		return out
	default:
		return v
	}
}

// ProjectionHash returns the sha256 hex digest of v's canonical JSON form.
// Used for dashboard_projection_hash / actions_projection_hash / bundle_hash
// on unified_analysis_publications.
func ProjectionHash(v any) (string, error) {
	canonical, err := CanonicalJSON(v)
	if err != nil {
		return "", err
	}
	return HashBytes(canonical), nil
}

// HashBytes returns the sha256 hex digest of raw bytes.
func HashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// BundleHash combines multiple already-computed hashes (e.g. dashboard +
// actions projection hashes) into one stable digest.
func BundleHash(parts ...string) string {
	var buf bytes.Buffer
	for i, p := range parts {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(p)
	}
	return HashBytes(buf.Bytes())
}
