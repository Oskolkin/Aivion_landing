package landscape

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// SemanticHashInput returns a LandscapeSet with stable entity ordering for hashing.
func SemanticHashInput(set LandscapeSet) LandscapeSet {
	out := set
	out.DomainWindows = append([]DomainWindow(nil), set.DomainWindows...)
	out.GlobalCapabilities = append([]CapabilityFlag(nil), set.GlobalCapabilities...)
	out.GlobalLimitations = append([]Limitation(nil), set.GlobalLimitations...)
	out.Categories = append([]CategoryLandscape(nil), set.Categories...)
	out.SKUs = append([]SKULandscape(nil), set.SKUs...)
	out.Campaigns = append([]CampaignLandscape(nil), set.Campaigns...)
	SortSet(&out)
	return out
}

// HashSet returns the canonical SHA-256 hex digest of the semantic LandscapeSet.
func HashSet(set LandscapeSet) string {
	return canonicaljson.Hash(SemanticHashInput(set))
}

// MarshalAndHash canonicalizes the semantic set and returns body + hash.
// Body is the full set (including provenance fields); hash uses SemanticHashInput.
func MarshalAndHash(set LandscapeSet) ([]byte, string, error) {
	sorted := SemanticHashInput(set)
	body, err := canonicaljson.Marshal(sorted)
	if err != nil {
		return nil, "", err
	}
	return body, canonicaljson.Hash(sorted), nil
}
