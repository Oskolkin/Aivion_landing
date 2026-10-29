import { renderToStaticMarkup } from "react-dom/server";
import { SellerPulseSection } from "@/components/dashboard/seller-pulse-section";
import {
  buildPulseViewModel,
  formatNarrativeSentence,
  formatPulseReportDate,
  getPulseMetricDeltaClass,
  getPulseSummaryStripClass,
  getSparklineEmptyLabel,
  mapNarrativeKnowledge,
  mapNarrativeKeyPoints,
  mapNarrativeQuickActions,
  mapNarrativeStatusToSummaryTone,
  mapNarrativeSummary,
  normalizePulseMetric,
  parseSummaryHighlightLabel,
  PULSE_AD_SPEND_SHARE_TOOLTIP,
} from "@/lib/seller-pulse-view";
import type { DashboardHomeResponse, KPICard } from "@/lib/dashboard-home-types";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function makeKpi(overrides: Partial<KPICard> = {}) {
  return {
    key: "revenue",
    label: "Выручка",
    value: "100 000 ₽",
    ...overrides,
  };
}

function makePulse(
  kpis: DashboardHomeResponse["seller_pulse"]["kpis"] = [],
  narrative?: DashboardHomeResponse["seller_pulse"]["narrative"],
): DashboardHomeResponse["seller_pulse"] {
  return { kpis, trend_7d: [], narrative };
}

const readyNarrative: NonNullable<DashboardHomeResponse["seller_pulse"]["narrative"]> = {
  status: "ready",
  tone: "critical",
  headline: "Backend AI headline",
  summary: "Backend AI summary",
  key_points: [
    { title: "Выручка", text: "Backend key point one", tone: "warning" },
    { text: "Backend key point two", tone: "neutral" },
  ],
  important_notes: [
    { title: "Риски", text: "Backend note one", tone: "warning" },
    { text: "Backend note two", tone: "neutral" },
  ],
  quick_actions: [
    {
      label: "Backend action",
      description: "Проверить кампании",
      kind: "action",
      priority: "high",
      href: "/app/actions?action=unified%3Apack%3A1%3Aaction_001",
    },
    { label: "Plain action", kind: "action", href: "/app/actions" },
  ],
  chips: [
    { label: "ДРР: критично", tone: "critical" },
    { label: "Риски: 4", tone: "warning" },
  ],
};

const readyPulse = buildPulseViewModel({
  data: makePulse(
    [
      makeKpi({ key: "revenue", value: "80 000 ₽", delta: { abs: -20_000, pct: -20, label: "к вчера" } }),
      makeKpi({ key: "orders", label: "Заказы", value: "20", delta: { abs: -8, label: "к вчера" } }),
      makeKpi({ key: "ad_spend_share_total_revenue", label: "ДРР", value: "42.0%" }),
      makeKpi({ key: "risks", label: "Критичные риски", value: "4" }),
    ],
    readyNarrative,
  ),
});

assert(readyPulse.summary.headline === "Backend AI headline", "ready narrative headline is used");
assert(readyPulse.summary.summary === "Backend AI summary", "ready narrative summary is used");
assert(readyPulse.keyPoints[0]?.text === "Backend key point one", "ready narrative key points are used");
assert(readyPulse.knowledge[0]?.text === "Backend note one", "ready narrative important notes are used");

const riskAwareNarrative: NonNullable<DashboardHomeResponse["seller_pulse"]["narrative"]> = {
  ...readyNarrative,
  important_notes: [{ text: "Есть открытые риски высокой важности.", tone: "critical" }],
  quick_actions: [{ label: "Открыть действия", kind: "action", href: "/app/actions" }],
};
const riskAwarePulse = buildPulseViewModel({
  data: makePulse([makeKpi({ key: "revenue", value: "10 000 ₽" })], riskAwareNarrative),
});
assert(
  riskAwarePulse.knowledge.some((item) => item.text.includes("риски")),
  "important notes about risks still render",
);
assert(
  riskAwarePulse.quickActions.some((action) => action.href === "/app/actions"),
  "risk-related quick actions still render",
);
assert(readyPulse.knowledge[0]?.title === "Риски", "important note title preserved");
assert(readyPulse.keyPoints[0]?.title === "Выручка", "key point title preserved");
assert(
  readyPulse.quickActions.some((action) => action.href?.startsWith("/app/actions")),
  "ready narrative quick action href preserved",
);
assert(readyPulse.quickActions[0]?.kind === "action", "quick action kind preserved");
assert(readyPulse.metrics.length === 4, "only four KPI cards are rendered");
assert(!readyPulse.metrics.some((metric) => metric.key === "risks"), "risks KPI card is excluded");
assert(
  readyPulse.summary.signals.some((signal) => signal.label.includes("Риски")),
  "risk chips from narrative still appear in summary",
);
assert(readyPulse.summary.highlights.length === 2, "narrative chips mapped to summary highlights");
assert(
  readyPulse.summary.highlights.some((highlight) => highlight.title === "Главный риск дня"),
  "DRR chip mapped to highlight title",
);
assert(
  readyPulse.summary.highlights.some((highlight) => highlight.value === "4"),
  "risk count mapped to highlight value",
);

const pendingPulse = buildPulseViewModel({
  data: makePulse(
    [makeKpi({ key: "revenue", value: "50 000 ₽" })],
    {
      status: "pending",
      tone: "neutral",
      headline: "Backend pending headline",
      summary: "Backend pending summary",
      important_notes: [{ text: "Синхронизация ещё идёт." }],
      quick_actions: [{ label: "Дождаться завершения синхронизации" }],
      data_quality_note: "AI-сводка блока формируется после синхронизации метрик.",
    },
  ),
});

assert(pendingPulse.summary.headline === "Backend pending headline", "pending backend narrative is shown");
assert(pendingPulse.summary.tone === "info", "pending uses controlled info tone");
assert(pendingPulse.dataQualityNote?.includes("формируется") === true, "pending quality note is shown");

const missingNarrativePulse = buildPulseViewModel({
  dashboardStatus: "not_enough_data",
  data: makePulse([]),
});

assert(
  missingNarrativePulse.summary.headline.includes("AI-вывод"),
  "missing narrative shows non-analytical empty state",
);
assert(
  missingNarrativePulse.narrativeStatus === "source_not_available",
  "missing narrative uses source_not_available status",
);

const revenueMetric = normalizePulseMetric(
  "revenue",
  makeKpi({ delta: { abs: 5000, pct: 8, label: "к вчера" } }),
  true,
);
assert(revenueMetric.state === "positive", "revenue growth is positive");
assert(getPulseMetricDeltaClass(revenueMetric.state).includes("027a48"), "positive delta class is green");
assert(getSparklineEmptyLabel("no_data").includes("синхронизации"), "no_data sparkline label is seller-facing");

const summary = mapNarrativeSummary(readyNarrative);
assert(summary.signals.length === 2, "chips mapped to summary signals");
assert(summary.highlights.length === 2, "chips mapped to summary highlights");
assert(summary.tone === "critical", "ready status keeps narrative tone");
assert(formatPulseReportDate("2026-05-21") === "21.05.2026", "metric date uses russian format");
assert(formatNarrativeSentence("Текст без точки") === "Текст без точки.", "narrative sentence adds trailing period");
assert(formatNarrativeSentence("Уже есть.") === "Уже есть.", "narrative sentence keeps existing period");
assert(formatNarrativeSentence("Вопрос?") === "Вопрос?", "narrative sentence keeps question mark");
assert(formatNarrativeSentence("  много   пробелов  ") === "много пробелов.", "narrative sentence collapses spaces");
assert(formatNarrativeSentence("") === "", "empty narrative sentence stays empty");
assert(
  parseSummaryHighlightLabel("ДРР: критично").title === "Главный риск дня",
  "DRR chip label parsed into highlight title",
);
assert(
  parseSummaryHighlightLabel("Риски: 4").value === "4",
  "risk chip label parsed into highlight value",
);
assert(mapNarrativeStatusToSummaryTone("pending", "critical") === "info", "pending overrides tone visually");
assert(getPulseSummaryStripClass("critical").includes("fef3f2"), "critical summary strip uses red palette");
assert(!summary.headline.includes("null"), "no technical tokens in summary");

const knowledge = mapNarrativeKnowledge(readyNarrative);
assert(knowledge.length === 2, "knowledge mapped from important_notes only");

const keyPoints = mapNarrativeKeyPoints(readyNarrative);
assert(keyPoints.length === 2, "key points mapped from key_points");
assert(keyPoints[0]?.text === "Backend key point one", "key point text preserved");

const actions = mapNarrativeQuickActions(readyNarrative);
assert(actions.length === 2, "quick actions mapped from backend");

const html = renderToStaticMarkup(
  <SellerPulseSection
    data={makePulse(
      [
        makeKpi({ key: "revenue", value: "80 000 ₽", delta: { abs: -1000, pct: -2, label: "к вчера" } }),
        makeKpi({ key: "orders", label: "Заказы", value: "20" }),
        makeKpi({ key: "average_order_value", label: "Средний чек", value: "592 ₽" }),
        makeKpi({ key: "ad_spend_share_total_revenue", label: "ДРР", value: "12%" }),
        makeKpi({ key: "risks", label: "Риски", value: "0" }),
      ],
      readyNarrative,
    )}
    dashboardStatus="ready"
    metricDate="2026-05-21"
  />,
);

assert(html.includes("Пульс продавца"), "section title rendered");
assert(html.includes("21.05.2026"), "metric date rendered in russian format");
assert(html.includes("Анализ по данным на"), "metric date subtitle rendered");
assert(!html.includes('data-testid="pulse-summary-highlight"'), "summary strip does not render highlight cards");
assert(!html.includes("Главный риск дня"), "chip highlights are not shown in summary strip");
assert(!html.includes("Открытых рисков"), "chip highlights are not shown in summary strip");
assert(!html.includes("Риски: 4"), "raw chip text is not shown in summary strip");
assert(html.includes("Средний чек"), "AOV card rendered");
assert(html.includes("Реклама в общей выручке"), "ad spend share of store revenue label");
assert(!html.includes(">ДРР<"), "bare DRR label not rendered");
assert(!html.includes(">CTR<"), "English CTR label not rendered");
assert(html.includes("Средний чек"), "AOV card is rendered in seller pulse");
assert(html.includes("Backend AI summary."), "backend summary rendered with normalized punctuation");
assert(html.includes("Backend AI headline"), "pulse AI thesis headline is shown in AI-резюме card");
assert(html.includes("AI-резюме по кабинету"), "static AI resume label rendered");
assert(html.includes('data-testid="seller-pulse-ai-key-points"'), "key points rendered inside AI resume");
assert(html.includes("Что важно знать"), "knowledge card always shown alongside quick actions");
assert(html.includes("Backend note one"), "important note text rendered in knowledge card");
assert(html.includes("Backend key point one"), "key point text rendered in AI resume");
assert(html.includes("Быстрые действия"), "quick actions card rendered");
assert(html.includes('href="/app/actions'), "href actions render as action detail links");
assert(!html.includes('href="/app/advertising"'), "legacy advertising route not used");
assert(!html.includes('href=""'), "no broken href attributes");
assert(!html.includes("без графика"), "no technical graph fallback");
assert(!html.includes(">н/д<"), "no n/d fallback");
assert(!html.includes(">critical<"), "no raw critical label");
assert(!html.includes(">high<"), "no raw high label");
assert(html.includes("Нет истории за период"), "seller-facing sparkline empty state");
assert(html.includes("rounded-[22px]"), "hero card radius");
assert(html.includes("xl:grid-cols-4"), "desktop four-column grid");
assert(!html.includes("Критичные риски"), "risks KPI card label is not rendered");
assert(html.includes("Выручка"), "key point title rendered in AI resume");
assert(
  html.includes("lucide-circle-question-mark") || html.includes("circle-question"),
  "drr metric renders info hint icon",
);
assert(
  html.includes("общую выручку кабинета") || html.includes("общей выручки кабинета"),
  "drr info tooltip explains store revenue denominator",
);

const pulseSellerHeadlineHtml = renderToStaticMarkup(
  <SellerPulseSection
    data={makePulse(
      [makeKpi({ key: "revenue", value: "80 000 ₽" })],
      {
        status: "ready",
        tone: "info",
        headline: "Пульс продавца.",
        summary: "День выглядит напряжённо из-за просадки выручки.",
      },
    )}
  />,
);
assert(
  (pulseSellerHeadlineHtml.match(/Пульс продавца/g) || []).length === 1,
  "generic pulse headline hidden in blue strip (only section title remains)",
);
assert(pulseSellerHeadlineHtml.includes("День выглядит напряжённо"), "pulse summary still rendered");

const sparklineWithoutDelta = normalizePulseMetric(
  "average_order_value",
  makeKpi({
    key: "average_order_value",
    label: "CTR",
    value: "13,2%",
    sparkline_7d: [
      { date: "2026-06-24", value: 1300, has_data: true },
      { date: "2026-06-25", value: 1350, has_data: true },
      { date: "2026-06-26", value: 1400, has_data: true },
    ],
  }),
  true,
);
assert(sparklineWithoutDelta.hasSparkline, "sparkline detected");
assert(sparklineWithoutDelta.deltaText === "Динамика за 7 дней", "sparkline without delta shows 7-day label");
assert(sparklineWithoutDelta.statusLabel !== "Динамика недоступна", "status label not unavailable when sparkline exists");

const drrMetric = normalizePulseMetric(
  "ad_spend_share_total_revenue",
  makeKpi({
    key: "ad_spend_share_total_revenue",
    label: "ДРР",
    value: "12%",
    hint: PULSE_AD_SPEND_SHARE_TOOLTIP,
  }),
  true,
);
assert(
  drrMetric.label === "Реклама в общей выручке (7 дней)",
  "frontend metric label overrides legacy API label and states weekly period",
);
assert(drrMetric.hint === PULSE_AD_SPEND_SHARE_TOOLTIP, "drr hint preserved for tooltip");
assert(
  PULSE_AD_SPEND_SHARE_TOOLTIP.toLowerCase().includes("недел") ||
    PULSE_AD_SPEND_SHARE_TOOLTIP.includes("7 дней"),
  "ad spend share hint explicitly states weekly/7-day period",
);

const drrMetricWithCalc = normalizePulseMetric(
  "ad_spend_share_total_revenue",
  makeKpi({ key: "ad_spend_share_total_revenue", value: "19,4%" }),
  true,
  { spend: 23400, revenue: 120791 },
);
assert(Boolean(drrMetricWithCalc.hint?.includes("Расчёт за 7 дней:")), "dynamic pulse calc appended when totals available, labeled as 7-day calc");
assert(
  Boolean(drrMetricWithCalc.hint?.includes("23 400 ₽") || drrMetricWithCalc.hint?.includes("23 400 ₽")),
  "spend in calc",
);
const sparklineSectionHtml = renderToStaticMarkup(
  <SellerPulseSection
    data={makePulse([
      makeKpi({
        key: "average_order_value",
        label: "CTR",
        value: "13,2%",
        sparkline_7d: [
          { date: "2026-06-24", value: 1300, has_data: true },
          { date: "2026-06-25", value: 1350, has_data: true },
        ],
      }),
      makeKpi({
        key: "ad_spend_share_total_revenue",
        label: "ДРР",
        value: "12.0%",
        sparkline_7d: [
          { date: "2026-06-24", value: 11, has_data: true },
          { date: "2026-06-25", value: 12, has_data: true },
        ],
      }),
    ])}
  />,
);
assert(!sparklineSectionHtml.includes("Динамика недоступна"), "sparkline cards do not show unavailable dynamics label");

console.log("seller-pulse-view.test.tsx: ok");
