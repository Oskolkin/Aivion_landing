/**
 * Production Dashboard Overview acceptance against real API + production Next build.
 * Captures screenshots with real AppShell/sidebar and real publication data.
 *
 * Viewport notes (Playwright CSS pixels):
 * - desktop: 1440×900, deviceScaleFactor=1
 * - tablet: 1024×900, deviceScaleFactor=1
 * - mobile: 390×844, deviceScaleFactor=1
 */
import { expect, test, type Page, type Request } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { stubOzonConnectionReady } from "./helpers/auth";

const outDir = path.resolve("/opt/cursor/artifacts/screenshots");
const VIEWPORTS = {
  desktop: { width: 1440, height: 900, deviceScaleFactor: 1 },
  tablet: { width: 1024, height: 900, deviceScaleFactor: 1 },
  mobile: { width: 390, height: 844, deviceScaleFactor: 1 },
} as const;

async function shot(page: Page, name: string, fullPage = false) {
  fs.mkdirSync(outDir, { recursive: true });
  await page.screenshot({
    path: path.join(outDir, `${name}.png`),
    fullPage,
    animations: "disabled",
  });
}

async function waitOverview(page: Page) {
  await expect(page.getByRole("heading", { name: "Дашборд", level: 1 })).toBeVisible({
    timeout: 60_000,
  });
  await expect(page.getByTestId("dashboard-overview")).toBeVisible({ timeout: 60_000 });
  await expect(page.getByTestId("overview-kpi-grid")).toBeVisible();
}

function trackDashboardHome(page: Page) {
  const hits: Request[] = [];
  page.on("request", (req) => {
    if (req.url().includes("/api/v1/dashboard/home") && req.method() === "GET") {
      hits.push(req);
    }
  });
  return hits;
}

test.beforeEach(async ({ page }) => {
  await stubOzonConnectionReady(page);
});

test.describe("Dashboard Overview production acceptance", () => {
  test("desktop overview, nested routes, deep links, single fetch", async ({ page }) => {
    const consoleErrors: string[] = [];
    page.on("console", (msg) => {
      if (msg.type() === "error") consoleErrors.push(msg.text());
    });
    page.on("pageerror", (err) => consoleErrors.push(String(err)));

    await page.setViewportSize({
      width: VIEWPORTS.desktop.width,
      height: VIEWPORTS.desktop.height,
    });

    const homeHits = trackDashboardHome(page);
    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
    await waitOverview(page);

    // Real AppShell sidebar (desktop aside; avoid matching the hidden mobile drawer link)
    await expect(page.locator('aside nav a[href="/app/dashboard"]')).toBeVisible();
    await expect(page.getByTestId("dashboard-tabs")).toBeVisible();
    await expect(page.getByTestId("dashboard-tab-overview")).toHaveAttribute("aria-current", "page");
    await expect(page.getByTestId("dashboard-period-controls")).toBeVisible();

    // Duplicate AppShell «Дашборд» strip must be gone on dashboard routes
    await expect(page.getByTestId("app-shell-section-title")).toHaveCount(0);
    await expect(page.getByRole("heading", { name: "Дашборд", level: 1 })).toHaveCount(1);

    // No false CTAs
    await expect(page.getByText("Смотреть детали в рекомендациях")).toHaveCount(0);
    await expect(page.getByText("Подробнее об индексе")).toHaveCount(0);

    // Polish labels
    await expect(page.getByText("Ключевые факторы снижения")).toBeVisible();
    await expect(page.getByTestId("overview-funnel-preview")).toContainText("CTR по кабинету");
    await expect(page.getByTestId("overview-funnel-preview")).toContainText("Переходы в карточку");
    await expect(page.getByTestId("overview-mini-funnel")).toBeVisible();
    await expect(page.getByTestId("overview-stock-bar-chart")).toBeVisible();
    await expect(page.getByTestId("overview-ad-week-drr")).toContainText("ДРР за 7 дней");
    await expect(page.getByTestId("overview-ad-yesterday-drr")).toContainText("ДРР вчера");
    await expect(page.getByTestId("overview-what-changed-tabs")).toContainText("Новые риски");
    await expect(page.getByTestId("overview-what-changed-tabs")).toContainText("Актуальные риски");
    await expect(page.locator("aside")).not.toContainText("Свернуть меню");

    // No legacy long-page sections
    await expect(page.getByRole("heading", { name: "Пульс продавца" })).toHaveCount(0);
    await expect(page.getByRole("heading", { name: "AI-диагностика продаж" })).toHaveCount(0);
    await expect(page.getByText("DEV visual harness")).toHaveCount(0);
    await expect(page.locator("[data-nextjs-toast], #__next-build-watcher")).toHaveCount(0);

    // KPI short ad label
    await expect(page.getByTestId("overview-kpi-ad_spend_share_total_revenue")).toContainText(
      "Доля рекламы",
    );

    // Publication identity consistency from network payload
    await expect.poll(() => homeHits.length).toBeGreaterThan(0);
    // Use APIRequestContext (does not inflate page request tracker)
    const homeApi = await page.request.get("/api/v1/dashboard/home");
    expect(homeApi.ok()).toBeTruthy();
    const homeJson = await homeApi.json();
    expect(homeJson.status).toBeTruthy();
    expect(homeJson.metric_date).toBeTruthy();
    expect(homeJson.analysis_source).toBe("unified");
    // Ensure overview uses API-backed structure (real publication values)
    const bodyText = await page.locator("[data-testid=dashboard-overview]").innerText();
    expect(bodyText).toMatch(/AI-сводка|Фокус на сегодня|Индекс состояния|Что изменилось/);
    expect(bodyText).toContain(String(homeJson.metric_date).includes("2026") ? "2026" : homeJson.metric_date);
    // KPI values from seller_pulse must appear (no hard-coded fixture-only path)
    const revenueKpi = ((homeJson.seller_pulse || {}).kpis || []).find(
      (k: { key?: string }) => k.key === "revenue",
    );
    if (revenueKpi?.value) {
      expect(bodyText).toContain(String(revenueKpi.value).replace(/\s+/g, " ").trim().split(" ")[0]);
    }

    await shot(page, "prod-overview-first-viewport-desktop-1440", false);
    await shot(page, "prod-overview-full-page-desktop-1440", true);

    const whatChanged = page.getByTestId("overview-what-changed");
    await whatChanged.scrollIntoViewIfNeeded();
    const worsened = page.getByTestId("overview-what-changed-tab-worsened");
    if (await worsened.isVisible()) {
      await worsened.click();
      await expect(worsened).toHaveAttribute("aria-selected", "true");
    }
    await shot(page, "prod-overview-what-changed-tabs-desktop-1440", false);

    // Nested routes + sidebar active + no extra home fetches beyond initial layout load
    const beforeNav = homeHits.length;
    for (const [tab, testId] of [
      ["funnel", "dashboard-funnel-page"],
      ["stock", "dashboard-stock-page"],
      ["advertising", "dashboard-advertising-page"],
      ["categories", "dashboard-categories-page"],
      ["sku", "dashboard-sku-page"],
    ] as const) {
      await page.getByTestId(`dashboard-tab-${tab}`).click();
      await page.waitForURL(new RegExp(`/app/dashboard/${tab}`));
      await expect(page.getByTestId(testId)).toBeVisible({ timeout: 45_000 });
      await expect(page.getByTestId(`dashboard-tab-${tab}`)).toHaveAttribute("aria-current", "page");
      await expect(page.locator('aside nav a[href="/app/dashboard"]')).toBeVisible();
      await expect(page.locator('aside nav a[href="/app/dashboard"]')).toHaveAttribute("aria-current", "page");
      await shot(page, `prod-detail-${tab}-desktop-1440`, true);
    }
    // Layout provider should not refetch on every tab when staying in dashboard section.
    // Allow at most one extra (StrictMode/remount), never one-per-tab.
    expect(homeHits.length - beforeNav).toBeLessThanOrEqual(2);

    // Browser back
    await page.goBack();
    await page.waitForURL(/\/app\/dashboard\/categories/);

    // Direct load
    await page.goto("/app/dashboard/funnel", { waitUntil: "domcontentloaded" });
    await expect(page.getByTestId("dashboard-funnel-page")).toBeVisible({ timeout: 45_000 });

    // Focus action deep link
    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
    await waitOverview(page);
    const firstFocus = page.getByTestId("overview-focus-action-1");
    if (await firstFocus.count()) {
      await firstFocus.click();
      await page.waitForURL(/\/app\/actions/, { timeout: 30_000 });
      await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
      await waitOverview(page);
      await page.getByTestId("overview-all-actions-link").click();
      await page.waitForURL(/\/app\/actions/, { timeout: 30_000 });
    }

    // What Changed linked row
    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
    await waitOverview(page);
    const linkedRow = page.locator('[data-testid="overview-what-changed-row"][href]').first();
    if (await linkedRow.count()) {
      await linkedRow.click();
      await page.waitForURL(/\/app\/actions/, { timeout: 30_000 });
    }

    const serious = consoleErrors.filter(
      (t) =>
        !/favicon|Download the React DevTools|hydration/i.test(t) &&
        !/Failed to load resource:.*favicon/i.test(t) &&
        // AppShell probes /api/v1/admin/me for non-admins; Chromium still logs the 403.
        !/Failed to load resource: the server responded with a status of 403/i.test(t),
    );
    expect(serious, `console errors: ${serious.join("\n")}`).toEqual([]);

    // Record viewport metadata artifact
    fs.writeFileSync(
      path.join(outDir, "viewport-metadata.json"),
      JSON.stringify(
        {
          desktop: VIEWPORTS.desktop,
          tablet: VIEWPORTS.tablet,
          mobile: VIEWPORTS.mobile,
          homeFetchCount: homeHits.length,
          publicationHint: {
            metric_date: homeJson.metric_date,
            publication_id: homeJson.publication_id ?? null,
            analysis_source: homeJson.analysis_source ?? null,
            pipeline_run_id: homeJson.pipeline?.pipeline_run_id ?? null,
          },
        },
        null,
        2,
      ),
    );
  });

  test("tablet and mobile responsive overview", async ({ page }) => {
    await page.setViewportSize({
      width: VIEWPORTS.tablet.width,
      height: VIEWPORTS.tablet.height,
    });
    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
    await waitOverview(page);
    await shot(page, "prod-overview-tablet-1024", true);

    // Advertising preview spans full width on tablet (md:col-span-2)
    await expect(page.getByTestId("overview-advertising-preview")).toBeVisible();

    await page.setViewportSize({
      width: VIEWPORTS.mobile.width,
      height: VIEWPORTS.mobile.height,
    });
    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
    await waitOverview(page);
    await expect(page.getByTestId("dashboard-tabs")).toBeVisible();
    const whatChangedTabs = page.getByTestId("overview-what-changed-tabs");
    await expect(whatChangedTabs).toBeVisible();
    await page.getByTestId("overview-what-changed").scrollIntoViewIfNeeded();
    const worsenedMobile = page.getByTestId("overview-what-changed-tab-worsened");
    if (await worsenedMobile.isVisible()) {
      await worsenedMobile.click();
      await expect(worsenedMobile).toHaveAttribute("aria-selected", "true");
      await expect(worsenedMobile).toBeInViewport();
    }
    await shot(page, "prod-overview-what-changed-tabs-mobile-390", false);

    // Active dashboard tab visible after scroll affordance
    await page.getByTestId("dashboard-tab-sku").click();
    await page.waitForURL(/\/app\/dashboard\/sku/);
    await expect(page.getByTestId("dashboard-tab-sku")).toHaveAttribute("aria-current", "page");
    await expect(page.getByTestId("dashboard-tab-sku")).toBeInViewport();
    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
    await waitOverview(page);

    const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    const clientWidth = await page.evaluate(() => document.documentElement.clientWidth);
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth + 1);

    await shot(page, "prod-overview-mobile-390", true);
  });
});
