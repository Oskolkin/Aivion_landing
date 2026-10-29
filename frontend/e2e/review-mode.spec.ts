/**
 * Temporary review-mode gates against live apex (manual / post-deploy).
 * REVIEW_TOKEN from env or file; never commit the token.
 */
import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

const BASE = process.env.PUBLIC_DEMO_BASE_URL || "https://aivion.pro";
const TOKEN =
  process.env.REVIEW_TOKEN ||
  (process.env.REVIEW_TOKEN_FILE
    ? fs.readFileSync(process.env.REVIEW_TOKEN_FILE, "utf8").trim().split("/").pop()
    : "");
const DEMO_EMAIL = process.env.PUBLIC_DEMO_EMAIL || "demo@example.com";
const DEMO_PASSWORD = process.env.DEMO_PASSWORD || "password123";
const OUT =
  process.env.REVIEW_EVIDENCE_DIR ||
  path.resolve(__dirname, "../../tmp/review-mode-evidence");

test.use({ baseURL: BASE });

test.beforeAll(() => {
  fs.mkdirSync(OUT, { recursive: true });
});

test("invalid review token redirects to login", async ({ page }) => {
  const bad = "a".repeat(64);
  await page.goto(`/review/${bad}`);
  await expect(page).toHaveURL(/\/login/, { timeout: 30_000 });
});

test("valid review token opens read-only dashboard without token in URL", async ({ page, request }) => {
  test.skip(!TOKEN || TOKEN.length !== 64, "REVIEW_TOKEN required");

  await page.goto(`/review/${TOKEN}`);
  await expect(page).toHaveURL(/\/app(\/dashboard)?/, { timeout: 45_000 });
  expect(page.url()).not.toContain(TOKEN);
  expect(page.url()).not.toMatch(/\/review\//);

  await expect(page.getByTestId("review-mode-banner")).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText(/Дашборд|Обзор|AI-сводка|пульс продавца|Seller Pulse|Главная/i).first()).toBeVisible({
    timeout: 90_000,
  });
  await page.screenshot({ path: path.join(OUT, "review-dashboard.png"), fullPage: true });

  await page.goto("/app/actions");
  await expect(page.getByTestId("action-card").first()).toBeVisible({ timeout: 60_000 });
  await page.getByTestId("action-card").first().click();
  await expect(page.getByTestId("action-ask-ai").or(page.getByText(/Спросить AI/i)).first()).toBeVisible({
    timeout: 60_000,
  });
  await page.screenshot({ path: path.join(OUT, "review-actions.png"), fullPage: true });

  await page.goto("/app/chat");
  await expect(
    page.getByRole("button", { name: /новый чат/i }).or(page.getByText(/история|сесси|сообщени/i)).first(),
  ).toBeVisible({ timeout: 60_000 });
  await page.screenshot({ path: path.join(OUT, "review-chat.png"), fullPage: true });

  // Mutations blocked by backend.
  const cookies = await page.context().cookies();
  const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join("; ");
  const dismiss = await request.fetch(`${BASE}/api/v1/actions/unified%3Apack%3A1%3A2026-%3Aaction_008/dismiss`, {
    method: "POST",
    headers: { Cookie: cookieHeader },
  });
  expect(dismiss.status()).toBe(403);

  const admin = await request.fetch(`${BASE}/api/v1/admin/me`, {
    headers: { Cookie: cookieHeader },
  });
  expect([401, 403]).toContain(admin.status());

  await page.goto("/app/admin");
  await expect(page).not.toHaveURL(/\/app\/admin$/, { timeout: 15_000 });
});

test("mobile review viewport renders dashboard", async ({ browser }) => {
  test.skip(!TOKEN || TOKEN.length !== 64, "REVIEW_TOKEN required");
  const context = await browser.newContext({ viewport: { width: 390, height: 844 } });
  const page = await context.newPage();
  await page.goto(`${BASE}/review/${TOKEN}`);
  await expect(page).toHaveURL(/\/app/, { timeout: 45_000 });
  await expect(page.getByTestId("review-mode-banner")).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText(/Дашборд|Обзор|AI-сводка|пульс продавца|Seller Pulse|Главная/i).first()).toBeVisible({
    timeout: 90_000,
  });
  const canScrollX = await page.evaluate(() => {
    const before = window.scrollX;
    window.scrollTo(2000, window.scrollY);
    const after = window.scrollX;
    window.scrollTo(before, window.scrollY);
    return after > before + 1;
  });
  expect(canScrollX).toBe(false);
  await page.screenshot({ path: path.join(OUT, "review-mobile.png"), fullPage: true });
  await context.close();
});

test("normal demo login still works", async ({ page }) => {
  await page.goto("/login");
  await page.locator("#email").fill(DEMO_EMAIL);
  await page.locator("#password").fill(DEMO_PASSWORD);
  await page.getByRole("button", { name: /войти|вход/i }).click();
  await expect(page).toHaveURL(/\/app/, { timeout: 30_000 });
  await expect(page.getByTestId("review-mode-banner")).toHaveCount(0);
  await expect(page.getByText(/Дашборд|Обзор|AI-сводка|пульс продавца|Seller Pulse|Главная/i).first()).toBeVisible({
    timeout: 90_000,
  });
});
