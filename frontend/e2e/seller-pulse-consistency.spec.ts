import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { loginAsDemoSeller } from "./helpers/auth";

/**
 * Live acceptance for seller 1 current Unified publication (demo account).
 */
test.use({ storageState: { cookies: [], origins: [] } });

test("seller pulse period + AI summary + important facts @live", async ({ page }) => {
  const consoleErrors: string[] = [];
  const failedRequests: string[] = [];
  page.on("pageerror", (err) => consoleErrors.push(err.message));
  page.on("console", (msg) => {
    if (msg.type() === "error") consoleErrors.push(msg.text());
  });
  page.on("response", (res) => {
    if (res.url().includes("/api/") && res.status() >= 500) {
      failedRequests.push(`${res.status()} ${res.url()}`);
    }
  });

  await loginAsDemoSeller(page);
  await page.goto("/app/dashboard");
  await expect(page.getByRole("heading", { name: /Пульс продавца/i })).toBeVisible({
    timeout: 60_000,
  });

  await expect(page.getByTestId("seller-pulse-period-context")).toBeVisible();
  await expect(page.getByTestId("seller-pulse-period-groups")).toBeVisible();
  await expect(page.getByTestId("seller-pulse-period-context")).toContainText("Период анализа");
  await expect(page.getByTestId("seller-pulse-period-context")).toContainText("2–8 июля");
  await expect(page.getByTestId("seller-pulse-period-context")).toContainText("Сравнение");
  await expect(page.getByTestId("seller-pulse-period-context")).toContainText("25 июня–1 июля");
  await expect(page.getByTestId("seller-pulse-period-context")).toContainText("Данные обновлены");
  await expect(page.getByTestId("seller-pulse-period-context")).toContainText("8 июля");
  await expect(page.getByTestId("seller-pulse-period-context")).not.toContainText("Карточки");
  await expect(page.getByTestId("seller-pulse-period-context")).not.toContainText("Графики");
  await expect(page.getByTestId("seller-pulse-period-context")).not.toContainText("AI-анализ");
  await expect(page.getByTestId("seller-pulse-period-context")).not.toContainText("Изменение");

  await expect(page.getByTestId("seller-pulse-ai-resume")).toBeVisible();
  // Grounded weekly totals from seller-1 current Unified publication (pub 92 / run 206):
  // orders 2 199 and store ad spend share 26,2% in the AI summary.
  await expect(page.getByTestId("seller-pulse-ai-summary")).toContainText(/2\s*199/);
  await expect(page.getByTestId("seller-pulse-ai-summary")).toContainText(/26[,.]2\s*%/);

  await expect(page.getByRole("heading", { name: /Что важно знать/i })).toBeVisible();
  await expect(page.getByRole("heading", { name: /Быстрые действия/i })).toBeVisible();

  const outDir = path.join(__dirname, "../debug-pulse-accept");
  fs.mkdirSync(outDir, { recursive: true });
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.locator("#seller-pulse-heading").scrollIntoViewIfNeeded();
  await page.screenshot({
    path: path.join(outDir, "seller-pulse-desktop.png"),
    fullPage: false,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator("#seller-pulse-heading").scrollIntoViewIfNeeded();
  await page.screenshot({
    path: path.join(outDir, "seller-pulse-mobile.png"),
    fullPage: true,
  });

  expect(failedRequests, failedRequests.join("\n")).toEqual([]);
  const severe = consoleErrors.filter(
    (e) => !/favicon|Download the React DevTools|Failed to load resource/i.test(e),
  );
  expect(severe, severe.join("\n")).toEqual([]);
});
