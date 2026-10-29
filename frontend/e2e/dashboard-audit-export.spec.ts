import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { loginAsDemoSeller, stubOzonConnectionReady } from "./helpers/auth";

/**
 * Read-only audit screenshots for seller 1 current publication.
 */
test.use({ storageState: { cookies: [], origins: [] } });

const OUT = path.resolve(__dirname, "../../backend/debug/dashboard-audit-export/screenshots");

test("dashboard audit screenshots @live", async ({ page }) => {
  fs.mkdirSync(OUT, { recursive: true });
  await stubOzonConnectionReady(page);
  await loginAsDemoSeller(page);
  await page.setViewportSize({ width: 1600, height: 1200 });

  await page.goto("/app/dashboard");
  await expect(page.getByRole("heading", { name: /Пульс продавца/i })).toBeVisible({
    timeout: 60_000,
  });
  await page.waitForTimeout(1500);
  await page.screenshot({
    path: path.join(OUT, "dashboard-full-desktop.png"),
    fullPage: true,
  });

  // Optional block crops if present
  const pulse = page.locator("section").filter({ has: page.getByRole("heading", { name: /Пульс продавца/i }) }).first();
  if (await pulse.count()) {
    await pulse.screenshot({ path: path.join(OUT, "block-seller-pulse.png") });
  }
  const important = page.locator("article, section, div").filter({
    has: page.getByRole("heading", { name: /Что важно знать/i }),
  }).first();
  if (await important.count()) {
    await important.screenshot({ path: path.join(OUT, "block-important-facts.png") }).catch(() => undefined);
  }

  await page.goto("/app/actions");
  await expect(page.getByRole("heading", { name: /Действия/i }).first()).toBeVisible({
    timeout: 60_000,
  });
  await page.waitForTimeout(1500);
  await page.screenshot({
    path: path.join(OUT, "actions-list-full.png"),
    fullPage: true,
  });

  const cards = page.locator("[data-testid='action-card']");
  const n = await cards.count();
  for (let i = 0; i < n; i++) {
    await cards.nth(i).click();
    await page.waitForTimeout(800);
    await page.screenshot({
      path: path.join(OUT, `action-detail-${String(i + 1).padStart(2, "0")}.png`),
      fullPage: true,
    });
    // close panel if possible
    const close = page.getByRole("button", { name: /Закрыть|Close|Назад/i }).first();
    if (await close.count()) {
      await close.click().catch(() => undefined);
      await page.waitForTimeout(300);
    } else {
      await page.keyboard.press("Escape").catch(() => undefined);
      await page.waitForTimeout(300);
    }
  }
});
