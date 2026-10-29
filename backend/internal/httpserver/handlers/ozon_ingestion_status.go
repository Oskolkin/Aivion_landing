package handlers

import (
	"errors"
	"net/http"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ingestion"
)

type OzonIngestionStatusHandler struct {
	statusService *ingestion.StatusService
}

func NewOzonIngestionStatusHandler(statusService *ingestion.StatusService) *OzonIngestionStatusHandler {
	return &OzonIngestionStatusHandler{
		statusService: statusService,
	}
}

func (h *OzonIngestionStatusHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	status, err := h.statusService.GetStatus(r.Context(), sellerAccount.ID)
	if err != nil {
		if errors.Is(err, ingestion.ErrStatusConnectionNotFound) {
			writeJSON(w, http.StatusOK, ingestion.StatusResult{
				ConnectionStatus:            "not_connected",
				LastCheckAt:                 nil,
				LastCheckResult:             nil,
				LastError:                   nil,
				PerformanceConnectionStatus: "not_connected",
				PerformanceTokenSet:         false,
				PerformanceLastCheckAt:      nil,
				PerformanceLastCheckResult:  nil,
				PerformanceLastError:        nil,
				CurrentSync:                 nil,
				LastSuccessfulSyncAt:        nil,
				LatestImportJobs:            []ingestion.ImportJobStatusDTO{},
			})
			return
		}

		writeJSONError(w, http.StatusInternalServerError, "failed to get ingestion sync status")
		return
	}

	writeJSON(w, http.StatusOK, status)
}
