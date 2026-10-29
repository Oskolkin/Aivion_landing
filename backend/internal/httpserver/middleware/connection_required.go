package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

// DemoSellerChecker reports whether a seller is allowed to use staging demo data
// (source_mode=demo) without a live Ozon connection.
type DemoSellerChecker interface {
	IsStagingDemoSeller(sellerID int64) bool
}

// RequireOzonConnection blocks user-facing routes until Seller + Performance APIs are validated.
// Staging demo sellers (explicit allow-list) bypass the live Ozon connection gate without
// weakening the gate for ordinary sellers.
func RequireOzonConnection(ozonService *ozon.Service, demo DemoSellerChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
			if !ok {
				writeConnectionJSONError(w, http.StatusUnauthorized, "unauthorized", false, nil)
				return
			}

			if demo != nil && demo.IsStagingDemoSeller(sellerAccount.ID) {
				next.ServeHTTP(w, r)
				return
			}

			state, err := ozonService.GetUserConnectionState(r.Context(), sellerAccount.ID)
			if err != nil {
				writeConnectionJSONError(w, http.StatusInternalServerError, "failed to check ozon connection", false, nil)
				return
			}

			if !state.ConnectionReady {
				writeConnectionRequired(w, state)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeConnectionRequired(w http.ResponseWriter, state ozon.UserConnectionState) {
	msg := "Чтобы продолжить, подключите кабинет Ozon."
	if state.Status == "disconnected_retention" {
		msg = "Кабинет отключён. Подключите кабинет Ozon снова, чтобы продолжить работу."
	}
	writeConnectionJSONError(w, http.StatusConflict, msg, false, state.Missing)
}

func writeConnectionJSONError(w http.ResponseWriter, status int, message string, ready bool, missing []string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":            "connection_required",
		"message":          message,
		"connection_ready": ready,
		"missing":          missing,
	})
}
