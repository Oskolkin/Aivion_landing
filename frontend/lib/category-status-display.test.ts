import assert from "node:assert/strict";
import {
  formatCategoryConditionScore,
  formatCategoryHealthScore,
  getCategoryCTA,
  getCategoryConditionRiskBadge,
  getCategoryConditionScoreColor,
  getCategoryDisplayName,
  getCategorySectionText,
  getCategoryVisual,
  getSeverityBadgeConfig,
  splitTextToDisplayItems,
} from "./category-status-display";

function run() {
  assert.equal(getCategoryDisplayName(""), "Категория без названия");
  assert.equal(getCategoryDisplayName("  Текстиль  "), "Текстиль");

  assert.deepEqual(formatCategoryHealthScore(null), { value: "—", hasScore: false });
  assert.deepEqual(formatCategoryHealthScore(undefined), { value: "—", hasScore: false });
  assert.deepEqual(formatCategoryHealthScore(58), { value: "58", hasScore: true });
  assert.deepEqual(formatCategoryHealthScore(120), { value: "100", hasScore: true });

  assert.deepEqual(formatCategoryConditionScore(null, "high", 6), {
    value: 6,
    label: "6 / 10",
    ariaLabel: "Оценка состояния категории: 6 из 10",
    tone: "medium",
  });
  assert.deepEqual(formatCategoryConditionScore(null, "high", 7), {
    value: 7,
    label: "7 / 10",
    ariaLabel: "Оценка состояния категории: 7 из 10",
    tone: "low",
  });
  assert.deepEqual(formatCategoryConditionScore(null, null, 10), {
    value: 10,
    label: "10 / 10",
    ariaLabel: "Оценка состояния категории: 10 из 10",
    tone: "none",
  });
  assert.deepEqual(formatCategoryConditionScore(null, null, 9), {
    value: 9,
    label: "9 / 10",
    ariaLabel: "Оценка состояния категории: 9 из 10",
    tone: "low",
  });
  assert.deepEqual(formatCategoryConditionScore(null, null, 4), {
    value: 4,
    label: "4 / 10",
    ariaLabel: "Оценка состояния категории: 4 из 10",
    tone: "medium",
  });
  assert.deepEqual(formatCategoryConditionScore(null, "medium", 6), {
    value: 6,
    label: "6 / 10",
    ariaLabel: "Оценка состояния категории: 6 из 10",
    tone: "medium",
  });
  assert.deepEqual(formatCategoryConditionScore(null, null), {
    value: null,
    label: "— / 10",
    ariaLabel: "Оценка состояния категории недоступна",
    tone: "unknown",
  });
  assert.deepEqual(formatCategoryConditionScore(undefined, null), {
    value: null,
    label: "— / 10",
    ariaLabel: "Оценка состояния категории недоступна",
    tone: "unknown",
  });
  assert.deepEqual(formatCategoryConditionScore(Number.NaN, null), {
    value: null,
    label: "— / 10",
    ariaLabel: "Оценка состояния категории недоступна",
    tone: "unknown",
  });
  assert.deepEqual(formatCategoryConditionScore("", null), {
    value: null,
    label: "— / 10",
    ariaLabel: "Оценка состояния категории недоступна",
    tone: "unknown",
  });
  assert.equal(getCategoryConditionScoreColor("high"), "text-[#EA580C]");
  assert.equal(getCategoryConditionScoreColor("medium"), "text-[#D97706]");
  assert.equal(getCategoryConditionScoreColor("low"), "text-[#16A34A]");
  assert.equal(getCategoryConditionScoreColor("none"), "text-[#15803D]");
  assert.equal(getCategoryConditionScoreColor("unknown"), "text-[#64748B]");

  assert.equal(getCategoryConditionRiskBadge(null, "high", 6).label, "Средний риск");
  assert.equal(getCategoryConditionRiskBadge(null, "high", 7).label, "Небольшой риск");
  assert.equal(getCategoryConditionRiskBadge(null, null, 10).label, "Нет значимых рисков");
  assert.equal(getCategoryConditionRiskBadge(null, null, 9).label, "Небольшой риск");
  assert.equal(getCategoryConditionRiskBadge(null, null, 4).label, "Средний риск");
  assert.equal(getCategoryConditionRiskBadge(null, null, 0).label, "Высокий риск");
  assert.equal(getCategoryConditionRiskBadge(null, null).label, "Без оценки");
  assert.equal(getCategoryConditionRiskBadge(undefined, null).label, "Без оценки");
  assert.equal(getCategoryConditionRiskBadge(Number.NaN, null).label, "Без оценки");
  assert.equal(getCategoryConditionRiskBadge(null, "high").label, "Высокий риск");
  assert.equal(getCategoryConditionRiskBadge(null, "medium").label, "Средний риск");
  assert.equal(getCategoryConditionRiskBadge(null, "low").label, "Небольшой риск");
  assert.equal(getCategoryConditionRiskBadge(null, "stable").label, "Нет значимых рисков");

  assert.equal(getSeverityBadgeConfig("high").label, "Высокий риск");
  assert.equal(getSeverityBadgeConfig("medium").label, "Средний риск");
  assert.equal(getSeverityBadgeConfig("low").label, "Стабильно");
  assert.equal(getSeverityBadgeConfig("stable").label, "Стабильно");
  assert.equal(getSeverityBadgeConfig("").label, "Без оценки");
  assert.equal(getSeverityBadgeConfig(undefined).label, "Без оценки");

  const textileVisual = getCategoryVisual("Текстиль для дома");
  assert.match(textileVisual.badgeClassName, /F3E8FF/);
  assert.match(textileVisual.iconClassName, /7C3AED/);

  const fallbackVisual = getCategoryVisual("Неизвестная категория XYZ");
  assert.match(fallbackVisual.badgeClassName, /F1F5F9/);
  assert.match(fallbackVisual.iconClassName, /475569/);

  assert.deepEqual(
    splitTextToDisplayItems("Первый риск; Второй риск; Третий риск", { maxItems: 3 }),
    ["Первый риск", "Второй риск", "Третий риск"],
  );

  assert.deepEqual(splitTextToDisplayItems("Первый риск; Второй риск; Третий риск"), [
    "Первый риск",
    "Второй риск",
  ]);

  assert.deepEqual(
    splitTextToDisplayItems("Проверьте остатки. Затем обновите цены. После этого запустите рекламу.", {
      mode: "action",
      maxItems: 3,
    }),
    ["Проверьте остатки", "Обновите цены", "Запустите рекламу"],
  );

  assert.deepEqual(
    splitTextToDisplayItems(
      "В первую очередь пополните остатки по товарам-лидерам с покрытием до 3 дн. и отдельно проверьте 5 товаров без продаж, по которым заморожено 671 шт.",
      { mode: "action", maxItems: 2 },
    ),
    [
      "Пополните остатки по товарам-лидерам с покрытием до 3 дн",
      "Проверьте 5 товаров без продаж, по которым заморожено 671 шт",
    ],
  );

  assert.deepEqual(
    splitTextToDisplayItems("Один длинный риск без явных разделителей, который должен остаться цельным"),
    ["Один длинный риск без явных разделителей, который должен остаться цельным"],
  );

  assert.deepEqual(
    splitTextToDisplayItems("Риск A, риск B, риск C"),
    ["Риск A, риск B, риск C"],
  );

  const riskSection = getCategorySectionText(
    "key_risk",
    "Первый риск; Второй риск; Третий риск",
  );
  assert.equal(riskSection.items.length, 2, "risk section defaults to two items");

  const actionSection = getCategorySectionText(
    "recommended_action",
    "Проверьте остатки. Затем обновите цены. После этого запустите рекламу.",
  );
  assert.equal(actionSection.items.length, 2, "action section defaults to two items");

  const emptyHeadline = getCategorySectionText("headline", "");
  assert.equal(emptyHeadline.items[0], "Недостаточно данных для краткого вывода.");
  assert.equal(emptyHeadline.isFallback, true);

  const legacyHeadline = getCategorySectionText("headline", "Хранение на кухне · 35% выручки", {
    categoryName: "Хранение на кухне",
  });
  assert.equal(legacyHeadline.items[0], "Формирует 35% выручки");
  assert.equal(legacyHeadline.isFallback, false);

  const alreadyClean = getCategorySectionText("headline", "Формирует 23% выручки", {
    categoryName: "Организация пространства",
  });
  assert.equal(alreadyClean.items[0], "Формирует 23% выручки");

  const customHeadline = getCategorySectionText(
    "headline",
    "Категория держится на ограниченном числе SKU с устойчивым спросом.",
    { categoryName: "Ванная комната" },
  );
  assert.equal(
    customHeadline.items[0],
    "Категория держится на ограниченном числе SKU с устойчивым спросом.",
  );

  const emptyRisk = getCategorySectionText("key_risk", null);
  assert.equal(emptyRisk.items[0], "Явные риски не выделены.");

  const emptyAction = getCategorySectionText("recommended_action", undefined);
  assert.equal(emptyAction.items[0], "Действия не сформированы.");

  assert.equal(getCategoryCTA({}), null);
  assert.equal(getCategoryCTA({ category_id: "cat-42" }), null);

  console.log("category-status-display.test.ts: ok");
}

run();
