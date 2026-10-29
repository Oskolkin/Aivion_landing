import { isAITopActionsReady } from "@/lib/top-actions-narrative";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function run() {
  assert(
    isAITopActionsReady({ status: "available", narrative_source: "ai", freshness: "current" }),
    "current AI narrative should pass",
  );
  assert(
    isAITopActionsReady({
      status: "available",
      narrative_source: "ai",
      freshness: "current_with_minor_drift",
      validation_status: "completed_partial",
    }),
    "completed_partial with minor drift should pass",
  );
  assert(
    !isAITopActionsReady({ status: "available", narrative_source: "fallback", freshness: "fallback" }),
    "fallback narrative should fail",
  );
  assert(
    !isAITopActionsReady({ status: "available", narrative_source: "ai", freshness: "fallback" }),
    "mixed ai source with fallback freshness should fail",
  );
  assert(
    !isAITopActionsReady({ status: "available", narrative_source: "fallback", freshness: "current" }),
    "mixed fallback source with current freshness should fail",
  );
  assert(!isAITopActionsReady({ status: "available" }), "missing metadata should fail");

  console.log("top-actions-narrative.test.ts: ok");
}

run();
