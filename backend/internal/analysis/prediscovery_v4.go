package analysis

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
)

func validatePreDiscoveryV4(in PreDiscoveryInput) error {
	if err := validateSupportingProvenanceV4(in.Detail); err != nil {
		return err
	}
	if err := validateAdvertisingPeriodCoherenceV4(in.Core); err != nil {
		return err
	}
	if err := validateAdvertisingPeriodCoherenceV4(in.Detail); err != nil {
		return err
	}
	if err := validateCalculatedDRRCoherenceV4(in.Core); err != nil {
		return err
	}
	if err := validateCalculatedDRRCoherenceV4(in.Detail); err != nil {
		return err
	}
	return nil
}

func validateSupportingProvenanceV4(d landscape.LandscapeSet) error {
	for _, s := range d.SKUs {
		if err := requirePeriodContext(s.QueryContext, "sku.query_context"); err != nil {
			return err
		}
		if err := requirePeriodContext(s.CampaignContext, "sku.campaign_context"); err != nil {
			return err
		}
		if err := requireAsOfContext(s.PromotionContext, "sku.promotion_context"); err != nil {
			return err
		}
	}
	for _, c := range d.Campaigns {
		if err := requireAsOfWindow(c.SKUContext, "campaign.sku_context", "CAMPAIGN_TARGETS_AS_OF"); err != nil {
			return err
		}
	}
	return nil
}

func supportingReadyOrPartial[T any](sc *landscape.SupportingContext[T]) bool {
	if sc == nil {
		return false
	}
	return sc.State == landscape.BlockReady || sc.State == landscape.BlockPartial
}

func requirePeriodContext[T any](sc *landscape.SupportingContext[T], path string) error {
	if !supportingReadyOrPartial(sc) {
		return nil
	}
	if sc.WindowRef == "" || sc.Period == nil || sc.AsOf != nil {
		return fmt.Errorf("PRE_DISCOVERY_SUPPORTING_PROVENANCE: %s requires WindowRef+Period and nil AsOf", path)
	}
	return nil
}

func requireAsOfContext[T any](sc *landscape.SupportingContext[T], path string) error {
	if !supportingReadyOrPartial(sc) {
		return nil
	}
	if sc.WindowRef == "" || sc.AsOf == nil || sc.Period != nil {
		return fmt.Errorf("PRE_DISCOVERY_SUPPORTING_PROVENANCE: %s requires WindowRef+AsOf and nil Period", path)
	}
	return nil
}

func requireAsOfWindow[T any](sc *landscape.SupportingContext[T], path, window string) error {
	if err := requireAsOfContext(sc, path); err != nil {
		return err
	}
	if supportingReadyOrPartial(sc) && sc.WindowRef != window {
		return fmt.Errorf("PRE_DISCOVERY_SUPPORTING_PROVENANCE: %s WindowRef=%s want %s", path, sc.WindowRef, window)
	}
	return nil
}

func advertisingWindow(set landscape.LandscapeSet, family string) *landscape.DomainWindow {
	for i := range set.DomainWindows {
		w := &set.DomainWindows[i]
		if w.Domain == "advertising" && w.FactFamily == family {
			return w
		}
	}
	return nil
}

func periodMatchesWindow(p, w *landscape.DateRange) bool {
	if p == nil || w == nil {
		return false
	}
	return p.From == w.From && p.To == w.To
}

func advertisingBlockUsable(state landscape.BlockState) bool {
	return state == landscape.BlockReady || state == landscape.BlockPartial
}

func requireAdPeriods(label string, state landscape.BlockState, cur, prev *landscape.DateRange, win *landscape.DomainWindow) error {
	if !advertisingBlockUsable(state) {
		return nil
	}
	if win == nil {
		return fmt.Errorf("PRE_DISCOVERY_AD_PERIOD: %s missing DomainWindow", label)
	}
	if cur == nil {
		return fmt.Errorf("PRE_DISCOVERY_AD_PERIOD: %s READY/PARTIAL requires CurrentPeriod", label)
	}
	if prev == nil {
		return fmt.Errorf("PRE_DISCOVERY_AD_PERIOD: %s READY/PARTIAL requires PreviousPeriod", label)
	}
	if !periodMatchesWindow(cur, win.Current) {
		return fmt.Errorf("PRE_DISCOVERY_AD_PERIOD: %s current period mismatch", label)
	}
	if !periodMatchesWindow(prev, win.Previous) {
		return fmt.Errorf("PRE_DISCOVERY_AD_PERIOD: %s previous period mismatch", label)
	}
	return nil
}

func validateAdvertisingPeriodCoherenceV4(set landscape.LandscapeSet) error {
	trafficWin := advertisingWindow(set, "ADVERTISING.TRAFFIC")
	attrWin := advertisingWindow(set, "ADVERTISING.ATTRIBUTION")
	if set.Account.Advertising != nil {
		if set.Account.Advertising.Traffic != nil {
			tr := set.Account.Advertising.Traffic
			if err := requireAdPeriods("account.advertising.traffic", tr.State, tr.CurrentPeriod, tr.PreviousPeriod, trafficWin); err != nil {
				return err
			}
		}
		if set.Account.Advertising.Attribution != nil {
			at := set.Account.Advertising.Attribution
			if err := requireAdPeriods("account.advertising.attribution", at.State, at.CurrentPeriod, at.PreviousPeriod, attrWin); err != nil {
				return err
			}
		}
	}
	for _, c := range set.Campaigns {
		if c.Performance != nil && c.Performance.Traffic != nil {
			tr := c.Performance.Traffic
			if err := requireAdPeriods(fmt.Sprintf("campaign %d traffic", c.Identity.CampaignID), tr.State, tr.CurrentPeriod, tr.PreviousPeriod, trafficWin); err != nil {
				return err
			}
		}
		if c.Performance != nil && c.Performance.Attribution != nil {
			at := c.Performance.Attribution
			if err := requireAdPeriods(fmt.Sprintf("campaign %d attribution", c.Identity.CampaignID), at.State, at.CurrentPeriod, at.PreviousPeriod, attrWin); err != nil {
				return err
			}
		}
	}
	for _, s := range set.SKUs {
		if s.Advertising != nil && s.Advertising.Traffic != nil {
			tr := s.Advertising.Traffic
			if err := requireAdPeriods(fmt.Sprintf("sku %d traffic", s.Identity.CanonicalSKUID), tr.State, tr.CurrentPeriod, tr.PreviousPeriod, trafficWin); err != nil {
				return err
			}
		}
		if s.Advertising != nil && s.Advertising.Attribution != nil {
			at := s.Advertising.Attribution
			if err := requireAdPeriods(fmt.Sprintf("sku %d attribution", s.Identity.CanonicalSKUID), at.State, at.CurrentPeriod, at.PreviousPeriod, attrWin); err != nil {
				return err
			}
		}
	}
	for _, c := range set.Categories {
		if c.Advertising != nil && c.Advertising.Traffic != nil {
			tr := c.Advertising.Traffic
			if err := requireAdPeriods(fmt.Sprintf("category %d traffic", c.Identity.CategoryID), tr.State, tr.CurrentPeriod, tr.PreviousPeriod, trafficWin); err != nil {
				return err
			}
		}
		if c.Advertising != nil && c.Advertising.Attribution != nil {
			at := c.Advertising.Attribution
			if err := requireAdPeriods(fmt.Sprintf("category %d attribution", c.Identity.CategoryID), at.State, at.CurrentPeriod, at.PreviousPeriod, attrWin); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateCalculatedDRRCoherenceV4(set landscape.LandscapeSet) error {
	check := func(traffic *landscape.CampaignTrafficBlock, attr *landscape.CampaignAttributionBlock, label string) error {
		if attr == nil || attr.CalculatedDRR == nil {
			return nil
		}
		if traffic == nil || !dateRangesEqualV4(traffic.CurrentPeriod, attr.CurrentPeriod) {
			return fmt.Errorf("PRE_DISCOVERY_DRR: %s CalculatedDRR requires matching traffic/attribution current periods", label)
		}
		if attr.CalculatedDRR.PreviousPct != nil && !dateRangesEqualV4(traffic.PreviousPeriod, attr.PreviousPeriod) {
			return fmt.Errorf("PRE_DISCOVERY_DRR: %s CalculatedDRR previous requires matching previous periods", label)
		}
		return nil
	}
	for _, c := range set.Campaigns {
		if c.Performance == nil {
			continue
		}
		if err := check(c.Performance.Traffic, c.Performance.Attribution, fmt.Sprintf("campaign %d", c.Identity.CampaignID)); err != nil {
			return err
		}
	}
	for _, s := range set.SKUs {
		if s.Advertising == nil || s.Advertising.Attribution == nil || s.Advertising.Attribution.CalculatedDRR == nil {
			continue
		}
		tr := s.Advertising.Traffic
		at := s.Advertising.Attribution
		if tr == nil || !dateRangesEqualV4(tr.CurrentPeriod, at.CurrentPeriod) {
			return fmt.Errorf("PRE_DISCOVERY_DRR: sku %d CalculatedDRR requires matching periods", s.Identity.CanonicalSKUID)
		}
	}
	return nil
}

func dateRangesEqualV4(a, b *landscape.DateRange) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.From == b.From && a.To == b.To
}
