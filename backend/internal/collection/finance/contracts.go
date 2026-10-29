package finance

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"

const (
	TypesEndpoint    = "ozon.finance.accrual.types.v1"
	ByDayEndpoint    = "ozon.finance.accrual.by_day.v1"
	PostingsEndpoint = "ozon.finance.accrual.postings.v1"
	BalanceEndpoint  = "ozon.finance.balance.v1"
	RedactionProfile = "OZON_FINANCE_REDACTION_V1"
	LedgerPolicyV1   = "finance_daily_ledger_policy_v1"
	BalancePolicyV1  = "finance_balance_snapshot_policy_v1"
)

var TypesContract = collection.NewSourceContract(
	"FINANCE.TYPE_DICTIONARY", "ozon.seller.finance.accrual.types", "v1",
	"POST /v1/finance/accrual/types", "finance_type_registry_v1",
	"unknown provider types remain UNMAPPED and retain their signed amount",
)

var ByDayContract = collection.NewSourceContract(
	"FINANCE.DAILY_LEDGER", "ozon.seller.finance.accrual.by-day", "v1",
	"POST /v1/finance/accrual/by-day", "finance_daily_ledger_v1",
	"day snapshot revisions are authoritative; account net is the sum of parent totals only",
)
