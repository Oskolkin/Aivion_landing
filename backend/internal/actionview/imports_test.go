package actionview

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActionViewPackageHasNoForbiddenImports(t *testing.T) {
	forbidden := []string{"dashboardhome", "unifiedanalysis", "internal/actions", "action_items", "openai", "integrations/ozon"}
	fset := token.NewFileSet()
	entries, _ := os.ReadDir(".")
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
			for _, f := range forbidden {
				if strings.Contains(path, f) {
					t.Fatalf("%s imports %s", e.Name(), path)
				}
			}
		}
	}
}
