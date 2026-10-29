package aisummaries

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aijsonrepair"
)

func repairRawSummaryJSON(raw string) (string, []string, error) {
	return aijsonrepair.RepairRawJSON(raw)
}
