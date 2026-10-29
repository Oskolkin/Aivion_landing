package ozon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	ozonperf "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon/performance"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const disconnectConfirmationPhrase = "ОТКЛЮЧИТЬ"

// PutConnectionInput is the unified user-facing credential payload.
type PutConnectionInput struct {
	SellerAccountID         int64
	SellerClientID          string
	SellerAPIKey            string
	PerformanceClientID     string
	PerformanceClientSecret string
}

// PutConnectionResult is returned after attempting to save validated credentials.
type PutConnectionResult struct {
	State              UserConnectionState
	ValidationErrors   []ConnectionError
	InitialSyncStarted bool
}

func (s *Service) GetUserConnectionState(ctx context.Context, sellerAccountID int64) (UserConnectionState, error) {
	conn, err := s.GetBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		if errors.Is(err, ErrConnectionNotFound) {
			return UserConnectionState{
				ConnectionReady: false,
				Status:          "not_connected",
				Missing:         []string{"seller_api", "performance_api"},
			}, nil
		}
		return UserConnectionState{}, err
	}
	return BuildUserConnectionState(conn, s), nil
}

func (s *Service) PutUserConnection(ctx context.Context, input PutConnectionInput) (PutConnectionResult, error) {
	sellerClientID := strings.TrimSpace(input.SellerClientID)
	sellerAPIKey := strings.TrimSpace(input.SellerAPIKey)
	perfClientID := strings.TrimSpace(input.PerformanceClientID)
	perfSecret := strings.TrimSpace(input.PerformanceClientSecret)

	if sellerClientID == "" || sellerAPIKey == "" || perfClientID == "" || perfSecret == "" {
		return PutConnectionResult{}, fmt.Errorf("all credential fields are required")
	}

	var existing dbgen.OzonConnection
	hasExisting := false
	if conn, err := s.GetBySellerAccountID(ctx, input.SellerAccountID); err == nil {
		existing = conn
		hasExisting = true
	} else if !errors.Is(err, ErrConnectionNotFound) {
		return PutConnectionResult{}, err
	}

	sellerErr := s.client.CheckConnection(ctx, sellerClientID, sellerAPIKey)
	sellerConnErr := mapSellerCheckError(sellerErr)

	tokenResp, perfErr := s.performanceAuth.ObtainToken(ctx, perfClientID, perfSecret)
	perfConnErr := mapPerformanceCheckError(perfErr)

	var validationErrors []ConnectionError
	if sellerConnErr.Code != "" {
		validationErrors = append(validationErrors, sellerConnErr)
	}
	if perfConnErr.Code != "" {
		validationErrors = append(validationErrors, perfConnErr)
	}

	if len(validationErrors) > 0 {
		if preserveConnectionOnTemporaryValidationFailure(existing, validationErrors) {
			state := BuildUserConnectionState(existing, s)
			state.Errors = validationErrors
			return PutConnectionResult{State: state, ValidationErrors: validationErrors}, nil
		}

		if hasExisting {
			errMsg := validationErrors[0].Message
			errCode := validationErrors[0].Code
			if len(validationErrors) > 1 {
				errMsg = validationErrors[0].Message + " " + validationErrors[1].Message
			}
			_, _ = s.queries.UpdateOzonConnectionValidationErrors(ctx, dbgen.UpdateOzonConnectionValidationErrorsParams{
				SellerAccountID:        input.SellerAccountID,
				SellerApiValid:         sellerConnErr.Code == "",
				PerformanceApiValid:    perfConnErr.Code == "",
				ConnectionStatus:       "connection_error",
				ConnectionErrorCode:    pgtype.Text{String: errCode, Valid: errCode != ""},
				ConnectionErrorMessage: pgtype.Text{String: errMsg, Valid: true},
			})
		}

		missing := []string{}
		if sellerConnErr.Code != "" {
			missing = append(missing, "seller_api")
		}
		if perfConnErr.Code != "" {
			missing = append(missing, "performance_api")
		}
		return PutConnectionResult{
			State: UserConnectionState{
				ConnectionReady: false,
				Status:          "connection_error",
				Missing:         missing,
				Errors:          validationErrors,
			},
			ValidationErrors: validationErrors,
		}, nil
	}

	// Probe Performance API with obtained token (empty campaign list is valid).
	if probeErr := s.probePerformanceAPI(ctx, tokenResp.AccessToken); probeErr != nil {
		perfConnErr = mapPerformanceCheckError(probeErr)
		if preserveConnectionOnTemporaryValidationFailure(existing, []ConnectionError{perfConnErr}) {
			state := BuildUserConnectionState(existing, s)
			state.Errors = []ConnectionError{perfConnErr}
			return PutConnectionResult{State: state, ValidationErrors: []ConnectionError{perfConnErr}}, nil
		}
		return PutConnectionResult{
			State: UserConnectionState{
				ConnectionReady: false,
				Status:          "connection_error",
				Missing:         []string{"performance_api"},
				Errors:          []ConnectionError{perfConnErr},
			},
			ValidationErrors: []ConnectionError{perfConnErr},
		}, nil
	}

	clientIDEnc, err := s.secretCodec.Encrypt(sellerClientID)
	if err != nil {
		return PutConnectionResult{}, err
	}
	apiKeyEnc, err := s.secretCodec.Encrypt(sellerAPIKey)
	if err != nil {
		return PutConnectionResult{}, err
	}
	perfIDEnc, err := s.secretCodec.Encrypt(perfClientID)
	if err != nil {
		return PutConnectionResult{}, err
	}
	perfSecretEnc, err := s.secretCodec.Encrypt(perfSecret)
	if err != nil {
		return PutConnectionResult{}, err
	}
	tokenEnc, err := s.secretCodec.Encrypt(tokenResp.AccessToken)
	if err != nil {
		return PutConnectionResult{}, err
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if tokenResp.ExpiresIn <= 0 {
		expiresAt = time.Now().Add(30 * time.Minute)
	}

	wasReady := hasExisting && ConnectionReady(existing)

	conn, err := s.queries.UpsertOzonConnectionValidated(ctx, dbgen.UpsertOzonConnectionValidatedParams{
		SellerAccountID:                  input.SellerAccountID,
		ClientIDEncrypted:                clientIDEnc,
		ApiKeyEncrypted:                  apiKeyEnc,
		PerformanceClientIDEncrypted:     pgtype.Text{String: perfIDEnc, Valid: true},
		PerformanceClientSecretEncrypted: pgtype.Text{String: perfSecretEnc, Valid: true},
		PerformanceTokenEncrypted:        pgtype.Text{String: tokenEnc, Valid: true},
		PerformanceTokenExpiresAt:        pgtypeTimestamptz(expiresAt),
	})
	if err != nil {
		return PutConnectionResult{}, fmt.Errorf("upsert ozon connection: %w", err)
	}

	state := BuildUserConnectionState(conn, s)
	result := PutConnectionResult{
		State:              state,
		InitialSyncStarted: initialSyncStarted(wasReady, state.ConnectionReady),
	}
	return result, nil
}

func initialSyncStarted(wasReady, connectionReady bool) bool {
	return !wasReady && connectionReady
}

// probePerformanceAPI checks bearer access; an empty campaigns list is not an error.
func (s *Service) probePerformanceAPI(ctx context.Context, accessToken string) error {
	_, err := s.performance.ListCampaigns(ctx, "", accessToken, ozonperf.ListCampaignsRequest{Page: 1, PageSize: 1})
	return err
}

// preserveConnectionOnTemporaryValidationFailure keeps a working connection when Ozon is temporarily unavailable.
func preserveConnectionOnTemporaryValidationFailure(existing dbgen.OzonConnection, validationErrors []ConnectionError) bool {
	if !ConnectionReady(existing) || len(validationErrors) == 0 {
		return false
	}
	for _, ve := range validationErrors {
		if IsCredentialOrAccessProblem(ve.Code) {
			return false
		}
	}
	return true
}

func (s *Service) DisconnectUserConnection(ctx context.Context, sellerAccountID int64, confirmation string) (UserConnectionState, error) {
	if confirmation != disconnectConfirmationPhrase {
		return UserConnectionState{}, fmt.Errorf("invalid disconnect confirmation")
	}

	emptyEnc, err := s.secretCodec.Encrypt("")
	if err != nil {
		return UserConnectionState{}, err
	}

	now := time.Now()
	days := s.retentionDays
	if days <= 0 {
		days = 90
	}
	retentionUntil := now.Add(time.Duration(days) * 24 * time.Hour)

	conn, err := s.queries.DisconnectOzonConnection(ctx, dbgen.DisconnectOzonConnectionParams{
		SellerAccountID:      sellerAccountID,
		ClientIDEncrypted:    emptyEnc,
		ApiKeyEncrypted:      emptyEnc,
		DisconnectedAt:       pgtypeTimestamptz(now),
		RetentionDeleteAfter: pgtypeTimestamptz(retentionUntil),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserConnectionState{}, ErrConnectionNotFound
		}
		return UserConnectionState{}, fmt.Errorf("disconnect ozon connection: %w", err)
	}

	return BuildUserConnectionState(conn, s), nil
}

func (s *Service) ensurePerformanceAccessToken(ctx context.Context, conn dbgen.OzonConnection) (string, dbgen.OzonConnection, error) {
	if conn.PerformanceTokenEncrypted.Valid && conn.PerformanceTokenExpiresAt.Valid {
		if time.Until(conn.PerformanceTokenExpiresAt.Time) > 2*time.Minute {
			token, err := s.secretCodec.Decrypt(conn.PerformanceTokenEncrypted.String)
			if err == nil && strings.TrimSpace(token) != "" {
				return strings.TrimSpace(token), conn, nil
			}
		}
	}

	perfClientID := ""
	if conn.PerformanceClientIDEncrypted.Valid {
		var err error
		perfClientID, err = s.secretCodec.Decrypt(conn.PerformanceClientIDEncrypted.String)
		if err != nil {
			return "", conn, err
		}
	}
	perfSecret := ""
	if conn.PerformanceClientSecretEncrypted.Valid {
		var err error
		perfSecret, err = s.secretCodec.Decrypt(conn.PerformanceClientSecretEncrypted.String)
		if err != nil {
			return "", conn, err
		}
	}

	if strings.TrimSpace(perfClientID) != "" && strings.TrimSpace(perfSecret) != "" {
		tokenResp, err := s.performanceAuth.ObtainToken(ctx, perfClientID, perfSecret)
		if err != nil {
			return "", conn, err
		}
		tokenEnc, err := s.secretCodec.Encrypt(tokenResp.AccessToken)
		if err != nil {
			return "", conn, err
		}
		expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		if tokenResp.ExpiresIn <= 0 {
			expiresAt = time.Now().Add(30 * time.Minute)
		}
		updated, err := s.queries.UpsertOzonConnectionValidated(ctx, dbgen.UpsertOzonConnectionValidatedParams{
			SellerAccountID:                  conn.SellerAccountID,
			ClientIDEncrypted:                conn.ClientIDEncrypted,
			ApiKeyEncrypted:                  conn.ApiKeyEncrypted,
			PerformanceClientIDEncrypted:     conn.PerformanceClientIDEncrypted,
			PerformanceClientSecretEncrypted: conn.PerformanceClientSecretEncrypted,
			PerformanceTokenEncrypted:        pgtype.Text{String: tokenEnc, Valid: true},
			PerformanceTokenExpiresAt:        pgtypeTimestamptz(expiresAt),
		})
		if err != nil {
			return "", conn, err
		}
		return tokenResp.AccessToken, updated, nil
	}

	// Legacy bearer token only.
	if conn.PerformanceTokenEncrypted.Valid {
		token, err := s.secretCodec.Decrypt(conn.PerformanceTokenEncrypted.String)
		if err != nil {
			return "", conn, err
		}
		return strings.TrimSpace(token), conn, nil
	}

	return "", conn, ErrPerformanceTokenNotConfigured
}
