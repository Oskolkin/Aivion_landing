package dashboardhome

import (
	"os"
	"strings"
)

func dashboardAIOnlyEnabled() bool {
	if strings.TrimSpace(os.Getenv("DASHBOARD_LEGACY_MODE")) == "1" {
		return false
	}
	return true
}

type DashboardModeInfo struct {
	Mode          string
	LegacyEnabled bool
	Reason        string
	PromptVersion string
}

func CurrentDashboardModeInfo() DashboardModeInfo {
	legacy := strings.TrimSpace(os.Getenv("DASHBOARD_LEGACY_MODE")) == "1"
	mode := "ai_only_default"
	reason := ""
	if legacy {
		mode = "legacy_explicit"
		reason = "DASHBOARD_LEGACY_MODE=1"
	}
	return DashboardModeInfo{
		Mode:          mode,
		LegacyEnabled: legacy,
		Reason:        reason,
		PromptVersion: DashboardNarrativePromptVersion,
	}
}
