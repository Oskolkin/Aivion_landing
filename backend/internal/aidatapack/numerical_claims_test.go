package aidatapack

import "testing"

func TestValidateOrderDeclineGrounding_RequiresOrderFacts(t *testing.T) {
	text := "Количество заказов снизилось на 6,01%"
	issues := ValidateOrderDeclineGrounding(text, []string{
		"account.last_7d.revenue",
		"account.revenue.last_7d_vs_previous_7d_delta_pct",
	}, "candidate.problem")
	if len(issues) != 1 || issues[0].Code != OrderDeclineGroundingMismatchCode {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestValidateOrderDeclineGrounding_AcceptsOrderDeltaFact(t *testing.T) {
	text := "Количество заказов снизилось на 6,01%"
	issues := ValidateOrderDeclineGrounding(text, []string{
		"account.last_7d.orders",
		"account.previous_7d.orders",
		"account.orders.last_7d_vs_previous_7d_delta_pct",
	}, "candidate.problem")
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestValidateOrderDeclineGrounding_AcceptsPeriodFacts(t *testing.T) {
	text := "Заказы снизились на 6%"
	issues := ValidateOrderDeclineGrounding(text, []string{
		"account.last_7d.orders",
		"account.previous_7d.orders",
	}, "candidate.problem")
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}
