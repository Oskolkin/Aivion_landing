package pack

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// Sensitive-name allowlist guard: frozen pack columns/JSON keys must not look like
// credentials or buyer PII. This is not DLP — the real protection is the field allowlist.
func TestContractFieldsHaveNoSensitiveNames(t *testing.T) {
	forbidden := []string{
		"api_key", "apikey", "password", "secret", "token", "jwt", "cookie",
		"authorization", "refresh_token", "access_token",
		"buyer_email", "buyer_phone", "buyer_address", "buyer_name",
		"recipient", "email", "phone", "password_hash",
	}
	var names []string
	for _, cols := range [][]string{
		contract.AccountSummaryColumnsV1,
		contract.AccountDailyColumnsV1,
		contract.SKUSummaryColumnsV1,
		contract.SKUDailyColumnsV1,
		contract.CampaignSummaryColumnsV1,
		contract.CampaignDailyColumnsV1,
		contract.CampaignSKUColumnsV1,
		contract.ProductEnrichmentColumnsV1,
	} {
		names = append(names, cols...)
	}
	for _, name := range names {
		lower := strings.ToLower(name)
		for _, bad := range forbidden {
			if lower == bad || strings.Contains(lower, bad) {
				// Allow benign substrings that are not PII (e.g. none currently).
				t.Fatalf("sensitive-looking column %q matched %q", name, bad)
			}
		}
	}
}
