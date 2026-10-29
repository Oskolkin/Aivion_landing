package factualapi

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFactualAPIHasNoPersistenceOrProviderImports(t *testing.T) {
	entries, e := os.ReadDir(".")
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		f, e := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if e != nil {
			t.Fatal(e)
		}
		for _, im := range f.Imports {
			p := strings.Trim(im.Path.Value, "\"")
			if strings.Contains(p, "pgx") || strings.Contains(p, "dbgen") || strings.Contains(p, "integrations/ozon") {
				t.Fatalf("%s imports forbidden persistence/provider package %s", entry.Name(), p)
			}
		}
	}
}
