package dashboardhome

// ProductionChunkAnalysisSchemaVersion returns the active chunk analysis schema version.
func ProductionChunkAnalysisSchemaVersion() string {
	return DashboardChunkAnalysisSchemaVersion
}

// ProductionChunkSystemPrompt returns the active chunk analysis system prompt.
func ProductionChunkSystemPrompt() string {
	return systemPromptDashboardChunkAnalysis()
}
