package config

import (
	"fmt"
	"os"
	"strings"
)

// LoadSecretFromFileOrEnv loads a secret preferring a file path env var.
//
// Rules:
//  1. If fileEnv is set to a non-empty path, read that file, trim only trailing CR/LF,
//     and require a non-empty value. The secret value is never included in errors.
//  2. Otherwise fall back to valueEnv (local/dev compatibility).
func LoadSecretFromFileOrEnv(fileEnv, valueEnv string) (string, error) {
	filePath := strings.TrimSpace(os.Getenv(fileEnv))
	if filePath == "" {
		return os.Getenv(valueEnv), nil
	}

	raw, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s: secret file does not exist", fileEnv)
		}
		return "", fmt.Errorf("%s: cannot read secret file", fileEnv)
	}

	value := trimTrailingNewlines(string(raw))
	if value == "" {
		return "", fmt.Errorf("%s: secret file is empty", fileEnv)
	}
	return value, nil
}

func trimTrailingNewlines(s string) string {
	return strings.TrimRight(s, "\r\n")
}

// OpenAIAPIKeyConfigured reports whether an OpenAI key is configured via env or file path.
// It does not read or return the secret value.
func OpenAIAPIKeyConfigured() bool {
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY_FILE")) != "" {
		return true
	}
	return strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != ""
}
