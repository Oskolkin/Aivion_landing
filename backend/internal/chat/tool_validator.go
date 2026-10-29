package chat

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	MaxToolsPerPlan          = 5
	MaxSameToolCallsPerPlan  = 2
	MaxToolDateRangeDays     = 90
	DefaultToolDateRangeDays = 30
	// DefaultChatDateRangeDays is the default analytics window when the user does not specify a period.
	DefaultChatDateRangeDays = 7
	MaxContextTargetBytes    = 50 * 1024
)

type ToolPlanValidator struct {
	registry         *ToolRegistry
	MaxTools         int
	MaxSameToolCalls int
	DefaultLimit     int
	MaxLimit         int
	MaxDateRangeDays int
	MaxContextBytes  int
}

func NewToolPlanValidator(registry *ToolRegistry) *ToolPlanValidator {
	return &ToolPlanValidator{
		registry:         registry,
		MaxTools:         MaxToolsPerPlan,
		MaxSameToolCalls: MaxSameToolCallsPerPlan,
		DefaultLimit:     DefaultToolLimit,
		MaxLimit:         MaxDefaultToolLimit,
		MaxDateRangeDays: MaxToolDateRangeDays,
		MaxContextBytes:  MaxContextTargetBytes,
	}
}

func (v *ToolPlanValidator) Validate(plan *ToolPlan) (*ValidatedToolPlan, error) {
	if plan == nil {
		return nil, errors.New("tool plan is required")
	}
	if plan.Confidence < 0 || plan.Confidence > 1 {
		return nil, fmt.Errorf("invalid confidence: %v", plan.Confidence)
	}
	coercedIntentWarning := ""
	if !isKnownIntent(plan.Intent) {
		// Planner occasionally invents free-form intent labels for conversational turns.
		coercedIntentWarning = fmt.Sprintf("unknown intent %q coerced to general_overview", plan.Intent)
		plan.Intent = ChatIntentGeneralOverview
	}
	if plan.Intent == ChatIntentUnsupported {
		if len(plan.ToolCalls) > 0 {
			return nil, errors.New("unsupported intent requires empty tool_calls")
		}
		if plan.UnsupportedReason == nil || strings.TrimSpace(*plan.UnsupportedReason) == "" {
			return nil, errors.New("unsupported intent requires unsupported_reason")
		}
		return &ValidatedToolPlan{
			Intent:            plan.Intent,
			ToolCalls:         []ToolCall{},
			Assumptions:       append([]string(nil), plan.Assumptions...),
			Warnings:          []string{},
			UnsupportedReason: plan.UnsupportedReason,
		}, nil
	}
	if len(plan.ToolCalls) == 0 {
		// Conversational / advisory turns may legitimately need no cabinet tools.
		switch plan.Intent {
		case ChatIntentGeneralOverview, ChatIntentUnknown:
			warnings := make([]string, 0, 1)
			normalizedLanguage, languageWarning := normalizeLanguage(plan.Language)
			if languageWarning != "" {
				warnings = append(warnings, languageWarning)
			}
			_ = normalizedLanguage
			warnings = append(warnings, "no tools; answering without cabinet tool reads")
			if coercedIntentWarning != "" {
				warnings = append(warnings, coercedIntentWarning)
			}
			return &ValidatedToolPlan{
				Intent:      plan.Intent,
				ToolCalls:   []ToolCall{},
				Assumptions: append([]string(nil), plan.Assumptions...),
				Warnings:    warnings,
			}, nil
		default:
			return nil, errors.New("non-unsupported intent requires at least one tool call")
		}
	}
	if len(plan.ToolCalls) > v.MaxTools {
		return nil, fmt.Errorf("too many tools requested: %d > %d", len(plan.ToolCalls), v.MaxTools)
	}

	warnings := make([]string, 0)
	if coercedIntentWarning != "" {
		warnings = append(warnings, coercedIntentWarning)
	}
	normalizedLanguage, languageWarning := normalizeLanguage(plan.Language)
	if languageWarning != "" {
		warnings = append(warnings, languageWarning)
	}
	_ = normalizedLanguage

	if estimatePlanSizeBytes(plan) > v.MaxContextBytes {
		return nil, fmt.Errorf("tool plan exceeds context size limit: %d bytes", v.MaxContextBytes)
	}

	seenByTool := map[string]int{}
	outCalls := make([]ToolCall, 0, len(plan.ToolCalls))
	for _, call := range plan.ToolCalls {
		if looksLikeWriteOrRawSemantic(call.Name) {
			return nil, fmt.Errorf("tool name contains forbidden write/raw semantic: %s", call.Name)
		}
		seenByTool[call.Name]++
		if seenByTool[call.Name] > v.MaxSameToolCalls {
			return nil, fmt.Errorf("duplicate tool call: %s", call.Name)
		}
		def, ok := v.registry.Get(call.Name)
		if !ok {
			return nil, fmt.Errorf("unknown tool: %s", call.Name)
		}
		if !def.ReadOnly {
			return nil, fmt.Errorf("tool is not read-only: %s", call.Name)
		}
		if !containsIntent(def.SupportedIntents, plan.Intent) {
			return nil, fmt.Errorf("tool %s is not supported for intent %s", call.Name, plan.Intent)
		}
		for argName := range call.Args {
			if isForbiddenToolArg(argName) {
				return nil, fmt.Errorf("forbidden tool arg: %s", argName)
			}
			if _, ok := def.AllowedArgs[argName]; !ok {
				return nil, fmt.Errorf("arg %s is not allowed for tool %s", argName, call.Name)
			}
		}
		cleanArgs := map[string]any{}
		for k, val := range call.Args {
			if isForbiddenToolArg(k) {
				return nil, fmt.Errorf("forbidden tool arg: %s", k)
			}
			if containsSQLLikeString(val) {
				return nil, fmt.Errorf("arg %s contains forbidden sql/write semantic for tool %s", k, call.Name)
			}
			argDef, ok := def.AllowedArgs[k]
			if !ok {
				return nil, fmt.Errorf("arg %s is not allowed for tool %s", k, call.Name)
			}
			switch argDef.Type {
			case "integer":
				intVal, ok := toIntStrict(val)
				if !ok {
					return nil, fmt.Errorf("invalid integer arg %s for tool %s", k, call.Name)
				}
				if argDef.MinInt != nil && intVal < *argDef.MinInt {
					return nil, fmt.Errorf("arg %s is below minimum for tool %s", k, call.Name)
				}
				if argDef.MaxInt != nil && intVal > *argDef.MaxInt {
					if k == "limit" {
						intVal = *argDef.MaxInt
						warnings = append(warnings, fmt.Sprintf("limit for %s was clamped to %d", call.Name, intVal))
					} else {
						return nil, fmt.Errorf("arg %s is above maximum for tool %s", k, call.Name)
					}
				}
				cleanArgs[k] = intVal
			case "string":
				s, ok := val.(string)
				if !ok {
					return nil, fmt.Errorf("invalid string arg %s for tool %s", k, call.Name)
				}
				if len(argDef.AllowedValues) > 0 && !containsString(argDef.AllowedValues, s) {
					return nil, fmt.Errorf("arg %s has unsupported value for tool %s", k, call.Name)
				}
				cleanArgs[k] = s
			case "array<string>":
				values, ok := toStringSlice(val)
				if !ok {
					return nil, fmt.Errorf("invalid array<string> arg %s for tool %s", k, call.Name)
				}
				if len(argDef.AllowedValues) > 0 {
					for _, value := range values {
						if !containsString(argDef.AllowedValues, value) {
							return nil, fmt.Errorf("arg %s has unsupported value %s for tool %s", k, value, call.Name)
						}
					}
				}
				cleanArgs[k] = values
			case "date":
				dt, ok, err := parseToolDate(val)
				if err != nil {
					return nil, fmt.Errorf("invalid date arg %s for tool %s: %w", k, call.Name, err)
				}
				if ok {
					cleanArgs[k] = dt.Format("2006-01-02")
				}
			default:
				cleanArgs[k] = val
			}
		}
		for argName, argDef := range def.AllowedArgs {
			if argDef.Required {
				if _, ok := cleanArgs[argName]; !ok {
					return nil, fmt.Errorf("missing required arg %s for tool %s", argName, call.Name)
				}
			}
			if _, ok := cleanArgs[argName]; !ok && argDef.Default != nil {
				cleanArgs[argName] = argDef.Default
			}
		}
		for argName, value := range def.DefaultArgs {
			if _, ok := cleanArgs[argName]; !ok {
				cleanArgs[argName] = value
			}
		}
		if _, ok := cleanArgs["limit"]; !ok && def.MaxLimit > 1 {
			if value, ok := def.DefaultArgs["limit"]; ok {
				cleanArgs["limit"] = value
			} else {
				cleanArgs["limit"] = v.DefaultLimit
			}
		}
		if limitRaw, ok := cleanArgs["limit"]; ok {
			limit, ok := toIntStrict(limitRaw)
			if !ok {
				return nil, fmt.Errorf("invalid limit for tool %s", call.Name)
			}
			if limit <= 0 {
				return nil, fmt.Errorf("invalid limit for tool %s", call.Name)
			}
			toolMax := def.MaxLimit
			if argDef, ok := def.AllowedArgs["limit"]; ok && argDef.MaxInt != nil && *argDef.MaxInt < toolMax {
				toolMax = *argDef.MaxInt
			}
			if toolMax <= 0 {
				toolMax = v.MaxLimit
			}
			if limit > toolMax {
				limit = toolMax
				warnings = append(warnings, fmt.Sprintf("limit for %s was clamped to %d", call.Name, limit))
			}
			cleanArgs["limit"] = limit
		}
		if err := validateDateRange(call.Name, cleanArgs, v.MaxDateRangeDays); err != nil {
			return nil, err
		}
		if call.Name == ToolGetSKUContext {
			_, hasSKU := cleanArgs["sku"]
			offerID, hasOffer := cleanArgs["offer_id"]
			if !hasSKU && (!hasOffer || strings.TrimSpace(fmt.Sprintf("%v", offerID)) == "") {
				return nil, fmt.Errorf("tool %s requires sku or offer_id", call.Name)
			}
		}
		outCalls = append(outCalls, ToolCall{Name: call.Name, Args: cleanArgs})
	}

	return &ValidatedToolPlan{
		Intent:            plan.Intent,
		ToolCalls:         outCalls,
		Assumptions:       append([]string(nil), plan.Assumptions...),
		Warnings:          warnings,
		UnsupportedReason: plan.UnsupportedReason,
	}, nil
}

func isForbiddenToolArg(name string) bool {
	normalized := normalizeToken(name)
	forbidden := map[string]struct{}{
		"selleraccountid": {},
		"userid":          {},
		"apikey":          {},
		"token":           {},
		"authorization":   {},
		"password":        {},
		"secret":          {},
		"sql":             {},
		"rawquery":        {},
		"table":           {},
		"database":        {},
		"rawdata":         {},
		"rawpayload":      {},
		"write":           {},
		"action":          {},
	}
	_, ok := forbidden[normalized]
	return ok
}

func looksLikeWriteOrRawSemantic(value string) bool {
	lower := strings.ToLower(value)
	needles := []string{"sql", "raw", "write", "update", "delete", "create", "mutate", "action"}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func containsSQLLikeString(v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(s))
	needles := []string{"select ", "insert ", "update ", "delete ", "drop ", "alter ", "truncate ", "create "}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func normalizeToken(v string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(v)), "_", "")
}

func isKnownIntent(intent ChatIntent) bool {
	switch intent {
	case ChatIntentPriorities,
		ChatIntentExplainRecommendation,
		ChatIntentUnsafeAds,
		ChatIntentAdLoss,
		ChatIntentSales,
		ChatIntentStock,
		ChatIntentAdvertising,
		ChatIntentPricing,
		ChatIntentAlerts,
		ChatIntentRecommendations,
		ChatIntentABCAnalysis,
		ChatIntentGeneralOverview,
		ChatIntentUnknown,
		ChatIntentUnsupported:
		return true
	default:
		return false
	}
}

func containsIntent(intents []ChatIntent, target ChatIntent) bool {
	for _, intent := range intents {
		if intent == target {
			return true
		}
	}
	return false
}

func normalizeLanguage(language string) (string, string) {
	lang := strings.ToLower(strings.TrimSpace(language))
	switch lang {
	case "", "unknown":
		return "unknown", ""
	case "ru", "en":
		return lang, ""
	default:
		return "unknown", fmt.Sprintf("language %q is not supported and was normalized to unknown", language)
	}
}

func parseToolDate(value any) (time.Time, bool, error) {
	if value == nil {
		return time.Time{}, false, nil
	}
	s, ok := value.(string)
	if !ok {
		return time.Time{}, false, errors.New("date must be a string")
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false, err
	}
	return t, true, nil
}

func validateDateRange(toolName string, args map[string]any, maxDays int) error {
	from, hasFrom, err := parseToolDate(args["date_from"])
	if err != nil {
		return fmt.Errorf("tool %s has invalid date_from: %w", toolName, err)
	}
	to, hasTo, err := parseToolDate(args["date_to"])
	if err != nil {
		return fmt.Errorf("tool %s has invalid date_to: %w", toolName, err)
	}
	if _, _, err := parseToolDate(args["as_of_date"]); err != nil {
		return fmt.Errorf("tool %s has invalid as_of_date: %w", toolName, err)
	}
	if hasFrom && hasTo {
		if from.After(to) {
			return fmt.Errorf("tool %s has date_from after date_to", toolName)
		}
		if int(to.Sub(from).Hours()/24) > maxDays {
			return fmt.Errorf("tool %s date range exceeds %d days", toolName, maxDays)
		}
	}
	return nil
}

func estimatePlanSizeBytes(plan *ToolPlan) int {
	size := len(string(plan.Intent)) + len(plan.Language)
	for _, assumption := range plan.Assumptions {
		size += len(assumption)
	}
	for _, call := range plan.ToolCalls {
		size += len(call.Name)
		for key, value := range call.Args {
			size += len(key) + len(fmt.Sprintf("%v", value))
		}
	}
	return size
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func toStringSlice(v any) ([]string, bool) {
	switch x := v.(type) {
	case []string:
		return x, true
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func toIntStrict(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float64:
		if math.Trunc(x) != x {
			return 0, false
		}
		return int(x), true
	case float32:
		if math.Trunc(float64(x)) != float64(x) {
			return 0, false
		}
		return int(x), true
	default:
		return 0, false
	}
}
