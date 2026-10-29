package outputextract_test

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputextract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

const completedStructuredJSON = `{"schema_version":"aivion_unified_analysis_v1","analysis_meta":{"pack_id":"pack_example_v1","pack_hash":"sha256:example","metric_date":"2026-07-08"},"analysis_outcome":"no_material_actions","dashboard":{"account_summary":{"status":"available","headline":"ok","executive_summary":"ok","metric_date":"2026-07-08"},"what_changed":{"status":"available","groups":[]},"category_summaries":[],"sales_funnel":{"status":"available","stages":[]},"funnel_analysis":{"status":"available","ai_analysis":{"status":"available","action_items":[],"related_signals":[],"recommended_focus":[]}},"stock_coverage":{"status":"available","segments":[],"total_skus":0},"ad_efficiency":{"status":"available"},"risk_distribution":{"status":"available","groups":[],"total":0},"seller_pulse":{"kpis":[],"trend_7d":[]},"sku_focus":{"order_drop":[],"stock_risk":[],"no_sales_with_stock":[],"slow_turnover":[],"key_skus":[]},"top_action_ids":[],"recommended_action_ids":[]},"actions":[],"limitations":[]}`

func TestExtractCompletedStructuredOutput(t *testing.T) {
	resp := &provider.ProviderResponse{
		ID:                    "resp_fixture_completed",
		Status:                provider.ProviderResponseStatusCompleted,
		OutputText:            completedStructuredJSON,
		AssistantMessageCount: 1,
		CodeInterpreterCalls:  []provider.CodeInterpreterCall{{ID: "ci_1", Status: "completed"}},
	}
	got, err := outputextract.ExtractStructuredOutput(resp)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(got.JSONBytes) == 0 {
		t.Fatal("empty json")
	}
	out, err := outputextract.ParseOutput(got.JSONBytes)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if out.AnalysisOutcome == "" {
		t.Fatal("missing outcome")
	}
}

func TestExtractToolOnlyResponse(t *testing.T) {
	// Structural shape of runs 116/118: completed provider status, CI calls only, no final message text.
	resp := &provider.ProviderResponse{
		ID:                    "resp_fixture_tool_only",
		Status:                provider.ProviderResponseStatusCompleted,
		OutputText:            "",
		AssistantMessageCount: 0,
		CodeInterpreterCalls: []provider.CodeInterpreterCall{
			{ID: "ci_1", Status: "completed"},
			{ID: "ci_2", Status: "completed"},
		},
	}
	_, err := outputextract.ExtractStructuredOutput(resp)
	code, ok := outputextract.CodeOf(err)
	if !ok || code != outputextract.CodeStructuredOutputMissing {
		t.Fatalf("want %s, got code=%q err=%v", outputextract.CodeStructuredOutputMissing, code, err)
	}
}

func TestExtractIncompleteResponse(t *testing.T) {
	// Structural shape of run 127 class: incomplete status with reason.
	resp := &provider.ProviderResponse{
		ID:     "resp_fixture_incomplete",
		Status: provider.ProviderResponseStatusIncomplete,
		IncompleteDetails: &provider.IncompleteDetails{
			Reason: "max_output_tokens",
		},
		CodeInterpreterCalls: []provider.CodeInterpreterCall{{ID: "ci_1", Status: "completed"}},
	}
	_, err := outputextract.ExtractStructuredOutput(resp)
	code, ok := outputextract.CodeOf(err)
	if !ok || code != outputextract.CodeProviderResponseIncomplete {
		t.Fatalf("want %s, got code=%q err=%v", outputextract.CodeProviderResponseIncomplete, code, err)
	}
}

func TestExtractRefusal(t *testing.T) {
	resp := &provider.ProviderResponse{
		Status:  provider.ProviderResponseStatusCompleted,
		Refusal: "policy refusal",
	}
	_, err := outputextract.ExtractStructuredOutput(resp)
	code, ok := outputextract.CodeOf(err)
	if !ok || code != outputextract.CodeProviderRefusal {
		t.Fatalf("want %s, got code=%q err=%v", outputextract.CodeProviderRefusal, code, err)
	}
}

func TestExtractAmbiguousMultipleMessages(t *testing.T) {
	resp := &provider.ProviderResponse{
		Status:                provider.ProviderResponseStatusCompleted,
		OutputText:            `{"schema_version":"aivion_unified_analysis_v1"}`,
		AssistantMessageCount: 2,
	}
	_, err := outputextract.ExtractStructuredOutput(resp)
	code, ok := outputextract.CodeOf(err)
	if !ok || code != outputextract.CodeStructuredOutputAmbiguous {
		t.Fatalf("want %s, got code=%q err=%v", outputextract.CodeStructuredOutputAmbiguous, code, err)
	}
}
