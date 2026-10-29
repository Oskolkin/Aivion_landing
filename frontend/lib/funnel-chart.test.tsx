import { renderToStaticMarkup } from "react-dom/server";
import { FunnelChart } from "@/components/dashboard/funnel-chart";
import type { FunnelStage } from "@/lib/dashboard-home-types";
import { testFunnelStage } from "@/lib/test-helpers/funnel-fixtures";
import { buildSalesFunnelDisplay, formatFunnelNumber } from "@/lib/sales-funnel-display";
import { buildFunnelTransitions, markWeakestTransition, resolveHighlightedStageKey } from "@/lib/funnel-transitions-display";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

/** Произвольные значения — не demo 4025/533/129/60/54. */
const arbitraryStages: FunnelStage[] = [
  testFunnelStage({ key: "impressions", value: 1000, change_vs_previous_day_pct: -2 }),
  testFunnelStage({ key: "product_views", value: 250, change_vs_previous_day_pct: -8 }),
  testFunnelStage({ key: "add_to_cart", value: 75, change_vs_previous_day_pct: -15 }),
  testFunnelStage({ key: "orders", value: 30, change_vs_previous_day_pct: 4 }),
];

function run() {
  const display = buildSalesFunnelDisplay("available", arbitraryStages);
  const html = renderToStaticMarkup(<FunnelChart status="available" stages={arbitraryStages} />);

  assert(display.length === 4, "four stages rendered");
  assert(display.every((s) => !s.conversionLabel.includes("NaN") && !s.conversionLabel.includes("Infinity")), "no invalid percents");

  for (const v of [1000, 250, 75, 30]) {
    assert(html.includes(formatFunnelNumber(v)), `value ${v} visible`);
  }

  for (const pct of ["100,0%", "25,0%", "30,0%", "40,0%"]) {
    assert(html.includes(pct), `transition ${pct} visible`);
  }

  assert(html.includes("от карточек"), "caption from previous stage");
  assert(html.includes("%") || html.includes("Нет данных") || html.includes("Без изменений") || html.includes("открытий") || html.includes("показов"), "compact count delta");
  assert(!html.includes("по объёму"), "chart has no по объёму");
  assert(!html.includes("объём этапа"), "chart has no объём этапа");
  assert(html.includes("открытий") || html.includes("показов") || html.includes("добавлений"), "event noun in chart badges");

  const weakStages: FunnelStage[] = [
    { key: "stage1", label: "Этап 1", value: 100 },
    {
      key: "stage2",
      label: "Этап 2",
      value: 50,
      conversion_delta_pp: -2,
      change_vs_previous_day_pct: -5,
    },
    {
      key: "stage3",
      label: "Этап 3",
      value: 10,
      conversion_delta_pp: -12,
      change_vs_previous_day_pct: -30,
    },
  ];
  const transitions = markWeakestTransition(buildFunnelTransitions(weakStages));
  const highlight = resolveHighlightedStageKey(transitions);
  const weakHtml = renderToStaticMarkup(
    <FunnelChart
      status="available"
      stages={weakStages}
      highlightedStageKey={highlight?.stageKey}
      highlightKind={highlight?.kind ?? null}
    />,
  );
  assert(highlight?.stageKey === "stage3", "weak highlights stage3 for stage2→stage3");
  assert(weakHtml.includes('data-funnel-stage-highlight="weak_spot"'), "pyramid weak segment marker");
  assert(weakHtml.includes("Слабое место"), "weak spot callout");

  console.log("funnel-chart.test.tsx: ok");
}

run();
