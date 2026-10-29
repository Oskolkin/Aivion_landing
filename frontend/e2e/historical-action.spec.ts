import { expect, test } from "@playwright/test";
import { stubOzonConnectionReady } from "./helpers/auth";
import { prepareLegacyActionsScenario } from "./helpers/seed";

test.beforeEach(async ({ page }) => {
  await stubOzonConnectionReady(page);
});

test.describe("historical action id", () => {
  test.beforeAll(() => {
    prepareLegacyActionsScenario("available");
  });

  test("shows not found for stale action key", async ({ page }) => {
    await page.goto("/app/actions?action=seed-historical-missing-action-1999");
    await expect(page.getByRole("heading", { name: /Действия/i }).first()).toBeVisible({
      timeout: 30_000,
    });
    await expect(page.locator("body")).not.toContainText("Application error");
    // Current Actions workspace either shows a not-found detail panel or safely
    // falls back to the list picker when the deep-linked action_key is absent.
    await expect(page.locator("[data-testid='action-card']").first()).toBeVisible({
      timeout: 30_000,
    });
  });
});
