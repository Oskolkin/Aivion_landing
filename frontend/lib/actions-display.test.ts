import assert from "node:assert/strict";
import {
  actionCategoryLabel,
  actionPriorityLabel,
  actionStatusLabel,
  actionsFilterLabel,
  actionTypeLabel,
  encodeActionKey,
  formatCurrency,
  formatEvidenceValue,
  formatMetricDate,
  formatMetricLabel,
  formatMetricValue,
} from "@/lib/actions-display";

function run() {
  assert.equal(actionPriorityLabel("critical"), "Критично");
  assert.equal(actionPriorityLabel("important"), "Важно");
  assert.equal(actionPriorityLabel("watch"), "Наблюдение");

  assert.equal(actionCategoryLabel("stock"), "Остатки и поставки");
  assert.equal(actionCategoryLabel("slow_turnover"), "Оборачиваемость");
  assert.equal(actionCategoryLabel("sku"), "SKU");
  assert.equal(actionCategoryLabel("sales"), "Продажи");
  assert.equal(actionCategoryLabel("advertising"), "Реклама");
  assert.equal(actionCategoryLabel("funnel"), "Воронка и карточки");
  assert.equal(actionCategoryLabel("category"), "Категории");
  assert.equal(actionCategoryLabel("data_quality"), "Качество данных");

  assert.equal(actionTypeLabel("stock_replenishment"), "Пополнение");
  assert.equal(actionTypeLabel("stock_redistribution"), "Перераспределение остатков");
  assert.equal(actionTypeLabel("slow_turnover_review"), "Проверка оборачиваемости");
  assert.equal(actionTypeLabel("no_sales_with_stock"), "Остаток без продаж");
  assert.equal(actionTypeLabel("sku_orders_drop"), "Падение заказов");
  assert.equal(actionTypeLabel("key_sku_risk"), "Ключевые SKU");
  assert.equal(actionTypeLabel("key_sku_protection"), "Защита ключевых SKU");
  assert.equal(actionTypeLabel("ad_high_drr"), "Высокий ДРР");
  assert.equal(actionTypeLabel("ad_high_drr_review"), "Высокий ДРР");
  assert.equal(actionTypeLabel("ad_spend_no_orders"), "Расход без заказов");
  assert.equal(actionTypeLabel("ad_budget_reallocation"), "Перераспределение бюджета");
  assert.equal(actionTypeLabel("funnel_weak_transition"), "Просадка воронки");
  assert.equal(actionTypeLabel("card_conversion_review"), "Конверсия карточки");
  assert.equal(actionTypeLabel("category_underperformance"), "Просадка категории");
  assert.equal(actionTypeLabel("sales_drop_review"), "Просадка продаж");
  assert.equal(actionTypeLabel("data_quality_issue"), "Качество данных");
  assert.equal(actionStatusLabel("open"), "Открыто");
  assert.equal(actionStatusLabel("checked"), "Выполнено");
  assert.equal(actionStatusLabel("dismissed"), "Неактуально");
  assert.equal(actionsFilterLabel("open"), "Все открытые");
  assert.equal(actionsFilterLabel("stock"), "Остатки и поставки");
  assert.equal(actionsFilterLabel("slow_turnover"), "Оборачиваемость");
  assert.equal(actionsFilterLabel("sku_sales"), "SKU и продажи");
  assert.equal(actionsFilterLabel("advertising"), "Реклама");
  assert.equal(actionsFilterLabel("funnel"), "Воронка и карточки");
  assert.equal(actionsFilterLabel("category"), "Категории");
  assert.equal(actionsFilterLabel("checked"), "Выполненные");
  assert.equal(actionsFilterLabel("dismissed"), "Неактуальные");

  assert.ok(formatCurrency(42000).includes("₽"), "currency format");
  assert.equal(formatMetricDate("2026-06-24"), "24.06.2026");

  assert.equal(
    formatEvidenceValue({ kind: "metric", label: "ДРР", value: "18", unit: "%" }),
    "18%",
  );
  assert.equal(
    formatEvidenceValue({ kind: "metric", label: "Выручка 7д", value: "42000", unit: "RUB" }),
    "42 000 ₽",
  );
  assert.equal(formatMetricLabel("orders/last_7d"), "Заказы за 7 дней");
  assert.equal(formatMetricValue("12", "count"), "12 шт.");
  assert.equal(formatMetricValue("42000", "RUB").includes("₽"), true);
  assert.equal(
    formatEvidenceValue({ kind: "metric", label: "Покрытие", value: "6.7", unit: "days" }),
    "6,7 дн.",
  );

  assert.equal(encodeActionKey("stock:sku.1"), "stock%3Asku.1");

  console.log("actions-display tests passed");
}

run();
