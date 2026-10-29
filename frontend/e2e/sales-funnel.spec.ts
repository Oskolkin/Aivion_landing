import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { loginAsDemoSeller, stubOzonConnectionReady } from "./helpers/auth";

test.use({ storageState: { cookies: [], origins: [] } });

const OUT = path.resolve(__dirname, "../../backend/debug/dashboard-audit-export/screenshots");

test.describe("Sales funnel redesign @live", () => {
  test("desktop / tablet / mobile layout and API-backed values", async ({ page }) => {
    fs.mkdirSync(OUT, { recursive: true });
    await stubOzonConnectionReady(page);
    await loginAsDemoSeller(page);

    await page.goto("/app/dashboard");
    const funnelHeading = page.getByRole("heading", { name: /Воронка продаж/i });
    await expect(funnelHeading).toBeVisible({ timeout: 60_000 });

    const funnelSection = page.locator("section[aria-labelledby='sales-funnel-heading']");
    await expect(funnelSection).toBeVisible();

    const apiJson = await page.evaluate(async () => {
      const res = await fetch("/api/v1/dashboard/home");
      return res.json();
    });
    const apiStages: Array<{ value?: number | null }> =
      apiJson?.ai_diagnostics?.sales_funnel?.stages ?? [];
    expect(apiStages.length).toBeGreaterThanOrEqual(2);

    for (const [width, height, shot] of [
      [1440, 900, "funnel-desktop-1440.png"],
      [1024, 768, "funnel-tablet-1024.png"],
      [390, 844, "funnel-mobile-390.png"],
    ] as const) {
      await page.setViewportSize({ width, height });
      await funnelHeading.scrollIntoViewIfNeeded();
      await page.waitForTimeout(700);

      const overflow = await funnelSection.evaluate((el) => ({
        clientWidth: el.clientWidth,
        scrollWidth: el.scrollWidth,
      }));
      expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.clientWidth + 2);

      await expect(funnelSection.getByText(/Где теряется воронка/i)).toBeVisible();

      const accent = funnelSection.locator(
        '[data-funnel-stage-highlight="weak_spot"], [data-funnel-transition-highlight="weak_spot"], [data-funnel-insight-banner], [data-funnel-transition-highlight="bottleneck"]',
      );
      await expect(accent.first()).toBeVisible();

      const bodyText = await funnelSection.innerText();
      expect(bodyText).not.toMatch(/\bNaN\b/);
      expect(bodyText).not.toMatch(/\bundefined\b/);
      expect(bodyText).not.toMatch(/\bnull\b/i);
      expect(bodyText).not.toMatch(/по объёму/);
      expect(bodyText).not.toMatch(/объём этапа/);
      expect(bodyText).not.toMatch(/% объём/);
      expect(bodyText).toMatch(/открытий|показов|добавлений|заказанных|доставленных/i);

      await funnelSection.screenshot({ path: path.join(OUT, shot) });
    }

    await page.setViewportSize({ width: 1440, height: 900 });
    await page.screenshot({ path: path.join(OUT, "dashboard-full-with-funnel.png"), fullPage: true });
    await funnelHeading.scrollIntoViewIfNeeded();
    await funnelSection.screenshot({ path: path.join(OUT, "funnel-block-closeup.png") });

    const info = funnelSection.getByRole("button", { name: /Процент конверсии показывает/i });
    await info.focus();
    await expect(info).toBeFocused();

    const firstValue = apiStages[0]?.value;
    if (typeof firstValue === "number" && Number.isFinite(firstValue)) {
      const formatted = new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 }).format(firstValue);
      await expect(funnelSection.getByText(formatted, { exact: false }).first()).toBeVisible();
    }
  });
});
