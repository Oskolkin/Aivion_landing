package dashboardhome

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestActionPriorityToTopActionPriority(t *testing.T) {
	cases := map[string]string{
		"critical":  "critical",
		"urgent":    "critical",
		"important": "high",
		"high":      "high",
		"watch":     "low",
		"low":       "low",
		"":          "medium",
	}
	for in, want := range cases {
		if got := actionPriorityToTopActionPriority(in); got != want {
			t.Fatalf("priority %q => %q want %q", in, got, want)
		}
	}
}

func TestIsOpenActionUserStatus(t *testing.T) {
	if !isOpenActionUserStatus("open") || !isOpenActionUserStatus("") {
		t.Fatal("open statuses expected")
	}
	if isOpenActionUserStatus("checked") || isOpenActionUserStatus("dismissed") {
		t.Fatal("non-open statuses must be excluded")
	}
}

func TestIsValidTopActionFromActionsRequiresActionID(t *testing.T) {
	item := TopActionItem{
		ID:            "key-1",
		ActionKey:     "key-1",
		Source:        topActionsSourceFinal,
		Priority:      "high",
		PriorityLabel: "Высокий",
		Category:      "stock",
		CategoryLabel: "Остатки",
		Title:         "T",
		Reason:        "R",
		NextStep:      "N",
	}
	if isValidTopActionFromActions(item) {
		t.Fatal("expected invalid without action_id")
	}
	item.ActionID = 42
	if !isValidTopActionFromActions(item) {
		t.Fatal("expected valid with action_id")
	}
}

func TestDashboardTopActionsNoReviewerFields(t *testing.T) {
	item := TopActionItem{
		ID:            "k",
		ActionID:      1,
		ActionKey:     "k",
		Source:        topActionsSourceFinal,
		Priority:      "critical",
		PriorityLabel: "Критический",
		Category:      "stock",
		CategoryLabel: "Остатки",
		Title:         "T",
		Reason:        "R",
		NextStep:      "N",
	}
	rawBytes, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(rawBytes)
	for _, forbidden := range []string{"reviewer", "consistency_review", "patch", "temp_action_id"} {
		if strings.Contains(strings.ToLower(raw), forbidden) {
			t.Fatalf("forbidden field %q in %s", forbidden, raw)
		}
	}
}

func TestSelectDashboardTopActionRowsRespectsTopKeysOrder(t *testing.T) {
	rows := []dbgen.ListActionItemsWithUserStateByRunIDRow{
		{ID: 10, ActionKey: "a", Priority: "low", UserStatus: "open"},
		{ID: 20, ActionKey: "b", Priority: "critical", UserStatus: "open"},
		{ID: 30, ActionKey: "c", Priority: "high", UserStatus: "open"},
	}
	selected := SelectDashboardTopActionRows(rows, []string{"a", "c"}, 2)
	if len(selected) != 2 || selected[0].ID != 10 || selected[1].ID != 30 {
		t.Fatalf("selected=%#v", selected)
	}
	ids := DashboardTopActionIDsFromRows(rows, []byte(`["b","a"]`), 2)
	if len(ids) != 2 || ids[0] != 20 || ids[1] != 10 {
		t.Fatalf("ids=%v", ids)
	}
}

func TestDashboardTopActionsExcludeCompletedAndIrrelevant(t *testing.T) {
	if isOpenActionUserStatus("checked") || isOpenActionUserStatus("dismissed") || isOpenActionUserStatus("snoozed") {
		t.Fatal("completed/dismissed/snoozed must not be open")
	}
}
