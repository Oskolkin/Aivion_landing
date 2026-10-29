package db

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func TestMigrationSourceURL(t *testing.T) {
	dir := t.TempDir()
	got, err := MigrationSourceURL(dir)
	if err != nil {
		t.Fatalf("MigrationSourceURL: %v", err)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	expectedPath := filepath.ToSlash(abs)
	if runtime.GOOS == "windows" {
		vol := filepath.VolumeName(abs)
		want := "file://" + vol + strings.TrimPrefix(expectedPath, vol)
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
		if strings.Contains(got, `\`) {
			t.Fatalf("URL must not contain backslashes: %q", got)
		}
		return
	}

	want := "file://" + expectedPath
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMigrationSourceURL_opensWithGolangMigrate(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	migrationsPath := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "migrations"))

	sourceURL, err := MigrationSourceURL(migrationsPath)
	if err != nil {
		t.Fatalf("MigrationSourceURL: %v", err)
	}

	m, err := migrate.New(sourceURL, "postgres://invalid")
	if err != nil {
		if strings.Contains(err.Error(), "failed to open source") || strings.Contains(err.Error(), "open .") {
			t.Fatalf("migrate source open failed for %q: %v", sourceURL, err)
		}
		// Database URL parse/connection errors are expected.
		return
	}
	if m != nil {
		_, _ = m.Close()
	}
}
