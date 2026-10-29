package dashboardhome

import (
	"context"
	"strings"
	"time"
)

// shouldOverrideTopActionsFromFinal reports whether GET may replace Top Actions
// from the actions pipeline. Unified publications freeze Top 3 at publish time.
func shouldOverrideTopActionsFromFinal(promptVersion string) bool {
	return !IsUnifiedPublicationPromptVersion(promptVersion)
}

func (s *Service) overrideTopActionsFromFinalActions(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	resp *Response,
) {
	if resp == nil {
		return
	}
	var pipelineRunID int64
	if stored := resp.AIDiagnostics.AccountSummary.storedNarrative; stored != nil && stored.AiPipelineRunID > 0 {
		pipelineRunID = stored.AiPipelineRunID
	}
	resolved := s.resolveDashboardTopActions(ctx, topActionsFromActionsInput{
		SellerAccountID: sellerAccountID,
		MetricDate:      metricDate,
		PipelineRunID:   pipelineRunID,
		Limit:           topActionsLimit,
		Fallback:        nil,
	})
	if resolved.Source == topActionsSourceFinal && len(resolved.Items) > 0 {
		resp.AIDiagnostics.TopActions = resolved.Items
		resp.TopActions = resolved.Items
	}
	if resolved.Source == topActionsSourceEmpty {
		resp.AIDiagnostics.TopActions = []TopActionItem{}
		resp.TopActions = []TopActionItem{}
	}
	if resolved.Source != "" {
		resp.AIDiagnostics.TopActionsSource = resolved.Source
	}
	if resolved.Pipeline != nil {
		if resp.Pipeline == nil {
			resp.Pipeline = resolved.Pipeline
		} else {
			mergeDashboardPipelineInfo(resp.Pipeline, resolved.Pipeline)
		}
		resp.Pipeline.TopActionsSource = resolved.Source
	}
}

func mergeDashboardPipelineInfo(dst, src *DashboardPipelineInfo) {
	if dst == nil || src == nil {
		return
	}
	if dst.PipelineRunID == 0 {
		dst.PipelineRunID = src.PipelineRunID
	}
	if dst.DataPackHash == "" {
		dst.DataPackHash = src.DataPackHash
	}
	if dst.GlobalContextHash == "" {
		dst.GlobalContextHash = src.GlobalContextHash
	}
	if dst.DashboardRunID == "" {
		dst.DashboardRunID = src.DashboardRunID
	}
	if dst.ActionsRunID == 0 {
		dst.ActionsRunID = src.ActionsRunID
	}
	if dst.TopActionsSource == "" {
		dst.TopActionsSource = src.TopActionsSource
	}
}

// finalizeResponseTopActions mirrors ai_diagnostics.top_actions to root top_actions when needed.
func finalizeResponseTopActions(resp *Response) {
	if resp == nil {
		return
	}
	if len(resp.TopActions) == 0 && len(resp.AIDiagnostics.TopActions) > 0 {
		resp.TopActions = resp.AIDiagnostics.TopActions
	}
	if resp.AIDiagnostics.TopActionsSource == "" && resp.Pipeline != nil {
		resp.AIDiagnostics.TopActionsSource = strings.TrimSpace(resp.Pipeline.TopActionsSource)
	}
}
