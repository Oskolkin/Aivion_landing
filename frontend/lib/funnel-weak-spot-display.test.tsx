import { renderToStaticMarkup } from "react-dom/server";
import { FunnelAIAnalysisContent } from "@/components/dashboard/funnel-ai-analysis";
import {
  buildFunnelAnalysisDisplayModel,
  buildPreviousComparisonGroups,
  hasNegativePreviousFunnelDynamics,
} from "@/lib/funnel-ai-analysis-view";
import {
  buildVolumeVsConversionNote,
  buildWeakSpotInterpretation,
  isMetricRepeatExplanation,
  messageMatchesTransition,
  pickConversionDeltaForTransition,
  pickStrongestNegativeConversion,
  pickStrongestNegativeStageVolume,
  pickWeakestExplanation,
  resolveWeakestTransition,
} from "@/lib/funnel-weak-spot-display";
import type { FunnelAIAnalysis, FunnelStage } from "@/lib/dashboard-home-types";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

const fullStages: FunnelStage[] = [
  { key: "impressions", label: "Показы", value: 4025, conversion_from_top: 100, change_vs_previous_day_pct: -4.2 },
  { key: "product_views", label: "Просмотры карточки", value: 533, conversion_from_previous: 13.24, change_vs_previous_day_pct: -13.6 },
  {
    key: "add_to_cart",
    label: "Добавления в корзину",
    value: 129,
    conversion_from_previous: 24.2,
    change_vs_previous_day_pct: -27.9,
  },
  {
    key: "orders",
    label: "Заказанные единицы",
    value: 60,
    conversion_from_previous: 46.51,
    change_vs_previous_day_pct: -9.1,
  },
  { key: "delivered_units", label: "Доставленные единицы", value: 54, conversion_from_previous: 90, change_vs_previous_day_pct: -3 },
];

const conversionMetrics = [
  {
    key: "product_views_to_add_to_cart",
    label: "Карточка → Корзина",
    delta: "-4,81 п.п. к прошлой сводке",
    tone: "negative" as const,
  },
  {
    key: "add_to_cart_to_orders",
    label: "Корзина → Заказанные единицы",
    delta: "-12,30 п.п. к прошлой сводке",
    tone: "negative" as const,
  },
  {
    key: "impressions_to_product_views",
    label: "Показы → Карточка",
    delta: "-1,00 п.п. к прошлой сводке",
    tone: "negative" as const,
  },
];

/** Extracts the rendered weak-spot block markup (between heading and next section). */
function weakSpotChunk(html: string): string {
  if (!html.includes("Что могло усилить просадку")) return "";
  return html.split("Главное ухудшение перехода")[1]?.split("К прошлой сводке")[0] ?? "";
}

function run() {
  // ---- Unit: delta picking helpers stay universal --------------------------
  const deltasCartOrders = pickConversionDeltaForTransition("add_to_cart_to_orders", conversionMetrics);
  assert(deltasCartOrders.previous.includes("12,30"), "pickConversionDeltaForTransition cart→orders");
  const deltasCardCart = pickConversionDeltaForTransition("product_views_to_add_to_cart", conversionMetrics);
  assert(deltasCardCart.previous.includes("4,81"), "pickConversionDeltaForTransition card→cart");

  const strongest = pickStrongestNegativeConversion(conversionMetrics);
  assert(strongest?.transitionId === "add_to_cart_to_orders", "strongest negative is cart→orders");

  const strongestVolume = pickStrongestNegativeStageVolume(fullStages);
  assert(strongestVolume?.transitionId === "product_views_to_add_to_cart", "strongest negative volume → add_to_cart transition");

  // ---- Universal interpretation text per transition -------------------------
  assert(buildWeakSpotInterpretation("impressions_to_views").includes("выдаче"), "impressions interpretation");
  assert(buildWeakSpotInterpretation("product_views_to_add_to_cart").includes("карточек"), "card→cart interpretation");
  assert(buildWeakSpotInterpretation("add_to_cart_to_orders").includes("финальном шаге"), "cart→orders interpretation");
  assert(buildWeakSpotInterpretation("orders_to_delivered").includes("исполнения заказа"), "orders→delivered interpretation");
  assert(buildWeakSpotInterpretation("unknown").includes("теряются"), "unknown interpretation fallback");

  // ---- Metric-repeat heuristic ----------------------------------------------
  assert(isMetricRepeatExplanation("конверсия снизилась на -4.81 п.п. к прошлой сводке"), "pp + comparison ref is repeat");
  assert(isMetricRepeatExplanation("-4,81 п.п. / -1,37 п.п. к среднему за 7 дней"), "deltas repeat");
  assert(!isMetricRepeatExplanation(buildWeakSpotInterpretation("add_to_cart_to_orders")), "interpretation is not a repeat");

  // ---- pickWeakestExplanation prefers interpretation ------------------------
  const interpCart = buildWeakSpotInterpretation("product_views_to_add_to_cart");
  const repeatAi = "конверсия снизилась на -4.81 п.п. к прошлой сводке";
  assert(
    pickWeakestExplanation("product_views_to_add_to_cart", repeatAi, interpCart) === interpCart,
    "metric-repeat AI explanation replaced by interpretation",
  );
  const goodAi = "Покупатели открывают карточки, но реже добавляют товар в корзину из-за слабых карточек.";
  assert(
    pickWeakestExplanation("product_views_to_add_to_cart", goodAi, interpCart) === goodAi,
    "genuine matching interpretation kept",
  );
  const conflictAi = "Основная потеря сейчас на этапе Показы → Просмотры карточки.";
  assert(
    pickWeakestExplanation("product_views_to_add_to_cart", conflictAi, interpCart) === interpCart,
    "conflicting transition explanation replaced",
  );

  // ---- Test 1: hidden when no negative previous dynamics --------------------
  const noDeteriorationAnalysis: FunnelAIAnalysis = {
    status: "available",
    weakest_stage: { status: "available", key: "product_views_to_add_to_cart", label: "Карточка → корзина", value: "24,2%" },
    comparisons: {
      stage_metrics: [
        { key: "impressions", label: "Показы", delta: "+2,0% к прошлой сводке", tone: "positive" },
        { key: "add_to_cart", label: "Добавления в корзину", delta: "+5,0% к прошлой сводке", tone: "positive" },
      ],
      conversion_metrics: [
        { key: "product_views_to_add_to_cart", label: "Карточка → Корзина", delta: "+1,00 п.п. к прошлой сводке", tone: "positive" },
        { key: "add_to_cart_to_orders", label: "Корзина → Заказанные единицы", delta: "0,00 п.п. к прошлой сводке", tone: "neutral" },
      ],
    },
  };
  const modelHidden = buildFunnelAnalysisDisplayModel(noDeteriorationAnalysis, { stages: fullStages });
  assert(modelHidden.weakestStage === null, "weakSpot null when all previous deltas >= 0");
  const hiddenHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={noDeteriorationAnalysis} salesStages={fullStages} detailsOpen={false} />,
  );
  assert(!hiddenHtml.includes("Слабое место воронки"), "WeakSpotSection not rendered without deterioration");
  assert(!hiddenHtml.includes("Недостаточно данных"), "no empty-state placeholder");

  // ---- Funnel-only criterion ------------------------------------------------
  // Case 1: only revenue/AOV dropped → no funnel weak spot.
  const businessOnlyAnalysis: FunnelAIAnalysis = {
    status: "available",
    weakest_stage: { status: "available", key: "product_views_to_add_to_cart", label: "Карточка → корзина", value: "24,2%" },
    comparisons: {
      stage_metrics: [
        { key: "impressions", label: "Показы", delta: "+1,0% к прошлой сводке", tone: "positive" },
        { key: "add_to_cart", label: "Добавления в корзину", delta: "+3,0% к прошлой сводке", tone: "positive" },
        { key: "orders", label: "Заказанные единицы", delta: "0,0% к прошлой сводке", tone: "neutral" },
      ],
      conversion_metrics: [
        { key: "product_views_to_add_to_cart", label: "Карточка → Корзина", delta: "+2,00 п.п. к прошлой сводке", tone: "positive" },
        { key: "add_to_cart_to_orders", label: "Корзина → Заказанные единицы", delta: "+1,50 п.п. к прошлой сводке", tone: "positive" },
      ],
      business_metrics: [
        { key: "revenue", label: "Выручка", delta: "-10,0% к прошлой сводке", tone: "negative" },
        { key: "average_order_value", label: "Средний чек", delta: "-5,0% к прошлой сводке", tone: "negative" },
      ],
    },
  };
  assert(!hasNegativePreviousFunnelDynamics(businessOnlyAnalysis), "revenue/AOV drop is not funnel deterioration");
  const modelBusinessOnly = buildFunnelAnalysisDisplayModel(businessOnlyAnalysis, { stages: fullStages });
  assert(modelBusinessOnly.weakestStage === null, "weakSpot null when only revenue/AOV dropped");
  // business metrics are still present in the comparison model (not dropped).
  const businessGroups = buildPreviousComparisonGroups(businessOnlyAnalysis);
  assert(businessGroups.business.some((b) => b.key === "revenue"), "revenue stays in comparison model");
  assert(businessGroups.business.some((b) => b.key === "average_order_value"), "AOV stays in comparison model");
  const businessOnlyHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={businessOnlyAnalysis} salesStages={fullStages} detailsOpen={false} />,
  );
  assert(!businessOnlyHtml.includes("Слабое место воронки"), "WeakSpotSection hidden for revenue/AOV-only drop");
  assert(businessOnlyHtml.includes("Выручка"), "revenue still rendered in comparison cards");
  assert(businessOnlyHtml.includes("Средний чек"), "AOV still rendered in comparison cards");

  // Case 2: a funnel stage dropped (conversions ≥ 0) → weak spot shows.
  const stageOnlyAnalysis: FunnelAIAnalysis = {
    status: "available",
    comparisons: {
      stage_metrics: [{ key: "add_to_cart", label: "Добавления в корзину", delta: "-20,0% к прошлой сводке", tone: "negative" }],
      conversion_metrics: [
        { key: "product_views_to_add_to_cart", label: "Карточка → Корзина", delta: "+0,50 п.п. к прошлой сводке", tone: "positive" },
      ],
      business_metrics: [{ key: "revenue", label: "Выручка", delta: "-2,0% к прошлой сводке", tone: "negative" }],
    },
  };
  assert(hasNegativePreviousFunnelDynamics(stageOnlyAnalysis), "negative stage delta is funnel deterioration");
  const modelStageOnly = buildFunnelAnalysisDisplayModel(stageOnlyAnalysis, { stages: fullStages });
  assert(modelStageOnly.weakestStage !== null, "weakSpot shown for negative stage delta");
  assert(modelStageOnly.weakestStage?.transitionId !== "unknown", "negative stage maps to a transition, not revenue");

  // Case 4: previous funnel deltas ≥ 0 but 7-day negative → still hidden.
  const sevenDayOnlyAnalysis: FunnelAIAnalysis = {
    status: "available",
    weakest_stage: { status: "available", key: "product_views_to_add_to_cart", label: "Карточка → корзина" },
    comparisons: {
      stage_metrics: [{ key: "add_to_cart", label: "Добавления в корзину", delta: "+4,0% к прошлой сводке", tone: "positive" }],
      conversion_metrics: [
        { key: "product_views_to_add_to_cart", label: "Карточка → Корзина", delta: "+1,00 п.п. к прошлой сводке", tone: "positive" },
      ],
      seven_day_average: [
        { key: "add_to_cart", label: "Добавления в корзину", delta: "-12,2% к среднему за 7 дней", tone: "negative" },
        { key: "product_views_to_add_to_cart", label: "Карточка → Корзина", delta: "-3,00 п.п. к среднему за 7 дней", tone: "negative" },
      ],
    },
  };
  assert(!hasNegativePreviousFunnelDynamics(sevenDayOnlyAnalysis), "7-day negativity does not count");
  const modelSevenDay = buildFunnelAnalysisDisplayModel(sevenDayOnlyAnalysis, { stages: fullStages });
  assert(modelSevenDay.weakestStage === null, "weakSpot null when only 7-day is negative");

  // ---- Test 2: shown when negative previous conversion (card→cart) ----------
  const cardCartAnalysis: FunnelAIAnalysis = {
    status: "available",
    weakest_stage: { status: "available", key: "product_views_to_add_to_cart", label: "Карточка → корзина", value: "24,2%" },
    comparisons: {
      conversion_metrics: [
        { key: "product_views_to_add_to_cart", label: "Карточка → Корзина", delta: "-4,81 п.п. к прошлой сводке", tone: "negative" },
      ],
    },
  };
  const modelCardCart = buildFunnelAnalysisDisplayModel(cardCartAnalysis, { stages: fullStages });
  assert(modelCardCart.weakestStage?.label === "Карточка → корзина", "card→cart label");
  const cardCartHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={cardCartAnalysis} salesStages={fullStages} detailsOpen={false} />,
  );
  assert(cardCartHtml.includes("Что это значит"), "renders 'Что это значит'");
  assert(!cardCartHtml.includes("Почему важно"), "no 'Почему важно'");
  const cardCartWeak = weakSpotChunk(cardCartHtml);
  assert(cardCartWeak.includes("Карточка → корзина"), "card→cart main label in block");

  // ---- Test 3: universal text for impressions → product_views ---------------
  const impressionsAnalysis: FunnelAIAnalysis = {
    status: "available",
    weakest_stage: { status: "available", key: "impressions_to_product_views", label: "Показы → карточка" },
    comparisons: {
      conversion_metrics: [
        { key: "impressions_to_product_views", label: "Показы → Карточка", delta: "-1,00 п.п. к прошлой сводке", tone: "negative" },
      ],
    },
  };
  const modelImpr = buildFunnelAnalysisDisplayModel(impressionsAnalysis, { stages: fullStages });
  assert(modelImpr.weakestStage?.label === "Показы → карточка", "impressions label");
  assert(modelImpr.weakestStage?.explanation.includes("выдаче") === true, "impressions interpretation used");
  assert(!modelImpr.weakestStage?.explanation.includes("корзин"), "impressions text not about cart");
  const imprHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={impressionsAnalysis} salesStages={fullStages} detailsOpen={false} />,
  );
  const imprWeak = weakSpotChunk(imprHtml);
  assert(imprWeak.includes("Показы → карточка"), "impressions main label");
  assert(!imprWeak.includes("Карточка → корзина"), "impressions block not about card→cart");

  // ---- Test 4: universal text for add_to_cart → orders ----------------------
  const cartOrdersAnalysis: FunnelAIAnalysis = {
    status: "available",
    weakest_stage: { status: "available", key: "add_to_cart_to_orders", label: "Корзина → заказанные единицы", value: "46,5%" },
    comparisons: {
      conversion_metrics: [
        { key: "add_to_cart_to_orders", label: "Корзина → Заказанные единицы", delta: "-12,30 п.п. к прошлой сводке", tone: "negative" },
      ],
    },
  };
  const modelCartOrders = buildFunnelAnalysisDisplayModel(cartOrdersAnalysis, { stages: fullStages });
  assert(modelCartOrders.weakestStage?.label === "Корзина → заказанные единицы", "cart→orders label");
  assert(modelCartOrders.weakestStage?.explanation.includes("финальном шаге") === true, "cart→orders interpretation used");
  const cartOrdersHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={cartOrdersAnalysis} salesStages={fullStages} detailsOpen={false} />,
  );
  const cartOrdersWeak = weakSpotChunk(cartOrdersHtml);
  assert(cartOrdersWeak.includes("Корзина → заказанные единицы"), "cart→orders main label");
  assert(!cartOrdersWeak.includes("Карточка → корзина"), "cart→orders not about card→cart");

  // ---- Test 5: universal text for orders → delivered ------------------------
  const deliveredAnalysis: FunnelAIAnalysis = {
    status: "available",
    weakest_stage: { status: "available", key: "orders_to_delivered_units", label: "Заказ → доставка", value: "90%" },
    comparisons: {
      conversion_metrics: [
        { key: "orders_to_delivered_units", label: "Заказанные → Доставленные", delta: "-6,00 п.п. к прошлой сводке", tone: "negative" },
      ],
    },
  };
  const modelDelivered = buildFunnelAnalysisDisplayModel(deliveredAnalysis, { stages: fullStages });
  assert(modelDelivered.weakestStage?.label.includes("доставленные") === true, "orders→delivered canonical label");
  assert(modelDelivered.weakestStage?.explanation.includes("исполнения заказа") === true, "orders→delivered interpretation used");
  assert(
    modelDelivered.weakestStage?.explanation.includes("отмены") === true ||
      modelDelivered.weakestStage?.explanation.includes("логистика") === true,
    "orders→delivered mentions fulfillment risks",
  );
  const deliveredHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={deliveredAnalysis} salesStages={fullStages} detailsOpen={false} />,
  );
  const deliveredWeak = weakSpotChunk(deliveredHtml);
  assert(!deliveredWeak.includes("Карточка → корзина"), "orders→delivered not about card→cart");

  // ---- Test 6: AI explanation that just repeats numbers is replaced ---------
  const repeatAnalysis: FunnelAIAnalysis = {
    status: "available",
    weakest_stage: {
      status: "available",
      key: "product_views_to_add_to_cart",
      label: "Карточка → корзина",
      value: "24,2%",
      explanation: "конверсия снизилась на -4.81 п.п. к прошлой сводке",
    },
    comparisons: {
      conversion_metrics: [
        { key: "product_views_to_add_to_cart", label: "Карточка → Корзина", delta: "-4,81 п.п. к прошлой сводке", tone: "negative" },
      ],
    },
  };
  const modelRepeat = buildFunnelAnalysisDisplayModel(repeatAnalysis, {
    primaryDropMessage: "Основное ухудшение: конверсия снизилась на -4.81 п.п. к вчера / -1.37 п.п. к среднему за 7 дней.",
    stages: fullStages,
  });
  assert(
    modelRepeat.weakestStage?.explanation === buildWeakSpotInterpretation("product_views_to_add_to_cart"),
    "repeated-number AI explanation replaced by interpretation",
  );
  assert(!modelRepeat.weakestStage?.explanation.includes("4,81"), "interpretation has no repeated pp");
  assert(!modelRepeat.weakestStage?.explanation.includes("4.81"), "interpretation has no repeated pp (dot)");

  // ---- Test 7: universal volume vs conversion note --------------------------
  const noteOrders = buildVolumeVsConversionNote("add_to_cart_to_orders", fullStages, "-12,30 п.п.");
  assert(noteOrders?.includes("Заказанные единицы") === true, "note uses orders stage label");
  assert(noteOrders?.includes("Корзина → заказанные единицы") === true, "note uses transition label");
  assert(
    noteOrders?.includes("-12,30 п.п.") === true || noteOrders?.includes("−12,30 п.п.") === true,
    "note keeps conversion delta",
  );
  assert(!noteOrders?.includes("Добавили в корзину"), "note not hardcoded to add_to_cart label");

  const noteCard = buildVolumeVsConversionNote("product_views_to_add_to_cart", fullStages, "-4,81 п.п.");
  assert(noteCard?.includes("Добавили в корзину") === true, "card→cart note uses add_to_cart stage label");
  assert(noteCard?.includes("Карточка → корзина") === true, "card→cart note uses transition label");

  const noteNoDelta = buildVolumeVsConversionNote("add_to_cart_to_orders", fullStages, "");
  assert(noteNoDelta === null, "no conversion delta → no note");

  // ---- Test 8: legacy terms never appear in the block -----------------------
  const legacyHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={cartOrdersAnalysis} salesStages={fullStages} detailsOpen={false} />,
  );
  const legacyWeak = weakSpotChunk(legacyHtml);
  assert(!legacyWeak.includes("Оформленные заказы"), "no 'Оформленные заказы'");
  assert(!legacyWeak.toLowerCase().includes("выкуп"), "no 'выкуп'");

  // ---- messageMatchesTransition guards --------------------------------------
  assert(
    messageMatchesTransition("Конверсия из корзины в заказанные единицы снизилась.", "add_to_cart_to_orders"),
    "matching message",
  );
  assert(!messageMatchesTransition("Основное ухудшение на переходе Карточка → корзина.", "add_to_cart_to_orders"), "conflict detected");

  // ---- resolveWeakestTransition does not pick a non-negative transition -----
  const stableWeakAnalysis: FunnelAIAnalysis = {
    status: "available",
    weakest_stage: { status: "available", key: "orders_to_delivered_units", label: "Заказ → доставка" },
  };
  const resolved = resolveWeakestTransition(
    stableWeakAnalysis,
    { stages: [{ key: "delivered_units", label: "Доставленные единицы", value: 54, change_vs_previous_day_pct: 5 }] },
    [],
  );
  assert(resolved.transitionId === "unknown", "weakest_stage without negative signal falls through to unknown");

  console.log("funnel-weak-spot-display.test.tsx: ok");
}

run();
