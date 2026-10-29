import {
  buildFunnelMainConclusionDisplay,
  buildFunnelTransitions,
  detectHighlightedTransition,
  detectWeakestTransition,
  markWeakestTransition,
  normalizeTransitionText,
  formatFunnelFallbackDisplay,
  resolveFunnelAIEmptyMessage,
  resolveHighlightedStageKey,
} from "@/lib/funnel-transitions-display";
import {
  buildFunnelStageCompactChainSubtitle,
  formatFunnelDeltaText,
  getFixedFunnelStageWidth,
  getFunnelStageCompactLabel,
  normalizeFunnelStageLabel,
} from "@/lib/sales-funnel-display";
import type { FunnelAIAnalysis, FunnelStage } from "@/lib/dashboard-home-types";
import { testFunnelStage } from "@/lib/test-helpers/funnel-fixtures";

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

function run() {
  assert(formatFunnelDeltaText(null, true) === "нет сравнения", "null delta compact");
  assert(formatFunnelDeltaText(undefined, true) === "нет сравнения", "undefined delta compact");
  assert(formatFunnelDeltaText(0, { compact: true, stageKey: "product_views" }) === "без изменений", "zero delta compact");
  assert(formatFunnelDeltaText(-5, { compact: true, stageKey: "product_views" }) === "−5%", "negative delta compact");
  assert(
    formatFunnelDeltaText(-5, { stageKey: "product_views", periodType: "current_7d" }) ===
      "Открытий карточек стало на 5% меньше, чем неделю назад",
    "negative delta full event label",
  );
  assert(
    formatFunnelDeltaText(-51, { compact: true, withPeriodHint: true, stageKey: "impressions", periodType: "current_7d" }).includes(
      "показов",
    ),
    "top-stage period hint with event noun",
  );

  assert(normalizeFunnelStageLabel("delivered_units") === "Доставленные единицы", "delivered_units label");
  assert(normalizeFunnelStageLabel("", "Выкупы") === "Доставленные единицы", "legacy выкупы label");
  assert(normalizeFunnelStageLabel("hits_view") === "Увидели товар", "hits_view label");
  assert(!normalizeFunnelStageLabel("hits_view").includes("hits_view"), "no raw key in UI label");

  assert(getFunnelStageCompactLabel("product_views") === "карточка", "compact stage label");
  assert(getFunnelStageCompactLabel("impressions") === "Показы", "compact impressions");
  const chain = buildFunnelStageCompactChainSubtitle([
    { key: "impressions", label: "Показы", value: 1 },
    { key: "product_views", label: "Просмотры", value: 1 },
    { key: "orders", label: "Заказы", value: 1 },
  ]);
  assert(chain.includes("Показы") && chain.includes("→"), "compact header chain");

  const demoTransitions = buildFunnelTransitions(demoStages);
  assert(demoTransitions[0]?.compactLabel === "Показы → карточка", "first transition compact label");
  assert(demoTransitions[1]?.compactLabel === "Карточка → корзина", "second transition compact label");

  assert(getFixedFunnelStageWidth(0, 0) === 100, "totalStages=0 safe");
  assert(getFixedFunnelStageWidth(0, 2) === 100 && getFixedFunnelStageWidth(1, 2) === 58, "two stages");
  const widths3 = [0, 1, 2].map((i) => getFixedFunnelStageWidth(i, 3));
  assert(JSON.stringify(widths3) === JSON.stringify([100, 79, 58]), "three stages smooth taper");
  const widths5 = [0, 1, 2, 3, 4].map((i) => getFixedFunnelStageWidth(i, 5));
  assert(JSON.stringify(widths5) === JSON.stringify([100, 89.5, 79, 68.5, 58]), "five stages smooth taper");
  const widths6 = [0, 1, 2, 3, 4, 5].map((i) => getFixedFunnelStageWidth(i, 6));
  assert(widths6[0] === 100 && widths6[5] === 58 && widths6[1] === 91.6, "six stages formula");

  const transitions = buildFunnelTransitions(demoStages);
  assert(transitions.length === 4, "transitions = stages - 1");
  assert(transitions[1].conversionLabel === "24,2%", "backend conversion used");
  assert(transitions[2].conversionCaption === "Конверсия из корзины в заказ", "cart-to-order caption");
  assert(
    transitions[3].conversionCaption === "Доля доставленных единиц от заказанных",
    "orders-to-delivered caption",
  );
  assert(transitions[0].conversionCaption.length > 0, "generic transitions still get a caption");

  const stagesWithExplicitDelta: FunnelStage[] = [
    { key: "add_to_cart", label: "Добавления в корзину", value: 129, conversion_from_previous: 24.2 },
    {
      key: "orders",
      label: "Заказанные единицы",
      value: 60,
      conversion_from_previous: 46.5,
      conversion_delta_pp: -3.1,
    },
  ];
  const explicitDeltaTransitions = buildFunnelTransitions(stagesWithExplicitDelta);
  assert(explicitDeltaTransitions[0].conversionDeltaPp === -3.1, "explicit conversion_delta_pp used");

  const stagesWithPreviousConversion: FunnelStage[] = [
    { key: "add_to_cart", label: "Добавления в корзину", value: 129, conversion_from_previous: 24.2 },
    {
      key: "orders",
      label: "Заказанные единицы",
      value: 60,
      conversion_from_previous: 46.5,
      previous_conversion_from_previous: 50,
    },
  ];
  const derivedDeltaTransitions = buildFunnelTransitions(stagesWithPreviousConversion);
  assert(derivedDeltaTransitions[0].conversionDeltaPp === -3.5, "conversion_delta_pp derived from previous period");

  const aiWithWeakest: FunnelAIAnalysis = {
    status: "available",
    weakest_stage: { key: "add_to_cart", label: "Карточка → корзина" },
    comparisons: {
      conversion_metrics: [
        {
          key: "product_views_to_add_to_cart",
          label: "Карточка → Корзина",
          delta: "-4,81 п.п.",
        },
      ],
    },
  };
  const withAi = markWeakestTransition(buildFunnelTransitions(demoStages, aiWithWeakest), aiWithWeakest, undefined, demoStages);
  const weak = withAi.find((t) => t.highlightKind === "weak_spot");
  assert(weak?.key === "product_views_to_add_to_cart", "weakest_stage.key resolves weak_spot");
  assert(weak?.conversionDeltaLabel?.includes("4,81") === true, "conversion delta pp from backend");
  assert(weak?.volumeDeltaSecondary?.includes("добавлений") === true, "count delta shown with event noun");
  assert(!weak?.volumeDeltaLabel.includes("по объёму"), "no abstract volume phrasing");

  const positiveOnly = buildFunnelTransitions([
    testFunnelStage({ key: "a", value: 100, change_vs_previous_day_pct: 5 }),
    testFunnelStage({ key: "b", value: 80, change_vs_previous_day_pct: 3 }),
    testFunnelStage({ key: "c", value: 8, change_vs_previous_day_pct: 1 }),
  ]);
  const bottleneck = positiveOnly.find((t) => t.highlightKind === "bottleneck");
  assert(bottleneck?.key === "b_to_c", "no deterioration → bottleneck by min conversion");
  assert(!positiveOnly.some((t) => t.highlightKind === "weak_spot"), "no weak_spot without deterioration");

  const noAi = detectWeakestTransition(
    buildFunnelTransitions([
      testFunnelStage({ key: "a", value: 100 }),
      testFunnelStage({ key: "b", value: 50, change_vs_previous_day_pct: -5 }),
      testFunnelStage({ key: "c", value: 10, change_vs_previous_day_pct: -30 }),
    ]),
  );
  assert(noAi === "b_to_c", "fallback without conversion Δpp: narrowest conversion");
  const noAiHighlight = detectHighlightedTransition(
    buildFunnelTransitions([
      testFunnelStage({ key: "a", value: 100 }),
      testFunnelStage({ key: "b", value: 50, change_vs_previous_day_pct: -5 }),
      testFunnelStage({ key: "c", value: 10, change_vs_previous_day_pct: -30 }),
    ]),
  );
  assert(noAiHighlight?.kind === "weak_spot", "narrowest + negative volume upgrades to weak_spot");

  const healthyNarrow = detectHighlightedTransition(
    buildFunnelTransitions([
      testFunnelStage({ key: "a", value: 100, change_vs_previous_day_pct: 2 }),
      testFunnelStage({ key: "b", value: 80, change_vs_previous_day_pct: 1 }),
      testFunnelStage({ key: "c", value: 8, change_vs_previous_day_pct: 0 }),
    ]),
  );
  assert(healthyNarrow?.kind === "bottleneck", "narrowest without deterioration stays bottleneck");

  const conversionFallback = detectHighlightedTransition(
    buildFunnelTransitions([
      testFunnelStage({ key: "a", value: 100 }),
      testFunnelStage({
        key: "b",
        value: 50,
        conversion_from_previous: 50,
        conversion_delta_pp: -1,
        change_vs_previous_day_pct: -5,
      }),
      testFunnelStage({
        key: "c",
        value: 10,
        conversion_from_previous: 20,
        conversion_delta_pp: -8,
        change_vs_previous_day_pct: -30,
      }),
    ]),
  );
  assert(conversionFallback?.key === "b_to_c", "fallback picks most negative conversion Δpp");
  assert(conversionFallback?.kind === "weak_spot", "negative conversion Δpp yields weak_spot");

  const highlight = detectHighlightedTransition(
    buildFunnelTransitions([
      testFunnelStage({ key: "impressions", value: 1000 }),
      testFunnelStage({ key: "product_views", value: 500, change_vs_previous_day_pct: 2 }),
      testFunnelStage({ key: "orders", value: 50, change_vs_previous_day_pct: 1 }),
    ]),
  );
  assert(highlight?.kind === "bottleneck", "min conversion without negatives is bottleneck");

  assert(detectWeakestTransition([]) === null, "no transitions → null");

  const normA = normalizeTransitionText("Открыли карточку → Добавили в корзину");
  assert(normA.includes("карточка") && normA.includes("корзина"), "label normalization");

  const conclusion = buildFunnelMainConclusionDisplay({
    funnelAnalysis: {
      status: "available",
      main_conclusion:
        "Основная просадка: показы -8,75%. Ключевое слабое место — переход «Показы → Просмотры карточки». Нижние этапы не выглядят главной причиной. Но компенсации недостаточно. Что сделать: обновить фото; проверить рекламу; сверить цены.",
      weakest_stage: { explanation: "Просадка в середине воронки." },
      comparisons_commentary: { previous_period: "Верх стабилен. Низ просел. Ещё одно предложение." },
    },
  });
  assert(conclusion.source === "ai", "AI main_conclusion preferred");
  assert(conclusion.text.includes("Что сделать"), "main conclusion from main_conclusion field");
  assert(conclusion.text.split(/(?<=[.!?])\s+/).filter(Boolean).length <= 7, "commentary truncated to seven sentences");

  const explanationOnly = buildFunnelMainConclusionDisplay({
    funnelAnalysis: {
      status: "available",
      weakest_stage: { explanation: "Просадка в середине воронки." },
      comparisons_commentary: { previous_period: "Верх стабилен. Низ просел. Ещё одно предложение." },
    },
  });
  assert(explanationOnly.source === "ai", "AI explanation preferred when main_conclusion missing");
  assert(explanationOnly.text.includes("Просадка в середине"), "main conclusion from weakest_stage.explanation");

  const commentaryOnly = buildFunnelMainConclusionDisplay({
    funnelAnalysis: {
      status: "available",
      comparisons_commentary: { previous_period: "Верх стабилен. Низ просел. Ещё одно предложение." },
    },
  });
  assert(commentaryOnly.source === "ai", "commentary used when explanation missing");
  assert(commentaryOnly.text.includes("Верх стабилен"), "main conclusion from commentary");

  const aiMissingFallback = buildFunnelMainConclusionDisplay({
    funnelAnalysis: { status: "not_enough_data" },
    primaryDropStage: "add_to_cart",
    primaryDropMessage: "Конверсия в корзину снизилась.",
  });
  assert(aiMissingFallback.source === "fallback", "fallback when AI missing");
  assert(aiMissingFallback.text.includes("корзину"), "fallback uses primary_drop_message");

  const aiEmptyWithFallback = buildFunnelMainConclusionDisplay({
    funnelAnalysis: { status: "available", comparisons_commentary: { previous_period: "" } },
    primaryDropMessage: "Конверсия в корзину снизилась.",
  });
  assert(aiEmptyWithFallback.source === "fallback", "fallback when AI available but empty");
  assert(aiEmptyWithFallback.text.includes("корзину"), "primary_drop when AI text empty");

  const emptyConclusion = buildFunnelMainConclusionDisplay({
    funnelAnalysis: { status: "not_enough_data" },
  });
  assert(emptyConclusion.source === "empty", "empty when no AI and no fallback");
  assert(!emptyConclusion.text.includes("Основное ухудшение"), "transitions do not drive main conclusion");

  const stageHighlight = resolveHighlightedStageKey(withAi);
  assert(stageHighlight?.stageKey === "add_to_cart", "weak transition maps to toStage key");

  const pending = resolveFunnelAIEmptyMessage({ aiStatus: "source_pending" });
  assert(!pending.toLowerCase().includes("запустите"), "no technical pending wording");

  const fallbackParsed = formatFunnelFallbackDisplay(
    "Основное ухудшение: конверсия снизилась на -4.81 п.п. к вчера / -1.37 п.п. к среднему за 7 дней.",
  );
  assert(fallbackParsed.metricChips.length === 2, "fallback extracts pp chips");
  assert(!fallbackParsed.lead.includes("п.п."), "pp moved to chips");

  console.log("funnel-transitions-display.test.ts: ok");
}

run();
