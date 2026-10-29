package analysisdata

import (
	"context"
	"time"
)

// Loader loads a shared AnalysisDataSnapshot for a seller and metric date.
type Loader interface {
	Load(
		ctx context.Context,
		sellerAccountID int64,
		metricDate time.Time,
	) (*AnalysisDataSnapshot, error)
}
