package aidatapack

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

type skuMeta struct {
	SKU           string
	OzonProductID string
	OfferID       string
	Name          string
	CategoryID    string
	Status        string
	IsArchived    bool
	ImageURL      *string
	Sources       map[string]struct{}
}

func (m *skuMeta) addSource(s string) {
	if m.Sources == nil {
		m.Sources = make(map[string]struct{})
	}
	m.Sources[s] = struct{}{}
}

func sourcesList(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for s := range m {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func buildKnownSKUSet(ctx *loadContext) map[string]*skuMeta {
	known := make(map[string]*skuMeta)

	upsert := func(sku string, fn func(*skuMeta)) {
		if sku == "" {
			return
		}
		m, ok := known[sku]
		if !ok {
			m = &skuMeta{SKU: sku, Sources: make(map[string]struct{})}
			known[sku] = m
		}
		fn(m)
	}

	for _, p := range ctx.Products {
		sku := int64ToString(p.Sku.Int64)
		if !p.Sku.Valid {
			continue
		}
		upsert(sku, func(m *skuMeta) {
			m.addSource("products")
			if p.OzonProductID > 0 {
				m.OzonProductID = int64ToString(p.OzonProductID)
			}
			if p.OfferID.Valid {
				m.OfferID = p.OfferID.String
			}
			if p.Name != "" {
				m.Name = p.Name
			}
			if p.DescriptionCategoryID.Valid {
				m.CategoryID = categoryEntityID(int64ToString(p.DescriptionCategoryID.Int64))
			}
			if textValue(p.Status) != "" {
				m.Status = textValue(p.Status)
			}
			m.IsArchived = p.IsArchived
			if p.PrimaryImageUrl.Valid {
				u := p.PrimaryImageUrl.String
				m.ImageURL = &u
			}
		})
	}

	for _, row := range ctx.SKUMetrics {
		if !row.Sku.Valid {
			continue
		}
		sku := int64ToString(row.Sku.Int64)
		upsert(sku, func(m *skuMeta) {
			m.addSource("daily_sku_metrics")
			if row.OzonProductID > 0 && m.OzonProductID == "" {
				m.OzonProductID = int64ToString(row.OzonProductID)
			}
			if row.OfferID.Valid && m.OfferID == "" {
				m.OfferID = row.OfferID.String
			}
			if row.ProductName.Valid && m.Name == "" {
				m.Name = row.ProductName.String
			}
		})
	}

	for _, snap := range ctx.ProductInfoSnapshots {
		sku := int64ToString(snap.Sku.Int64)
		if !snap.Sku.Valid {
			continue
		}
		upsert(sku, func(m *skuMeta) {
			m.addSource("ozon_product_info_snapshots")
			if snap.Name.Valid && snap.Name.String != "" {
				m.Name = snap.Name.String
			}
			if snap.ProductID.Valid && m.OzonProductID == "" {
				m.OzonProductID = int64ToString(snap.ProductID.Int64)
			}
			if snap.OfferID.Valid && m.OfferID == "" {
				m.OfferID = snap.OfferID.String
			}
			if snap.DescriptionCategoryID.Valid && m.CategoryID == "" {
				m.CategoryID = categoryEntityID(int64ToString(snap.DescriptionCategoryID.Int64))
			}
			if snap.Status.Valid {
				m.Status = snap.Status.String
			}
			if snap.IsArchived.Valid {
				m.IsArchived = snap.IsArchived.Bool
			}
			if snap.PrimaryImage.Valid && m.ImageURL == nil {
				u := snap.PrimaryImage.String
				m.ImageURL = &u
			}
		})
	}

	for _, link := range ctx.CampaignSKUs {
		if !link.Sku.Valid {
			continue
		}
		sku := int64ToString(link.Sku.Int64)
		upsert(sku, func(m *skuMeta) {
			m.addSource("ad_campaign_skus")
			if link.ProductTitle.Valid && m.Name == "" {
				m.Name = link.ProductTitle.String
			}
			if link.OfferID.Valid && m.OfferID == "" {
				m.OfferID = link.OfferID.String
			}
		})
	}

	for _, row := range ctx.StockSnapshots {
		sku := int64ToString(row.Sku)
		upsert(sku, func(m *skuMeta) {
			m.addSource("stock_analytics_snapshots")
			if row.OfferID.Valid && m.OfferID == "" {
				m.OfferID = row.OfferID.String
			}
			if row.ProductName.Valid && m.Name == "" {
				m.Name = row.ProductName.String
			}
		})
	}

	for _, m := range known {
		if m.Name == "" {
			m.Name = "SKU " + m.SKU
		}
	}
	return known
}

func buildSKUEntities(known map[string]*skuMeta) map[string]SKUEntity {
	out := make(map[string]SKUEntity, len(known))
	for sku, m := range known {
		eid := skuEntityID(sku)
		out[eid] = SKUEntity{
			EntityID:      eid,
			EntityType:    "sku",
			SKU:           sku,
			OzonProductID: m.OzonProductID,
			OfferID:       m.OfferID,
			Name:          m.Name,
			CategoryID:    m.CategoryID,
			Status:        m.Status,
			IsArchived:    m.IsArchived,
			ImageURL:      m.ImageURL,
			Sources:       sourcesList(m.Sources),
		}
	}
	return out
}

func buildSKUGroupEntities(ctx *loadContext, knownSKUs map[string]struct{}) map[string]SKUGroupEntity {
	groups := make(map[string][]dbgen.OzonRelatedSku)
	for _, row := range ctx.RelatedSKUs {
		pid := ""
		if row.ProductID.Valid {
			pid = int64ToString(row.ProductID.Int64)
		} else {
			pid = int64ToString(row.RequestedSku)
		}
		groups[pid] = append(groups[pid], row)
	}

	out := make(map[string]SKUGroupEntity)
	for pid, rows := range groups {
		eid := skuGroupEntityID(pid)
		primary := int64ToString(rows[0].RequestedSku)
		related := make([]RelatedSKUEntry, 0, len(rows))
		seen := map[int64]struct{}{}
		for _, row := range rows {
			if _, ok := seen[row.RelatedSku]; ok {
				continue
			}
			seen[row.RelatedSku] = struct{}{}
			skuStr := int64ToString(row.RelatedSku)
			var deleted *string
			if row.DeletedAt.Valid {
				s := row.DeletedAt.Time.UTC().Format("2006-01-02T15:04:05Z")
				deleted = &s
			}
			_, isKnown := knownSKUs[skuStr]
			related = append(related, RelatedSKUEntry{
				SKU:               skuStr,
				DeliverySchema:    textValue(row.DeliverySchema),
				Availability:      textValue(row.Availability),
				DeletedAt:         deleted,
				IsKnownProductSKU: isKnown,
			})
		}
		out[eid] = SKUGroupEntity{
			EntityID:    eid,
			EntityType:  "sku_group",
			ProductID:   pid,
			PrimarySKU:  primary,
			RelatedSKUs: related,
		}
	}
	return out
}

func buildCampaignEntities(ctx *loadContext) map[string]CampaignEntity {
	out := make(map[string]CampaignEntity)
	for _, c := range ctx.Campaigns {
		extID := int64ToString(c.CampaignExternalID)
		eid := campaignEntityID(extID)
		var placement []string
		if len(c.PlacementJson) > 0 {
			_ = json.Unmarshal(c.PlacementJson, &placement)
		}
		budget := CampaignBudget{Currency: "RUB"}
		if c.BudgetDaily.Valid {
			v := numericToFloat(c.BudgetDaily)
			budget.DailyAmount = &v
		}
		if c.BudgetWeekly.Valid {
			v := numericToFloat(c.BudgetWeekly)
			budget.WeeklyAmount = &v
		}
		if c.BudgetAmount.Valid {
			v := numericToFloat(c.BudgetAmount)
			budget.TotalAmount = &v
		}
		out[eid] = CampaignEntity{
			EntityID:                 eid,
			EntityType:               "campaign",
			CampaignExternalID:       extID,
			Name:                     c.CampaignName,
			State:                    textValue(c.Status),
			PaymentType:              textValue(c.PaymentType),
			AdvObjectType:            textValue(c.CampaignType),
			Placement:                placement,
			ProductCampaignMode:      textValue(c.ProductCampaignMode),
			ProductAutopilotStrategy: textValue(c.ProductAutopilotStrategy),
			Budget:                   budget,
		}
	}
	return out
}

func buildWarehouseEntities(ctx *loadContext) map[string]WarehouseEntity {
	type wh struct {
		id   int64
		name string
	}
	seen := map[int64]wh{}
	for _, row := range ctx.StockSnapshots {
		if row.WarehouseID <= 0 {
			continue
		}
		if _, ok := seen[row.WarehouseID]; !ok {
			seen[row.WarehouseID] = wh{id: row.WarehouseID, name: textValue(row.WarehouseName)}
		}
	}
	out := make(map[string]WarehouseEntity, len(seen))
	for _, w := range seen {
		id := int64ToString(w.id)
		eid := warehouseEntityID(id)
		out[eid] = WarehouseEntity{
			EntityID:    eid,
			EntityType:  "warehouse",
			WarehouseID: id,
			Name:        w.name,
		}
	}
	return out
}

func buildCategoryEntities(ctx *loadContext, known map[string]*skuMeta) map[string]CategoryEntity {
	needed := map[int64]struct{}{}
	for _, m := range known {
		if m.CategoryID == "" {
			continue
		}
		idStr := strings.TrimPrefix(m.CategoryID, "category:")
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			needed[id] = struct{}{}
		}
	}
	for _, snap := range ctx.ProductInfoSnapshots {
		if snap.DescriptionCategoryID.Valid {
			needed[snap.DescriptionCategoryID.Int64] = struct{}{}
		}
	}
	for _, p := range ctx.Products {
		if p.DescriptionCategoryID.Valid {
			needed[p.DescriptionCategoryID.Int64] = struct{}{}
		}
	}

	catByID := make(map[int64]dbgen.DescriptionCategory)
	for _, c := range ctx.Categories {
		catByID[c.DescriptionCategoryID] = c
	}

	out := make(map[string]CategoryEntity)
	for id := range needed {
		idStr := int64ToString(id)
		eid := categoryEntityID(idStr)
		cat := catByID[id]
		name := resolveCategoryDisplayName(idStr, cat)
		var path *string
		if cat.Path.Valid {
			p := cat.Path.String
			path = &p
		}
		out[eid] = CategoryEntity{
			EntityID:   eid,
			EntityType: "category",
			CategoryID: idStr,
			Name:       name,
			Path:       path,
		}
	}
	return out
}
