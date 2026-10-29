package ozon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrCanonicalOAuthNotConnected = errors.New("canonical Ozon private OAuth connection is not configured")

type OAuthTokens struct {
	AccessToken  string    `json:"-"`
	RefreshToken string    `json:"-"`
	ExpiresAt    time.Time `json:"-"`
	Scopes       []string  `json:"-"`
	TokenType    string    `json:"-"`
}

type OAuthTokenClient interface {
	ExchangeAuthorizationCode(context.Context, string, string) (OAuthTokens, error)
	Refresh(context.Context, string) (OAuthTokens, error)
}

type privateOAuthTokenClient struct {
	httpClient        *http.Client
	baseURL, clientID string
	clientSecret      string
}

func newPrivateOAuthTokenClientFromEnv() OAuthTokenClient {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("OZON_OAUTH_TOKEN_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "https://xapi.ozon.ru"
	}
	return &privateOAuthTokenClient{
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		baseURL:      baseURL,
		clientID:     strings.TrimSpace(os.Getenv("OZON_OAUTH_CLIENT_ID")),
		clientSecret: strings.TrimSpace(os.Getenv("OZON_OAUTH_CLIENT_SECRET")),
	}
}

func (c *privateOAuthTokenClient) ExchangeAuthorizationCode(ctx context.Context, code, redirectURI string) (OAuthTokens, error) {
	return c.request(ctx, map[string]string{"grant_type": "authorization_code", "code": code, "redirect_uri": redirectURI})
}

func (c *privateOAuthTokenClient) Refresh(ctx context.Context, refreshToken string) (OAuthTokens, error) {
	return c.request(ctx, map[string]string{"grant_type": "refresh_token", "refresh_token": refreshToken})
}

func (c *privateOAuthTokenClient) request(ctx context.Context, values map[string]string) (OAuthTokens, error) {
	if c.clientID == "" || c.clientSecret == "" {
		return OAuthTokens{}, fmt.Errorf("private OAuth application is not configured")
	}
	values["client_id"] = c.clientID
	values["client_secret"] = c.clientSecret
	body, err := json.Marshal(values)
	if err != nil {
		return OAuthTokens{}, fmt.Errorf("marshal OAuth token request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/oauth/token", bytes.NewReader(body))
	if err != nil {
		return OAuthTokens{}, fmt.Errorf("build OAuth token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(req)
	if err != nil {
		return OAuthTokens{}, fmt.Errorf("OAuth token request failed")
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return OAuthTokens{}, fmt.Errorf("read OAuth token response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return OAuthTokens{}, fmt.Errorf("OAuth token endpoint returned status %d", response.StatusCode)
	}
	var decoded struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return OAuthTokens{}, fmt.Errorf("decode OAuth token response: %w", err)
	}
	if strings.TrimSpace(decoded.AccessToken) == "" {
		return OAuthTokens{}, fmt.Errorf("OAuth token response missing access_token")
	}
	expiresIn := decoded.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 30 * 60
	}
	return OAuthTokens{
		AccessToken: strings.TrimSpace(decoded.AccessToken), RefreshToken: strings.TrimSpace(decoded.RefreshToken),
		ExpiresAt: time.Now().UTC().Add(time.Duration(expiresIn) * time.Second),
		Scopes:    strings.Fields(decoded.Scope), TokenType: strings.TrimSpace(decoded.TokenType),
	}, nil
}

func (s *Service) CompletePrivateOAuth(ctx context.Context, sellerID int64, code, redirectURI string) error {
	tokens, err := s.oauthTokens.ExchangeAuthorizationCode(ctx, code, redirectURI)
	if err != nil {
		return err
	}
	if tokens.RefreshToken == "" {
		return fmt.Errorf("OAuth token response missing refresh_token")
	}
	accessEncrypted, refreshEncrypted, scopes, err := s.encryptOAuthTokens(tokens)
	if err != nil {
		return err
	}
	emptyLegacy, err := s.secretCodec.Encrypt("")
	if err != nil {
		return err
	}
	_, err = s.queries.UpsertOzonOAuthConnection(ctx, dbgen.UpsertOzonOAuthConnectionParams{
		SellerAccountID: sellerID, ClientIDEncrypted: emptyLegacy,
		OauthAccessTokenEncrypted:  pgtype.Text{String: accessEncrypted, Valid: true},
		OauthRefreshTokenEncrypted: pgtype.Text{String: refreshEncrypted, Valid: true},
		OauthAccessTokenExpiresAt:  pgtype.Timestamptz{Time: tokens.ExpiresAt, Valid: true},
		Column6:                    scopes, OauthTokenType: pgtype.Text{String: normalizedTokenType(tokens.TokenType), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("persist private OAuth connection: %w", err)
	}
	return nil
}

func (s *Service) CanonicalAccessToken(ctx context.Context, sellerID int64) (string, error) {
	connection, err := s.GetBySellerAccountID(ctx, sellerID)
	if err != nil {
		return "", err
	}
	if connection.AuthMode != "PRIVATE_OAUTH" || connection.OauthConnectionState == "DISCONNECTED" {
		return "", ErrCanonicalOAuthNotConnected
	}
	if token, ok := s.validOAuthAccessToken(connection); ok {
		return token, nil
	}
	var refreshedToken string
	var refreshErr error
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.queries.WithTx(tx)
		locked, lockErr := q.GetOzonConnectionForOAuthRefresh(ctx, sellerID)
		if lockErr != nil {
			return lockErr
		}
		if locked.AuthMode != "PRIVATE_OAUTH" {
			refreshErr = ErrCanonicalOAuthNotConnected
			return nil
		}
		if token, ok := s.validOAuthAccessToken(locked); ok {
			refreshedToken = token
			return nil
		}
		if !locked.OauthRefreshTokenEncrypted.Valid {
			refreshErr = ErrCanonicalOAuthNotConnected
			return nil
		}
		refreshToken, decryptErr := s.secretCodec.Decrypt(locked.OauthRefreshTokenEncrypted.String)
		if decryptErr != nil {
			return decryptErr
		}
		tokens, tokenErr := s.oauthTokens.Refresh(ctx, refreshToken)
		if tokenErr != nil {
			refreshErr = tokenErr
			_, recordErr := q.RecordOzonOAuthRefreshError(ctx, dbgen.RecordOzonOAuthRefreshErrorParams{SellerAccountID: sellerID, OauthLastRefreshError: pgtype.Text{String: safeRefreshError(tokenErr), Valid: true}})
			return recordErr
		}
		if tokens.RefreshToken == "" {
			tokens.RefreshToken = refreshToken
		}
		accessEncrypted, refreshEncrypted, scopes, encryptErr := s.encryptOAuthTokens(tokens)
		if encryptErr != nil {
			return encryptErr
		}
		_, updateErr := q.UpdateOzonOAuthTokens(ctx, dbgen.UpdateOzonOAuthTokensParams{
			SellerAccountID:            sellerID,
			OauthAccessTokenEncrypted:  pgtype.Text{String: accessEncrypted, Valid: true},
			OauthRefreshTokenEncrypted: pgtype.Text{String: refreshEncrypted, Valid: true},
			OauthAccessTokenExpiresAt:  pgtype.Timestamptz{Time: tokens.ExpiresAt, Valid: true},
			Column5:                    scopes, OauthTokenType: pgtype.Text{String: normalizedTokenType(tokens.TokenType), Valid: true},
		})
		if updateErr == nil {
			refreshedToken = tokens.AccessToken
		}
		return updateErr
	})
	if err != nil {
		return "", fmt.Errorf("refresh private OAuth token: %w", err)
	}
	if refreshErr != nil {
		return "", fmt.Errorf("refresh private OAuth token: %w", refreshErr)
	}
	if refreshedToken == "" {
		return "", ErrCanonicalOAuthNotConnected
	}
	return refreshedToken, nil
}

func (s *Service) validOAuthAccessToken(connection dbgen.OzonConnection) (string, bool) {
	if !connection.OauthAccessTokenEncrypted.Valid || !connection.OauthAccessTokenExpiresAt.Valid || time.Until(connection.OauthAccessTokenExpiresAt.Time) <= 2*time.Minute {
		return "", false
	}
	token, err := s.secretCodec.Decrypt(connection.OauthAccessTokenEncrypted.String)
	return strings.TrimSpace(token), err == nil && strings.TrimSpace(token) != ""
}

func (s *Service) encryptOAuthTokens(tokens OAuthTokens) (string, string, []byte, error) {
	access, err := s.secretCodec.Encrypt(tokens.AccessToken)
	if err != nil {
		return "", "", nil, err
	}
	refresh, err := s.secretCodec.Encrypt(tokens.RefreshToken)
	if err != nil {
		return "", "", nil, err
	}
	scopes, err := json.Marshal(tokens.Scopes)
	return access, refresh, scopes, err
}

func normalizedTokenType(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Bearer"
	}
	return strings.TrimSpace(value)
}

func safeRefreshError(err error) string {
	if err == nil {
		return ""
	}
	return "oauth_refresh_failed"
}
