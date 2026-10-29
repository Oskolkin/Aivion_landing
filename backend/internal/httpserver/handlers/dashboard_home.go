package handlers

import (
	"net/http"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboard"
)

type DashboardHomeHandler struct {
	pub *PublicationDashboardHandler
}

func NewDashboardHomeHandler(service *dashboard.Service) *DashboardHomeHandler {
	return &DashboardHomeHandler{pub: NewPublicationDashboardHandler(service)}
}

func (h *DashboardHomeHandler) GetHome(w http.ResponseWriter, r *http.Request) {
	h.pub.GetHome(w, r)
}

func (h *DashboardHomeHandler) GetStockSKUs(w http.ResponseWriter, r *http.Request) {
	h.pub.GetStockSKUs(w, r)
}

func (h *DashboardHomeHandler) GetAdCampaigns(w http.ResponseWriter, r *http.Request) {
	h.pub.GetAdCampaigns(w, r)
}
