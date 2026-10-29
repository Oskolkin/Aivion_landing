package aipipeline

import (
	"context"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

const (
	ResumeStageDataPack              = "data_pack"
	ResumeStageChunkSet              = "chunk_set"
	ResumeStageSharedChunkValidation = "shared_chunk_validation"
	ResumeStageSharedChunkAnalysis   = "shared_chunk_analysis"
	ResumeStageGlobalContext         = "global_context"
	ResumeStageDashboard             = "dashboard"
	ResumeStageActions               = "actions"
	ResumeStagePublication           = "publication"
	ResumeStageCompleted             = "completed"
)

// PipelineResumePlan describes which pipeline stages will be reused vs executed on resume.
type PipelineResumePlan struct {
	ResumeFromStage string `json:"resume_from_stage"`

	ReuseDataPack            bool `json:"reuse_data_pack"`
	ReuseChunkSet            bool `json:"reuse_chunk_set"`
	ReuseSharedChunkAnalysis bool `json:"reuse_shared_chunk_analysis"`
	ReuseGlobalContext       bool `json:"reuse_global_context"`
	ReuseDashboard           bool `json:"reuse_dashboard"`
	ReuseActions             bool `json:"reuse_actions"`

	NeedsGlobalContext bool `json:"needs_global_context"`
	NeedsDashboard     bool `json:"needs_dashboard"`
	NeedsActions       bool `json:"needs_actions"`
	NeedsPublication   bool `json:"needs_publication"`

	Reasons []string `json:"reasons,omitempty"`
}

type resumePlanInput struct {
	Req                   RunSellerAIPipelineRequest
	Persisted             dbgen.AiPipelineRun
	DataPackHash          string
	GlobalContextHash     string
	DashboardReusable     bool
	ActionsReusable       bool
	SharedChunkReusable   bool
	GlobalContextReusable bool
	Published             bool
}

func (s *SellerAIPipelineService) buildResumePlanInput(
	ctx context.Context,
	req RunSellerAIPipelineRequest,
	persisted dbgen.AiPipelineRun,
	dataPackHash string,
	globalContextHash string,
	globalContextLoadedFromArtifact bool,
) resumePlanInput {
	in := resumePlanInput{
		Req:               req,
		Persisted:         persisted,
		DataPackHash:      dataPackHash,
		GlobalContextHash: globalContextHash,
		Published:         persisted.PublishedAt.Valid,
	}
	if SharedChunkAnalysisEnabled() && pipelineResumeMode(req) {
		in.SharedChunkReusable = textOrEmpty(persisted.GlobalContextStatus) == GlobalContextStatusCompleted ||
			textOrEmpty(persisted.GlobalContextStatus) == GlobalContextStatusFailed ||
			persistedDashboardCompleted(persisted) ||
			textOrEmpty(persisted.ActionsStatus) != ""
	}
	gcStatus := textOrEmpty(persisted.GlobalContextStatus)
	if gcStatus == GlobalContextStatusCompleted && persistedResumeHashesCompatible(persisted, dataPackHash, globalContextHash) {
		savedGC := strings.TrimSpace(textOrEmpty(persisted.GlobalContextHash))
		currentGC := strings.TrimSpace(globalContextHash)
		switch {
		case globalContextLoadedFromArtifact:
			in.GlobalContextReusable = true
		case savedGC == "" || savedGC == currentGC:
			in.GlobalContextReusable = true
		}
	}
	if req.GenerateDashboard {
		in.DashboardReusable = s.canReusePersistedDashboard(ctx, persisted, req.SellerAccountID, dataPackHash, globalContextHash)
	}
	if req.GenerateActions {
		in.ActionsReusable = s.persistedActionsReusable(ctx, persisted, req.SellerAccountID, dataPackHash, globalContextHash)
	}
	return in
}

// ResolvePipelineResumePlan determines the first incomplete stage and reuse flags for resume.
func ResolvePipelineResumePlan(in resumePlanInput) PipelineResumePlan {
	plan := PipelineResumePlan{
		ReuseSharedChunkAnalysis: in.SharedChunkReusable,
		ReuseGlobalContext:       in.GlobalContextReusable,
		ReuseDashboard:           in.DashboardReusable,
		ReuseActions:             in.ActionsReusable,
	}
	if pipelineResumeMode(in.Req) {
		plan.ReuseDataPack = true
		plan.ReuseChunkSet = true
	}
	if !pipelineResumeMode(in.Req) {
		plan.ResumeFromStage = ResumeStageDataPack
		plan.NeedsGlobalContext = true
		plan.NeedsDashboard = in.Req.GenerateDashboard
		plan.NeedsActions = in.Req.GenerateActions
		plan.NeedsPublication = in.Req.GenerateDashboard && in.Req.GenerateActions
		plan.Reasons = append(plan.Reasons, "new_pipeline_run")
		return plan
	}
	if in.Published {
		plan.ResumeFromStage = ResumeStageCompleted
		plan.Reasons = append(plan.Reasons, "already_published")
		return plan
	}

	needsGC := true
	if in.GlobalContextReusable {
		needsGC = false
		plan.ReuseGlobalContext = true
	} else if gcStatus := textOrEmpty(in.Persisted.GlobalContextStatus); gcStatus == GlobalContextStatusCompleted {
		plan.Reasons = append(plan.Reasons, "global_context_integrity_mismatch")
	}
	plan.NeedsGlobalContext = needsGC

	if needsGC {
		plan.ResumeFromStage = ResumeStageGlobalContext
		plan.NeedsDashboard = in.Req.GenerateDashboard
		plan.NeedsActions = in.Req.GenerateActions
		plan.NeedsPublication = in.Req.GenerateDashboard && in.Req.GenerateActions
		if SharedChunkAnalysisEnabled() && !in.SharedChunkReusable {
			plan.ResumeFromStage = ResumeStageSharedChunkAnalysis
			plan.ReuseSharedChunkAnalysis = false
		}
		return plan
	}

	needsDashboard := in.Req.GenerateDashboard && !in.DashboardReusable
	if needsDashboard {
		plan.ResumeFromStage = ResumeStageDashboard
		plan.NeedsDashboard = true
		plan.NeedsActions = in.Req.GenerateActions
		plan.NeedsPublication = in.Req.GenerateDashboard && in.Req.GenerateActions
		if in.Req.GenerateDashboard && persistedDashboardCompleted(in.Persisted) && !in.DashboardReusable {
			plan.Reasons = append(plan.Reasons, "dashboard_integrity_mismatch")
		}
		return plan
	}
	if in.Req.GenerateDashboard {
		plan.ReuseDashboard = true
	}

	needsActions := in.Req.GenerateActions && !in.ActionsReusable
	if needsActions {
		plan.ResumeFromStage = ResumeStageActions
		plan.NeedsActions = true
		plan.NeedsPublication = in.Req.GenerateDashboard && in.Req.GenerateActions
		if actionsStatus := textOrEmpty(in.Persisted.ActionsStatus); actionsStatus == StepStatusFailed {
			plan.Reasons = append(plan.Reasons, "actions_failed_retry")
		}
		return plan
	}
	if in.Req.GenerateActions && in.ActionsReusable {
		plan.ReuseActions = true
	}

	if in.Req.GenerateDashboard && in.Req.GenerateActions && actions.IsPublishableActionFinalStatus(textOrEmpty(in.Persisted.ActionsStatus)) {
		plan.ResumeFromStage = ResumeStagePublication
		plan.NeedsPublication = true
		plan.Reasons = append(plan.Reasons, "awaiting_publication")
		return plan
	}

	plan.ResumeFromStage = ResumeStageCompleted
	plan.Reasons = append(plan.Reasons, "all_stages_complete")
	return plan
}

func applyResumePlanDiagnostics(diag *PipelineDiagnostics, plan PipelineResumePlan, dataPackSource string) {
	if diag == nil {
		return
	}
	if diag.Resume == nil {
		diag.Resume = &ResumeDiagnostics{}
	}
	diag.Resume.ResumeFromStage = plan.ResumeFromStage
	diag.Resume.ReuseDataPack = plan.ReuseDataPack
	diag.Resume.ReuseChunkSet = plan.ReuseChunkSet
	diag.Resume.ReuseSharedChunkAnalysis = plan.ReuseSharedChunkAnalysis
	diag.Resume.ReuseGlobalContext = plan.ReuseGlobalContext
	diag.Resume.ReuseDashboard = plan.ReuseDashboard
	diag.Resume.ReuseActions = plan.ReuseActions
	diag.Resume.NeedsActions = plan.NeedsActions
	diag.Resume.NeedsPublication = plan.NeedsPublication
	diag.Resume.PlanReasons = append([]string(nil), plan.Reasons...)
	if dataPackSource != "" {
		diag.Resume.DataPackSource = dataPackSource
	}
}
