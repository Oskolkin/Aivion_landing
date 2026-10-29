package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

type dataPackBuilder interface {
	Build(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*aidatapack.SellerAIDataPack, error)
}

type AIDataPackHandler struct {
	svc dataPackBuilder
}

func NewAIDataPackHandler(svc *aidatapack.Service) *AIDataPackHandler {
	return &AIDataPackHandler{svc: svc}
}

func NewAIDataPackHandlerWithBuilder(svc dataPackBuilder) *AIDataPackHandler {
	return &AIDataPackHandler{svc: svc}
}

func (h *AIDataPackHandler) Get(w http.ResponseWriter, r *http.Request) {
	sellerID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("seller_id")), 10, 64)
	if err != nil || sellerID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "seller_id is required")
		return
	}
	metricDateRaw := strings.TrimSpace(r.URL.Query().Get("metric_date"))
	if metricDateRaw == "" {
		writeJSONError(w, http.StatusBadRequest, "metric_date is required")
		return
	}
	metricDate, err := time.Parse("2006-01-02", metricDateRaw)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid metric_date")
		return
	}
	metricDate = time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)

	pack, err := h.svc.Build(r.Context(), sellerID, metricDate)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	audit := strings.TrimSpace(r.URL.Query().Get("audit"))
	if audit == "1" || strings.EqualFold(audit, "true") {
		writeJSON(w, http.StatusOK, map[string]any{
			"pack":  pack,
			"audit": aidatapack.AuditDataPack(pack),
		})
		return
	}

	writeJSON(w, http.StatusOK, pack)
}

// MarshalAIDataPackPretty returns JSON bytes for devtools output.
func MarshalAIDataPackPretty(pack *aidatapack.SellerAIDataPack, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(pack, "", "  ")
	}
	return json.Marshal(pack)
}
