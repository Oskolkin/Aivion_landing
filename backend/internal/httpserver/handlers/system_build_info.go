package handlers

import (
	"net/http"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/buildinfo"
)

type SystemHandler struct {
	buildInfo *buildinfo.Provider
}

func NewSystemHandler(buildInfo *buildinfo.Provider) *SystemHandler {
	return &SystemHandler{buildInfo: buildInfo}
}

func (h *SystemHandler) GetBuildInfo(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.buildInfo == nil {
		writeJSONError(w, http.StatusInternalServerError, "build info unavailable")
		return
	}
	writeJSON(w, http.StatusOK, h.buildInfo.Info())
}
