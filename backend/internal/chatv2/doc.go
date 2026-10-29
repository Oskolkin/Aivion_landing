// Package chatv2 is the seller-facing Chat owner path after Prompt 15.
//
// Active Ask/stream/handoff/export must not call the legacy planner, SQL
// analytics repository, alerts, recommendations, legacy action item tables, Ozon, or
// chatexport analytical DataService.
package chatv2
