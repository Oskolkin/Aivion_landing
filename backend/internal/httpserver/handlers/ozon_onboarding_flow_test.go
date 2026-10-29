package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboard"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	appmw "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/httpserver/middleware"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestShouldEnqueueInitialSyncAfterPut(t *testing.T) {
	t.Parallel()

	result := ozon.PutConnectionResult{InitialSyncStarted: true}
	if shouldEnqueueInitialSyncAfterPut(result, nil) {
		t.Fatal("must not enqueue when orchestration is nil")
	}
	if shouldEnqueueInitialSyncAfterPut(ozon.PutConnectionResult{InitialSyncStarted: false}, nil) {
		t.Fatal("expected false when InitialSyncStarted is false")
	}
}

func TestBuildUserConnectionResponseReadyShape(t *testing.T) {
	resp := buildUserConnectionResponse(ozon.UserConnectionState{
		ConnectionReady: true,
		Status:          "connected",
	})
	if !resp.ConnectionReady || resp.Status != "connected" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestSellerOnboardingGateAndDashboardIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run seller onboarding integration test")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)

	encKey := os.Getenv("ENCRYPTION_KEY")
	if encKey == "" {
		encKey = "0123456789abcdef0123456789abcdef"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("db connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("db ping: %v", err)
	}

	email := fmt.Sprintf("onboarding_%d@test.local", time.Now().UnixNano())
	password := "password123"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	q := dbgen.New(pool)
	user, err := q.CreateUser(ctx, dbgen.CreateUserParams{Email: email, PasswordHash: hash, Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	seller, err := q.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{
		UserID: user.ID,
		Name:   "Onboarding IT",
		Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}

	authSvc := auth.NewService(pool, 24*time.Hour, nil)
	login, err := authSvc.Login(ctx, auth.LoginInput{Email: email, Password: password})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.SessionToken == "" || login.SellerAccount == nil {
		t.Fatal("expected session and seller account after login")
	}

	ozonSvc, err := ozon.NewService(pool, encKey, "https://api-performance.ozon.ru", 90)
	if err != nil {
		t.Fatal(err)
	}

	state, err := ozonSvc.GetUserConnectionState(ctx, seller.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ConnectionReady {
		t.Fatal("expected connection_ready=false before credentials")
	}

	connMW := appmw.RequireOzonConnection(ozonSvc, nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/home", nil)
	req = req.WithContext(auth.WithAuthContext(ctx, user, &seller))
	rr := httptest.NewRecorder()
	connMW(next).ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("dashboard gate before connect: status=%d body=%s", rr.Code, rr.Body.String())
	}

	codec, err := ozon.NewSecretCodec(encKey)
	if err != nil {
		t.Fatal(err)
	}
	clientEnc, _ := codec.Encrypt("test-client")
	apiEnc, _ := codec.Encrypt("test-api")
	perfIDEnc, _ := codec.Encrypt("perf-id")
	perfSecretEnc, _ := codec.Encrypt("perf-secret")
	tokenEnc, _ := codec.Encrypt("perf-token")
	expires := time.Now().Add(time.Hour)

	_, err = q.UpsertOzonConnectionValidated(ctx, dbgen.UpsertOzonConnectionValidatedParams{
		SellerAccountID:                  seller.ID,
		ClientIDEncrypted:                clientEnc,
		ApiKeyEncrypted:                  apiEnc,
		PerformanceClientIDEncrypted:     pgtypeText(perfIDEnc),
		PerformanceClientSecretEncrypted: pgtypeText(perfSecretEnc),
		PerformanceTokenEncrypted:        pgtypeText(tokenEnc),
		PerformanceTokenExpiresAt:        pgtypeTimestamptz(expires),
	})
	if err != nil {
		t.Fatal(err)
	}

	state, err = ozonSvc.GetUserConnectionState(ctx, seller.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !state.ConnectionReady {
		t.Fatalf("expected connection_ready=true after validated upsert, state=%+v", state)
	}

	rr = httptest.NewRecorder()
	connMW(next).ServeHTTP(rr, req.WithContext(auth.WithAuthContext(ctx, user, &seller)))
	if rr.Code != http.StatusOK {
		t.Fatalf("dashboard gate after connect: status=%d body=%s", rr.Code, rr.Body.String())
	}

	dashHandler := NewDashboardHomeHandler(dashboard.NewService(analysispg.New(pool)))
	rr = httptest.NewRecorder()
	dashReq := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/home", nil)
	dashReq = dashReq.WithContext(auth.WithAuthContext(ctx, user, &seller))
	dashHandler.GetHome(rr, dashReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("GetHome status=%d body=%s", rr.Code, rr.Body.String())
	}
	var home map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &home); err != nil {
		t.Fatalf("unmarshal home: %v", err)
	}
}

func pgtypeText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

func pgtypeTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
