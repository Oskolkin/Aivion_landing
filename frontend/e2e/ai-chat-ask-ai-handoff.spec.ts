import { expect, test } from "@playwright/test";
import { loginAsDemoSeller, stubOzonConnectionReady } from "./helpers/auth";

/**
 * P0: «Спросить AI» must open a stable chat session without redirect/API loops.
 */
test.describe("action ask AI handoff", () => {
  test("opens stable session without action refetch loop", async ({ page }) => {
    await stubOzonConnectionReady(page);
    await loginAsDemoSeller(page);

    const actionGets: string[] = [];
    page.on("request", (req) => {
      const url = req.url();
      if (req.method() === "GET" && url.includes("/api/v1/actions/") && !url.includes("/api/v1/actions?")) {
        actionGets.push(url);
      }
    });

    await page.goto("/app/actions");
    await expect(page.getByRole("button", { name: /Спросить AI/i }).first()).toBeVisible({
      timeout: 30_000,
    });

    // Pick a mid-list action to avoid always hitting the same first session.
    const listButtons = page.locator('[aria-current="true"], button').filter({
      hasText: /SKU|кампан|остат|ДРР|аудит/i,
    });
    const candidate = page.getByRole("button", { name: /Запланировать пополнение|Снизить избыточный|Провести аудит/i }).first();
    if (await candidate.isVisible().catch(() => false)) {
      await candidate.click();
    }

    await page.getByRole("button", { name: /Спросить AI/i }).first().click();

    await expect(page).toHaveURL(/\/app\/chat\?(session=\d+|action_id=\d+&mode=draft)/, {
      timeout: 20_000,
    });

    // Handoff must settle on a session URL (not stay forever on draft).
    await expect(page).toHaveURL(/\/app\/chat\?session=\d+/, { timeout: 120_000 });

    // No infinite getActionByKey storm (pre-fix was 200+ in a few seconds).
    await page.waitForTimeout(2500);
    const actionDetailGets = actionGets.filter((u) => /\/api\/v1\/actions\/[^?]+/.test(u));
    expect(actionDetailGets.length).toBeLessThan(12);

    await expect(
      page.getByText(/Помоги выполнить это действие|Чат по действию|действи/i).first(),
    ).toBeVisible({ timeout: 30_000 });

    // URL stays stable — no oscillating draft↔session.
    const href = page.url();
    await page.waitForTimeout(1500);
    expect(page.url()).toBe(href);
  });
});
