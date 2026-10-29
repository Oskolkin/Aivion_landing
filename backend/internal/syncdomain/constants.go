package syncdomain

import "strings"

// SyncType identifies how much data a sync job loads.
type SyncType string

const (
	SyncTypeInitial     SyncType = "initial_sync"
	SyncTypeIncremental SyncType = "incremental_sync"
)

func (t SyncType) String() string { return string(t) }

func NormalizeSyncType(raw string) SyncType {
	switch SyncType(raw) {
	case SyncTypeInitial:
		return SyncTypeInitial
	default:
		return SyncTypeIncremental
	}
}

type SyncStatus string

const (
	SyncStatusPending   SyncStatus = "pending"
	SyncStatusRunning   SyncStatus = "running"
	SyncStatusCompleted SyncStatus = "completed"
	SyncStatusFailed    SyncStatus = "failed"
)

func (s SyncStatus) String() string { return string(s) }

type ImportDomain string

const (
	ImportDomainProducts ImportDomain = "products"
	ImportDomainOrders   ImportDomain = "orders"
	ImportDomainStocks   ImportDomain = "stocks"
	ImportDomainAds      ImportDomain = "ads"
)

func (d ImportDomain) String() string { return string(d) }

var AllImportDomains = []ImportDomain{
	ImportDomainProducts,
	ImportDomainOrders,
	ImportDomainStocks,
	ImportDomainAds,
}

type ImportStatus string

const (
	ImportStatusPending    ImportStatus = "pending"
	ImportStatusFetching   ImportStatus = "fetching"
	ImportStatusImporting  ImportStatus = "importing"
	ImportStatusRetrying   ImportStatus = "retrying"
	ImportStatusCompleted  ImportStatus = "completed"
	ImportStatusFailed     ImportStatus = "failed"
	ImportStatusSuperseded ImportStatus = "superseded"
)

func (s ImportStatus) String() string { return string(s) }

func IsTerminalImportStatus(status string) bool {
	switch ImportStatus(status) {
	case ImportStatusCompleted, ImportStatusFailed, ImportStatusSuperseded:
		return true
	default:
		return false
	}
}

// IsCanonicalImportJobForFinalization reports whether an import row participates in parent sync finalization.
func IsCanonicalImportJobForFinalization(domain, status string, supersededBy *int64) bool {
	if supersededBy != nil && *supersededBy > 0 {
		return false
	}
	if status == ImportStatusSuperseded.String() {
		return false
	}
	return !strings.Contains(domain, "__dedup_")
}

func IsActiveImportStatus(status string) bool {
	switch ImportStatus(status) {
	case ImportStatusPending, ImportStatusFetching, ImportStatusImporting, ImportStatusRetrying:
		return true
	default:
		return false
	}
}

const (
	TaskTypeOzonSyncCoordinator = "ozon.sync_coordinator"
)
