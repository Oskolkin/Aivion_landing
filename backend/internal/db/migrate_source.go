package db

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

// MigrationSourceURL returns a golang-migrate compatible file:// URL for the given migrations directory.
func MigrationSourceURL(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve migrations path: %w", err)
	}

	normalized := filepath.ToSlash(abs)
	if runtime.GOOS == "windows" {
		vol := filepath.VolumeName(abs)
		if vol != "" {
			rest := strings.TrimPrefix(normalized, vol)
			u := url.URL{
				Scheme: "file",
				Host:   vol,
				Path:   rest,
			}
			return u.String(), nil
		}
	}

	u := url.URL{
		Scheme: "file",
		Path:   normalized,
	}
	return u.String(), nil
}
