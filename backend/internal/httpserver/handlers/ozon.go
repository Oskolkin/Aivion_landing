// Ozon integration HTTP handlers.
//
// Active seller-facing API: GET/PUT /connection, POST /disconnect (see server.go).
// Legacy CRUD, manual checks, sync status, and manual initial-sync are admin-only.
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ingestion"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type OzonHandler struct {
	ozonService   *ozon.Service
	orchestration *ingestion.OrchestrationService
	demoChecker   DemoSellerChecker
}

func NewOzonHandler(ozonService *ozon.Service, orchestration *ingestion.OrchestrationService) *OzonHandler {
	return &OzonHandler{
		ozonService:   ozonService,
		orchestration: orchestration,
	}
}

func (h *OzonHandler) WithDemoSellerChecker(checker DemoSellerChecker) *OzonHandler {
	h.demoChecker = checker
	return h
}

type ozonUserConnectionResponse struct {
	ConnectionReady      bool                     `json:"connection_ready"`
	Status               string                   `json:"status"`
	AuthenticationMode   string                   `json:"authentication_mode"`
	Missing              []string                 `json:"missing"`
	Errors               []ozonConnectionErrorDTO `json:"errors"`
	SellerClientIDMasked string                   `json:"seller_client_id_masked,omitempty"`
	SellerCredentialsSet bool                     `json:"seller_credentials_set"`
	PerformanceSecretSet bool                     `json:"performance_secret_set"`
	RetentionDeleteAfter *string                  `json:"retention_delete_after"`
}

type ozonConnectionErrorDTO struct {
	Source  string `json:"source"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type putOzonUserConnectionRequest struct {
	SellerClientID          string `json:"seller_client_id"`
	SellerAPIKey            string `json:"seller_api_key"`
	PerformanceClientID     string `json:"performance_client_id"`
	PerformanceClientSecret string `json:"performance_client_secret"`
}

type disconnectOzonRequest struct {
	Confirmation string `json:"confirmation"`
}

func (h *OzonHandler) GetUserConnection(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if h.demoChecker != nil && h.demoChecker.IsStagingDemoSeller(sellerAccount.ID) {
		// Demo sellers browse imported snapshot without live Ozon credentials.
		// Report ready so the seller UI does not redirect to onboarding.
		writeJSON(w, http.StatusOK, ozonUserConnectionResponse{
			ConnectionReady:      true,
			Status:               "demo_ready",
			Missing:              []string{},
			Errors:               []ozonConnectionErrorDTO{},
			SellerCredentialsSet: true,
			PerformanceSecretSet: true,
		})
		return
	}

	state, err := h.ozonService.GetUserConnectionState(r.Context(), sellerAccount.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to get ozon connection")
		return
	}

	writeJSON(w, http.StatusOK, buildUserConnectionResponse(state))
}

func (h *OzonHandler) PutUserConnection(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if h.demoChecker != nil && h.demoChecker.IsStagingDemoSeller(sellerAccount.ID) {
		writeJSONError(w, http.StatusForbidden, "В демонстрационном режиме подключение Ozon недоступно.")
		return
	}

	var req putOzonUserConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.ozonService.PutUserConnection(r.Context(), ozon.PutConnectionInput{
		SellerAccountID:         sellerAccount.ID,
		SellerClientID:          req.SellerClientID,
		SellerAPIKey:            req.SellerAPIKey,
		PerformanceClientID:     req.PerformanceClientID,
		PerformanceClientSecret: req.PerformanceClientSecret,
	})
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(result.ValidationErrors) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"connection":       buildUserConnectionResponse(result.State),
			"errors":           mapConnectionErrors(result.ValidationErrors),
			"connection_ready": false,
		})
		return
	}

	if shouldEnqueueInitialSyncAfterPut(result, h.orchestration) {
		if _, syncErr := h.orchestration.StartInitialSync(r.Context(), sellerAccount.ID); syncErr != nil {
			// Connection is valid; sync enqueue failure is non-fatal for the user response.
			_ = syncErr
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"connection": buildUserConnectionResponse(result.State),
		"message":    "Кабинет подключён. Мы начинаем загрузку данных.",
	})
}

func shouldEnqueueInitialSyncAfterPut(result ozon.PutConnectionResult, orchestration *ingestion.OrchestrationService) bool {
	return result.InitialSyncStarted && orchestration != nil
}

func (h *OzonHandler) DisconnectUserConnection(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if h.demoChecker != nil && h.demoChecker.IsStagingDemoSeller(sellerAccount.ID) {
		writeJSONError(w, http.StatusForbidden, "В демонстрационном режиме отключение Ozon недоступно.")
		return
	}

	var req disconnectOzonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	state, err := h.ozonService.DisconnectUserConnection(r.Context(), sellerAccount.ID, strings.TrimSpace(req.Confirmation))
	if err != nil {
		if errors.Is(err, ozon.ErrConnectionNotFound) {
			writeJSONError(w, http.StatusNotFound, "ozon connection not found")
			return
		}
		if err.Error() == "invalid disconnect confirmation" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":   "invalid_confirmation",
				"message": "Для отключения введите слово ОТКЛЮЧИТЬ",
			})
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to disconnect ozon connection")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"connection": buildUserConnectionResponse(state),
	})
}

func buildUserConnectionResponse(state ozon.UserConnectionState) ozonUserConnectionResponse {
	resp := ozonUserConnectionResponse{
		ConnectionReady:      state.ConnectionReady,
		Status:               state.Status,
		AuthenticationMode:   state.AuthenticationMode,
		Missing:              state.Missing,
		Errors:               mapConnectionErrors(state.Errors),
		SellerClientIDMasked: state.SellerClientIDMasked,
		SellerCredentialsSet: state.SellerCredentialsSet,
		PerformanceSecretSet: state.PerformanceSecretSet,
	}
	if state.RetentionDeleteAfter != nil {
		v := state.RetentionDeleteAfter.Format(time.RFC3339)
		resp.RetentionDeleteAfter = &v
	}
	if resp.Missing == nil {
		resp.Missing = []string{}
	}
	if resp.Errors == nil {
		resp.Errors = []ozonConnectionErrorDTO{}
	}
	return resp
}

func mapConnectionErrors(errs []ozon.ConnectionError) []ozonConnectionErrorDTO {
	if len(errs) == 0 {
		return []ozonConnectionErrorDTO{}
	}
	out := make([]ozonConnectionErrorDTO, 0, len(errs))
	for _, e := range errs {
		out = append(out, ozonConnectionErrorDTO{
			Source:  e.Source,
			Code:    e.Code,
			Message: e.Message,
		})
	}
	return out
}

func (h *OzonHandler) GetConnection(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	connection, err := h.ozonService.GetBySellerAccountID(r.Context(), sellerAccount.ID)
	if err != nil {
		if err == ozon.ErrConnectionNotFound {
			writeJSON(w, http.StatusOK, map[string]any{
				"connection": nil,
			})
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to get ozon connection")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"connection": buildOzonConnectionResponse(connection, h.ozonService),
	})
}

func (h *OzonHandler) CreateConnection(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertOzonConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ClientID == "" || req.APIKey == "" {
		writeJSONError(w, http.StatusBadRequest, "client_id and api_key are required")
		return
	}

	connection, err := h.ozonService.Create(r.Context(), ozon.UpsertConnectionInput{
		SellerAccountID:        sellerAccount.ID,
		ClientID:               req.ClientID,
		APIKey:                 req.APIKey,
		PerformanceBearerToken: req.PerformanceBearerToken,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create ozon connection")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"connection": buildOzonConnectionResponse(connection, h.ozonService),
	})
}

func (h *OzonHandler) UpdateConnection(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req upsertOzonConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ClientID == "" || req.APIKey == "" {
		writeJSONError(w, http.StatusBadRequest, "client_id and api_key are required")
		return
	}

	connection, err := h.ozonService.Update(r.Context(), ozon.UpsertConnectionInput{
		SellerAccountID: sellerAccount.ID,
		ClientID:        req.ClientID,
		APIKey:          req.APIKey,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to update ozon connection")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"connection": buildOzonConnectionResponse(connection, h.ozonService),
	})
}

func (h *OzonHandler) PutPerformanceToken(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		PerformanceBearerToken *string `json:"performance_bearer_token,omitempty"`
		ClearPerformanceToken  bool    `json:"clear_performance_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var (
		connection dbgen.OzonConnection
		err        error
	)
	switch {
	case req.ClearPerformanceToken:
		connection, err = h.ozonService.ClearPerformanceBearerToken(r.Context(), sellerAccount.ID)
	case req.PerformanceBearerToken != nil && strings.TrimSpace(*req.PerformanceBearerToken) != "":
		connection, err = h.ozonService.SetPerformanceBearerToken(
			r.Context(),
			sellerAccount.ID,
			strings.TrimSpace(*req.PerformanceBearerToken),
		)
	default:
		connection, err = h.ozonService.GetBySellerAccountID(r.Context(), sellerAccount.ID)
	}
	if err != nil {
		if err == ozon.ErrConnectionNotFound {
			writeJSONError(w, http.StatusNotFound, "ozon connection not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to update performance token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"connection": buildOzonConnectionResponse(connection, h.ozonService),
	})
}

func buildOzonConnectionResponse(connection dbgen.OzonConnection, service *ozon.Service) ozonConnectionResponse {
	var lastCheckAt *string
	if connection.LastCheckAt.Valid {
		value := connection.LastCheckAt.Time.Format(time.RFC3339)
		lastCheckAt = &value
	}

	var lastCheckResult *string
	if connection.LastCheckResult.Valid {
		value := connection.LastCheckResult.String
		lastCheckResult = &value
	}

	var lastError *string
	if connection.LastError.Valid {
		value := connection.LastError.String
		lastError = &value
	}

	var perfLastCheckAt *string
	if connection.PerformanceLastCheckAt.Valid {
		v := connection.PerformanceLastCheckAt.Time.Format(time.RFC3339)
		perfLastCheckAt = &v
	}
	var perfLastCheckResult *string
	if connection.PerformanceLastCheckResult.Valid {
		v := connection.PerformanceLastCheckResult.String
		perfLastCheckResult = &v
	}
	var perfLastError *string
	if connection.PerformanceLastError.Valid {
		v := connection.PerformanceLastError.String
		perfLastError = &v
	}

	perfTokenSet := connection.PerformanceTokenEncrypted.Valid &&
		strings.TrimSpace(connection.PerformanceTokenEncrypted.String) != ""

	return ozonConnectionResponse{
		ID:              connection.ID,
		SellerAccountID: connection.SellerAccountID,
		Status:          connection.Status,
		LastCheckAt:     lastCheckAt,
		LastCheckResult: lastCheckResult,
		LastError:       lastError,
		HasCredentials:  true,
		ClientIDMasked:  service.MaskedClientID(connection),

		PerformanceTokenSet:        perfTokenSet,
		PerformanceStatus:          connection.PerformanceStatus,
		PerformanceLastCheckAt:     perfLastCheckAt,
		PerformanceLastCheckResult: perfLastCheckResult,
		PerformanceLastError:       perfLastError,
	}
}

func (h *OzonHandler) CheckConnection(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	result, err := h.ozonService.CheckConnection(r.Context(), sellerAccount.ID)
	if err != nil {
		if err == ozon.ErrConnectionNotFound {
			writeJSONError(w, http.StatusNotFound, "ozon connection not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to check ozon connection")
		return
	}

	writeJSON(w, http.StatusOK, ozonCheckResponse{
		Status:    result.Status,
		CheckedAt: result.CheckedAt.Format(time.RFC3339),
		Message:   result.Message,
		ErrorCode: result.ErrorCode,
	})
}

func (h *OzonHandler) CheckPerformanceConnection(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	result, err := h.ozonService.CheckPerformanceConnection(r.Context(), sellerAccount.ID)
	if err != nil {
		if err == ozon.ErrConnectionNotFound {
			writeJSONError(w, http.StatusNotFound, "ozon connection not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to check ozon performance connection")
		return
	}

	writeJSON(w, http.StatusOK, ozonCheckResponse{
		Status:    result.Status,
		CheckedAt: result.CheckedAt.Format(time.RFC3339),
		Message:   result.Message,
		ErrorCode: result.ErrorCode,
	})
}
