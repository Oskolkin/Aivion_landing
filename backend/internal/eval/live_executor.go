package eval

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatv2"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

type LiveOpenAIExecutor struct {
	Inner      CaseExecutor
	Discovery  discovery.Provider
	Deep       deep.Provider
	Chat       chatv2.ChatAgentProvider
	Budget     *Budget
	smoked     sync.Once
	smokeErr   error
	smokeCount int
}

func NewFrozenLiveExecutor(apiKey string, budget *Budget) *LiveOpenAIExecutor {
	httpClient := &http.Client{Timeout: 90 * time.Second}
	client := openaix.NewResponsesClient(httpClient, apiKey, "")
	models := ResolveConfiguredModels()
	inner := NewFrozenFixtureExecutor()
	return &LiveOpenAIExecutor{
		Inner:     inner,
		Budget:    budget,
		Discovery: &discovery.OpenAIProvider{Client: client, Model: models.Discovery.Configured},
		Deep:      &deep.OpenAIProvider{Client: client, Model: models.Deep.Configured},
		Chat:      chatv2.NewOpenAIProvider(apiKey, models.Chat.Configured, "", 90*time.Second),
	}
}

func (e *LiveOpenAIExecutor) Execute(ctx context.Context, in CaseInput, opts ExecuteOptions) (CaseActual, error) {
	if e.Inner == nil {
		e.Inner = NewFrozenFixtureExecutor()
	}
	act, err := e.Inner.Execute(ctx, in, opts)
	if err != nil {
		return act, err
	}
	if !opts.Live {
		return act, nil
	}
	e.smoked.Do(func() {
		if err := e.hitDiscovery(ctx, opts.Budget); err != nil {
			e.smokeErr = err
			return
		}
		e.smokeCount++
	})
	if e.smokeErr != nil {
		return act, e.smokeErr
	}
	tags := strings.Join(in.Tags, ",")
	switch {
	case strings.Contains(tags, "deep") || strings.Contains(tags, "trap"):
		if strings.Contains(in.CaseKey, "stable_account") || strings.Contains(in.CaseKey, "discovery") {
			if err := e.hitDiscovery(ctx, opts.Budget); err != nil {
				return act, err
			}
		} else {
			if err := e.hitDeep(ctx, opts.Budget); err != nil {
				return act, err
			}
		}
		act.ProviderRequests++
	case strings.HasPrefix(in.CaseKey, "chat_"):
		if err := e.hitChat(ctx, in, opts.Budget); err != nil {
			return act, err
		}
		act.ProviderRequests++
	default:
		act.ProviderRequests += e.smokeCount
		e.smokeCount = 0
	}
	return act, nil
}

func (e *LiveOpenAIExecutor) hitDiscovery(ctx context.Context, b *Budget) error {
	if err := b.Consume(); err != nil {
		return err
	}
	req := discovery.Request{
		Model: ResolveConfiguredModels().Discovery.Configured, Instructions: discovery.StableInstructionsV1,
		PayloadJSON: []byte(`{"account":{"ordered_amount":"300"},"note":"frozen eval pack"}`),
		Schema:      discovery.OutputJSONSchema(), SchemaName: discovery.OutputSchemaNameV1,
	}
	resp, err := e.Discovery.Discover(ctx, req)
	if err != nil {
		return err
	}
	if strings.TrimSpace(resp.OutputText) == "" && resp.ProviderRequestID == "" {
		return fmt.Errorf("live discovery produced empty provider response")
	}
	return nil
}

func (e *LiveOpenAIExecutor) hitDeep(ctx context.Context, b *Budget) error {
	if err := b.Consume(); err != nil {
		return err
	}
	req := deep.Request{
		Model: ResolveConfiguredModels().Deep.Configured, Instructions: deep.StableInstructionsV2,
		PayloadJSON: []byte(`{"frozen":true}`), Schema: deep.OutputJSONSchema(), SchemaName: deep.OutputSchemaNameV4,
	}
	resp, err := e.Deep.Analyze(ctx, req)
	if err != nil {
		return err
	}
	if strings.TrimSpace(resp.OutputText) == "" && resp.ProviderRequestID == "" {
		return fmt.Errorf("live deep produced empty provider response")
	}
	return nil
}

func (e *LiveOpenAIExecutor) hitChat(ctx context.Context, in CaseInput, b *Budget) error {
	if err := b.Consume(); err != nil {
		return err
	}
	tools := []chatv2.ToolSpec{}
	for _, t := range chatv2.CatalogV1() {
		if t.Key == chatv2.ToolSearchEntities || t.Key == chatv2.ToolSubmitTurnResult {
			tools = append(tools, t)
		}
	}
	q := in.Question
	if q == "" {
		q = "Сколько заказано SKU417 за 7 дней?"
	}
	_, err := e.Chat.Turn(ctx, chatv2.ProviderRequest{
		Instructions: "You are Aivion eval. Use tools. Do not invent seller facts.",
		Input:        []map[string]any{{"question": q}},
		Tools:        tools,
	})
	return err
}

func MustLiveAPIKey() (string, error) {
	if !LiveEnabled() {
		return "", fmt.Errorf("openai provider requires OPENAI_LIVE_EVAL=1 and OPENAI_API_KEY")
	}
	k := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if k == "" {
		return "", fmt.Errorf("OPENAI_API_KEY empty")
	}
	return k, nil
}
