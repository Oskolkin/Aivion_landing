import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { AdEfficiencyModule } from "@/components/dashboard/ad-efficiency-module";
import {
  AD_EFFICIENCY_PENDING_CHECKLIST,
  AD_EFFICIENCY_PENDING_SUMMARY,
  AD_EFFICIENCY_ACTIONS_HREF,
  parseAdEfficiencyAIConclusion,
} from "@/lib/ad-efficiency-display";
import type { AdEfficiencyWidget } from "@/lib/ad-efficiency-display";

const FORBIDDEN = [
  "Кампании: расход vs ДРР",
  "sku_risks",
  "spendPerSKU",
  "артикул",
  "not_enough_data",
  "partial",
  "pending",
  "unavailable",
  "current_drr_pct",
  "high_drr_campaigns",
  "ad_waste sku",
  "ad_waste",
  "[dev]",
  "расход по sku",
  "sku сливает бюджет",
  "/app/advertising",
  "Открыть рекламные кампании",
  "Открыть кампании",
];

function makeWidget(overrides: Partial<AdEfficiencyWidget> = {}): AdEfficiencyWidget {
  return {
    status: "available",
    period: {
      metric_date: "2026-05-14",
      date_from: "2026-05-08",
      date_to: "2026-05-14",
      sample_days: 7,
    },
    summary: {
      current_drr_pct: 38,
      previous_drr_pct: 31.6,
      average_drr_7d_pct: 30,
      delta_vs_previous_pp: 6.4,
      delta_vs_average_7d_pp: 8,
      total_spend_today: 1200,
      total_revenue_today: 3000,
      total_orders_today: 12,
      campaigns_high_drr_count: 5,
      campaigns_no_orders_count: 4,
      high_drr_spend: 147302,
      spend_without_orders: 26417,
      high_drr_threshold_pct: 100,
    },
    trend_7d: [
      { date: "2026-05-08", spend: 100, revenue: 400, orders: 2, drr_pct: 25, data_status: "available" },
      { date: "2026-05-09", spend: 120, revenue: 350, orders: 2, drr_pct: 34.3, data_status: "available" },
      { date: "2026-05-10", spend: 90, revenue: 300, orders: 1, drr_pct: 30, data_status: "available" },
      { date: "2026-05-11", spend: 110, revenue: 280, orders: 2, drr_pct: 39.3, data_status: "available" },
      { date: "2026-05-12", spend: 130, revenue: 320, orders: 2, drr_pct: 40.6, data_status: "available" },
      { date: "2026-05-13", spend: 100, revenue: 260, orders: 1, drr_pct: 38.5, is_yesterday: true, data_status: "available" },
      { date: "2026-05-14", spend: 150, revenue: 395, orders: 2, drr_pct: 38, is_today: true, data_status: "available" },
    ],
    high_drr_campaigns: [
      { campaign_external_id: 1, campaign_name: "Поиск — бренд", drr_pct: 162, spend: 90000, orders: 3, revenue: 55555, reason_code: "high_drr" },
      {
        campaign_external_id: 3,
        campaign_name: "Автокампания + категория A с длинным названием для проверки переноса",
        drr_pct: 190,
        spend: 120000,
        orders: 2,
        revenue: 63157,
        reason_code: "high_drr",
      },
      { campaign_external_id: 4, campaign_name: "Холодный трафик", drr_pct: 121, spend: 65302, orders: 1, revenue: 53968, reason_code: "high_drr" },
      { campaign_external_id: 5, campaign_name: "Тестовая кампания", drr_pct: 145, spend: 45200, orders: 3, revenue: 31172, reason_code: "high_drr" },
    ],
    no_order_campaigns: [
      { campaign_external_id: 2, campaign_name: "Трафареты общие", drr_pct: null, spend: 26417, orders: 0, revenue: 0, reason_code: "no_orders" },
      { campaign_external_id: 6, campaign_name: "РСЯ + интересы", drr_pct: null, spend: 14000, orders: 0, revenue: 0, reason_code: "no_orders" },
      { campaign_external_id: 7, campaign_name: "Динамический ретаргетинг", drr_pct: null, spend: 12000, orders: 0, revenue: 0, reason_code: "no_orders" },
      { campaign_external_id: 8, campaign_name: "Сезонное промо", drr_pct: null, spend: 9000, orders: 0, revenue: 0, reason_code: "no_orders" },
    ],
    ad_efficiency_narrative: {
      status: "ready",
      headline: "Общий ДРР кабинета в норме, но часть кампаний требует внимания",
      summary:
        "ДРР кабинета за 8 июля составляет 19,4%, а средний показатель за последние 7 дней — 16,8%. При этом две кампании работают с повышенным ДРР, а одна кампания потратила 3 300 ₽ без получения заказов.",
      recommendation:
        "Проверить ставки, площадки и рекламируемые SKU проблемных кампаний и перераспределить бюджет в пользу эффективных.",
      checklist: [{ title: "Проверить ставки", text: "Сравните ставки в кампаниях с высоким ДРР.", target: "high_drr_campaigns" }],
    },
    cta: { label: "Открыть рекламные кампании", href: "/app/advertising" },
    ...overrides,
  };
}

function assertNoForbidden(html: string) {
  const lower = html.toLowerCase();
  for (const token of FORBIDDEN) {
    assert.ok(!lower.includes(token.toLowerCase()), `forbidden token: ${token}`);
  }
}

function run() {
  // Separate headline + summary — never split on decimal point.
  const parsed = parseAdEfficiencyAIConclusion(
    "ДРР кабинета за 8 июля составляет 19,4%, а средний показатель за последние 7 дней — 16,8%. При этом две кампании требуют внимания.",
    "ai",
    "Общий ДРР кабинета в норме, но часть кампаний требует внимания",
  );
  assert.equal(parsed.title, "Общий ДРР кабинета в норме, но часть кампаний требует внимания");
  assert.match(parsed.body, /19,4%/);
  assert.match(parsed.body, /16,8%/);
  assert.ok(!("recommendation" in parsed));
  assert.ok(!parsed.title?.includes("19."));
  assert.ok(!parsed.body.startsWith("4%)"));

  // Decimal in a single legacy summary must NOT invent a title by splitting on '.'.
  const legacyDecimal = parseAdEfficiencyAIConclusion(
    "ДРР кабинета составляет 19.4%, но две кампании требуют внимания.",
    "ai",
  );
  assert.equal(legacyDecimal.title, null);
  assert.match(legacyDecimal.body, /19\.4%/);
  assert.ok(!legacyDecimal.body.startsWith("4%)"));
  assert.ok(!legacyDecimal.body.includes("ДРР 19."));

  const html = renderToStaticMarkup(<AdEfficiencyModule widget={makeWidget()} />);
  assert.ok(html.includes("Реклама: эффективность кампаний"));
  assert.ok(
    html.includes(
      "В «Пульсе продавца» рекламные расходы сравниваются со всей выручкой кабинета. Здесь — только с выручкой, полученной с рекламы.",
    ),
  );
  assert.ok(html.includes('data-testid="ad-efficiency-denominator-note"'));
  assert.ok(html.includes('data-testid="ad-efficiency-ai-headline"'));
  assert.ok(html.includes('data-testid="ad-efficiency-ai-explanation"'));
  assert.ok(!html.includes('data-testid="ad-efficiency-ai-recommendation"'));
  assert.ok(!html.includes("Рекомендация:"));
  assert.ok(html.includes("AI-вывод"));
  assert.ok(html.includes("Общий ДРР кабинета в норме, но часть кампаний требует внимания"));
  assert.ok(html.includes("19,4%"));
  assert.ok(html.includes("16,8%"));
  assert.ok(!html.includes("ДРР 19."));
  assert.ok(!html.includes("4%), но"));
  assert.ok(!html.includes("ad-efficiency-cta"));
  assert.ok(!html.includes('data-testid="ad-efficiency-kpi-high-drr"'));
  assert.ok(!html.includes('data-testid="ad-efficiency-kpi-no-orders"'));
  assert.ok(!html.includes('data-testid="ad-efficiency-kpi-spend-without-orders"'));
  assert.ok(html.includes("ДРР в выручке с рекламных кампаний"));
  assert.ok(!html.includes("ДРР сегодня"));
  assert.ok(!html.includes("Общий ДРР сегодня"));
  assert.ok(!html.includes("Средний ДРР"));
  assert.ok(html.includes("38%"));
  assert.ok(html.includes("п.п."));
  assert.ok(html.includes("Динамика ДРР в выручке с рекламных кампаний за 7 дней"));
  assert.ok(html.includes("За 7 дней"));
  assert.ok(html.includes("За 7 дней: 30%"));
  assert.ok(!html.includes("Среднее:"));
  assert.ok(html.includes('data-testid="ad-efficiency-average-drr-card"'));
  assert.ok(html.includes('data-testid="ad-efficiency-overview-row"'));
  assert.ok(html.includes("items-start"));
  assert.ok(html.includes('data-component="advertising-metric-card"'));
  assert.ok(html.includes('data-testid="ad-efficiency-drr-card-supporting"'));
  assert.ok(html.includes('data-testid="ad-efficiency-average-drr-card-supporting"'));
  assert.ok(html.includes("min-h-[184px]"));
  assert.ok(html.includes('data-testid="ad-efficiency-ai-summary"'));
  assert.ok(html.includes('data-testid="problem-campaigns-panel"'));
  assert.ok(html.includes('data-testid="ad-efficiency-tab-high_drr"'));
  assert.ok(html.includes("Высокий ДРР · 5"));
  assert.ok(html.includes('data-testid="ad-efficiency-average-line"'));
  assert.ok(html.includes('data-testid="ad-efficiency-today-marker"'));
  assert.ok(!html.includes('data-testid="ad-efficiency-delta-label"'));
  assert.ok(html.includes("Проблемные кампании"));
  assert.ok(html.includes("Что проверить в первую очередь"));
  assert.ok(html.includes(`href="${AD_EFFICIENCY_ACTIONS_HREF}"`));
  assert.ok(html.includes("Открыть действия"));
  assert.ok(html.includes("Показаны кампании и расходы за последние 7 дней"));
  assert.ok(html.includes('data-testid="ad-efficiency-toggle-high_drr"'));
  assert.ok(!html.includes('data-testid="ad-efficiency-toggle-no_orders"'));
  assert.ok(!html.includes('data-testid="ad-efficiency-campaign-table-no_orders"'));
  assert.ok(!html.includes("Кампании без заказов</h3>"));
  assert.ok(!html.includes("Кампании с высоким ДРР"));
  assert.ok(!html.includes("line-clamp-2"));
  assert.ok(!html.includes("доход от рекламы"));
  assertNoForbidden(html);

  // Legacy summary-only: no invented title, full body preserved including decimals.
  const legacyHtml = renderToStaticMarkup(
    <AdEfficiencyModule
      widget={makeWidget({
        ad_efficiency_narrative: {
          status: "ready",
          summary: "ДРР кабинета составляет 19.4%, но две кампании требуют внимания.",
          checklist: [],
        },
      })}
    />,
  );
  assert.ok(legacyHtml.includes("19.4%"));
  assert.ok(!legacyHtml.includes('data-testid="ad-efficiency-ai-headline"'));
  assert.ok(!legacyHtml.includes("ДРР 19."));
  assert.ok(!legacyHtml.includes("4%), но"));
  assert.ok(!legacyHtml.includes('data-testid="ad-efficiency-ai-recommendation"'));

  const pendingHtml = renderToStaticMarkup(
    <AdEfficiencyModule widget={makeWidget({ ad_efficiency_narrative: { status: "pending", summary: "", checklist: [] } })} />,
  );
  assert.ok(pendingHtml.includes(AD_EFFICIENCY_PENDING_SUMMARY));

  const emptyChecklistHtml = renderToStaticMarkup(
    <AdEfficiencyModule widget={makeWidget({ ad_efficiency_narrative: { status: "pending", checklist: [] } })} />,
  );
  assert.ok(emptyChecklistHtml.includes(AD_EFFICIENCY_PENDING_CHECKLIST));

  const partialHtml = renderToStaticMarkup(
    <AdEfficiencyModule widget={makeWidget({ status: "partial", message: "Часть рекламных метрик за выбранный период недоступна." })} />,
  );
  assert.ok(partialHtml.includes("Часть рекламных метрик"));
  assert.ok(partialHtml.includes("ДРР в выручке с рекламных кампаний"));
  assert.ok(partialHtml.includes('data-testid="ad-efficiency-partial-banner"'));

  const notEnoughHtml = renderToStaticMarkup(
    <AdEfficiencyModule widget={{ status: "not_enough_data", message: "Рекламные кампании пока недоступны или не синхронизированы." }} />,
  );
  assert.ok(notEnoughHtml.includes("Рекламные кампании пока недоступны"));
  assert.ok(!notEnoughHtml.includes('data-testid="ad-efficiency-drr-card"'));

  const errorHtml = renderToStaticMarkup(
    <AdEfficiencyModule widget={{ status: "error" }} />,
  );
  assert.ok(errorHtml.includes("Не удалось загрузить данные по рекламе"));
  assert.ok(errorHtml.includes("Повторите попытку позже"));
  assert.ok(!errorHtml.includes('data-testid="ad-efficiency-drr-card"'));

  console.log("ad-efficiency-module.test.tsx: ok");
}

run();
