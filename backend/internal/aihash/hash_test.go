package aihash_test

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
)

func TestHashDataPackStable(t *testing.T) {
	pack := &aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		SellerContext: aidatapack.SellerContext{SellerAccountID: 1, MetricDate: "2026-06-24"},
		FactRegistry: map[string]aidatapack.Fact{
			"account.current_day.revenue": {FactID: "account.current_day.revenue", FormattedValue: "1000"},
		},
	}
	h1 := aihash.HashDataPack(pack)
	h2 := aihash.HashDataPack(pack)
	if h1 == "" || h2 == "" {
		t.Fatal("expected non-empty hash")
	}
	if h1 != h2 {
		t.Fatalf("expected stable hash, got %q vs %q", h1, h2)
	}
	if !hasPrefix(h1, "sha256:") {
		t.Fatalf("expected sha256 prefix, got %q", h1)
	}
}

func TestHashDataPackChangesWhenFactChanges(t *testing.T) {
	pack1 := &aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		FactRegistry: map[string]aidatapack.Fact{
			"account.current_day.revenue": {FactID: "account.current_day.revenue", FormattedValue: "1000"},
		},
	}
	pack2 := &aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		FactRegistry: map[string]aidatapack.Fact{
			"account.current_day.revenue": {FactID: "account.current_day.revenue", FormattedValue: "2000"},
		},
	}
	if aihash.HashDataPack(pack1) == aihash.HashDataPack(pack2) {
		t.Fatal("expected different hashes for different fact values")
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
