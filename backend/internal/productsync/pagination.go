package productsync

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

const (
	SyncModeFull        = "full"
	SyncModeIncremental = "incremental"
)

var ErrMaxPagesPerRun = errors.New("products sync: exceeded max pages per run")

func errMaxPagesExceeded(lastID string, pages, received, imported int32) error {
	return fmt.Errorf("%w: last_id=%q pages=%d records_received=%d records_imported=%d",
		ErrMaxPagesPerRun, lastID, pages, received, imported)
}

// effectiveListLastID returns the Ozon pagination cursor for the next /v3/product/list request.
// If the API omits last_id on the last page, we fall back to the last item id in the batch.
func effectiveListLastID(items []ozon.ProductItem, apiLastID string) string {
	apiLastID = strings.TrimSpace(apiLastID)
	if apiLastID != "" {
		return apiLastID
	}
	if len(items) == 0 {
		return ""
	}
	return strconv.FormatInt(items[len(items)-1].ID, 10)
}

// anotherProductsPage reports whether the Ozon product list contract suggests another page exists.
func anotherProductsPage(itemsLen, limit int, requestLastID, effectiveResponseLastID string) bool {
	if limit <= 0 {
		return false
	}
	if itemsLen == 0 {
		return false
	}
	if effectiveResponseLastID == "" {
		return false
	}
	if effectiveResponseLastID == requestLastID {
		return false
	}
	if itemsLen < limit {
		return false
	}
	return true
}
