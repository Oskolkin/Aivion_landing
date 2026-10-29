package chatv2

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

func providerToolCallFailureCode(err error) string {
	switch {
	case errors.Is(err, ErrForbiddenToolArg):
		return "FORBIDDEN_ARG"
	case errors.Is(err, ErrMalformedToolArgs), errors.Is(err, ErrInvalidToolArgs):
		return "INVALID_ARGS"
	case errors.Is(err, ErrInvalidToolCall):
		return "INVALID_TOOL_CALL"
	default:
		return "INVALID_TOOL_CALL"
	}
}

func CatalogByKey() map[string]ToolSpec {
	out := make(map[string]ToolSpec, len(CatalogV1()))
	for _, spec := range CatalogV1() {
		out[spec.Key] = spec
	}
	return out
}

func ValidateProviderToolCalls(calls []ProviderToolCall) error {
	return validateProviderToolCalls(calls, nil)
}

func validateProviderToolCalls(calls []ProviderToolCall, seen map[string]struct{}) error {
	local := make(map[string]struct{}, len(calls))
	ids := make([]string, 0, len(calls))
	for _, call := range calls {
		id := strings.TrimSpace(call.CallID)
		if id == "" {
			return fmt.Errorf("%w: empty call_id", ErrInvalidToolCall)
		}
		if _, exists := local[id]; exists {
			return fmt.Errorf("%w: duplicate call_id", ErrInvalidToolCall)
		}
		if seen != nil {
			if _, exists := seen[id]; exists {
				return fmt.Errorf("%w: duplicate call_id", ErrInvalidToolCall)
			}
		}
		local[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, call := range calls {
		if err := ValidateToolArguments(call.Name, call.Args); err != nil {
			return err
		}
	}
	if seen != nil {
		for _, id := range ids {
			seen[id] = struct{}{}
		}
	}
	return nil
}

func ValidateProviderResponse(resp *ProviderResponse, seenCallIDs map[string]struct{}) error {
	if resp == nil {
		return fmt.Errorf("%w: empty provider response", ErrInvalidToolCall)
	}
	submitCount := 0
	for _, call := range resp.ToolCalls {
		if strings.TrimSpace(call.Name) == ToolSubmitTurnResult {
			submitCount++
		}
	}
	if resp.Final != nil && len(resp.ToolCalls) > 0 {
		return fmt.Errorf("%w: mixed final and tool calls", ErrInvalidToolCall)
	}
	if submitCount > 0 && (submitCount != 1 || len(resp.ToolCalls) != 1 || resp.Final != nil) {
		return fmt.Errorf("%w: mixed submit_turn_result", ErrInvalidToolCall)
	}
	if resp.Final != nil {
		return ValidateTurnResult(*resp.Final)
	}
	if err := validateProviderToolCalls(resp.ToolCalls, seenCallIDs); err != nil {
		return err
	}
	if submitCount == 1 {
		fin, err := turnResultFromArgs(resp.ToolCalls[0].Args)
		if err != nil {
			return err
		}
		resp.Final = &fin
		resp.ToolCalls = nil
	}
	return nil
}

func ValidateTurnResult(res TurnResult) error {
	return ValidateToolArguments(ToolSubmitTurnResult, turnResultAsSchemaArgs(res))
}

func turnResultFromArgs(args map[string]any) (TurnResult, error) {
	if err := ValidateToolArguments(ToolSubmitTurnResult, args); err != nil {
		return TurnResult{}, err
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return TurnResult{}, fmt.Errorf("%w: %v", ErrMalformedToolArgs, err)
	}
	var fin TurnResult
	if err := json.Unmarshal(raw, &fin); err != nil {
		return TurnResult{}, fmt.Errorf("%w: %v", ErrMalformedToolArgs, err)
	}
	return fin, nil
}

func turnResultAsSchemaArgs(res TurnResult) map[string]any {
	ents := make([]any, 0, len(res.ReferencedEntities))
	for _, e := range res.ReferencedEntities {
		var label any
		if strings.TrimSpace(e.Label) == "" {
			label = nil
		} else {
			label = e.Label
		}
		ents = append(ents, map[string]any{"kind": e.Kind, "id": e.ID, "label": label})
	}
	var export any
	if res.ExportDataset != nil {
		filters := make([]any, 0, len(res.ExportDataset.Filters))
		for _, f := range res.ExportDataset.Filters {
			filters = append(filters, map[string]any{"field": f.Field, "operator": f.Operator, "value": f.Value})
		}
		var kind any
		if strings.TrimSpace(res.ExportDataset.Kind) == "" {
			kind = nil
		} else {
			kind = res.ExportDataset.Kind
		}
		var period any
		if strings.TrimSpace(res.ExportDataset.Period) == "" {
			period = nil
		} else {
			period = res.ExportDataset.Period
		}
		export = map[string]any{"kind": kind, "period": period, "filters": filters}
	}
	handles := make([]any, 0, len(res.GroundingHandles))
	for _, h := range res.GroundingHandles {
		handles = append(handles, h)
	}
	lims := make([]any, 0, len(res.Limitations))
	for _, l := range res.Limitations {
		lims = append(lims, l)
	}
	return map[string]any{
		"answer":              res.Answer,
		"answer_mode":         string(res.AnswerMode),
		"referenced_entities": ents,
		"grounding_handles":   handles,
		"limitations":         lims,
		"export_dataset":      export,
	}
}

func ValidateToolArguments(toolKey string, args map[string]any) error {
	spec, ok := CatalogByKey()[strings.TrimSpace(toolKey)]
	if !ok {
		return fmt.Errorf("%w: unknown tool", ErrInvalidToolCall)
	}
	if args == nil {
		args = map[string]any{}
	}
	if err := RejectForbiddenArgs(args); err != nil {
		return err
	}
	if err := validateJSONSchema(spec.Schema, args); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidToolArgs, err)
	}
	return nil
}

func DecodeToolArgumentsJSON(raw string) (map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var args map[string]any
	if err := dec.Decode(&args); err != nil {
		return nil, fmt.Errorf("%w: invalid json", ErrMalformedToolArgs)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("%w: trailing json", ErrMalformedToolArgs)
	}
	if args == nil {
		args = map[string]any{}
	}
	return args, nil
}

func validateJSONSchema(schema map[string]any, value any) error {
	if schema == nil {
		return fmt.Errorf("missing schema")
	}
	types := schemaTypes(schema)
	if len(types) == 0 {
		return fmt.Errorf("missing type")
	}
	if value == nil {
		if schemaAllowsType(types, "null") {
			return nil
		}
		return fmt.Errorf("null not allowed")
	}
	matched := false
	var last error
	for _, typ := range types {
		if typ == "null" {
			continue
		}
		if err := validateTypedValue(schema, typ, value); err != nil {
			last = err
			continue
		}
		matched = true
		break
	}
	if !matched {
		if last != nil {
			return last
		}
		return fmt.Errorf("value does not match schema type")
	}
	if enum, ok := schemaEnumValues(schema["enum"]); ok && len(enum) > 0 {
		if !enumContains(enum, value) {
			return fmt.Errorf("value not in enum")
		}
	}
	return nil
}

func validateTypedValue(schema map[string]any, typ string, value any) error {
	switch typ {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("expected object")
		}
		return validateObject(schema, obj)
	case "array":
		arr, err := asAnySlice(value)
		if err != nil {
			return err
		}
		if max, ok := schemaInt(schema["maxItems"]); ok && len(arr) > max {
			return fmt.Errorf("array longer than maxItems")
		}
		if min, ok := schemaInt(schema["minItems"]); ok && len(arr) < min {
			return fmt.Errorf("array shorter than minItems")
		}
		itemSchema, _ := schema["items"].(map[string]any)
		if itemSchema == nil {
			return nil
		}
		for _, item := range arr {
			if err := validateJSONSchema(itemSchema, item); err != nil {
				return err
			}
		}
		return nil
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string")
		}
		return nil
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean")
		}
		return nil
	case "integer":
		n, err := asStrictInt64(value)
		if err != nil {
			return err
		}
		if min, ok := schemaInt(schema["minimum"]); ok && n < int64(min) {
			return fmt.Errorf("integer below minimum")
		}
		if max, ok := schemaInt(schema["maximum"]); ok && n > int64(max) {
			return fmt.Errorf("integer above maximum")
		}
		return nil
	case "number":
		if _, err := asJSONFloat(value); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported schema type %q", typ)
	}
}

func validateObject(schema map[string]any, obj map[string]any) error {
	if ap, ok := schema["additionalProperties"].(bool); ok && ap {
		return fmt.Errorf("additionalProperties must be false")
	}
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		props = map[string]any{}
	}
	for key := range obj {
		if _, ok := props[key]; !ok {
			return fmt.Errorf("unknown property")
		}
	}
	required := schemaRequired(schema)
	for _, key := range required {
		if _, present := obj[key]; !present {
			return fmt.Errorf("missing required property")
		}
	}
	for key, raw := range obj {
		child, _ := props[key].(map[string]any)
		if raw == nil {
			if schemaAllowsNull(child) {
				continue
			}
			return fmt.Errorf("property cannot be null")
		}
		if err := validateJSONSchema(child, raw); err != nil {
			return err
		}
	}
	return nil
}

func schemaRequired(schema map[string]any) []string {
	switch t := schema["required"].(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func schemaEnumValues(v any) ([]any, bool) {
	switch x := v.(type) {
	case []any:
		return x, true
	case []string:
		out := make([]any, len(x))
		for i := range x {
			out[i] = x[i]
		}
		return out, true
	default:
		return nil, false
	}
}

func schemaAllowsNull(schema map[string]any) bool {
	if schema == nil {
		return false
	}
	return schemaAllowsType(schemaTypes(schema), "null")
}

func schemaTypes(schema map[string]any) []string {
	switch t := schema["type"].(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func schemaAllowsType(types []string, want string) bool {
	for _, t := range types {
		if t == want {
			return true
		}
	}
	return false
}

func schemaInt(v any) (int, bool) {
	n, err := asStrictInt64(v)
	if err != nil {
		return 0, false
	}
	return int(n), true
}

func asAnySlice(v any) ([]any, error) {
	switch t := v.(type) {
	case []any:
		return t, nil
	case []int64:
		out := make([]any, len(t))
		for i, n := range t {
			out[i] = n
		}
		return out, nil
	case []int:
		out := make([]any, len(t))
		for i, n := range t {
			out[i] = n
		}
		return out, nil
	case []string:
		out := make([]any, len(t))
		for i, s := range t {
			out[i] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("expected array")
	}
}

func asStrictInt64(v any) (int64, error) {
	switch t := v.(type) {
	case int:
		return int64(t), nil
	case int32:
		return int64(t), nil
	case int64:
		return t, nil
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) || t != math.Trunc(t) {
			return 0, fmt.Errorf("expected integer")
		}
		return int64(t), nil
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i, nil
		}
		f, err := t.Float64()
		if err != nil || f != math.Trunc(f) {
			return 0, fmt.Errorf("expected integer")
		}
		return int64(f), nil
	default:
		return 0, fmt.Errorf("expected integer")
	}
}

func asJSONFloat(v any) (float64, error) {
	switch t := v.(type) {
	case float64:
		return t, nil
	case int:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case json.Number:
		return t.Float64()
	default:
		return 0, fmt.Errorf("expected number")
	}
}

func enumContains(enum []any, value any) bool {
	for _, item := range enum {
		if jsonEqual(item, value) {
			return true
		}
	}
	return false
}

func jsonEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if ia, err := asStrictInt64(a); err == nil {
		if ib, err := asStrictInt64(b); err == nil {
			return ia == ib
		}
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func AssertCatalogSchemasRuntimeValidatable() error {
	seen := map[string]struct{}{}
	for _, spec := range CatalogV1() {
		if strings.TrimSpace(spec.Key) == "" {
			return fmt.Errorf("empty tool key")
		}
		if _, ok := seen[spec.Key]; ok {
			return fmt.Errorf("duplicate tool %s", spec.Key)
		}
		seen[spec.Key] = struct{}{}
		if err := assertOpenAIStrictSchema(spec.Schema, spec.Key); err != nil {
			return err
		}
		empty := map[string]any{}
		err := ValidateToolArguments(spec.Key, empty)
		if err == nil {
			continue
		}
		if !errors.Is(err, ErrInvalidToolArgs) && !errors.Is(err, ErrInvalidToolCall) && !errors.Is(err, ErrForbiddenToolArg) {
			return fmt.Errorf("%s validator unavailable: %v", spec.Key, err)
		}
	}
	return nil
}

func assertOpenAIStrictSchema(schema map[string]any, path string) error {
	typ := schema["type"]
	switch t := typ.(type) {
	case string:
		if t != "object" {
			return validateNonObjectSchema(schema, path)
		}
	case []any:
		hasObject := false
		for _, item := range t {
			if s, ok := item.(string); ok && s == "object" {
				hasObject = true
				break
			}
		}
		if !hasObject {
			return validateNonObjectSchema(schema, path)
		}
	default:
		return fmt.Errorf("%s missing type", path)
	}
	if ap, ok := schema["additionalProperties"].(bool); ok && ap {
		return fmt.Errorf("%s additionalProperties must be false", path)
	}
	if _, ok := schema["additionalProperties"].(map[string]any); ok {
		return fmt.Errorf("%s additionalProperties must be false", path)
	}
	props, _ := schema["properties"].(map[string]any)
	req, _ := schema["required"].([]string)
	reqSet := map[string]struct{}{}
	for _, k := range req {
		reqSet[k] = struct{}{}
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, ok := reqSet[k]; !ok {
			return fmt.Errorf("%s missing required %s", path, k)
		}
		child, _ := props[k].(map[string]any)
		if child == nil {
			continue
		}
		if err := assertOpenAIStrictSchema(child, path+"."+k); err != nil {
			return err
		}
		if items, ok := child["items"].(map[string]any); ok {
			if err := assertOpenAIStrictSchema(items, path+"."+k+"[]"); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateNonObjectSchema(schema map[string]any, path string) error {
	if enum, ok := schemaEnumValues(schema["enum"]); ok {
		if types, ok := schema["type"].([]any); ok {
			allowsNull := false
			for _, item := range types {
				if s, ok := item.(string); ok && s == "null" {
					allowsNull = true
				}
			}
			if allowsNull {
				hasNull := false
				for _, e := range enum {
					if e == nil {
						hasNull = true
					}
				}
				if !hasNull {
					return fmt.Errorf("%s nullable enum must include null or drop enum", path)
				}
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		return assertOpenAIStrictSchema(items, path+"[]")
	}
	return nil
}
