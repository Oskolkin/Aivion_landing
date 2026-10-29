package landscape_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Architecture gate: Landscape packages may only depend on Factual API (+ shared analysis helpers),
// never canonical DB repos, dbgen, or Ozon/provider clients.
func TestLandscapeImportBoundary(t *testing.T) {
	root := "."
	forbidden := []string{
		"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen",
		"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon",
		"github.com/jackc/pgx",
		"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection",
		"github.com/sashabaranov/go-openai",
		"openai",
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				if strings.Contains(impPath, bad) {
					t.Errorf("%s imports forbidden %s", path, bad)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
