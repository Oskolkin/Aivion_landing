import { isInappropriateImprovementSummary, replaceArtikulWithSKU, sanitizeSellerFacingText } from "@/lib/seller-facing-text";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function run() {
  assert(replaceArtikulWithSKU("6 артикулов") === "6 SKU", "articul plural replaced");
  assert(sanitizeSellerFacingText("  6   артикулов  ") === "6 SKU", "sanitize trims and replaces");

  assert(isInappropriateImprovementSummary("Заметных улучшений не выявлено."), "detects lack of improvements");
  assert(!isInappropriateImprovementSummary("Индекс вырастет после снижения рекламных расходов."), "allows valid summary");

  console.log("seller-facing-text.test.ts: ok");
}

run();
