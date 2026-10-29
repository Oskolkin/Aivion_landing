package aipipeline

import "testing"

func TestFinalizeStageRestartUsageDiagnostics_OnlyActionsInTotals(t *testing.T) {
	diag := PipelineDiagnostics{
		SharedChunkAnalysis: SharedChunkAnalysisDiagnostics{
			Source:       stageRestartArtifactSourceMeta,
			UsageSource:  UsageSourceStageRestartSource,
			InputTokens:  259880,
			OutputTokens: 31648,
		},
		GlobalContext: GlobalContextDiagnostics{
			Source:       "artifact",
			UsageSource:  UsageSourceArtifact,
			InputTokens:  1000,
			OutputTokens: 500,
		},
		Dashboard: StepDiagnostics{
			Reused:                true,
			ArtifactSource:        "cloned_from_source_pipeline",
			SynthesisInputTokens:  2000,
			SynthesisOutputTokens: 800,
			SynthesisAICalls:      0,
		},
		Actions: ActionsDiagnostics{
			ComposeCalls: 1,
			InputTokens:  29543,
			OutputTokens: 10389,
			UsageSource:  UsageSourceProvider,
		},
	}

	finalizeStageRestartUsageDiagnostics(&diag)

	if diag.TotalInputTokens != 29543 {
		t.Fatalf("total input=%d want 29543", diag.TotalInputTokens)
	}
	if diag.TotalOutputTokens != 10389 {
		t.Fatalf("total output=%d want 10389", diag.TotalOutputTokens)
	}
	if diag.ReusedSharedChunkInputTokens != 259880 {
		t.Fatalf("reused chunk in=%d", diag.ReusedSharedChunkInputTokens)
	}
	if diag.ReusedSharedChunkOutputTokens != 31648 {
		t.Fatalf("reused chunk out=%d", diag.ReusedSharedChunkOutputTokens)
	}
	if diag.SharedChunkAnalysis.UsageSource != UsageSourceStageRestartSource {
		t.Fatalf("shared chunk usage source=%q", diag.SharedChunkAnalysis.UsageSource)
	}
	if diag.SharedChunkAnalysis.InputTokens != 0 || diag.SharedChunkAnalysis.OutputTokens != 0 {
		t.Fatalf("shared chunk executed tokens should be zero in new run diag")
	}
	if diag.Actions.UsageSource != UsageSourceProvider {
		t.Fatalf("actions usage source=%q", diag.Actions.UsageSource)
	}
}

func TestFinalizePipelineUsageDiagnostics_FullRunUnchanged(t *testing.T) {
	diag := PipelineDiagnostics{
		SharedChunkAnalysis: SharedChunkAnalysisDiagnostics{
			Source:       "generated",
			InputTokens:  100,
			OutputTokens: 50,
		},
		GlobalContext: GlobalContextDiagnostics{
			InputTokens:  200,
			OutputTokens: 80,
		},
		Dashboard: StepDiagnostics{
			SynthesisAICalls:      1,
			SynthesisInputTokens:  300,
			SynthesisOutputTokens: 120,
		},
		Actions: ActionsDiagnostics{
			ComposeCalls: 1,
			InputTokens:  400,
			OutputTokens: 160,
		},
	}
	finalizePipelineUsageDiagnostics(&diag)
	if diag.TotalInputTokens != 1000 {
		t.Fatalf("total input=%d want 1000", diag.TotalInputTokens)
	}
	if diag.TotalOutputTokens != 410 {
		t.Fatalf("total output=%d want 410", diag.TotalOutputTokens)
	}
}
