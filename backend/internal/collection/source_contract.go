package collection

import (
	"crypto/sha256"
	"encoding/hex"
)

// SourceContract identifies an immutable provider-adapter semantic contract.
// SemanticMappingHash is derived only from the checked-in semantic definition;
// time, build and git metadata are deliberately excluded.
type SourceContract struct {
	CapabilityKey          string
	ContractKey            string
	ContractVersion        string
	EndpointFamily         string
	SemanticMappingVersion string
	SemanticMappingHash    string
}

func NewSourceContract(capability, key, contractVersion, endpointFamily, mappingVersion, semanticDefinition string) SourceContract {
	sum := sha256.Sum256([]byte(semanticDefinition))
	return SourceContract{
		CapabilityKey:          capability,
		ContractKey:            key,
		ContractVersion:        contractVersion,
		EndpointFamily:         endpointFamily,
		SemanticMappingVersion: mappingVersion,
		SemanticMappingHash:    hex.EncodeToString(sum[:]),
	}
}
