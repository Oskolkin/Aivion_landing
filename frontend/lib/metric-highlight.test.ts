import { splitMetricHighlightText } from "@/lib/metric-highlight";

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function boldTexts(text: string): string[] {
  return splitMetricHighlightText(text)
    .filter((segment) => segment.bold)
    .map((segment) => segment.text);
}

function assertHighlighted(text: string, expected: string, message: string) {
  const bold = boldTexts(text);
  assert(
    bold.some((value) => value.includes(expected) || expected.includes(value)),
    `${message}: expected "${expected}" in [${bold.join(", ")}]`,
  );
}

function run() {
  assertHighlighted("Выручка снизилась до 101 721 ₽, на 20 845 ₽ ниже.", "101 721 ₽", "money with spaces");
  assertHighlighted("Расход 101721 ₽ без отдачи.", "101721 ₽", "money without spaces");
  assertHighlighted("Сумма 32 790,36 ₽.", "32 790,36 ₽", "money with decimals");

  assertHighlighted("Доля отмен 13,6%.", "13,6%", "percent comma");
  assertHighlighted("Рост +1,1% за день.", "+1,1%", "signed percent");
  assertHighlighted("Падение -17,0%.", "-17,0%", "negative percent");
  assertHighlighted("Пик 257,9%.", "257,9%", "large percent");

  assertHighlighted("Сдвиг на 4,81 п.п.", "4,81 п.п.", "percentage points");
  assertHighlighted("Изменение +9,64 п.п.", "+9,64 п.п.", "signed percentage points");
  assertHighlighted("Просадка -1,37 п.п.", "-1,37 п.п.", "negative percentage points");

  assertHighlighted("6 SKU с коротким покрытием.", "6 SKU", "sku count");
  assertHighlighted("Осталось 11 SKU.", "11 SKU", "sku count alt");

  assertHighlighted("Было 76 заказов.", "76 заказов", "orders plural");
  assertHighlighted("Только 1 заказ.", "1 заказ", "orders singular");
  assertHighlighted("При 33 заказа.", "33 заказа", "orders genitive");

  assertHighlighted("Зафиксировано 12 отмен.", "12 отмен", "cancellations plural");
  assertHighlighted("Есть 1 отмена.", "1 отмена", "cancellation singular");
  assertHighlighted("Выросло до 4 отмены.", "4 отмены", "cancellations genitive");

  assertHighlighted("Открыто 20 рисков.", "20 рисков", "risks");
  assertHighlighted("Среди них 2 критичных.", "2 критичных", "critical risks");
  assertHighlighted("Ещё 12 высоких.", "12 высоких", "high risks");
  assertHighlighted("5 коротких остатков.", "5 коротких", "short coverage");

  assertHighlighted("По 6 рекламных кампаний.", "6 рекламных кампаний", "ad campaigns");
  assertHighlighted("Активна 1 кампания.", "1 кампания", "single campaign");
  assertHighlighted("Разбор по 6 кампаниям.", "6 кампаниям", "campaigns dative");

  assertHighlighted("Покрытие 3,5 дня.", "3,5 дня", "coverage days");
  assertHighlighted("Осталось 7 дней.", "7 дней", "days plural");
  assertHighlighted("Запас 3 дня.", "3 дня", "days few");

  const cancellationLine =
    "На 21.05.2026 отмены составили 13,6% от заказов: 12 отмен при 76 заказах.";
  assertHighlighted(cancellationLine, "13,6%", "cancellation line percent");
  assertHighlighted(cancellationLine, "12 отмен", "cancellation line count");
  assertHighlighted(cancellationLine, "76 заказах", "cancellation line orders");

  const segments = splitMetricHighlightText(cancellationLine);
  assert(segments.some((segment) => !segment.bold), "cancellation line keeps plain text");
  assert(segments.filter((segment) => segment.bold).length >= 3, "cancellation line highlights multiple metrics");

  console.log("metric-highlight.test.ts: ok");
}

run();
