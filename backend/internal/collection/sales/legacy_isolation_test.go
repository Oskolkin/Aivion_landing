package sales

import (
	"os"
	"strings"
	"testing"
)

func TestCanonicalSalesImplementationDoesNotReferenceLegacyFactSources(t *testing.T) {
	for _, name := range []string{"runner.go", "repository.go"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(body))
		for _, forbidden := range []string{"productanalyticssync", "daily_account_metrics", "daily_sku_metrics", "from orders", "from postings", "from sales "} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s depends on forbidden legacy source %q", name, forbidden)
			}
		}
	}
}
