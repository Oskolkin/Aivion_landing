package actions

import (
	"bytes"
	"context"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestRenderActionExportXLSX_IncludesEntityGroupIDs(t *testing.T) {
	item := ActionItem{
		ActionKey:      "k1",
		Type:           ActionTypeStockReplenishment,
		Category:       ActionCategoryStock,
		Priority:       ActionPriorityCritical,
		Status:         ActionStatusOpen,
		Title:          "title",
		Reason:         "reason",
		NextStep:       "next",
		MetricDate:     "2026-06-24",
		EntityGroupRef: "seed_stock",
		Evidence:       []ActionEvidence{{Kind: "metric", Label: "SKU", Value: "3"}},
		AffectedEntities: ActionAffectedEntities{
			SKUs: []ActionSKURef{{SKU: "100001", Name: "preview"}},
		},
	}
	entityCtx := &ExportEntityContext{
		EntityType: "sku",
		EntityIDs:  []string{"100001", "100002", "100003"},
		Source:     "entity_group",
	}
	data, _, err := renderActionExportXLSX(item, entityCtx)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = f.Close() }()
	for _, id := range []string{"100001", "100002", "100003"} {
		rows, err := f.GetRows("Объекты")
		if err != nil {
			t.Fatalf("rows: %v", err)
		}
		found := false
		for _, row := range rows {
			if len(row) > 1 && row[1] == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("entity id %q not found in objects sheet", id)
		}
	}
}

func TestRenderActionExportCSV_IncludesEntityGroupIDs(t *testing.T) {
	item := ActionItem{
		ActionKey:  "k1",
		Type:       ActionTypeAdHighDRR,
		Category:   ActionCategoryAdvertising,
		Priority:   ActionPriorityWatch,
		Status:     ActionStatusOpen,
		Title:      "title",
		Reason:     "reason",
		NextStep:   "next",
		MetricDate: "2026-06-24",
		Evidence:   []ActionEvidence{{Kind: "metric", Label: "Campaigns", Value: "2"}},
	}
	entityCtx := &ExportEntityContext{
		EntityType: "campaign",
		EntityIDs:  []string{"200001", "200002"},
		Source:     "entity_group",
	}
	data, _, err := renderActionExportCSV(item, entityCtx)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	body := string(data)
	for _, id := range []string{"200001", "200002"} {
		if !bytes.Contains(data, []byte(id)) {
			t.Fatalf("csv missing entity id %q body=%s", id, body)
		}
	}
}

func TestResolveExportEntityContext_MissingGroupGraceful(t *testing.T) {
	repo := &mockRepo{}
	item := ActionItem{
		EntityGroupRef: "missing-group",
		AffectedEntities: ActionAffectedEntities{
			SKUs: []ActionSKURef{{SKU: "100001"}},
		},
	}
	ctx := resolveExportEntityContext(context.Background(), repo, 1, item)
	if ctx == nil || !ctx.MissingFullList || ctx.Warning == "" {
		t.Fatalf("expected graceful warning, got %+v", ctx)
	}
}

func TestSanitizeExportCell_FormulaInjectionOnEntityID(t *testing.T) {
	got := sanitizeExportCell("=evil")
	if got[0] != '\'' {
		t.Fatalf("expected escaped entity id, got %q", got)
	}
}
