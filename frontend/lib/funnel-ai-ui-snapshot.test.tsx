/**
 * Generates static HTML DOM snapshots of the funnel AI block.
 * Run from frontend/: npm run snapshot:funnel
 */
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { FunnelAIAnalysisContent } from "@/components/dashboard/funnel-ai-analysis";
import type { FunnelAIAnalysis } from "@/lib/dashboard-home-types";

const __dirname = dirname(fileURLToPath(import.meta.url));

const collapsedFixture: FunnelAIAnalysis = {
  status: "available",
  weakest_stage: {
    status: "available",
    key: "add_to_cart",
    label: "Переход в корзину",
    metric: "Добавления в корзину",
    value: "129",
    explanation: "Самое сильное отрицательное отклонение до оформления заказа.",
    delta_previous: "-27.9%",
    delta_7d: "-12.2%",
    tone: "negative",
  },
  comparisons: {
    previous_period: [
      { key: "impressions", label: "Показы", delta: "-4.2%", tone: "negative" },
      { key: "product_views", label: "Просмотры карточки", delta: "-13.6%", tone: "negative" },
      { key: "add_to_cart", label: "Добавления в корзину", delta: "-27.9%", tone: "negative" },
      { key: "orders", label: "Заказанные единицы", delta: "+7.5%", tone: "positive" },
    ],
    seven_day_average: [
      { key: "impressions", label: "Показы", delta: "+0.6%", tone: "neutral" },
      { key: "product_views", label: "Просмотры карточки", delta: "-7.3%", tone: "negative" },
      { key: "add_to_cart", label: "Добавления в корзину", delta: "-12.2%", tone: "negative" },
      { key: "orders", label: "Заказанные единицы", delta: "+7.1%", tone: "positive" },
      { key: "cart_to_order", label: "Корзина → Заказанные единицы", delta: "+8.42 п.п.", tone: "positive" },
    ],
  },
  comparisons_commentary: {
    previous_period: "Верх воронки просел сильнее оформления.",
    seven_day_average: "Заказы и конверсия в заказ выше среднего за 7 дней.",
  },
  drop_factors: [
    {
      key: "views_decline",
      label: "Снижение просмотров карточки",
      metric: "533 просмотра карточки",
      evidence_level: "likely",
      explanation: "Просмотры ниже показов.",
      limitation: "Факт снижения подтверждён, связь с падением следующего этапа вероятна.",
      tone: "warning",
    },
    {
      key: "cart_add_decline",
      label: "Резкое падение добавлений в корзину",
      metric: "129 добавлений",
      evidence_level: "confirmed",
      explanation: "Прямой сигнал ослабления этапа выбора товара.",
      tone: "negative",
    },
    {
      key: "aov_drop",
      label: "Снижение среднего чека",
      metric: "1 156 ₽",
      evidence_level: "confirmed",
      explanation: "Усиливает финансовый эффект дня.",
      limitation: "Это подтверждённый финансовый сигнал, а не доказанная причина просадки переходов.",
      tone: "negative",
    },
    {
      key: "stock_pressure",
      label: "Дефицит по значимым товарам",
      evidence_level: "limited",
      limitation: "Остатки доступны текущим срезом.",
      tone: "warning",
    },
    {
      key: "inefficient_ads",
      label: "Слабая эффективность части рекламы",
      metric: "5 кампаний с ДРР выше 100%",
      evidence_level: "hypothesis",
      explanation:
        "Часть рекламного трафика расходует бюджет при слабом результате. Это может указывать на менее эффективное привлечение.",
      limitation: "Причинность с просадкой этапов воронки не доказана.",
      tone: "warning",
    },
  ],
  check_directions: [
    { key: "1", label: "SKU и категории с падением", text: "Сопоставить просмотры и корзину по SKU и категориям.", tone: "neutral" },
    { key: "2", label: "Отмены после оформления", text: "Разложить отмены по SKU и этапам заказа.", tone: "neutral" },
  ],
  details_sections: [
    {
      title: "Где просадка",
      items: [
        {
          text: "Добавления в корзину просели сильнее остальных этапов.",
          metrics: ["129 добавлений", "−27,9% к прошлой сводке", "−12,2% к среднему за 7 дней"],
        },
      ],
    },
    { title: "Итог", items: [{ text: "Давление сосредоточено до этапа оформления.", metrics: [] }] },
  ],
};

function loadAnalysis(): FunnelAIAnalysis {
  const livePath = process.env.FUNNEL_LIVE_JSON?.trim();
  if (livePath && existsSync(livePath)) {
    const parsed = JSON.parse(readFileSync(livePath, "utf8")) as FunnelAIAnalysis;
    return { ...parsed, status: parsed.status ?? "available" };
  }
  return collapsedFixture;
}

const analysis = loadAnalysis();
const live = Boolean(process.env.FUNNEL_LIVE_JSON?.trim());

const collapsed = renderToStaticMarkup(
  <FunnelAIAnalysisContent analysis={analysis} detailsOpen={false} />,
);
const expanded = renderToStaticMarkup(
  <FunnelAIAnalysisContent analysis={analysis} detailsOpen={true} />,
);

const wrap = (body: string, title: string) => `<!DOCTYPE html>
<html lang="ru"><head><meta charset="utf-8"/><title>${title}</title>
<style>body{font-family:system-ui,sans-serif;background:#f1f5f9;margin:0;padding:24px 16px}.wrap{max-width:720px;margin:0 auto}</style>
</head><body><div class="wrap">${body}</div></body></html>`;

const outDir = join(__dirname, "../../docs/ui-snapshots");
mkdirSync(outDir, { recursive: true });
const collapsedName = live ? "funnel-ai-analysis-collapsed-live.html" : "funnel-ai-analysis-collapsed.html";
const expandedName = live ? "funnel-ai-analysis-expanded-live.html" : "funnel-ai-analysis-expanded.html";
const polishedCollapsedName = live
  ? "funnel-ai-analysis-polished-collapsed-live.html"
  : "funnel-ai-analysis-polished-collapsed.html";
const polishedExpandedName = live
  ? "funnel-ai-analysis-polished-expanded-live.html"
  : "funnel-ai-analysis-polished-expanded.html";
writeFileSync(join(outDir, collapsedName), wrap(collapsed, "Funnel AI collapsed"), "utf8");
writeFileSync(join(outDir, expandedName), wrap(expanded, "Funnel AI expanded"), "utf8");
writeFileSync(join(outDir, polishedCollapsedName), wrap(collapsed, "Funnel AI polished collapsed"), "utf8");
writeFileSync(join(outDir, polishedExpandedName), wrap(expanded, "Funnel AI polished expanded"), "utf8");
console.log(`Wrote ${collapsedName}, ${expandedName}, ${polishedCollapsedName}, ${polishedExpandedName}`);
