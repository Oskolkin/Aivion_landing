package aipipeline

const (
	AIUsageComponentChunkAnalysis      = "chunk_analysis"
	AIUsageComponentGlobalContext      = "global_context"
	AIUsageComponentDashboardSynthesis = "dashboard_synthesis"
	AIUsageComponentActionsSynthesis   = "actions_synthesis"
)

const (
	UsageSourceProvider           = "provider"
	UsageSourceProviderMissing    = "provider_missing"
	UsageSourceFake               = "fake"
	UsageSourceArtifact           = "artifact"
	UsageSourceSkipped            = "skipped"
	UsageSourceStageRestartSource = "stage_restart_source"
)

const pipelineUsageAttempt = 1
