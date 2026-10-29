import assert from "node:assert/strict";
import type { DashboardHomeResponse } from "./dashboard-home-types";
import { buildStockAttentionChartLayout, buildStockPageViewModel, getInitialStockPageFilter, getStockPageItems } from "./stock-page-view-model";

const data = {
  ai_diagnostics: {
    stock_coverage: {
      total_skus: 80,
      stock_coverage_narrative: {
        status: "available",
        revenue_at_risk: { status: "available", value: "85 900 ₽ / день" },
        no_sales_with_stock: { status: "available" },
        sku_focus_link: { label: "", target: "sku-focus-stock" },
        segments: [
          ["out_of_stock", 4], ["critical_0_3", 7], ["shortage_4_7", 7], ["planning_8_14", 4],
          ["healthy_14plus", 28], ["surplus", 15], ["no_sales", 15], ["collecting_data", 0],
        ].map(([key, count]) => ({ key: String(key), count: Number(count), label: "", severity: "" })),
      },
    },
  },
  sku_focus: {
    stock_risk: [
      { ozon_product_id: 1, product_name: "Critical", problem: "", key_metric: "", coverage_bucket: "critical_0_3", current_stock: 2, avg_daily_revenue_7d: 100 },
      { ozon_product_id: 2, product_name: "Short", problem: "", key_metric: "", coverage_bucket: "shortage_4_7" },
      { ozon_product_id: 3, product_name: "Planning", problem: "", key_metric: "", coverage_bucket: "planning_8_14" },
    ],
    no_sales_with_stock: [{ ozon_product_id: 4, product_name: "No sales", problem: "", key_metric: "", no_sales_days: 14, frozen_stock_value: 300 }],
  },
  top_actions: [],
} as unknown as DashboardHomeResponse;

const vm = buildStockPageViewModel(data);
assert.equal(vm.urgentCount, 11, "urgent excludes shortage");
assert.deepEqual(vm.attentionBuckets.map((bucket) => bucket.key), ["out_of_stock", "critical_0_3", "shortage_4_7", "planning_8_14"]);
assert.equal(vm.attentionBuckets.reduce((sum, bucket) => sum + bucket.count, 0), vm.attentionCount, "chart reconciles with attention");
assert.equal(vm.kpis.critical, 11);
assert.equal(vm.kpis.shortage, 11);
assert.equal(vm.attentionCount, 22);
assert.equal(vm.sufficientCoverageCount, 58);
assert.equal(vm.healthyCount, 58);
assert.equal(vm.attentionCount, vm.urgentCount + vm.supplyPlanCount);
assert.equal(vm.attentionCount + vm.sufficientCoverageCount, vm.totalSkus);
assert.equal(vm.totalFromSegments, 80, "exclusive segments reconcile with total");
assert.equal(vm.hasReconciliationMismatch, false);
const chart = buildStockAttentionChartLayout(vm.attentionBuckets);
assert.deepEqual(chart.yAxisTicks, [0, 2, 4, 6, 8]);
assert.equal(chart.yAxisMax, 8);
assert.ok(chart.yAxisTicks.every(Number.isInteger), "chart ticks are integers");
assert.deepEqual(
  buildStockAttentionChartLayout([
    { key: "out_of_stock", count: 22 },
    { key: "critical_0_3", count: 0 },
    { key: "shortage_4_7", count: 0 },
    { key: "planning_8_14", count: 0 },
  ]).yAxisTicks,
  [0, 5, 10, 15, 20, 25],
);
assert.ok(buildStockAttentionChartLayout([{ key: "out_of_stock", count: 150 }]).yAxisMax >= 150);
assert.equal(getInitialStockPageFilter(data), "critical");
assert.equal(getStockPageItems(data, "critical")[0]?.coverage_bucket, "critical_0_3", "stock filter uses coverage_bucket");
assert.equal(getStockPageItems(data, "no-sales")[0]?.no_sales_days, 14, "no-sales uses the separate factual dataset");

console.log("stock-page-view-model tests passed");
