import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { SKUFocusSection } from "@/components/dashboard/sku-focus-section";
import { SKU_FOCUS_SECTION_ID } from "@/lib/sku-focus-hash";
import type { SKUFocusCard } from "@/lib/dashboard-home-types";

const fullItem: SKUFocusCard = {
  ozon_product_id: 901000100036,
  sku: 801000100036,
  offer_id: "HOME-TEXT-036",
  product_name: "Занавеска blackout 140×260",
  image_url: "https://example.com/image.jpg",
  problem: "Просадка заказов",
  key_metric: "Δ заказы: -1",
  orders_delta_day_to_day: -1,
  revenue_delta_day_to_day: -1200,
  sparkline_7d: [
    { date: "2026-06-11", value: 1, revenue: 1000 },
    { date: "2026-06-12", value: 1, revenue: 1100 },
    { date: "2026-06-13", value: 2, revenue: 900 },
    { date: "2026-06-14", value: 3, revenue: 800 },
    { date: "2026-06-15", value: 3, revenue: 700 },
    { date: "2026-06-16", value: 2, revenue: 600 },
    { date: "2026-06-17", value: 1, revenue: 500 },
  ],
};

const partialItem: SKUFocusCard = {
  ozon_product_id: 2,
  product_name: "Товар без полных данных",
  problem: "Просадка заказов",
  key_metric: "",
};

const itemWithOrdersComparison: SKUFocusCard = {
  ...fullItem,
  ozon_product_id: 3,
  previous_orders: 3,
  current_orders: 2,
  orders_delta_day_to_day: -1,
};

const itemWithRevenue: SKUFocusCard = {
  ...fullItem,
  ozon_product_id: 4,
  revenue_delta_day_to_day: -1200,
};

const flatTrendItem: SKUFocusCard = {
  ...fullItem,
  ozon_product_id: 5,
  sparkline_7d: [
    { date: "2026-06-11", value: 2, revenue: 100 },
    { date: "2026-06-12", value: 2, revenue: 100 },
    { date: "2026-06-13", value: 2, revenue: 100 },
    { date: "2026-06-14", value: 2, revenue: 100 },
  ],
};

const sortItemB: SKUFocusCard = {
  ...fullItem,
  ozon_product_id: 102,
  product_name: "B",
  orders_delta_day_to_day: -10,
  revenue_delta_day_to_day: -200,
};

const sortItemC: SKUFocusCard = {
  ...fullItem,
  ozon_product_id: 103,
  product_name: "C",
  orders_delta_day_to_day: -7,
  revenue_delta_day_to_day: -500,
};

const sortItemA: SKUFocusCard = {
  ...fullItem,
  ozon_product_id: 101,
  product_name: "A",
  orders_delta_day_to_day: -3,
  revenue_delta_day_to_day: -100,
};

const sortTieItemHeavyRevenue: SKUFocusCard = {
  ...fullItem,
  ozon_product_id: 201,
  product_name: "HeavyRev",
  orders_delta_day_to_day: -1,
  revenue_delta_day_to_day: -7010,
};

const sortTieItemLightRevenue: SKUFocusCard = {
  ...fullItem,
  ozon_product_id: 202,
  product_name: "LightRev",
  orders_delta_day_to_day: -1,
  revenue_delta_day_to_day: -2126,
};

const stockRiskItem: SKUFocusCard = {
  ozon_product_id: 301,
  sku: 801000100301,
  offer_id: "HOME-LIGHT-301",
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

function renderStockRiskSection(items: SKUFocusCard[] = [stockRiskItem]) {
  return renderToStaticMarkup(
    <SKUFocusSection
      initialTab="stock_risk"
      data={{
        order_drop: [],
        stock_risk: items,
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [],
      }}
    />,
  );
}

function run() {
  const stockHtml = renderStockRiskSection();
  assert.ok(!stockHtml.includes("Остатки"), "stock tab is no longer rendered on the SKU page");
  assert.ok(!stockHtml.includes("Без продаж"), "no-sales tab is no longer rendered on the SKU page");

  const emptyHtml = renderToStaticMarkup(
    <SKUFocusSection
      data={{
        order_drop: [],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [],
      }}
    />,
  );

  assert.ok(emptyHtml.includes(`id="${SKU_FOCUS_SECTION_ID}"`), "stable section id");
  assert.ok(emptyHtml.includes("SKU-фокус"));
  assert.ok(emptyHtml.includes('data-testid="sku-focus-tabs"'));
  assert.ok(emptyHtml.includes("Заказы"));
  assert.ok(!emptyHtml.includes("Остатки"));
  assert.ok(!emptyHtml.includes("Без продаж"));
  assert.ok(emptyHtml.includes("Ключевые"));
  assert.ok(!emptyHtml.includes("Излишки"));
  assert.ok(!emptyHtml.includes('data-testid="sku-focus-tab-surplus"'));
  const tabButtonCount = (emptyHtml.match(/data-testid="sku-focus-tab-/g) ?? []).length;
  assert.equal(tabButtonCount, 2, "SKU-фокус shows only orders and key-SKU tabs");
  assert.ok(!emptyHtml.includes(">Просадка<"));
  assert.ok(
    emptyHtml.includes("Нет SKU с подтверждённой просадкой заказов за выбранный период."),
  );
  assert.ok(!emptyHtml.includes('data-testid="sku-focus-summary-banner"'), "no banner when empty");

  const html = renderToStaticMarkup(
    <SKUFocusSection
      data={{
        order_drop: [fullItem, partialItem],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [],
      }}
    />,
  );

  assert.ok(html.includes('data-testid="sku-focus-summary-banner"'));
  assert.match(html, /2 SKU просели по заказам и выручке/);
  assert.ok(!html.includes("Падение заказов"));
  assert.ok(!html.includes('data-testid="sku-focus-scenario-badge"'));
  assert.ok(html.includes("Динамика заказов"));
  assert.ok(html.includes("к предыдущим 7 дням"));
  assert.ok(!html.includes("Δ заказов"));
  assert.ok(!html.includes("Что проверить"));
  assert.ok(html.includes('data-testid="product-image-product-card"'), "rectangular product-card image");
  assert.ok(html.includes('src="https://example.com/image.jpg"'), "uses image_url when present");

  const relativeImageHtml = renderToStaticMarkup(
    <SKUFocusSection
      data={{
        order_drop: [{ ...fullItem, image_url: "/demo/products/product-demo-2.jpg" }],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [],
      }}
    />,
  );
  assert.ok(
    relativeImageHtml.includes('src="/demo/products/product-demo-2.jpg"'),
    "supports relative image_url from API",
  );
  assert.ok(html.includes('data-testid="sku-focus-trend-point"'));
  assert.ok(html.includes('data-testid="sku-focus-kpi-revenue"'));

  const noImageHtml = renderToStaticMarkup(
    <SKUFocusSection
      data={{
        order_drop: [partialItem],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [],
      }}
    />,
  );
  assert.ok(
    noImageHtml.includes('src="/demo/products/'),
    "demo fallback image when no image_url",
  );
  assert.ok(noImageHtml.includes('data-testid="product-image-product-card"'));
  assert.ok(noImageHtml.includes("rounded-xl"), "product image uses rectangular corners");

  const comparisonHtml = renderToStaticMarkup(
    <SKUFocusSection
      data={{
        order_drop: [itemWithOrdersComparison],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [],
      }}
    />,
  );
  assert.ok(comparisonHtml.includes("Заказы"));
  assert.ok(comparisonHtml.includes("3 → 2"));

  const revenueHtml = renderToStaticMarkup(
    <SKUFocusSection
      data={{
        order_drop: [fullItem, itemWithRevenue],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [],
      }}
    />,
  );
  assert.ok(revenueHtml.includes("Выручка"));
  assert.ok(revenueHtml.includes("к предыдущим 7 дням"));
  assert.ok(!revenueHtml.includes('data-testid="sku-focus-card-cta"'));
  assert.ok(!revenueHtml.includes("Найти в товарах"));
  assert.ok(!revenueHtml.includes("/app/products?search=HOME-TEXT-036"));
  assert.ok(!revenueHtml.includes("Открыть товар"));

  const directUrlHtml = renderToStaticMarkup(
    <SKUFocusSection
      data={{
        order_drop: [{ ...fullItem, product_url: "https://ozon.ru/product/123" }],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [],
      }}
    />,
  );
  assert.ok(directUrlHtml.includes("Открыть товар"));
  assert.ok(directUrlHtml.includes('data-cta-kind="direct"'));
  assert.ok(directUrlHtml.includes('href="https://ozon.ru/product/123"'));

  const noCtaHtml = renderToStaticMarkup(
    <SKUFocusSection
      data={{
        order_drop: [partialItem],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [],
      }}
    />,
  );
  assert.ok(!noCtaHtml.includes('data-testid="sku-focus-card-cta"'));
  assert.ok(!noCtaHtml.includes("Найти в товарах"));
  assert.ok(!noCtaHtml.includes("Открыть товар"));

  const flatTrendHtml = renderToStaticMarkup(
    <SKUFocusSection
      data={{
        order_drop: [flatTrendItem],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [],
      }}
    />,
  );
  assert.ok(flatTrendHtml.includes("<polyline"), "flat sparkline still renders");

  const sortHtml = renderToStaticMarkup(
    <SKUFocusSection
      data={{
        order_drop: [sortItemA, sortItemB, sortItemC],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [],
      }}
    />,
  );
  const bIndex = sortHtml.indexOf(">B<");
  const cIndex = sortHtml.indexOf(">C<");
  const aIndex = sortHtml.indexOf(">A<");
  assert.ok(bIndex < cIndex && cIndex < aIndex, "sorted by strongest order drop first");

  const revenueTieHtml = renderToStaticMarkup(
    <SKUFocusSection
      data={{
        order_drop: [sortTieItemLightRevenue, sortTieItemHeavyRevenue],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [],
      }}
    />,
  );
  assert.ok(
    revenueTieHtml.indexOf("HeavyRev") < revenueTieHtml.indexOf("LightRev"),
    "equal orders delta sorts by stronger revenue drop first",
  );

  const surplusItem: SKUFocusCard = {
    ozon_product_id: 601,
    sku: 900601,
    offer_id: "SURPLUS-1",
    product_name: "Товар с излишком",
    problem: "Медленная оборачиваемость",
    key_metric: "Остаток 120",
    sparkline_7d: [
      { date: "2026-06-11", value: 1 },
      { date: "2026-06-12", value: 1 },
      { date: "2026-06-13", value: 0 },
    ],
  };
  const surplusHiddenHtml = renderToStaticMarkup(
    <SKUFocusSection
      initialTab="surplus"
      data={{
        order_drop: [],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [surplusItem],
        key_skus: [],
      }}
    />,
  );
  assert.ok(!surplusHiddenHtml.includes("Излишки"));
  assert.ok(!surplusHiddenHtml.includes("Товар с излишком"));
  assert.ok(
    surplusHiddenHtml.includes("Нет SKU с подтверждённой просадкой заказов за выбранный период."),
    "hidden surplus tab falls back to order_drop",
  );
  assert.ok(surplusHiddenHtml.includes('data-testid="sku-focus-tab-order_drop"'));

  const keySkuItem = {
    ozon_product_id: 701,
    sku: 900701,
    offer_id: "KEY-1",
    product_name: "Ключевой товар A",
    product_url: "https://www.ozon.ru/product/701",
    revenue_7d: 180000,
    orders_7d: 40,
    revenue_share_pct: 18,
    cumulative_revenue_share_pct: 18,
    coverage_days: 2,
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
  const keySkusHtml = renderToStaticMarkup(
    <SKUFocusSection
      initialTab="key_skus"
      data={{
        order_drop: [],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [keySkuItem],
        key_skus_total_count: 9,
        store_total_revenue_7d: 1_000_000,
        key_skus_total_revenue_7d: 724_000,
      }}
    />,
  );
  assert.ok(!keySkusHtml.includes("Ключевой SKU"));
  assert.ok(!keySkusHtml.includes("Индекс"));
  assert.ok(!keySkusHtml.includes("Что проверить"));
  assert.ok(!keySkusHtml.includes('data-testid="sku-focus-check-section"'));
  assert.ok(keySkusHtml.includes("18%"));
  assert.match(keySkusHtml, /180\s?000 ₽/);
  assert.ok(keySkusHtml.includes("2,0 дн."));
  assert.ok(keySkusHtml.includes("Выручка, 7 дней"));
  assert.ok(keySkusHtml.includes("1 SKU формируют 72,4% выручки"));
  assert.ok(keySkusHtml.includes('data-rendered-count="1"'));
  assert.ok(!keySkusHtml.includes("9 SKU формируют"), "banner must match rendered cards, not total_count");
  assert.ok(!keySkusHtml.includes("формируют 80% выручки"));
  assert.ok(!keySkusHtml.includes('data-testid="sku-focus-key-skus-view-all"'));
  assert.ok(!keySkusHtml.includes("/app/products?filter=key-skus"));

  const keySkusSmallHtml = renderToStaticMarkup(
    <SKUFocusSection
      initialTab="key_skus"
      data={{
        order_drop: [],
        stock_risk: [],
        no_sales_with_stock: [],
        surplus: [],
        key_skus: [keySkuItem],
        key_skus_total_count: 3,
      }}
    />,
  );
  assert.ok(!keySkusSmallHtml.includes('data-testid="sku-focus-key-skus-view-all"'));

  console.log("sku-focus-section tests passed");
}

run();
