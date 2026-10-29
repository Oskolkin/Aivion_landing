//go:build integration

package analysis_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatv2"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factual"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestPrompt15ToolCatalogSchemasAreRuntimeValidatable(t *testing.T) {
	if err := chatv2.AssertCatalogSchemasRuntimeValidatable(); err != nil {
		t.Fatal(err)
	}
}

func TestPrompt15RejectsTrailingToolArgumentJSON(t *testing.T) {
	if _, err := chatv2.DecodeToolArgumentsJSON(`{"query":"Polaris"} garbage`); !errors.Is(err, chatv2.ErrMalformedToolArgs) {
		t.Fatalf("err=%v", err)
	}
}

func TestPrompt15RejectsDuplicateProviderCallIDBeforeExecution(t *testing.T) {
	f, _ := prompt14FirstPublication(t)
	cnt := &countingFactual{API: prompt15FactualAPI(t, f)}
	ex := &chatv2.ToolExecutor{API: cnt, Policy: chatv2.DefaultToolPolicy()}
	fp := chatv2.NewFakeProvider().Script(chatv2.ProviderResponse{ToolCalls: []chatv2.ProviderToolCall{
		{CallID: "dup", Name: chatv2.ToolSearchEntities, Args: map[string]any{"query": "Polaris 417", "limit": nil}},
		{CallID: "dup", Name: chatv2.ToolGetSKUContexts, Args: map[string]any{"sku_ids": []any{int64(1)}}},
	}})
	svc := prompt15ChatServiceAPI(t, f, fp, cnt)
	svc.Agent.Tools = ex
	var events []chatv2.StreamEvent
	res, err := svc.AskStream(context.Background(), chatv2.AskInput{
		SellerAccountID: f.seller, ClientRequestID: "p15-dup-call", Question: "Выгрузи продажи по Polaris 417",
	}, func(ev chatv2.StreamEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPrompt15InvalidToolOutcome(t, f, res, events, "INVALID_TOOL_CALL")
	if ex.Executions.Load() != 0 || cnt.searchCalls != 0 || cnt.skuBatch != 0 {
		t.Fatalf("executed search=%d skuBatch=%d exec=%d", cnt.searchCalls, cnt.skuBatch, ex.Executions.Load())
	}
}

func TestPrompt15RejectsEmptyProviderCallID(t *testing.T) {
	f, _ := prompt14FirstPublication(t)
	cnt := &countingFactual{API: prompt15FactualAPI(t, f)}
	ex := &chatv2.ToolExecutor{API: cnt, Policy: chatv2.DefaultToolPolicy()}
	fp := chatv2.NewFakeProvider().Script(chatv2.ProviderResponse{ToolCalls: []chatv2.ProviderToolCall{
		{CallID: "", Name: chatv2.ToolSearchEntities, Args: map[string]any{"query": "Polaris 417", "limit": nil}},
	}})
	svc := prompt15ChatServiceAPI(t, f, fp, cnt)
	svc.Agent.Tools = ex
	var events []chatv2.StreamEvent
	res, err := svc.AskStream(context.Background(), chatv2.AskInput{
		SellerAccountID: f.seller, ClientRequestID: "p15-empty-call", Question: "Сколько остатков Polaris 417?",
	}, func(ev chatv2.StreamEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPrompt15InvalidToolOutcome(t, f, res, events, "INVALID_TOOL_CALL")
	if ex.Executions.Load() != 0 || cnt.searchCalls != 0 {
		t.Fatalf("executed search=%d exec=%d", cnt.searchCalls, ex.Executions.Load())
	}
}

func TestPrompt15ToolAuditExactlyMatchesExecutedCalls(t *testing.T) {
	f, _ := prompt14FirstPublication(t)
	skuID := prompt15PolarisID(t, f)
	cnt := &countingFactual{API: prompt15FactualAPI(t, f)}
	ex := &chatv2.ToolExecutor{API: cnt, Policy: chatv2.DefaultToolPolicy()}
	fp := chatv2.NewFakeProvider().Script(
		chatv2.ProviderResponse{ToolCalls: []chatv2.ProviderToolCall{
			{CallID: "call_a", Name: chatv2.ToolSearchEntities, Args: map[string]any{"query": "Polaris 417", "limit": 8}},
			{CallID: "call_b", Name: chatv2.ToolGetSKUContexts, Args: map[string]any{"sku_ids": []any{skuID}}},
			{CallID: "call_c", Name: chatv2.ToolGetPublicationOverview, Args: map[string]any{}},
		}},
		chatv2.ProviderResponse{Final: &chatv2.TurnResult{
			Answer: "ABC-анализ группирует ассортимент по вкладу. Это общее определение.", AnswerMode: chatv2.ModeGeneralGuidance,
			GroundingHandles: []string{}, Limitations: []string{},
		}},
	)
	svc := prompt15ChatServiceAPI(t, f, fp, cnt)
	svc.Agent.Tools = ex
	res, err := svc.Ask(context.Background(), chatv2.AskInput{
		SellerAccountID: f.seller, ClientRequestID: "p15-audit-exact", Question: "Расскажи про кабинет",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.ErrorCode != "" {
		t.Fatalf("result %#v", res)
	}
	var rows, distinct int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*), count(DISTINCT call_id) FROM chat_turn_tool_calls WHERE turn_id=$1`, res.TurnID).Scan(&rows, &distinct)
	if int64(rows) != ex.Executions.Load() || rows != 3 || distinct != 3 {
		t.Fatalf("executed=%d rows=%d distinct=%d", ex.Executions.Load(), rows, distinct)
	}
}

func TestPrompt15RejectsUnknownToolArgument(t *testing.T) {
	f, _ := prompt14FirstPublication(t)
	cnt := &countingFactual{API: prompt15FactualAPI(t, f)}
	ex := &chatv2.ToolExecutor{API: cnt, Policy: chatv2.DefaultToolPolicy()}
	fp := chatv2.NewFakeProvider().Script(chatv2.ProviderResponse{ToolCalls: []chatv2.ProviderToolCall{
		{CallID: "se1", Name: chatv2.ToolSearchEntities, Args: map[string]any{"query": "Polaris 417", "limit": 8, "garbage": true}},
	}})
	svc := prompt15ChatServiceAPI(t, f, fp, cnt)
	svc.Agent.Tools = ex
	var events []chatv2.StreamEvent
	res, err := svc.AskStream(context.Background(), chatv2.AskInput{
		SellerAccountID: f.seller, ClientRequestID: "p15-unknown-arg", Question: "Найди Polaris 417",
	}, func(ev chatv2.StreamEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPrompt15InvalidToolOutcome(t, f, res, events, "INVALID_ARGS")
	if cnt.searchCalls != 0 || ex.Executions.Load() != 0 {
		t.Fatalf("search=%d exec=%d", cnt.searchCalls, ex.Executions.Load())
	}
}

func TestPrompt15RejectsMissingRequiredToolArgument(t *testing.T) {
	prompt15AssertInvalidArgs(t, chatv2.ProviderToolCall{
		CallID: "se1", Name: chatv2.ToolSearchEntities, Args: map[string]any{"limit": 8},
	}, "p15-missing-arg")
}

func TestPrompt15RejectsWrongToolArgumentType(t *testing.T) {
	prompt15AssertInvalidArgs(t, chatv2.ProviderToolCall{
		CallID: "se1", Name: chatv2.ToolSearchEntities, Args: map[string]any{"query": 123, "limit": 8},
	}, "p15-wrong-type")
}

func TestPrompt15RejectsOutOfRangeToolArgument(t *testing.T) {
	prompt15AssertInvalidArgs(t, chatv2.ProviderToolCall{
		CallID: "se1", Name: chatv2.ToolSearchEntities, Args: map[string]any{"query": "Polaris", "limit": 1_000_000},
	}, "p15-oob")
}

func TestPrompt15RejectsInvalidArrayToolArgument(t *testing.T) {
	prompt15AssertInvalidArgs(t, chatv2.ProviderToolCall{
		CallID: "sku1", Name: chatv2.ToolGetSKUContexts, Args: map[string]any{"sku_ids": []any{5, "not-an-id"}},
	}, "p15-bad-array")
}

func TestPrompt15RejectsInvalidEnumToolArgument(t *testing.T) {
	prompt15AssertInvalidArgs(t, chatv2.ProviderToolCall{
		CallID: "m1", Name: chatv2.ToolListSKUMetrics, Args: map[string]any{
			"period": "forever", "date_from": nil, "date_to": nil, "limit": 10, "sort": nil,
		},
	}, "p15-bad-enum")
}

func TestPrompt15ValidToolCallsStillExecute(t *testing.T) {
	f, _ := prompt14FirstPublication(t)
	skuID := prompt15PolarisID(t, f)
	cnt := &countingFactual{API: prompt15FactualAPI(t, f)}
	fp := chatv2.NewFakeProvider().Script(
		chatv2.ProviderResponse{ToolCalls: []chatv2.ProviderToolCall{
			{CallID: "se1", Name: chatv2.ToolSearchEntities, Args: map[string]any{"query": "Polaris 417", "limit": 8}},
		}},
		chatv2.ProviderResponse{Final: &chatv2.TurnResult{
			Answer: "ABC-анализ группирует ассортимент по вкладу. Это общее определение.", AnswerMode: chatv2.ModeGeneralGuidance,
			GroundingHandles: []string{}, Limitations: []string{},
		}},
	)
	svc := prompt15ChatServiceAPI(t, f, fp, cnt)
	res, err := svc.Ask(context.Background(), chatv2.AskInput{
		SellerAccountID: f.seller, ClientRequestID: "p15-valid-search", Question: "Найди Polaris 417",
	})
	if err != nil || res == nil || res.ErrorCode != "" {
		t.Fatalf("err=%v res=%#v", err, res)
	}
	if cnt.searchCalls != 1 {
		t.Fatalf("searchCalls=%d", cnt.searchCalls)
	}

	fp2 := chatv2.NewFakeProvider().Script(
		chatv2.ProviderResponse{ToolCalls: []chatv2.ProviderToolCall{
			{CallID: "sku1", Name: chatv2.ToolGetSKUContexts, Args: map[string]any{"sku_ids": []any{skuID}}},
		}},
		chatv2.ProviderResponse{Final: &chatv2.TurnResult{
			Answer: "ABC-анализ группирует ассортимент по вкладу. Это общее определение.", AnswerMode: chatv2.ModeGeneralGuidance,
			GroundingHandles: []string{}, Limitations: []string{},
		}},
	)
	cnt2 := &countingFactual{API: prompt15FactualAPI(t, f)}
	svc2 := prompt15ChatServiceAPI(t, f, fp2, cnt2)
	res2, err := svc2.Ask(context.Background(), chatv2.AskInput{
		SellerAccountID: f.seller, ClientRequestID: "p15-valid-sku", Question: "Контекст SKU",
	})
	if err != nil || res2 == nil || res2.ErrorCode != "" {
		t.Fatalf("sku err=%v res=%#v", err, res2)
	}
	if cnt2.skuBatch != 1 {
		t.Fatalf("skuBatch=%d", cnt2.skuBatch)
	}
}

func TestPrompt15RejectsDuplicateCallIDAcrossRounds(t *testing.T) {
	f, _ := prompt14FirstPublication(t)
	skuID := prompt15PolarisID(t, f)
	cnt := &countingFactual{API: prompt15FactualAPI(t, f)}
	ex := &chatv2.ToolExecutor{API: cnt, Policy: chatv2.DefaultToolPolicy()}
	fp := chatv2.NewFakeProvider().Script(
		chatv2.ProviderResponse{ToolCalls: []chatv2.ProviderToolCall{
			{CallID: "call_1", Name: chatv2.ToolSearchEntities, Args: map[string]any{"query": "Polaris 417", "limit": 8}},
		}},
		chatv2.ProviderResponse{ToolCalls: []chatv2.ProviderToolCall{
			{CallID: "call_1", Name: chatv2.ToolGetSKUContexts, Args: map[string]any{"sku_ids": []any{skuID}}},
		}},
	)
	svc := prompt15ChatServiceAPI(t, f, fp, cnt)
	svc.Agent.Tools = ex
	var events []chatv2.StreamEvent
	res, err := svc.AskStream(context.Background(), chatv2.AskInput{
		SellerAccountID: f.seller, ClientRequestID: "p15-dup-rounds", Question: "Найди Polaris 417",
	}, func(ev chatv2.StreamEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.ErrorCode != "INVALID_TOOL_CALL" {
		t.Fatalf("res=%#v", res)
	}
	if cnt.skuBatch != 0 {
		t.Fatalf("second tool executed skuBatch=%d", cnt.skuBatch)
	}
	if ex.Executions.Load() != 1 || cnt.searchCalls != 1 {
		t.Fatalf("first round exec=%d search=%d", ex.Executions.Load(), cnt.searchCalls)
	}
	var status string
	var asst *int64
	_ = f.pool.QueryRow(context.Background(), `SELECT status, assistant_message_id FROM chat_turns WHERE id=$1`, res.TurnID).Scan(&status, &asst)
	if status != "FAILED" || asst != nil {
		t.Fatalf("status=%s asst=%v", status, asst)
	}
	var dup int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_turn_tool_calls WHERE turn_id=$1 AND call_id='call_1'`, res.TurnID).Scan(&dup)
	if dup > 1 {
		t.Fatalf("duplicate audit rows=%d", dup)
	}
	done, errEvents := 0, 0
	for _, ev := range events {
		if ev.Type == "done" {
			done++
		}
		if ev.Type == "error" {
			errEvents++
		}
	}
	if done != 0 || errEvents != 1 {
		t.Fatalf("done=%d error=%d", done, errEvents)
	}
}

func TestPrompt15RejectsSubmitMixedWithExecutableTools(t *testing.T) {
	prompt15AssertInvalidToolCall(t, []chatv2.ProviderToolCall{
		{CallID: "se1", Name: chatv2.ToolSearchEntities, Args: map[string]any{"query": "Polaris 417", "limit": 8}},
		{CallID: "fin", Name: chatv2.ToolSubmitTurnResult, Args: map[string]any{
			"answer": "ok", "answer_mode": "GENERAL_GUIDANCE",
			"referenced_entities": []any{}, "grounding_handles": []any{}, "limitations": []any{}, "export_dataset": nil,
		}},
	}, "p15-mixed-submit")
}

func TestPrompt15RejectsSubmitTurnResultUnknownProperty(t *testing.T) {
	prompt15AssertInvalidArgs(t, chatv2.ProviderToolCall{
		CallID: "fin", Name: chatv2.ToolSubmitTurnResult, Args: map[string]any{
			"answer": "ok", "answer_mode": "GENERAL_GUIDANCE",
			"referenced_entities": []any{}, "grounding_handles": []any{}, "limitations": []any{},
			"export_dataset": nil, "garbage": true,
		},
	}, "p15-submit-garbage")
}

func TestPrompt15AcceptsValidSubmitTurnResult(t *testing.T) {
	f, _ := prompt14FirstPublication(t)
	cnt := &countingFactual{API: prompt15FactualAPI(t, f)}
	fp := chatv2.NewFakeProvider().Script(chatv2.ProviderResponse{ToolCalls: []chatv2.ProviderToolCall{
		{CallID: "fin", Name: chatv2.ToolSubmitTurnResult, Args: map[string]any{
			"answer":      "ABC-анализ группирует ассортимент по вкладу. Это общее определение.",
			"answer_mode": "GENERAL_GUIDANCE", "referenced_entities": []any{},
			"grounding_handles": []any{}, "limitations": []any{}, "export_dataset": nil,
		}},
	}})
	svc := prompt15ChatServiceAPI(t, f, fp, cnt)
	res, err := svc.Ask(context.Background(), chatv2.AskInput{
		SellerAccountID: f.seller, ClientRequestID: "p15-valid-submit", Question: "Что такое ABC?",
	})
	if err != nil || res == nil || res.ErrorCode != "" || res.Answer == "" {
		t.Fatalf("err=%v res=%#v", err, res)
	}
}

func TestPrompt15AcceptsUniqueCallIDsAcrossRounds(t *testing.T) {
	f, _ := prompt14FirstPublication(t)
	cnt := &countingFactual{API: prompt15FactualAPI(t, f)}
	ex := &chatv2.ToolExecutor{API: cnt, Policy: chatv2.DefaultToolPolicy()}
	fp := chatv2.NewFakeProvider().Script(
		chatv2.ProviderResponse{ToolCalls: []chatv2.ProviderToolCall{
			{CallID: "call_a", Name: chatv2.ToolSearchEntities, Args: map[string]any{"query": "Polaris 417", "limit": 8}},
		}},
		chatv2.ProviderResponse{ToolCalls: []chatv2.ProviderToolCall{
			{CallID: "call_b", Name: chatv2.ToolGetPublicationOverview, Args: map[string]any{}},
		}},
		chatv2.ProviderResponse{Final: &chatv2.TurnResult{
			Answer: "ABC-анализ группирует ассортимент по вкладу. Это общее определение.", AnswerMode: chatv2.ModeGeneralGuidance,
			GroundingHandles: []string{}, Limitations: []string{},
		}},
	)
	svc := prompt15ChatServiceAPI(t, f, fp, cnt)
	svc.Agent.Tools = ex
	res, err := svc.Ask(context.Background(), chatv2.AskInput{
		SellerAccountID: f.seller, ClientRequestID: "p15-unique-rounds", Question: "Найди Polaris 417",
	})
	if err != nil || res == nil || res.ErrorCode != "" {
		t.Fatalf("err=%v res=%#v", err, res)
	}
	if ex.Executions.Load() != 2 {
		t.Fatalf("exec=%d", ex.Executions.Load())
	}
	var rows, distinct int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*), count(DISTINCT call_id) FROM chat_turn_tool_calls WHERE turn_id=$1`, res.TurnID).Scan(&rows, &distinct)
	if rows != 2 || distinct != 2 {
		t.Fatalf("rows=%d distinct=%d", rows, distinct)
	}
}

func prompt15AssertInvalidToolCall(t *testing.T, calls []chatv2.ProviderToolCall, reqID string) {
	t.Helper()
	f, _ := prompt14FirstPublication(t)
	cnt := &countingFactual{API: prompt15FactualAPI(t, f)}
	ex := &chatv2.ToolExecutor{API: cnt, Policy: chatv2.DefaultToolPolicy()}
	fp := chatv2.NewFakeProvider().Script(chatv2.ProviderResponse{ToolCalls: calls})
	svc := prompt15ChatServiceAPI(t, f, fp, cnt)
	svc.Agent.Tools = ex
	var events []chatv2.StreamEvent
	res, err := svc.AskStream(context.Background(), chatv2.AskInput{
		SellerAccountID: f.seller, ClientRequestID: reqID, Question: "Найди Polaris 417",
	}, func(ev chatv2.StreamEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPrompt15InvalidToolOutcome(t, f, res, events, "INVALID_TOOL_CALL")
	if ex.Executions.Load() != 0 || cnt.searchCalls != 0 {
		t.Fatalf("executed search=%d exec=%d", cnt.searchCalls, ex.Executions.Load())
	}
}

func TestPrompt15FinalizeRejectsDuplicateToolAuditCallID(t *testing.T) {
	f, _ := prompt14FirstPublication(t)
	ctx := context.Background()
	store := chatv2.NewStore(f.pool)
	sess, err := store.CreateSession(ctx, f.seller, nil, "dup-audit")
	if err != nil {
		t.Fatal(err)
	}
	userMsgID, err := store.CreateMessage(ctx, sess.ID, f.seller, "user", "q", "question")
	if err != nil {
		t.Fatal(err)
	}
	turnID, err := store.InsertTurn(ctx, chatv2.TurnRow{
		SessionID: sess.ID, SellerID: f.seller, UserMessageID: userMsgID,
		AnswerMode: chatv2.ModeGeneralGuidance, ContextMode: chatv2.ContextGeneral, Status: chatv2.StatusValidating,
		ModelPolicyVersion: chatv2.ModelPolicyV1, ToolPolicyVersion: chatv2.ToolPolicyV1,
		MemoryPolicyVersion: chatv2.MemoryPolicyV1, PromptVersion: chatv2.PromptVersionV1,
		ResultSchemaVersion: chatv2.ResultSchemaV1, GroundingStatus: chatv2.GroundingNone,
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := chatv2.ToolCallRecord{CallID: "dup", Round: 1, Ordinal: 1, ToolKey: chatv2.ToolSearchEntities, Version: "v1", Status: "SUCCEEDED"}
	rec2 := rec
	rec2.Ordinal = 2
	_, ferr := store.FinalizeSuccessfulTurn(ctx, chatv2.FinalizeSuccessInput{
		SellerID: f.seller, SessionID: sess.ID, TurnID: turnID, UserMessageID: userMsgID,
		Result:    chatv2.TurnResult{Answer: "ok", AnswerMode: chatv2.ModeGeneralGuidance},
		ToolCalls: []chatv2.ToolCallRecord{rec, rec2},
	})
	if ferr == nil {
		t.Fatal("expected duplicate audit insert to fail")
	}
	if !strings.Contains(ferr.Error(), "finalize.tool_audit_insert") {
		t.Fatalf("err=%v", ferr)
	}
	var status string
	var asst *int64
	_ = f.pool.QueryRow(ctx, `SELECT status, assistant_message_id FROM chat_turns WHERE id=$1`, turnID).Scan(&status, &asst)
	if status != string(chatv2.StatusValidating) || asst != nil {
		t.Fatalf("status=%s asst=%v want rolled-back VALIDATING", status, asst)
	}
	var n int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM chat_turn_tool_calls WHERE turn_id=$1`, turnID).Scan(&n)
	if n != 0 {
		t.Fatalf("audit rows=%d", n)
	}
}

func prompt15FactualAPI(t *testing.T, f prompt81Fixture) factualapi.API {
	t.Helper()
	secret := strings.Repeat("t", 32)
	signer, err := factualapi.NewTokenSigner([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return factual.NewService(f.pool, signer, 0, 0)
}

func prompt15AssertInvalidArgs(t *testing.T, call chatv2.ProviderToolCall, reqID string) {
	t.Helper()
	f, _ := prompt14FirstPublication(t)
	cnt := &countingFactual{API: prompt15FactualAPI(t, f)}
	ex := &chatv2.ToolExecutor{API: cnt, Policy: chatv2.DefaultToolPolicy()}
	fp := chatv2.NewFakeProvider().Script(chatv2.ProviderResponse{ToolCalls: []chatv2.ProviderToolCall{call}})
	svc := prompt15ChatServiceAPI(t, f, fp, cnt)
	svc.Agent.Tools = ex
	var events []chatv2.StreamEvent
	res, err := svc.AskStream(context.Background(), chatv2.AskInput{
		SellerAccountID: f.seller, ClientRequestID: reqID, Question: "Найди Polaris 417",
	}, func(ev chatv2.StreamEvent) error {
		events = append(events, ev)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPrompt15InvalidToolOutcome(t, f, res, events, "INVALID_ARGS")
	if ex.Executions.Load() != 0 || cnt.searchCalls != 0 || cnt.skuBatch != 0 {
		t.Fatalf("executed search=%d sku=%d exec=%d", cnt.searchCalls, cnt.skuBatch, ex.Executions.Load())
	}
}

func assertPrompt15InvalidToolOutcome(t *testing.T, f prompt81Fixture, res *chatv2.AskResult, events []chatv2.StreamEvent, code string) {
	t.Helper()
	if res == nil || res.ErrorCode != code {
		t.Fatalf("res=%#v want %s", res, code)
	}
	if !strings.Contains(res.Answer, "Не удалось корректно обработать запрос") {
		t.Fatalf("seller copy=%q", res.Answer)
	}
	var status, errCode string
	var asst *int64
	_ = f.pool.QueryRow(context.Background(), `SELECT status, COALESCE(error_code,''), assistant_message_id FROM chat_turns WHERE id=$1`, res.TurnID).Scan(&status, &errCode, &asst)
	if status != "FAILED" || errCode != code || asst != nil {
		t.Fatalf("status=%s code=%s asst=%v", status, errCode, asst)
	}
	var audit, asstMsgs int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_turn_tool_calls WHERE turn_id=$1`, res.TurnID).Scan(&audit)
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_messages WHERE seller_account_id=$1 AND role='assistant' AND session_id=(SELECT session_id FROM chat_turns WHERE id=$2)`, f.seller, res.TurnID).Scan(&asstMsgs)
	if audit != 0 || asstMsgs != 0 {
		t.Fatalf("audit=%d assistant=%d", audit, asstMsgs)
	}
	done, errEvents := 0, 0
	for _, ev := range events {
		if ev.Type == "done" {
			done++
		}
		if ev.Type == "error" {
			errEvents++
		}
	}
	if done != 0 || errEvents != 1 {
		t.Fatalf("done=%d error=%d", done, errEvents)
	}
}
