//go:build integration

package ozon

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rotatingOAuthClient struct {
	mu            sync.Mutex
	exchange      OAuthTokens
	refresh       OAuthTokens
	refreshCalls  int
	refreshInputs []string
}

func (f *rotatingOAuthClient) ExchangeAuthorizationCode(context.Context, string, string) (OAuthTokens, error) {
	return f.exchange, nil
}

func (f *rotatingOAuthClient) Refresh(_ context.Context, token string) (OAuthTokens, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshCalls++
	f.refreshInputs = append(f.refreshInputs, token)
	return f.refresh, nil
}

func TestPrivateOAuthPersistsEncryptedReusesAndSerializesRefresh(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run Ozon OAuth integration tests")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("integration database unavailable: %v", err)
	}
	var userID, sellerID int64
	email := fmt.Sprintf("oauth_refresh_%d@test.local", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status) VALUES($1,'integration','active') RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID) })
	if err := pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status) VALUES($1,'oauth-refresh','active') RETURNING id`, userID).Scan(&sellerID); err != nil {
		t.Fatal(err)
	}

	service, err := NewService(pool, "0123456789abcdef0123456789abcdef", "", 90)
	if err != nil {
		t.Fatal(err)
	}
	fake := &rotatingOAuthClient{
		exchange: OAuthTokens{AccessToken: "expiring-access", RefreshToken: "refresh-one", ExpiresAt: time.Now().UTC().Add(time.Minute), Scopes: []string{"seller.read"}, TokenType: "Bearer"},
		refresh:  OAuthTokens{AccessToken: "fresh-access", RefreshToken: "refresh-two", ExpiresAt: time.Now().UTC().Add(time.Hour), Scopes: []string{"seller.read", "performance.read"}, TokenType: "Bearer"},
	}
	service.oauthTokens = fake
	if err := service.CompletePrivateOAuth(ctx, sellerID, "authorization-code", "https://example.test/callback"); err != nil {
		t.Fatal(err)
	}
	var encryptedAccess, encryptedRefresh, mode string
	if err := pool.QueryRow(ctx, `SELECT oauth_access_token_encrypted,oauth_refresh_token_encrypted,auth_mode FROM ozon_connections WHERE seller_account_id=$1`, sellerID).Scan(&encryptedAccess, &encryptedRefresh, &mode); err != nil {
		t.Fatal(err)
	}
	if encryptedAccess == "expiring-access" || encryptedRefresh == "refresh-one" || mode != "PRIVATE_OAUTH" {
		t.Fatalf("OAuth persistence is not encrypted/mode-safe: access=%q refresh=%q mode=%q", encryptedAccess, encryptedRefresh, mode)
	}

	const workers = 8
	start := make(chan struct{})
	results := make(chan string, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			token, tokenErr := service.CanonicalAccessToken(ctx, sellerID)
			results <- token
			errs <- tokenErr
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for tokenErr := range errs {
		if tokenErr != nil {
			t.Fatal(tokenErr)
		}
	}
	for token := range results {
		if token != "fresh-access" {
			t.Fatalf("token=%q", token)
		}
	}
	if fake.refreshCalls != 1 || len(fake.refreshInputs) != 1 || fake.refreshInputs[0] != "refresh-one" {
		t.Fatalf("refresh calls=%d inputs=%v", fake.refreshCalls, fake.refreshInputs)
	}
	if token, err := service.CanonicalAccessToken(ctx, sellerID); err != nil || token != "fresh-access" || fake.refreshCalls != 1 {
		t.Fatalf("valid access token was not reused: token=%q calls=%d err=%v", token, fake.refreshCalls, err)
	}
	state, err := service.DisconnectUserConnection(ctx, sellerID, disconnectConfirmationPhrase)
	if err != nil || state.Status != "disconnected_retention" {
		t.Fatalf("disconnect state=%+v err=%v", state, err)
	}
	var sellerCount, tokenCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM seller_accounts WHERE id=$1`, sellerID).Scan(&sellerCount); err != nil || sellerCount != 1 {
		t.Fatalf("disconnect deleted seller root: count=%d err=%v", sellerCount, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ozon_connections WHERE seller_account_id=$1 AND oauth_access_token_encrypted IS NULL AND oauth_refresh_token_encrypted IS NULL`, sellerID).Scan(&tokenCount); err != nil || tokenCount != 1 {
		t.Fatalf("disconnect did not clear OAuth grants: count=%d err=%v", tokenCount, err)
	}
}
