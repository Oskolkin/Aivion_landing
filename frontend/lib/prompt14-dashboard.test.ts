import assert from "node:assert/strict";
import {
  CANONICAL_ACTION_TYPES,
  ACTION_PRESENTATION_REGISTRY,
  formatActionParameters,
  presentationOrFallback,
} from "@/lib/action-presentation-registry";
import { buildOverviewViewModel } from "@/lib/overview-view-model";
import type { PublicationDashboardResponse } from "@/lib/publication-dashboard";

function run() {
  assert.equal(CANONICAL_ACTION_TYPES.length, 19);
  assert.equal(Object.keys(ACTION_PRESENTATION_REGISTRY).length, 19);
  for (const key of CANONICAL_ACTION_TYPES) {
    assert.equal(ACTION_PRESENTATION_REGISTRY[key].action_type, key);
    assert.equal(ACTION_PRESENTATION_REGISTRY[key].template, key);
    assert.ok(ACTION_PRESENTATION_REGISTRY[key].family);
    assert.ok(ACTION_PRESENTATION_REGISTRY[key].icon);
    assert.ok(ACTION_PRESENTATION_REGISTRY[key].columns.length >= 1);
    assert.ok(ACTION_PRESENTATION_REGISTRY[key].parameter_formatter);
    assert.equal(ACTION_PRESENTATION_REGISTRY[key].presentation_template_key, key);
  }

  assert.equal(formatActionParameters("SKU_ADJUST_PRICE", { current_price: "100", recommended_price: "90" })[0].label, "Текущая цена");
  assert.equal(formatActionParameters("SKU_INCREASE_REPLENISHMENT", { recommended_units: "12" }).some((r) => r.label === "Пополнение"), true);
  assert.equal(formatActionParameters("SKU_JOIN_PROMOTION", { promotion: "promo-1" })[0].value, "promo-1");
  assert.equal(formatActionParameters("CAMPAIGN_ADJUST_BUDGET", { recommended_budget: "5000" }).some((r) => r.label === "Рекомендуемый бюджет"), true);
  assert.equal(formatActionParameters("CAMPAIGN_ADJUST_PRODUCT_SET", { add_skus: ["1"] })[0].value, "1");
  assert.equal(formatActionParameters("CATEGORY_REPLENISHMENT_PLAN", { category: "cat" })[0].value, "cat");
  assert.equal(formatActionParameters("ACCOUNT_RATING_REMEDIATION", { rating: "4.2" })[0].value, "4.2");
  const unknown = presentationOrFallback("NOT_A_TYPE");
  assert.equal("fallback" in unknown && unknown.fallback === true, true);

  const data: PublicationDashboardResponse = {
    state: "READY",
    publication: {
      id: "12",
      analysis_date: "2026-08-31",
      analysis_cutoff_at: "2026-08-31T21:00:00Z",
      published_at: "2026-08-31T22:00:00Z",
    },
    overview: {
      schema_version: "overview_projection_v2",
      seller_account_id: 1,
      analysis_date: "2026-08-31",
      analysis_cutoff_at: "2026-08-31T21:00:00Z",
      kpis: [
        { key: "ORDERED_AMOUNT", status: "READY", amount: "1000", unit: "RUB" },
        { key: "ORDERED_UNITS", status: "READY", amount: "10", unit: "UNITS" },
        { key: "AD_SHARE", status: "UNAVAILABLE", reason_code: "NO_DENOMINATOR" },
      ],
      executive_summary: "Сводка",
      executive_headline: "Заголовок",
      takeaways: [{ statement: "Точка", primary_domain: "STOCK" }],
      findings: [
        { title: "Риск", statement: "Текст", primary_domain: "STOCK", overview_bucket: "ACTIVE_RISK" },
      ],
    },
    funnel: { schema_version: "funnel_projection_v2", state: "READY", status: "READY", period: { current: { key: "current_7d" }, previous: { key: "previous_7d" } }, stages: [], transitions: [] },
    stock: { schema_version: "stock_projection_v2", coverage_bands: [{ key: "out_of_stock", count: 1, state: "KNOWN" }] },
    advertising: { schema_version: "advertising_projection_v2", account_ad_spend_share: { key: "account_ad_spend_share", status: "MISSING" } },
    focus: [],
  };
  const vm = buildOverviewViewModel(data);
  assert.ok(vm);
  assert.equal(vm!.kpis.length, 3);
  assert.equal(vm!.kpis[0].label, "Заказано на сумму");
  assert.equal(vm!.kpis[1].label, "Заказано товаров");
  assert.equal(vm!.kpis[2].valueDisplay, "—");
  const raw = JSON.stringify(data);
  assert.equal(raw.includes("average_order_value"), false);
  assert.equal(raw.includes("analysis_source"), false);
  assert.equal(raw.includes("fallback_used"), false);
  console.log("prompt14 frontend contract tests passed");
}

run();
