import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { ActionDetailPanel } from "@/components/actions/action-detail-panel";
import type { ActionItem, OperationalDetail } from "@/lib/actions-types";

const operationalDetail: OperationalDetail = {
  status: "available",
  format_version: "actions_detail_v2",
  why: {
    headline: "5 кампаний тратят бюджет с ДРР 364,7%",
    text: "За 7 дней 5 кампаний потратили 1 074 747 ₽ при подтверждённом высоком ДРР. Это создаёт риск неэффективного расхода бюджета.",
    facts: [
      { fact_id: "importance_1", label: "Факт 1", value: "Кампании расходуют бюджет с ДРР выше порога." },
      { fact_id: "importance_2", label: "Факт 2", value: "Продолжение без корректировки увеличит неэффективный расход." },
    ],
  },
  action_plan: {
    headline: "Что сделать",
    steps: [
      {
        step_id: "step_1",
        title: "Ограничить расход",
        instruction: "Снизьте бюджет или ставки.",
        why_this_step: "Высокий ДРР.",
        expected_result: "Остановить перерасход.",
        decision_mode: "direct",
      },
      {
        step_id: "step_2",
        title: "Проверить связанные SKU",
        instruction: "Убедитесь, что снижение ставок не отключит ключевые товары.",
        why_this_step: "",
        expected_result: "",
        decision_mode: "direct",
      },
      {
        step_id: "step_3",
        title: "Проверить результат за 7 дней",
        instruction: "Сверьте ДРР и расход после корректировки.",
        why_this_step: "",
        expected_result: "",
        decision_mode: "direct",
      },
    ],
  },
  object_actions: [
    {
      entity_type: "campaign",
      entity_id: "1",
      name: "Test campaign",
      metrics: [
        { label: "ДРР", value: "364,7%" },
        { label: "Расход 7д", value: "500 000 ₽" },
      ],
      recommended_action: "Снизить бюджет",
    },
  ],
  secondary_actions: [{ type: "ask_ai", label: "Спросить AI" }],
};

const v2Action: ActionItem = {
  id: 1,
  action_key: "test:ad_high_drr",
  type: "ad_high_drr",
  category: "advertising",
  priority: "critical",
  status: "open",
  title: "Ограничить расходы по 1 кампании",
  summary: "Кампании расходуют бюджет неэффективно.",
  reason: "legacy reason",
  next_step: "legacy step",
  expected_effect:
    "После корректировки снизится неэффективный расход бюджета. Итоговый эффект зависит от реакции ставок.",
  evidence: [{ kind: "metric", label: "ДРР", value: "364.7" }],
  limitations: ["legacy limitation"],
  visible_limitations: ["visible limitation"],
  ctas: [{ type: "ask_ai", label: "Спросить AI" }],
  affected_objects_total: 1,
  scope_type: "entity",
  operational_detail: operationalDetail,
};

const v2Html = renderToStaticMarkup(
  <ActionDetailPanel
    action={v2Action}
    loading={false}
    mutationError=""
    isMutating={false}
    onToggleChecked={() => undefined}
    onMarkIrrelevant={() => undefined}
    onAskAI={async () => undefined}
  />,
);
assert.match(v2Html, /Почему это важно/);
assert.match(v2Html, /План действий/);
assert.match(v2Html, /Затронутые кампании \(1\)/);
assert.match(v2Html, /Test campaign/);
assert.doesNotMatch(v2Html, /overflow-x-auto/);
assert.doesNotMatch(v2Html, /Доказательства/);
assert.doesNotMatch(v2Html, /Связанные объекты/);
assert.doesNotMatch(v2Html, /Ограничения анализа/);
assert.doesNotMatch(v2Html, /Ограничения данных/);
assert.doesNotMatch(v2Html, /Оборачиваемость/);
assert.doesNotMatch(v2Html, /Анализ от/);
assert.match(v2Html, /Спросить AI/);
assert.match(v2Html, /Ожидаемый результат/);
assert.doesNotMatch(v2Html, /Открыть проблемные кампании/);
assert.doesNotMatch(v2Html, /Открыть затронутые/);
assert.doesNotMatch(v2Html, /Открыть список SKU/);
assert.doesNotMatch(v2Html, /Ещё действия/);
assert.doesNotMatch(v2Html, /Скачать XLSX/);
assert.doesNotMatch(v2Html, /Скопировать/);
assert.doesNotMatch(v2Html, /Отложить/);
assert.match(v2Html, /Отметить неактуальным/);
assert.match(v2Html, /Отметить выполненным/);
assert.doesNotMatch(v2Html, /Затронутые товары 1 SKU/);
assert.doesNotMatch(v2Html, /Минимальн/);

const legacyHtml = renderToStaticMarkup(
  <ActionDetailPanel
    action={{ ...v2Action, operational_detail: undefined }}
    loading={false}
    mutationError=""
    isMutating={false}
    onToggleChecked={() => undefined}
    onMarkIrrelevant={() => undefined}
    onAskAI={async () => undefined}
  />,
);
assert.match(legacyHtml, /Почему это важно/);
assert.match(legacyHtml, /Ожидаемый результат/);

const headerHtml = renderToStaticMarkup(
  <ActionDetailPanel
    action={v2Action}
    loading={false}
    mutationError=""
    isMutating={false}
    onToggleChecked={() => undefined}
    onMarkIrrelevant={() => undefined}
    onAskAI={async () => undefined}
  />,
);
assert.match(headerHtml, /Ограничить расходы по 1 кампании/);
assert.match(headerHtml, /Критический/);
assert.match(headerHtml, /Реклама/);
assert.doesNotMatch(headerHtml, /MoreHorizontal/);
assert.doesNotMatch(headerHtml, /Открыть затронутые/);

const entitiesOnlyHtml = renderToStaticMarkup(
  <ActionDetailPanel
    action={{
      ...v2Action,
      category: "stock",
      type: "stock_replenishment",
      operational_detail: { ...operationalDetail, object_actions: [] },
      affected_objects_total: 2,
      affected_entities: {
        skus: [
          { sku: "sku_1", name: "SKU 1" },
          { sku: "sku_2", name: "SKU 2" },
        ],
      },
    }}
    loading={false}
    mutationError=""
    isMutating={false}
    onToggleChecked={() => undefined}
    onMarkIrrelevant={() => undefined}
    onAskAI={async () => undefined}
  />,
);
assert.match(entitiesOnlyHtml, /Затронутые товары \(2\)/);
assert.match(entitiesOnlyHtml, /SKU 1/);
assert.match(entitiesOnlyHtml, /SKU 2/);
assert.doesNotMatch(entitiesOnlyHtml, /Индивидуальные рекомендации временно не сформированы/);
assert.doesNotMatch(entitiesOnlyHtml, /Действие относится ко всему кабинету/);
assert.doesNotMatch(entitiesOnlyHtml, /available_stock/);

console.log("action-detail-panel.test.tsx: ok");
