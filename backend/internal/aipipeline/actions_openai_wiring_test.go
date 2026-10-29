package aipipeline_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

func TestSelectActionsOpenAIClient_UsesStageTimeout(t *testing.T) {
	base, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{
		APIKey:         "test-key",
		TimeoutSeconds: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		OpenAI: config.OpenAIConfig{
			TimeoutSeconds:        120,
			ActionsTimeoutSeconds: 300,
		},
	}
	narrativeSvc := dashboardhome.NewNarrativeServiceWithOpenAIClientForTest(base)

	cloned, timeoutSeconds, err := aipipeline.SelectActionsOpenAIClientForTest(narrativeSvc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutSeconds != 300 {
		t.Fatalf("timeoutSeconds=%d", timeoutSeconds)
	}
	if int(cloned.HTTPClientTimeout().Seconds()) != 300 {
		t.Fatalf("http timeout=%s", cloned.HTTPClientTimeout())
	}
	if int(base.HTTPClientTimeout().Seconds()) != 120 {
		t.Fatalf("base timeout changed=%s", base.HTTPClientTimeout())
	}
}

func TestSelectActionsOpenAIClient_Default300NotBase120(t *testing.T) {
	cfg := &config.Config{
		OpenAI: config.OpenAIConfig{
			TimeoutSeconds:        120,
			ActionsTimeoutSeconds: 0,
		},
	}
	base, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{
		APIKey:         "test-key",
		TimeoutSeconds: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	narrativeSvc := dashboardhome.NewNarrativeServiceWithOpenAIClientForTest(base)

	_, timeoutSeconds, err := aipipeline.SelectActionsOpenAIClientForTest(narrativeSvc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutSeconds != openaix.DefaultActionsTimeoutSeconds {
		t.Fatalf("timeoutSeconds=%d want %d", timeoutSeconds, openaix.DefaultActionsTimeoutSeconds)
	}
}

func TestActionsClientTimeoutExceedsBaseClientDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte(`{"model":"gpt-test","output":[{"type":"message","content":[{"type":"output_text","text":"{\"actions\":[]}"}]}],"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}`))
	}))
	defer server.Close()

	base, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{
		APIKey:         "test-key",
		Model:          "gpt-test",
		TimeoutSeconds: 1,
		MaxRetries:     0,
		BaseURL:        server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{OpenAI: config.OpenAIConfig{ActionsTimeoutSeconds: 3}}
	actionsClient, timeoutSeconds, err := aipipeline.SelectActionsOpenAIClientFromBaseForTest(base, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutSeconds != 3 {
		t.Fatalf("timeout=%d", timeoutSeconds)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if _, err := actionsClient.GenerateStructuredJSON(ctx, recommendations.GenerateStructuredJSONInput{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Context:      map[string]any{"ok": true},
	}); err != nil {
		t.Fatalf("actions client should succeed within stage timeout: %v", err)
	}

	ctxShort, cancelShort := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancelShort()
	if _, err := base.GenerateStructuredJSON(ctxShort, recommendations.GenerateStructuredJSONInput{
		SystemPrompt: "sys",
		UserPrompt:   "user",
		Context:      map[string]any{"ok": true},
	}); err == nil {
		t.Fatal("base client expected timeout")
	}
}
