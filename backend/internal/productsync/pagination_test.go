package productsync

import (
	"errors"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func TestEffectiveListLastID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		items  []ozon.ProductItem
		api    string
		expect string
	}{
		{"api wins", []ozon.ProductItem{{ID: 1}}, "42", "42"},
		{"fallback last item", []ozon.ProductItem{{ID: 10}, {ID: 20}}, "", "20"},
		{"empty items empty api", nil, "", ""},
		{"trim api", []ozon.ProductItem{{ID: 1}}, "  7  ", "7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := effectiveListLastID(tc.items, tc.api)
			if got != tc.expect {
				t.Fatalf("effectiveListLastID: want %q got %q", tc.expect, got)
			}
		})
	}
}

func TestAnotherProductsPage(t *testing.T) {
	t.Parallel()
	const limit = 1000
	cases := []struct {
		name     string
		n        int
		reqLast  string
		respLast string
		want     bool
	}{
		{"empty items", 0, "", "1", false},
		{"no cursor", 10, "", "", false},
		{"stalled same id", 1000, "5", "5", false},
		{"partial page done", 400, "", "99", false},
		{"full page more", 1000, "", "100", true},
		{"full page more incremental", 1000, "50", "150", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := anotherProductsPage(tc.n, limit, tc.reqLast, tc.respLast)
			if got != tc.want {
				t.Fatalf("anotherProductsPage: want %v got %v", tc.want, got)
			}
		})
	}
}

func TestAnotherProductsPage_fullBatchWithNextCursor(t *testing.T) {
	t.Parallel()
	if !anotherProductsPage(1000, 1000, "", "last-1000") {
		t.Fatal("expected another page")
	}
}

func TestAnotherProductsPage_zeroLimit(t *testing.T) {
	t.Parallel()
	if anotherProductsPage(5, 0, "", "x") {
		t.Fatal("limit 0 should not request another page")
	}
}

func TestErrMaxPagesExceededIs(t *testing.T) {
	t.Parallel()
	err := errMaxPagesExceeded("abc", 1000, 1_000_000, 999_000)
	if !errors.Is(err, ErrMaxPagesPerRun) {
		t.Fatalf("expected errors.Is ErrMaxPagesPerRun, got %v", err)
	}
}
