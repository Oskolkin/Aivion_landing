package handlers

import (
	"net/http"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/account"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
)

type AccountHandler struct {
	accountService *account.Service
}

func NewAccountHandler(accountService *account.Service) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
	}
}

func (h *AccountHandler) GetCurrentAccount(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	account, err := h.accountService.GetByUserID(r.Context(), user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to get seller account")
		return
	}

	writeJSON(w, http.StatusOK, accountResponse{
		ID:     account.ID,
		Name:   account.Name,
		Status: account.Status,
	})
}
