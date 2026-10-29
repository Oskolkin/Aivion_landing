package financeregistry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

const Version = "finance_type_registry_v1"

type ProviderType struct {
	ProviderTypeID   string
	Name             string
	Description      string
	CanonicalGroup   string
	CanonicalSubtype string
	EconomicRole     string
	DefaultScope     string
	CrossDomainOwner string
	MappingStatus    string
}

// Normalize is deliberately conservative. Provider dictionary entries without
// an explicitly accepted canonical mapping remain UNMAPPED; they are never
// collapsed into an "OTHER" bucket.
func Normalize(v ProviderType) ProviderType {
	v.ProviderTypeID = strings.TrimSpace(v.ProviderTypeID)
	if v.DefaultScope == "" {
		v.DefaultScope = "UNKNOWN"
	}
	if v.MappingStatus != "MAPPED" && v.MappingStatus != "PENDING_ACCEPTANCE" {
		v.MappingStatus = "UNMAPPED"
		v.CanonicalGroup = ""
		v.CanonicalSubtype = ""
		v.EconomicRole = ""
	}
	return v
}

func DictionaryHash(types []ProviderType) string {
	copyTypes := append([]ProviderType(nil), types...)
	sort.Slice(copyTypes, func(i, j int) bool { return copyTypes[i].ProviderTypeID < copyTypes[j].ProviderTypeID })
	body, _ := json.Marshal(copyTypes)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
