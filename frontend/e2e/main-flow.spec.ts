import { expect, test } from "@playwright/test";
import { stubOzonConnectionReady } from "./helpers/auth";
import { prepareLegacyActionsScenario } from "./helpers/seed";

test.beforeEach(async ({ page }) => {
  await stubOzonConnectionReady(page);
});

test.describe("seller main flow", () => {
  test.beforeAll(() => {
    prepareLegacyActionsScenario("available");
  });

  test("login → dashboard → top actions → action detail → chat → export", async ({ page }) => {
    await page.goto("/app/dashboard");
    await expect(page.getByRole("heading", { name: "Дашборд" })).toBeVisible({ timeout: 30_000 });

    await page.goto("/app/actions");
    const firstActionCard = page.locator("[data-testid='action-card']").first();
    await expect(firstActionCard).toBeVisible({ timeout: 30_000 });
    await firstActionCard.click();

    await expect(
      page.getByRole("button", { name: /Спросить AI/i }),
    ).toBeVisible({ timeout: 20_000 });
    await page.getByRole("button", { name: /Спросить AI/i }).click();
    await page.waitForURL(/\/app\/chat/, { timeout: 20_000 });

    await page.goto("/app/chat");
    const composer = page.getByPlaceholder(/ABC-анализ|Задайте вопрос/i);
    await composer.fill("Выгрузи товары с риском остатка в Excel");
    await page.getByRole("button", { name: /Отправить сообщение/i }).click();

    await expect(page.getByRole("link", { name: /Скачать файл/i })).toBeVisible({
      timeout: 90_000,
    });
  });
});
