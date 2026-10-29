package discovery

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidencecatalog"

type CapabilityMatch = evidencecatalog.CapabilityMatch

const (
	MatchALL = evidencecatalog.MatchALL
	MatchANY = evidencecatalog.MatchANY
)

type EvidenceType = evidencecatalog.Type

func CatalogV1() []EvidenceType { return evidencecatalog.Catalog() }

func CatalogByKey() map[string]EvidenceType { return evidencecatalog.ByKey() }

func ExpectedCatalogKeysV1() []string { return evidencecatalog.ExpectedKeys() }
