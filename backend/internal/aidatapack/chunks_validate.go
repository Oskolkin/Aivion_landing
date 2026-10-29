package aidatapack

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

var forbiddenBusinessSelectionPolicies = []string{
	"top_revenue",
	"top_losses",
	"high_drr",
	"deficit_only",
	"problematic_only",
	"important_only",
}

// ValidateChunkSet checks structural integrity and full table coverage.
func ValidateChunkSet(pack *SellerAIDataPack, chunks *ChunkSet) error {
	if pack == nil {
		return fmt.Errorf("pack is nil")
	}
	if chunks == nil {
		return fmt.Errorf("chunk set is nil")
	}
	if strings.TrimSpace(chunks.SchemaVersion) == "" {
		return fmt.Errorf("schema_version is empty")
	}
	if chunks.SellerAccountID != pack.SellerContext.SellerAccountID {
		return fmt.Errorf("seller_account_id mismatch: chunk=%d pack=%d", chunks.SellerAccountID, pack.SellerContext.SellerAccountID)
	}
	if chunks.MetricDate != pack.SellerContext.MetricDate {
		return fmt.Errorf("metric_date mismatch: chunk=%s pack=%s", chunks.MetricDate, pack.SellerContext.MetricDate)
	}
	if chunks.ChunkingPolicy.SelectionPolicy != SelectionPolicyTechnical {
		return fmt.Errorf("invalid selection_policy: %s", chunks.ChunkingPolicy.SelectionPolicy)
	}
	if chunks.ChunkingPolicy.BusinessFilteringAllowed {
		return fmt.Errorf("business_filtering_allowed must be false")
	}
	for _, banned := range forbiddenBusinessSelectionPolicies {
		if strings.Contains(strings.ToLower(chunks.ChunkingPolicy.SelectionPolicy), banned) {
			return fmt.Errorf("forbidden business selection policy: %s", chunks.ChunkingPolicy.SelectionPolicy)
		}
	}

	seenIDs := map[string]struct{}{}
	for i, ch := range chunks.Chunks {
		if err := validateChunk(pack, ch); err != nil {
			return fmt.Errorf("chunk[%d] %s: %w", i, ch.ChunkID, err)
		}
		if _, ok := seenIDs[ch.ChunkID]; ok {
			return fmt.Errorf("duplicate chunk_id %q", ch.ChunkID)
		}
		seenIDs[ch.ChunkID] = struct{}{}
		if err := validateChunkHashStable(ch); err != nil {
			return fmt.Errorf("chunk %s: %w", ch.ChunkID, err)
		}
	}

	if err := validateChunkCoverage(chunks.Coverage); err != nil {
		return err
	}
	return validateChunksNoSecrets(chunks)
}

func validateChunk(pack *SellerAIDataPack, ch DataPackChunk) error {
	if ch.SelectionPolicy != SelectionPolicyTechnical {
		return fmt.Errorf("selection_policy must be %s, got %s", SelectionPolicyTechnical, ch.SelectionPolicy)
	}
	for _, banned := range forbiddenBusinessSelectionPolicies {
		if strings.Contains(strings.ToLower(ch.SelectionPolicy), banned) {
			return fmt.Errorf("forbidden selection policy: %s", ch.SelectionPolicy)
		}
	}
	if ch.Ordering != OrderingPolicyStableAsc {
		return fmt.Errorf("ordering must be %s", OrderingPolicyStableAsc)
	}

	allowedFacts := map[string]struct{}{}
	for id, cf := range ch.LocalFactCatalog {
		if id != cf.FactID {
			return fmt.Errorf("local_fact_catalog key mismatch: %s vs %s", id, cf.FactID)
		}
		if _, ok := pack.FactRegistry[id]; !ok {
			return fmt.Errorf("unknown fact_id in local_fact_catalog: %s", id)
		}
		allowedFacts[id] = struct{}{}
	}

	for id, ce := range ch.LocalEntityCatalog {
		if id != ce.EntityID {
			return fmt.Errorf("local_entity_catalog key mismatch")
		}
		if !entityExistsInPack(pack, id) {
			return fmt.Errorf("unknown entity_id in local_entity_catalog: %s", id)
		}
	}

	for _, fid := range collectChunkRowFactIDs(ch) {
		if _, ok := allowedFacts[fid]; !ok {
			if _, ok := pack.FactRegistry[fid]; !ok {
				return fmt.Errorf("row references unknown fact_id: %s", fid)
			}
		}
	}
	for _, eid := range collectChunkRowEntityIDs(ch) {
		if !entityExistsInPack(pack, eid) {
			return fmt.Errorf("row references unknown entity_id: %s", eid)
		}
	}
	return nil
}

func validateChunkHashStable(ch DataPackChunk) error {
	if ch.ChunkHash == "" {
		return fmt.Errorf("chunk_hash is empty")
	}
	copy := ch
	copy.ChunkHash = ""
	copy.ApproxTokens = 0
	raw, err := json.Marshal(copy)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	want := hex.EncodeToString(sum[:])
	if want != ch.ChunkHash {
		return fmt.Errorf("chunk_hash mismatch")
	}
	return nil
}

func validateChunkCoverage(cov ChunkCoverage) error {
	type named struct {
		name string
		c    ChunkTableCoverage
	}
	checks := []named{
		{"sku_rollups", cov.SKURollups},
		{"stock_positions", cov.StockPositions},
		{"product_enrichment", cov.ProductEnrichment},
		{"campaign_rollups", cov.CampaignRollups},
		{"campaign_product_links", cov.CampaignProductLinks},
		{"category_rollups", cov.CategoryRollups},
		{"sku_entities", cov.SKUEntities},
		{"sku_groups", cov.SKUGroups},
	}
	for _, item := range checks {
		if item.c.MissingCount > 0 {
			return fmt.Errorf("%s missing_count=%d", item.name, item.c.MissingCount)
		}
		if item.c.DuplicateCount > 0 {
			return fmt.Errorf("%s duplicate_count=%d", item.name, item.c.DuplicateCount)
		}
		if item.c.SourceCount > 0 && item.c.ChunkedCount != item.c.SourceCount {
			return fmt.Errorf("%s chunked_count=%d source_count=%d", item.name, item.c.ChunkedCount, item.c.SourceCount)
		}
	}
	return nil
}

func validateChunksNoSecrets(chunks *ChunkSet) error {
	raw, err := json.Marshal(chunks)
	if err != nil {
		return err
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range forbiddenSecretSubstrings {
		if strings.Contains(lower, forbidden) {
			return fmt.Errorf("chunk set contains forbidden secret substring: %s", forbidden)
		}
	}
	for _, forbidden := range []string{"raw_response_json", "raw_attributes"} {
		if strings.Contains(lower, forbidden) {
			return fmt.Errorf("chunk set contains forbidden raw payload key: %s", forbidden)
		}
	}
	return nil
}

func entityExistsInPack(pack *SellerAIDataPack, entityID string) bool {
	switch entityID {
	case "account:seller", "funnel:account":
		return true
	}
	if _, ok := pack.EntityRegistry.SKUs[entityID]; ok {
		return true
	}
	if _, ok := pack.EntityRegistry.Campaigns[entityID]; ok {
		return true
	}
	if _, ok := pack.EntityRegistry.Categories[entityID]; ok {
		return true
	}
	if _, ok := pack.EntityRegistry.Warehouses[entityID]; ok {
		return true
	}
	if _, ok := pack.EntityRegistry.SKUGroups[entityID]; ok {
		return true
	}
	return false
}

func collectChunkRowFactIDs(ch DataPackChunk) []string {
	var ids []string
	if ch.Rows == nil {
		return ids
	}
	rv := reflect.ValueOf(ch.Rows)
	if rv.Kind() != reflect.Slice {
		return ids
	}
	for i := 0; i < rv.Len(); i++ {
		row := rv.Index(i)
		if row.Kind() == reflect.Ptr {
			row = row.Elem()
		}
		if row.Kind() != reflect.Struct {
			continue
		}
		f := row.FieldByName("FactIDs")
		if f.IsValid() && f.Kind() == reflect.Slice {
			for j := 0; j < f.Len(); j++ {
				ids = append(ids, f.Index(j).String())
			}
		}
	}
	return compactStringIDs(ids)
}

func collectChunkRowEntityIDs(ch DataPackChunk) []string {
	var ids []string
	if ch.Rows == nil {
		return ids
	}
	rv := reflect.ValueOf(ch.Rows)
	if rv.Kind() != reflect.Slice {
		return ids
	}
	for i := 0; i < rv.Len(); i++ {
		row := rv.Index(i)
		if row.Kind() == reflect.Ptr {
			row = row.Elem()
		}
		if row.Kind() != reflect.Struct {
			continue
		}
		for _, name := range []string{"EntityID", "SKUEntityID", "CampaignEntityID"} {
			f := row.FieldByName(name)
			if f.IsValid() && f.Kind() == reflect.String && f.String() != "" {
				ids = append(ids, f.String())
			}
		}
	}
	return compactStringIDs(ids)
}
