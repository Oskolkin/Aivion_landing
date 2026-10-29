package aipipeline

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"

func actRawResponse(actRes *actions.GenerateActionsResult) *actions.AIRawResponse {
	if actRes == nil {
		return nil
	}
	return &actions.AIRawResponse{
		RequestID:    actRes.RequestID,
		InputTokens:  actRes.InputTokens,
		OutputTokens: actRes.OutputTokens,
		LatencyMs:    actRes.LatencyMs,
	}
}
