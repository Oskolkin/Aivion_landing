import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { StockCoverageModule } from "@/components/dashboard/stock-coverage-module";
import type { DashboardHomeResponse, StockCoverageNarrative } from "@/lib/dashboard-home-types";

const TECHNICAL_WORDS = ["narrative", "fallback", "generation", "daily summary", "raw payload", "артикул"];

function makeNarrative(overrides: Partial<StockCoverageNarrative> = {}): StockCoverageNarrative {
  return {
    status: "available",
    headline: "Есть риск дефицита у продающих SKU",
    summary: "Основной риск сосредоточен в коротком покрытии по SKU с подтверждённым спросом за последнюю неделю.",
    no_sales_insight: "1 SKU с остатком 51 шт. не имеет продаж последние 7 дней.",
    revenue_at_risk: {
      status: "available",
      label: "Выручка под риском",
      value: "35 914 ₽/день",
      method_note: "По SKU с продажами за последние 7 дней и коротким покрытием.",
    },
    segments: [
      { key: "critical_0_3", label: "0–3 дня", count: 6, severity: "critical" },
      { key: "shortage_4_7", label: "4–7 дней", count: 5, severity: "high" },
      { key: "planning_8_14", label: "8–14 дней", count: 0, severity: "medium" },
      { key: "healthy_14plus", label: ">14 дней", count: 69, severity: "low" },
      { key: "surplus", label: "Излишки / хранение", count: 0, severity: "high" },
    ],
    actions: [
      {
        title: "Проверить критичные SKU",
        text: "Проверьте товары с покрытием 0–3 дня и активными продажами.",
        priority: "high",
        target: { type: "sku_focus_tab", tab: "stock" },
      },
    ],
    no_sales_with_stock: {
      status: "available",
      count: 1,
      stock_units: 51,
    },
    advertising_link: { status: "unavailable", comment: "" },
    sku_focus_link: { label: "Смотреть товары в SKU-фокусе", target: "sku-focus-stock" },
    no_sales_focus_link: { label: "Проверить SKU без продаж", target: "sku-focus-no-sales" },
    ...overrides,
  };
}

function makeStockCoverage(narrative?: Partial<StockCoverageNarrative>): DashboardHomeResponse["ai_diagnostics"]["stock_coverage"] {
  return {
    status: "available",
    segments: [
      { key: "critical_0_3", label: "0–3 дня", sku_count: 6, severity: "critical" },
      { key: "shortage_4_7", label: "4–7 дней", sku_count: 5, severity: "high" },
      { key: "planning_8_14", label: "8–14 дней", sku_count: 0, severity: "medium" },
      { key: "healthy_14plus", label: ">14 дней", sku_count: 69, severity: "low" },
      { key: "surplus", label: "Излишки / хранение", sku_count: 0, severity: "high" },
    ],
    total_skus: 80,
    stock_coverage_narrative: makeNarrative(narrative),
  };
}

function assertNoForbiddenTerms(html: string) {
  const lower = html.toLowerCase();
  for (const word of TECHNICAL_WORDS) {
    assert.ok(!lower.includes(word), `forbidden term found: ${word}`);
  }
}

function assertBarSegmentCount(html: string, key: string, count: number) {
  const re = new RegExp(`data-testid="stock-coverage-bar-segment-${key}"[^>]*data-segment-count="${count}"`);
  assert.ok(re.test(html), `bar segment ${key} should show count ${count}`);
}

function run() {
  const html = renderToStaticMarkup(<StockCoverageModule stockCoverage={makeStockCoverage()} />);

  assert.ok(html.includes('data-testid="stock-coverage-module"'));
  assert.ok(html.includes('data-testid="stock-coverage-diagnosis-panel"'));
  assert.ok(html.includes('data-testid="stock-coverage-diagnosis-title"'));
  assert.ok(html.includes("Есть риск дефицита у продающих SKU"), "diagnosis title");
  assert.ok(html.includes('data-testid="stock-coverage-diagnosis-description"'));

  assert.ok(html.includes("Открыть 11 SKU с риском"), "primary CTA with dynamic count");
  assert.ok(html.includes('data-testid="stock-coverage-primary-cta"'));
  assert.ok(html.includes('data-testid="stock-coverage-primary-cta-icon"'));
  assert.ok(html.includes('data-testid="stock-coverage-secondary-cta"'));
  assert.ok(html.includes('data-testid="stock-coverage-secondary-cta-icon"'));
  assert.ok(html.includes("Проверить 1 SKU"), "no-sales CTA inside card");
  assert.ok(html.includes('href="#sku-focus-no-sales"'), "secondary CTA hash target");
  assert.ok(html.includes('data-sku-focus-target="sku-focus-no-sales"'), "secondary CTA navigate target");
  assert.ok(html.includes('data-sku-focus-target="sku-focus-stock"'), "primary CTA navigate target");

  assert.ok(html.includes('data-testid="stock-coverage-summary-icon"'));
  assert.ok(html.includes('data-testid="stock-coverage-no-sales-card"'));
  assert.ok(html.includes("1 SKU без продаж"), "no-sales title");
  assert.ok(html.includes("Отдельная проблема оборачиваемости"));

  assert.ok(html.includes('data-testid="stock-coverage-kpi-revenue"'));
  assert.ok(html.includes("35 914 ₽/день"), "revenue at risk");
  assert.ok(html.includes('data-testid="stock-coverage-kpi-critical"'));
  assert.ok(html.includes('data-testid="stock-coverage-kpi-shortage"'));
  assert.ok(html.includes('data-testid="stock-coverage-kpi-control"'), "control zone KPI");
  assert.ok(html.includes('data-testid="stock-coverage-kpi-stable"'));
  assert.ok(html.includes("6 SKU"), "critical count");
  assert.ok(html.includes("5 SKU"), "short count");
  assert.ok(html.includes("0 SKU"), "control count");
  assert.ok(html.includes("69 SKU"), "stable count");

  assert.ok(html.includes('data-testid="stock-coverage-distribution-bar"'));
  assert.ok(html.includes('data-testid="stock-coverage-zero-bucket"'));
  assert.ok(html.includes("Распределение покрытия"));
  assert.ok(html.includes("Всего: 80 SKU"));
  assertBarSegmentCount(html, "critical_0_3", 6);
  assertBarSegmentCount(html, "shortage_4_7", 5);
  assertBarSegmentCount(html, "healthy_14plus", 69);
  assert.ok(html.includes("8–14 дней"), "legend includes 8–14 at zero");
  assert.ok(html.includes('aria-label="Из 80 SKU:'));

  assert.ok(!html.includes('data-testid="stock-coverage-actions-card"'), "no actions card");
  assert.ok(!html.includes("Что сделать"));
  assert.ok(!html.includes("Отдельный сигнал"), "old no-sales label removed");
  assert.ok(!html.includes("Открыть SKU без продаж"), "footer duplicate CTA removed");
  assert.ok(!html.includes("Смотреть товары в SKU-фокусе"));
  assertNoForbiddenTerms(html);

  const pending = makeStockCoverage({
    headline: "",
    summary: "",
    no_sales_insight: "",
    actions: [],
    sku_focus_link: { label: "Смотреть товары в SKU-фокусе", target: "sku-focus-stock" },
    no_sales_with_stock: {
      status: "available",
      count: 1,
      stock_units: 51,
      comment: "Есть 1 SKU с остатком 51 без продаж за последние 7 дней — часть запаса не участвует в обороте.",
    },
  });
  const pendingHtml = renderToStaticMarkup(<StockCoverageModule stockCoverage={pending} />);
  assert.ok(pendingHtml.includes("11 SKU требуют внимания"), "factual diagnosis when AI pending");
  assert.ok(pendingHtml.includes('data-testid="stock-coverage-no-sales-factual"'));
  assert.ok(!pendingHtml.includes("часть запаса не участвует"));

  const withRiskComment = makeStockCoverage({
    headline: "",
    summary: "",
    risk_comment: "Короткое покрытие концентрируется в нескольких SKU с активными продажами.",
  });
  const riskHtml = renderToStaticMarkup(<StockCoverageModule stockCoverage={withRiskComment} />);
  assert.ok(riskHtml.includes("Короткое покрытие концентрируется"));
  assert.ok(!riskHtml.includes("Основной риск сосредоточен"));

  const noSecondary = makeStockCoverage({
    no_sales_with_stock: { status: "unavailable" },
    no_sales_insight: "",
    no_sales_focus_link: undefined,
  });
  const noSecondaryHtml = renderToStaticMarkup(<StockCoverageModule stockCoverage={noSecondary} />);
  assert.ok(!noSecondaryHtml.includes('data-testid="stock-coverage-secondary-cta"'));
  assert.ok(!noSecondaryHtml.includes("без продаж"));
  assert.ok(!noSecondaryHtml.includes("lg:grid-cols-[minmax(0,1.7fr"), "main diagnosis expands");

  const noRisk = makeStockCoverage({
    headline: "",
    summary: "",
    segments: [
      { key: "critical_0_3", label: "0–3 дня", count: 0, severity: "critical" },
      { key: "shortage_4_7", label: "4–7 дней", count: 0, severity: "high" },
      { key: "planning_8_14", label: "8–14 дней", count: 4, severity: "medium" },
      { key: "healthy_14plus", label: ">14 дней", count: 76, severity: "low" },
    ],
    revenue_at_risk: { status: "available", value: "0 ₽/день" },
  });
  const noRiskHtml = renderToStaticMarkup(<StockCoverageModule stockCoverage={noRisk} />);
  assert.ok(noRiskHtml.includes("Критического риска дефицита не обнаружено"));
  assert.ok(!noRiskHtml.includes('data-testid="stock-coverage-primary-cta"'), "no misleading risk CTA");
  assert.ok(!noRiskHtml.includes('data-testid="stock-coverage-kpi-revenue"'), "revenue hidden when no risk");

  const partialRevenue = makeStockCoverage({
    revenue_at_risk: { status: "unavailable" },
  });
  const partialHtml = renderToStaticMarkup(<StockCoverageModule stockCoverage={partialRevenue} />);
  assert.ok(partialHtml.includes("Нет данных"), "partial revenue shows no data");

  console.log("stock-coverage-module tests passed");
}

run();
