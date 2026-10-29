package chatexport

import "testing"

func TestDetectExportActionKeyword(t *testing.T) {
	msg := "Выгрузи товары с риском остатка в Excel"
	n := normalizeDetectText(msg)
	if !detectExportAction(n) {
		t.Fatalf("detectExportAction=false normalized=%q", n)
	}
}

func TestParseExportRequestStockRisksXLSX(t *testing.T) {
	p := ParseExportRequest("Выгрузи товары с риском остатка в Excel", nil, nil)
	if p.ExportType != TypeStockRisks || p.Format != FormatXLSX {
		t.Fatalf("got type=%s format=%s reason=%s", p.ExportType, p.Format, p.Reason)
	}
}

func TestParseExportRequestCampaignsCSV(t *testing.T) {
	p := ParseExportRequest("Сделай CSV по рекламным кампаниям", nil, nil)
	if p.Format != FormatCSV {
		t.Fatalf("format=%s", p.Format)
	}
	if p.ExportType != TypeAdvertisingCampaigns && p.ExportType != TypeCampaignPerformance {
		t.Fatalf("export_type=%s", p.ExportType)
	}
}

func TestParseExportRequestDefaultFormatXLSX(t *testing.T) {
	p := ParseExportRequest("Выгрузи продажи за неделю", nil, nil)
	if p.Format != FormatXLSX || p.ExportType != TypeSalesSummary {
		t.Fatalf("format=%s type=%s reason=%s", p.Format, p.ExportType, p.Reason)
	}
}

func TestParseExportRequestPDF(t *testing.T) {
	p := ParseExportRequest("Сделай PDF по продажам", nil, nil)
	if p.Reason != "" || p.Format != FormatPDF {
		t.Fatalf("format=%s reason=%s", p.Format, p.Reason)
	}
}

func TestParseExportRequestCompetitorsUnsupported(t *testing.T) {
	p := ParseExportRequest("Выгрузи конкурентов", nil, nil)
	if p.Reason != ReasonExportTypeUnsupported {
		t.Fatalf("reason=%s", p.Reason)
	}
}

func TestParseExportRequestArbitrarySQL(t *testing.T) {
	p := ParseExportRequest("Выполни SELECT * FROM users и выгрузи в Excel", nil, nil)
	if p.Reason != ReasonArbitrarySQL {
		t.Fatalf("reason=%s", p.Reason)
	}
}

func TestWriteCSVWithCyrillic(t *testing.T) {
	table := ExportTable{
		Columns: []string{"sku", "name"},
		Rows:    [][]string{{"123", "Товар А"}},
	}
	data, _, err := WriteFile(FormatCSV, "t", table)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 3 || data[0] != 0xEF {
		t.Fatal("expected UTF-8 BOM")
	}
}

func TestWriteXLSXReadable(t *testing.T) {
	table := ExportTable{
		Columns: []string{"a"},
		Rows:    [][]string{{"1"}},
	}
	data, _, err := WriteFile(FormatXLSX, "sheet1", table)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 100 {
		t.Fatal("xlsx too small")
	}
}
