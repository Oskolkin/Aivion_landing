import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { stubOzonConnectionReady } from "./helpers/auth";
import { setUnifiedAnalysisMode } from "./helpers/seed";

const OUT = path.join(__dirname, "..", "..", "backend", "debug", "actions-stock-fix-screenshots");

test.beforeEach(async ({ page }) => {
  await stubOzonConnectionReady(page);
});

test.describe("stock actions after cohort repair @live", () => {
  test.beforeAll(() => {
    fs.mkdirSync(OUT, { recursive: true });
    // Legacy suites retarget pipeline.actions_run_id onto seed runs.
    // Always restore unified publication linkage before this audit.
    setUnifiedAnalysisMode("unified");
  });

  test("exclusive cohorts, coverage, recommendations", async ({ page }) => {
    await page.goto("/app/actions");
    await expect(page.locator("[data-testid='action-card']").first()).toBeVisible({ timeout: 30_000 });

    // Umbrella "11 дефицитным" must be gone; exclusive bands present.
    await expect(page.getByText(/11 дефицитн/i)).toHaveCount(0);
    await expect(page.getByText(/без остатка/i).first()).toBeVisible();
    await expect(page.getByText(/покрытием до трёх дней|покрытием до трех дней/i).first()).toBeVisible();
    await expect(page.getByText(/4–7 дней|4-7 дней/i).first()).toBeVisible();
    await expect(page.getByText(/Снизить избыточный запас|избыточн/i).first()).toBeVisible();
    await expect(page.getByText(/Перераспределить излишки/i)).toHaveCount(0);

    // Open OOS action
    await page.getByText(/без остатка/i).first().click();
    await expect(page.getByText(/Ситуация/i).first()).toBeVisible();
    await expect(page.getByText(/Почему это важно/i).first()).toBeVisible();
    await expect(page.getByText(/Рекомендуется поставить|Поставить \d+ шт/i).first()).toBeVisible({
      timeout: 10_000,
    });

    // Key metrics: zero stock shown as 0, not "Нет данных" for whole row.
    const detail = page.locator("article.action-detail-container");
    await expect(detail.getByText(/^0(\s*шт\.?)?$/).first()).toBeVisible();
    await expect(detail.locator("table tbody tr").first()).not.toContainText(
      /Нет данных[\s\S]*Нет данных[\s\S]*Нет данных[\s\S]*Нет данных/,
    );

    // No internal field names
    await expect(page.locator("body")).not.toContainText("available_stock_units");
    await expect(page.locator("body")).not.toContainText("stock_coverage_days_7d");
    await expect(page.locator("body")).not.toContainText("surplus_120d");

    await page.setViewportSize({ width: 1440, height: 900 });
    await page.screenshot({ path: path.join(OUT, "oos-1440.png"), fullPage: true });

    // Critical coverage action — click the list card, not any matching body text.
    await page
      .locator("[data-testid='action-card']")
      .filter({ hasText: /покрытием до трёх дней|покрытием до трех дней/i })
      .first()
      .click();
    await expect(detail.getByRole("heading", { level: 2 })).toContainText(
      /покрытием до трёх дней|покрытием до трех дней/i,
      { timeout: 15_000 },
    );
    await page.screenshot({ path: path.join(OUT, "critical-1440.png"), fullPage: true });
    // Coverage must not equal sales (e.g. 1.9 дн vs sales 61)
    await expect(detail.getByText(/1[.,]9\s*дн/i).first()).toBeVisible();

    await page
      .locator("[data-testid='action-card']")
      .filter({ hasText: /4–7 дней|4-7 дней/i })
      .first()
      .click();
    await expect(detail.getByRole("heading", { level: 2 })).toContainText(/4–7 дней|4-7 дней/i, {
      timeout: 15_000,
    });
    await page.screenshot({ path: path.join(OUT, "short-1440.png"), fullPage: true });

    await page
      .locator("[data-testid='action-card']")
      .filter({ hasText: /Снизить избыточный запас|избыточн/i })
      .first()
      .click();
    await expect(detail.getByRole("heading", { level: 2 })).toContainText(/избыточн/i, {
      timeout: 15_000,
    });
    await page.screenshot({ path: path.join(OUT, "surplus-1440.png"), fullPage: true });
    // SKU with 90 stock / 7 sales belongs here with ~90 days coverage
    await expect(detail.getByText(/90([.,]0)?\s*дн/i).first()).toBeVisible();


    for (const [w, h, name] of [
      [1280, 800, "1280"],
      [1024, 768, "1024"],
      [390, 844, "390"],
    ] as const) {
      await page.setViewportSize({ width: w, height: h });
      await page.screenshot({ path: path.join(OUT, `surplus-${name}.png`), fullPage: true });
    }
  });
});
