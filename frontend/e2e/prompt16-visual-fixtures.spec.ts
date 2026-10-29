import { expect, test } from "@playwright/test";
import { stubOzonConnectionReady } from "./helpers/auth";

const VIEWPORTS = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 768, height: 1024 },
  { width: 390, height: 844 },
] as const;

test.beforeEach(async ({ page }) => {
  await stubOzonConnectionReady(page);
});

test.describe("Prompt16 real Aivion responsive surfaces", () => {
  for (const vp of VIEWPORTS) {
    test(`viewport ${vp.width} dashboard overview stock advertising funnel`, async ({ page }) => {
      await page.setViewportSize(vp);
      await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
      await expect(page.getByRole("heading", { name: "Дашборд", level: 1 })).toBeVisible({
        timeout: 45_000,
      });
      await expect(page.getByTestId("dashboard-tabs")).toBeVisible();
      await expect(page.getByTestId("overview-kpi-grid")).toBeVisible({ timeout: 45_000 });

      await page.getByTestId("dashboard-tab-stock").click();
      await expect(page.getByTestId("dashboard-stock-page")).toBeVisible({ timeout: 30_000 });

      await page.getByTestId("dashboard-tab-advertising").click();
      await expect(page.getByTestId("dashboard-advertising-page")).toBeVisible({ timeout: 30_000 });

      await page.getByTestId("dashboard-tab-funnel").click();
      await expect(page.getByTestId("dashboard-funnel-page")).toBeVisible({ timeout: 30_000 });
    });

    test(`viewport ${vp.width} actions workspace`, async ({ page }) => {
      await page.setViewportSize(vp);
      await page.goto("/app/actions", { waitUntil: "domcontentloaded" });
      await expect(page.getByTestId("actions-workspace")).toBeVisible({ timeout: 45_000 });
    });

    test(`viewport ${vp.width} chat`, async ({ page }) => {
      await page.setViewportSize(vp);
      await page.goto("/app/chat", { waitUntil: "domcontentloaded" });
      await expect(page.getByLabel("Сообщение в чат")).toBeVisible({ timeout: 45_000 });
    });
  }
});

test("Prompt16 keyboard a11y on real dashboard", async ({ page }) => {
  await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });
  await expect(page.getByTestId("dashboard-tabs")).toBeVisible({ timeout: 45_000 });
  await page.keyboard.press("Tab");
  await expect(page.getByTestId("dashboard-tab-overview")).toBeVisible();
});
