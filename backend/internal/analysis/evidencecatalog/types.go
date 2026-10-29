package evidencecatalog

type EntityKind string

const (
	KindAccount  EntityKind = "ACCOUNT"
	KindCategory EntityKind = "CATEGORY"
	KindSKU      EntityKind = "SKU"
	KindCampaign EntityKind = "CAMPAIGN"
)

type CapabilityMatch string

const (
	MatchALL CapabilityMatch = "ALL"
	MatchANY CapabilityMatch = "ANY"
)

type Type struct {
	Key                   string
	Domain                string
	Description           string
	AllowedSubjectKinds   []EntityKind
	AllowedRelatedKinds   []EntityKind
	RequiredCapabilities  []string
	CapabilityMatch       CapabilityMatch
	TemporalProfileKey    string
	EnabledForDiscoveryV1 bool
	DisabledReason        string
}

func (t Type) Version() string { return TypeVersionV1 }
