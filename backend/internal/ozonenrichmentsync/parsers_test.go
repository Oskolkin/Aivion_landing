package ozonenrichmentsync

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func TestDedupeMethods(t *testing.T) {
	t.Parallel()
	roles := ozon.RolesResponse{
		ExpiresAt: "2026-02-18T09:54:23.296Z",
		Roles: []struct {
			Name    string   `json:"name"`
			Methods []string `json:"methods"`
		}{
			{Name: "Admin", Methods: []string{"/v1/actions", "/v3/product/info/list"}},
			{Name: "Viewer", Methods: []string{"/v3/product/info/list", "/v1/analytics/stocks"}},
		},
	}
	methods := dedupeMethods(roles)
	if len(methods) != 3 {
		t.Fatalf("expected 3 unique methods, got %d: %v", len(methods), methods)
	}
}

func TestParseDecimalStringBadValueReturnsNull(t *testing.T) {
	t.Parallel()
	n := parseDecimalString("not-a-price")
	if n.Valid {
		t.Fatal("expected invalid numeric for bad price")
	}
}

func TestParseDecimalStringParsesPrice(t *testing.T) {
	t.Parallel()
	n := parseDecimalString("1299.50")
	if !n.Valid {
		t.Fatal("expected valid numeric")
	}
}

func TestChunkProductIDsBy1000(t *testing.T) {
	t.Parallel()
	ids := make([]int64, 2500)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	chunks := chunkProductIDs(ids, nil, productInfoChunkSize)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0].ProductID) != 1000 || len(chunks[2].ProductID) != 500 {
		t.Fatalf("unexpected chunk sizes: %d %d", len(chunks[0].ProductID), len(chunks[2].ProductID))
	}
}

func TestClassifySellerAPIStatusUnauthorized(t *testing.T) {
	t.Parallel()
	status, code, _ := classifySellerAPIStatus(ozon.ErrInvalidCredentials)
	if status != statusUnauthorized || code != "unauthorized" {
		t.Fatalf("got status=%s code=%s", status, code)
	}
}

func TestSanitizeErrorMessageHidesSecrets(t *testing.T) {
	t.Parallel()
	msg := sanitizeErrorMessage("failed with Api-Key=secret and Bearer token")
	if msg == "failed with Api-Key=secret and Bearer token" {
		t.Fatal("expected sanitized message")
	}
}

func TestFinalizeRunStatusPartial(t *testing.T) {
	t.Parallel()
	if got := finalizeRunStatus(1, 0, true); got != runStatusCompletedPartial {
		t.Fatalf("expected completed_partial, got %s", got)
	}
}

func TestStatsDateRange14Days(t *testing.T) {
	t.Parallel()
	metricDate := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	from, to := statsDateRange(metricDate)
	if to.Format("2006-01-02") != "2026-06-25" {
		t.Fatalf("unexpected to: %s", to)
	}
	if from.Format("2006-01-02") != "2026-06-12" {
		t.Fatalf("unexpected from: %s", from)
	}
}
