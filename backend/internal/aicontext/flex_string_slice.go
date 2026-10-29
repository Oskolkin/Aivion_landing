package aicontext

import (
	"encoding/json"
)

// FlexibleStringSlice accepts either a JSON string or string array from AI output.
type FlexibleStringSlice []string

func (f *FlexibleStringSlice) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*f = nil
		return nil
	}
	if data[0] == '"' {
		var single string
		if err := json.Unmarshal(data, &single); err != nil {
			return err
		}
		*f = FlexibleStringSlice{single}
		return nil
	}
	var items []string
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	*f = FlexibleStringSlice(items)
	return nil
}

func (f FlexibleStringSlice) Strings() []string {
	return []string(f)
}
