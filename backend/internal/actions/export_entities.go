package actions

import (
	"fmt"
	"strings"
)

type exportObjectRow struct {
	ObjectType string
	ObjectID   string
	Name       string
	OfferID    string
	Metric     string
	Source     string
}

func buildExportObjectRows(item ActionItem, entityCtx *ExportEntityContext) []exportObjectRow {
	if entityCtx != nil && len(entityCtx.EntityIDs) > 0 && entityCtx.Source == "entity_group" {
		rows := make([]exportObjectRow, 0, len(entityCtx.EntityIDs))
		for _, id := range entityCtx.EntityIDs {
			row := exportObjectRow{
				ObjectType: entityCtx.EntityType,
				ObjectID:   id,
				Source:     "entity_group",
			}
			enrichExportObjectRowFromSummary(&row, entityCtx.Summary, id)
			rows = append(rows, row)
		}
		return rows
	}
	return buildExportObjectRowsFromPreview(item)
}

func buildExportObjectRowsFromPreview(item ActionItem) []exportObjectRow {
	var rows []exportObjectRow
	for _, sku := range item.AffectedEntities.SKUs {
		rows = append(rows, exportObjectRow{
			ObjectType: "sku",
			ObjectID:   sku.SKU,
			Name:       sku.Name,
			OfferID:    sku.OfferID,
			Source:     "affected_entities",
		})
	}
	for _, c := range item.AffectedEntities.Campaigns {
		rows = append(rows, exportObjectRow{
			ObjectType: "campaign",
			ObjectID:   c.CampaignID,
			Name:       c.Name,
			Source:     "affected_entities",
		})
	}
	if item.AffectedEntities.Funnel != nil {
		f := item.AffectedEntities.Funnel
		rows = append(rows, exportObjectRow{
			ObjectType: "funnel_transition",
			ObjectID:   f.TransitionKey,
			Name:       strings.TrimSpace(f.FromStage + " -> " + f.ToStage),
			Source:     "affected_entities",
		})
	}
	return rows
}

func enrichExportObjectRowFromSummary(row *exportObjectRow, summary map[string]any, entityID string) {
	if summary == nil {
		return
	}
	if entities, ok := summary["entities"].([]any); ok {
		for _, raw := range entities {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id := strings.TrimSpace(fmt.Sprint(m["id"]))
			if id == "" {
				id = strings.TrimSpace(fmt.Sprint(m["sku"]))
			}
			if id == "" {
				id = strings.TrimSpace(fmt.Sprint(m["campaign_id"]))
			}
			if id != entityID {
				continue
			}
			if name, ok := m["name"].(string); ok {
				row.Name = name
			}
			if offer, ok := m["offer_id"].(string); ok {
				row.OfferID = offer
			}
			if metric, ok := m["metric"].(string); ok {
				row.Metric = metric
			} else if value, ok := m["value"].(string); ok {
				row.Metric = value
			}
			return
		}
	}
	if name, ok := summary[entityID+"_name"].(string); ok {
		row.Name = name
	}
	if offer, ok := summary[entityID+"_offer_id"].(string); ok {
		row.OfferID = offer
	}
	if metric, ok := summary[entityID+"_metric"].(string); ok {
		row.Metric = metric
	}
}
