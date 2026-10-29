package aidatapack

import (
	"encoding/json"
)

func buildProductEnrichment(b *Builder, ctx *loadContext, known map[string]*skuMeta) []ProductEnrichmentRow {
	bySKU := map[string]ProductEnrichmentRow{}
	w := b.Windows()
	snapshotDate := w.CurrentDay
	source := ctx.EnrichmentSource
	if source == "" {
		source = "unknown"
	}

	for _, snap := range ctx.ProductInfoSnapshots {
		if !snap.Sku.Valid {
			continue
		}
		sku := int64ToString(snap.Sku.Int64)
		row := ProductEnrichmentRow{
			EntityID:     skuEntityID(sku),
			SnapshotDate: snapshotDate,
			DataStatus:   "available",
			Price:        numericToFloatPtr(snap.Price),
			OldPrice:     numericToFloatPtr(snap.OldPrice),
			MinPrice:     numericToFloatPtr(snap.MinPrice),
			Currency:     textValue(snap.CurrencyCode),
			Status: map[string]any{
				"is_archived":       snap.IsArchived.Bool,
				"is_autoarchived":   snap.IsAutoarchived.Bool,
				"status":            textValue(snap.Status),
				"status_name":       textValue(snap.StatusName),
				"moderate_status":   textValue(snap.ModerateStatus),
				"validation_status": textValue(snap.ValidationStatus),
			},
			Visibility: map[string]any{
				"has_price": snap.VisibilityHasPrice.Bool,
				"has_stock": snap.VisibilityHasStock.Bool,
			},
			PriceIndexes: map[string]any{
				"color":                      textValue(snap.PriceIndexColor),
				"external_minimal_price":     nullableFloat(numericToFloatPtr(snap.ExternalMinimalPrice)),
				"external_price_index_value": nullableFloat(numericToFloatPtr(snap.ExternalPriceIndexValue)),
				"ozon_minimal_price":         nullableFloat(numericToFloatPtr(snap.OzonMinimalPrice)),
				"ozon_price_index_value":     nullableFloat(numericToFloatPtr(snap.OzonPriceIndexValue)),
			},
			Commissions: parseJSONArray(snap.CommissionsJson),
			Promotions:  parseJSONArray(snap.PromotionsJson),
			Errors:      parseJSONArray(snap.ErrorsJson),
			Source:      source,
		}
		if row.Currency == "" {
			row.Currency = "RUB"
		}
		bySKU[sku] = row
	}

	rows := make([]ProductEnrichmentRow, 0, len(known))
	for sku := range known {
		row, ok := bySKU[sku]
		if !ok {
			row = ProductEnrichmentRow{
				EntityID:     skuEntityID(sku),
				SnapshotDate: snapshotDate,
				DataStatus:   "not_available",
				Commissions:  []any{},
				Promotions:   []any{},
				Errors:       []any{},
			}
			rows = append(rows, row)
			continue
		}

		var factIDs []string
		if row.Price != nil {
			if id, err := b.AddFact(Fact{
				FactID: "sku." + sku + ".product.price", EntityType: "sku", EntityID: skuEntityID(sku),
				Metric: "price", Date: snapshotDate, Value: *row.Price, Unit: UnitRUB,
				Source: FactSource{Table: "ozon_product_info_snapshots", Field: "price"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if color, ok := row.PriceIndexes["color"].(string); ok && color != "" {
			if id, err := b.AddFact(Fact{
				FactID: "sku." + sku + ".product.price_index_color", EntityType: "sku", EntityID: skuEntityID(sku),
				Metric: "price_index_color", Date: snapshotDate, Value: color, Unit: UnitEnum,
				Source: FactSource{Table: "ozon_product_info_snapshots", Field: "price_index_color"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if hasStock, ok := row.Visibility["has_stock"].(bool); ok {
			if id, err := b.AddFact(Fact{
				FactID: "sku." + sku + ".product.visibility_has_stock", EntityType: "sku", EntityID: skuEntityID(sku),
				Metric: "visibility_has_stock", Date: snapshotDate, Value: hasStock, Unit: UnitBoolean,
				Source: FactSource{Table: "ozon_product_info_snapshots", Field: "visibility_has_stock"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if hasPrice, ok := row.Visibility["has_price"].(bool); ok {
			if id, err := b.AddFact(Fact{
				FactID: "sku." + sku + ".product.visibility_has_price", EntityType: "sku", EntityID: skuEntityID(sku),
				Metric: "visibility_has_price", Date: snapshotDate, Value: hasPrice, Unit: UnitBoolean,
				Source: FactSource{Table: "ozon_product_info_snapshots", Field: "visibility_has_price"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if vs, ok := row.Status["validation_status"].(string); ok && vs != "" {
			if id, err := b.AddFact(Fact{
				FactID: "sku." + sku + ".product.validation_status", EntityType: "sku", EntityID: skuEntityID(sku),
				Metric: "validation_status", Date: snapshotDate, Value: vs, Unit: UnitEnum,
				Source: FactSource{Table: "ozon_product_info_snapshots", Field: "validation_status"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		row.FactIDs = factIDs
		rows = append(rows, row)
	}
	return rows
}

func parseJSONArray(raw []byte) []any {
	if len(raw) == 0 {
		return []any{}
	}
	var out []any
	if err := json.Unmarshal(raw, &out); err != nil {
		return []any{}
	}
	return out
}
