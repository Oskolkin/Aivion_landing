import assert from "node:assert/strict";
import {
  actionPriorityDisplayLabel,
  buildActionMetrics,
  isUnknownMetricValue,
  mapActionToDetail,
  mapActionToListCard,
  mapDailyFocus,
  matchesActionSearch,
  pluralizeEntityCount,
  sortActionsList,
} from "@/lib/actions-view-model";
import type { ActionItem } from "@/lib/actions-types";

function baseAction(overrides: Partial<ActionItem> = {}): ActionItem {
  return {
    id: 1,
    action_key: "stock:test",
    type: "stock_replenishment",
    category: "stock",
    priority: "important",
    status: "open",
    title: "Пополнить запас по 4 SKU с нулевыми остатками",
    summary: "Товары полностью закончились и перестали приносить продажи.",
    reason:
      "Эти товары формируют значимую долю недельной выручки. При текущем темпе продаж дефицитные SKU могут закончиться раньше следующей поставки. После обнуления остатка покупатели не смогут оформить новые заказы.",
    next_step: "Рассчитать поставку",
    expected_effect:
      "После пополнения товары снова станут доступны для заказа, а риск обнуления остатков снизится. Это поможет сохранить продажи по позициям с подтверждённым спросом.",
    evidence: [
      { kind: "metric", label: "SKU без остатка", value: "4", unit: "count" },
      { kind: "metric", label: "coverage_days", value: "0", unit: "days" },
    ],
    limitations: [],
    ctas: [],
    completion_criteria: "available_stock_units > 0",
    completion_conditions: [
      {
        metric_code: "available_stock_units",
        operator: "gt",
        target_value: 0,
        scope: "each_entity",
        display_text: "Запас всех указанных SKU стал больше нуля",
      },
    ],
    affected_objects_total: 4,
    scope_type: "entity",
    operational_detail: {
      status: "available",
      format_version: "actions_detail_v2",
      why: {
        headline: "Дефицит по 4 SKU",
        text: "Четыре ключевых SKU закончились при подтверждённом спросе за последние 7 дней. Текущий остаток не покрывает ближайшую поставку.",
        facts: [
          { fact_id: "importance_1", label: "", value: "SKU формируют значимую долю выручки." },
          { fact_id: "importance_2", label: "", value: "Дефицитные позиции могут закончиться раньше поставки." },
          { fact_id: "importance_3", label: "", value: "После обнуления остатка новые заказы будут недоступны." },
        ],
      },
      action_plan: {
        headline: "План",
        steps: [
          {
            step_id: "s1",
            title: "Проверить остатки и поставки",
            instruction: "Сверить остаток и продажи.",
            why_this_step: "",
            expected_result: "",
          },
          {
            step_id: "s2",
            title: "Рассчитать потребность",
            instruction: "Определить объём поставки.",
            why_this_step: "",
            expected_result: "",
          },
          {
            step_id: "s3",
            title: "Создать поставку",
            instruction: "Добавить SKU в ближайшую поставку.",
            why_this_step: "",
            expected_result: "",
          },
        ],
      },
      object_actions: [
        {
          entity_type: "sku",
          entity_id: "1001",
          name: "Товар 1",
          metrics: [
            { label: "Остаток", value: "0" },
            { label: "Заказы 7д", value: "3" },
            { label: "Покрытие", value: "0 дн." },
          ],
          recommended_action: "Срочно пополнить",
        },
        {
          entity_type: "sku",
          entity_id: "1002",
          name: "Товар 2",
          metrics: [
            { label: "Остаток", value: "1" },
            { label: "Заказы 7д", value: "2" },
            { label: "Покрытие", value: "0.5 дн." },
          ],
          recommended_action: "Срочно пополнить",
        },
        {
          entity_type: "sku",
          entity_id: "1003",
          name: "Товар 3",
          metrics: [
            { label: "Остаток", value: "2" },
            { label: "Заказы 7д", value: "4" },
            { label: "Покрытие", value: "1 дн." },
          ],
          recommended_action: "Пополнить",
        },
        {
          entity_type: "sku",
          entity_id: "1004",
          name: "Товар 4",
          metrics: [
            { label: "Остаток", value: "0" },
            { label: "Заказы 7д", value: "1" },
            { label: "Покрытие", value: "0 дн." },
          ],
          recommended_action: "Срочно пополнить",
        },
      ],
    },
    created_at: "2026-07-08T10:00:00Z",
    ...overrides,
  };
}

assert.equal(actionPriorityDisplayLabel("critical"), "Критический");
assert.equal(actionPriorityDisplayLabel("important"), "Высокий приоритет");
assert.equal(pluralizeEntityCount(1, "sku"), "1 SKU");
assert.equal(pluralizeEntityCount(2, "campaign"), "2 кампании");
assert.equal(pluralizeEntityCount(5, "campaign"), "5 кампаний");
assert.equal(pluralizeEntityCount(1, "category"), "1 категория");
assert.equal(isUnknownMetricValue("—"), true);
assert.equal(isUnknownMetricValue("0"), false);

const listVm = mapActionToListCard(baseAction());
assert.equal(listVm.priority.label, "Высокий приоритет");
assert.equal(listVm.entitiesLabel, "4 SKU");
assert.equal(listVm.showStatusBadge, false);
assert.doesNotMatch(listVm.title, /объект/i);

const detail = mapActionToDetail(baseAction());
assert.equal(detail.situation?.title, "Ситуация");
assert.ok(detail.importanceFacts.length >= 2);
assert.equal(detail.affectedEntities?.label, "Затронутые товары (4)");
assert.equal(detail.affectedEntities?.count, 4);
assert.equal(detail.affectedEntities?.rows.length, 4);
assert.equal(detail.primaryAction, null);
assert.equal(detail.openEntitiesHref, null);
assert.equal(detail.metrics.length, 0);
assert.ok(detail.steps.length >= 3);
assert.ok(detail.completionCriteria[0]?.label.includes("Запас"));
assert.doesNotMatch(detail.completionCriteria[0]?.label ?? "", /available_stock_units/);
assert.ok(detail.completionCriteria[0]?.technicalExplanation);

// Stale affected_objects_total must not override validated object_actions length.
const pruned = mapActionToDetail(
  baseAction({
    title: "Пополнить запас по 4 SKU с нулевыми остатками",
    affected_objects_total: 4,
    operational_detail: {
      status: "available",
      format_version: "actions_detail_v2",
      why: {
        headline: "Дефицит",
        text: "Один SKU остался после pruning.",
        facts: [{ fact_id: "importance_1", label: "Важность", value: "Риск потери заказов." }],
      },
      action_plan: {
        headline: "План",
        steps: [
          { step_id: "s1", title: "Проверить остаток", instruction: "Сверить остаток.", why_this_step: "", expected_result: "" },
          { step_id: "s2", title: "Рассчитать", instruction: "Рассчитать объём.", why_this_step: "", expected_result: "" },
          { step_id: "s3", title: "Поставить", instruction: "Создать поставку.", why_this_step: "", expected_result: "" },
        ],
      },
      object_actions: [
        {
          entity_type: "sku",
          entity_id: "1001",
          name: "Товар 1",
          metrics: [{ label: "Остаток", value: "0" }],
          recommended_action: "Срочно пополнить",
        },
      ],
    },
  }),
);
assert.equal(pruned.affectedEntities?.count, 1);
const prunedList = mapActionToListCard(
  baseAction({
    affected_objects_total: 4,
    operational_detail: {
      ...baseAction().operational_detail!,
      object_actions: [
        {
          entity_type: "sku",
          entity_id: "1001",
          name: "Товар 1",
          metrics: [{ label: "Остаток", value: "0" }],
          recommended_action: "Срочно",
        },
      ],
    },
  }),
);
assert.equal(prunedList.entitiesLabel, "1 SKU");

const metrics = buildActionMetrics(baseAction());
assert.ok(metrics.length > 0);
assert.ok(!metrics.some((m) => m.value === "—"));

const focus = mapDailyFocus({
  focus: {
    status: "available",
    source: "ai",
    headline: "4 SKU без остатка могут ограничивать продажи",
    summary: "Рекомендуется рассчитать пополнение",
    start_with: "Рекомендуется рассчитать и запустить срочное пополнение",
  },
  priorityAction: baseAction(),
  hasOpenActions: true,
});
assert.equal(focus.headline, "4 SKU без остатка могут ограничивать продажи");
assert.doesNotMatch(focus.headline, /Главное действие/);
assert.equal(focus.affectedLabel, "4 SKU");
assert.ok(focus.recommendation);

const emptyFocus = mapDailyFocus({ hasOpenActions: false });
assert.equal(emptyFocus.empty, true);

const sorted = sortActionsList(
  [
    baseAction({ action_key: "a", priority: "watch", created_at: "2026-07-01T00:00:00Z" }),
    baseAction({ action_key: "b", priority: "critical", created_at: "2026-07-02T00:00:00Z" }),
    baseAction({ action_key: "c", priority: "important", created_at: "2026-07-03T00:00:00Z" }),
  ],
  "priority",
);
assert.deepEqual(
  sorted.map((item) => item.action_key),
  ["b", "c", "a"],
);

assert.equal(matchesActionSearch(baseAction(), "нулевыми"), true);
assert.equal(matchesActionSearch(baseAction(), "кампания-xyz"), false);
assert.equal(matchesActionSearch(baseAction(), "1001"), true);

const completed = mapActionToDetail(baseAction({ status: "checked" }));
assert.equal(completed.primaryAction, null);

const noEntities = mapActionToDetail(
  baseAction({
    affected_objects_total: 0,
    scope_type: "account",
    operational_detail: {
      status: "available",
      format_version: "v2",
      why: { headline: "x", text: "y" },
      action_plan: { headline: "p", steps: [] },
      object_actions: [],
    },
    affected_entities: {},
  }),
);
assert.equal(noEntities.affectedEntities, null);

const ad = mapActionToDetail(
  baseAction({
    category: "advertising",
    type: "ad_high_drr_review",
    affected_objects_total: 2,
    operational_detail: {
      status: "available",
      format_version: "v2",
      why: { headline: "ДРР", text: "Высокий ДРР" },
      action_plan: { headline: "p", steps: [] },
      object_actions: [
        {
          entity_type: "campaign",
          entity_id: "c1",
          name: "Кампания 1",
          recommended_action: "Снизить ставки",
        },
        {
          entity_type: "campaign",
          entity_id: "c2",
          name: "Кампания 2",
          recommended_action: "Снизить ставки",
        },
      ],
    },
  }),
);
assert.equal(ad.primaryAction, null);
assert.match(ad.affectedEntities?.label ?? "", /кампани/i);
assert.equal(ad.affectedEntities?.count, 2);

console.log("actions-view-model.test.ts: ok");
