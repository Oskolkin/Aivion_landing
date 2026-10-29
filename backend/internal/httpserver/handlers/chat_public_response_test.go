package handlers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestChatHandlerPublicResponsesOmitRawAIFields(t *testing.T) {
	body, err := json.Marshal(askChatResponse{
		SessionID:          1,
		UserMessageID:      2,
		AssistantMessageID: ptrInt64(3),
		Answer:             "ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := string(body)
	for _, forbidden := range []string{
		"raw_ai_response",
		"raw_response",
		"system_prompt",
		"user_prompt",
		"prompt_version",
		"planner_prompt",
		"full_context",
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("seller chat JSON must not expose %q: %s", forbidden, raw)
		}
	}
}

func ptrInt64(v int64) *int64 { return &v }
