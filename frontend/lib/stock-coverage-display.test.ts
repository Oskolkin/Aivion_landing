import assert from "node:assert/strict";
import {
  buildCoverageDistributionBuckets,
  buildDiagnosisTitle,
  buildDistributionBarSummary,
  buildFactualDiagnosisDescription,
  buildFactualNoSalesChipText,
  buildNoSalesCTALabel,
  buildRiskCTALabel,
  COVERAGE_DISTRIBUTION_BUCKET_ORDER,
  formatSKUCount,
  getNoSalesInsightText,
  getRiskSKUCount,
  getStockCoverageActions,
  getStockCoverageSummaryText,
  hasStockCoverageRisk,
  isStockCoverageAISummaryAvailable,
  replaceArtikulWithSKU,
  sanitizeSellerFacingStockText,
  sellerFacingRevenueMethodNote,
  stockCoverageLinkLabel,
  noSalesFocusLinkLabel,
} from "./stock-coverage-display";

function run() {
  const segments = [
    { key: "critical_0_3", label: "0–3 дня", sku_count: 6 },
    { key: "shortage_4_7", label: "4–7 дней", sku_count: 5 },
    { key: "planning_8_14", label: "8–14 дней", sku_count: 0 },
    { key: "healthy_14plus", label: ">14 дней", sku_count: 69 },
    { key: "surplus", label: "Излишки", sku_count: 2 },
  ];

  const buckets = buildCoverageDistributionBuckets(segments);
  assert.deepEqual(
    buckets.map((b) => b.key),
    COVERAGE_DISTRIBUTION_BUCKET_ORDER,
    "fixed bucket order",
  );
  assert.equal(buckets.find((b) => b.key === "planning_8_14")?.count, 0, "keeps 8–14 at zero");
  assert.equal(buckets.length, COVERAGE_DISTRIBUTION_BUCKET_ORDER.length, "exclusive stock segments in distribution");
  assert.ok(buckets.some((b) => b.key === "surplus"), "surplus is part of exclusive distribution");
  assert.ok(buckets.some((b) => b.key === "no_sales"), "no_sales is part of exclusive distribution");

  assert.equal(stockCoverageLinkLabel("Смотреть товары в SKU-фокусе"), "Открыть SKU с риском");
  assert.equal(
    stockCoverageLinkLabel("Перейти к артикул с риском по остаткам"),
    "Открыть SKU с риском",
  );
  assert.equal(noSalesFocusLinkLabel("Проверить SKU без продаж"), "Открыть SKU без продаж");
  assert.equal(noSalesFocusLinkLabel(""), "Открыть SKU без продаж");

  const baseNarrative = {
    status: "available" as const,
    revenue_at_risk: { status: "unavailable" },
    no_sales_with_stock: { status: "unavailable" as const },
    sku_focus_link: { label: "", target: "sku-focus-stock" },
  };

  assert.ok(
    getStockCoverageSummaryText({ ...baseNarrative, summary: "" }).includes("AI-вывод по остаткам обновляется"),
  );

  const withSummary = {
    ...baseNarrative,
    summary: "Основной риск сосредоточен в коротком покрытии по SKU с подтверждённым спросом.",
    risk_comment: "Дублирующий risk_comment не должен попадать в UI при наличии summary.",
  };
  assert.ok(getStockCoverageSummaryText(withSummary).includes("Основной риск"));
  assert.ok(!getStockCoverageSummaryText(withSummary).includes("Дублирующий"));

  const legacyRiskOnly = {
    ...baseNarrative,
    risk_comment: "Короткое покрытие концентрируется в нескольких SKU с активными продажами.",
  };
  assert.ok(getStockCoverageSummaryText(legacyRiskOnly).includes("Короткое покрытие"));

  const noSalesAI = getNoSalesInsightText({
    ...baseNarrative,
    no_sales_insight: "1 SKU с остатком 51 шт. не имеет продаж последние 7 дней.",
    no_sales_with_stock: { status: "available", count: 1, stock_units: 51, comment: "часть запаса не участвует в обороте" },
  });
  assert.equal(noSalesAI.kind, "ai");
  assert.ok(!noSalesAI.text.includes("часть запаса не участвует"), "ignores comment field");

  const noSalesFactual = getNoSalesInsightText({
    ...baseNarrative,
    no_sales_with_stock: { status: "available", count: 3, stock_units: 51, comment: "часть запаса не участвует в обороте" },
  });
  assert.equal(noSalesFactual.kind, "factual");
  assert.equal(noSalesFactual.text, "SKU без продаж: 3 · остаток 51 шт.");
  assert.ok(!noSalesFactual.text.includes("часть запаса"));

  assert.equal(buildFactualNoSalesChipText(1, 51), "SKU без продаж: 1 · остаток 51 шт.");
  assert.equal(getStockCoverageActions(baseNarrative).length, 0);

  assert.ok(!sellerFacingRevenueMethodNote("x weighted_avg_daily_revenue_7d").includes("weighted_avg"));
  assert.equal(replaceArtikulWithSKU("11 из 80 артикулов"), "11 из 80 SKU");
  assert.equal(sanitizeSellerFacingStockText("raw payload narrative fallback"), "");
  assert.ok(!isStockCoverageAISummaryAvailable({ ...baseNarrative, summary: "Покрытие показывает, на сколько дней хватит текущего запаса." }));

  assert.equal(formatSKUCount(8), "8 SKU");
  assert.equal(getRiskSKUCount(segments), 11);
  assert.equal(buildRiskCTALabel(8), "Открыть 8 SKU с риском");
  assert.equal(buildNoSalesCTALabel(3), "Проверить 3 SKU");
  assert.ok(hasStockCoverageRisk(segments));
  assert.ok(!hasStockCoverageRisk([{ key: "critical_0_3", count: 0 }, { key: "shortage_4_7", count: 0 }]));

  const factualDesc = buildFactualDiagnosisDescription(segments, "408 158 ₽/день");
  assert.ok(factualDesc.includes("11 SKU требуют внимания"));
  assert.ok(factualDesc.includes("408 158 ₽/день"));

  assert.equal(
    buildDiagnosisTitle(baseNarrative, [{ key: "critical_0_3", count: 0 }, { key: "shortage_4_7", count: 0 }]),
    "Критического риска дефицита не обнаружено",
  );

  const barSummary = buildDistributionBarSummary(buckets, 80);
  assert.ok(barSummary.includes("Из 80 SKU"));

  console.log("stock-coverage-display tests passed");
}

run();
