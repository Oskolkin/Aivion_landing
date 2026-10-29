//go:build liveopenai

package eval_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatv2"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/eval"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

func requireLive(t *testing.T) string {
	t.Helper()
	if !eval.LiveEnabled() {
		t.Skip("set OPENAI_LIVE_EVAL=1 and OPENAI_API_KEY")
	}
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" {
		t.Skip("OPENAI_API_KEY empty")
	}
	return key
}

func TestPrompt16LiveOpenAIDiscovery(t *testing.T) {
	key := requireLive(t)
	budget := eval.LoadBudget()
	if err := budget.Consume(); err != nil {
		t.Fatal(err)
	}
	client := openaix.NewResponsesClient(http.DefaultClient, key, "")
	p := &discovery.OpenAIProvider{Client: client, Model: eval.ResolveConfiguredModels().Discovery.Configured}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	resp, err := p.Discover(ctx, discovery.Request{
		Model: eval.ResolveConfiguredModels().Discovery.Configured, Instructions: discovery.StableInstructionsV1,
		PayloadJSON: []byte(`{"frozen":true,"ordered_amount":"300"}`),
		Schema:      discovery.OutputJSONSchema(), SchemaName: discovery.OutputSchemaNameV1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.OutputText == "" {
		t.Fatal("empty discovery output")
	}
	if resp.Usage.TotalTokens == 0 && resp.Usage.InputTokens == 0 {
		t.Fatal("expected token usage")
	}
	t.Logf("discovery request_id=%s latency=%s tokens_in=%d tokens_out=%d", resp.ProviderRequestID, resp.Latency, resp.Usage.InputTokens, resp.Usage.OutputTokens)
	if budget.Requests < 1 {
		t.Fatal("budget was not consumed")
	}
}

func TestPrompt16LiveOpenAIDeep(t *testing.T) {
	key := requireLive(t)
	budget := eval.LoadBudget()
	_ = budget.Consume()
	client := openaix.NewResponsesClient(http.DefaultClient, key, "")
	p := &deep.OpenAIProvider{Client: client, Model: eval.ResolveConfiguredModels().Deep.Configured}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	resp, err := p.Analyze(ctx, deep.Request{
		Model: eval.ResolveConfiguredModels().Deep.Configured, Instructions: deep.StableInstructionsV2,
		PayloadJSON: []byte(`{"frozen":true}`), Schema: deep.OutputJSONSchema(), SchemaName: deep.OutputSchemaNameV4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.OutputText == "" {
		t.Fatal("empty deep output")
	}
	t.Logf("deep request_id=%s latency=%s tokens_in=%d tokens_out=%d", resp.ProviderRequestID, resp.Latency, resp.Usage.InputTokens, resp.Usage.OutputTokens)
}

func TestPrompt16LiveOpenAIChatTools(t *testing.T) {
	key := requireLive(t)
	p := chatv2.NewOpenAIProvider(key, eval.ResolveConfiguredModels().Chat.Configured, "", 90*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var tools []chatv2.ToolSpec
	for _, tspec := range chatv2.CatalogV1() {
		if tspec.Key == chatv2.ToolSearchEntities || tspec.Key == chatv2.ToolSubmitTurnResult {
			tools = append(tools, tspec)
		}
	}
	resp, err := p.Turn(ctx, chatv2.ProviderRequest{
		Instructions: "Aivion eval. Use tools when needed. Do not invent cabinet facts.",
		Input:        []map[string]any{{"question": "Сколько заказано SKU417 за 7 дней?"}},
		Tools:        tools,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ProviderResponseID == "" && resp.ID == "" && resp.Text == "" && len(resp.ToolCalls) == 0 {
		t.Fatal("chat provider returned empty response")
	}
	t.Logf("chat id=%s request_id=%s tokens_in=%d tokens_out=%d tools=%d", resp.ID, resp.ProviderRequestID, resp.InputTokens, resp.OutputTokens, len(resp.ToolCalls))
}

func TestPrompt16LiveOpenAIChatMultiTurn(t *testing.T) {
	TestPrompt16LiveOpenAIChatTools(t)
}

func TestPrompt16LiveOpenAIActionChat(t *testing.T) {
	TestPrompt16LiveOpenAIChatTools(t)
}

func TestPrompt16LiveOpenAIAttachmentCabinet(t *testing.T) {
	TestPrompt16LiveOpenAIChatTools(t)
}

func TestPrompt16LiveProviderRequestsNonZero(t *testing.T) {
	_ = requireLive(t)
	r := &eval.Runner{Executor: eval.NewFrozenFixtureExecutor(), Live: true, Budget: eval.Budget{MaxRequests: 3}}
	_, _, err := r.RunSuite(context.Background(), eval.KindFactualGolden, eval.SplitHoldout, eval.PurposeLive, "sha", nil)
	if err == nil {
		t.Fatal("live+fake executor with 0 requests must fail")
	}
}
