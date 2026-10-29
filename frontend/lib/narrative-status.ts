import type { DashboardHomeResponse } from "@/lib/dashboard-home-types";

type AccountSummary = DashboardHomeResponse["ai_diagnostics"]["account_summary"];

const AI_VALIDATION_STATUSES = new Set([
  "completed",
  "completed_repaired",
  "completed_partial",
]);

const AI_FRESHNESS_VALUES = new Set(["current", "current_with_minor_drift"]);

/** True when account_summary narrative came from AI and is still usable. */
export function isAccountSummaryNarrativeAI(accountSummary: AccountSummary): boolean {
  if (accountSummary.narrative_source !== "ai") {
    return false;
  }
  const freshness = accountSummary.freshness ?? "";
  if (!AI_FRESHNESS_VALUES.has(freshness)) {
    return false;
  }
  const validationStatus = accountSummary.validation_status ?? "";
  return validationStatus === "" || AI_VALIDATION_STATUSES.has(validationStatus);
}

/** Diagnostics helper for top actions sourced from a current AI narrative. */
export function isAITopActionsReady(accountSummary: AccountSummary): boolean {
  return isAccountSummaryNarrativeAI(accountSummary);
}
