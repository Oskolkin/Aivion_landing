import {
  isActiveAlertGroup,
  isActiveAlertType,
  isLegacyAlertGroup,
  isLegacyAlertType,
  isSellerFacingAlertGroup,
  isSellerHiddenAlertGroup,
} from "@/lib/alert-labels";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function run() {
  assert(isLegacyAlertGroup("price_economics"), "price_economics is legacy group");
  assert(!isActiveAlertGroup("price_economics"), "price_economics is not active group");
  assert(!isSellerFacingAlertGroup("price_economics"), "price_economics hidden from seller filters");
  assert(isSellerHiddenAlertGroup("price_economics"), "price_economics seller-hidden");

  assert(isActiveAlertType("ad_weak_campaign_efficiency"), "campaign ad alert active");
  assert(isActiveAlertType("fbo_surplus_storage_risk"), "fbo surplus active");
  assert(isLegacyAlertType("ad_spend_without_result"), "sku-level ad legacy");
  assert(isLegacyAlertType("ad_budget_on_weak_sku"), "ad_budget_on_weak_sku legacy");
  assert(isLegacyAlertType("price_below_min_constraint"), "price alert legacy");

  assert(isActiveAlertGroup("advertising"), "advertising active");
  assert(!isLegacyAlertType("stock_oos_risk"), "stock_oos_risk not legacy");

  console.log("alert-labels.test.ts: ok");
}

run();
