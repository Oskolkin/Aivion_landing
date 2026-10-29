/**
 * One-off visual acceptance screenshots for UI polish task.
 * Run: npx playwright test e2e/ui-polish-screenshots.spec.ts --project=chromium
 */
import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { loginAsDemoSeller } from "./helpers/auth";

const outDir = path.resolve(__dirname, "../../backend/debug/ui-polish-screenshots");

test.use({ storageState: { cookies: [], origins: [] } });

async function shot(page: import("@playwright/test").Page, name: string) {
  fs.mkdirSync(outDir, { recursive: true });
  await page.screenshot({
    path: path.join(outDir, `${name}.png`),
    fullPage: false,
  });
}

test("ui polish screenshots @live", async ({ page }) => {
  await loginAsDemoSeller(page);

  // Desktop
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/app/dashboard");
  await expect(page.getByRole("heading", { name: /Пульс продавца/i })).toBeVisible({
    timeout: 60_000,
  });

  const period = page.getByTestId("seller-pulse-period-context");
  await expect(period).toBeVisible();
  await expect(period).toContainText("Период анализа");
  await expect(period).toContainText("Сравнение");
  await expect(period).toContainText("Данные обновлены");
  await expect(period).not.toContainText("Карточки");
  await expect(period).not.toContainText("AI-анализ");
  await period.scrollIntoViewIfNeeded();
  await shot(page, "period-header-desktop-1440");

  const revenue = page.getByTestId("stock-coverage-kpi-revenue");
  if (await revenue.count()) {
    await revenue.scrollIntoViewIfNeeded();
    await shot(page, "revenue-at-risk-desktop-1440");
  }

  const categorySection = page.getByRole("heading", { name: /Карточки состояния категорий/i });
  if (await categorySection.count()) {
    await categorySection.scrollIntoViewIfNeeded();
    await shot(page, "category-cards-desktop-1440");
  }

  // Tablet
  await page.setViewportSize({ width: 1024, height: 900 });
  await page.goto("/app/dashboard");
  await expect(period).toBeVisible({ timeout: 60_000 });
  await period.scrollIntoViewIfNeeded();
  await shot(page, "period-header-tablet-1024");
  if (await revenue.count()) {
    await revenue.scrollIntoViewIfNeeded();
    await shot(page, "revenue-at-risk-tablet-1024");
  }

  // Mobile
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/app/dashboard");
  await expect(period).toBeVisible({ timeout: 60_000 });
  await period.scrollIntoViewIfNeeded();
  await shot(page, "period-header-mobile-390");
  if (await revenue.count()) {
    await revenue.scrollIntoViewIfNeeded();
    await shot(page, "revenue-at-risk-mobile-390");
  }

  // Actions — no «Обязательное»
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/app/actions");
  await expect(page.getByTestId("action-card").first()).toBeVisible({ timeout: 60_000 });
  const cards = page.getByTestId("action-card");
  const count = await cards.count();
  expect(count).toBeGreaterThan(0);
  for (let i = 0; i < Math.min(count, 8); i += 1) {
    await expect(cards.nth(i)).not.toContainText("Обязательное");
  }
  await expect(page.getByTestId("action-required-badge")).toHaveCount(0);
  await cards.first().scrollIntoViewIfNeeded();
  await shot(page, "action-card-no-required-desktop-1440");

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/app/actions");
  await expect(page.getByTestId("action-card").first()).toBeVisible({ timeout: 60_000 });
  await expect(page.getByTestId("action-card").first()).not.toContainText("Обязательное");
  await shot(page, "action-card-no-required-mobile-390");
});
