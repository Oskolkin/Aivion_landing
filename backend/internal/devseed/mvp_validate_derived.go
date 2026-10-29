package devseed

import (
	"context"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ValidateMVPDerivedSeller checks that manual MVP flows left rows in alerts, recommendations, chat, and admin support tables.
// Read-only: no inserts and no external API calls.
func ValidateMVPDerivedSeller(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64) ([]MVPValidateRow, bool) {
	q := dbgen.New(pool)
	var rows []MVPValidateRow
	allOK := true

	add := func(component, expected, actual string, ok bool) {
		if !ok {
			allOK = false
		}
		rows = append(rows, MVPValidateRow{
			Component: component,
			Expected:  expected,
			Actual:    actual,
			OK:        ok,
		})
	}

	if _, err := q.GetSellerAccountByID(ctx, sellerAccountID); err != nil {
		add("seller_account", "exists", "not found", false)
		return rows, false
	}

	aruns, err := q.CountAlertRunsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		add("alerts: alert_runs", "> 0", fmt.Sprintf("error: %v", err), false)
		return rows, false
	}
	add("alerts: alert_runs", "> 0", fmt.Sprintf("%d", aruns), aruns > 0)

	openAlerts, err := q.CountOpenAlertsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		add("alerts: open total", "> 0", fmt.Sprintf("error: %v", err), false)
		return rows, false
	}
	add("alerts: open total", "> 0", fmt.Sprintf("%d", openAlerts), openAlerts > 0)

	groupRows, err := q.CountOpenAlertsByGroup(ctx, sellerAccountID)
	if err != nil {
		add("alerts: open by group", "sales,stock,advertising,price_economics", fmt.Sprintf("error: %v", err), false)
		return rows, false
	}
	byGroup := make(map[string]int64, len(groupRows))
	for _, r := range groupRows {
		byGroup[r.AlertGroup] = r.AlertsCount
	}
	requiredGroups := []string{"sales", "stock", "advertising", "price_economics"}
	for _, g := range requiredGroups {
		n := byGroup[g]
		add(fmt.Sprintf("alerts: open group %s", g), "> 0", fmt.Sprintf("%d", n), n > 0)
	}

	recRuns, err := q.CountRecommendationRunsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		add("recommendations: runs", "> 0", fmt.Sprintf("error: %v", err), false)
		return rows, false
	}
	add("recommendations: runs", "> 0", fmt.Sprintf("%d", recRuns), recRuns > 0)

	recTotal, err := q.CountRecommendationsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		add("recommendations: rows", "> 0", fmt.Sprintf("error: %v", err), false)
		return rows, false
	}
	add("recommendations: rows", "> 0", fmt.Sprintf("%d", recTotal), recTotal > 0)

	linkN, err := q.CountRecommendationAlertLinksBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		add("recommendations: linked to alerts", "> 0", fmt.Sprintf("error: %v", err), false)
		return rows, false
	}
	add("recommendations: linked to alerts", "> 0", fmt.Sprintf("%d", linkN), linkN > 0)

	openRec, err := q.CountOpenRecommendationsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		add("recommendations: open", "> 0", fmt.Sprintf("error: %v", err), false)
		return rows, false
	}
	add("recommendations: open", "> 0", fmt.Sprintf("%d", openRec), openRec > 0)

	sessions, err := q.CountChatSessionsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		add("chat: sessions", "> 0", fmt.Sprintf("error: %v", err), false)
		return rows, false
	}
	add("chat: sessions", "> 0", fmt.Sprintf("%d", sessions), sessions > 0)

	msgs, err := q.CountChatMessagesBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		add("chat: messages", "> 0", fmt.Sprintf("error: %v", err), false)
		return rows, false
	}
	add("chat: messages", "> 0", fmt.Sprintf("%d", msgs), msgs > 0)

	traces, err := q.CountChatTracesBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		add("chat: traces", "> 0", fmt.Sprintf("error: %v", err), false)
		return rows, false
	}
	add("chat: traces", "> 0", fmt.Sprintf("%d", traces), traces > 0)

	doneTraces, err := q.CountCompletedChatTracesBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		add("chat: completed traces", "> 0", fmt.Sprintf("error: %v", err), false)
		return rows, false
	}
	add("chat: completed traces", "> 0", fmt.Sprintf("%d", doneTraces), doneTraces > 0)

	add("admin: recommendation_runs (listable)", "> 0", fmt.Sprintf("%d", recRuns), recRuns > 0)
	add("admin: chat_traces (listable)", "> 0", fmt.Sprintf("%d", traces), traces > 0)

	adminLogs, err := q.CountAdminActionLogsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		add("admin: action_logs", "> 0", fmt.Sprintf("error: %v", err), false)
		return rows, false
	}
	add("admin: action_logs", "> 0", fmt.Sprintf("%d", adminLogs), adminLogs > 0)

	return rows, allOK
}
