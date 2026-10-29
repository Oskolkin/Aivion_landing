import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  buildFunnelInsightText,
  mapSalesFunnelToViewModel,
} from "@/lib/sales-funnel-view-model";
import {
  formatFunnelDeltaPct,
  formatFunnelDeltaText,
  formatFunnelNumber,
  formatFunnelPercent,
  formatFunnelPpDelta,
} from "@/lib/sales-funnel-display";
import type { FunnelStage } from "@/lib/dashboard-home-types";
import { testFunnelStage } from "@/lib/test-helpers/funnel-fixtures";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

const __dirname = dirname(fileURLToPath(import.meta.url));

function run() {
  assert(formatFunnelNumber(78687) === "78 687" || formatFunnelNumber(78687) === "78 687", "locale integers");
  assert(formatFunnelPercent(13.34) === "13,3%", "percent formatting");
  assert(formatFunnelPpDelta(-3.4) === "−3,4 п.п.", "pp formatting with typographic minus");
  assert(formatFunnelPpDelta(2.1) === "+2,1 п.п.", "positive pp");
  assert(formatFunnelPpDelta(null) === null, "null pp hidden");
  assert(formatFunnelDeltaPct(-55.16) === "−55,2%", "volume delta pct");
  assert(formatFunnelDeltaText(null, true) === "нет сравнения", "missing previous period");
  assert(formatFunnelDeltaText(18.8, { compact: true, stageKey: "add_to_cart" }) === "+18,8%", "positive count");
  assert(
    formatFunnelDeltaText(-55.16, { stageKey: "product_views", periodType: "current_7d" }).includes("Открытий карточек"),
    "full event wording",
  );

  const stages: FunnelStage[] = [
    testFunnelStage({ key: "impressions", value: 1000, change_vs_previous_day_pct: -10 }),
    testFunnelStage({
      key: "product_views",
      value: 200,
      conversion_from_previous: 20,
      conversion_delta_pp: -5,
      change_vs_previous_day_pct: -40,
    }),
    testFunnelStage({
      key: "add_to_cart",
      value: 80,
      conversion_from_previous: 40,
      conversion_delta_pp: 2,
      change_vs_previous_day_pct: -5,
    }),
    testFunnelStage({ key: "orders", value: 40, conversion_from_previous: 50, change_vs_previous_day_pct: 1 }),
    testFunnelStage({ key: "delivered_units", value: 36, conversion_from_previous: 90, change_vs_previous_day_pct: 0 }),
  ];

  const vm = mapSalesFunnelToViewModel({
    status: "available",
    stages,
    funnelAnalysis: {
      status: "available",
      weakest_stage: { key: "impressions_to_product_views" },
      main_conclusion: "Нужно проверить CTR карточек.",
    },
  });

  assert(vm.stages.length === 5, "five stages");
  assert(vm.transitions.length === 4, "four transitions");
  assert(vm.weakTransition?.highlightKind === "weak_spot", "authoritative weak spot");
  assert(vm.highlightedStageKey === "product_views", "highlight toStage");
  assert(vm.insightText != null && vm.insightText.includes("Показы → карточка"), "insight from weak transition");
  assert(vm.insightText != null && !vm.insightText.includes("объём"), "insight has no abstract volume");
  assert(
    vm.insightText != null && (vm.insightText.includes("открытий") || vm.insightText.includes("Открытий")),
    "insight names the event",
  );
  assert(Boolean(vm.mainConclusion?.text.includes("CTR")), "main conclusion present");
  assert(vm.transitions.filter((t) => t.highlightKind === "weak_spot").length === 1, "single accent card");

  const noWeak = mapSalesFunnelToViewModel({
    status: "available",
    stages: [
      testFunnelStage({ key: "impressions", value: 100 }),
      testFunnelStage({ key: "product_views", value: 80, change_vs_previous_day_pct: 2 }),
      testFunnelStage({ key: "orders", value: 40, change_vs_previous_day_pct: 1 }),
    ],
  });
  assert(noWeak.weakTransition == null, "no weak spot without deterioration");
  assert(noWeak.insightText == null, "no insight banner without weak spot");
  assert(noWeak.mainConclusion == null, "hide empty conclusion");

  const zeroDenom = mapSalesFunnelToViewModel({
    status: "available",
    stages: [
      testFunnelStage({ key: "impressions", value: 0 }),
      testFunnelStage({ key: "product_views", value: 10 }),
    ],
  });
  assert(zeroDenom.transitions[0]?.conversionLabel === "н/д", "zero denominator → nd");

  const partial = mapSalesFunnelToViewModel({
    status: "available",
    stages: [
      testFunnelStage({ key: "impressions", value: 500 }),
      testFunnelStage({ key: "product_views", value: null }),
      testFunnelStage({ key: "orders", value: 20 }),
    ],
  });
  assert(partial.stages.length === 3, "partial stages still mapped");
  assert(!JSON.stringify(partial).includes("NaN"), "no NaN in view model");

  const insightFromExplanation = buildFunnelInsightText({
    weakTransition: vm.weakTransition,
    funnelAnalysis: {
      status: "available",
      weakest_stage: { explanation: "Короткое объяснение слабого места." },
    },
  });
  assert(insightFromExplanation === "Короткое объяснение слабого места.", "prefer structured explanation");

  const noInsight = buildFunnelInsightText({ weakTransition: null });
  assert(noInsight === null, "no insight without weak transition");

  const src = readFileSync(join(__dirname, "sales-funnel-view-model.ts"), "utf8");
  for (const forbidden of ["78687", "78 687", "13,3%", "55,2%"]) {
    assert(!src.includes(forbidden), `no hardcoded ${forbidden}`);
  }

  console.log("sales-funnel-view-model.test.ts: ok");
}

run();
