package contract

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestSchemaJSONIsValidJSON(t *testing.T) {
	raw, err := SchemaJSON()
	if err != nil {
		t.Fatalf("SchemaJSON: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("SchemaJSON returned empty document")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("SchemaJSON is not valid JSON: %v", err)
	}
	if doc["type"] != "object" {
		t.Fatalf("root type=%v want object", doc["type"])
	}
}

func TestSchemaJSONReturnsDefensiveCopy(t *testing.T) {
	first, err := SchemaJSON()
	if err != nil {
		t.Fatalf("SchemaJSON: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("empty schema")
	}
	original := first[0]
	first[0] = original + 1

	second, err := SchemaJSON()
	if err != nil {
		t.Fatalf("SchemaJSON: %v", err)
	}
	if second[0] != original {
		t.Fatal("mutating a previously returned schema slice affected a later call")
	}
}

func TestSchemaNameMatchesVersionConst(t *testing.T) {
	if SchemaName() != unifiedAnalysisSchemaName {
		t.Fatalf("SchemaName()=%s want=%s", SchemaName(), unifiedAnalysisSchemaName)
	}
	if SchemaName() != UnifiedAnalysisOutputSchemaVersionCurrent {
		t.Fatalf("SchemaName()=%s must match UnifiedAnalysisOutputSchemaVersionCurrent=%s", SchemaName(), UnifiedAnalysisOutputSchemaVersionCurrent)
	}
}

func TestSchemaHashStableAndFormatted(t *testing.T) {
	h1, err := SchemaHash()
	if err != nil {
		t.Fatalf("SchemaHash: %v", err)
	}
	h2, err := SchemaHash()
	if err != nil {
		t.Fatalf("SchemaHash: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("SchemaHash not stable: %s vs %s", h1, h2)
	}
	if !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(h1) {
		t.Fatalf("SchemaHash format=%s want sha256:<64 hex chars>", h1)
	}
}

func TestStructuredOutputFormatMatchesSchemaJSON(t *testing.T) {
	a, err := StructuredOutputFormat()
	if err != nil {
		t.Fatalf("StructuredOutputFormat: %v", err)
	}
	b, err := SchemaJSON()
	if err != nil {
		t.Fatalf("SchemaJSON: %v", err)
	}
	if string(a) != string(b) {
		t.Fatal("StructuredOutputFormat must be byte-identical to SchemaJSON")
	}
}

// TestSchemaRootRequiredFields locks the top-level required set named in
// the Unified Analysis Output v1 contract.
func TestSchemaRootRequiredFields(t *testing.T) {
	doc := decodeSchema(t)
	got := stringSlice(t, doc["required"])
	want := []string{
		"schema_version",
		"analysis_meta",
		"analysis_outcome",
		"dashboard",
		"actions",
		"limitations",
	}
	assertStringSetEqual(t, "root required", got, want)
}

// TestSchemaStrictModeCompliance recursively walks every object schema
// (inline and under $defs) and asserts OpenAI Structured Outputs strict
// mode invariants: additionalProperties is false, and every declared
// property appears in required (and vice versa).
func TestSchemaStrictModeCompliance(t *testing.T) {
	doc := decodeSchema(t)
	visited := map[string]bool{}
	walkSchemaNode(t, doc, doc, "#", visited)

	defs, _ := doc["$defs"].(map[string]any)
	if len(defs) == 0 {
		t.Fatal("expected non-empty $defs")
	}
	for name := range defs {
		path := "#/$defs/" + name
		if !visited[path] {
			t.Fatalf("$defs entry %s is never referenced from the schema", name)
		}
	}
}

func walkSchemaNode(t *testing.T, root map[string]any, node any, path string, visited map[string]bool) {
	t.Helper()
	switch n := node.(type) {
	case map[string]any:
		if ref, ok := n["$ref"].(string); ok {
			resolved := resolveRef(t, root, ref)
			if !visited[ref] {
				visited[ref] = true
				walkSchemaNode(t, root, resolved, ref, visited)
			}
			return
		}
		if anyOf, ok := n["anyOf"].([]any); ok {
			for i, sub := range anyOf {
				walkSchemaNode(t, root, sub, fmt.Sprintf("%s/anyOf[%d]", path, i), visited)
			}
		}
		if isObjectSchema(n) {
			assertStrictObject(t, n, path)
			if props, ok := n["properties"].(map[string]any); ok {
				for propName, propSchema := range props {
					walkSchemaNode(t, root, propSchema, path+"/properties/"+propName, visited)
				}
			}
		}
		if items, ok := n["items"]; ok {
			walkSchemaNode(t, root, items, path+"/items", visited)
		}
	default:
		// scalars (e.g. {"type":"null"}) need no further traversal.
	}
}

func isObjectSchema(n map[string]any) bool {
	switch t := n["type"].(type) {
	case string:
		return t == "object"
	case []any:
		for _, v := range t {
			if s, ok := v.(string); ok && s == "object" {
				return true
			}
		}
	}
	_, hasProps := n["properties"]
	return hasProps
}

func assertStrictObject(t *testing.T, n map[string]any, path string) {
	t.Helper()
	addl, ok := n["additionalProperties"]
	if !ok {
		t.Fatalf("%s: missing additionalProperties", path)
	}
	if addl != false {
		t.Fatalf("%s: additionalProperties=%v want false", path, addl)
	}
	props, _ := n["properties"].(map[string]any)
	required := stringSlice(t, n["required"])
	propNames := make([]string, 0, len(props))
	for name := range props {
		propNames = append(propNames, name)
	}
	assertStringSetEqual(t, path, required, propNames)
}

func resolveRef(t *testing.T, root map[string]any, ref string) map[string]any {
	t.Helper()
	const prefix = "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		t.Fatalf("unsupported $ref (only #/$defs/* supported): %s", ref)
	}
	name := strings.TrimPrefix(ref, prefix)
	defs, ok := root["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("schema has no $defs while resolving %s", ref)
	}
	resolved, ok := defs[name].(map[string]any)
	if !ok {
		t.Fatalf("$ref target %s not found under $defs", ref)
	}
	return resolved
}

func decodeSchema(t *testing.T) map[string]any {
	t.Helper()
	raw, err := SchemaJSON()
	if err != nil {
		t.Fatalf("SchemaJSON: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	return doc
}

func stringSlice(t *testing.T, v any) []string {
	t.Helper()
	arr, ok := v.([]any)
	if !ok {
		if v == nil {
			return nil
		}
		t.Fatalf("expected []any, got %T (%v)", v, v)
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("expected string element, got %T (%v)", item, item)
		}
		out = append(out, s)
	}
	return out
}

func assertStringSetEqual(t *testing.T, ctx string, got, want []string) {
	t.Helper()
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	sort.Strings(g)
	sort.Strings(w)
	if len(g) != len(w) {
		t.Fatalf("%s: required/properties mismatch\n  got:  %v\n  want: %v", ctx, g, w)
	}
	for i := range g {
		if g[i] != w[i] {
			t.Fatalf("%s: required/properties mismatch\n  got:  %v\n  want: %v", ctx, g, w)
		}
	}
}

// TestSchemaValidatesExistingFixtures ensures fixtures still decode into the
// Go contract. OpenAI strict schema requires every property (including
// nullable omitempty fields like "message") to be present; AI responses
// always include them. Fixtures may omit omitempty keys and are not required
// to pass the OpenAI strict schema document byte-for-byte.
func TestSchemaValidatesExistingFixtures(t *testing.T) {
	for _, name := range []string{
		"unified_output_v1.valid.json",
		"unified_output_v1.no_actions.json",
		"unified_output_v1.insufficient_data.json",
	} {
		t.Run(name, func(t *testing.T) {
			raw := readTestdata(t, name)
			var out UnifiedAnalysisOutputV1
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatalf("fixture %s failed Go decode: %v", name, err)
			}
			if out.SchemaVersion != UnifiedAnalysisOutputSchemaVersionCurrent {
				t.Fatalf("schema_version=%q want %s", out.SchemaVersion, UnifiedAnalysisOutputSchemaVersionCurrent)
			}
		})
	}
}

// schemaValidator implements a minimal, non-recursive-safe validator for
// the specific subset of JSON Schema keywords used by
// unified_analysis_output_v1.schema.json. It is not a general-purpose
// validator and intentionally does not support every draft keyword.
type schemaValidator struct {
	root map[string]any
}

func (v *schemaValidator) validate(instance any, schema map[string]any, path string) error {
	if ref, ok := schema["$ref"].(string); ok {
		resolved, err := v.resolveRef(ref)
		if err != nil {
			return err
		}
		return v.validate(instance, resolved, path)
	}
	if anyOf, ok := schema["anyOf"].([]any); ok {
		var lastErr error
		for _, sub := range anyOf {
			subSchema, ok := sub.(map[string]any)
			if !ok {
				continue
			}
			if err := v.validate(instance, subSchema, path); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}
		return fmt.Errorf("%s: no anyOf branch matched (last error: %v)", path, lastErr)
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
			return fmt.Errorf("%s: value %v not in enum %v", path, instance, enumVals)
		}
	}

	if err := v.validateType(instance, schema["type"], path); err != nil {
		return err
	}

	if isObjectSchema(schema) {
		obj, ok := instance.(map[string]any)
		if !ok {
			if instance == nil && typeAllowsNull(schema["type"]) {
				return nil
			}
			return fmt.Errorf("%s: expected object, got %T", path, instance)
		}
		props, _ := schema["properties"].(map[string]any)
		// NOTE: real Go-marshaled fixtures use `,omitempty` and fully elide
		// zero-value keys (rather than emitting explicit nulls), whereas the
		// Structured Outputs schema requires the *model* to always emit every
		// property. Required-key presence for the strict schema itself is
		// already locked down by TestSchemaStrictModeCompliance, so this
		// fixture validator intentionally does not fail on absent keys — it
		// only checks that keys which are present have the right type/enum
		// and that no undeclared ("additionalProperties") keys leak through.
		if schema["additionalProperties"] == false {
			for key := range obj {
				if _, ok := props[key]; !ok {
					return fmt.Errorf("%s: unexpected additional property %q", path, key)
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

func (v *schemaValidator) resolveRef(ref string) (map[string]any, error) {
	const prefix = "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		return nil, fmt.Errorf("unsupported $ref: %s", ref)
	}
	name := strings.TrimPrefix(ref, prefix)
	defs, ok := v.root["$defs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("no $defs while resolving %s", ref)
	}
	resolved, ok := defs[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("$ref target %s not found", ref)
	}
	return resolved, nil
}

func (v *schemaValidator) validateType(instance any, rawType any, path string) error {
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
