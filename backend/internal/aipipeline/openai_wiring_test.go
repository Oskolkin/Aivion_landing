package aipipeline_test

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

func TestSelectGlobalContextOpenAIClient_UsesStageTimeout(t *testing.T) {
	base, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{
		APIKey:         "test-key",
		TimeoutSeconds: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		OpenAI: config.OpenAIConfig{
			TimeoutSeconds:              120,
			GlobalContextTimeoutSeconds: 300,
		},
	}
	narrativeSvc := dashboardhome.NewNarrativeServiceWithOpenAIClientForTest(base)

	cloned, timeoutSeconds, err := aipipeline.SelectGlobalContextOpenAIClientForTest(narrativeSvc, cfg)
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

func TestSelectGlobalContextOpenAIClient_FallbackToBaseTimeout(t *testing.T) {
	cfg := &config.Config{
		OpenAI: config.OpenAIConfig{
			TimeoutSeconds:              120,
			GlobalContextTimeoutSeconds: 0,
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

	_, timeoutSeconds, err := aipipeline.SelectGlobalContextOpenAIClientForTest(narrativeSvc, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if timeoutSeconds != 120 {
		t.Fatalf("timeoutSeconds=%d want 120", timeoutSeconds)
	}
}

func TestOpenAIGlobalContextTimeoutDefaultConstant(t *testing.T) {
	if openaix.DefaultGlobalContextTimeoutSeconds != 300 {
		t.Fatalf("default=%d", openaix.DefaultGlobalContextTimeoutSeconds)
	}
	if openaix.DefaultGlobalContextTimeout != 300*time.Second {
		t.Fatalf("default duration=%s", openaix.DefaultGlobalContextTimeout)
	}
}
