package chatv2

import (
	"context"
	"errors"
	"testing"
)

func TestPrompt15ToolCatalogSchemasAreRuntimeValidatable(t *testing.T) {
	if err := AssertCatalogSchemasRuntimeValidatable(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateToolArguments(ToolSearchEntities, map[string]any{"query": "Polaris 417", "limit": 8}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateToolArguments(ToolGetSKUContexts, map[string]any{"sku_ids": []any{5}}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateToolArguments(ToolGetActionContext, map[string]any{}); err != nil {
		t.Fatal(err)
	}
}

func TestPrompt15RejectsUnknownToolArgument(t *testing.T) {
	err := ValidateToolArguments(ToolSearchEntities, map[string]any{"query": "Polaris 417", "limit": 8, "garbage": true})
	if !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("err=%v", err)
	}
}

func TestPrompt15RejectsMissingRequiredToolArgument(t *testing.T) {
	err := ValidateToolArguments(ToolSearchEntities, map[string]any{"limit": 8})
	if !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("err=%v", err)
	}
}

func TestPrompt15RejectsWrongToolArgumentType(t *testing.T) {
	if err := ValidateToolArguments(ToolSearchEntities, map[string]any{"query": 123, "limit": 8}); !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("query type err=%v", err)
	}
	if err := ValidateToolArguments(ToolSearchEntities, map[string]any{"query": "Polaris", "limit": "8"}); !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("limit type err=%v", err)
	}
}

func TestPrompt15RejectsOutOfRangeToolArgument(t *testing.T) {
	err := ValidateToolArguments(ToolSearchEntities, map[string]any{"query": "Polaris", "limit": 1_000_000})
	if !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("err=%v", err)
	}
}

func TestPrompt15RejectsInvalidArrayToolArgument(t *testing.T) {
	if err := ValidateToolArguments(ToolGetSKUContexts, map[string]any{"sku_ids": []any{5, "not-an-id"}}); !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("mixed array err=%v", err)
	}
	ids := make([]any, 21)
	for i := range ids {
		ids[i] = i + 1
	}
	if err := ValidateToolArguments(ToolGetSKUContexts, map[string]any{"sku_ids": ids}); !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("maxItems err=%v", err)
	}
}

func TestPrompt15RejectsInvalidEnumToolArgument(t *testing.T) {
	err := ValidateToolArguments(ToolListSKUMetrics, map[string]any{
		"period": "forever", "date_from": nil, "date_to": nil, "limit": 10, "sort": nil,
	})
	if !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("err=%v", err)
	}
}

func TestPrompt15RejectsTrailingToolArgumentJSON(t *testing.T) {
	if _, err := DecodeToolArgumentsJSON(`{"query":"Polaris"} garbage`); !errors.Is(err, ErrMalformedToolArgs) {
		t.Fatalf("err=%v", err)
	}
	if _, err := DecodeToolArgumentsJSON(`{"query":"Polaris"}{"limit":8}`); !errors.Is(err, ErrMalformedToolArgs) {
		t.Fatalf("multi json err=%v", err)
	}
	args, err := DecodeToolArgumentsJSON(`{"query":"Polaris","limit":8}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateToolArguments(ToolSearchEntities, args); err != nil {
		t.Fatal(err)
	}
}

func TestPrompt15RejectsForbiddenBackendOwnedToolArgs(t *testing.T) {
	for _, key := range []string{"seller_account_id", "publication_id", "universe_id", "cutoff"} {
		args := map[string]any{"query": "Polaris", key: 1}
		if key != "seller_account_id" && key != "universe_id" && key != "cutoff" {
			// publication_id is unknown property (not in forbidden list) and must still fail
		}
		err := ValidateToolArguments(ToolSearchEntities, args)
		if err == nil {
			t.Fatalf("expected reject %s", key)
		}
		if key == "publication_id" && !errors.Is(err, ErrInvalidToolArgs) {
			t.Fatalf("%s err=%v", key, err)
		}
		if key != "publication_id" && !errors.Is(err, ErrForbiddenToolArg) && !errors.Is(err, ErrInvalidToolArgs) {
			t.Fatalf("%s err=%v", key, err)
		}
	}
}

func TestPrompt15RejectsDuplicateProviderCallIDBeforeExecution(t *testing.T) {
	ex := &ToolExecutor{Policy: DefaultToolPolicy()}
	ag := &Agent{Provider: NewFakeProvider().Script(ProviderResponse{ToolCalls: []ProviderToolCall{
		{CallID: "dup", Name: ToolSearchEntities, Args: map[string]any{"query": "Polaris 417", "limit": nil}},
		{CallID: "dup", Name: ToolGetSKUContexts, Args: map[string]any{"sku_ids": []any{5}}},
	}}), Tools: ex, Policy: DefaultToolPolicy(), Retry: RetryPolicy{MaxAttempts: 1}}
	out, err := ag.Run(context.Background(), FrozenTurnContext{Question: "q", HasFactual: true}, nil)
	if err == nil || !errors.Is(err, ErrInvalidToolCall) {
		t.Fatalf("err=%v", err)
	}
	if !out.Failed || out.ErrorCode != "INVALID_TOOL_CALL" {
		t.Fatalf("out=%#v", out)
	}
	if ex.Executions.Load() != 0 {
		t.Fatalf("executions=%d", ex.Executions.Load())
	}
	if len(out.ToolCalls) != 0 {
		t.Fatalf("records=%d", len(out.ToolCalls))
	}
}

func TestPrompt15RejectsEmptyProviderCallID(t *testing.T) {
	if err := ValidateProviderToolCalls([]ProviderToolCall{{CallID: " ", Name: ToolGetPublicationOverview, Args: map[string]any{}}}); !errors.Is(err, ErrInvalidToolCall) {
		t.Fatalf("err=%v", err)
	}
}

func TestPrompt15RejectsFractionalIntegerToolArgument(t *testing.T) {
	err := ValidateToolArguments(ToolSearchEntities, map[string]any{"query": "Polaris", "limit": 8.5})
	if !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("err=%v", err)
	}
}

func TestValidateProviderToolCallsSameToolSameID(t *testing.T) {
	err := ValidateProviderToolCalls([]ProviderToolCall{
		{CallID: "dup", Name: ToolSearchEntities, Args: map[string]any{"query": "a", "limit": nil}},
		{CallID: "dup", Name: ToolSearchEntities, Args: map[string]any{"query": "b", "limit": nil}},
	})
	if !errors.Is(err, ErrInvalidToolCall) {
		t.Fatalf("err=%v", err)
	}
}

func TestExecuteRejectsUnknownArgsWithoutCallingSearch(t *testing.T) {
	ex := &ToolExecutor{Policy: DefaultToolPolicy()}
	_, err := ex.Execute(context.Background(), FrozenTurnContext{HasFactual: true}, NewGroundingRegistry(), ToolSearchEntities, map[string]any{
		"query": "Polaris", "garbage": true,
	})
	if !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("err=%v", err)
	}
	if ex.Executions.Load() != 0 {
		t.Fatalf("executions=%d", ex.Executions.Load())
	}
}

func TestRequiredNullableToolArgMustBePresent(t *testing.T) {
	if err := ValidateToolArguments(ToolSearchEntities, map[string]any{"query": "Polaris 417"}); !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("omitted limit err=%v", err)
	}
	if err := ValidateToolArguments(ToolSearchEntities, map[string]any{"query": "Polaris 417", "limit": nil}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateToolArguments(ToolSearchEntities, map[string]any{"query": "Polaris 417", "limit": 8}); err != nil {
		t.Fatal(err)
	}
	metricsNull := map[string]any{"period": nil, "date_from": nil, "date_to": nil, "limit": nil, "sort": nil}
	if err := ValidateToolArguments(ToolListSKUMetrics, metricsNull); err != nil {
		t.Fatal(err)
	}
}

func TestPrompt15RejectsInvalidDomainAndAnswerModeEnums(t *testing.T) {
	if err := ValidateToolArguments(ToolGetAccountDomainFacts, map[string]any{
		"domain": "ABSOLUTELY_INVALID_DOMAIN", "period": nil, "date_from": nil, "date_to": nil,
	}); !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("domain err=%v", err)
	}
	if err := ValidateToolArguments(ToolGetAccountDomainFacts, map[string]any{
		"domain": "SALES", "period": nil, "date_from": nil, "date_to": nil,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateToolArguments(ToolSubmitTurnResult, validSubmitArgs("ok", "NOT_A_MODE")); !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("answer_mode err=%v", err)
	}
	if err := ValidateToolArguments(ToolSubmitTurnResult, validSubmitArgs("ok", "GENERAL_GUIDANCE")); err != nil {
		t.Fatal(err)
	}
}

func TestPrompt15RejectsInvalidExportFilterEnum(t *testing.T) {
	args := validSubmitArgs("Файл выгрузки подготовлен.", "EXPORT")
	args["export_dataset"] = map[string]any{
		"kind": "SKU", "period": "last_7d",
		"filters": []any{map[string]any{"field": "not_a_field", "operator": "eq", "value": "1"}},
	}
	if err := ValidateToolArguments(ToolSubmitTurnResult, args); !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("export field err=%v", err)
	}
	args["export_dataset"] = map[string]any{
		"kind": "SKU", "period": "last_7d",
		"filters": []any{map[string]any{"field": "canonical_sku_id", "operator": "eq", "value": "5"}},
	}
	if err := ValidateToolArguments(ToolSubmitTurnResult, args); err != nil {
		t.Fatal(err)
	}
}

func TestPrompt15SubmitTurnResultUnknownPropertyRejected(t *testing.T) {
	args := validSubmitArgs("ok", "GENERAL_GUIDANCE")
	args["garbage"] = "must be rejected"
	if err := ValidateToolArguments(ToolSubmitTurnResult, args); !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("err=%v", err)
	}
}

func TestPrompt15SubmitTurnResultWrongTypeRejected(t *testing.T) {
	args := validSubmitArgs("ok", "GENERAL_GUIDANCE")
	args["answer"] = 123
	if err := ValidateToolArguments(ToolSubmitTurnResult, args); !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("err=%v", err)
	}
}

func TestPrompt15RejectsDuplicateCallIDAcrossRounds(t *testing.T) {
	ex := &ToolExecutor{Policy: DefaultToolPolicy()}
	ag := &Agent{Provider: NewFakeProvider().Script(
		ProviderResponse{ToolCalls: []ProviderToolCall{
			{CallID: "call_1", Name: ToolGetPublicationOverview, Args: map[string]any{}},
		}},
		ProviderResponse{ToolCalls: []ProviderToolCall{
			{CallID: "call_1", Name: ToolGetActionContext, Args: map[string]any{}},
		}},
	), Tools: ex, Policy: DefaultToolPolicy(), Retry: RetryPolicy{MaxAttempts: 1}}
	out, err := ag.Run(context.Background(), FrozenTurnContext{Question: "q", HasFactual: true}, nil)
	if err == nil || !errors.Is(err, ErrInvalidToolCall) {
		t.Fatalf("err=%v", err)
	}
	if !out.Failed || out.ErrorCode != "INVALID_TOOL_CALL" {
		t.Fatalf("out=%#v", out)
	}
	if ex.Executions.Load() != 1 {
		t.Fatalf("executions=%d want first round only", ex.Executions.Load())
	}
}

func TestPrompt15AcceptsUniqueCallIDsAcrossRounds(t *testing.T) {
	ex := &ToolExecutor{Policy: DefaultToolPolicy()}
	ag := &Agent{Provider: NewFakeProvider().Script(
		ProviderResponse{ToolCalls: []ProviderToolCall{
			{CallID: "call_a", Name: ToolGetPublicationOverview, Args: map[string]any{}},
		}},
		ProviderResponse{ToolCalls: []ProviderToolCall{
			{CallID: "call_b", Name: ToolGetActionContext, Args: map[string]any{}},
		}},
		ProviderResponse{Final: &TurnResult{
			Answer: "ABC-анализ группирует ассортимент по вкладу. Это общее определение.", AnswerMode: ModeGeneralGuidance,
			GroundingHandles: []string{}, Limitations: []string{},
		}},
	), Tools: ex, Policy: DefaultToolPolicy(), Retry: RetryPolicy{MaxAttempts: 1}}
	out, err := ag.Run(context.Background(), FrozenTurnContext{Question: "q", HasFactual: true}, nil)
	if err != nil || out.Failed {
		t.Fatalf("err=%v out=%#v", err, out)
	}
	if ex.Executions.Load() != 2 {
		t.Fatalf("executions=%d", ex.Executions.Load())
	}
}

func TestPrompt15RejectsSubmitWithExecutableTools(t *testing.T) {
	ex := &ToolExecutor{Policy: DefaultToolPolicy()}
	ag := &Agent{Provider: NewFakeProvider().Script(ProviderResponse{
		ToolCalls: []ProviderToolCall{
			{CallID: "se1", Name: ToolSearchEntities, Args: map[string]any{"query": "Polaris 417", "limit": 8}},
			{CallID: "fin", Name: ToolSubmitTurnResult, Args: validSubmitArgs("ok", "GENERAL_GUIDANCE")},
		},
	}), Tools: ex, Policy: DefaultToolPolicy(), Retry: RetryPolicy{MaxAttempts: 1}}
	out, err := ag.Run(context.Background(), FrozenTurnContext{Question: "q", HasFactual: true}, nil)
	if err == nil || !errors.Is(err, ErrInvalidToolCall) {
		t.Fatalf("err=%v", err)
	}
	if !out.Failed || ex.Executions.Load() != 0 {
		t.Fatalf("out=%#v exec=%d", out, ex.Executions.Load())
	}
}

func TestPrompt15AcceptsValidSubmitTurnResult(t *testing.T) {
	ex := &ToolExecutor{Policy: DefaultToolPolicy()}
	ag := &Agent{Provider: NewFakeProvider().Script(ProviderResponse{
		ToolCalls: []ProviderToolCall{
			{CallID: "fin", Name: ToolSubmitTurnResult, Args: validSubmitArgs("ABC-анализ группирует ассортимент по вкладу. Это общее определение.", "GENERAL_GUIDANCE")},
		},
	}), Tools: ex, Policy: DefaultToolPolicy(), Retry: RetryPolicy{MaxAttempts: 1}}
	out, err := ag.Run(context.Background(), FrozenTurnContext{Question: "q", HasFactual: true}, nil)
	if err != nil || out.Failed || out.Result.Answer == "" {
		t.Fatalf("err=%v out=%#v", err, out)
	}
	if ex.Executions.Load() != 0 {
		t.Fatalf("submit must not execute as a data tool, exec=%d", ex.Executions.Load())
	}
}

func TestPrompt15OpenAISubmitUnknownPropertyRejected(t *testing.T) {
	tc := openaiToolCall{ID: "s1"}
	tc.Function.Name = ToolSubmitTurnResult
	tc.Function.Arguments = `{"answer":"ok","answer_mode":"GENERAL_GUIDANCE","referenced_entities":[],"grounding_handles":[],"limitations":[],"export_dataset":null,"garbage":true}`
	_, err := decodeOpenAIMessage("r1", []openaiToolCall{tc})
	if !errors.Is(err, ErrInvalidToolArgs) {
		t.Fatalf("err=%v", err)
	}
}

func TestPrompt15OpenAIRejectsPlainContentFinal(t *testing.T) {
	out, err := decodeOpenAIMessage("r1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.Final != nil {
		t.Fatalf("plain content must not become Final: %#v", out.Final)
	}
}

func validSubmitArgs(answer, mode string) map[string]any {
	return map[string]any{
		"answer":              answer,
		"answer_mode":         mode,
		"referenced_entities": []any{},
		"grounding_handles":   []any{},
		"limitations":         []any{},
		"export_dataset":      nil,
	}
}
