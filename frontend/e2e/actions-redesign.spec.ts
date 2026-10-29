import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { stubOzonConnectionReady } from "./helpers/auth";
import { prepareLegacyActionsScenario } from "./helpers/seed";

const OUT = path.join(__dirname, "..", "..", "backend", "debug", "actions-detail-fix-screenshots");

test.beforeEach(async ({ page }) => {
  await stubOzonConnectionReady(page);
});

test.describe("actions detail consistency", () => {
  test.beforeAll(() => {
    fs.mkdirSync(OUT, { recursive: true });
    prepareLegacyActionsScenario("available");
  });

  test("desktop / tablet / mobile capture", async ({ page }, testInfo) => {
    await page.goto("/app/actions");
    const firstCard = page.locator("[data-testid='action-card']").first();
    await expect(firstCard).toBeVisible({ timeout: 30_000 });
    await firstCard.click();

    await expect(page.getByText("AI-фокус дня")).toBeVisible();
    await expect(page.getByRole("button", { name: /Спросить AI/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Отметить выполненным/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /^Отметить неактуальным$/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /Ещё действия/i })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /Открыть список SKU/i })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /Открыть затронутые/i })).toHaveCount(0);
    await expect(page.getByText(/Ситуация|План действий/i).first()).toBeVisible();
    // Importance is required for new publications; legacy seeded fixtures may
    // only expose it when reason/problem is present.
    const importance = page.getByText(/Почему это важно/i);
    if ((await importance.count()) > 0) {
      await expect(importance.first()).toBeVisible();
    }
    await expect(page.getByText(/План действий/i).first()).toBeVisible();

    await expect(page.getByRole("group", { name: "Статусы действий" })).toBeVisible();
    await expect(page.getByPlaceholder("Поиск по действиям…")).toBeVisible();

    await page.setViewportSize({ width: 1440, height: 900 });
    await page.screenshot({
      path: path.join(OUT, "desktop-full.png"),
      fullPage: true,
    });

    await page.locator("article.action-detail-container").screenshot({
      path: path.join(OUT, "desktop-detail.png"),
    });

    await page.locator("aside[aria-label='Список действий']").screenshot({
      path: path.join(OUT, "desktop-list.png"),
    });

    await page.getByRole("button", { name: /^Отметить неактуальным$/i }).click();
    await expect(page.getByRole("dialog", { name: /неактуальн/i })).toBeVisible();
    await page.screenshot({
      path: path.join(OUT, "desktop-dismiss-dialog.png"),
      fullPage: false,
    });
    await page.getByRole("button", { name: "Отмена" }).click();

    await page.setViewportSize({ width: 1024, height: 768 });
    await page.goto("/app/actions");
    await expect(page.locator("[data-testid='action-card']").first()).toBeVisible({ timeout: 30_000 });
    await page.screenshot({ path: path.join(OUT, "tablet.png"), fullPage: true });

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/app/actions");
    await expect(page.locator("[data-testid='action-card']").first()).toBeVisible({ timeout: 30_000 });
    await page.screenshot({ path: path.join(OUT, "mobile-list.png"), fullPage: true });
    await page.locator("[data-testid='action-card']").first().click();
    await expect(page.getByRole("button", { name: /Назад к действиям/i })).toBeVisible();
    await page.screenshot({ path: path.join(OUT, "mobile-detail.png"), fullPage: true });

    testInfo.attachments.push({
      name: "screenshot-dir",
      contentType: "text/plain",
      body: Buffer.from(OUT),
    });
  });
});
