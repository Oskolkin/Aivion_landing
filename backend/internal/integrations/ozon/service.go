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
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrConnectionNotFound            = errors.New("ozon connection not found")
	ErrPerformanceTokenNotConfigured = errors.New("ozon performance API bearer token is not configured")
)

type Service struct {
	pool            *pgxpool.Pool
	queries         *dbgen.Queries
	secretCodec     *SecretCodec
	client          *Client
	performance     *ozonperf.Client
	performanceAuth *ozonperf.AuthClient
	retentionDays   int
	oauthTokens     OAuthTokenClient
}

func NewService(db *pgxpool.Pool, encryptionKey string, performanceAuthBaseURL string, retentionDays int) (*Service, error) {
	codec, err := NewSecretCodec(encryptionKey)
	if err != nil {
		return nil, err
	}
	if retentionDays <= 0 {
		retentionDays = 90
	}

	return &Service{
		pool:            db,
		queries:         dbgen.New(db),
		secretCodec:     codec,
		client:          NewClient(),
		performance:     ozonperf.NewClient(),
		performanceAuth: ozonperf.NewAuthClient(performanceAuthBaseURL),
		retentionDays:   retentionDays,
		oauthTokens:     newPrivateOAuthTokenClientFromEnv(),
	}, nil
}

type UpsertConnectionInput struct {
	SellerAccountID int64
	ClientID        string
	APIKey          string
	// Optional on create only; use SetPerformanceBearerToken for updates.
	PerformanceBearerToken *string
}

type CheckConnectionResult struct {
	Status    string
	CheckedAt time.Time
	Message   string
	ErrorCode *string
}

type DecryptedCredentials struct {
	ClientID string
	APIKey   string
}

func (s *Service) GetBySellerAccountID(ctx context.Context, sellerAccountID int64) (dbgen.OzonConnection, error) {
	connection, err := s.queries.GetOzonConnectionBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.OzonConnection{}, ErrConnectionNotFound
		}
		return dbgen.OzonConnection{}, fmt.Errorf("get ozon connection: %w", err)
	}

	return connection, nil
}

func (s *Service) GetDecryptedCredentials(ctx context.Context, sellerAccountID int64) (DecryptedCredentials, error) {
	connection, err := s.GetBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return DecryptedCredentials{}, err
	}

	clientID, err := s.secretCodec.Decrypt(connection.ClientIDEncrypted)
	if err != nil {
		return DecryptedCredentials{}, fmt.Errorf("decrypt client id: %w", err)
	}

	apiKey, err := s.secretCodec.Decrypt(connection.ApiKeyEncrypted)
	if err != nil {
		return DecryptedCredentials{}, fmt.Errorf("decrypt api key: %w", err)
	}

	return DecryptedCredentials{
		ClientID: clientID,
		APIKey:   apiKey,
	}, nil
}

func (s *Service) GetDecryptedPerformanceBearerToken(ctx context.Context, sellerAccountID int64) (string, error) {
	connection, err := s.GetBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return "", err
	}

	token, _, err := s.ensurePerformanceAccessToken(ctx, connection)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		return "", ErrPerformanceTokenNotConfigured
	}
	return token, nil
}

func (s *Service) SetPerformanceBearerToken(ctx context.Context, sellerAccountID int64, plainToken string) (dbgen.OzonConnection, error) {
	plainToken = strings.TrimSpace(plainToken)
	if plainToken == "" {
		return dbgen.OzonConnection{}, fmt.Errorf("performance bearer token is empty")
	}

	encrypted, err := s.secretCodec.Encrypt(plainToken)
	if err != nil {
		return dbgen.OzonConnection{}, fmt.Errorf("encrypt performance bearer token: %w", err)
	}

	connection, err := s.queries.UpdateOzonPerformanceBearerToken(ctx, dbgen.UpdateOzonPerformanceBearerTokenParams{
		SellerAccountID: sellerAccountID,
		PerformanceTokenEncrypted: pgtype.Text{
			String: encrypted,
			Valid:  true,
		},
	})
	if err != nil {
		return dbgen.OzonConnection{}, fmt.Errorf("update ozon performance bearer token: %w", err)
	}

	return connection, nil
}

func (s *Service) ClearPerformanceBearerToken(ctx context.Context, sellerAccountID int64) (dbgen.OzonConnection, error) {
	connection, err := s.queries.ClearOzonPerformanceBearerToken(ctx, sellerAccountID)
	if err != nil {
		return dbgen.OzonConnection{}, fmt.Errorf("clear ozon performance bearer token: %w", err)
	}

	return connection, nil
}

func (s *Service) Create(ctx context.Context, input UpsertConnectionInput) (dbgen.OzonConnection, error) {
	clientIDEncrypted, err := s.secretCodec.Encrypt(input.ClientID)
	if err != nil {
		return dbgen.OzonConnection{}, fmt.Errorf("encrypt client id: %w", err)
	}

	apiKeyEncrypted, err := s.secretCodec.Encrypt(input.APIKey)
	if err != nil {
		return dbgen.OzonConnection{}, fmt.Errorf("encrypt api key: %w", err)
	}

	connection, err := s.queries.CreateOzonConnection(ctx, dbgen.CreateOzonConnectionParams{
		SellerAccountID:   input.SellerAccountID,
		ClientIDEncrypted: clientIDEncrypted,
		ApiKeyEncrypted:   apiKeyEncrypted,
		Status:            "draft",
		LastCheckAt:       pgtypeTimestamptzNull(),
		LastCheckResult:   pgtypeTextNull(),
		LastError:         pgtypeTextNull(),
	})
	if err != nil {
		return dbgen.OzonConnection{}, fmt.Errorf("create ozon connection: %w", err)
	}

	if input.PerformanceBearerToken != nil {
		t := strings.TrimSpace(*input.PerformanceBearerToken)
		if t != "" {
			return s.SetPerformanceBearerToken(ctx, input.SellerAccountID, t)
		}
	}

	return connection, nil
}

func (s *Service) Update(ctx context.Context, input UpsertConnectionInput) (dbgen.OzonConnection, error) {
	clientIDEncrypted, err := s.secretCodec.Encrypt(input.ClientID)
	if err != nil {
		return dbgen.OzonConnection{}, fmt.Errorf("encrypt client id: %w", err)
	}

	apiKeyEncrypted, err := s.secretCodec.Encrypt(input.APIKey)
	if err != nil {
		return dbgen.OzonConnection{}, fmt.Errorf("encrypt api key: %w", err)
	}

	connection, err := s.queries.UpdateOzonConnectionCredentials(ctx, dbgen.UpdateOzonConnectionCredentialsParams{
		SellerAccountID:   input.SellerAccountID,
		ClientIDEncrypted: clientIDEncrypted,
		ApiKeyEncrypted:   apiKeyEncrypted,
		Status:            "draft",
		LastCheckAt:       pgtypeTimestamptzNull(),
		LastCheckResult:   pgtypeTextNull(),
		LastError:         pgtypeTextNull(),
	})
	if err != nil {
		return dbgen.OzonConnection{}, fmt.Errorf("update ozon connection: %w", err)
	}

	return connection, nil
}

func (s *Service) CheckConnection(ctx context.Context, sellerAccountID int64) (CheckConnectionResult, error) {
	connection, err := s.GetBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return CheckConnectionResult{}, err
	}

	clientID, err := s.secretCodec.Decrypt(connection.ClientIDEncrypted)
	if err != nil {
		return CheckConnectionResult{}, fmt.Errorf("decrypt client id: %w", err)
	}

	apiKey, err := s.secretCodec.Decrypt(connection.ApiKeyEncrypted)
	if err != nil {
		return CheckConnectionResult{}, fmt.Errorf("decrypt api key: %w", err)
	}

	checkedAt := time.Now()

	var (
		status          string
		lastCheckResult string
		lastError       string
		message         string
		errorCode       *string
	)

	err = s.client.CheckConnection(ctx, clientID, apiKey)
	switch {
	case err == nil:
		status = "valid"
		lastCheckResult = "valid"
		lastError = ""
		message = "connection is valid"

	case errors.Is(err, ErrInvalidCredentials):
		status = "invalid"
		lastCheckResult = "invalid_credentials"
		lastError = "invalid credentials"
		message = "invalid credentials"
		code := "invalid_credentials"
		errorCode = &code

	case errors.Is(err, ErrNetworkError):
		status = "invalid"
		lastCheckResult = "network_error"
		lastError = "network error"
		message = "network error"
		code := "network_error"
		errorCode = &code

	default:
		status = "invalid"
		lastCheckResult = "provider_error"
		lastError = err.Error()
		message = "provider error"
		code := "provider_error"
		errorCode = &code
	}

	_, updateErr := s.queries.UpdateOzonConnectionCheckResult(ctx, dbgen.UpdateOzonConnectionCheckResultParams{
		SellerAccountID: sellerAccountID,
		Status:          status,
		LastCheckAt: pgtype.Timestamptz{
			Time:  checkedAt,
			Valid: true,
		},
		LastCheckResult: pgtype.Text{
			String: lastCheckResult,
			Valid:  true,
		},
		LastError: pgtype.Text{
			String: lastError,
			Valid:  lastError != "",
		},
	})
	if updateErr != nil {
		return CheckConnectionResult{}, fmt.Errorf("update ozon connection check result: %w", updateErr)
	}

	return CheckConnectionResult{
		Status:    status,
		CheckedAt: checkedAt,
		Message:   message,
		ErrorCode: errorCode,
	}, nil
}

func (s *Service) CheckPerformanceConnection(ctx context.Context, sellerAccountID int64) (CheckConnectionResult, error) {
	_, err := s.GetBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return CheckConnectionResult{}, err
	}

	bearerToken, err := s.GetDecryptedPerformanceBearerToken(ctx, sellerAccountID)
	checkedAt := time.Now()

	if err != nil {
		if errors.Is(err, ErrPerformanceTokenNotConfigured) {
			return CheckConnectionResult{
				Status:    "not_configured",
				CheckedAt: checkedAt,
				Message:   "performance API bearer token is not set",
			}, nil
		}
		return CheckConnectionResult{}, err
	}

	var (
		status          string
		lastCheckResult string
		lastError       string
		message         string
		errorCode       *string
	)

	_, probeErr := s.performance.ListCampaigns(
		ctx,
		"",
		bearerToken,
		ozonperf.ListCampaignsRequest{Page: 1, PageSize: 1},
	)
	switch {
	case probeErr == nil:
		status = "valid"
		lastCheckResult = "valid"
		lastError = ""
		message = "performance connection is valid"

	case errors.Is(probeErr, ozonperf.ErrInvalidCredentials):
		status = "invalid"
		lastCheckResult = "invalid_credentials"
		lastError = "invalid performance token"
		message = "invalid performance token"
		code := "invalid_credentials"
		errorCode = &code

	case errors.Is(probeErr, ozonperf.ErrNetworkError):
		status = "invalid"
		lastCheckResult = "network_error"
		lastError = "network error"
		message = "network error"
		code := "network_error"
		errorCode = &code

	default:
		var providerErr *ozonperf.ProviderError
		if errors.As(probeErr, &providerErr) {
			status = "invalid"
			lastCheckResult = "provider_error"
			lastError = probeErr.Error()
			message = "performance provider error"
			code := "provider_error"
			errorCode = &code
			break
		}
		status = "invalid"
		lastCheckResult = "provider_error"
		lastError = probeErr.Error()
		message = "performance provider error"
		code := "provider_error"
		errorCode = &code
	}

	_, updateErr := s.queries.UpdateOzonPerformanceCheckResult(ctx, dbgen.UpdateOzonPerformanceCheckResultParams{
		SellerAccountID:   sellerAccountID,
		PerformanceStatus: status,
		PerformanceLastCheckAt: pgtype.Timestamptz{
			Time:  checkedAt,
			Valid: true,
		},
		PerformanceLastCheckResult: pgtype.Text{
			String: lastCheckResult,
			Valid:  true,
		},
		PerformanceLastError: pgtype.Text{
			String: lastError,
			Valid:  lastError != "",
		},
	})
	if updateErr != nil {
		return CheckConnectionResult{}, fmt.Errorf("update ozon performance check result: %w", updateErr)
	}

	return CheckConnectionResult{
		Status:    status,
		CheckedAt: checkedAt,
		Message:   message,
		ErrorCode: errorCode,
	}, nil
}

func (s *Service) MaskedClientID(connection dbgen.OzonConnection) string {
	decrypted, err := s.secretCodec.Decrypt(connection.ClientIDEncrypted)
	if err != nil {
		return ""
	}

	return MaskClientID(decrypted)
}
