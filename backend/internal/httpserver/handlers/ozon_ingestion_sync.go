package handlers

import (
	"net/http"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ingestion"
)

type OzonIngestionSyncHandler struct {
	orchestration *ingestion.OrchestrationService
}

func NewOzonIngestionSyncHandler(orchestration *ingestion.OrchestrationService) *OzonIngestionSyncHandler {
	return &OzonIngestionSyncHandler{
		orchestration: orchestration,
	}
}

func (h *OzonIngestionSyncHandler) StartInitialSync(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	job, err := h.orchestration.StartInitialSync(r.Context(), sellerAccount.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to start ingestion sync")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"sync_job": map[string]any{
			"id":     job.ID,
			"type":   job.Type,
			"status": job.Status,
		},
	})
}
