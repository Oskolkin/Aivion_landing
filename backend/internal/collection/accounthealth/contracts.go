package accounthealth

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"

const (
	SummaryEndpoint       = "ozon.account_health.rating_summary"
	HistoryEndpoint       = "ozon.account_health.rating_history"
	FBSIndexEndpoint      = "ozon.account_health.fbs_index"
	FBSErrorsEndpoint     = "ozon.account_health.fbs_posting_errors"
	RedactionProfile      = "OZON_ACCOUNT_HEALTH_REDACTION_V1"
	CurrentSnapshotFamily = "ACCOUNT_HEALTH.CURRENT_SNAPSHOT"
	RatingHistoryFamily   = "ACCOUNT_HEALTH.RATING_HISTORY"
	FBSIndexFamily        = "ACCOUNT_HEALTH.FBS_INDEX"
	FBSErrorEventsFamily  = "ACCOUNT_HEALTH.FBS_ERROR_EVENTS"
)

var SummaryContract = collection.NewSourceContract(CurrentSnapshotFamily, "ozon.seller.rating.summary", "v1", "POST /v1/rating/summary", "account_health_summary_mapping_v1", "identity is provider+provider rating key; group name is descriptive only; provider status is not DQ severity")
var HistoryContract = collection.NewSourceContract(RatingHistoryFamily, "ozon.seller.rating.history", "v1", "POST /v1/rating/history", "account_health_history_mapping_v1", "provider periods remain single period facts and retain the thresholds observed with that source state")
var FBSIndexContract = collection.NewSourceContract(FBSIndexFamily, "ozon.seller.rating.index.fbs.info", "v1", "POST /v1/rating/index/fbs/info", "account_health_fbs_index_mapping_v1", "account health cost is not Finance and FBS error index is account-health-local")
var FBSErrorsContract = collection.NewSourceContract(FBSErrorEventsFamily, "ozon.seller.rating.index.fbs.posting.list", "v1", "POST /v1/rating/index/fbs/posting/list", "account_health_fbs_posting_errors_mapping_v1", "posting references resolve to canonical Posting or remain UNRESOLVED; no synthetic Posting")
