package aipipeline

// PipelineStageRestartPlan describes derived-run stage restart execution.
type PipelineStageRestartPlan struct {
	SourcePipelineRunID  int64  `json:"source_pipeline_run_id"`
	CreateNewPipelineRun bool   `json:"create_new_pipeline_run"`
	RestartFromStage     string `json:"restart_from_stage"`

	ReuseDataPack            bool `json:"reuse_data_pack"`
	ReuseChunkSet            bool `json:"reuse_chunk_set"`
	ReuseSharedChunkAnalysis bool `json:"reuse_shared_chunk_analysis"`
	ReuseGlobalContext       bool `json:"reuse_global_context"`
	CloneDashboard           bool `json:"clone_dashboard"`
	ReuseActions             bool `json:"reuse_actions"`

	NeedsActions     bool `json:"needs_actions"`
	NeedsPublication bool `json:"needs_publication"`
}

// ResolvePipelineStageRestartPlan builds the restart plan for derived runs.
func ResolvePipelineStageRestartPlan(req RunSellerAIPipelineRequest) PipelineStageRestartPlan {
	plan := PipelineStageRestartPlan{
		SourcePipelineRunID:      req.SourcePipelineRunID,
		CreateNewPipelineRun:     true,
		RestartFromStage:         req.RestartFromStage,
		ReuseDataPack:            true,
		ReuseChunkSet:            true,
		ReuseSharedChunkAnalysis: req.RestartFromStage != ResumeStageSharedChunkValidation && req.RestartFromStage != ResumeStageSharedChunkAnalysis,
		ReuseGlobalContext:       req.RestartFromStage == ResumeStageActions,
		CloneDashboard:           req.RestartFromStage == ResumeStageActions && req.GenerateDashboard,
		ReuseActions:             false,
		NeedsActions:             req.GenerateActions,
		NeedsPublication:         req.GenerateDashboard && req.GenerateActions,
	}
	return plan
}
