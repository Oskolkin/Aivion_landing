package dashboardfacts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

func computeFactsHash(facts DashboardFactsV1) (string, error) {
	facts.FactsHash = ""
	canonical, err := canonicalJSON(facts)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalJSON(v any) ([]byte, error) {
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
