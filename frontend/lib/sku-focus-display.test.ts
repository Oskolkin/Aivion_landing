import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import type { SKUFocusCard } from "./dashboard-home-types";
import {
  buildKeySkuKpiItems,
  buildSkuFocusCheckText,
  buildSkuFocusKpiItems,
  buildSkuFocusSummaryText,
  buildStockRiskSummaryText,
  buildSkuFocusTrendTooltipText,
  classifyStockRiskBucket,
  countStockRiskItems,
  filterStockRiskItems,
  getCanonicalStockSegment,
  getConfirmedOrdersDelta,
  getConfirmedRevenueDelta,
  getSkuFocusProductSearchQuery,
  getSkuProductHref,
  getKeySkusTotalCount,
  getKeySkusViewAllHref,
  getNoSalesWithStockTotalCount,
  getNoSalesWithStockViewAllHref,
  getStockRiskFilterEmptyText,
  getSkuFocusScenarioBadge,
  getSkuFocusTabLabel,
  getSkuFocusTrendLabel,
  getSkuFocusTrendVariant,
  getVisibleSkuFocusKpiKeys,
  hasDrawableSparkline,
  hasDrawableStockTrend,
  hasSkuFocusProductImage,
  hasSkuFocusRevenueMetrics,
  hasSkuFocusTrendData,
  getSkuFocusTrendEmptyMessage,
  hasSkuProductDirectUrl,
  normalizeSkuFocusImageUrl,
  normalizeSkuFocusStockTrendPoints,
  normalizeSkuFocusTrendPoints,
  resolveVisibleSkuFocusTab,
  resolveSkuFocusCta,
  shouldShowSkuFocusCheckSection,
  shouldShowSkuFocusTrend,
  sortOrderDropItems,
  sortStockRiskItems,
  VISIBLE_SKU_FOCUS_TAB_KEYS,
} from "./sku-focus-display";

const sampleOrderDrop: SKUFocusCard = {
  ozon_product_id: 1,
  sku: 100,
  offer_id: "TEST-001",
  product_name: "Тестовый товар",
  problem: "Просадка заказов",
  key_metric: "Δ заказы: -1",
  orders_delta_day_to_day: -1,
  sparkline_7d: [
    { date: "2026-06-11", value: 1, revenue: 1000 },
    { date: "2026-06-12", value: 2, revenue: 1200 },
    { date: "2026-06-13", value: 3, revenue: 900 },
    { date: "2026-06-14", value: 2, revenue: 800 },
    { date: "2026-06-15", value: 1, revenue: 700 },
  ],
};

function run() {
  assert.deepEqual([...VISIBLE_SKU_FOCUS_TAB_KEYS], [
    "order_drop",
    "key_skus",
  ]);
  assert.equal(resolveVisibleSkuFocusTab("surplus"), "order_drop");
  assert.equal(resolveVisibleSkuFocusTab("stock_risk"), "order_drop");
  assert.equal(resolveVisibleSkuFocusTab(null, "stock_risk"), "stock_risk");

  assert.equal(getSkuFocusTabLabel("order_drop"), "Заказы");
  assert.equal(getSkuFocusScenarioBadge("order_drop"), null);
  assert.equal(getSkuFocusScenarioBadge("stock_risk"), null);
  assert.equal(getSkuFocusScenarioBadge("key_skus"), null);
  assert.equal(getSkuFocusScenarioBadge("surplus")?.label, "Излишки");
  assert.equal(getSkuFocusTrendLabel("stock_risk"), "Остаток, 7 дней");

  const withRevenueSummaryItems: SKUFocusCard[] = [
    { ...sampleOrderDrop, revenue_delta_day_to_day: -1200 },
  ];
  assert.match(
    buildSkuFocusSummaryText("order_drop", 6, withRevenueSummaryItems),
    /6 SKU просели по заказам и выручке/,
  );
  assert.match(
    buildSkuFocusSummaryText("order_drop", 6, [sampleOrderDrop]),
    /6 SKU просели по заказам\. Сначала — товары с самым сильным падением заказов по доступным данным\./,
  );
  assert.match(
    buildSkuFocusSummaryText("stock_risk", 6, []),
    /6 SKU требуют внимания по остаткам/,
  );

  const visibleKeys = getVisibleSkuFocusKpiKeys([sampleOrderDrop], "order_drop");
  assert.deepEqual(visibleKeys, ["orders_delta"]);
  assert.ok(!visibleKeys.includes("revenue"));
  assert.ok(!visibleKeys.includes("orders_comparison"));

  const kpis = buildSkuFocusKpiItems(sampleOrderDrop, "order_drop", visibleKeys);
  assert.equal(kpis.length, 1);
  assert.equal(kpis[0].label, "Динамика заказов");
  assert.equal(kpis[0].sublabel, "к предыдущим 7 дням");
  assert.equal(kpis[0].value, "−1");
  assert.equal(kpis[0].tone, "negative");
  assert.ok(!kpis.some((kpi) => kpi.label.includes("Δ")));

  const withOrdersComparison: SKUFocusCard = {
    ...sampleOrderDrop,
    previous_orders: 10,
    current_orders: 6,
    orders_delta_day_to_day: -4,
  };
  const keysWithComparison = getVisibleSkuFocusKpiKeys([withOrdersComparison], "order_drop");
  assert.ok(keysWithComparison.includes("orders_comparison"));
  const comparisonKpis = buildSkuFocusKpiItems(withOrdersComparison, "order_drop", keysWithComparison);
  assert.equal(comparisonKpis.find((kpi) => kpi.key === "orders_comparison")?.value, "10 → 6");

  const withRevenue: SKUFocusCard = {
    ...sampleOrderDrop,
    revenue_delta_day_to_day: -4830,
  };
  const keysWithRevenue = getVisibleSkuFocusKpiKeys(
    [sampleOrderDrop, withRevenue],
    "order_drop",
  );
  assert.ok(keysWithRevenue.includes("revenue"));
  assert.ok(hasSkuFocusRevenueMetrics([withRevenue]));
  const revenueMissingKpis = buildSkuFocusKpiItems(sampleOrderDrop, "order_drop", keysWithRevenue);
  assert.ok(!revenueMissingKpis.some((kpi) => kpi.key === "revenue"));
  const revenuePresentKpis = buildSkuFocusKpiItems(withRevenue, "order_drop", keysWithRevenue);
  const revenuePresent = revenuePresentKpis.find((kpi) => kpi.key === "revenue");
  assert.match(revenuePresent?.value ?? "", /4\s?830/);
  assert.equal(revenuePresent?.sublabel, "к предыдущим 7 дням");
  assert.equal(revenuePresent?.tone, "negative");
  assert.ok(!revenueMissingKpis.some((kpi) => kpi.label === "Выручка" && kpi.value === "—"));

  assert.equal(getConfirmedOrdersDelta(sampleOrderDrop), -1);
  assert.equal(getConfirmedRevenueDelta(withRevenue), -4830);

  const sorted = sortOrderDropItems([
    { ...sampleOrderDrop, ozon_product_id: 1, orders_delta_day_to_day: -3, product_name: "B" },
    { ...sampleOrderDrop, ozon_product_id: 2, orders_delta_day_to_day: -10, product_name: "A" },
    { ...sampleOrderDrop, ozon_product_id: 3, orders_delta_day_to_day: -7, product_name: "C" },
    { ...sampleOrderDrop, ozon_product_id: 4, key_metric: "", orders_delta_day_to_day: undefined },
  ]);
  assert.deepEqual(
    sorted.map((item) => getConfirmedOrdersDelta(item)),
    [-10, -7, -3, null],
  );

  const tieSorted = sortOrderDropItems([
    {
      ...sampleOrderDrop,
      ozon_product_id: 10,
      orders_delta_day_to_day: -5,
      revenue_delta_day_to_day: -100,
      product_name: "A",
    },
    {
      ...sampleOrderDrop,
      ozon_product_id: 11,
      orders_delta_day_to_day: -5,
      revenue_delta_day_to_day: -500,
      product_name: "B",
    },
  ]);
  assert.equal(getConfirmedRevenueDelta(tieSorted[0]), -500);

  assert.equal(hasSkuFocusTrendData(sampleOrderDrop), true);
  assert.equal(
    hasSkuFocusTrendData({ ...sampleOrderDrop, sparkline_7d: [{ date: "2026-06-11", value: 1 }] }),
    false,
  );
  assert.equal(
    hasSkuFocusTrendData({
      ...sampleOrderDrop,
      trend_status: "not_enough_data",
      sparkline_7d: [
        { date: "2026-06-11", value: 1 },
        { date: "2026-06-12", value: 2 },
      ],
    }),
    false,
  );
  assert.equal(
    getSkuFocusTrendEmptyMessage({
      ...sampleOrderDrop,
      trend_status: "not_enough_data",
      trend_message: "Недостаточно точек для тренда",
    }),
    "Недостаточно точек для тренда",
  );
  assert.equal(
    hasDrawableSparkline([
      { date: "2026-06-11", value: 1, has_data: true },
      { date: "2026-06-12", value: null, has_data: false },
      { date: "2026-06-13", value: 2, has_data: true },
    ]),
    true,
  );
  assert.equal(
    hasDrawableSparkline([
      { date: "2026-06-11", value: 0, has_data: true },
      { date: "2026-06-12", value: 0, has_data: true },
    ]),
    true,
  );

  assert.equal(
    buildSkuFocusCheckText({ ...sampleOrderDrop, check_hint: "Проверить цену" }, "stock_risk"),
    "Проверить цену",
  );
  assert.match(
    buildSkuFocusCheckText(
      { ...sampleOrderDrop, problem: "", key_metric: "Остаток 3" },
      "stock_risk",
    ),
    /остатки/,
  );

  assert.equal(normalizeSkuFocusImageUrl({ ...sampleOrderDrop, image_url: "https://example.com/a.jpg" }), "https://example.com/a.jpg");
  assert.equal(
    normalizeSkuFocusImageUrl({ ...sampleOrderDrop, image_url: null, imageUrl: "https://example.com/b.jpg" } as SKUFocusCard),
    "https://example.com/b.jpg",
  );
  assert.equal(hasSkuFocusProductImage(sampleOrderDrop), false);
  assert.equal(hasSkuProductDirectUrl({ ...sampleOrderDrop, product_url: "https://ozon.ru/product/1" }), true);

  const directCta = resolveSkuFocusCta({
    ...sampleOrderDrop,
    product_url: "https://ozon.ru/product/1",
  });
  assert.equal(directCta?.kind, "direct");
  assert.equal(directCta?.label, "Открыть товар");
  assert.equal(directCta?.href, "https://ozon.ru/product/1");

  const searchCta = resolveSkuFocusCta(sampleOrderDrop);
  assert.equal(searchCta, null);
  assert.equal(getSkuFocusProductSearchQuery(sampleOrderDrop), "TEST-001");

  const skuOnlyCta = resolveSkuFocusCta({
    ...sampleOrderDrop,
    offer_id: null,
    product_url: null,
  });
  assert.equal(skuOnlyCta, null);

  assert.equal(
    resolveSkuFocusCta({
      ozon_product_id: 2,
      product_name: "Без идентификаторов",
      problem: "",
      key_metric: "",
    }),
    null,
  );
  assert.equal(getSkuProductHref(sampleOrderDrop), null);

  const trendPoints = normalizeSkuFocusTrendPoints(sampleOrderDrop);
  assert.equal(trendPoints.length, 5);
  assert.equal(trendPoints[0].orders, 1);
  assert.equal(trendPoints[0].revenue, 1000);
  assert.equal(trendPoints[0].hasData, true);
  assert.match(buildSkuFocusTrendTooltipText(trendPoints[0]), /Заказы: 1/);
  assert.match(buildSkuFocusTrendTooltipText(trendPoints[0]), /Выручка: 1\s?000 ₽/);
  assert.match(
    buildSkuFocusTrendTooltipText({ date: "2026-06-11", orders: 2, revenue: null, stock: null, coverageDays: null, hasData: true }),
    /Выручка: нет данных/,
  );

  const stockItem: SKUFocusCard = {
    ozon_product_id: 10,
    product_name: "Светильник подсветки кухонного шкафа",
    problem: "Скоро закончатся",
    key_metric: "Остаток 2 · покрытие 0.6 дн.",
    current_stock: 2,
    previous_stock: 4,
    coverage_days: 0.6,
    coverage_bucket: "critical_0_3",
    revenue_at_risk_7d: 10675,
    stock_trend_7d: [
      { date: "2026-06-11", stock: 14, coverage_days: 4.5, has_data: true },
      { date: "2026-06-12", stock: 11, coverage_days: 3.6, has_data: true },
      { date: "2026-06-13", stock: 9, coverage_days: 3.0, has_data: true },
      { date: "2026-06-14", stock: 7, coverage_days: 2.3, has_data: true },
      { date: "2026-06-15", stock: 5, coverage_days: 1.6, has_data: true },
      { date: "2026-06-16", stock: 4, coverage_days: 1.3, has_data: true },
      { date: "2026-06-17", stock: 2, coverage_days: 0.6, has_data: true },
    ],
  };
  const stockTrendPoints = normalizeSkuFocusStockTrendPoints(stockItem);
  assert.equal(stockTrendPoints.length, 7);
  assert.equal(stockTrendPoints[0].stock, 14);
  assert.equal(stockTrendPoints[6].coverageDays, 0.6);
  assert.equal(hasSkuFocusTrendData(stockItem, "stock_risk"), true);
  assert.equal(
    hasDrawableStockTrend([
      { stock: 0, has_data: true },
      { stock: 2, has_data: true },
    ]),
    true,
  );

  const stockVisible = getVisibleSkuFocusKpiKeys([stockItem], "stock_risk");
  assert.deepEqual(stockVisible, ["stock", "coverage", "revenue_at_risk"]);
  const stockKpis = buildSkuFocusKpiItems(stockItem, "stock_risk", stockVisible);
  assert.equal(stockKpis.find((kpi) => kpi.key === "stock")?.value, "4 → 2 шт.");
  assert.match(stockKpis.find((kpi) => kpi.key === "coverage")?.value ?? "", /0,6 дн\./);
  assert.match(stockKpis.find((kpi) => kpi.key === "revenue_at_risk")?.value ?? "", /10\s?675 ₽ под риском/);
  assert.ok(!("scale" in (stockKpis.find((kpi) => kpi.key === "coverage") ?? {})), "no vertical scale on coverage");

  assert.equal(getCanonicalStockSegment(stockItem), "critical_0_3");
  assert.equal(classifyStockRiskBucket(stockItem), "critical");

  const stockUniverse: SKUFocusCard[] = [
    { ...stockItem, ozon_product_id: 1, coverage_bucket: "out_of_stock", current_stock: 0, coverage_days: 0 },
    { ...stockItem, ozon_product_id: 2, coverage_bucket: "critical_0_3", coverage_days: 1.5 },
    { ...stockItem, ozon_product_id: 3, coverage_bucket: "shortage_4_7", coverage_days: 5 },
    { ...stockItem, ozon_product_id: 4, coverage_bucket: "planning_8_14", coverage_days: 10, revenue_at_risk_7d: null },
    { ...stockItem, ozon_product_id: 5, coverage_bucket: "planning_8_14", coverage_days: 12, revenue_at_risk_7d: null },
  ];
  assert.equal(countStockRiskItems(stockUniverse, "all-risk"), 3);
  assert.equal(countStockRiskItems(stockUniverse, "out-of-stock"), 1);
  assert.equal(countStockRiskItems(stockUniverse, "critical"), 1);
  assert.equal(countStockRiskItems(stockUniverse, "short"), 1);
  assert.equal(countStockRiskItems(stockUniverse, "planning"), 2);
  assert.equal(filterStockRiskItems(stockUniverse, "planning").length, 2);
  assert.ok(
    filterStockRiskItems(stockUniverse, "all-risk").every((item) => item.coverage_bucket !== "planning_8_14"),
  );
  assert.equal(
    buildStockRiskSummaryText(21, "all-risk"),
    "21 SKU требуют внимания по остаткам. Сначала показаны товары с наиболее низким покрытием и максимальной выручкой под риском.",
  );
  assert.match(buildStockRiskSummaryText(7, "planning"), /7 SKU имеют покрытие от восьми до четырнадцати дней/);
  assert.equal(getStockRiskFilterEmptyText(), "В выбранном диапазоне SKU не обнаружены");
  assert.equal(
    buildSkuFocusSummaryText("stock_risk", 7, [], { stockFilter: "planning" }),
    buildStockRiskSummaryText(7, "planning"),
  );

  const stockSorted = sortStockRiskItems([
    { ...stockItem, ozon_product_id: 1, product_name: "A", coverage_days: 2.2, revenue_at_risk_7d: 1000 },
    { ...stockItem, ozon_product_id: 2, product_name: "B", coverage_days: 0.6, revenue_at_risk_7d: 5000 },
    { ...stockItem, ozon_product_id: 3, product_name: "C", coverage_days: 0.6, revenue_at_risk_7d: 8000 },
  ]);
  assert.equal(stockSorted[0].ozon_product_id, 3);
  assert.equal(stockSorted[1].ozon_product_id, 2);
  assert.equal(stockSorted[2].ozon_product_id, 1);

  const sparklineOnly: SKUFocusCard = {
    ...sampleOrderDrop,
    key_metric: "",
    orders_delta_day_to_day: undefined,
    sparkline_7d: [
      { date: "2026-06-11", value: 5 },
      { date: "2026-06-12", value: 3 },
      { date: "2026-06-13", value: 1 },
    ],
  };
  const sparklineKpis = buildSkuFocusKpiItems(sparklineOnly, "order_drop");
  assert.equal(sparklineKpis.length, 0, "does not derive orders comparison from sparkline");

  const displaySource = readFileSync(fileURLToPath(new URL("./sku-focus-display.ts", import.meta.url)), "utf8");
  const sectionSource = readFileSync(
    fileURLToPath(new URL("../components/dashboard/sku-focus-section.tsx", import.meta.url)),
    "utf8",
  );
  for (const token of ["7010", "2126", "1966", "1728", "1677", "1299", "Швабра с отжимом"]) {
    assert.ok(!displaySource.includes(token), `display must not hardcode ${token}`);
    assert.ok(!sectionSource.includes(token), `section must not hardcode ${token}`);
  }

  const noSalesItem: SKUFocusCard = {
    ozon_product_id: 100,
    sku: 900100,
    offer_id: "NO-SALES-1",
    product_name: "Демо SKU без продаж",
    problem: "",
    key_metric: "Остаток 78",
    current_stock: 78,
    no_sales_days: 3,
    frozen_stock_value: 12450,
    previous_day_orders: 0,
  };
  assert.equal(getSkuFocusScenarioBadge("no_sales_with_stock"), null);
  assert.equal(shouldShowSkuFocusTrend("no_sales_with_stock"), false);
  assert.equal(shouldShowSkuFocusCheckSection("no_sales_with_stock"), false);
  assert.match(
    buildSkuFocusSummaryText("no_sales_with_stock", 7, [noSalesItem]),
    /7 SKU без продаж 14\+ дней при наличии остатка\. Сначала — товары с наибольшей суммой в остатках\./,
  );
  assert.match(
    buildSkuFocusSummaryText("no_sales_with_stock", 3, [noSalesItem]),
    /3 SKU без продаж 14\+ дней при наличии остатка\./,
  );
  const noSalesKpis = buildSkuFocusKpiItems(
    noSalesItem,
    "no_sales_with_stock",
    getVisibleSkuFocusKpiKeys([noSalesItem], "no_sales_with_stock"),
  );
  assert.equal(noSalesKpis.find((kpi) => kpi.key === "stock")?.value, "78 шт.");
  assert.equal(noSalesKpis.find((kpi) => kpi.key === "no_sales_days")?.value, "3 дн.");
  assert.match(noSalesKpis.find((kpi) => kpi.key === "frozen_stock_value")?.value ?? "", /12\s?450 ₽/);
  const noFrozenKpis = buildSkuFocusKpiItems(
    { ...noSalesItem, frozen_stock_value: null },
    "no_sales_with_stock",
    getVisibleSkuFocusKpiKeys([{ ...noSalesItem, frozen_stock_value: null }], "no_sales_with_stock"),
  );
  assert.ok(!noFrozenKpis.some((kpi) => kpi.key === "frozen_stock_value"));
  assert.ok(!noFrozenKpis.some((kpi) => kpi.value.includes("—")));
  assert.equal(getNoSalesWithStockTotalCount({ no_sales_with_stock_total_count: 9, no_sales_with_stock: [noSalesItem] }, 1), 9);
  assert.equal(getNoSalesWithStockViewAllHref(), null);

  const keySkuItem = {
    ozon_product_id: 701,
    sku: 900701,
    offer_id: "KEY-1",
    product_name: "Ключевой товар A",
    image_url: "https://example.com/key.jpg",
    revenue_7d: 180000,
    orders_7d: 40,
    revenue_share_pct: 18,
    cumulative_revenue_share_pct: 18,
    coverage_days: 6.5,
    current_stock: 58,
    revenue_trend_7d: [
      { date: "2026-06-11", revenue: 24000, orders: 5, has_data: true },
      { date: "2026-06-12", revenue: 27000, orders: 6, has_data: true },
      { date: "2026-06-13", revenue: 21000, orders: 5, has_data: true },
      { date: "2026-06-14", revenue: 26000, orders: 6, has_data: true },
      { date: "2026-06-15", revenue: 25000, orders: 6, has_data: true },
      { date: "2026-06-16", revenue: 28000, orders: 6, has_data: true },
      { date: "2026-06-17", revenue: 29000, orders: 6, has_data: true },
    ],
  };
  const keyKpis = buildKeySkuKpiItems(keySkuItem);
  assert.equal(keyKpis.find((kpi) => kpi.key === "revenue_share")?.value, "18%");
  assert.match(keyKpis.find((kpi) => kpi.key === "revenue_7d")?.value ?? "", /180\s?000 ₽/);
  assert.match(keyKpis.find((kpi) => kpi.key === "coverage")?.value ?? "", /6,5 дн\./);
  assert.equal(getSkuFocusTrendLabel("key_skus"), "Выручка, 7 дней");
  assert.equal(getSkuFocusTrendVariant("key_skus"), "revenue");
  assert.equal(
    buildSkuFocusSummaryText("key_skus", 9, [], { keyRevenueSharePct: 72.4 }),
    "9 SKU формируют 72,4% выручки за последние 7 дней. Сначала — товары с наибольшей долей в выручке.",
  );
  assert.equal(
    buildSkuFocusSummaryText("key_skus", 3, [], { keyRevenueSharePct: 55 }),
    "3 SKU формируют 55% выручки за последние 7 дней.",
  );
  assert.equal(getKeySkusTotalCount({ key_skus_total_count: 9, key_skus: [keySkuItem] }, 1), 9);
  assert.equal(getKeySkusViewAllHref(), null);
  assert.equal(buildSkuFocusCheckText({ ...sampleOrderDrop, problem: "" }, "key_skus"), "Проверьте наличие ключевых SKU, актуальность цен и поддерживающую рекламу.");

  console.log("sku-focus-display tests passed");
}

run();
