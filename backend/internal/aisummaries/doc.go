// Package aisummaries generates and stores daily AI management summaries for Ozon seller accounts
// (account-wide and per description category). It is separate from alert-driven SKU recommendations.
//
// Status model:
//   - ai_daily_summaries.status (DB): operational lifecycle — pending, running, completed, failed.
//   - acceptance_status (logical): completed | completed_repaired | completed_partial | failed.
//     Stored in validation_errors JSON as acceptance_status:<value> at generation time and
//     recomputed on dashboard read via ReacceptSummaryPayload when needed.
package aisummaries
