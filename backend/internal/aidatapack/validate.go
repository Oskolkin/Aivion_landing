package aidatapack

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
)

var forbiddenSecretSubstrings = []string{
	"api_key_encrypted",
	"client_id_encrypted",
	"performance_client_secret",
	"performance_token_encrypted",
	"performance_token",
	"client_secret",
}

var forbiddenBusinessKeys = []string{
	"recommendation",
	"should_pause",
	"must_replenish",
	"create_supply",
	"pause_campaign",
	"bad_campaign",
	"top_action",
	"next_step",
	"expected_effect",
	"risk_level",
}

// ValidateDataPack checks structural integrity of a built data pack.
func ValidateDataPack(pack *SellerAIDataPack) error {
	if pack == nil {
		return fmt.Errorf("pack is nil")
	}
	if strings.TrimSpace(pack.SchemaVersion) == "" {
		return fmt.Errorf("schema_version is empty")
	}
	if pack.SellerContext.SellerAccountID <= 0 {
		return fmt.Errorf("seller_context.seller_account_id must be > 0")
	}
	if strings.TrimSpace(pack.SellerContext.MetricDate) == "" {
		return fmt.Errorf("seller_context.metric_date is empty")
	}
	if len(pack.FactRegistry) == 0 {
		return fmt.Errorf("fact_registry is empty")
	}
	seenFacts := map[string]struct{}{}
	for id, fact := range pack.FactRegistry {
		if id != fact.FactID {
			return fmt.Errorf("fact_registry key %q does not match fact.FactID %q", id, fact.FactID)
		}
		if _, ok := seenFacts[id]; ok {
			return fmt.Errorf("duplicate fact_id %q", id)
		}
		seenFacts[id] = struct{}{}
		if err := validateFactValue(fact.Value); err != nil {
			return fmt.Errorf("fact_registry.%s: %w", id, err)
		}
	}
	if err := validateEntityRegistryKeys(pack.EntityRegistry); err != nil {
		return err
	}
	if err := validateTableFactRefs(pack); err != nil {
		return err
	}
	if err := validateTableEntityRefs(pack); err != nil {
		return err
	}
	if err := validateCampaignRollupMetrics(pack.Tables.CampaignRollups); err != nil {
		return err
	}
	if err := validateNoTablesConstraintsDuplication(pack); err != nil {
		return err
	}
	if err := validateDataPackJSONSafety(pack); err != nil {
		return err
	}
	if err := walkNumericFields(pack.Tables, "tables"); err != nil {
		return err
	}
	return nil
}

func validateNoTablesConstraintsDuplication(pack *SellerAIDataPack) error {
	raw, err := json.Marshal(pack.Tables)
	if err != nil {
		return err
	}
	var tables map[string]json.RawMessage
	if err := json.Unmarshal(raw, &tables); err != nil {
		return err
	}
	if _, ok := tables["constraints"]; ok {
		return fmt.Errorf("tables.constraints must not exist; use top-level constraints only")
	}
	return nil
}

func validateCampaignRollupMetrics(rows []CampaignRollupRow) error {
	for i, row := range rows {
		for _, win := range []struct {
			name string
			w    CampaignMetricsWindow
		}{
			{"current_day", row.CurrentDay},
			{"last_7d", row.Last7d},
			{"previous_7d", row.Previous7d},
			{"last_14d", row.Last14d},
		} {
			if err := validateCampaignMetricsWindow(fmt.Sprintf("tables.campaign_rollups[%d].%s", i, win.name), win.w); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateCampaignMetricsWindow(path string, w CampaignMetricsWindow) error {
	for _, pair := range []struct {
		name string
		v    float64
	}{
		{"spend", w.Spend},
		{"revenue", w.Revenue},
	} {
		if math.IsNaN(pair.v) || math.IsInf(pair.v, 0) {
			return fmt.Errorf("%s.%s contains NaN/Inf", path, pair.name)
		}
	}
	if w.Revenue == 0 && w.DRRPct != nil {
		return fmt.Errorf("%s.drr_pct must be null when revenue=0", path)
	}
	if w.Spend > 0 && w.Orders == 0 && !w.SpendWithoutOrders {
		return fmt.Errorf("%s.spend_without_orders must be true when spend>0 and orders=0", path)
	}
	if w.Spend > 0 && w.Revenue == 0 && !w.SpendWithoutRevenue {
		return fmt.Errorf("%s.spend_without_revenue must be true when spend>0 and revenue=0", path)
	}
	return nil
}

func validateDataPackJSONSafety(pack *SellerAIDataPack) error {
	raw, err := json.Marshal(pack)
	if err != nil {
		return fmt.Errorf("marshal for safety scan: %w", err)
	}
	lower := strings.ToLower(string(raw))
	for _, needle := range forbiddenSecretSubstrings {
		if strings.Contains(lower, needle) {
			return fmt.Errorf("serialized pack contains forbidden secret field reference %q", needle)
		}
	}
	if err := assertFactRegistryJSONShape(raw); err != nil {
		return err
	}
	return validateForbiddenBusinessKeysInJSON(raw)
}

func assertFactRegistryJSONShape(raw []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("unmarshal root: %w", err)
	}
	frRaw, ok := root["fact_registry"]
	if !ok {
		return fmt.Errorf("fact_registry missing from JSON")
	}
	if len(frRaw) == 0 || frRaw[0] == '[' {
		return fmt.Errorf("fact_registry must be JSON object, not array")
	}
	if frRaw[0] != '{' {
		return fmt.Errorf("fact_registry must be JSON object")
	}
	var frMap map[string]json.RawMessage
	if err := json.Unmarshal(frRaw, &frMap); err != nil {
		return fmt.Errorf("fact_registry must be JSON object: %w", err)
	}
	return nil
}

func validateForbiddenBusinessKeysInJSON(raw []byte) error {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return err
	}
	return walkForbiddenBusinessKeys(root, "")
}

func walkForbiddenBusinessKeys(v any, path string) error {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			kl := strings.ToLower(k)
			for _, forbidden := range forbiddenBusinessKeys {
				if kl == forbidden {
					return fmt.Errorf("forbidden business conclusion key %q at %s", k, path)
				}
			}
			p := path + "." + k
			if path == "" {
				p = k
			}
			if err := walkForbiddenBusinessKeys(child, p); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range x {
			if err := walkForbiddenBusinessKeys(child, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateEntityRegistryKeys(reg EntityRegistry) error {
	for id, e := range reg.SKUs {
		if id != e.EntityID {
			return fmt.Errorf("entity_registry.skus key %q != entity_id %q", id, e.EntityID)
		}
	}
	for id, e := range reg.SKUGroups {
		if id != e.EntityID {
			return fmt.Errorf("entity_registry.sku_groups key %q != entity_id %q", id, e.EntityID)
		}
	}
	for id, e := range reg.Campaigns {
		if id != e.EntityID {
			return fmt.Errorf("entity_registry.campaigns key %q != entity_id %q", id, e.EntityID)
		}
	}
	for id, e := range reg.Warehouses {
		if id != e.EntityID {
			return fmt.Errorf("entity_registry.warehouses key %q != entity_id %q", id, e.EntityID)
		}
	}
	for id, e := range reg.Categories {
		if id != e.EntityID {
			return fmt.Errorf("entity_registry.categories key %q != entity_id %q", id, e.EntityID)
		}
	}
	return nil
}

func validateTableFactRefs(pack *SellerAIDataPack) error {
	check := func(path string, ids []string) error {
		for _, id := range ids {
			if _, ok := pack.FactRegistry[id]; !ok {
				return fmt.Errorf("%s references missing fact_id %q", path, id)
			}
		}
		return nil
	}
	if err := check("tables.account_rollup", pack.Tables.AccountRollup.FactIDs); err != nil {
		return err
	}
	for i, row := range pack.Tables.SKURollups {
		if err := check(fmt.Sprintf("tables.sku_rollups[%d]", i), row.FactIDs); err != nil {
			return err
		}
	}
	for i, row := range pack.Tables.ProductEnrichment {
		if err := check(fmt.Sprintf("tables.product_enrichment[%d]", i), row.FactIDs); err != nil {
			return err
		}
	}
	for i, row := range pack.Tables.StockPositions {
		if err := check(fmt.Sprintf("tables.stock_positions[%d]", i), row.FactIDs); err != nil {
			return err
		}
	}
	for i, row := range pack.Tables.CampaignRollups {
		if err := check(fmt.Sprintf("tables.campaign_rollups[%d]", i), row.FactIDs); err != nil {
			return err
		}
	}
	if err := check("tables.funnel_rollup", pack.Tables.FunnelRollup.FactIDs); err != nil {
		return err
	}
	for i, row := range pack.Tables.CategoryRollups {
		if err := check(fmt.Sprintf("tables.category_rollups[%d]", i), row.FactIDs); err != nil {
			return err
		}
	}
	return nil
}

func validateTableEntityRefs(pack *SellerAIDataPack) error {
	for i, row := range pack.Tables.SKURollups {
		if _, ok := pack.EntityRegistry.SKUs[row.EntityID]; !ok {
			return fmt.Errorf("tables.sku_rollups[%d] references missing sku entity %q", i, row.EntityID)
		}
	}
	for i, row := range pack.Tables.CampaignRollups {
		if _, ok := pack.EntityRegistry.Campaigns[row.EntityID]; !ok {
			return fmt.Errorf("tables.campaign_rollups[%d] references missing campaign entity %q", i, row.EntityID)
		}
	}
	for i, row := range pack.Tables.CampaignProductLinks {
		if _, ok := pack.EntityRegistry.Campaigns[row.CampaignEntityID]; !ok {
			return fmt.Errorf("tables.campaign_product_links[%d] references missing campaign entity %q", i, row.CampaignEntityID)
		}
		if row.SKUEntityID != "" {
			if _, ok := pack.EntityRegistry.SKUs[row.SKUEntityID]; !ok && !row.Partial {
				return fmt.Errorf("tables.campaign_product_links[%d] references missing sku entity %q", i, row.SKUEntityID)
			}
		}
	}
	for i, row := range pack.Tables.CategoryRollups {
		if _, ok := pack.EntityRegistry.Categories[row.EntityID]; !ok {
			return fmt.Errorf("tables.category_rollups[%d] references missing category entity %q", i, row.EntityID)
		}
	}
	return validateCampaignProductLinkDedup(pack.Tables.CampaignProductLinks)
}

func validateCampaignProductLinkDedup(links []CampaignProductLinkRow) error {
	seen := map[string]struct{}{}
	for i, row := range links {
		key := row.CampaignEntityID + "|" + row.SKU
		if _, ok := seen[key]; ok {
			return fmt.Errorf("tables.campaign_product_links[%d] duplicate link %q", i, key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// walkNumericFields scans nested maps/slices for NaN/Inf (used in tests).
func walkNumericFields(v any, path string) error {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map:
		for _, key := range rv.MapKeys() {
			p := path + "." + fmt.Sprint(key.Interface())
			if err := walkNumericFields(rv.MapIndex(key).Interface(), p); err != nil {
				return err
			}
		}
	case reflect.Slice:
		for i := 0; i < rv.Len(); i++ {
			if err := walkNumericFields(rv.Index(i).Interface(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case reflect.Struct:
		for i := 0; i < rv.NumField(); i++ {
			if !rv.Type().Field(i).IsExported() {
				continue
			}
			if err := walkNumericFields(rv.Field(i).Interface(), path+"."+rv.Type().Field(i).Name); err != nil {
				return err
			}
		}
	case reflect.Float32, reflect.Float64:
		f := rv.Convert(reflect.TypeOf(float64(0))).Float()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("%s contains NaN/Inf", path)
		}
	case reflect.Ptr:
		if !rv.IsNil() {
			return walkNumericFields(rv.Elem().Interface(), path)
		}
	}
	return nil
}
