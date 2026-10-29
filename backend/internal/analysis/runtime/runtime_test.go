package runtime

import (
	"os"
	"strings"
	"testing"

	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
)

func TestLeaseFencing(t *testing.T) {
	lease := analysispg.Lease{Token: "new", Version: 4}
	if !FenceMatches("new", 4, lease) || FenceMatches("old", 4, lease) || FenceMatches("new", 3, lease) {
		t.Fatal("stale worker crossed lease fence")
	}
}

func TestFactualTokenHMACSecretRequiresConfiguredKey(t *testing.T) {
	t.Setenv("FACTUAL_TOKEN_HMAC_SECRET", "")
	t.Setenv("FACTUAL_TOKEN_SIGNING_KEY", "")
	if _, err := factualTokenHMACSecret(); err == nil {
		t.Fatal("expected missing secret to fail")
	}
	t.Setenv("FACTUAL_TOKEN_HMAC_SECRET", "short")
	if _, err := factualTokenHMACSecret(); err == nil {
		t.Fatal("expected short secret to fail")
	}
	want := strings.Repeat("k", 32)
	t.Setenv("FACTUAL_TOKEN_HMAC_SECRET", want)
	got, err := factualTokenHMACSecret()
	if err != nil || string(got) != want {
		t.Fatalf("got=%q err=%v", got, err)
	}
	t.Setenv("FACTUAL_TOKEN_HMAC_SECRET", "")
	t.Setenv("FACTUAL_TOKEN_SIGNING_KEY", want)
	got, err = factualTokenHMACSecret()
	if err != nil || string(got) != want {
		t.Fatalf("fallback got=%q err=%v", got, err)
	}
}

func TestNoHardCodedFactualTokenLiteral(t *testing.T) {
	body, err := os.ReadFile("compose.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "analysis-runtime-factual-token-v1!!!!") {
		t.Fatal("hard-coded HMAC secret must not appear in compose.go")
	}
}
