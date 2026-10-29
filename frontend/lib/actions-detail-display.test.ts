import assert from "node:assert/strict";
import {
  ACTION_DETAIL_EXPANDED_TABLE_MIN_WIDTH,
  ACTION_DETAIL_TWO_COLUMN_MIN_WIDTH,
  COMPACT_OBJECT_LIMIT,
  compactObjectSlice,
  hiddenObjectCount,
  shouldUseExpandedTableLayout,
  shouldUseTwoColumnDetailLayout,
} from "@/lib/actions-detail-layout";
import {
  areActionTextsNearDuplicate,
  collapseLeadingEntityIdList,
  formatActionMetricValue,
  formatActionObjectIdentifier,
  getCompactObjectRecommendation,
  normalizeActionReasonForDisplay,
  normalizeAffectedObjects,
  normalizeEntityId,
} from "@/lib/actions-display";
import type { OperationalObjectAction } from "@/lib/actions-types";

assert.equal(shouldUseTwoColumnDetailLayout(1100), false);
assert.equal(shouldUseTwoColumnDetailLayout(ACTION_DETAIL_TWO_COLUMN_MIN_WIDTH), true);
assert.equal(shouldUseExpandedTableLayout(850), false);
assert.equal(shouldUseExpandedTableLayout(ACTION_DETAIL_EXPANDED_TABLE_MIN_WIDTH), true);
assert.equal(hiddenObjectCount(8, COMPACT_OBJECT_LIMIT), 4);
assert.equal(compactObjectSlice([1, 2, 3, 4, 5]).length, 4);

assert.equal(
  formatActionMetricValue({ metricKey: "Остаток", value: "386", rawUnit: "count" }),
  "386 шт.",
);
assert.equal(
  formatActionMetricValue({ metricKey: "Заказы 7д", value: "0", rawUnit: "count", context: "orders_metric" }),
  "0",
);
assert.equal(
  formatActionMetricValue({ metricKey: "Выручка 7д", value: "160", rawUnit: "RUB" }),
  "160 ₽",
);
assert.equal(
  formatActionMetricValue({ metricKey: "Выручка 7д", value: "160 RUB", rawUnit: "RUB" }).includes("RUB"),
  false,
);
assert.doesNotMatch(
  formatActionMetricValue({ metricKey: "Выручка 7д", value: "160 ₽", rawUnit: "RUB" }),
  /RUB/,
);
assert.equal(formatActionObjectIdentifier("sku", "sku:801000100041"), "SKU 801000100041");
assert.equal(normalizeEntityId("sku:801000100041"), "801000100041");
assert.equal(
  formatActionMetricValue({ metricKey: "ДРР", value: "13.2", rawUnit: "pct" }),
  "13,2%",
);

const skuObjects: OperationalObjectAction[] = [
  {
    entity_type: "sku",
    entity_id: "801000100001",
    name: "Товар 1",
    metrics: [{ label: "Остаток", value: "10" }],
    recommended_action: "Проверить цену.",
  },
  {
    entity_type: "sku",
    entity_id: "801000100002",
    name: "Товар 2",
    metrics: [{ label: "Остаток", value: "20" }],
    recommended_action: "Проверить карточку.",
  },
];

const normalized = normalizeAffectedObjects(skuObjects);
assert.equal(normalized[0]?.metrics[0]?.formattedValue, "10 шт.");
assert.ok(getCompactObjectRecommendation("Первое. Второе. Третье предложение."));

const collapsed = collapseLeadingEntityIdList(
  "SKU 801000100001, 801000100002 не получили заказов за 7 дней.",
  skuObjects,
);
assert.match(collapsed, /2 SKU/);
assert.doesNotMatch(collapsed, /801000100001/);

const reason = normalizeActionReasonForDisplay({
  reason: "SKU 801000100001, 801000100002 не получили заказов за 7 дней при остатке.",
  title: "Разберите 2 SKU",
  summary: "2 SKU не продавались 7 дней.",
  affectedObjects: skuObjects,
});
assert.ok(reason);
assert.doesNotMatch(reason ?? "", /801000100001/);

const hiddenDuplicate = normalizeActionReasonForDisplay({
  reason: "2 SKU не продавались 7 дней при остатке.",
  title: "Разберите 2 SKU",
  summary: "2 SKU не продавались 7 дней при остатке.",
  affectedObjects: skuObjects,
});
assert.equal(hiddenDuplicate, null);
assert.equal(areActionTextsNearDuplicate("Текст один", "Текст один"), true);

console.log("actions-detail-display.test.ts: ok");
