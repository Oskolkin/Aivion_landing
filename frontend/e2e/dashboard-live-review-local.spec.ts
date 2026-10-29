/**
 * Local Live Review regression against production Next + backend.
 * Simulates Caddy by injecting X-Aivion-Live-Review* headers (no cookies).
 *
 * Requires REVIEW_TOKEN (or REVIEW_TOKEN_FILE) and LIVE_REVIEW_HOST
 * (default: seller1.review.localhost).
 */
import { expect, test, type Browser, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

const outDir = path.resolve("/opt/cursor/artifacts/screenshots");
const REVIEW_TOKEN =
  process.env.REVIEW_TOKEN ||
  (process.env.REVIEW_TOKEN_FILE
    ? fs.readFileSync(process.env.REVIEW_TOKEN_FILE, "utf8").trim().split("/").pop()!.trim()
    : "");
const LIVE_HOST = process.env.LIVE_REVIEW_HOST || "seller1.review.localhost";
const BASE = (process.env.E2E_BASE_URL ?? "http://localhost:3000").replace(/\/$/, "");

async function liveContext(browser: Browser) {
  const context = await browser.newContext({
    storageState: { cookies: [], origins: [] },
    extraHTTPHeaders: {
      "X-Aivion-Live-Review": "1",
      "X-Aivion-Live-Review-Token": REVIEW_TOKEN!,
      "X-Aivion-Live-Review-Host": LIVE_HOST,
    },
  });
  await context.clearCookies();
  return context;
}

async function shot(page: Page, name: string) {
  fs.mkdirSync(outDir, { recursive: true });
  await page.screenshot({
    path: path.join(outDir, `${name}.png`),
    fullPage: true,
    animations: "disabled",
  });
}

test.describe("Live Review local regression", () => {
  test.skip(!REVIEW_TOKEN, "REVIEW_TOKEN required");

  test("overview + nested routes without login, mutations blocked", async ({ browser }) => {
    const context = await liveContext(browser);
    const page = await context.newPage();

    // No auth redirect
    await page.goto(`${BASE}/app/dashboard`, { waitUntil: "domcontentloaded" });
    await expect(page).not.toHaveURL(/\/login/, { timeout: 45_000 });
    await expect(page).toHaveURL(/\/app\/dashboard/);
    await expect(page.getByTestId("dashboard-overview")).toBeVisible({ timeout: 90_000 });
    await expect(page.locator('aside nav a[href="/app/dashboard"]')).toBeVisible();
    await shot(page, "live-review-overview-desktop");

    // Nested dashboard routes
    for (const route of ["funnel", "stock", "advertising", "categories", "sku"] as const) {
      await page.goto(`${BASE}/app/dashboard/${route}`, { waitUntil: "domcontentloaded" });
      await expect(page).not.toHaveURL(/\/login/);
      await expect(page.getByTestId(`dashboard-${route}-page`)).toBeVisible({ timeout: 60_000 });
      await expect(page.getByTestId(`dashboard-tab-${route}`)).toHaveAttribute("aria-current", "page");
      await expect(page.locator('aside nav a[href="/app/dashboard"]')).toBeVisible();
      await expect(page.locator('aside nav a[href="/app/dashboard"]')).toHaveAttribute("aria-current", "page");
    }

    // Seller 1 via /api/v1/auth/me (headers injected by context)
    const me = await context.request.get(`${BASE}/api/v1/auth/me`);
    expect(me.status()).toBe(200);
    expect(me.headers()["set-cookie"] ?? "").toBe("");
    const meBody = await me.json();
    expect(meBody.is_live_review).toBe(true);
    expect(meBody.read_only).toBe(true);
    expect(meBody.seller_account?.id).toBe(1);

    const home = await context.request.get(`${BASE}/api/v1/dashboard/home`);
    expect(home.status()).toBe(200);
    const homeBody = await home.json();
    expect(homeBody.status).toBeTruthy();
    expect(homeBody.metric_date).toBeTruthy();

    // Mutations blocked — no AI provider run via chat ask
    const ask = await context.request.fetch(`${BASE}/api/v1/chat/ask`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      data: { question: "live-review-regression-ping" },
    });
    expect(ask.status()).toBe(403);

    const dismiss = await context.request.fetch(
      `${BASE}/api/v1/actions/unified%3Apack%3A1%3A2026-%3Aaction_008/dismiss`,
      { method: "POST" },
    );
    expect(dismiss.status()).toBe(403);

    // Token must not leak into page
    const html = await page.content();
    expect(html).not.toMatch(/X-Aivion-Live-Review-Token/i);
    expect(html.toLowerCase()).not.toContain("live_review_token");
    expect(page.url()).not.toMatch(/[0-9a-f]{64}/i);

    await context.close();
  });
});
