package landscape

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

// ValidateCoreProfileCapabilitiesV3 fail-closes when a REQUIRED block capability is missing.
func ValidateCoreProfileCapabilitiesV3(sealed factualapi.FactualReadContext, profiles CoreProfileBundleV3) error {
	for _, p := range []ExecutableProfileV3{profiles.Account, profiles.Category, profiles.SKU, profiles.Campaign} {
		if p.Name == "" {
			continue
		}
		if err := validateProfileCapabilitiesV3(sealed, p); err != nil {
			return err
		}
	}
	return nil
}

func validateProfileCapabilitiesV3(sealed factualapi.FactualReadContext, profile ExecutableProfileV3) error {
	for _, spec := range profile.Blocks {
		if spec.Mode != BlockRequired {
			continue
		}
		if err := requireBlockCapabilitiesV3(sealed, spec); err != nil {
			return fmt.Errorf("%s: block %q: %w", ErrProfileCapabilityContractViolation, spec.BlockID, err)
		}
	}
	return nil
}

func requireBlockCapabilitiesV3(sealed factualapi.FactualReadContext, spec BlockSpecV3) error {
	if len(spec.CapabilityKeys) == 0 {
		return nil
	}
	match := spec.CapabilityMatch
	if match == "" {
		match = CapabilityAll
	}
	switch match {
	case CapabilityAny:
		for _, key := range spec.CapabilityKeys {
			if err := requireSealedCapability(sealed, key); err == nil {
				return nil
			}
		}
		return fmt.Errorf("no capability bound: %v", spec.CapabilityKeys)
	default:
		for _, key := range spec.CapabilityKeys {
			if err := requireSealedCapability(sealed, key); err != nil {
				return fmt.Errorf("capability %q: %w", key, err)
			}
		}
		return nil
	}
}

// ValidateCoreProfileConformanceV3 checks built Core against executable V3 profiles.
func ValidateCoreProfileConformanceV3(set LandscapeSet, profiles CoreProfileBundleV3) error {
	if err := validateEntityProfileConformanceV3(AccountPresentBlocks(set.Account), profiles.Account, set.GlobalCapabilities); err != nil {
		return fmt.Errorf("account: %w", err)
	}
	for _, c := range set.Categories {
		if err := validateEntityProfileConformanceV3(CategoryPresentBlocks(c), profiles.Category, set.GlobalCapabilities); err != nil {
			return fmt.Errorf("category %d: %w", c.Identity.CategoryID, err)
		}
	}
	for _, s := range set.SKUs {
		if err := validateEntityProfileConformanceV3(SKUPresentBlocks(s), profiles.SKU, set.GlobalCapabilities); err != nil {
			return fmt.Errorf("sku %d: %w", s.Identity.CanonicalSKUID, err)
		}
	}
	for _, c := range set.Campaigns {
		if err := validateEntityProfileConformanceV3(CampaignPresentBlocks(c), profiles.Campaign, set.GlobalCapabilities); err != nil {
			return fmt.Errorf("campaign %d: %w", c.Identity.CampaignID, err)
		}
	}
	return nil
}

// ValidateDetailProfileConformanceV3 checks built Detail against executable V3 Detail profiles.
func ValidateDetailProfileConformanceV3(set LandscapeSet, profiles DetailProfileBundleV3) error {
	if err := validateEntityProfileConformanceV3(AccountPresentBlocks(set.Account), profiles.Account, set.GlobalCapabilities); err != nil {
		return fmt.Errorf("account: %w", err)
	}
	for _, c := range set.Categories {
		if err := validateEntityProfileConformanceV3(CategoryPresentBlocks(c), profiles.Category, set.GlobalCapabilities); err != nil {
			return fmt.Errorf("category %d: %w", c.Identity.CategoryID, err)
		}
	}
	for _, s := range set.SKUs {
		if err := validateEntityProfileConformanceV3(SKUPresentBlocks(s), profiles.SKU, set.GlobalCapabilities); err != nil {
			return fmt.Errorf("sku %d: %w", s.Identity.CanonicalSKUID, err)
		}
	}
	for _, c := range set.Campaigns {
		if err := validateEntityProfileConformanceV3(CampaignPresentBlocks(c), profiles.Campaign, set.GlobalCapabilities); err != nil {
			return fmt.Errorf("campaign %d: %w", c.Identity.CampaignID, err)
		}
	}
	return nil
}

func validateEntityProfileConformanceV3(present map[string]bool, profile ExecutableProfileV3, capabilities []CapabilityFlag) error {
	allowed := map[string]BlockSpecV3{}
	for _, b := range profile.Blocks {
		allowed[b.BlockID] = b
	}
	for blockID := range present {
		if !present[blockID] {
			continue
		}
		if _, ok := allowed[blockID]; !ok {
			return fmt.Errorf("profile conformance: block %q not allowed by profile %s", blockID, profile.Name)
		}
	}
	for _, spec := range profile.Blocks {
		switch spec.Mode {
		case BlockRequired:
			if !capabilityReadyV3(capabilities, spec) {
				return fmt.Errorf("%s: block %q capability not ready", ErrProfileCapabilityContractViolation, spec.BlockID)
			}
			if !present[spec.BlockID] {
				return fmt.Errorf("profile conformance: required block %q missing for profile %s", spec.BlockID, profile.Name)
			}
		case BlockOptionalCapability:
			if capabilityReadyV3(capabilities, spec) && !present[spec.BlockID] {
				return fmt.Errorf("profile conformance: optional-capability block %q missing while capability READY for profile %s", spec.BlockID, profile.Name)
			}
			if !capabilityReadyV3(capabilities, spec) && present[spec.BlockID] {
				return fmt.Errorf("profile conformance: optional-capability block %q present while capability not usable for profile %s", spec.BlockID, profile.Name)
			}
		case BlockDerived:
			if len(spec.CapabilityKeys) > 0 && !capabilityReadyV3(capabilities, spec) {
				if present[spec.BlockID] {
					return fmt.Errorf("profile conformance: derived block %q present while capability not usable for profile %s", spec.BlockID, profile.Name)
				}
				continue
			}
			if derivedPrerequisitesReady(spec, present) && !present[spec.BlockID] {
				return fmt.Errorf("profile conformance: derived block %q missing for profile %s", spec.BlockID, profile.Name)
			}
		case BlockNotSupportedBySemantics:
			if present[spec.BlockID] {
				return fmt.Errorf("profile conformance: block %q not supported by semantics but present", spec.BlockID)
			}
		}
	}
	return nil
}

func capabilityReadyV3(capabilities []CapabilityFlag, spec BlockSpecV3) bool {
	if len(spec.CapabilityKeys) == 0 {
		return true
	}
	match := spec.CapabilityMatch
	if match == "" {
		match = CapabilityAll
	}
	if match == CapabilityAny {
		for _, key := range spec.CapabilityKeys {
			if err := requireCapabilityFlags(capabilities, key); err == nil {
				return true
			}
		}
		return false
	}
	for _, key := range spec.CapabilityKeys {
		if err := requireCapabilityFlags(capabilities, key); err != nil {
			return false
		}
	}
	return true
}

func derivedPrerequisitesReady(spec BlockSpecV3, present map[string]bool) bool {
	_ = spec
	return len(present) > 0
}
