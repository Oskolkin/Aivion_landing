package evidence

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// NormalizeEvidenceDataBytes decodes legacy canonical-json byte-array encoding of embedded JSON objects.
func NormalizeEvidenceDataBytes(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	if raw[0] == '{' {
		return raw
	}
	if raw[0] != '[' {
		return raw
	}
	var bytes []byte
	if err := json.Unmarshal(raw, &bytes); err != nil || len(bytes) == 0 || bytes[0] != '{' {
		return raw
	}
	return json.RawMessage(bytes)
}

func bundleWireAny(b BundleV1) (any, error) {
	jb, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	var wire any
	if err = json.Unmarshal(jb, &wire); err != nil {
		return nil, err
	}
	m, ok := wire.(map[string]any)
	if !ok {
		return canonicalizeWire(wire)
	}
	items, ok := m["items"].([]any)
	if !ok {
		return canonicalizeWire(wire)
	}
	for i, rawIt := range items {
		it, ok := rawIt.(map[string]any)
		if !ok {
			continue
		}
		if data, ok := it["data"]; ok {
			fixed, err := fixEvidenceDataWire(data)
			if err != nil {
				return nil, err
			}
			it["data"] = fixed
			items[i] = it
		}
	}
	m["items"] = items
	return canonicalizeWire(m)
}

func canonicalizeWire(v any) (any, error) {
	switch x := v.(type) {
	case float64:
		if x == float64(int64(x)) {
			return int64(x), nil
		}
		return canonicaljson.Decimal(strconv.FormatFloat(x, 'f', -1, 64)), nil
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			cv, err := canonicalizeWire(val)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			out[k] = cv
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			cv, err := canonicalizeWire(val)
			if err != nil {
				return nil, err
			}
			out[i] = cv
		}
		return out, nil
	default:
		return v, nil
	}
}

func fixEvidenceDataWire(v any) (any, error) {
	arr, ok := v.([]any)
	if !ok {
		return v, nil
	}
	bytes := make([]byte, 0, len(arr))
	for _, el := range arr {
		switch n := el.(type) {
		case float64:
			if n < 0 || n > 255 || n != float64(byte(n)) {
				return v, nil
			}
			bytes = append(bytes, byte(n))
		case int64:
			if n < 0 || n > 255 {
				return v, nil
			}
			bytes = append(bytes, byte(n))
		default:
			return v, nil
		}
	}
	if len(bytes) == 0 || bytes[0] != '{' {
		return v, nil
	}
	var obj any
	if err := json.Unmarshal(bytes, &obj); err != nil {
		return v, nil
	}
	return obj, nil
}

// MarshalBundleCanonical stores evidence bundle artifacts with embedded JSON data objects.
func MarshalBundleCanonical(b BundleV1) ([]byte, error) {
	wire, err := bundleWireAny(b)
	if err != nil {
		return nil, err
	}
	return canonicaljson.Marshal(wire)
}

// HashBundleCanonical hashes bundle content using the same wire encoding as artifacts.
func HashBundleCanonical(b BundleV1) string {
	wire, err := bundleWireAny(b)
	if err != nil {
		return ""
	}
	return canonicaljson.Hash(wire)
}
