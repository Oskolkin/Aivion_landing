package factualapi

import (
	"fmt"
	"sort"
	"time"
)

type ReadMode string

const (
	ModeLatestStable   ReadMode = "LATEST_STABLE"
	ModeSealedUniverse ReadMode = "SEALED_UNIVERSE"
)

// FactFamilyKey identifies a DomainBinding by domain and fact family.
type FactFamilyKey struct {
	Domain, FactFamily string
}

func (k FactFamilyKey) String() string {
	return k.Domain + "\x00" + k.FactFamily
}

type DomainBinding struct {
	Domain                                     string
	FactFamily                                 string
	DomainRunID                                int64
	FinalizedAt                                *time.Time // collection run finished_at; nil when unknown — never observed/cutoff
	ContractKey, ContractVersion, ContractHash string
	CoverageFrom, CoverageTo                   *time.Time
	WatermarkHash                              string
	DQRunID                                    int64
}

type CapabilityBinding struct {
	CapabilityKey, Scope, FinalState, ReasonCode string
	AssessmentID                                 int64 // 0 = optional / unknown provenance (allowed in SEALED_UNIVERSE)
	AssessedAt                                   time.Time
	// Multidimensional freeze fields (Prompt 8.1). Empty means unset for legacy LATEST_STABLE callers.
	ContractState, AccessState, SourceState, SemanticState      string
	CoverageState, TemporalState, HistoryState, PopulationState string
}

type SealedBinding interface {
	SealedBindingHash() string
	SealedUniverseID() string
}

type ContextSpec struct {
	SellerAccountID int64
	AnalysisDate    time.Time
	CutoffAt        time.Time
	Mode            ReadMode
	Policies        PolicyVersions
	Domains         []DomainBinding
	Capabilities    []CapabilityBinding
	Sealed          SealedBinding
}

type FactualReadContext struct {
	seller               int64
	analysisDate, cutoff time.Time
	mode                 ReadMode
	policies             PolicyVersions
	domains              map[string]DomainBinding // keyed by FactFamilyKey.String()
	capabilities         map[string]CapabilityBinding
	universeID           string
	hash                 string
}

// NewSealedUniverseContext accepts only a binding already validated by the
// universe repository and pins all future reads to that immutable universe.
func NewSealedUniverseContext(spec ContextSpec, sealed SealedBinding) (FactualReadContext, error) {
	spec.Mode, spec.Sealed = ModeSealedUniverse, sealed
	return NewReadContext(spec)
}

func NewReadContext(spec ContextSpec) (FactualReadContext, error) {
	if spec.SellerAccountID <= 0 || spec.CutoffAt.IsZero() || spec.AnalysisDate.IsZero() {
		return FactualReadContext{}, NewError(CodeContextInvalid, "seller, cutoff and analysis date are required")
	}
	if dateOnly(spec.AnalysisDate).After(LastCompletedBusinessDate(spec.CutoffAt)) {
		return FactualReadContext{}, NewError(CodeContextInvalid, "analysis date must be a completed Europe/Moscow business day")
	}
	if spec.Mode != ModeLatestStable && spec.Mode != ModeSealedUniverse {
		return FactualReadContext{}, NewError(CodeContextInvalid, "unsupported read mode")
	}
	if spec.Mode == ModeSealedUniverse && spec.Sealed == nil {
		return FactualReadContext{}, NewError(CodeContextInvalid, "sealed mode requires validated sealed binding")
	}
	if spec.Sealed != nil && (spec.Sealed.SealedUniverseID() == "" || spec.Sealed.SealedBindingHash() == "") {
		return FactualReadContext{}, NewError(CodeContextInvalid, "sealed binding is invalid")
	}
	p := spec.Policies
	if p.Temporal == "" || p.DQ == "" || p.Capability == "" || p.SalesMaturity == "" || p.API == "" || p.StockFreshness == "" || p.PriceFreshness == "" || p.AdvertisingMaturity == "" || p.AttributionAggregation == "" {
		return FactualReadContext{}, NewError(CodeContextInvalid, "all policy versions are required")
	}
	domains := make(map[string]DomainBinding, len(spec.Domains))
	capabilities := make(map[string]CapabilityBinding, len(spec.Capabilities))
	parts := []string{fmt.Sprint(spec.SellerAccountID), day(spec.AnalysisDate), spec.CutoffAt.UTC().Format(time.RFC3339Nano), string(spec.Mode), p.Temporal, p.DQ, p.Capability, p.SalesMaturity, p.API, p.StockFreshness, p.PriceFreshness, p.AdvertisingMaturity, p.AttributionAggregation}
	d := append([]DomainBinding(nil), spec.Domains...)
	sort.Slice(d, func(i, j int) bool {
		if d[i].Domain != d[j].Domain {
			return d[i].Domain < d[j].Domain
		}
		return d[i].FactFamily < d[j].FactFamily
	})
	sealed := spec.Mode == ModeSealedUniverse
	for _, b := range d {
		if b.Domain == "" || b.FactFamily == "" || (b.FinalizedAt != nil && b.FinalizedAt.After(spec.CutoffAt)) {
			return FactualReadContext{}, NewError(CodeContextInvalid, "invalid domain binding")
		}
		if !sealed && b.DomainRunID <= 0 {
			return FactualReadContext{}, NewError(CodeContextInvalid, "invalid domain binding")
		}
		key := FactFamilyKey{Domain: b.Domain, FactFamily: b.FactFamily}.String()
		if _, exists := domains[key]; exists {
			return FactualReadContext{}, NewError(CodeContextInvalid, "duplicate domain+fact_family binding")
		}
		b = cloneDomainBinding(b)
		domains[key] = b
		parts = append(parts, "d", b.Domain, b.FactFamily, optionalID(b.DomainRunID), optionalTime(b.FinalizedAt), b.ContractKey, b.ContractVersion, b.ContractHash, b.WatermarkHash, optionalID(b.DQRunID))
	}
	c := append([]CapabilityBinding(nil), spec.Capabilities...)
	sort.Slice(c, func(i, j int) bool {
		if c[i].CapabilityKey == c[j].CapabilityKey {
			return c[i].Scope < c[j].Scope
		}
		return c[i].CapabilityKey < c[j].CapabilityKey
	})
	for _, b := range c {
		if b.CapabilityKey == "" || b.AssessedAt.After(spec.CutoffAt) {
			return FactualReadContext{}, NewError(CodeContextInvalid, "invalid capability binding")
		}
		if !sealed && b.AssessmentID <= 0 {
			return FactualReadContext{}, NewError(CodeContextInvalid, "invalid capability binding")
		}
		capabilities[b.CapabilityKey+"/"+b.Scope] = b
		parts = append(parts, "c", b.CapabilityKey, b.Scope, b.FinalState, b.ReasonCode, optionalID(b.AssessmentID), b.AssessedAt.UTC().Format(time.RFC3339Nano),
			b.ContractState, b.AccessState, b.SourceState, b.SemanticState, b.CoverageState, b.TemporalState, b.HistoryState, b.PopulationState)
	}
	if spec.Sealed != nil {
		parts = append(parts, "sealed", spec.Sealed.SealedBindingHash())
	}
	universeID := ""
	if spec.Sealed != nil {
		universeID = spec.Sealed.SealedUniverseID()
	}
	return FactualReadContext{seller: spec.SellerAccountID, analysisDate: dateOnly(spec.AnalysisDate), cutoff: spec.CutoffAt.UTC(), mode: spec.Mode, policies: p, domains: domains, capabilities: capabilities, universeID: universeID, hash: StableHash(parts...)}, nil
}

func (c FactualReadContext) SellerAccountID() int64         { return c.seller }
func (c FactualReadContext) AnalysisDate() time.Time        { return c.analysisDate }
func (c FactualReadContext) CutoffAt() time.Time            { return c.cutoff }
func (c FactualReadContext) Mode() ReadMode                 { return c.mode }
func (c FactualReadContext) PolicyVersions() PolicyVersions { return c.policies }
func (c FactualReadContext) ContextHash() string            { return c.hash }
func (c FactualReadContext) UniverseID() string             { return c.universeID }

// DomainNames returns unique domain names in sorted order.
func (c FactualReadContext) DomainNames() []string {
	seen := make(map[string]struct{}, len(c.domains))
	out := make([]string, 0, len(c.domains))
	for _, b := range c.domains {
		if _, ok := seen[b.Domain]; ok {
			continue
		}
		seen[b.Domain] = struct{}{}
		out = append(out, b.Domain)
	}
	sort.Strings(out)
	return out
}

func (c FactualReadContext) CapabilityBindings() []CapabilityBinding {
	out := make([]CapabilityBinding, 0, len(c.capabilities))
	for _, binding := range c.capabilities {
		out = append(out, binding)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CapabilityKey+"/"+out[i].Scope < out[j].CapabilityKey+"/"+out[j].Scope
	})
	return out
}

// Binding returns the DomainBinding for an exact (domain, factFamily) pair.
func (c FactualReadContext) Binding(domain, factFamily string) (DomainBinding, bool) {
	v, ok := c.domains[FactFamilyKey{Domain: domain, FactFamily: factFamily}.String()]
	return cloneDomainBinding(v), ok
}

// HasBinding reports whether an exact (domain, factFamily) binding exists.
func (c FactualReadContext) HasBinding(domain, factFamily string) bool {
	_, ok := c.domains[FactFamilyKey{Domain: domain, FactFamily: factFamily}.String()]
	return ok
}

// BindingsForDomain returns all bindings for domain sorted by FactFamily.
func (c FactualReadContext) BindingsForDomain(domain string) []DomainBinding {
	out := make([]DomainBinding, 0)
	for _, b := range c.domains {
		if b.Domain == domain {
			out = append(out, cloneDomainBinding(b))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FactFamily < out[j].FactFamily })
	return out
}

// AllBindings returns every domain binding in deterministic Domain then FactFamily order.
func (c FactualReadContext) AllBindings() []DomainBinding {
	out := make([]DomainBinding, 0, len(c.domains))
	for _, b := range c.domains {
		out = append(out, cloneDomainBinding(b))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Domain != out[j].Domain {
			return out[i].Domain < out[j].Domain
		}
		return out[i].FactFamily < out[j].FactFamily
	})
	return out
}

// DomainBinding returns the binding when the domain has exactly one FactFamily.
// If zero or multiple families are bound, ok is false.
func (c FactualReadContext) DomainBinding(domain string) (DomainBinding, bool) {
	matches := c.BindingsForDomain(domain)
	if len(matches) != 1 {
		return DomainBinding{}, false
	}
	return matches[0], true
}

// UniqueDomainBinding returns the sole binding for domain, or an error if absent or ambiguous.
func (c FactualReadContext) UniqueDomainBinding(domain string) (DomainBinding, error) {
	matches := c.BindingsForDomain(domain)
	switch len(matches) {
	case 0:
		return DomainBinding{}, NewError(CodeContextInvalid, "domain binding not found")
	case 1:
		return matches[0], nil
	default:
		return DomainBinding{}, NewError(CodeContextInvalid, "ambiguous domain binding: multiple fact families")
	}
}

func (c FactualReadContext) CapabilityBinding(key, scope string) (CapabilityBinding, bool) {
	v, ok := c.capabilities[key+"/"+scope]
	return v, ok
}
func dateOnly(v time.Time) time.Time {
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
}
func day(v time.Time) string { return dateOnly(v).Format("2006-01-02") }
func optionalID(id int64) string {
	if id <= 0 {
		return "NULL"
	}
	return fmt.Sprint(id)
}

// FinalizedAtPtr returns a UTC pointer for optional DomainBinding.FinalizedAt.
func FinalizedAtPtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	x := t.UTC()
	return &x
}
func optionalTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "NULL"
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func cloneDomainBinding(v DomainBinding) DomainBinding {
	if v.CoverageFrom != nil {
		x := *v.CoverageFrom
		v.CoverageFrom = &x
	}
	if v.CoverageTo != nil {
		x := *v.CoverageTo
		v.CoverageTo = &x
	}
	if v.FinalizedAt != nil {
		x := *v.FinalizedAt
		v.FinalizedAt = &x
	}
	return v
}
