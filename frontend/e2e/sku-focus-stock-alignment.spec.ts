import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { loginAsDemoSeller, stubOzonConnectionReady } from "./helpers/auth";

/**
 * Acceptance: Stock Coverage counters ↔ SKU Focus stock filters,
 * no vertical metric scale, mini-charts have Y-axis.
 */
test.use({ storageState: { cookies: [], origins: [] } });

const OUT = path.resolve(__dirname, "../../backend/debug/sku-focus-stock-fix/screenshots");

async function readCoverageCounters(page: import("@playwright/test").Page) {
  const keys = ["out_of_stock", "critical_0_3", "shortage_4_7", "planning_8_14"] as const;
  const counts: Record<(typeof keys)[number], number> = {
    out_of_stock: 0,
    critical_0_3: 0,
    shortage_4_7: 0,
    planning_8_14: 0,
  };
  for (const key of keys) {
    const el = page.locator(`[data-testid="stock-coverage-bar-segment-${key}"]`).first();
    if ((await el.count()) === 0) continue;
    const raw = await el.getAttribute("data-segment-count");
    counts[key] = Number(raw ?? 0);
  }
  return counts;
}

async function openStockTab(page: import("@playwright/test").Page) {
  const section = page.getByTestId("sku-focus-section");
  await section.scrollIntoViewIfNeeded();
  await page.getByTestId("sku-focus-tab-stock_risk").click();
  await expect(page.getByTestId("sku-focus-stock-filter-chips")).toBeVisible();
}

async function selectStockFilter(
  page: import("@playwright/test").Page,
  filter: "all-risk" | "out-of-stock" | "critical" | "short" | "planning",
) {
  await page.getByTestId(`sku-focus-stock-filter-${filter}`).click();
  await expect(page.getByTestId("sku-focus-grid")).toHaveAttribute("data-stock-filter", filter);
}

async function renderedCount(page: import("@playwright/test").Page): Promise<number> {
  const grid = page.getByTestId("sku-focus-grid");
  if ((await grid.count()) === 0) return 0;
  const attr = await grid.getAttribute("data-rendered-count");
  return Number(attr ?? 0);
}

test("sku focus stock filters match coverage counters @live", async ({ page }) => {
  fs.mkdirSync(OUT, { recursive: true });
  await stubOzonConnectionReady(page);
  await loginAsDemoSeller(page);
  await page.setViewportSize({ width: 1440, height: 1100 });

  await page.goto("/app/dashboard");
  await expect(page.getByTestId("stock-coverage-module")).toBeVisible({ timeout: 60_000 });
  await expect(page.getByTestId("sku-focus-section")).toBeVisible({ timeout: 60_000 });

  const counters = await readCoverageCounters(page);
  const allRisk = counters.out_of_stock + counters.critical_0_3 + counters.shortage_4_7;

  const reconciliation = {
    stock_coverage: counters,
    all_risk: allRisk,
    checked_at: new Date().toISOString(),
  };
  fs.writeFileSync(
    path.join(OUT, "..", "reconciliation-report.json"),
    JSON.stringify(reconciliation, null, 2),
  );

  await openStockTab(page);

  // Out of stock
  await selectStockFilter(page, "out-of-stock");
  if (counters.out_of_stock > 0) {
    await expect(page.getByTestId("sku-focus-filter-empty")).toHaveCount(0);
    expect(await renderedCount(page)).toBe(counters.out_of_stock);
  }
  await expect(page.getByTestId("sku-focus-summary-banner")).toContainText(
    `${counters.out_of_stock} SKU уже отсутствуют`,
  );
  await page.screenshot({ path: path.join(OUT, "stocks-out-of-stock.png"), fullPage: false });

  // 0–3
  await selectStockFilter(page, "critical");
  if (counters.critical_0_3 > 0) {
    await expect(page.getByTestId("sku-focus-filter-empty")).toHaveCount(0);
    expect(await renderedCount(page)).toBe(counters.critical_0_3);
  }
  await expect(page.getByTestId("sku-focus-summary-banner")).toContainText(
    `${counters.critical_0_3} SKU имеют критический запас`,
  );
  await page.screenshot({ path: path.join(OUT, "stocks-0-3.png"), fullPage: false });

  // 4–7 — full list, not top-9
  await selectStockFilter(page, "short");
  if (counters.shortage_4_7 > 0) {
    await expect(page.getByTestId("sku-focus-filter-empty")).toHaveCount(0);
    expect(await renderedCount(page)).toBe(counters.shortage_4_7);
  }
  await expect(page.getByTestId("sku-focus-summary-banner")).toContainText(
    `${counters.shortage_4_7} SKU имеют короткий запас`,
  );
  if (counters.shortage_4_7 > 0) {
    await expect(page.getByTestId("sku-focus-card")).toHaveCount(counters.shortage_4_7);
  }
  await page.screenshot({ path: path.join(OUT, "stocks-4-7.png"), fullPage: false });

  // 8–14 — must not be empty when coverage counter > 0
  await selectStockFilter(page, "planning");
  expect(await renderedCount(page)).toBe(counters.planning_8_14);
  if (counters.planning_8_14 > 0) {
    await expect(page.getByTestId("sku-focus-filter-empty")).toHaveCount(0);
    await expect(page.getByTestId("sku-focus-card").first()).toBeVisible();
  }
  await expect(page.getByTestId("sku-focus-summary-banner")).toContainText(
    `${counters.planning_8_14} SKU имеют покрытие от восьми до четырнадцати дней`,
  );
  await page.screenshot({ path: path.join(OUT, "stocks-8-14.png"), fullPage: false });

  // CTA → all risk show all
  await page.getByTestId("stock-coverage-module").scrollIntoViewIfNeeded();
  const cta = page.getByTestId("stock-coverage-primary-cta");
  if ((await cta.count()) > 0 && allRisk > 0) {
    await cta.click();
    await expect(page.getByTestId("sku-focus-section")).toBeVisible();
    await expect(page.getByTestId("sku-focus-stock-filter-all-risk")).toHaveClass(/bg-slate-900/);
    await expect(page.getByTestId("sku-focus-grid")).toHaveAttribute("data-stock-filter", "all-risk");
    expect(await renderedCount(page)).toBe(allRisk);
    await expect(page.getByTestId("sku-focus-summary-banner")).toContainText(
      `${allRisk} SKU требуют внимания по остаткам`,
    );
  }
  await page.screenshot({ path: path.join(OUT, "stocks-all-risk.png"), fullPage: false });

  // Orders tab: no vertical scale, chart has Y-axis
  await page.getByTestId("sku-focus-tab-order_drop").click();
  await expect(page.locator('[data-testid="vertical-metric-scale"]')).toHaveCount(0);
  const orderCard = page.getByTestId("sku-focus-card").first();
  if (await orderCard.count()) {
    await expect(orderCard.locator('[data-testid="sku-focus-trend-y-tick"]').first()).toBeVisible();
  }
  await page.screenshot({ path: path.join(OUT, "orders.png"), fullPage: false });

  // Width smoke
  for (const width of [1920, 1440, 1366] as const) {
    await page.setViewportSize({ width, height: 1100 });
    await page.getByTestId("sku-focus-tab-stock_risk").click();
    await selectStockFilter(page, "planning");
    await page.screenshot({
      path: path.join(OUT, `stocks-8-14-${width}.png`),
      fullPage: false,
    });
  }

  fs.writeFileSync(
    path.join(OUT, "..", "reconciliation-report.json"),
    JSON.stringify(
      {
        ...reconciliation,
        sku_focus_rendered: {
          out_of_stock: counters.out_of_stock,
          critical_0_3: counters.critical_0_3,
          shortage_4_7: counters.shortage_4_7,
          planning_8_14: counters.planning_8_14,
          all_risk: allRisk,
        },
      },
      null,
      2,
    ),
  );
});
