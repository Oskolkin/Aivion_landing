package aipipeline

// finalizeStageRestartUsageDiagnostics aggregates token totals for derived stage-restart runs.
// Executed totals include only provider calls made in the new run; reused source usage is
// stored separately for observability.
func finalizeStageRestartUsageDiagnostics(diag *PipelineDiagnostics) {
	if diag == nil {
		return
	}
	diag.RetryCount = 0
	diag.AttemptCount = pipelineUsageAttempt

	diag.ReusedSharedChunkInputTokens = diag.SharedChunkAnalysis.InputTokens
	diag.ReusedSharedChunkOutputTokens = diag.SharedChunkAnalysis.OutputTokens
	diag.ReusedGlobalContextInputTokens = diag.GlobalContext.InputTokens
	diag.ReusedGlobalContextOutputTokens = diag.GlobalContext.OutputTokens
	dashIn := diag.Dashboard.SynthesisInputTokens
	dashOut := diag.Dashboard.SynthesisOutputTokens
	diag.ReusedDashboardInputTokens = dashIn
	diag.ReusedDashboardOutputTokens = dashOut

	if diag.SharedChunkAnalysis.Source == stageRestartArtifactSourceMeta ||
		diag.SharedChunkAnalysis.UsageSource == UsageSourceStageRestartSource {
		diag.SharedChunkAnalysis.UsageSource = UsageSourceStageRestartSource
	} else if diag.SharedChunkAnalysis.Source == "artifact" {
		diag.SharedChunkAnalysis.UsageSource = UsageSourceArtifact
	}
	diag.SharedChunkAnalysis.InputTokens = 0
	diag.SharedChunkAnalysis.OutputTokens = 0

	if diag.GlobalContext.Source == "artifact" || diag.GlobalContext.ArtifactSource == "artifact" ||
		diag.GlobalContext.UsageSource == UsageSourceArtifact {
		diag.GlobalContext.UsageSource = UsageSourceArtifact
	}
	diag.GlobalContext.InputTokens = 0
	diag.GlobalContext.OutputTokens = 0

	if diag.Dashboard.Reused || diag.Dashboard.ArtifactSource == "persisted" ||
		diag.Dashboard.ArtifactSource == "cloned_from_source_pipeline" || diag.Dashboard.SynthesisAICalls == 0 {
		diag.Dashboard.UsageSource = UsageSourceArtifact
	}
	diag.Dashboard.SynthesisInputTokens = 0
	diag.Dashboard.SynthesisOutputTokens = 0

	actIn := diag.Actions.InputTokens
	actOut := diag.Actions.OutputTokens
	if diag.Actions.ComposeCalls == 0 {
		diag.Actions.UsageSource = UsageSourceSkipped
		actIn = 0
		actOut = 0
	} else if actIn > 0 || actOut > 0 {
		diag.Actions.UsageSource = UsageSourceProvider
	}

	diag.TotalInputTokens = actIn
	diag.TotalOutputTokens = actOut
}
