package dashboard

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDashboardPackageHasNoForbiddenImports(t *testing.T) {
	forbidden := []string{
		"dashboardhome", "unifiedanalysis", "internal/actions", "integrations/ozon", "openai", "factualapi", "factual/",
	}
	root := "."
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := parser.ParseFile(fset, filepath.Join(root, e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range src.Imports {
			path := strings.Trim(im.Path.Value, `"`)
			for _, f := range forbidden {
				if strings.Contains(path, f) {
					t.Fatalf("%s imports forbidden %s", e.Name(), path)
				}
			}
		}
	}
}
