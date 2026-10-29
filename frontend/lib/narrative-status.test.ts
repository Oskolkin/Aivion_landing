import assert from "node:assert/strict";
import { isAccountSummaryNarrativeAI, isAITopActionsReady } from "@/lib/narrative-status";

const base = { status: "available" as const };
assert(isAccountSummaryNarrativeAI({ ...base, narrative_source: "ai", freshness: "current", validation_status: "completed" }));
assert(isAccountSummaryNarrativeAI({ ...base, narrative_source: "ai", freshness: "current_with_minor_drift", validation_status: "completed_repaired" }));
assert(isAccountSummaryNarrativeAI({ ...base, narrative_source: "ai", freshness: "current", validation_status: "completed_partial" }));
assert(!isAccountSummaryNarrativeAI({ ...base, narrative_source: "fallback", freshness: "fallback", validation_status: "fallback_stale" }));
assert(!isAccountSummaryNarrativeAI({ ...base, narrative_source: "ai", freshness: "fallback", validation_status: "completed" }));
assert(!isAccountSummaryNarrativeAI({ ...base, narrative_source: "ai", freshness: "current", validation_status: "fallback_stale" }));

assert(isAITopActionsReady({ ...base, narrative_source: "ai", freshness: "current_with_minor_drift", validation_status: "completed_partial" }));
assert(!isAITopActionsReady({ ...base, narrative_source: "fallback", freshness: "current", validation_status: "completed" }));

console.log("narrative-status.test.ts passed");
