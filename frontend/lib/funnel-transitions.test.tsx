import { renderToStaticMarkup } from "react-dom/server";
import { FunnelChart } from "@/components/dashboard/funnel-chart";
import { FunnelMainConclusion } from "@/components/dashboard/funnel-main-conclusion";
import { FunnelTransitions } from "@/components/dashboard/funnel-transitions";
import { SalesFunnelModule } from "@/components/dashboard/sales-funnel-module";
import type { FunnelAIAnalysis, FunnelStage } from "@/lib/dashboard-home-types";
import { testFunnelStage } from "@/lib/test-helpers/funnel-fixtures";
import {
  buildFunnelMainConclusionDisplay,
  buildFunnelTransitions,
  markWeakestTransition,
  resolveHighlightedStageKey,
} from "@/lib/funnel-transitions-display";
import { buildSalesFunnelDisplay, formatFunnelDeltaText } from "@/lib/sales-funnel-display";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

const demoStages: FunnelStage[] = [
  { key: "impressions", label: "Показы", value: 4025, change_vs_previous_day_pct: -4.2 },
  { key: "product_views", label: "Просмотры карточки", value: 533, conversion_from_previous: 13.2, change_vs_previous_day_pct: -13.6 },
  { key: "add_to_cart", label: "Добавления в корзину", value: 129, conversion_from_previous: 24.2, change_vs_previous_day_pct: -27.9 },
  { key: "orders", label: "Заказанные единицы", value: 60, conversion_from_previous: 46.5, change_vs_previous_day_pct: -9.1 },
  { key: "delivered_units", label: "Доставленные единицы", value: 54, conversion_from_previous: 90, change_vs_previous_day_pct: -10 },
];

function stagesForCount(count: number): FunnelStage[] {
  const keys = ["impressions", "product_views", "add_to_cart", "orders", "delivered_units", "custom_stage"];
  return Array.from({ length: count }, (_, index) => ({
    key: keys[index] ?? `stage_${index}`,
    label: `Очень длинное название этапа номер ${index + 1} для проверки переноса`,
    value: index === 0 ? 999999 : Math.max(1, 1000 / (index + 1)),
    change_vs_previous_day_pct: index % 2 === 0 ? -5 : index === 3 ? 0 : 3,
  }));
}

function run() {
  for (const count of [1, 2, 3, 5, 6]) {
    const stages = stagesForCount(count);
    const html = renderToStaticMarkup(<FunnelChart status="available" stages={stages} />);
    const display = buildSalesFunnelDisplay("available", stages);
    assert(display.length === count, `render ${count} stages`);
    assert(!html.includes("NaN"), `no NaN for ${count} stages`);
    assert(!html.includes("undefined"), `no undefined for ${count} stages`);
    for (const stage of display) {
      assert(stage.widthPercent >= 58 && stage.widthPercent <= 100, "width in range");
    }
    if (count <= 6) {
      assert(html.includes('aria-hidden="true"') || count === 1, "stage icons when <=6 stages");
    }
  }

  assert(formatFunnelDeltaText(null, true) === "нет сравнения", "missing delta label");
  assert(formatFunnelDeltaText(0, { compact: true, stageKey: "orders" }) === "без изменений", "zero delta label");

  const aiAnalysis: FunnelAIAnalysis = {
    status: "available",
    weakest_stage: { key: "add_to_cart" },
    comparisons: {
      conversion_metrics: [{ key: "product_views_to_add_to_cart", delta: "-4,81 п.п." }],
    },
  };
  const demoTransitions = markWeakestTransition(buildFunnelTransitions(demoStages, aiAnalysis), aiAnalysis, undefined, demoStages);
  const highlighted = resolveHighlightedStageKey(demoTransitions);
  assert(highlighted?.stageKey === "add_to_cart", "weak transition highlights toStage in pyramid");

  const pyramidHtml = renderToStaticMarkup(
    <FunnelChart
      status="available"
      stages={demoStages}
      highlightedStageKey={highlighted?.stageKey}
      highlightKind={highlighted?.kind ?? null}
    />,
  );
  assert(pyramidHtml.includes('data-funnel-stage-highlight="weak_spot"'), "weak stage segment styled in pyramid");
  assert(pyramidHtml.includes("ring-rose-300") || pyramidHtml.includes("bg-rose-50"), "weak pyramid rose styling");
  assert(pyramidHtml.includes("Слабое место"), "weak spot pill visible");

  const noWeakHtml = renderToStaticMarkup(<FunnelChart status="available" stages={demoStages} />);
  assert(!noWeakHtml.includes('data-funnel-stage-highlight="weak_spot"'), "no weak styling without highlight");

  const transitionsHtml = renderToStaticMarkup(<FunnelTransitions transitions={demoTransitions} />);
  assert(transitionsHtml.includes("Показы → карточка") || transitionsHtml.includes("Карточка → корзина"), "compact transition labels");
  assert(transitionsHtml.includes("Приоритет №1") || transitionsHtml.includes("Слабое место"), "weak spot badge on demo");
  assert(transitionsHtml.includes("Где теряется воронка"), "transitions section title");
  assert(transitionsHtml.includes("bg-rose-50"), "weak spot rose highlight");
  assert(transitionsHtml.includes("4,81"), "conversion pp visible");
  assert(!transitionsHtml.includes("h-1") && !transitionsHtml.includes("progress"), "no progress bars in transitions");
  assert(transitionsHtml.includes("добавлений") || transitionsHtml.includes("открытий") || transitionsHtml.includes("заказанных"), "event noun in count delta");
  assert(!transitionsHtml.includes("по объёму"), "no abstract volume phrasing in transitions");
  assert(!transitionsHtml.includes("объём этапа"), "no объём этапа in transitions");
  assert(transitionsHtml.includes("Конверсия"), "conversion caption visible");
  assert(transitionsHtml.includes("→"), "values subline from→to visible");
  assert(!transitionsHtml.includes("bg-emerald-200"), "default cards not green-filled");
  assert(!transitionsHtml.includes("bg-amber-200"), "default cards not yellow-filled");
  assert(!transitionsHtml.includes("hits_view"), "no raw keys");
  assert(!transitionsHtml.includes("line-clamp"), "transitions not clamped");

  const neutralTransition = [
    {
      ...buildFunnelTransitions([
        testFunnelStage({ key: "impressions", value: 100, change_vs_previous_day_pct: 2 }),
        testFunnelStage({ key: "product_views", value: 80, change_vs_previous_day_pct: 1 }),
      ])[0]!,
      highlightKind: null,
    },
  ];
  const defaultHtml = renderToStaticMarkup(<FunnelTransitions transitions={neutralTransition} />);
  assert(defaultHtml.includes("bg-white"), "default transition neutral white background");
  assert(!defaultHtml.includes("Приоритет №1"), "no weak badge without highlight");

  const bottleneckTransitions = buildFunnelTransitions([
    testFunnelStage({ key: "a", value: 100, change_vs_previous_day_pct: 2 }),
    testFunnelStage({ key: "b", value: 80, change_vs_previous_day_pct: 1 }),
    testFunnelStage({ key: "c", value: 8, change_vs_previous_day_pct: 0 }),
  ]);
  const bottleneckHtml = renderToStaticMarkup(<FunnelTransitions transitions={bottleneckTransitions} />);
  assert(bottleneckHtml.includes("Самый узкий переход"), "bottleneck badge");
  assert(bottleneckHtml.includes("bg-sky-50"), "bottleneck blue highlight");
  assert(!bottleneckHtml.includes("Приоритет №1"), "no weak spot without deterioration");

  const conclusion = buildFunnelMainConclusionDisplay({
    funnelAnalysis: {
      status: "available",
      weakest_stage: { explanation: "Основная просадка в середине воронки." },
    },
  });
  assert(conclusion.source === "ai", "demo main conclusion from AI");
  assert(conclusion.text.split(/(?<=[.!?])\s+/).filter(Boolean).length <= 2, "main conclusion compact at most two sentences");

  const conclusionHtml = renderToStaticMarkup(
    <FunnelMainConclusion text={conclusion.text} source={conclusion.source} />,
  );
  assert(conclusionHtml.includes("border-sky-200") && conclusionHtml.includes("Главный вывод"), "unified info card");
  assert(conclusionHtml.includes('data-funnel-main-conclusion-source="ai"'), "conclusion source exposed for tests");

  const unavailableHtml = renderToStaticMarkup(
    <SalesFunnelModule
      salesFunnel={{ status: "not_enough_data", current_completed_date: "", previous_completed_date: "", stages: [], daily_series: [] }}
      funnelAnalysis={{ status: "not_enough_data", ai_analysis: { status: "not_enough_data", related_signals: [], recommended_focus: [] } }}
    />,
  );
  assert(!unavailableHtml.includes("4 025"), "no fake pyramid when unavailable");

  const moduleHtml = renderToStaticMarkup(
    <SalesFunnelModule
      salesFunnel={{ status: "available", current_completed_date: "2026-07-07", previous_completed_date: "2026-07-06", stages: demoStages, daily_series: [] }}
      funnelAnalysis={{
        status: "available",
        ai_analysis: {
          status: "available",
          weakest_stage: { key: "product_views" },
          main_conclusion: "Нужно проверить CTR и выдачу.",
          related_signals: [],
          recommended_focus: [],
        },
      }}
    />,
  );
  assert(!moduleHtml.toLowerCase().includes("запустите генерацию"), "no technical AI empty state");
  assert(
    moduleHtml.includes("количество событий"),
    "updated footer copy",
  );
  assert(!moduleHtml.includes("по объёму"), "module has no по объёму");
  assert(!moduleHtml.includes("объём этапа"), "module has no объём этапа");
  assert(moduleHtml.includes("Главный инсайт"), "insight banner when weak spot present");
  assert(moduleHtml.includes("Главный вывод"), "main conclusion card");
  assert(
    moduleHtml.includes("открытий") || moduleHtml.includes("Открытий") || moduleHtml.includes("карточек"),
    "insight uses concrete event wording",
  );
  assert(!moduleHtml.includes("line-clamp"), "funnel module avoids clamping meaningful text");

  const customWeakStages: FunnelStage[] = [
    { key: "stage_a", label: "Этап A", value: 1000 },
    {
      key: "stage_b",
      label: "Этап B",
      value: 400,
      conversion_delta_pp: -2,
      change_vs_previous_day_pct: -2,
    },
    {
      key: "stage_c",
      label: "Этап C",
      value: 50,
      conversion_delta_pp: -40,
      change_vs_previous_day_pct: -40,
    },
  ];
  const customTransitions = markWeakestTransition(buildFunnelTransitions(customWeakStages), undefined, undefined, customWeakStages);
  const customHighlight = resolveHighlightedStageKey(customTransitions);
  assert(customHighlight?.stageKey === "stage_c", "weak highlights arbitrary toStage not hardcoded cart");

  const emptyConclusionHtml = renderToStaticMarkup(
    <SalesFunnelModule
      salesFunnel={{
        status: "available",
        current_completed_date: "2026-07-07",
        previous_completed_date: "2026-07-06",
        daily_series: [],
        stages: [
          testFunnelStage({ key: "impressions", value: 100, change_vs_previous_day_pct: 2 }),
          testFunnelStage({ key: "product_views", value: 80, change_vs_previous_day_pct: 1 }),
          testFunnelStage({ key: "orders", value: 40, change_vs_previous_day_pct: 1 }),
        ],
      }}
      funnelAnalysis={{ status: "not_enough_data", ai_analysis: { status: "not_enough_data", related_signals: [], recommended_focus: [] } }}
    />,
  );
  assert(!emptyConclusionHtml.includes("Главный вывод"), "hide empty main conclusion card");
  assert(!emptyConclusionHtml.includes("data-funnel-insight-banner"), "no empty insight banner");
  assert(emptyConclusionHtml.includes("Самый узкий переход"), "healthy narrow step stays bottleneck");

  console.log("funnel-transitions.test.tsx: ok");
}

run();
