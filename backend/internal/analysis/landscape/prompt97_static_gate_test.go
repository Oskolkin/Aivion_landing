package landscape

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrompt97NoParseRatOrZeroOrExactOrZero(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	internalRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	forbidden := []string{"parseRatOrZero", "exactOrZero"}
	err := filepath.Walk(internalRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "vendor" || strings.HasPrefix(base, ".tmp") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		src := string(body)
		for _, tok := range forbidden {
			if strings.Contains(src, tok) {
				t.Errorf("%s contains forbidden semantic helper %s", path, tok)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPrompt97CategorySearchNoFloat64Quantile(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "factual", "category_v4.go")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, tok := range []string{"float64", "sort.Float64s", "SetFloat64"} {
		if strings.Contains(src, tok) {
			t.Errorf("category_v4.go still uses %s in quantile path", tok)
		}
	}
}
