import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import {
  buildSkuFocusTrendChartModel,
  buildSkuFocusYAxisTickValues,
} from "@/lib/sku-focus-trend-chart-math";
import type { SkuFocusTrendPoint } from "@/lib/sku-focus-display";

function run() {
  const points: SkuFocusTrendPoint[] = [
    { date: "2026-06-11", orders: 0, revenue: 0, stock: null, coverageDays: null, hasData: true },
    { date: "2026-06-12", orders: 0, revenue: null, stock: null, coverageDays: null, hasData: false },
    { date: "2026-06-13", orders: 2, revenue: 8500, stock: null, coverageDays: null, hasData: true },
    { date: "2026-06-14", orders: 0, revenue: 0, stock: null, coverageDays: null, hasData: true },
    { date: "2026-06-15", orders: 1, revenue: 9100, stock: null, coverageDays: null, hasData: true },
    { date: "2026-06-16", orders: 0, revenue: 0, stock: null, coverageDays: null, hasData: true },
    { date: "2026-06-17", orders: 0, revenue: 0, stock: null, coverageDays: null, hasData: true },
  ];

  const model = buildSkuFocusTrendChartModel({ points, width: 320, height: 72 });
  assert.equal(model.coords.length, 7);
  assert.ok(model.coords[0].x < model.coords[6].x);
  assert.ok(model.yAxisTicks.length >= 2 && model.yAxisTicks.length <= 3);
  assert.ok(model.plotLeft > 0, "reserves space for Y-axis");
  const labels = new Set(model.yAxisTicks.map((t) => t.label));
  assert.equal(labels.size, model.yAxisTicks.length, "Y-axis labels must be unique");

  const flat = buildSkuFocusYAxisTickValues(0, 0);
  assert.deepEqual(flat, [0, 1]);
  const small = buildSkuFocusYAxisTickValues(3, 3);
  assert.ok(small.includes(0) || small.includes(3));
  assert.ok(new Set(small).size === small.length);

  const sectionSource = readFileSync(
    fileURLToPath(new URL("../components/dashboard/sku-focus-section.tsx", import.meta.url)),
    "utf8",
  );
  assert.ok(!sectionSource.includes("VerticalMetricScale"));
  assert.ok(!sectionSource.includes("vertical-metric-scale"));

  console.log("sku-focus-trend-chart-math tests passed");
}

run();
