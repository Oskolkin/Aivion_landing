package aicost

import (
	"os"
	"strings"
	"testing"
)

// Ensures usage counts are scoped per seller_account_id (not global).
func TestAIUsageLimitSQLScopedBySellerAccountID(t *testing.T) {
	raw, err := os.ReadFile("../../sql/queries/ai_usage_limits.sql")
	if err != nil {
		t.Fatalf("read sql: %v", err)
	}
	body := string(raw)
	for _, fragment := range []string{
		"CountChatUserQuestionsSince",
		"seller_account_id = $1",
		"CountRecommendationRunsSince",
		"CountAIDailySummaryRunsSince",
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected %q in ai_usage_limits.sql", fragment)
		}
	}
}
