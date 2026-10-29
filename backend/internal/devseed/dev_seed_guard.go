package devseed

import (
	"fmt"
	"os"
	"strings"
)

// AssertDevSeedAllowed blocks dev-only seed commands outside local/test/staging-demo environments.
func AssertDevSeedAllowed() error {
	if strings.TrimSpace(os.Getenv("ALLOW_DEV_SEED")) != "1" {
		return fmt.Errorf("Dev seed is disabled. Set ALLOW_DEV_SEED=1 and use non-production environment")
	}
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	}
	switch env {
	case "production", "prod":
		return fmt.Errorf("Dev seed is disabled in production environment")
	case "staging":
		if strings.TrimSpace(os.Getenv("STAGING_DEMO_MODE")) != "true" && strings.TrimSpace(os.Getenv("STAGING_DEMO_MODE")) != "1" {
			return fmt.Errorf("Dev seed on staging requires STAGING_DEMO_MODE=true")
		}
	}
	return nil
}
