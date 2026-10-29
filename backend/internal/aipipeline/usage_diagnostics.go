package aipipeline

// finalizePipelineUsageDiagnostics aggregates component token totals for the current run.
func finalizePipelineUsageDiagnostics(diag *PipelineDiagnostics) {
	if diag == nil {
		return
	}
	diag.RetryCount = 0
	diag.AttemptCount = pipelineUsageAttempt

	chunkIn := diag.SharedChunkAnalysis.InputTokens
	chunkOut := diag.SharedChunkAnalysis.OutputTokens
	if diag.SharedChunkAnalysis.Source == "artifact" || diag.SharedChunkAnalysis.Source == stageRestartArtifactSourceMeta {
		if diag.SharedChunkAnalysis.Source == stageRestartArtifactSourceMeta {
			diag.SharedChunkAnalysis.UsageSource = UsageSourceStageRestartSource
		} else {
			diag.SharedChunkAnalysis.UsageSource = UsageSourceArtifact
		}
		chunkIn = 0
		chunkOut = 0
	} else if chunkIn > 0 || chunkOut > 0 {
		diag.SharedChunkAnalysis.UsageSource = UsageSourceFake
		if diag.SharedChunkAnalysis.Enabled {
			for _, call := range diag.SharedChunkAnalysis.ChunkCalls {
				if call.UsageSource == UsageSourceProvider || call.UsageSource == "provider" {
					diag.SharedChunkAnalysis.UsageSource = UsageSourceProvider
					break
				}
			}
		}
	}

	gcIn := diag.GlobalContext.InputTokens
	gcOut := diag.GlobalContext.OutputTokens
	if diag.GlobalContext.Source == "artifact" {
		diag.GlobalContext.UsageSource = UsageSourceArtifact
		gcIn = 0
		gcOut = 0
	} else if gcIn > 0 || gcOut > 0 {
		diag.GlobalContext.UsageSource = usageSourceForProvider(diag.GlobalContext.Provider, true)
	}

	dashIn := diag.Dashboard.SynthesisInputTokens
	dashOut := diag.Dashboard.SynthesisOutputTokens
	if diag.Dashboard.Reused || diag.Dashboard.ArtifactSource == "persisted" || diag.Dashboard.SynthesisAICalls == 0 {
		if diag.Dashboard.SynthesisAICalls == 0 && (diag.Dashboard.Reused || diag.Dashboard.ArtifactSource == "persisted") {
			diag.Dashboard.UsageSource = UsageSourceArtifact
			dashIn = 0
			dashOut = 0
		}
	} else if dashIn > 0 || dashOut > 0 {
		diag.Dashboard.UsageSource = usageSourceForProvider(diag.Dashboard.Provider, true)
	}

	actIn := diag.Actions.InputTokens
	actOut := diag.Actions.OutputTokens
	if diag.Actions.ComposeCalls == 0 {
		diag.Actions.UsageSource = UsageSourceSkipped
		actIn = 0
		actOut = 0
	} else if actIn > 0 || actOut > 0 {
		diag.Actions.UsageSource = usageSourceForProvider(diag.Actions.Provider, true)
	}

	diag.TotalInputTokens = chunkIn + gcIn + dashIn + actIn
	diag.TotalOutputTokens = chunkOut + gcOut + dashOut + actOut
}
