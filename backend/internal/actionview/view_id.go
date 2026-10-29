package actionview

import (
	"encoding/base64"
	"fmt"
	"strings"
)

const viewVersion = "v1"

func EncodeGroup(id string) string {
	return encode("g", id)
}

func EncodeCase(id string) string {
	return encode("c", id)
}

func encode(kind, id string) string {
	raw := viewVersion + "." + kind + "." + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

type ViewRef struct {
	Kind string
	ID   string
}

func Decode(viewID string) (ViewRef, error) {
	b, err := base64.RawURLEncoding.DecodeString(viewID)
	if err != nil {
		return ViewRef{}, fmt.Errorf("invalid action_view_id")
	}
	parts := strings.Split(string(b), ".")
	if len(parts) != 3 || parts[0] != viewVersion || (parts[1] != "g" && parts[1] != "c") || parts[2] == "" {
		return ViewRef{}, fmt.Errorf("invalid action_view_id")
	}
	return ViewRef{Kind: parts[1], ID: parts[2]}, nil
}
