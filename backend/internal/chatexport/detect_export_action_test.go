package chatexport

import (
	"strings"
	"testing"
)

func TestExportActionNotTriggeredForShowQuestions(t *testing.T) {
	showOnly := []string{
		"Покажи товары с риском остатка",
		"Покажи рекламу",
		"Какие товары скоро закончатся?",
	}
	for _, q := range showOnly {
		n := normalizeDetectText(q)
		if HasExportAction(n) {
			t.Fatalf("%q: should not be export action", q)
		}
		if p := ParseExportRequest(q, nil, nil); p.ExportType != "" || p.Reason != "" {
			t.Fatalf("%q: unexpected parse %+v", q, p)
		}
	}
}

func TestExportActionTriggeredForExplicitFileRequests(t *testing.T) {
	cases := []struct {
		q          string
		exportType string
	}{
		{"Выгрузи товары с риском остатка в Excel", TypeStockRisks},
		{"Сделай CSV по рекламе", TypeAdvertisingCampaigns},
	}
	for _, tc := range cases {
		p := ParseExportRequest(tc.q, nil, nil)
		if p.Reason != "" {
			t.Fatalf("%q: reason=%s", tc.q, p.Reason)
		}
		if p.ExportType != tc.exportType {
			t.Fatalf("%q: type=%s", tc.q, p.ExportType)
		}
	}
}

func TestAmbiguousProductExportClarification(t *testing.T) {
	for _, q := range []string{
		"Выгрузи проблемные товары",
		"Сделай файл по товарам",
	} {
		p := ParseExportRequest(q, nil, nil)
		if p.Reason != ReasonExportClarification {
			t.Fatalf("%q: reason=%s type=%s", q, p.Reason, p.ExportType)
		}
	}
}

func TestSuccessMessageNoData(t *testing.T) {
	msg := SuccessMessage(TypeStockRisks, FormatXLSX, 0, 7)
	if !strings.Contains(msg, "нет") || !strings.Contains(msg, "заголовк") {
		t.Fatalf("message=%q", msg)
	}
}
