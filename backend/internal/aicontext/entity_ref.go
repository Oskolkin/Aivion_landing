package aicontext

import (
	"encoding/json"
	"strings"
)

// UnmarshalJSON accepts either a full object or a shorthand entity_id string from AI output.
func (e *SemanticEntityRef) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var entityID string
		if err := json.Unmarshal(data, &entityID); err != nil {
			return err
		}
		entityID = strings.TrimSpace(entityID)
		e.EntityID = entityID
		e.EntityType = inferEntityTypeFromID(entityID)
		return nil
	}
	type alias SemanticEntityRef
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*e = SemanticEntityRef(decoded)
	if e.EntityType == "" && e.EntityID != "" {
		e.EntityType = inferEntityTypeFromID(e.EntityID)
	}
	return nil
}

func inferEntityTypeFromID(entityID string) string {
	entityID = strings.TrimSpace(entityID)
	if entityID == "" {
		return ""
	}
	if i := strings.Index(entityID, ":"); i > 0 {
		return entityID[:i]
	}
	return ""
}
