package analysis

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// PreDiscoveryInput is the accepted artifact set before Discovery (Stage 7).
type PreDiscoveryInput struct {
	SellerAccountID int64
	UniverseID      string
	UniverseHash    string

	AnalysisDate canonicaljson.Date
	CutoffAt     time.Time

	ContractBundleHash string

	ExpectedCoreSchema        string
	ExpectedCoreProfileSeal   string
	ExpectedDetailSchema      string
	ExpectedDetailProfileSeal string

	Core     landscape.LandscapeSet
	CoreHash string

	Ranking   materiality.RankingArtifact
	Selection materiality.SelectionArtifact

	Detail     landscape.LandscapeSet
	DetailHash string
}

// ValidatePreDiscoveryFoundation delegates to ValidatePreDiscoveryFoundationV3.
func ValidatePreDiscoveryFoundation(in PreDiscoveryInput) error {
	return ValidatePreDiscoveryFoundationV3(in)
}

// ValidatePreDiscoveryFoundationV3 is the deterministic pre-Discovery firewall (Prompt №9.4 §97–107).
func ValidatePreDiscoveryFoundationV3(in PreDiscoveryInput) error {
	if in.SellerAccountID <= 0 || in.UniverseID == "" {
		return fmt.Errorf("PRE_DISCOVERY_INVALID: seller and universe required")
	}
	if in.CoreHash == "" || in.Ranking.RankingHash == "" || in.Selection.SelectionHash == "" || in.DetailHash == "" {
		return fmt.Errorf("PRE_DISCOVERY_INVALID: artifact hashes required")
	}
	if err := validateLandscapeContext("core", in.Core.Context, in); err != nil {
		return err
	}
	if err := validateLandscapeContext("detail", in.Detail.Context, in); err != nil {
		return err
	}
	if in.ExpectedCoreSchema != "" && in.Core.Context.LandscapeSchemaVersion != in.ExpectedCoreSchema {
		return fmt.Errorf("PRE_DISCOVERY_SCHEMA_MISMATCH: core schema")
	}
	if in.ExpectedDetailSchema != "" && in.Detail.Context.LandscapeSchemaVersion != in.ExpectedDetailSchema {
		return fmt.Errorf("PRE_DISCOVERY_SCHEMA_MISMATCH: detail schema")
	}
	if in.ExpectedCoreProfileSeal != "" && in.Core.Context.LandscapeProfileVersion != in.ExpectedCoreProfileSeal {
		return fmt.Errorf("PRE_DISCOVERY_PROFILE_MISMATCH: core profile seal")
	}
	if in.ExpectedDetailProfileSeal != "" && in.Detail.Context.LandscapeProfileVersion != in.ExpectedDetailProfileSeal {
		return fmt.Errorf("PRE_DISCOVERY_PROFILE_MISMATCH: detail profile seal")
	}
	if in.Core.Context.LandscapeProfileVersion == in.Detail.Context.LandscapeProfileVersion {
		return fmt.Errorf("PRE_DISCOVERY_PROFILE_MISMATCH: core and detail profile seals must differ")
	}

	_, recalcCore, err := landscape.MarshalAndHash(in.Core)
	if err != nil {
		return fmt.Errorf("PRE_DISCOVERY_HASH_RECOMPUTE: core: %w", err)
	}
	if recalcCore != in.CoreHash {
		return fmt.Errorf("PRE_DISCOVERY_HASH_MISMATCH: core hash")
	}
	_, recalcDetail, err := landscape.MarshalAndHash(in.Detail)
	if err != nil {
		return fmt.Errorf("PRE_DISCOVERY_HASH_RECOMPUTE: detail: %w", err)
	}
	if recalcDetail != in.DetailHash {
		return fmt.Errorf("PRE_DISCOVERY_HASH_MISMATCH: detail hash")
	}

	recalcRanking := materiality.RecomputeRankingHash(in.Ranking)
	if recalcRanking != in.Ranking.RankingHash {
		return fmt.Errorf("PRE_DISCOVERY_HASH_MISMATCH: ranking hash")
	}
	recalcSelection := materiality.RecomputeSelectionHash(in.Selection)
	if recalcSelection != in.Selection.SelectionHash {
		return fmt.Errorf("PRE_DISCOVERY_HASH_MISMATCH: selection hash")
	}

	if in.Ranking.InputCoreHash != in.CoreHash {
		return fmt.Errorf("PRE_DISCOVERY_HASH_MISMATCH: ranking input core hash")
	}
	if in.Selection.RankingHash != in.Ranking.RankingHash {
		return fmt.Errorf("PRE_DISCOVERY_HASH_MISMATCH: selection ranking hash")
	}
	if in.Detail.Context.MaterialitySelectionHash != "" &&
		in.Detail.Context.MaterialitySelectionHash != in.Selection.SelectionHash {
		return fmt.Errorf("PRE_DISCOVERY_HASH_MISMATCH: detail materiality selection hash")
	}

	sel := selectionFromArtifact(in.Selection)
	if err := validateExactDetailEntitySet(in.Detail, sel); err != nil {
		return err
	}
	if err := validateNoMaterialityMetadataLeak(in.Detail); err != nil {
		return err
	}
	if err := validateLandscapeDecimals(in.Core); err != nil {
		return fmt.Errorf("PRE_DISCOVERY_DECIMAL: core: %w", err)
	}
	if err := validateLandscapeDecimals(in.Detail); err != nil {
		return fmt.Errorf("PRE_DISCOVERY_DECIMAL: detail: %w", err)
	}
	if err := validateDetailSupportingContexts(in.Detail); err != nil {
		return err
	}
	if err := validatePreDiscoveryV4(in); err != nil {
		return err
	}
	return nil
}

func validateLandscapeContext(label string, ctx landscape.LandscapeContext, in PreDiscoveryInput) error {
	if ctx.SellerAccountID != in.SellerAccountID {
		return fmt.Errorf("PRE_DISCOVERY_CONTEXT_MISMATCH: %s seller", label)
	}
	if in.UniverseID != "" && ctx.FactualUniverseID != in.UniverseID {
		return fmt.Errorf("PRE_DISCOVERY_CONTEXT_MISMATCH: %s universe id", label)
	}
	if in.UniverseHash != "" && ctx.FactualUniverseHash != in.UniverseHash {
		return fmt.Errorf("PRE_DISCOVERY_CONTEXT_MISMATCH: %s universe hash", label)
	}
	if !time.Time(in.AnalysisDate).IsZero() && ctx.AnalysisDate != in.AnalysisDate {
		return fmt.Errorf("PRE_DISCOVERY_CONTEXT_MISMATCH: %s analysis date", label)
	}
	if !in.CutoffAt.IsZero() && !ctx.AnalysisCutoffAt.Equal(in.CutoffAt.UTC()) {
		return fmt.Errorf("PRE_DISCOVERY_CONTEXT_MISMATCH: %s cutoff", label)
	}
	if in.ContractBundleHash != "" && ctx.ContractBundleHash != in.ContractBundleHash {
		return fmt.Errorf("PRE_DISCOVERY_CONTEXT_MISMATCH: %s contract bundle", label)
	}
	return nil
}

func validateExactDetailEntitySet(d landscape.LandscapeSet, sel materiality.Selection) error {
	wantCat := map[int64]struct{}{}
	wantSKU := map[int64]struct{}{}
	wantCamp := map[int64]struct{}{}
	for _, ref := range materiality.SelectionEntityRefs(sel) {
		switch ref.Kind {
		case landscape.EntityCategory:
			wantCat[ref.ID] = struct{}{}
		case landscape.EntitySKU:
			wantSKU[ref.ID] = struct{}{}
		case landscape.EntityCampaign:
			wantCamp[ref.ID] = struct{}{}
		case landscape.EntityAccount:
			if d.Account.Identity.SellerAccountID != ref.ID {
				return fmt.Errorf("PRE_DISCOVERY_MISSING_ENTITY: ACCOUNT:%d", ref.ID)
			}
		}
	}
	if len(d.Categories) != len(wantCat) {
		return fmt.Errorf("PRE_DISCOVERY_ENTITY_SET_MISMATCH: categories")
	}
	if len(d.SKUs) != len(wantSKU) {
		return fmt.Errorf("PRE_DISCOVERY_ENTITY_SET_MISMATCH: skus")
	}
	if len(d.Campaigns) != len(wantCamp) {
		return fmt.Errorf("PRE_DISCOVERY_ENTITY_SET_MISMATCH: campaigns")
	}
	for _, c := range d.Categories {
		if _, ok := wantCat[c.Identity.CategoryID]; !ok {
			return fmt.Errorf("PRE_DISCOVERY_EXTRA_ENTITY: CATEGORY:%d", c.Identity.CategoryID)
		}
	}
	for _, s := range d.SKUs {
		if _, ok := wantSKU[s.Identity.CanonicalSKUID]; !ok {
			return fmt.Errorf("PRE_DISCOVERY_EXTRA_ENTITY: SKU:%d", s.Identity.CanonicalSKUID)
		}
	}
	for _, c := range d.Campaigns {
		if _, ok := wantCamp[c.Identity.CampaignID]; !ok {
			return fmt.Errorf("PRE_DISCOVERY_EXTRA_ENTITY: CAMPAIGN:%d", c.Identity.CampaignID)
		}
	}
	for _, ref := range materiality.SelectionEntityRefs(sel) {
		if !detailHasEntity(d, ref) {
			return fmt.Errorf("PRE_DISCOVERY_MISSING_ENTITY: %s:%d", ref.Kind, ref.ID)
		}
	}
	return nil
}

var prediscoveryMaterialityLeakKeys = []string{
	"materiality_score",
	"fusion_utility",
	"fusion_rank",
	"diversity_rank",
	"lane_rank",
	"lane_memberships",
	"lane_fingerprint",
	"rrf",
	"diversity_penalty",
	"selection_reason",
	"anchor_reason",
	"reason_codes",
}

func validateNoMaterialityMetadataLeak(d landscape.LandscapeSet) error {
	return scanJSONKeys(reflect.ValueOf(d), prediscoveryMaterialityLeakKeys)
}

func scanJSONKeys(v reflect.Value, banned []string) error {
	if !v.IsValid() {
		return nil
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return scanJSONKeys(v.Elem(), banned)
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := scanJSONKeys(v.Index(i), banned); err != nil {
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
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			for _, b := range banned {
				if tag == b {
					return fmt.Errorf("PRE_DISCOVERY_MATERIALITY_LEAK: %s", tag)
				}
			}
			if err := scanJSONKeys(v.Field(i), banned); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateLandscapeDecimals(set landscape.LandscapeSet) error {
	return scanDecimalFields(reflect.ValueOf(set))
}

func scanDecimalFields(v reflect.Value) error {
	if !v.IsValid() {
		return nil
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return scanDecimalFields(v.Elem())
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := scanDecimalFields(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(canonicaljson.Decimal("")) {
			return validateDecimal(canonicaljson.Decimal(v.String()))
		}
		for i := 0; i < v.NumField(); i++ {
			if err := scanDecimalFields(v.Field(i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateDecimal(d canonicaljson.Decimal) error {
	s := strings.TrimSpace(string(d))
	if s == "" {
		return fmt.Errorf("empty decimal")
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "nan") || strings.Contains(lower, "inf") {
		return fmt.Errorf("invalid decimal %q", d)
	}
	if _, ok := new(big.Rat).SetString(s); !ok {
		return fmt.Errorf("invalid decimal %q", d)
	}
	return nil
}

func validateDetailSupportingContexts(d landscape.LandscapeSet) error {
	for _, s := range d.SKUs {
		if err := validateSupportingContext(s.CampaignContext, "sku.campaign_context"); err != nil {
			return err
		}
		if err := validateSupportingContext(s.PromotionContext, "sku.promotion_context"); err != nil {
			return err
		}
		if err := validateSupportingContext(s.QueryContext, "sku.query_context"); err != nil {
			return err
		}
	}
	for _, c := range d.Campaigns {
		if err := validateSupportingContext(c.SKUContext, "campaign.sku_context"); err != nil {
			return err
		}
	}
	return nil
}

func validateSupportingContext[T any](sc *landscape.SupportingContext[T], path string) error {
	if sc == nil {
		return nil
	}
	if sc.State == landscape.BlockUnavailable || sc.State == landscape.BlockNotApplicable || sc.State == landscape.BlockUnknown {
		if len(sc.Items) != 0 {
			return fmt.Errorf("PRE_DISCOVERY_SUPPORTING: %s must not carry items when state=%s", path, sc.State)
		}
		if sc.State == landscape.BlockUnavailable && len(sc.Limitations) == 0 {
			return fmt.Errorf("PRE_DISCOVERY_SUPPORTING: %s UNAVAILABLE requires limitations", path)
		}
		return nil
	}
	if sc.ReturnedCount != int64(len(sc.Items)) {
		return fmt.Errorf("PRE_DISCOVERY_SUPPORTING: %s returned_count != len(items)", path)
	}
	if sc.ReturnedCount > sc.TotalCount {
		return fmt.Errorf("PRE_DISCOVERY_SUPPORTING: %s returned_count > total_count", path)
	}
	wantTruncated := sc.TotalCount > sc.ReturnedCount
	if sc.Truncated != wantTruncated {
		return fmt.Errorf("PRE_DISCOVERY_SUPPORTING: %s truncated flag", path)
	}
	if sc.Truncated && sc.ReturnedCount >= sc.TotalCount && sc.TotalCount > 0 {
		return fmt.Errorf("PRE_DISCOVERY_SUPPORTING: %s truncated=true but returned covers total_count", path)
	}
	if !sc.Truncated && sc.ReturnedCount < sc.TotalCount {
		return fmt.Errorf("PRE_DISCOVERY_SUPPORTING: %s truncated=false but returned_count < total_count", path)
	}
	if sc.ReturnedCount > 0 {
		if strings.TrimSpace(sc.SortBasis) == "" || strings.TrimSpace(sc.SemanticScope) == "" {
			return fmt.Errorf("PRE_DISCOVERY_SUPPORTING: %s metadata incomplete", path)
		}
	}
	if sc.State == landscape.BlockReadyEmpty {
		if len(sc.Items) != 0 || sc.TotalCount != 0 || sc.ReturnedCount != 0 {
			return fmt.Errorf("PRE_DISCOVERY_SUPPORTING: %s READY_EMPTY must have zero items and counts", path)
		}
	}
	if sc.State == landscape.BlockPartial && len(sc.Limitations) == 0 {
		return fmt.Errorf("PRE_DISCOVERY_SUPPORTING: %s PARTIAL requires limitations", path)
	}
	return nil
}

func selectionFromArtifact(a materiality.SelectionArtifact) materiality.Selection {
	return materiality.Selection{
		Account:    a.Account,
		Categories: a.Categories,
		SKUs:       a.SKUs,
		Campaigns:  a.Campaigns,
	}
}

func detailHasEntity(d landscape.LandscapeSet, ref landscape.EntityRef) bool {
	switch ref.Kind {
	case landscape.EntitySKU:
		for _, s := range d.SKUs {
			if s.Identity.CanonicalSKUID == ref.ID {
				return true
			}
		}
	case landscape.EntityCategory:
		for _, c := range d.Categories {
			if c.Identity.CategoryID == ref.ID {
				return true
			}
		}
	case landscape.EntityCampaign:
		for _, c := range d.Campaigns {
			if c.Identity.CampaignID == ref.ID {
				return true
			}
		}
	case landscape.EntityAccount:
		return d.Account.Identity.SellerAccountID == ref.ID
	}
	return false
}
