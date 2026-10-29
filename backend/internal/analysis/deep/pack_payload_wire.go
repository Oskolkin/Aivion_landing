package deep

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

func payloadWireAny(p PayloadV1) (any, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var wire any
	if err = json.Unmarshal(b, &wire); err != nil {
		return nil, err
	}
	return canonicalizeAny(wire)
}

func canonicalizeAny(v any) (any, error) {
	switch x := v.(type) {
	case float64:
		if x == float64(int64(x)) {
			return int64(x), nil
		}
		return canonicaljson.Decimal(strconv.FormatFloat(x, 'f', -1, 64)), nil
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			cv, err := canonicalizeAny(val)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", k, err)
			}
			out[k] = cv
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			cv, err := canonicalizeAny(val)
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

// MarshalPayloadCanonical serializes Deep pack payload with embedded evidence JSON objects.
func MarshalPayloadCanonical(p PayloadV1) ([]byte, error) {
	wire, err := payloadWireAny(p)
	if err != nil {
		return nil, err
	}
	return canonicaljson.Marshal(wire)
}

// HashPayloadCanonical returns the canonical payload hash used by pack artifacts.
func HashPayloadCanonical(p PayloadV1) string {
	wire, err := payloadWireAny(p)
	if err != nil {
		return ""
	}
	return canonicaljson.Hash(wire)
}

// MarshalPackWireCanonical serializes the full Deep pack wire artifact.
func MarshalPackWireCanonical(w PackWireV1) ([]byte, error) {
	payloadWire, err := payloadWireAny(w.Payload)
	if err != nil {
		return nil, err
	}
	return canonicaljson.Marshal(map[string]any{
		"AuditEnvelope": w.AuditEnvelope,
		"payload":       payloadWire,
	})
}
