import assert from "node:assert/strict";
import { counterForFilter, getFilterRequestParams, normalizeFilterKey } from "@/lib/actions-filters";

function run() {
  assert.equal(normalizeFilterKey("open"), "open");
  assert.equal(normalizeFilterKey("slow_turnover"), "slow_turnover");
  assert.equal(normalizeFilterKey("dismissed"), "dismissed");
  assert.equal(normalizeFilterKey("invalid"), "open");
  assert.equal(normalizeFilterKey("critical"), "open");
  assert.equal(normalizeFilterKey("today"), "open");
  assert.equal(normalizeFilterKey("data_quality"), "open");
  assert.equal(normalizeFilterKey("snoozed"), "open");

  assert.deepEqual(getFilterRequestParams("open"), [{ status: "open", metric_date: "latest", limit: 50, offset: 0 }]);
  assert.deepEqual(getFilterRequestParams("slow_turnover"), [
    { status: "open", category: "slow_turnover", metric_date: "latest", limit: 50, offset: 0 },
  ]);
  assert.deepEqual(getFilterRequestParams("category"), [
    { status: "open", category: "category", metric_date: "latest", limit: 50, offset: 0 },
  ]);
  assert.deepEqual(getFilterRequestParams("stock"), [
    { status: "open", category: "stock", metric_date: "latest", limit: 50, offset: 0 },
  ]);
  assert.deepEqual(getFilterRequestParams("advertising"), [
    { status: "open", category: "advertising", metric_date: "latest", limit: 50, offset: 0 },
  ]);
  assert.deepEqual(getFilterRequestParams("funnel"), [
    { status: "open", category: "funnel", metric_date: "latest", limit: 50, offset: 0 },
  ]);
  assert.deepEqual(getFilterRequestParams("checked"), [{ status: "checked", metric_date: "latest", limit: 50, offset: 0 }]);
  assert.deepEqual(getFilterRequestParams("dismissed"), [{ status: "dismissed", metric_date: "latest", limit: 50, offset: 0 }]);
  assert.deepEqual(getFilterRequestParams("sku_sales"), [
    { status: "open", category: "sku", metric_date: "latest", limit: 50, offset: 0 },
    { status: "open", category: "sales", metric_date: "latest", limit: 50, offset: 0 },
  ]);

  const counters = {
    total_open: 10,
    stock: 2,
    slow_turnover: 4,
    sku: 4,
    advertising: 1,
    funnel: 2,
    sales: 5,
    category_count: 3,
    checked: 6,
    dismissed: 2,
  };
  assert.equal(counterForFilter("open", counters), 10);
  assert.equal(counterForFilter("stock", counters), 2);
  assert.equal(counterForFilter("slow_turnover", counters), 4);
  assert.equal(counterForFilter("advertising", counters), 1);
  assert.equal(counterForFilter("funnel", counters), 2);
  assert.equal(counterForFilter("category", counters), 3);
  assert.equal(counterForFilter("checked", counters), 6);
  assert.equal(counterForFilter("dismissed", counters), 2);
  assert.equal(counterForFilter("sku_sales", counters), 9);

  console.log("actions-filters tests passed");
}

run();
