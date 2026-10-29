package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnvFiles loads repository-local dotenv files for non-production environments.
// Existing process environment variables are never overwritten.
// Search order (later files fill unset keys only): .env, .env.development.local, .env.local.
func LoadEnvFiles() error {
	return loadEnvFiles(false)
}

// LoadEnvFilesOverride loads repository-local dotenv files and allows file values to
// replace existing process environment variables. Skipped in production-like APP_ENV.
func LoadEnvFilesOverride() error {
	return loadEnvFiles(true)
}

func loadEnvFiles(override bool) error {
	if shouldSkipEnvFileLoad() {
		return nil
	}
	root, err := FindRepositoryRootFromWD()
	if err != nil {
		return nil
	}
	candidates := []string{
		filepath.Join(root, ".env"),
		filepath.Join(root, ".env.development.local"),
		filepath.Join(root, ".env.local"),
	}
	for _, path := range candidates {
		if err := loadEnvFile(path, override); err != nil {
			return err
		}
	}
	return nil
}

func shouldSkipEnvFileLoad() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	switch env {
	case "production", "staging", "prod":
		return true
	default:
		return false
	}
}

// FindRepositoryRootFromWD walks up from the current working directory until it
// finds a directory that contains both .git and docker-compose.yml.
func FindRepositoryRootFromWD() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if pathExists(filepath.Join(dir, ".git")) && pathExists(filepath.Join(dir, "docker-compose.yml")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repository root not found from %s", wd)
		}
		dir = parent
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func loadEnvFile(path string, override bool) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open env file %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key, value, ok := ParseEnvLine(scanner.Text())
		if !ok {
			continue
		}
		if !override && strings.TrimSpace(os.Getenv(key)) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read env file %s: %w", path, err)
	}
	return nil
}

// ParseEnvLine parses a single dotenv line. Comments and blank lines return ok=false.
func ParseEnvLine(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	eq := strings.Index(line, "=")
	if eq <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:eq])
	if key == "" {
		return "", "", false
	}
	value = strings.TrimSpace(line[eq+1:])
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			value = value[1 : len(value)-1]
		}
	}
	return key, value, true
}
