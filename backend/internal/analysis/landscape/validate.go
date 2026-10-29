package landscape

import (
	"fmt"
	"reflect"
	"strings"
)

// ValidateLandscapeCore checks Core contract invariants (not semantic meaning).
func ValidateLandscapeCore(set LandscapeSet, sellerID int64) error {
	if err := validateCommon(set, sellerID, false); err != nil {
		return err
	}
	for _, s := range set.SKUs {
		if s.CampaignContext != nil || s.PromotionContext != nil || s.QueryContext != nil {
			return fmt.Errorf("landscape: Core SKU must not include detail supporting lists")
		}
		if err := validateSKUCoreProfileName(set.Context.LandscapeSchemaVersion, s.Profile); err != nil {
			return err
		}
	}
	for _, c := range set.Campaigns {
		if c.SKUContext != nil {
			return fmt.Errorf("landscape: Core Campaign must not include sku_context")
		}
	}
	return nil
}

// ValidateLandscapeDetail checks Detail contract invariants.
func ValidateLandscapeDetail(set LandscapeSet, sellerID int64) error {
	if err := validateCommon(set, sellerID, true); err != nil {
		return err
	}
	for _, s := range set.SKUs {
		if err := validateSupporting(s.CampaignContext, "sku.campaign_context"); err != nil {
			return err
		}
		if err := validateSupporting(s.PromotionContext, "sku.promotion_context"); err != nil {
			return err
		}
		if err := validateSupporting(s.QueryContext, "sku.query_context"); err != nil {
			return err
		}
	}
	for _, c := range set.Campaigns {
		if err := validateSupporting(c.SKUContext, "campaign.sku_context"); err != nil {
			return err
		}
	}
	return nil
}

func validateCommon(set LandscapeSet, sellerID int64, detail bool) error {
	switch set.Context.LandscapeSchemaVersion {
	case LandscapeSchemaV1, LandscapeSchemaV2, LandscapeSchemaV3, LandscapeSchemaV4, LandscapeSchemaV5:
	default:
		return fmt.Errorf("landscape: unsupported schema version %q", set.Context.LandscapeSchemaVersion)
	}
	if set.Context.SellerAccountID != sellerID || set.Account.Identity.SellerAccountID != sellerID {
		return fmt.Errorf("landscape: seller_account_id mismatch")
	}
	if set.Context.FactualUniverseHash == "" || set.Context.ContractBundleHash == "" {
		return fmt.Errorf("landscape: universe/contract hashes required")
	}
	if hit, ok := containsForbidden(set.Context.LandscapeProfileVersion); ok {
		return fmt.Errorf("landscape: forbidden vocabulary in profile version: %s", hit)
	}
	if err := scanForbiddenStruct(reflect.ValueOf(set)); err != nil {
		return err
	}
	for _, c := range set.Categories {
		if !c.Identity.IsLeaf && !c.Identity.Unmapped {
			return fmt.Errorf("landscape: category %d is not leaf and not UNMAPPED", c.Identity.CategoryID)
		}
		if err := validateMoneyBlock(c.Sales); err != nil {
			return fmt.Errorf("category %d sales: %w", c.Identity.CategoryID, err)
		}
	}
	for _, s := range set.SKUs {
		if err := validateSKUSales(s.Sales); err != nil {
			return fmt.Errorf("sku %d sales: %w", s.Identity.CanonicalSKUID, err)
		}
		if err := validateReadinessVsZeros(s); err != nil {
			return err
		}
	}
	if set.Account.Sales != nil &&
		set.Account.Sales.State != BlockReadyEmpty &&
		set.Account.Sales.State != BlockUnavailable &&
		set.Account.Sales.State != BlockNotApplicable &&
		set.Account.Sales.State != BlockUnknown {
		if err := validateMoneyComparison(set.Account.Sales.OrderedAmount); err != nil {
			return fmt.Errorf("account sales amount: %w", err)
		}
		if err := validateCountComparison(set.Account.Sales.OrderedUnits); err != nil {
			return fmt.Errorf("account sales units: %w", err)
		}
	}
	_ = detail
	return nil
}

func validateSKUSales(b *SKUSalesBlock) error {
	if b == nil {
		return nil
	}
	if b.State == BlockReadyEmpty || b.State == BlockUnavailable || b.State == BlockNotApplicable || b.State == BlockUnknown {
		return nil
	}
	if err := validateMoneyComparison(b.OrderedAmount); err != nil {
		return err
	}
	return validateCountComparison(b.OrderedUnits)
}

func validateMoneyBlock(b *CategorySalesBlock) error {
	if b == nil {
		return nil
	}
	if b.State == BlockReadyEmpty || b.State == BlockUnavailable || b.State == BlockNotApplicable || b.State == BlockUnknown {
		return nil
	}
	if err := validateMoneyComparison(b.OrderedAmount); err != nil {
		return err
	}
	return validateCountComparison(b.OrderedUnits)
}

func validateMoneyComparison(m MoneyComparison) error {
	if m.Current == nil && m.Previous == nil && m.ChangeState == "" {
		return nil
	}
	if m.Current != nil && strings.TrimSpace(m.Current.CurrencyCode) == "" {
		return fmt.Errorf("money currency missing on current")
	}
	if m.Previous != nil && strings.TrimSpace(m.Previous.CurrencyCode) == "" {
		return fmt.Errorf("money currency missing on previous")
	}
	if m.AbsoluteDelta != nil && strings.TrimSpace(m.AbsoluteDelta.CurrencyCode) == "" {
		return fmt.Errorf("money currency missing on absolute_delta")
	}
	rebuilt := BuildMoneyComparison(m.Current, m.Previous)
	if rebuilt.ChangeState != m.ChangeState {
		return fmt.Errorf("change_state mismatch: have %s want %s", m.ChangeState, rebuilt.ChangeState)
	}
	if m.ChangeState == ChangeFromZero && m.PctDelta != nil {
		return fmt.Errorf("FROM_ZERO must have nil pct_delta")
	}
	if m.PctDelta != nil {
		if strings.Contains(strings.ToLower(string(*m.PctDelta)), "inf") || strings.Contains(strings.ToLower(string(*m.PctDelta)), "nan") {
			return fmt.Errorf("pct_delta must not be Inf/NaN")
		}
	}
	return nil
}

func validateCountComparison(c CountComparison) error {
	if c.Current == nil && c.Previous == nil && c.ChangeState == "" {
		return nil
	}
	rebuilt := BuildCountComparison(c.Current, c.Previous)
	if rebuilt.ChangeState != c.ChangeState {
		return fmt.Errorf("change_state mismatch: have %s want %s", c.ChangeState, rebuilt.ChangeState)
	}
	if c.ChangeState == ChangeFromZero && c.PctDelta != nil {
		return fmt.Errorf("FROM_ZERO must have nil pct_delta")
	}
	return nil
}

func validateSupporting[T any](sc *SupportingContext[T], path string) error {
	if sc == nil {
		return nil
	}
	if sc.State == BlockUnavailable || sc.State == BlockNotApplicable || sc.State == BlockUnknown {
		if len(sc.Items) != 0 {
			return fmt.Errorf("landscape: %s must not carry items when state=%s", path, sc.State)
		}
		return nil
	}
	if sc.ReturnedCount != int64(len(sc.Items)) {
		return fmt.Errorf("landscape: %s returned_count != len(items)", path)
	}
	if sc.ReturnedCount > sc.TotalCount {
		return fmt.Errorf("landscape: %s returned_count > total_count", path)
	}
	if sc.Truncated && sc.ReturnedCount >= sc.TotalCount && sc.TotalCount > 0 {
		return fmt.Errorf("landscape: %s truncated=true but returned covers total_count", path)
	}
	if !sc.Truncated && sc.ReturnedCount < sc.TotalCount {
		return fmt.Errorf("landscape: %s truncated=false but returned_count < total_count", path)
	}
	if sc.ReturnedCount > 0 {
		if sc.SortBasis == "" || sc.SemanticScope == "" {
			return fmt.Errorf("landscape: %s truncation metadata incomplete", path)
		}
	}
	if sc.State == BlockReadyEmpty {
		if len(sc.Items) != 0 || sc.TotalCount != 0 {
			return fmt.Errorf("landscape: %s READY_EMPTY must have zero items and total_count", path)
		}
	}
	return nil
}

// validateReadinessVsZeros: UNAVAILABLE blocks must not fabricate known zeros as "current fact".
func validateReadinessVsZeros(s SKULandscape) error {
	if s.Advertising != nil && s.Advertising.State == BlockUnavailable {
		if s.Advertising.Traffic != nil && s.Advertising.Traffic.PaidSpend.Current != nil {
			return fmt.Errorf("sku %d: UNAVAILABLE advertising must not carry current values", s.Identity.CanonicalSKUID)
		}
		if s.Advertising.Attribution != nil && s.Advertising.Attribution.AttributedOrderAmount.Current != nil {
			return fmt.Errorf("sku %d: UNAVAILABLE advertising must not carry current values", s.Identity.CanonicalSKUID)
		}
	}
	if s.Supply != nil && s.Supply.State == BlockUnavailable {
		if s.Supply.OpenInboundDeclaredUnits != nil || s.Supply.OpenSupplyCount != nil {
			return fmt.Errorf("sku %d: UNAVAILABLE supply must not carry counts", s.Identity.CanonicalSKUID)
		}
	}
	return nil
}

func scanForbiddenStruct(v reflect.Value) error {
	if !v.IsValid() {
		return nil
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return scanForbiddenStruct(v.Elem())
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := scanForbiddenStruct(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue
			}
			name := f.Name
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "MaterialitySelectionHash" || tag == "materiality_selection_hash" {
				if err := scanForbiddenStruct(v.Field(i)); err != nil {
					return err
				}
				continue
			}
			// Provider-mirrored counters/statuses (e.g. provider_critical_count) are factual.
			if !strings.HasPrefix(strings.ToLower(tag), "provider_") && !strings.HasPrefix(strings.ToLower(name), "provider") {
				if hit, ok := containsForbidden(name); ok {
					return fmt.Errorf("landscape: forbidden vocabulary in field %s (%s)", name, hit)
				}
				if hit, ok := containsForbidden(tag); ok {
					return fmt.Errorf("landscape: forbidden vocabulary in json tag %s (%s)", tag, hit)
				}
			}
			if err := scanForbiddenStruct(v.Field(i)); err != nil {
				return err
			}
		}
	case reflect.String:
		// Do not scan free-form provider strings (names/titles) for vocabulary —
		// only schema field names/tags are gated above.
	}
	return nil
}

func validateSKUCoreProfileName(schemaVersion, profile string) error {
	if profile == "" {
		return nil
	}
	switch schemaVersion {
	case LandscapeSchemaV5:
		if profile != ProfileSKUCoreV5 {
			return fmt.Errorf("landscape: unexpected SKU core profile %q", profile)
		}
	case LandscapeSchemaV4:
		if profile != ProfileSKUCoreV4 {
			return fmt.Errorf("landscape: unexpected SKU core profile %q", profile)
		}
	case LandscapeSchemaV3:
		if profile != ProfileSKUCoreV3 {
			return fmt.Errorf("landscape: unexpected SKU core profile %q", profile)
		}
	case LandscapeSchemaV2:
		if profile != ProfileSKUCoreV2 {
			return fmt.Errorf("landscape: unexpected SKU core profile %q", profile)
		}
	case LandscapeSchemaV1:
		if profile != ProfileSKUCoreV1 {
			return fmt.Errorf("landscape: unexpected SKU core profile %q", profile)
		}
	}
	return nil
}
