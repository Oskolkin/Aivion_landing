import { isLegacyRecommendationType, recommendationTypeLabel } from "@/lib/recommendation-labels";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function run() {
  assert(isLegacyRecommendationType("review_price_margin"), "legacy price rec");
  assert(!isLegacyRecommendationType("replenish_sku"), "active rec");
  assert(recommendationTypeLabel("review_price_margin") === "Устаревший тип", "legacy label");
  assert(recommendationTypeLabel("replenish_sku") === "Пополнить SKU", "active label");
  console.log("recommendation-labels.test.ts: ok");
}

run();
