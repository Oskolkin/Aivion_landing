import assert from "node:assert/strict";
import {
  formatAffectedObjectsDetailCaption,
  formatAffectedObjectsHeader,
  formatCompletionConditionLine,
  formatMetricLabel,
  normalizeAffectedObjects,
  resolveAffectedObjectCounts,
} from "@/lib/actions-display";
import type { ActionItem } from "@/lib/actions-types";

function makeAction(partial: Partial<ActionItem>): ActionItem {
  return {
    id: 1,
    action_key: "k1",
    type: "stock_replenishment",
    category: "stock",
    priority: "critical",
    status: "open",
    title: "Срочно пополнить 12 SKU",
    reason: "reason",
    next_step: "step",
    evidence: [],
    ctas: [],
    ...partial,
  };
}

const twelveSkuObjects = Array.from({ length: 12 }, (_, i) => ({
  entity_type: "sku",
  entity_id: `sku_${i + 1}`,
  name: `SKU ${i + 1}`,
  metrics: [{ label: "idc_days", value: "5", display_label: "Покрытие запасом", formatted_value: "5 дн." }],
  recommended_action: "Пополнить",
}));

const action12of12 = makeAction({
  affected_objects_total: 12,
  affected_objects_displayed: 12,
  affected_objects_truncated: false,
  operational_detail: { status: "available", format_version: "v2", why: { headline: "", text: "" }, action_plan: { headline: "", steps: [] }, object_actions: twelveSkuObjects },
});

const action2of12 = makeAction({
  affected_objects_total: 12,
  affected_objects_displayed: 2,
  affected_objects_truncated: true,
  operational_detail: {
    status: "available",
    format_version: "v2",
    why: { headline: "", text: "" },
    action_plan: { headline: "", steps: [] },
    object_actions: twelveSkuObjects.slice(0, 2),
  },
});

assert.equal(formatAffectedObjectsHeader(12, "sku"), "12 SKU");
assert.equal(formatAffectedObjectsDetailCaption(12, 2, true), "Затронуто: 12 SKU. Подробно показано: 2 из 12. Для 10 объектов индивидуальные рекомендации не сформированы. Общий план относится ко всем 12 SKU.");
assert.equal(formatAffectedObjectsDetailCaption(3, 3, false, "sku"), "Затронуто: 3 SKU. Подробные рекомендации сформированы для всех 3.");
assert.match(formatAffectedObjectsDetailCaption(6, 0, true, "sku") ?? "", /не сформированы/);

const noObjectActions = makeAction({
  affected_objects_total: 6,
  affected_objects_displayed: 0,
  affected_objects_truncated: true,
  affected_entities: { skus: Array.from({ length: 6 }, (_, i) => ({ sku: `s${i + 1}` })) },
});
const noObjectCounts = resolveAffectedObjectCounts(noObjectActions);
assert.equal(noObjectCounts.total, 6);
assert.equal(noObjectCounts.displayed, 0);
assert.equal(noObjectCounts.truncated, true);

const counts = resolveAffectedObjectCounts(action2of12);
assert.equal(counts.total, 12);
assert.equal(counts.displayed, 2);
assert.equal(counts.truncated, true);

const legacy = makeAction({
  affected_entities: { skus: Array.from({ length: 4 }, (_, i) => ({ sku: `s${i}` })) },
  operational_detail: {
    status: "available",
    format_version: "v2",
    why: { headline: "", text: "" },
    action_plan: { headline: "", steps: [] },
    object_actions: twelveSkuObjects.slice(0, 2),
  },
});
assert.equal(resolveAffectedObjectCounts(legacy).total, 4);

const normalized = normalizeAffectedObjects([
  {
    entity_type: "sku",
    entity_id: "1",
    metrics: [{ label: "idc_days", value: "5", display_label: "Покрытие запасом", formatted_value: "5 дн." }],
    recommended_action: "Пополнить",
  },
]);
assert.equal(normalized[0]?.metrics[0]?.label, "Покрытие запасом");
assert.equal(formatMetricLabel("idc_days"), "Покрытие запасом");
assert.equal(
  formatCompletionConditionLine({
    metric_code: "drr_pct",
    operator: "lte",
    target_value: 15,
    scope: "per_campaign",
    period_days: 7,
  }),
  "ДРР ≤ 15 (per_campaign, 7 дн.)",
);
assert.equal(action12of12.title, "Срочно пополнить 12 SKU");

const accountAction = makeAction({ scope_type: "account" });
const accountCounts = resolveAffectedObjectCounts(accountAction);
assert.equal(accountCounts.total, 0);
assert.equal(accountCounts.displayed, 0);
assert.equal(accountCounts.scopeType, "account");

const mixedAction = makeAction({
  scope_type: "mixed",
  affected_objects_total: 5,
  affected_objects_displayed: 3,
  affected_objects_truncated: true,
  object_actions_coverage: { mode: "partial", reason: "Показаны наиболее значимые SKU." },
  operational_detail: {
    status: "available",
    format_version: "v2",
    why: { headline: "", text: "" },
    action_plan: { headline: "", steps: [] },
    object_actions: twelveSkuObjects.slice(0, 3),
  },
});
assert.equal(resolveAffectedObjectCounts(mixedAction).scopeType, "mixed");
assert.match(formatAffectedObjectsDetailCaption(5, 3, true, "sku", "partial") ?? "", /Подробно показано: 3 из 5/);

console.log("actions-count-display.test.ts: ok");
