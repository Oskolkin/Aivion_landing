import { expect, test } from "@playwright/test";
import { stubOzonConnectionReady } from "./helpers/auth";

test.beforeEach(async ({ page }) => {
  await stubOzonConnectionReady(page);
});

test.describe("Dashboard Overview redesign", () => {
  test("desktop overview composition and nested routes", async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error") consoleErrors.push(msg.text());
    });

    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: "Дашборд", level: 1 })).toBeVisible({
      timeout: 45_000,
    });
    await expect(page.getByTestId("dashboard-tabs")).toBeVisible();
    await expect(page.getByTestId("dashboard-tab-overview")).toHaveAttribute("aria-current", "page");

    await expect(page.getByTestId("overview-kpi-grid")).toBeVisible({ timeout: 45_000 });
    await expect(page.getByTestId("overview-ai-summary")).toBeVisible();
    await expect(page.getByTestId("overview-focus-actions")).toBeVisible();
    await expect(page.getByTestId("overview-what-changed")).toBeVisible();
    await expect(page.getByTestId("overview-domain-previews")).toBeVisible();

    await expect(page.getByTestId("app-shell-section-title")).toHaveCount(0);
    await expect(page.getByText("Смотреть детали в рекомендациях")).toHaveCount(0);
    await expect(page.getByText("Подробнее об индексе")).toHaveCount(0);
    await expect(page.getByText("Ключевые факторы снижения")).toBeVisible();
    await expect(page.getByTestId("overview-funnel-preview")).toContainText("CTR по кабинету");
    await expect(page.getByTestId("overview-ad-yesterday-drr")).toContainText("ДРР вчера");

    // Legacy full sections must not appear on Overview
    await expect(page.getByRole("heading", { name: "Пульс продавца" })).toHaveCount(0);
    await expect(page.getByRole("heading", { name: "AI-диагностика продаж" })).toHaveCount(0);
    await expect(page.getByRole("heading", { name: "Спросите ИИ о магазине" })).toHaveCount(0);
    await expect(page.getByTestId("dashboard-overview")).not.toContainText("SKU-фокус");

    // Nested navigation
    await page.getByTestId("dashboard-tab-funnel").click();
    await page.waitForURL(/\/app\/dashboard\/funnel/);
    await expect(page.getByTestId("dashboard-funnel-page")).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId("dashboard-tab-funnel")).toHaveAttribute("aria-current", "page");

    await page.getByTestId("dashboard-tab-stock").click();
    await page.waitForURL(/\/app\/dashboard\/stock/);
    await expect(page.getByTestId("dashboard-stock-page")).toBeVisible({ timeout: 30_000 });

    await page.getByTestId("dashboard-tab-advertising").click();
    await page.waitForURL(/\/app\/dashboard\/advertising/);
    await expect(page.getByTestId("dashboard-advertising-page")).toBeVisible({ timeout: 30_000 });

    await page.getByTestId("dashboard-tab-categories").click();
    await page.waitForURL(/\/app\/dashboard\/categories/);
    await expect(page.getByTestId("dashboard-categories-page")).toBeVisible({ timeout: 30_000 });

    await page.getByTestId("dashboard-tab-sku").click();
    await page.waitForURL(/\/app\/dashboard\/sku/);
    await expect(page.getByTestId("dashboard-sku-page")).toBeVisible({ timeout: 30_000 });

    await page.goBack();
    await page.waitForURL(/\/app\/dashboard\/categories/);

    // Direct URL
    await page.goto("/app/dashboard/funnel", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("dashboard-funnel-page")).toBeVisible({ timeout: 30_000 });

    // Sidebar stays active for nested routes
    const dashboardNav = page.locator('a[href="/app/dashboard"]').first();
    await expect(dashboardNav).toBeVisible();

    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("dashboard-overview")).toBeVisible({ timeout: 45_000 });

    // What Changed tabs
    const worsenedTab = page.getByTestId("overview-what-changed-tab-worsened");
    if (await worsenedTab.isVisible().catch(() => false)) {
      await worsenedTab.click();
      await expect(worsenedTab).toHaveAttribute("aria-selected", "true");
    }

    // Focus action deep link (if actions present)
    const firstFocus = page.getByTestId("overview-focus-action-1");
    if (await firstFocus.isVisible().catch(() => false)) {
      await firstFocus.click();
      await page.waitForURL(/\/app\/actions/, { timeout: 20_000 });
      await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
      await expect(page.getByTestId("dashboard-overview")).toBeVisible({ timeout: 45_000 });
      await page.getByTestId("overview-all-actions-link").click();
      await page.waitForURL(/\/app\/actions/, { timeout: 20_000 });
    }

    const serious = consoleErrors.filter(
      (text) => !/favicon|Download the React DevTools/i.test(text),
    );
    expect(serious).toEqual([]);
  });

  test("mobile overview has no page horizontal overflow", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
    await expect(page.getByRole("heading", { name: "Дашборд", level: 1 })).toBeVisible({
      timeout: 45_000,
    });
    await expect(page.getByTestId("dashboard-tabs")).toBeVisible();
    await expect(page.getByTestId("overview-kpi-grid")).toBeVisible({ timeout: 45_000 });

    const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    const clientWidth = await page.evaluate(() => document.documentElement.clientWidth);
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth + 1);
  });
});
