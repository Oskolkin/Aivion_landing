import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { loginAsDemoSeller, stubOzonConnectionReady } from "./helpers/auth";

test.use({ storageState: { cookies: [], origins: [] } });

const OUT = path.resolve(__dirname, "../../backend/debug/dashboard-audit-export/screenshots");

test("dashboard width screenshots @live", async ({ page }) => {
  fs.mkdirSync(OUT, { recursive: true });
  await stubOzonConnectionReady(page);
  await loginAsDemoSeller(page);

  for (const [width, height, name] of [
    [1920, 1080, "dashboard-1920.png"],
    [1440, 900, "dashboard-1440.png"],
    [1366, 768, "dashboard-1366.png"],
  ] as const) {
    await page.setViewportSize({ width, height });
    await page.goto("/app/dashboard");
    await expect(page.getByRole("heading", { name: /Пульс продавца/i })).toBeVisible({
      timeout: 60_000,
    });
    await page.waitForTimeout(1200);
    await page.screenshot({ path: path.join(OUT, name), fullPage: true });
  }
});
