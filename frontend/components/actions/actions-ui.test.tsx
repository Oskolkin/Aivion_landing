import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { ActionCard } from "@/components/actions/action-card";
import { ActionsFocusCard } from "@/components/actions/actions-focus-card";
import { ActionDetailPanel } from "@/components/actions/action-detail-panel";
import type { ActionItem } from "@/lib/actions-types";
import {
  actionCardFooter,
  actionObjectsSummary,
  formatMetricLabel,
  formatMetricValue,
  focusHeadlineText,
  parseObjectMetricColumns,
  removeRepeatedLeadingTitle,
} from "@/lib/actions-display";

const sampleAction: ActionItem = {
  id: 1,
  action_key: "slow_turnover:test",
  type: "no_sales_with_stock",
  category: "slow_turnover",
  priority: "important",
  status: "open",
  title: "Разберите 8 SKU без заказов за 7 дней",
  summary: "Восстановите продажи по неликвидным товарам.",
  reason: "Разберите 8 SKU без заказов за 7 дней. Нет заказов при значимом остатке.",
  next_step: "Проанализируйте причины.",
  evidence: [
    { kind: "metric", label: "stock_available/current_day", value: "51-386", unit: "count" },
    { kind: "metric", label: "orders/last_7d", value: "0", unit: "orders" },
  ],
  limitations: [],
  ctas: [],
  metric_date: "2026-06-30",
  affected_entities: {
    skus: Array.from({ length: 8 }, (_, index) => ({ sku: `sku-${index}` })),
  },
  operational_detail: {
    status: "available",
    format_version: "actions_detail_v2",
    why: { headline: "Краткий вывод", text: "8 товаров не продаются уже 7 дней." },
    action_plan: {
      headline: "Что сделать",
      steps: [
        {
          step_id: "s1",
          title: "Проанализируйте причины",
          instruction: "Проверьте цену и карточку.",
          why_this_step: "",
          expected_result: "",
        },
        {
          step_id: "s2",
          title: "Запустите мероприятия",
          instruction: "Корректировка цены и реклама.",
          why_this_step: "",
          expected_result: "",
        },
      ],
    },
    object_actions: [
      {
        entity_type: "sku",
        entity_id: "801000100001",
        name: "Ведро с отжимом 12 л",
        metrics: [
          { label: "Остаток", value: "386" },
          { label: "Заказы 7д", value: "0" },
          { label: "Выручка 7д", value: "0 ₽" },
        ],
        recommended_action: "Проанализируйте и оживите спрос",
      },
    ],
  },
};

const cardHtml = renderToStaticMarkup(
  <ActionCard action={sampleAction} selected={false} onSelect={() => undefined} />,
);
assert.doesNotMatch(cardHtml, /slow_turnover/);
assert.doesNotMatch(cardHtml, /stock_available/);
assert.doesNotMatch(cardHtml, /last_7d/);
assert.match(cardHtml, /Оборачиваемость/);
assert.match(cardHtml, /8 SKU/);
assert.doesNotMatch(cardHtml, /Открыто/);
assert.match(cardHtml, /Высокий приоритет/);

assert.equal(actionObjectsSummary(sampleAction), "8 SKU");
assert.ok(actionCardFooter(sampleAction).includes("30.06.2026"));

const focusHtml = renderToStaticMarkup(
  <ActionsFocusCard
    focus={{
      status: "available",
      source: "deterministic",
      headline: "Разберите неликвидный остаток",
      summary: "8 SKU не имеют заказов за 7 дней.",
      top_categories: ["slow_turnover"],
    }}
    priorityAction={
      {
        title: "Разберите неликвидный остаток",
        summary: "Краткий фокус.",
      } as React.ComponentProps<typeof ActionsFocusCard>["priorityAction"]
    }
    priorityCount={3}
    hasOpenActions
    onOpenPriority={() => undefined}
  />,
);
assert.doesNotMatch(focusHtml, /slow_turnover/);
assert.doesNotMatch(focusHtml, /Оборачиваемость/);
assert.doesNotMatch(focusHtml, /Выполнено/);
assert.doesNotMatch(focusHtml, /Анализ от/);
assert.match(focusHtml, /AI-фокус дня/);
assert.doesNotMatch(focusHtml, /Главное действие:/);
assert.match(focusHtml, /Перейти к действию/);
assert.match(focusHtml, /Разберите неликвидный остаток/);
assert.equal(focusHeadlineText("Фокус: test"), "test");

const detailHtml = renderToStaticMarkup(
  <ActionDetailPanel
    action={sampleAction}
    loading={false}
    mutationError=""
    isMutating={false}
    onToggleChecked={() => undefined}
    onMarkIrrelevant={() => undefined}
    onAskAI={async () => undefined}
  />,
);
assert.match(detailHtml, /Ситуация/);
assert.match(detailHtml, /Почему это важно/);
assert.match(detailHtml, /План действий/);
assert.match(detailHtml, /Проанализируйте причины/);
assert.match(detailHtml, /Ведро с отжимом 12 л/);
assert.match(detailHtml, /SKU 801000100001/);
assert.match(detailHtml, /386 шт\./);
assert.doesNotMatch(detailHtml, /overflow-x-auto/);
assert.doesNotMatch(detailHtml, /\bcount\b/i);
assert.doesNotMatch(detailHtml, /\bRUB\b/);
assert.match(detailHtml, /Спросить AI/);
assert.doesNotMatch(detailHtml, /Ограничения/);
assert.match(detailHtml, /Нет заказов при значимом остатке/);
assert.doesNotMatch(detailHtml, /slow_turnover/);
assert.doesNotMatch(detailHtml, /available_stock/);
assert.doesNotMatch(detailHtml, /Открыть список SKU/);
assert.doesNotMatch(detailHtml, /Ещё действия/);
assert.match(detailHtml, /Отметить неактуальным/);

const cols = parseObjectMetricColumns(sampleAction.operational_detail?.object_actions?.[0]?.metrics);
assert.equal(cols.stock, "386 шт.");
assert.equal(cols.orders, "0");

assert.equal(formatMetricLabel("stock_available/current_day"), "Остаток");
assert.equal(formatMetricLabel("orders/last_7d"), "Заказы за 7 дней");
assert.equal(formatMetricValue("1200", "RUB").includes("₽"), true);
assert.equal(formatMetricValue("5", "count"), "5 шт.");
assert.equal(
  removeRepeatedLeadingTitle("Разберите 8 SKU. Нет заказов.", "Разберите 8 SKU"),
  "Нет заказов.",
);

console.log("actions-ui.test.tsx: ok");
