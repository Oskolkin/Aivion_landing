import { renderToStaticMarkup } from "react-dom/server";
import { FunnelAIAnalysisContent } from "@/components/dashboard/funnel-ai-analysis";
import { normalizeDashboardHome } from "@/lib/normalize-dashboard-home";
import {
  buildFunnelAnalysisDisplayModel,
  buildPreviousComparisonGroups,
  buildSevenDayComparisonGroups,
  canonicalComparisonKey,
  classifyComparisonBucket,
  hasPreviousComparisonSectionContent,
  hasSevenDayComparisonSectionContent,
} from "@/lib/funnel-ai-analysis-view";
import type { FunnelAIAnalysis, FunnelStage } from "@/lib/dashboard-home-types";
import { testDashboardHome, testFunnelStage } from "@/lib/test-helpers/funnel-fixtures";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

const salesStages: FunnelStage[] = [
  testFunnelStage({ key: "product_views", value: 533, conversion_from_previous: 13.24 }),
  testFunnelStage({ key: "add_to_cart", value: 129, conversion_from_previous: 24.2, change_vs_previous_day_pct: -27.9 }),
];

const structuredAnalysis: FunnelAIAnalysis = {
  status: "available",
  weakest_stage: {
    status: "available",
    key: "product_views_to_add_to_cart",
    label: "Карточка → корзина",
    value: "24,2%",
    delta_previous: "-4,81 п.п.",
  },
  comparisons: {
    stage_metrics: [
      { key: "orders", label: "Оформленные заказы", delta: "-9.09% к 20.05.2026", tone: "negative" },
      { key: "delivered_units", label: "Выкуп", delta: "-5.00%", tone: "negative" },
    ],
    conversion_metrics: [{ key: "product_views_to_add_to_cart", label: "Карточка → Корзина", delta: "-4.81 п.п. к прошлому дню", tone: "negative" }],
    business_metrics: [{ key: "average_order_value", label: "Средний чек", delta: "-17.95% к 20.05.2026", tone: "negative" }],
    context_signals: [{ key: "history_status", label: "История сравнения", value: "Сравнение ограничено: доступно меньше 7 дней истории.", delta: "limited" }],
  },
  comparisons_commentary: {
    previous_summary: "Верх воронки просел сильнее низа.",
    seven_day_average: "Заказы держатся выше среднего за 7 дней.",
  },
  drop_factors: [
    {
      key: "cart_add_decline",
      label: "Просадка добавлений в корзину",
      metric: "Добавления в корзину",
      delta: "-27.9% к прошлой сводке",
      evidence_level: "confirmed",
      explanation: "Падает середина воронки.",
      tone: "negative",
    },
    {
      key: "views_decline",
      label: "Снижение просмотров карточки",
      metric: "533 просмотра карточки",
      evidence_level: "likely",
      explanation: "Просмотры ниже показов.",
      limitation: "Факт снижения подтверждён, связь с падением следующего этапа вероятна.",
    },
    {
      key: "aov_drop",
      label: "Снижение среднего чека",
      metric: "1 156 ₽",
      evidence_level: "confirmed",
      explanation: "Чек ниже вчера.",
      limitation: "Это подтверждённый финансовый сигнал, а не доказанная причина просадки переходов.",
    },
    { key: "stock_pressure", evidence_level: "limited", limitation: "Истории по остаткам недостаточно." },
    { key: "inefficient_ads", evidence_level: "hypothesis", limitation: "Нет причинной связи по кампаниям." },
  ],
  check_directions: [
    { key: "p2", label: "Второй шаг", text: "Проверить карточки", priority: "2", reason: "Переход просел", data_needed: "Отчёт карточек." },
    { key: "p1", label: "Первый шаг", text: "Проверить корзину", priority: "1", reason: "Главный провал", data_needed: "SKU-level воронка в MVP недоступна; используйте косвенные признаки." },
  ],
  details_sections: [
    { title: "Что смягчает вывод", items: [] },
    { title: "Итог", items: [{ text: "Ключевая проблема в середине воронки.", metrics: ["-4,81 п.п."] }] },
    { title: "Что подтверждает вывод", items: [{ text: "Корзина просела.", metrics: ["-27,9%"] }] },
    {
      title: "Ограничения анализа",
      items: [{ text: "История ограничена.", metrics: [] }],
    },
  ],
};

function assertRemovedSectionsAbsent(html: string) {
  const forbidden = [
    "Сигналы просадки и возможные усилители",
    "Сигналы просадки",
    "Факторы, которые могли усилить эффект",
    "Направления проверки",
    "Что проверить",
    "Подробнее",
    "Что подтверждает вывод",
    "Что смягчает вывод",
    "Связанные сигналы",
    "Ограничения анализа",
    ">Итог<",
    "Система не нашла дополнительных факторов",
    "Пока нет отдельных направлений проверки",
  ];
  for (const text of forbidden) {
    assert(!html.includes(text), `removed section must not render: ${text}`);
  }
}

function run() {
  const model = buildFunnelAnalysisDisplayModel(structuredAnalysis, {
    primaryDropStage: "Просмотры карточки → Добавления в корзину",
    primaryDropMessage: "Основное ухудшение: конверсия снизилась на -4.81 п.п. к вчера / -1.37 п.п. к среднему за 7 дней.",
    stages: salesStages,
  });
  assert(model.weakestStage?.label === "Карточка → корзина", "weak spot uses card-to-cart label");
  assert(model.weakestStage?.conversionRate === "24,2%", "conversion rate from stages");
  assert(model.comparisonGroups.stages[0]?.label === "Заказанные единицы", "legacy label replaced");
  assert(model.comparisonGroups.stages[1]?.label === "Доставленные единицы", "buyout label replaced");
  assert(model.comparisonGroups.conversions.length === 1, "conversion group mapped");
  assert(model.comparisonGroups.business.length === 1, "business group mapped");
  assert(model.checkDirections[0]?.priority === 1, "priority sorted");
  assert(model.checkDirections[1]?.priority === 2, "priority sorted 2");

  const compactHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent
      analysis={structuredAnalysis}
      primaryDropStage="Просмотры карточки → Добавления в корзину"
      primaryDropMessage="Основное ухудшение: конверсия снизилась на -4.81 п.п. к вчера / -1.37 п.п. к среднему за 7 дней."
      salesStages={salesStages}
      isExpanded={false}
      onToggle={() => undefined}
    />,
  );
  assert(compactHtml.includes("Что могло усилить просадку"), "weak spot section title");
  assert(compactHtml.includes("Что это значит"), "analysis header");
  assert(compactHtml.includes("К прошлой сводке"), "previous comparison card");
  assert(compactHtml.includes("К среднему за 7 дней"), "seven-day comparison card");
  assertRemovedSectionsAbsent(compactHtml);
  assert(!compactHtml.includes("Оформленные заказы"), "forbidden label hidden");
  assert(!compactHtml.toLowerCase().includes("выкуп"), "forbidden buyout hidden");

  const legacyPayloadHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={structuredAnalysis} isExpanded={true} onToggle={() => undefined} />,
  );
  assertRemovedSectionsAbsent(legacyPayloadHtml);

  // Test A: structured shape with empty arrays still shows groups from items (not empty section)
  const structuredEmptyArrays: FunnelAIAnalysis = {
    status: "available",
    comparisons: {
      stage_metrics: [],
      conversion_metrics: [],
      business_metrics: [],
      previous_period: [{ key: "orders", label: "Заказанные единицы", delta: "-1%", tone: "negative" }],
      seven_day_average: [{ key: "orders", label: "Заказанные единицы", delta: "+2%", tone: "positive" }],
    },
    comparisons_commentary: { previous_summary: "Сводка есть.", seven_day_average: "7д сводка." },
  };
  const normalizedLegacy = normalizeDashboardHome(
    testDashboardHome({
      ai_diagnostics: {
        account_summary: { status: "available", risk_drivers: [] },
        what_changed: { status: "available", groups: [] },
        top_actions: [],
        category_summaries: [],
        sales_funnel: { status: "available", current_completed_date: "", previous_completed_date: "", stages: [], daily_series: [] },
        funnel_analysis: { status: "available", ai_analysis: structuredEmptyArrays },
        stock_coverage: { status: "available", segments: [] },
        ad_efficiency: { status: "available", points: [] },
        risk_distribution: { status: "available", groups: [] },
      },
      data_quality_notes: [],
    }),
  ).ai_diagnostics.funnel_analysis.ai_analysis!;
  const prevGroups = buildPreviousComparisonGroups(normalizedLegacy);
  assert(prevGroups.stages.length === 1, "normalized empty structured arrays fall back to legacy previous_period");
  const sevenGroups = buildSevenDayComparisonGroups(normalizedLegacy);
  assert(sevenGroups.stages.length === 1, "seven day groups use seven_day_average legacy");
  const normalizedHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={normalizedLegacy} isExpanded={false} onToggle={() => undefined} />,
  );
  assert(normalizedHtml.includes("Сводка есть."), "summary rendered after normalize");
  assert(!normalizedHtml.includes("Нет достаточно данных для сравнения с прошлой сводкой."), "no previous empty when summary+legacy");
  assert(!normalizedHtml.includes("Пока недостаточно истории для сравнения со средним уровнем."), "no seven-day empty when summary+legacy");

  // Test B: legacy-only shape
  const legacyOnly: FunnelAIAnalysis = {
    status: "available",
    comparisons: {
      previous_period: [{ key: "product_views", label: "Просмотры", delta: "-5%", tone: "negative" }],
      seven_day_average: [{ key: "product_views", label: "Просмотры", delta: "-2%", tone: "warning" }],
    },
  };
  assert(hasPreviousComparisonSectionContent(legacyOnly), "legacy previous metrics count as content");
  assert(hasSevenDayComparisonSectionContent(legacyOnly), "legacy seven day metrics count as content");

  // Test C: summary only
  const summaryOnly: FunnelAIAnalysis = {
    status: "available",
    comparisons: { stage_metrics: [], conversion_metrics: [], business_metrics: [], previous_period: [], seven_day_average: [] },
    comparisons_commentary: {
      previous_summary: "Только текст прошлой сводки.",
      seven_day_average: "Только текст 7 дней.",
    },
  };
  assert(hasPreviousComparisonSectionContent(summaryOnly), "previous summary counts as content");
  assert(hasSevenDayComparisonSectionContent(summaryOnly), "seven day summary counts as content");
  const summaryHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={summaryOnly} isExpanded={false} onToggle={() => undefined} />,
  );
  assert(summaryHtml.includes("Только текст прошлой сводки."), "previous summary shown");
  assert(!summaryHtml.includes("Нет достаточно данных для сравнения с прошлой сводкой."), "no dashed empty for previous");
  assert(summaryHtml.includes("Детальные показатели сравнения пока недоступны."), "muted note when summary without metrics");

  // Test D: truly empty
  const emptyComparisons: FunnelAIAnalysis = { status: "available", comparisons: { stage_metrics: [], conversion_metrics: [], business_metrics: [] } };
  assert(!hasPreviousComparisonSectionContent(emptyComparisons), "truly empty previous");
  assert(!hasSevenDayComparisonSectionContent(emptyComparisons), "truly empty seven day");
  const emptyHtml = renderToStaticMarkup(<FunnelAIAnalysisContent analysis={emptyComparisons} isExpanded={false} onToggle={() => undefined} />);
  assert(emptyHtml.includes("Нет достаточно данных для сравнения с прошлой сводкой."), "previous empty state");
  assert(emptyHtml.includes("Пока недостаточно истории для сравнения со средним уровнем."), "seven day empty state");
  assertRemovedSectionsAbsent(emptyHtml);

  const noDirections: FunnelAIAnalysis = { ...structuredAnalysis, check_directions: [] };
  const noDirHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={noDirections} isExpanded={false} onToggle={() => undefined} />,
  );
  assertRemovedSectionsAbsent(noDirHtml);

  // Test 1: same skeleton group order for previous and 7-day
  const skeletonFixture: FunnelAIAnalysis = {
    status: "available",
    comparisons: {
      previous_period: [
        { key: "impressions", delta: "-4,2%", tone: "warning" },
        { key: "product_views", delta: "-13,6%", tone: "negative" },
        { key: "add_to_cart", delta: "-27,9%", tone: "negative" },
        { key: "orders", delta: "-9,1%", tone: "warning" },
        { key: "average_order_value", delta: "-17,9%", tone: "negative" },
      ],
      seven_day_average: [
        { key: "impressions", delta: "+0,6%", tone: "neutral" },
        { key: "product_views", delta: "-7,4%", tone: "warning" },
        { key: "add_to_cart", delta: "-12,2% к среднему за 7 дней", tone: "negative" },
        { key: "orders", delta: "+7,5%", tone: "positive" },
        { key: "delivered_units", delta: "+7,3%", tone: "positive" },
        { key: "average_order_value", delta: "-16,56% к среднему за 7 дней", tone: "negative" },
      ],
    },
  };
  const prevSkeleton = buildPreviousComparisonGroups(skeletonFixture);
  const sevenSkeleton = buildSevenDayComparisonGroups(skeletonFixture);
  assert(prevSkeleton.stages.map((s) => s.key).join(",") === "impressions,product_views,add_to_cart,orders", "previous stages order");
  assert(sevenSkeleton.stages.map((s) => s.key).join(",") === "impressions,product_views,add_to_cart,orders,delivered_units", "seven-day stages order");
  assert(prevSkeleton.business[0]?.key === "average_order_value", "previous AOV in business");
  assert(sevenSkeleton.business[0]?.key === "average_order_value", "seven-day AOV in business");

  // Test 2: add_to_cart in 7-day is stage, not conversion
  const sevenCart = buildSevenDayComparisonGroups({
    status: "available",
    comparisons: { seven_day_average: [{ key: "add_to_cart", delta: "-12,2% к среднему за 7 дней", tone: "negative" }] },
  });
  assert(sevenCart.stages.some((s) => s.key === "add_to_cart"), "add_to_cart in stages");
  assert(!sevenCart.conversions.some((s) => s.key === "add_to_cart"), "add_to_cart not in conversions");

  // Test 3: average_order_value in economics for 7-day
  const sevenAov = buildSevenDayComparisonGroups({
    status: "available",
    comparisons: {
      seven_day_average: [{ key: "average_order_value", delta: "-16,56% к среднему за 7 дней", tone: "negative" }],
    },
  });
  assert(sevenAov.business[0]?.label === "Средний чек", "AOV label");

  // Test 4: conversion key in 7-day conversions group
  const sevenConv = buildSevenDayComparisonGroups({
    status: "available",
    comparisons: {
      seven_day_average: [{ key: "add_to_cart_to_orders", delta: "+8,90 п.п. к среднему за 7 дней", tone: "positive" }],
    },
  });
  assert(
    sevenConv.conversions.some((c) => c.key === "add_to_cart_to_orders" && c.label === "Корзина → Заказанные единицы"),
    "cart conversion in conversions",
  );

  // Test 5–6: empty-state (covered above)

  // Test 7: labels normalized
  assert(classifyComparisonBucket("add_to_cart") === "stage", "add_to_cart bucket");
  assert(classifyComparisonBucket("cart_to_order") === "conversion", "cart_to_order bucket");
  assert(canonicalComparisonKey("cart_to_order") === "add_to_cart_to_orders", "cart_to_order alias");

  // Test 8: delta short format
  const shortDelta = buildSevenDayComparisonGroups({
    status: "available",
    comparisons: {
      seven_day_average: [{ key: "product_views", delta: "-7,4% к среднему за 7 дней", tone: "warning" }],
    },
  });
  assert(sevenCart.stages[0]?.delta === "-12,2%", "seven-day strips base from add_to_cart delta");
  assert(shortDelta.stages[0]?.delta === "-7,4%", "short delta without base");

  const prevShort = buildPreviousComparisonGroups({
    status: "available",
    comparisons: {
      conversion_metrics: [{ key: "product_views_to_add_to_cart", delta: "-4,81 п.п. к прошлому дню", tone: "negative" }],
    },
  });
  assert(prevShort.conversions[0]?.delta === "-4,81 п.п.", "previous conversion short delta");

  const skeletonHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={skeletonFixture} isExpanded={false} onToggle={() => undefined} />,
  );
  assert(skeletonHtml.includes("изменение количества"), "group subtitle");
  assert(skeletonHtml.includes("Добавления в корзину"), "add_to_cart label");
  assert(!skeletonHtml.includes("Корзина → заказ"), "no short legacy conversion label");
  assert(skeletonHtml.includes("Средний чек"), "AOV in seven-day card");

  // Commentary guardrails -------------------------------------------------

  // 1: good AI commentary is kept as the primary source.
  const goodCommentary: FunnelAIAnalysis = {
    status: "available",
    comparisons: { previous_period: [{ key: "add_to_cart", delta: "-27,9%", tone: "negative" }] },
    comparisons_commentary: {
      previous_period: "Основная просадка в середине воронки: после просмотра карточек реже добавляют товары в корзину.",
      seven_day_average: "Заказы держатся за счёт нижнего перехода, проблема локальна в середине воронки.",
    },
  };
  const goodModel = buildFunnelAnalysisDisplayModel(goodCommentary);
  assert(
    goodModel.comparisonsCommentary.previousPeriod ===
      "Основная просадка в середине воронки: после просмотра карточек реже добавляют товары в корзину.",
    "good AI previous commentary kept",
  );
  assert(
    goodModel.comparisonsCommentary.sevenDayAverage.includes("проблема локальна в середине воронки"),
    "good AI seven-day commentary kept",
  );

  // 2: limited history rephrases "за 7 дней" wording and keeps the note separate.
  const limitedModel = buildFunnelAnalysisDisplayModel(structuredAnalysis);
  const sevenText = limitedModel.comparisonsCommentary.sevenDayAverage.toLowerCase();
  assert(!sevenText.includes("за 7 дней"), "limited history strips 'за 7 дней' wording");
  assert(!sevenText.includes("за последние 7 дней"), "limited history strips 'за последние 7 дней'");
  assert(sevenText.includes("доступн"), "limited history uses period-safe wording");
  assert(
    !limitedModel.comparisonsCommentary.sevenDayAverage.includes("Сравнение ограничено"),
    "limited-history note not duplicated inside commentary",
  );
  assert(
    limitedModel.contextSignals.some(
      (s) => s.key === "history_status" && (s.value?.includes("доступно меньше 7 дней истории") ?? false),
    ),
    "limited-history note kept as separate context signal",
  );

  // 3: rendered 7-day card shows the note once (as InfoNote), not inside the summary text.
  const limitedHtml = renderToStaticMarkup(
    <FunnelAIAnalysisContent analysis={structuredAnalysis} isExpanded={false} onToggle={() => undefined} />,
  );
  const noteOccurrences = limitedHtml.split("Сравнение ограничено: доступно меньше 7 дней истории").length - 1;
  assert(noteOccurrences === 1, `limited-history note rendered exactly once, got ${noteOccurrences}`);

  // 4: legacy labels / raw keys are stripped from AI commentary.
  const legacyCommentary: FunnelAIAnalysis = {
    status: "available",
    comparisons: { previous_period: [{ key: "orders", delta: "-9%", tone: "negative" }] },
    comparisons_commentary: {
      previous_period: "Оформленные заказы держатся, но выкуп просел сильнее остальных этапов.",
      seven_day_average: "",
    },
  };
  const legacyModel = buildFunnelAnalysisDisplayModel(legacyCommentary);
  const legacyText = legacyModel.comparisonsCommentary.previousPeriod.toLowerCase();
  assert(!legacyText.includes("оформленные заказы"), "legacy 'оформленные заказы' replaced in commentary");
  assert(!legacyText.includes("выкуп"), "legacy 'выкуп' replaced in commentary");

  // 5: empty AI commentary maps to empty (deterministic fallback is backend-side).
  const emptyCommentary: FunnelAIAnalysis = {
    status: "available",
    comparisons: { previous_period: [] },
    comparisons_commentary: { previous_period: "", seven_day_average: "" },
  };
  const emptyModel = buildFunnelAnalysisDisplayModel(emptyCommentary);
  assert(emptyModel.comparisonsCommentary.previousPeriod === "", "empty previous commentary stays empty");
  assert(emptyModel.comparisonsCommentary.sevenDayAverage === "", "empty seven-day commentary stays empty");
}

run();
