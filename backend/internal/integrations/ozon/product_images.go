package ozon

import (
	"bytes"
	"encoding/json"
)

// FlexibleStringList unmarshals Ozon fields that may be a JSON string or array of strings.
type FlexibleStringList []string

func (f *FlexibleStringList) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*f = nil
		return nil
	}
	if b[0] == '[' {
		var items []string
		if err := json.Unmarshal(b, &items); err != nil {
			return err
		}
		*f = items
		return nil
	}
	var single string
	if err := json.Unmarshal(b, &single); err != nil {
		return err
	}
	if single != "" {
		*f = []string{single}
	} else {
		*f = nil
	}
	return nil
}

// PickPrimaryImageURL chooses display image from Ozon primary_image then images.
func PickPrimaryImageURL(primary, images FlexibleStringList) string {
	for _, list := range []FlexibleStringList{primary, images} {
		for _, url := range list {
			if url != "" {
				return url
			}
		}
	}
	return ""
}

// ImageURLsJSON returns JSON array bytes for storage, or nil when empty.
func ImageURLsJSON(images FlexibleStringList) []byte {
	if len(images) == 0 {
		return nil
	}
	out := make([]string, 0, len(images))
	for _, url := range images {
		if url != "" {
			out = append(out, url)
		}
	}
	if len(out) == 0 {
		return nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil
	}
	return b
}
