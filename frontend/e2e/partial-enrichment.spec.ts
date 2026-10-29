import { expect, test } from "@playwright/test";
import { stubOzonConnectionReady } from "./helpers/auth";

test.beforeEach(async ({ page }) => {
  await stubOzonConnectionReady(page);
});

test.describe("partial enrichment", () => {
  test("dashboard loads when AI blocks are incomplete", async ({ page }) => {    await page.goto("/app/dashboard");
    await expect(page.getByRole("heading", { name: "Дашборд" })).toBeVisible({ timeout: 30_000 });
    await expect(page.getByRole("main").last()).not.toContainText(/Дашборд недоступен|Главная недоступна/i);
  });
});
