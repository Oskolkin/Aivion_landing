package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/go-chi/chi/v5"
)

type fakeDataPackBuilder struct {
	buildFn func(ctx context.Context, sellerID int64, metricDate time.Time) (*aidatapack.SellerAIDataPack, error)
	called  bool
}

func (f *fakeDataPackBuilder) Build(ctx context.Context, sellerID int64, metricDate time.Time) (*aidatapack.SellerAIDataPack, error) {
	f.called = true
	if f.buildFn != nil {
		return f.buildFn(ctx, sellerID, metricDate)
	}
	return minimalTestDataPack(), nil
}

func TestAIDataPackHandler_AdminCanGetPack(t *testing.T) {
	fake := &fakeDataPackBuilder{}
	h := NewAIDataPackHandlerWithBuilder(fake)
	router := chi.NewRouter()
	router.With(auth.AdminMiddleware([]string{"admin@example.com"})).Get("/api/v1/admin/ai/data-pack", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai/data-pack?seller_id=1&metric_date=2026-06-25", nil)
	req = req.WithContext(auth.WithAuthContext(req.Context(), dbgen.User{ID: 99, Email: "admin@example.com"}, nil))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !fake.called {
		t.Fatal("expected build to be called")
	}
	var got aidatapack.SellerAIDataPack
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != aidatapack.SchemaVersion {
		t.Fatalf("schema=%s", got.SchemaVersion)
	}
}

func TestAIDataPackHandler_NonAdminForbidden(t *testing.T) {
	fake := &fakeDataPackBuilder{
		buildFn: func(context.Context, int64, time.Time) (*aidatapack.SellerAIDataPack, error) {
			t.Fatal("build should not be called for non-admin")
			return nil, nil
		},
	}
	h := NewAIDataPackHandlerWithBuilder(fake)
	router := chi.NewRouter()
	router.With(auth.AdminMiddleware([]string{"admin@example.com"})).Get("/api/v1/admin/ai/data-pack", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai/data-pack?seller_id=1&metric_date=2026-06-25", nil)
	req = req.WithContext(auth.WithAuthContext(req.Context(), dbgen.User{ID: 1, Email: "user@example.com"}, nil))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rr.Code)
	}
	if fake.called {
		t.Fatal("build should not run for non-admin route")
	}
}

func TestAIDataPackHandler_MissingSellerID(t *testing.T) {
	h := NewAIDataPackHandlerWithBuilder(&fakeDataPackBuilder{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai/data-pack?metric_date=2026-06-25", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestAIDataPackHandler_MissingMetricDate(t *testing.T) {
	h := NewAIDataPackHandlerWithBuilder(&fakeDataPackBuilder{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai/data-pack?seller_id=1", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestAIDataPackHandler_InvalidMetricDate(t *testing.T) {
	h := NewAIDataPackHandlerWithBuilder(&fakeDataPackBuilder{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai/data-pack?seller_id=1&metric_date=bad", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestAIDataPackHandler_AuditFlag(t *testing.T) {
	h := NewAIDataPackHandlerWithBuilder(&fakeDataPackBuilder{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ai/data-pack?seller_id=1&metric_date=2026-06-25&audit=1", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if _, ok := resp["audit"]; !ok {
		t.Fatal("expected audit field")
	}
	if _, ok := resp["pack"]; !ok {
		t.Fatal("expected pack field")
	}
}

func minimalTestDataPack() *aidatapack.SellerAIDataPack {
	return &aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		SellerContext: aidatapack.SellerContext{SellerAccountID: 1, MetricDate: "2026-06-25", Marketplace: aidatapack.MarketplaceOzon},
		FactRegistry:  map[string]aidatapack.Fact{"account.current_day.revenue": {FactID: "account.current_day.revenue", Unit: "RUB"}},
		Tables: aidatapack.Tables{
			AccountRollup: aidatapack.AccountRollupTable{DataStatus: "available"},
			FinanceRollup: aidatapack.FinanceRollupTable{DataStatus: "not_available"},
		},
	}
}
