package pack

import "encoding/json"

// parseJSONElements returns the number of elements in a JSON array payload,
// or an empty slice when raw is empty/invalid.
func parseJSONElements(raw []byte) []any {
	if len(raw) == 0 {
		return nil
	}
	var out []any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
