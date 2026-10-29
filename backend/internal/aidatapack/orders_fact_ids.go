package aidatapack

// RequiredAccountOrderFactIDs lists canonical order comparison facts expected in account rollup.
var RequiredAccountOrderFactIDs = []string{
	"account.last_7d.orders",
	"account.previous_7d.orders",
	"account.orders.last_7d_vs_previous_7d_delta_abs",
	"account.orders.last_7d_vs_previous_7d_delta_pct",
}
