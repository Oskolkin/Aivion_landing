package actions

import (
	"strings"
	"testing"
)

func TestAIFirstPrompt_ContainsKeyStage2Rules(t *testing.T) {
	system := strings.ToLower(aiFirstSystemPrompt())
	user := strings.ToLower(aiFirstUserPrompt())
	retry := strings.ToLower(aiFirstRetrySystemPrompt())

	requiredSystem := []string{
		"используй только факты из data pack",
		"не создавай действие без конкретных объектов",
		"не генерируй data_quality actions",
		"не используй legacy action types key_sku_risk и ad_high_drr",
		"реклама — только на уровне кампаний",
		"не генерируй cta",
		"completion_criteria",
	}
	for _, marker := range requiredSystem {
		if !strings.Contains(system, strings.ToLower(marker)) {
			t.Fatalf("system prompt missing rule: %q", marker)
		}
	}

	if !strings.Contains(user, `"schema_version": "actions_ai_first_v1"`) {
		t.Fatalf("user prompt must require strict schema_version")
	}
	if !strings.Contains(user, `"focus"`) || !strings.Contains(user, `"actions"`) {
		t.Fatalf("user prompt must require focus/actions output")
	}

	if !strings.Contains(retry, "validation_issues") || !strings.Contains(retry, "не добавляй новые sku") {
		t.Fatalf("retry prompt must require fixes using validation_issues only")
	}
}
