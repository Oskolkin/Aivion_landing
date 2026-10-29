package discovery

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidencecatalog"

type EvidenceMenuItem = evidencecatalog.MenuItem

func BuildEvidenceMenu(caps map[string]string, kinds map[EntityKind]bool, universeDomains map[string]bool) []EvidenceMenuItem {
	ck := map[evidencecatalog.EntityKind]bool{}
	for k, v := range kinds {
		ck[evidencecatalog.EntityKind(k)] = v
	}
	items := evidencecatalog.BuildMenu(caps, ck, universeDomains, EvidenceMenuPolicyV3)
	out := make([]EvidenceMenuItem, len(items))
	copy(out, items)
	return out
}

func MenuHash(items []EvidenceMenuItem) string {
	return evidencecatalog.MenuHash(items, EvidenceMenuPolicyV3)
}
