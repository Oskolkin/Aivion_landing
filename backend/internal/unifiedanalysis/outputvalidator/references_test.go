package outputvalidator_test

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
)

func TestBuildPackReferenceIndexUsesSKUIdColumn(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("sku_summary.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("row_id,sku_id,offer_id,title\nr1,901042000001,offer-1,Test\nr2,901042000002,offer-2,Test2\n")); err != nil {
		t.Fatal(err)
	}
	w, err = zw.Create("campaign_summary.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("row_id,campaign_id,name\nc1,882042000001,Camp\n")); err != nil {
		t.Fatal(err)
	}
	w, err = zw.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(`{"pack_id":"pack:test"}`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	idx, err := outputvalidator.BuildPackReferenceIndex(buf.Bytes())
	if err != nil {
		t.Fatalf("BuildPackReferenceIndex: %v", err)
	}
	if !idx.HasSKU("901042000001") || !idx.HasSKU("901042000002") {
		t.Fatalf("expected sku_id values indexed, got %#v", idx.SKUIds)
	}
	if idx.HasSKU("offer-1") {
		t.Fatal("offer_id must not be treated as sku_id unless present in sku_id column")
	}
	if !idx.HasCampaign("882042000001") {
		t.Fatal("expected campaign_id indexed")
	}
	if idx.PackID != "pack:test" {
		t.Fatalf("PackID=%q", idx.PackID)
	}
}
