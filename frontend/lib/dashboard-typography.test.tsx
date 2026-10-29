import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { SellerPulseSection } from "@/components/dashboard/seller-pulse-section";
import { dashboardTypography } from "@/lib/dashboard-typography";
import type { DashboardHomeResponse, KPICard } from "@/lib/dashboard-home-types";

function makeKpi(overrides: Partial<KPICard> = {}): KPICard {
  return {
    key: "revenue",
    label: "Выручка",
    value: "100 000 ₽",
    ...overrides,
  };
}

const longSummary =
  "Кабинет держится на сильных категориях, но часть рекламного бюджета уходит в кампании с очень слабой окупаемостью и это требует немедленного пересмотра стратегии продвижения по всему ассортименту.";

const longActionLabel =
  "Остановить или резко ограничить самые убыточные кампании с ДРР выше ста процентов по выручке";

const longActionDescription =
  "В первую очередь пересмотрите кампании с наибольшим расходом без заказов и снизьте ставки на SKU с низкой маржинальностью до стабилизации показателей.";

const narrative: NonNullable<DashboardHomeResponse["seller_pulse"]["narrative"]> = {
  status: "ready",
  tone: "warning",
  headline: "Пульс продавца",
  summary: longSummary,
  key_points: [
    {
      title: "Недельная выручка снижается",
      text: "Снижение затрагивает несколько ключевых SKU и требует проверки цен, остатков и рекламных ставок.",
    },
  ],
  important_notes: [
    {
      title: "Проверьте остатки",
      text: "Часть SKU может уйти в out-of-stock уже на этой неделе.",
    },
  ],
  quick_actions: [
    {
      label: longActionLabel,
      description: longActionDescription,
      kind: "action",
      priority: "high",
      href: "/app/actions?action=unified%3Apack%3A1%3A2026-%3Aaction_001",
    },
  ],
};

const html = renderToStaticMarkup(
  <SellerPulseSection
    data={{
      kpis: [
        makeKpi({
          key: "revenue",
          value: "117 924 ₽",
          delta: { abs: -13_057, pct: -10, label: "к вчера" },
        }),
        makeKpi({ key: "orders", label: "Заказы", value: "84" }),
        makeKpi({ key: "average_order_value", label: "Средний чек", value: "592 ₽" }),
        makeKpi({ key: "drr", label: "ДРР", value: "222,5%" }),
      ],
      trend_7d: [],
      narrative,
    }}
    metricDate="2026-06-30"
    dashboardStatus="ready"
  />,
);

assert.match(html, /<h2[^>]*>Пульс продавца<\/h2>/, "module title renders as h2");
assert.ok(html.includes("AI-резюме по кабинету"), "AI resume static label rendered");
assert.ok(html.includes("Недельная выручка снижается"), "key point title rendered inside AI resume");
assert.match(html, /<h3[^>]*>Что важно знать<\/h3>/, "knowledge card section title renders as h3");
assert.ok(html.includes("Проверьте остатки"), "important note title rendered in knowledge card");
assert.match(html, /<h3[^>]*>Быстрые действия<\/h3>/, "quick actions section title renders as h3");
assert.ok(html.includes(longSummary), "long AI summary is not truncated");
assert.ok(html.includes(longActionLabel), "long quick action title is not truncated");
assert.ok(html.includes(longActionDescription), "long quick action description is not truncated");
assert.ok(!html.includes("/app/advertising"), "legacy advertising route not used");
assert.ok(html.includes("/app/actions?action="), "quick action links to actions detail");
assert.ok(!html.includes("line-clamp"), "seller pulse does not use line-clamp");
assert.ok(!html.includes("truncate"), "seller pulse does not use truncate");
assert.ok(html.includes(dashboardTypography.kpiValue.split(" ")[0]), "KPI value uses dashboard typography scale");
assert.ok(html.includes("text-slate-950"), "primary KPI values use slate-950");
assert.ok(!html.includes("text-slate-400"), "seller pulse avoids very muted slate-400 for body text");

console.log("dashboard-typography.test.tsx: ok");
