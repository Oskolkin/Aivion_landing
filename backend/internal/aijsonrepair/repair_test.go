package aijsonrepair

import "testing"

func TestRepairRawJSON_markdownFence(t *testing.T) {
	raw := "```json\n{\"a\":1}\n```"
	repaired, repairs, err := RepairRawJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if repaired != `{"a":1}` {
		t.Fatalf("got %q", repaired)
	}
	if len(repairs) == 0 {
		t.Fatal("expected repair note")
	}
}

func TestRepairRawJSON_trailingComma(t *testing.T) {
	raw := `{"a":1,}`
	repaired, _, err := RepairRawJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if repaired != `{"a":1}` {
		t.Fatalf("got %q", repaired)
	}
}
