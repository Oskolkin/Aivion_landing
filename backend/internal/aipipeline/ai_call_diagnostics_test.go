package aipipeline

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
)

func TestBuildGlobalContextDiagnosticEnvelope_InvalidJSON(t *testing.T) {
	raw := &GlobalContextRawResponse{
		Content:     `{"overall_story":"x"`,
		ParseStatus: ParseStatusInvalidJSON,
		ParseError:  "invalid_json: eof",
	}
	env := BuildAICallDiagnosticEnvelopeFromGlobalContextRaw("fake", raw, 600)
	if !env.RawResponseCaptured || env.ParseStatus != ParseStatusInvalidJSON {
		t.Fatalf("env=%+v", env)
	}
}

func TestReadinessGlobalContextInvalidJSONWiring(t *testing.T) {
	if !ReadinessVerifyGlobalContextInvalidJSONWiring() {
		t.Fatal("expected wiring pass")
	}
}

func TestBuildAICallRawResponseArtifact_InvalidJSONBody(t *testing.T) {
	body := `{"actions":[{"title":"Test"`
	env := AICallDiagnosticEnvelope{
		SchemaVersion:            AICallDiagnosticsSchemaVersion,
		Stage:                    "actions_compose",
		Provider:                 "fake",
		ParseStatus:              ParseStatusInvalidJSON,
		RawResponseCaptured:      true,
		RequestedMaxOutputTokens: 16384,
		OutputTokens:             len(body),
	}
	artifact := BuildAICallRawResponseArtifact(env, body)
	if artifact["body"] != body {
		t.Fatal("raw body must be saved before parsing")
	}
	if artifact["content_type"] != ArtifactContentFullBody {
		t.Fatalf("content_type=%v", artifact["content_type"])
	}
}

func TestClassifyOutputTruncation_Confirmed(t *testing.T) {
	env := AICallDiagnosticEnvelope{
		ProviderStatus:           "incomplete",
		IncompleteReason:         "max_output_tokens",
		RequestedMaxOutputTokens: 16384,
		OutputTokens:             16384,
		RawResponseCaptured:      true,
	}
	if got := ClassifyOutputTruncation(env); got != TruncationConfirmed {
		t.Fatalf("got=%s want=%s", got, TruncationConfirmed)
	}
}

func TestClassifyOutputTruncation_SuspectedLegacy(t *testing.T) {
	env := AICallDiagnosticEnvelope{
		RequestedMaxOutputTokens: 8192,
		OutputTokens:             8192,
		RawResponseCaptured:      false,
	}
	if got := ClassifyOutputTruncation(env); got != TruncationNotConfirm {
		t.Fatalf("got=%s want=%s", got, TruncationNotConfirm)
	}
	env2 := AICallDiagnosticEnvelope{
		RequestedMaxOutputTokens: 8192,
		OutputTokens:             8192,
		RawResponseCaptured:      true,
	}
	if got := ClassifyOutputTruncation(env2); got != TruncationSuspected {
		t.Fatalf("got=%s want=%s", got, TruncationSuspected)
	}
}

func TestBuildActionsLifecycleArtifactFromResult_DroppedAction(t *testing.T) {
	res := &actions.GenerateActionsResult{
		LifecycleSummary: actions.ActionsLifecycleSummary{
			ProviderReturnedCount: 1,
			ParsedCount:           1,
			PersistedCount:        0,
			DroppedCount:          1,
		},
		ActionLifecycles: []actions.ActionLifecycleDiagnostic{{
			RawActionIndex: 0, ParseOutcome: "parsed",
			NormalizationOutcome: "dropped", PersistOutcome: "dropped",
			DropStage: "normalization", DropCode: "duplicate_scope_after_ai_order_normalization",
		}},
	}
	artifact := BuildActionsLifecycleArtifactFromResult(res)
	if artifact == nil {
		t.Fatal("expected lifecycle artifact")
	}
	summaryMap, _ := artifact["summary"].(map[string]any)
	if summaryMap == nil {
		if s, ok := artifact["summary"].(actions.ActionsLifecycleSummary); ok {
			if s.PersistedCount != 0 || s.DroppedCount != 1 {
				t.Fatalf("summary=%+v", s)
			}
			return
		}
	}
	if intValue(summaryMap["persisted_count"]) != 0 || intValue(summaryMap["dropped_count"]) != 1 {
		t.Fatalf("summary=%v", summaryMap)
	}
}

func TestReadinessVerifyDiagnosticArtifactBuilders(t *testing.T) {
	if !ReadinessVerifyAICallDiagnosticBuilders() {
		t.Fatal("expected diagnostic builders readiness pass")
	}
}
