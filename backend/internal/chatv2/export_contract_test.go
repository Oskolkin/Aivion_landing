package chatv2

import (
	"errors"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestPrompt153ExportSchemaMatchesResolver(t *testing.T) {
	for _, spec := range CatalogV1() {
		if spec.Key != ToolSubmitTurnResult {
			continue
		}
		props := spec.Schema["properties"].(map[string]any)
		ds := props["export_dataset"].(map[string]any)
		dsProps := ds["properties"].(map[string]any)
		kind := dsProps["kind"].(map[string]any)
		for _, v := range kind["enum"].([]any) {
			if v == nil {
				continue
			}
			s := v.(string)
			if _, ok := supportedExportKinds[s]; !ok {
				t.Errorf("schema kind %q not in resolver", s)
			}
		}
		period := dsProps["period"].(map[string]any)
		for _, v := range period["enum"].([]any) {
			if v == nil {
				continue
			}
			s := strings.ToLower(v.(string))
			if _, ok := supportedExportPeriods[s]; !ok {
				t.Errorf("schema period %q not in resolver", s)
			}
		}
		filters := dsProps["filters"].(map[string]any)
		items := filters["items"].(map[string]any)
		fprops := items["properties"].(map[string]any)
		field := fprops["field"].(map[string]any)
		enum, ok := schemaEnumValues(field["enum"])
		if !ok {
			t.Fatal("filter field enum missing")
		}
		for _, v := range enum {
			s, _ := v.(string)
			if s == "" {
				continue
			}
			if _, ok := supportedExportFilterFields[s]; !ok {
				t.Errorf("schema filter field %q not in resolver", s)
			}
		}
	}
}

func TestPrompt153UnsupportedExportPeriodFailsClosed(t *testing.T) {
	_, err := exportPeriod("last_14d")
	if !errors.Is(err, ErrUnsupportedExportPeriod) {
		t.Fatalf("err=%v", err)
	}
	std, err := exportPeriod("")
	if err != nil {
		t.Fatal(err)
	}
	if std != factualapi.PeriodCurrent7D {
		t.Fatalf("default period=%s", std)
	}
}
