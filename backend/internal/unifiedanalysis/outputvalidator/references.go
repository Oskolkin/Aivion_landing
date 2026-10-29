package outputvalidator

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// PackReferenceIndex indexes identifiers from the exact Analysis Pack ZIP used by a run.
type PackReferenceIndex struct {
	PackID string

	SKUIds          map[string]struct{}
	CampaignIds     map[string]struct{}
	LimitationCodes map[string]struct{}
	RowIDs          map[string]struct{}
	// CategoryIds is the set of non-empty sku_summary.csv category_id
	// values observed in the pack. Used to decide whether AI output's
	// category_summaries completeness rule applies (never invented by the
	// backend — it only reflects what the Pack actually contains).
	CategoryIds map[string]struct{}
	// HasCampaignsAny is true when campaign_summary.csv has at least one
	// row (regardless of whether the row parsed a non-empty campaign_id),
	// i.e. the Pack genuinely carries advertising/campaign data. Used to
	// decide whether the AI's ad_efficiency.narrative.checklist
	// completeness rule applies.
	HasCampaignsAny bool
	// HasSKUsAny is true when sku_summary.csv has at least one data row.
	HasSKUsAny bool
	// accountMetrics indexes account_summary.csv numeric values for claim grounding.
	accountMetrics map[string]AccountMetricFact
	// SKUFacts/CampaignFacts index full sku_summary.csv/campaign_summary.csv
	// rows (keyed by bare sku_id/campaign_id) for deterministic material-domain
	// backfill (see ensure_domain_actions.go). Nil/empty when the pack lacks
	// the corresponding table — callers must treat that as "no candidates".
	SKUFacts      map[string]SKUFactRow
	CampaignFacts map[string]CampaignFactRow
}

// HasSKU reports whether sku exists in the pack.
func (idx *PackReferenceIndex) HasSKU(id string) bool {
	if idx == nil {
		return true
	}
	return hasIndexedID(idx.SKUIds, id, "sku:")
}

// HasCampaign reports whether campaign exists in the pack.
func (idx *PackReferenceIndex) HasCampaign(id string) bool {
	if idx == nil {
		return true
	}
	return hasIndexedID(idx.CampaignIds, id, "campaign:")
}

func hasIndexedID(ids map[string]struct{}, id, prefix string) bool {
	id = strings.TrimSpace(id)
	if id == "" || ids == nil {
		return false
	}
	if _, ok := ids[id]; ok {
		return true
	}
	if strings.HasPrefix(id, prefix) {
		bare := strings.TrimPrefix(id, prefix)
		if _, ok := ids[bare]; ok {
			return true
		}
	} else if _, ok := ids[prefix+id]; ok {
		return true
	}
	return false
}

// HasLimitation reports whether limitation code exists in pack limitations.json or allowlist.
func (idx *PackReferenceIndex) HasLimitation(code string) bool {
	if idx == nil {
		return true
	}
	_, ok := idx.LimitationCodes[strings.TrimSpace(code)]
	return ok
}

// HasCategories reports whether the pack carries any category_id values.
// A nil index is treated as "unknown" and never blocks (returns false, so
// checklist/category presence rules degrade gracefully rather than
// spuriously rejecting output when reference indexing wasn't available).
func (idx *PackReferenceIndex) HasCategories() bool {
	if idx == nil {
		return false
	}
	return len(idx.CategoryIds) > 0
}

// HasSKUData reports whether the pack carries any SKU summary rows.
func (idx *PackReferenceIndex) HasSKUData() bool {
	if idx == nil {
		return false
	}
	return idx.HasSKUsAny || len(idx.SKUIds) > 0
}

// HasAdvertisingData reports whether the pack carries any campaign data.
func (idx *PackReferenceIndex) HasAdvertisingData() bool {
	if idx == nil {
		return false
	}
	return idx.HasCampaignsAny
}

// HasContentScoreEvidence reports whether the Pack exposes usable product
// content-quality signals. Today Pack always ships null content scores and
// the product_content_score_unavailable limitation, so this returns false
// unless that limitation is absent (future enrichment).
func (idx *PackReferenceIndex) HasContentScoreEvidence() bool {
	if idx == nil {
		return false
	}
	if idx.HasLimitation("product_content_score_unavailable") {
		return false
	}
	// Without an explicit content limitation, still require a future positive
	// signal; current packs never provide one.
	return false
}

// BuildPackReferenceIndex indexes SKU/campaign/limitation identifiers from pack ZIP bytes.
func BuildPackReferenceIndex(zipBytes []byte) (*PackReferenceIndex, error) {
	idx := &PackReferenceIndex{
		SKUIds:          make(map[string]struct{}),
		CampaignIds:     make(map[string]struct{}),
		LimitationCodes: make(map[string]struct{}),
		RowIDs:          make(map[string]struct{}),
		CategoryIds:     make(map[string]struct{}),
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("open pack zip: %w", err)
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	if err := indexManifestPackID(idx, files["manifest.json"]); err != nil {
		return nil, err
	}
	if err := indexCSVColumn(idx.SKUIds, files["sku_summary.csv"], "sku_id"); err != nil {
		return nil, err
	}
	// Older fixtures may use "sku"; index both without failing when absent.
	_ = indexCSVColumn(idx.SKUIds, files["sku_summary.csv"], "sku")
	_ = indexCSVColumn(idx.SKUIds, files["sku_summary.csv"], "row_id")
	idx.HasSKUsAny = hasCSVDataRows(files["sku_summary.csv"])
	_ = indexCSVColumn(idx.CategoryIds, files["sku_summary.csv"], "category_id")
	if err := indexCSVColumn(idx.CampaignIds, files["campaign_summary.csv"], "campaign_id"); err != nil {
		return nil, err
	}
	_ = indexCSVColumn(idx.CampaignIds, files["campaign_summary.csv"], "row_id")
	idx.HasCampaignsAny = hasCSVDataRows(files["campaign_summary.csv"])
	_ = indexCSVColumn(idx.SKUIds, files["campaign_sku.csv"], "sku_id")
	_ = indexCSVColumn(idx.SKUIds, files["campaign_sku.csv"], "sku")
	_ = indexCSVColumn(idx.CampaignIds, files["campaign_sku.csv"], "campaign_id")
	_ = indexCSVColumn(idx.SKUIds, files["sku_daily.csv"], "sku_id")
	_ = indexCSVColumn(idx.SKUIds, files["product_enrichment.csv"], "sku_id")
	_ = indexCSVColumn(idx.SKUIds, files["stock_snapshot.csv"], "sku_id")
	indexPrefixedAliases(idx.SKUIds, "sku:")
	indexPrefixedAliases(idx.CampaignIds, "campaign:")
	if err := indexLimitations(idx, files["limitations.json"]); err != nil {
		return nil, err
	}
	if f := files["account_summary.csv"]; f != nil {
		if raw, err := readZipFile(f); err == nil {
			indexAccountSummaryFromCSV(idx, raw)
		}
	}
	indexSKUFactsFromCSV(idx, files["sku_summary.csv"])
	indexCampaignFactsFromCSV(idx, files["campaign_summary.csv"])
	return idx, nil
}

func indexPrefixedAliases(dst map[string]struct{}, prefix string) {
	if dst == nil || prefix == "" {
		return
	}
	extra := make([]string, 0, len(dst))
	for id := range dst {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if strings.HasPrefix(id, prefix) {
			extra = append(extra, strings.TrimPrefix(id, prefix))
		} else {
			extra = append(extra, prefix+id)
		}
	}
	for _, id := range extra {
		if id != "" {
			dst[id] = struct{}{}
		}
	}
}

// hasCSVDataRows reports whether f exists and has at least one data row
// beyond the header. Used to detect advertising-data presence independent
// of whether campaign_id parsed as non-empty.
func hasCSVDataRows(f *zip.File) bool {
	if f == nil {
		return false
	}
	raw, err := readZipFile(f)
	if err != nil {
		return false
	}
	r := csv.NewReader(bytes.NewReader(raw))
	if _, err := r.Read(); err != nil {
		return false
	}
	_, err = r.Read()
	return err == nil
}

func indexManifestPackID(idx *PackReferenceIndex, f *zip.File) error {
	if f == nil {
		return nil
	}
	raw, err := readZipFile(f)
	if err != nil {
		return err
	}
	var manifest struct {
		PackID string `json:"pack_id"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return fmt.Errorf("manifest.json: %w", err)
	}
	idx.PackID = strings.TrimSpace(manifest.PackID)
	return nil
}

func indexLimitations(idx *PackReferenceIndex, f *zip.File) error {
	if f == nil {
		return nil
	}
	raw, err := readZipFile(f)
	if err != nil {
		return err
	}
	var payload struct {
		Items []struct {
			Code string `json:"code"`
		} `json:"items"`
		Limitations []struct {
			Code string `json:"code"`
		} `json:"limitations"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("limitations.json: %w", err)
	}
	for _, item := range payload.Items {
		if c := strings.TrimSpace(item.Code); c != "" {
			idx.LimitationCodes[c] = struct{}{}
		}
	}
	for _, item := range payload.Limitations {
		if c := strings.TrimSpace(item.Code); c != "" {
			idx.LimitationCodes[c] = struct{}{}
		}
	}
	return nil
}

func indexCSVColumn(dst map[string]struct{}, f *zip.File, column string) error {
	if f == nil {
		return nil
	}
	raw, err := readZipFile(f)
	if err != nil {
		return err
	}
	r := csv.NewReader(bytes.NewReader(raw))
	r.ReuseRecord = true
	header, err := r.Read()
	if err != nil {
		return err
	}
	col := -1
	for i, h := range header {
		if strings.TrimSpace(h) == column {
			col = i
			break
		}
	}
	if col < 0 {
		return nil
	}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if col >= len(rec) {
			continue
		}
		v := strings.TrimSpace(rec[col])
		if v != "" {
			dst[v] = struct{}{}
		}
	}
	return nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
