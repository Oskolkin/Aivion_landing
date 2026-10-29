package chatv2

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatV2HasNoForbiddenImports(t *testing.T) {
	forbidden := []string{
		"tool_data_repository", "dashboardhome", "unifiedanalysis", "internal/actions",
		"integrations/ozon", "internal/alerts", "internal/recommendations", "internal/chatexport",
		"internal/dbgen",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := parser.ParseFile(fset, filepath.Join(".", e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range src.Imports {
			path := strings.Trim(im.Path.Value, `"`)
			if path == "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chat" || strings.Contains(path, "/internal/chat/") {
				t.Fatalf("%s imports forbidden %s", e.Name(), path)
			}
			for _, f := range forbidden {
				if strings.Contains(path, f) {
					t.Fatalf("%s imports forbidden %s", e.Name(), path)
				}
			}
		}
	}
}

func TestChatV2SourceHasNoLegacySQL(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	needles := []string{"daily_sku_metrics", "daily_account_metrics", "action_items", "execute_sql", "FROM alerts", "FROM recommendations"}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		body := string(b)
		for _, n := range needles {
			if strings.Contains(body, n) {
				t.Fatalf("%s contains legacy SQL/table %s", e.Name(), n)
			}
		}
	}
}
