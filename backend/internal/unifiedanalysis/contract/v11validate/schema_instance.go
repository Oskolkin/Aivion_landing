package v11validate

import (
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// ValidateInstanceAgainstSchemaV11 structurally validates a JSON instance
// against the embedded v11 schema (types, enums, additionalProperties, anyOf).
// Required-key presence is enforced (provider structured-output path).
func ValidateInstanceAgainstSchemaV11(raw []byte) error {
	schemaRaw, err := contract.SchemaJSONV11()
	if err != nil {
		return err
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaRaw, &schema); err != nil {
		return fmt.Errorf("schema unmarshal: %w", err)
	}
	var instance any
	if err := json.Unmarshal(raw, &instance); err != nil {
		return fmt.Errorf("instance unmarshal: %w", err)
	}
	v := &instanceValidator{root: schema}
	return v.validate(instance, schema, "$")
}

type instanceValidator struct {
	root map[string]any
}

func (v *instanceValidator) validate(instance any, schema map[string]any, path string) error {
	if ref, ok := schema["$ref"].(string); ok {
		resolved, err := resolve(v.root, ref)
		if err != nil {
			return err
		}
		return v.validate(instance, resolved, path)
	}
	if anyOf, ok := schema["anyOf"].([]any); ok {
		var last error
		for _, sub := range anyOf {
			subSchema, ok := sub.(map[string]any)
			if !ok {
				continue
			}
			if err := v.validate(instance, subSchema, path); err == nil {
				return nil
			} else {
				last = err
			}
		}
		return fmt.Errorf("%s: no anyOf branch matched (last: %v)", path, last)
	}
	if enumVals, ok := schema["enum"].([]any); ok {
		matched := false
		for _, ev := range enumVals {
			if fmt.Sprintf("%v", ev) == fmt.Sprintf("%v", instance) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%s: value %v not in enum", path, instance)
		}
	}
	if err := v.validateType(instance, schema["type"], path); err != nil {
		return err
	}
	if isObj(schema) {
		obj, ok := instance.(map[string]any)
		if !ok {
			if instance == nil && typeAllowsNull(schema["type"]) {
				return nil
			}
			return fmt.Errorf("%s: expected object, got %T", path, instance)
		}
		props, _ := schema["properties"].(map[string]any)
		if schema["additionalProperties"] == false {
			for key := range obj {
				if _, ok := props[key]; !ok {
					return fmt.Errorf("%s: unexpected additional property %q", path, key)
				}
			}
		}
		if req, ok := schema["required"].([]any); ok {
			for _, r := range req {
				name, _ := r.(string)
				if name == "" {
					continue
				}
				if _, ok := obj[name]; !ok {
					return fmt.Errorf("%s: missing required property %q", path, name)
				}
			}
		}
		for key, val := range obj {
			propSchema, ok := props[key].(map[string]any)
			if !ok {
				continue
			}
			if err := v.validate(val, propSchema, path+"."+key); err != nil {
				return err
			}
		}
		return nil
	}
	if hasArrayType(schema["type"]) {
		arr, ok := instance.([]any)
		if !ok {
			if instance == nil && typeAllowsNull(schema["type"]) {
				return nil
			}
			return fmt.Errorf("%s: expected array, got %T", path, instance)
		}
		itemSchema, ok := schema["items"].(map[string]any)
		if ok {
			for i, item := range arr {
				if err := v.validate(item, itemSchema, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (v *instanceValidator) validateType(instance any, rawType any, path string) error {
	if rawType == nil {
		return nil
	}
	types := typeList(rawType)
	if len(types) == 0 {
		return nil
	}
	for _, want := range types {
		if matchesJSONType(instance, want) {
			return nil
		}
	}
	return fmt.Errorf("%s: value %v (%T) does not match type(s) %v", path, instance, instance, types)
}

func typeList(rawType any) []string {
	switch t := rawType.(type) {
	case string:
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, v := range t {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func typeAllowsNull(rawType any) bool {
	for _, t := range typeList(rawType) {
		if t == "null" {
			return true
		}
	}
	return false
}

func hasArrayType(rawType any) bool {
	for _, t := range typeList(rawType) {
		if t == "array" {
			return true
		}
	}
	return false
}

func matchesJSONType(instance any, want string) bool {
	switch want {
	case "null":
		return instance == nil
	case "string":
		_, ok := instance.(string)
		return ok
	case "boolean":
		_, ok := instance.(bool)
		return ok
	case "integer":
		f, ok := instance.(float64)
		return ok && f == float64(int64(f))
	case "number":
		_, ok := instance.(float64)
		return ok
	case "object":
		_, ok := instance.(map[string]any)
		return ok
	case "array":
		_, ok := instance.([]any)
		return ok
	default:
		return false
	}
}
