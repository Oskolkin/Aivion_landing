import { expect, test } from "@playwright/test";
import { stubOzonConnectionReady } from "./helpers/auth";
import { prepareLegacyActionsScenario } from "./helpers/seed";

test.beforeEach(async ({ page }) => {
  await stubOzonConnectionReady(page);
});

test.describe("empty actions", () => {
  test.beforeAll(() => {
    prepareLegacyActionsScenario("empty");
  });

  test("shows empty state when no open actions", async ({ page }) => {
    await page.goto("/app/actions");
    await expect(page.getByText("Активных рекомендаций сейчас нет")).toBeVisible({
      timeout: 30_000,
    });
  });
});
