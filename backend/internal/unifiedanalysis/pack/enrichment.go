package pack

import (
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/jackc/pgx/v5/pgtype"
)

// BuildProductEnrichmentTable renders product_enrichment.csv rows in SKU-ascending order.
func BuildProductEnrichmentTable(idx *Indexes, limitations *limitationCollector) csvTable {
	rows := make([][]string, 0, len(idx.SKUUniverse))
	for _, sku := range idx.SKUUniverse {
		snap, ok := idx.ProductInfoBySKU[sku]
		if !ok {
			limitations.Add(LimitationEnrichmentUnavailable)
			rows = append(rows, []string{
				"sku:" + sku, sku,
				"", "", "", "", "", "", "", "", "",
			})
			continue
		}

		validationStatus := textValue(snap.ValidationStatus)
		priceIndex := textValue(snap.PriceIndexColor)
		var hasStock *bool
		if snap.VisibilityHasStock.Valid {
			v := snap.VisibilityHasStock.Bool
			hasStock = &v
		}

		limitations.Add(LimitationContentScoreUnavailable)
		limitations.Add(LimitationAttributeCompletionUnknown)

		var imageCount *int64
		if n := len(parseJSONElements(snap.ImagesJson)); n > 0 {
			count := int64(n)
			imageCount = &count
		} else if snap.PrimaryImage.Valid {
			one := int64(1)
			imageCount = &one
		}

		warningCodes := extractWarningCodes(snap.ErrorsJson)

		rows = append(rows, []string{
			"sku:" + sku,
			sku,
			validationStatus,
			priceIndex,
			FormatBoolCSVPtr(hasStock),
			"", // content_score
			"", // title_quality_status
			"", // description_quality_status
			FormatIntCSVPtr(imageCount),
			"", // attribute_completion_pct
			warningCodes,
		})
	}
	return csvTable{Name: "product_enrichment.csv", Columns: contract.ProductEnrichmentColumnsV1, Rows: rows}
}

func textValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func extractWarningCodes(raw []byte) string {
	elems := parseJSONElements(raw)
	if len(elems) == 0 {
		return ""
	}
	codes := make([]string, 0, len(elems))
	for _, e := range elems {
		switch v := e.(type) {
		case string:
			codes = append(codes, v)
		case map[string]any:
			if code, ok := v["code"].(string); ok && code != "" {
				codes = append(codes, code)
			} else if src, ok := v["source"].(string); ok && src != "" {
				codes = append(codes, src)
			}
		}
	}
	return strings.Join(codes, "|")
}
