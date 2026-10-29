package provider

import (
	"strings"
	"testing"
)

const sampleResponseJSON = `{
  "id": "resp_sample_001",
  "status": "completed",
  "model": "gpt-test",
  "created_at": 1735689600,
  "completed_at": 1735689700,
  "output": [
    {
      "type": "message",
      "content": [
        {"type": "output_text", "text": "{\"pack_id\":\"pack:1:2026-07-08:abc\",\"schema_version\":\"aivion_analysis_pack_v1\"}"}
      ]
    },
    {
      "type": "code_interpreter_call",
      "id": "ci_call_sample",
      "container_id": "cntr_sample_001",
      "status": "completed",
      "code": "import zipfile"
    }
  ],
  "usage": {
    "input_tokens": 120,
    "output_tokens": 45,
    "total_tokens": 165
  }
}`

func TestParseResponseJSONCodeInterpreterCall(t *testing.T) {
	resp, err := ParseResponseJSON([]byte(sampleResponseJSON))
	if err != nil {
		t.Fatalf("ParseResponseJSON: %v", err)
	}
	if resp.ID != "resp_sample_001" {
		t.Fatalf("id=%q", resp.ID)
	}
	if resp.Status != ProviderResponseStatusCompleted {
		t.Fatalf("status=%q", resp.Status)
	}
	if resp.Usage.InputTokens != 120 || resp.Usage.OutputTokens != 45 || resp.Usage.TotalTokens != 165 {
		t.Fatalf("usage=%+v", resp.Usage)
	}
	if !strings.Contains(resp.OutputText, "aivion_analysis_pack_v1") {
		t.Fatalf("output=%q", resp.OutputText)
	}
	if !HasCodeInterpreterCall(resp) {
		t.Fatal("expected code interpreter call")
	}
	if !CallsMatchContainer(resp, "cntr_sample_001") {
		t.Fatalf("calls=%+v", resp.CodeInterpreterCalls)
	}
	if resp.CodeInterpreterCalls[0].Code != "import zipfile" {
		t.Fatalf("code=%q", resp.CodeInterpreterCalls[0].Code)
	}
}

func TestParseResponseJSONFailedWithError(t *testing.T) {
	raw := `{
	  "id": "resp_failed",
	  "status": "failed",
	  "error": {"code": "server_error", "message": "boom"}
	}`
	resp, err := ParseResponseJSON([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != "server_error" {
		t.Fatalf("error=%+v", resp.Error)
	}
}

func TestParseResponseJSONIncomplete(t *testing.T) {
	raw := `{
	  "id": "resp_incomplete",
	  "status": "incomplete",
	  "incomplete_details": {"reason": "max_output_tokens"}
	}`
	resp, err := ParseResponseJSON([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if resp.IncompleteDetails == nil || resp.IncompleteDetails.Reason != "max_output_tokens" {
		t.Fatalf("incomplete=%+v", resp.IncompleteDetails)
	}
}

func TestUsageHelpers(t *testing.T) {
	a := &ProviderResponse{Usage: ProviderUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}}
	b := &ProviderResponse{Usage: ProviderUsage{InputTokens: 20, OutputTokens: 10, TotalTokens: 30}}
	sum := SumUsage(a, b)
	if sum.TotalTokens != 45 {
		t.Fatalf("sum=%+v", sum)
	}
	if CountCodeInterpreterCalls(a) != 0 {
		t.Fatal("expected zero calls")
	}
	if !IsTerminalStatus(ProviderResponseStatusCompleted) || IsActiveStatus(ProviderResponseStatusCompleted) {
		t.Fatal("terminal/active mismatch")
	}
}
