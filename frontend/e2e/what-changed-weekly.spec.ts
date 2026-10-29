import { test, expect, type Page, type Locator } from "@playwright/test";
import path from "node:path";
import fs from "node:fs";
import { stubOzonConnectionReady } from "./helpers/auth";

/**
 * Visual + structural checks for Dashboard «Что изменилось за неделю».
 * Contract: desktop/tablet must stay 2×2 equal-height groups (never 4 columns).
 */
const FIXTURE = {
  status: "available",
  summary:
    "Продажи продолжают расти, но структура рисков ухудшилась.\n\nЗа неделю выручка выросла на 12,1%, а заказы — на 9,1%. Одновременно число SKU без остатка увеличилось до четырёх, а три кампании превысили ДРР 80%.\n\nНовым риском стала кампания без заказов. Главный приоритет — восстановить наличие и сократить неэффективные расходы.",
  main_conclusion:
    "Продажи продолжают расти, но структура рисков ухудшилась.\n\nЗа неделю выручка выросла на 12,1%, а заказы — на 9,1%. Одновременно число SKU без остатка увеличилось до четырёх, а три кампании превысили ДРР 80%.\n\nНовым риском стала кампания без заказов. Главный приоритет — восстановить наличие и сократить неэффективные расходы.",
  groups: [
    {
      key: "worsened",
      label: "Ухудшилось",
      tone: "negative",
      items: [
        {
          label: "Без остатка осталось 4 SKU",
          text: "Количество товаров с нулевым остатком выросло с 2 до 4. На эти позиции приходилось 35 000 ₽ выручки за последние 7 дней.",
          current_value_text: "4 SKU",
          previous_value_text: "2 SKU",
          delta_text: "2 → 4",
          impact_text: "На эти позиции приходилось 35 000 ₽ выручки за последние 7 дней.",
          entity_count: 4,
          entity_names: ["Товар А", "Товар Б"],
          severity: "high",
          linked_action_id: "action_stock_1",
          action_link_label: "Посмотреть 4 SKU",
          change_id: "worsened:stock.stockout_sku_count",
        },
        {
          label: "Три кампании превысили ДРР 80%",
          text: "Количество проблемных кампаний выросло с 1 до 3. На них пришлось 318 000 ₽ расходов за неделю. Основной вклад дали кампании «Альфа» и «Бета».",
          current_value_text: "3 кампании",
          previous_value_text: "1 кампания",
          impact_text: "На них пришлось 318 000 ₽ расходов за неделю.",
          entity_names: ["Альфа", "Бета"],
          entity_count: 3,
          severity: "critical",
          linked_action_id: "action_ad_1",
          action_link_label: "Проверить кампании",
          change_id: "worsened:advertising.high_drr_campaign_count",
        },
      ],
    },
    {
      key: "new_risks",
      label: "Новые риски",
      tone: "warning",
      items: [
        {
          label: "Новая кампания потратила 21 700 ₽ без заказов",
          text: "Кампания «Новая» израсходовала 21 700 ₽ за неделю и не получила ни одного заказа. В предыдущем периоде такой проблемы не было.",
          current_value_text: "21 700 ₽",
          impact_text: "Суммарный расход без заказов — 21 700 ₽.",
          entity_names: ["Новая"],
          entity_count: 1,
          linked_action_id: "action_ad_2",
          action_link_label: "Проверить кампанию",
          change_id: "new_risk:advertising.spend_without_orders",
        },
      ],
    },
    {
      key: "unchanged_risks",
      label: "Активные проблемы",
      tone: "neutral",
      items: [
        {
          label: "2 SKU без продаж при наличии остатка",
          text: "Товары с остатком, но без заказов за неделю, сохраняются вторую неделю. Замороженные позиции продолжают занимать складской ресурс.",
          current_value_text: "2 SKU",
          previous_value_text: "2 SKU",
          entity_names: ["Тихий товар"],
          entity_count: 2,
          linked_action_id: "action_nosales_1",
          action_link_label: "Посмотреть 2 SKU",
          change_id: "active_problem:no_sales.with_stock_count",
        },
        {
          label: "Дефицит по ключевым SKU сохраняется",
          text: "Ключевые товары с коротким запасом остаются в зоне риска относительно предыдущего периода.",
          current_value_text: "3 SKU",
          previous_value_text: "3 SKU",
          severity: "high",
          linked_action_id: "action_stock_2",
          action_link_label: "Открыть действие",
          change_id: "active_problem:stock.critical_sku_count",
        },
      ],
    },
    {
      key: "improved",
      label: "Улучшилось",
      tone: "positive",
      items: [
        {
          label: "Выручка выросла на 12,1%",
          text: "За неделю выручка составила 3 359 790 ₽ против предыдущего периода.",
          current_value_text: "3 359 790 ₽",
          previous_value_text: "2 996 000 ₽",
          delta_text: "+12,1%",
          change_id: "improved:sales.revenue_7d",
        },
        {
          label: "Заказы выросли на 9,1%",
          text: "Количество заказов выросло до 1 362.",
          current_value_text: "1 362",
          previous_value_text: "1 249",
          delta_text: "+9,1%",
          change_id: "improved:sales.orders_7d",
        },
        {
          label: "Рекламная нагрузка кабинета стабильна",
          text: "Доля рекламных расходов к выручке осталась около 28%.",
          current_value_text: "28%",
          previous_value_text: "27,5%",
          change_id: "improved:advertising.ad_share_of_revenue_pct",
        },
        {
          label: "Дополнительное улучшение воронки",
          text: "Конверсия в заказ выросла на 1,2 п.п.",
          current_value_text: "41,1%",
          previous_value_text: "39,9%",
          change_id: "improved:funnel.cart_to_order_conversion",
        },
      ],
    },
  ],
};

async function box(locator: Locator) {
  const box = await locator.boundingBox();
  if (!box) throw new Error("missing bounding box");
  return box;
}

async function assertTwoByTwo(page: Page, width: number) {
  const grid = page.getByTestId("what-changed-groups-grid");
  await expect(grid).toBeVisible();

  const worsened = page.getByTestId("what-changed-group-worsened");
  const newRisks = page.getByTestId("what-changed-group-new_risks");
  const active = page.getByTestId("what-changed-group-unchanged_risks");
  const improved = page.getByTestId("what-changed-group-improved");

  const [w, n, a, i] = await Promise.all([box(worsened), box(newRisks), box(active), box(improved)]);

  // Positions: top-left / top-right / bottom-left / bottom-right
  expect(w.x).toBeLessThan(n.x);
  expect(a.x).toBeLessThan(i.x);
  expect(w.y).toBeLessThan(a.y);
  expect(n.y).toBeLessThan(i.y);

  // Equal widths (±5%)
  expect(Math.abs(w.width - n.width) / w.width).toBeLessThan(0.05);
  expect(Math.abs(a.width - i.width) / a.width).toBeLessThan(0.05);

  // Equal heights within a row (±5%) — large equal-height sections
  expect(Math.abs(w.height - n.height) / Math.max(w.height, 1)).toBeLessThan(0.08);
  expect(Math.abs(a.height - i.height) / Math.max(a.height, 1)).toBeLessThan(0.08);

  // Never a 4-column row: top and bottom pairs must share roughly same y
  expect(Math.abs(w.y - n.y)).toBeLessThan(8);
  expect(Math.abs(a.y - i.y)).toBeLessThan(8);

  // Groups are wide enough for readable copy (not narrow 4-col cards).
  // Compare against the card (what-changed sits in ~66% of diagnostics row),
  // not the full viewport — 4-col would be ~15–20% of card width.
  const cardBox = await box(page.getByTestId("what-changed-card"));
  expect(w.width).toBeGreaterThan(cardBox.width * 0.38);

  // No clipping of active-problems copy
  await expect(active).toContainText("2 SKU без продаж");
  await expect(active).not.toHaveCSS("overflow", "hidden");
}

test.describe("what-changed weekly block 2x2", () => {
  test.beforeEach(async ({ page }) => {
    await stubOzonConnectionReady(page);

    await page.route("**/api/v1/dashboard/home**", async (route) => {
      const json = {
        seller_pulse: { status: "available", kpis: [], trend_7d: [] },
        ai_diagnostics: {
          account_summary: {
            status: "available",
            health_score: 72,
            severity: "medium",
            headline: "Кабинет",
            executive_summary: "Тест",
            risk_drivers: [],
          },
          what_changed: FIXTURE,
          top_actions: [
            {
              id: "action_stock_1",
              action_key: "action_stock_1",
              source: "actions_final",
              priority: "high",
              priority_label: "Высокий",
              title: "Пополнить остатки",
              summary: "Пополнить",
            },
          ],
          stock_coverage: { status: "available", segments: [] },
          ad_efficiency: { status: "available" },
          sales_funnel: { status: "available", current_completed_date: "", previous_completed_date: "", stages: [], daily_series: [] },
          funnel_analysis: { status: "available" },
          risk_distribution: { status: "available", groups: [] },
          category_summaries: [],
        },
        sku_focus: {
          status: "available",
          order_drop: [],
          stock_risk: [],
          no_sales_with_stock: [],
          slow_turnover: [],
          key_skus: [],
        },
        period_context: {},
      };
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(json) });
    });
  });

  for (const width of [1920, 1440, 1366] as const) {
    test(`2x2 layout @${width}`, async ({ page }) => {
      await page.setViewportSize({ width, height: 1400 });
      await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });

      const card = page.getByTestId("what-changed-card");
      await expect(card).toBeVisible({ timeout: 30000 });
      await expect(page.getByTestId("what-changed-main-conclusion")).toBeVisible();

      await assertTwoByTwo(page, width);

      const gridClass = await page.getByTestId("what-changed-groups-grid").getAttribute("class");
      expect(gridClass ?? "").toContain("sm:grid-cols-2");
      expect(gridClass ?? "").not.toContain("grid-cols-4");
      expect(gridClass ?? "").not.toContain("xl:grid-cols-4");

      const bodyText = await page.locator("body").innerText();
      expect(bodyText).not.toMatch(/\b882\d{9}\b/);
      expect(bodyText).toMatch(/Было:.*Стало:/);
      expect(bodyText).toContain("Посмотреть 4 SKU");
      expect(bodyText).not.toContain("Нет долговременных проблем");

      // Groups use equal-height stretch (h-full), not content-height-only
      const worsenedClass = await page.getByTestId("what-changed-group-worsened").getAttribute("class");
      expect(worsenedClass ?? "").toContain("h-full");
      expect(worsenedClass ?? "").not.toMatch(/\bh-auto\b/);

      const outDir = path.join(process.cwd(), "..", "backend", "debug", "what-changed-screenshots");
      fs.mkdirSync(outDir, { recursive: true });
      const boxes = {
        width,
        card: await box(card),
        worsened: await box(page.getByTestId("what-changed-group-worsened")),
        new_risks: await box(page.getByTestId("what-changed-group-new_risks")),
        active: await box(page.getByTestId("what-changed-group-unchanged_risks")),
        improved: await box(page.getByTestId("what-changed-group-improved")),
      };
      fs.writeFileSync(path.join(outDir, `bounding-boxes-${width}.json`), JSON.stringify(boxes, null, 2));
      await card.screenshot({ path: path.join(outDir, `what-changed-2x2-${width}.png`) });
    });
  }

  test("show more expands improved group", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1400 });
    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
    await expect(page.getByText("Показать ещё 1")).toBeVisible({ timeout: 30000 });
    await page.getByText("Показать ещё 1").click();
    await expect(page.getByText("Дополнительное улучшение воронки")).toBeVisible();
  });

  test("action link opens actions detail", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1400 });
    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
    const link = page.getByRole("link", { name: "Посмотреть 4 SKU" });
    await expect(link).toBeVisible({ timeout: 30000 });
    await expect(link).toHaveAttribute("href", /action_stock_1/);
  });
});
