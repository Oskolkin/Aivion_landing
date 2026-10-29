package landscape

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

// BlockExecutionDecision is the Profile-authorized execution outcome for one V3 block.
type BlockExecutionDecision struct {
	Execute    bool
	State      BlockState
	Limitation *Limitation
}

// Block returns the block spec by ID.
func (p ExecutableProfileV3) Block(blockID string) (BlockSpecV3, bool) {
	return p.blockByID(blockID)
}

// CallPlanEntryV3 is one deterministic execution entry from a V3 profile.
type CallPlanEntryV3 struct {
	ProfileName     string
	BlockID         string
	Mode            BlockMode
	CapabilityKeys  []string
	CapabilityMatch CapabilityMatch
	FactFamilies    []string
	FactualMethods  []string
	Periods         []PeriodSpec
	Supporting      *SupportingSpec
	Serialization   SerializationPolicy
}

// CallPlan returns profile blocks in registry order for static validation.
func (p ExecutableProfileV3) CallPlan() []CallPlanEntryV3 {
	out := make([]CallPlanEntryV3, 0, len(p.Blocks))
	for _, b := range p.Blocks {
		out = append(out, CallPlanEntryV3{
			ProfileName:     p.Name,
			BlockID:         b.BlockID,
			Mode:            b.Mode,
			CapabilityKeys:  append([]string(nil), b.CapabilityKeys...),
			CapabilityMatch: b.CapabilityMatch,
			FactFamilies:    append([]string(nil), b.FactFamilies...),
			FactualMethods:  append([]string(nil), b.FactualMethods...),
			Periods:         append([]PeriodSpec(nil), b.Periods...),
			Supporting:      b.Supporting,
			Serialization:   b.Serialization,
		})
	}
	return out
}

// DecideBlockExecutionV3 determines whether a factual read is authorized for one block.
func DecideBlockExecutionV3(sealed factualapi.FactualReadContext, spec BlockSpecV3) (BlockExecutionDecision, error) {
	switch spec.Mode {
	case BlockRequired:
		ok, _, reason := capabilitySetUsable(sealed, spec.CapabilityKeys, spec.CapabilityMatch)
		if !ok {
			return BlockExecutionDecision{}, fmt.Errorf("profile capability contract violation: block=%s capability=%v reason=%s", spec.BlockID, spec.CapabilityKeys, reason)
		}
		return BlockExecutionDecision{Execute: true}, nil
	case BlockOptionalCapability:
		ok, _, _ := capabilitySetUsable(sealed, spec.CapabilityKeys, spec.CapabilityMatch)
		if !ok {
			return BlockExecutionDecision{
				Execute: false,
				State:   BlockUnavailable,
				Limitation: &Limitation{
					Code:    "CAPABILITY_NOT_AVAILABLE",
					Message: "optional block capability not usable in sealed context",
				},
			}, nil
		}
		return BlockExecutionDecision{Execute: true}, nil
	case BlockDerived, BlockNotSupportedBySemantics:
		return BlockExecutionDecision{Execute: false}, nil
	default:
		return BlockExecutionDecision{}, fmt.Errorf("profile execution: unknown block mode %q for %s", spec.Mode, spec.BlockID)
	}
}

func capabilitySetUsable(sealed factualapi.FactualReadContext, keys []string, mode CapabilityMatch) (bool, string, string) {
	if len(keys) == 0 {
		return true, "READY", ""
	}
	if mode == "" {
		mode = CapabilityAll
	}
	usable := 0
	var lastState, lastReason string
	for _, key := range keys {
		ok, state, reason := capabilityUsable(sealed, key)
		lastState, lastReason = state, reason
		if ok {
			usable++
		}
	}
	switch mode {
	case CapabilityAny:
		if usable > 0 {
			return true, "READY", ""
		}
		return false, lastState, lastReason
	default: // ALL
		if usable == len(keys) {
			return true, "READY", ""
		}
		if usable == 0 {
			return false, nzStr(lastState, "MISSING"), nzStr(lastReason, "CAPABILITY_NOT_BOUND")
		}
		return false, "PARTIAL", "CAPABILITY_PARTIAL"
	}
}

func nzStr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// ProfileBlockDecision resolves one profile block and its execution decision.
func ProfileBlockDecision(sealed factualapi.FactualReadContext, profile ExecutableProfileV3, blockID string) (spec BlockSpecV3, decision BlockExecutionDecision, present bool, err error) {
	spec, present = profile.Block(blockID)
	if !present {
		return BlockSpecV3{}, BlockExecutionDecision{}, false, nil
	}
	decision, err = DecideBlockExecutionV3(sealed, spec)
	return spec, decision, true, err
}
