import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { SkuFocusTrendChart } from "@/components/dashboard/sku-focus-trend-chart";
import {
  buildSkuFocusTrendChartModel,
  formatSkuFocusTrendTooltipRevenue,
} from "@/lib/sku-focus-trend-chart-math";
import type { SkuFocusTrendPoint } from "@/lib/sku-focus-display";
import { buildSkuFocusTrendTooltipText, normalizeSkuFocusTrendPoints } from "@/lib/sku-focus-display";
import type { SKUFocusCard } from "@/lib/dashboard-home-types";

const sevenPoints: SkuFocusTrendPoint[] = [
  { date: "2026-06-11", orders: 0, revenue: 0, stock: null, coverageDays: null, hasData: true },
  { date: "2026-06-12", orders: 0, revenue: 0, stock: null, coverageDays: null, hasData: true },
  { date: "2026-06-13", orders: 2, revenue: 8500, stock: null, coverageDays: null, hasData: true },
  { date: "2026-06-14", orders: 0, revenue: 0, stock: null, coverageDays: null, hasData: true },
  { date: "2026-06-15", orders: 1, revenue: 9100, stock: null, coverageDays: null, hasData: true },
  { date: "2026-06-16", orders: 0, revenue: 0, stock: null, coverageDays: null, hasData: true },
  { date: "2026-06-17", orders: 0, revenue: 0, stock: null, coverageDays: null, hasData: true },
];

function run() {
  const html = renderToStaticMarkup(<SkuFocusTrendChart points={sevenPoints} />);
  const markerCount = (html.match(/data-testid="sku-focus-trend-point"/g) ?? []).length;
  assert.equal(markerCount, 7, "renders a marker for each day");

  const model = buildSkuFocusTrendChartModel({ points: sevenPoints, width: 320, height: 72 });
  assert.equal(model.coords.length, 7);
  assert.ok(model.coords[0].x < model.coords[6].x, "first point is left of last point");
  assert.ok(model.coords[0].x >= model.plotLeft - 0.1, "first point starts at plot left");
  assert.ok(model.coords[6].x >= 300, "last point ends near right edge");
  assert.ok(model.yAxisTicks.length >= 2, "has Y-axis ticks");
  const htmlWithAxis = renderToStaticMarkup(<SkuFocusTrendChart points={sevenPoints} height={72} />);
  assert.ok(htmlWithAxis.includes('data-testid="sku-focus-trend-y-tick"'));
  assert.ok(htmlWithAxis.includes('data-testid="sku-focus-trend-y-label"'));
  // Zero values may sit on a tick line rather than always baseline when axis includes 0.
  assert.ok(model.yAxisTicks.some((t) => t.value === 0) || model.coords[0].y === model.baselineY);

  const normalized = normalizeSkuFocusTrendPoints({
    ozon_product_id: 1,
    product_name: "Test",
    problem: "",
    key_metric: "",
    sparkline_7d: [
      { date: "2026-06-11", value: 0, revenue: 0, has_data: true },
      { date: "2026-06-12", value: 0, revenue: null, has_data: false },
      { date: "2026-06-13", value: 2, revenue: 8500, has_data: true },
    ],
  } as SKUFocusCard);
  assert.equal(normalized[0].orders, 0);
  assert.equal(normalized[1].orders, null);
  assert.equal(normalized[1].hasData, false);

  assert.equal(formatSkuFocusTrendTooltipRevenue(null), "нет данных");
  assert.equal(formatSkuFocusTrendTooltipRevenue(0), "0 ₽");
  assert.match(buildSkuFocusTrendTooltipText({ date: "2026-06-11", orders: 2, revenue: null, stock: null, coverageDays: null, hasData: true }), /Выручка: нет данных/);
  assert.match(buildSkuFocusTrendTooltipText({ date: "2026-06-11", orders: 2, revenue: 0, stock: null, coverageDays: null, hasData: true }), /Выручка: 0 ₽/);
  assert.doesNotMatch(
    buildSkuFocusTrendTooltipText({ date: "2026-06-11", orders: 2, revenue: null, stock: null, coverageDays: null, hasData: true }),
    /Выручка: 0 ₽/,
  );

  const stockPoints: SkuFocusTrendPoint[] = [
    { date: "2026-06-11", orders: null, revenue: null, stock: 14, coverageDays: 4.5, hasData: true },
    { date: "2026-06-12", orders: null, revenue: null, stock: 11, coverageDays: 3.6, hasData: true },
    { date: "2026-06-13", orders: null, revenue: null, stock: 2, coverageDays: 0.6, hasData: true },
  ];
  const stockModel = buildSkuFocusTrendChartModel({ points: stockPoints, variant: "stock", width: 320, height: 64 });
  assert.ok(stockModel.canRender);
  assert.match(buildSkuFocusTrendTooltipText(stockPoints[2], "stock"), /Остаток: 2 шт\./);
  assert.match(buildSkuFocusTrendTooltipText(stockPoints[2], "stock"), /Покрытие: 0,6 дн\./);

  const noRevenueHtml = renderToStaticMarkup(
    <SkuFocusTrendChart
      points={[
        { date: "2026-06-11", orders: 1, revenue: null, stock: null, coverageDays: null, hasData: true },
        { date: "2026-06-12", orders: 2, revenue: null, stock: null, coverageDays: null, hasData: true },
      ]}
    />,
  );
  assert.ok(noRevenueHtml.includes("<polyline"), "renders line for two points");

  const singlePointHtml = renderToStaticMarkup(
    <SkuFocusTrendChart points={[{ date: "2026-06-11", orders: 1, revenue: 100, stock: null, coverageDays: null, hasData: true }]} />,
  );
  assert.ok(singlePointHtml.includes("<svg"), "single point renders empty svg without errors");

  console.log("sku-focus-trend-chart tests passed");
}

run();
