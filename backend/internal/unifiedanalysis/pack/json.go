package pack

import (
	"encoding/json"
	"fmt"
)

// marshalJSONDoc renders v as deterministic, pretty-printed, UTF-8 JSON with a
// trailing newline. encoding/json already sorts map keys, so output is stable
// across repeated builds of the same input.
func marshalJSONDoc(name string, v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", name, err)
	}
	return append(b, '\n'), nil
}
