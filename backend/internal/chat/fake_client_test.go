package chat

import (
	"context"
	"strings"
	"testing"
)

func TestDeterministicFakeAIClientEchoesControlCode(t *testing.T) {
	client := NewDeterministicFakeAIClient()
	out, err := client.GenerateAnswer(context.Background(), GenerateAnswerInput{
		UserPrompt: "Какой контрольный код указан в приложенном файле?",
		FactContext: &FactContext{
			Facts: FactContextFacts{
				Other: []map[string]any{{
					"source":    "attachment",
					"file_name": "control.txt",
					"excerpt":   "TXT_CONTROL_16735 — контрольное значение",
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Content, "TXT_CONTROL_16735") {
		t.Fatalf("expected control code in answer, got %q", out.Content)
	}
	if out.Model != fakeChatModel {
		t.Fatalf("model=%q", out.Model)
	}
}

func TestDeterministicFakeAIClientStreamsMarker(t *testing.T) {
	client := NewDeterministicFakeAIClient()
	var deltas strings.Builder
	out, err := client.StreamAnswer(context.Background(), GenerateAnswerInput{
		UserPrompt: "Отметь ответ маркером E2E-STREAM-12345.",
	}, func(delta string) error {
		deltas.WriteString(delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Content, "E2E-STREAM-12345") {
		t.Fatalf("expected marker in answer, got %q", out.Content)
	}
	if deltas.String() != out.Content {
		t.Fatalf("stream deltas mismatch: %q vs %q", deltas.String(), out.Content)
	}
}
