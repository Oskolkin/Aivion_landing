package devseed

import (
	"fmt"
	"os"
	"strings"
)

// AssertDevActionsRerunAllowed blocks real actions rerun outside local/test environments.
func AssertDevActionsRerunAllowed() error {
	if strings.TrimSpace(os.Getenv("ALLOW_DEV_ACTIONS_RERUN")) != "1" {
		return fmt.Errorf("Actions rerun is disabled. Set ALLOW_DEV_ACTIONS_RERUN=1 and use non-production environment")
	}
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	}
	switch env {
	case "production", "prod":
		return fmt.Errorf("Actions rerun is disabled in production environment")
	}
	return nil
}
