package devseed

import (
	"fmt"
	"os"
	"strings"
)

func AssertDevAIPipelineAllowed() error {
	if strings.TrimSpace(os.Getenv("ALLOW_DEV_AI_PIPELINE")) != "1" {
		return fmt.Errorf("AI pipeline devtool is disabled. Set ALLOW_DEV_AI_PIPELINE=1 and use non-production environment")
	}
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	}
	switch env {
	case "production", "prod":
		return fmt.Errorf("AI pipeline devtool is disabled in production environment")
	}
	return nil
}
