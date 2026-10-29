package dashboardhome

import "strings"

// recordNarrativeApplyRepair appends a seller-facing apply-path repair entry for debug/audit.
func recordNarrativeApplyRepair(resp *Response, entry string) {
	if resp == nil {
		return
	}
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return
	}
	resp.narrativeApplyRepairs = append(resp.narrativeApplyRepairs, entry)
}

func narrativeApplyRepairs(resp *Response) []string {
	if resp == nil || len(resp.narrativeApplyRepairs) == 0 {
		return nil
	}
	out := make([]string, len(resp.narrativeApplyRepairs))
	copy(out, resp.narrativeApplyRepairs)
	return out
}

func sourceWithApplyRepairs(base string, resp *Response, prefixes ...string) string {
	if base == "" {
		base = "ai"
	}
	if resp == nil || len(prefixes) == 0 {
		return base
	}
	for _, entry := range resp.narrativeApplyRepairs {
		for _, prefix := range prefixes {
			if strings.HasPrefix(entry, prefix) {
				return "ai_repaired_technical"
			}
		}
	}
	return base
}
