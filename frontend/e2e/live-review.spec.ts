/**
 * Live Seller Review acceptance against a temporary *.review.aivion.pro host.
 * Requires LIVE_REVIEW_BASE_URL (no cookies / no token in the browser).
 * Screenshots only on failure.
 */
import { expect, test } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

const LIVE_BASE = (process.env.LIVE_REVIEW_BASE_URL || "").replace(/\/$/, "");
const APEX = process.env.PUBLIC_DEMO_BASE_URL || "https://aivion.pro";
const REVIEW_TOKEN =
  process.env.REVIEW_TOKEN ||
  (process.env.REVIEW_TOKEN_FILE
    ? fs.readFileSync(process.env.REVIEW_TOKEN_FILE, "utf8").trim().split("/").pop()
    : "");

test.describe("live seller review", () => {
  test.skip(!LIVE_BASE, "LIVE_REVIEW_BASE_URL required");

  test("opens dashboard without cookies, login, or token leakage", async ({ browser }) => {
    const context = await browser.newContext();
    expect(await context.cookies()).toEqual([]);

    const page = await context.newPage();
    await page.goto(`${LIVE_BASE}/app/dashboard`, { waitUntil: "domcontentloaded" });
    await expect(page).not.toHaveURL(/\/login/, { timeout: 45_000 });
    await expect(page).toHaveURL(/\/app\/dashboard/);

    expect(page.url()).not.toMatch(/[0-9a-f]{64}/i);
    const cookiesAfter = await context.cookies();
    expect(cookiesAfter.filter((c) => /session/i.test(c.name))).toHaveLength(0);

    const html = await page.content();
    expect(html).not.toMatch(/X-Aivion-Live-Review-Token/i);
    expect(html.toLowerCase()).not.toContain("live_review_token");
    const ls = await page.evaluate(() => JSON.stringify(localStorage));
    const ss = await page.evaluate(() => JSON.stringify(sessionStorage));
    expect(ls).not.toMatch(/[0-9a-f]{64}/i);
    expect(ss).not.toMatch(/[0-9a-f]{64}/i);
    const docCookie = await page.evaluate(() => document.cookie);
    expect(docCookie).not.toMatch(/session/i);

    await expect(page.getByTestId("review-mode-banner")).toHaveCount(0);
    await expect(page.getByRole("link", { name: /dashboard|главная|пульс/i }).first()).toBeVisible({
      timeout: 60_000,
    });
    await expect(page.getByRole("link", { name: /actions|действия/i }).first()).toBeVisible();
    await expect(page.getByRole("link", { name: /chat|чат/i }).first()).toBeVisible();
    await expect(page.getByRole("link", { name: /аккаунт|account/i }).first()).toBeVisible();
    await expect(page.getByRole("link", { name: /ozon/i }).first()).toBeVisible();
    await expect(page.getByRole("link", { name: /admin/i })).toHaveCount(0);

    await expect(page.getByText(/Дашборд|Обзор|AI-сводка|пульс продавца|Seller Pulse|Главная/i).first()).toBeVisible({
      timeout: 90_000,
    });

    const me = await context.request.get(`${LIVE_BASE}/api/v1/auth/me`);
    expect(me.status()).toBe(200);
    expect(me.headers()["set-cookie"] ?? "").toBe("");
    const meBody = await me.json();
    expect(meBody.is_live_review).toBe(true);
    expect(meBody.is_admin).toBe(false);
    expect(meBody.read_only).toBe(true);
    expect(meBody.seller_account?.id).toBe(1);
    expect(JSON.stringify(meBody)).not.toMatch(/[0-9a-f]{64}/i);

    const home = await context.request.get(`${LIVE_BASE}/api/v1/dashboard/home`);
    expect(home.status()).toBe(200);
    expect(home.headers()["x-aivion-analysis-source"] || home.headers()["X-Aivion-Analysis-Source"]).toBeTruthy();

    await page.goto(`${LIVE_BASE}/app/actions`);
    await expect(page.getByTestId("action-card").first()).toBeVisible({ timeout: 60_000 });
    await page.getByTestId("action-card").first().click();
    await expect(page.getByTestId("action-ask-ai").or(page.getByText(/Спросить AI/i)).first()).toBeVisible({
      timeout: 60_000,
    });

    await page.goto(`${LIVE_BASE}/app/chat`);
    await expect(
      page.getByRole("button", { name: /новый чат/i }).or(page.getByText(/история|сесси|сообщени/i)).first(),
    ).toBeVisible({ timeout: 60_000 });

    await page.goto(`${LIVE_BASE}/app/account`);
    await expect(page.getByText(/аккаунт продавца|Account/i).first()).toBeVisible({ timeout: 60_000 });

    await page.goto(`${LIVE_BASE}/app/integrations/ozon`);
    await expect(page.getByText(/ozon|подключ/i).first()).toBeVisible({ timeout: 60_000 });

    const dismiss = await context.request.fetch(
      `${LIVE_BASE}/api/v1/actions/unified%3Apack%3A1%3A2026-%3Aaction_008/dismiss`,
      { method: "POST" },
    );
    expect(dismiss.status()).toBe(403);

    const ask = await context.request.fetch(`${LIVE_BASE}/api/v1/chat/ask`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      data: { question: "ping" },
    });
    expect(ask.status()).toBe(403);

    const upload = await context.request.fetch(`${LIVE_BASE}/api/v1/chat/attachments`, {
      method: "POST",
    });
    expect([403, 404, 405]).toContain(upload.status());

    const ozonPut = await context.request.fetch(`${LIVE_BASE}/api/v1/ozon/connection`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      data: {},
    });
    expect([403, 404, 405]).toContain(ozonPut.status());

    const admin = await context.request.get(`${LIVE_BASE}/api/v1/admin/me`);
    expect(admin.status()).toBe(403);

    await page.goto(`${LIVE_BASE}/app/admin`);
    await expect(page).not.toHaveURL(/\/app\/admin$/, { timeout: 15_000 });

    const index = await page.goto(`${LIVE_BASE}/live-review-index`);
    expect(index?.ok()).toBeTruthy();
    await expect(page.getByRole("link", { name: "Dashboard" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Actions" })).toBeVisible();

    await context.close();
  });

  test("apex still requires login without cookie", async ({ browser }) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    await page.goto(`${APEX}/app/dashboard`, { waitUntil: "domcontentloaded" });
    await expect(page).toHaveURL(/\/login/, { timeout: 30_000 });
    await context.close();
  });

  test("classic cookie review flow still works when REVIEW_TOKEN provided", async ({ browser }) => {
    test.skip(!REVIEW_TOKEN || REVIEW_TOKEN.length !== 64, "REVIEW_TOKEN optional for classic flow");
    const context = await browser.newContext();
    const page = await context.newPage();
    await page.goto(`${APEX}/review/${REVIEW_TOKEN}`);
    await expect(page).toHaveURL(/\/app(\/dashboard)?/, { timeout: 45_000 });
    await expect(page.getByTestId("review-mode-banner")).toBeVisible({ timeout: 30_000 });
    await context.close();
  });

  test("mobile viewport has no horizontal overflow on key pages", async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 390, height: 844 },
    });
    const page = await context.newPage();
    for (const pathSuffix of ["/app/dashboard", "/app/actions", "/app/chat"]) {
      await page.goto(`${LIVE_BASE}${pathSuffix}`, { waitUntil: "domcontentloaded" });
      await expect(page).not.toHaveURL(/\/login/, { timeout: 45_000 });
      const overflow = await page.evaluate(() => {
        const doc = document.documentElement;
        return doc.scrollWidth > doc.clientWidth + 1;
      });
      expect(overflow, `${pathSuffix} horizontal overflow`).toBeFalsy();
    }
    await page.getByRole("button", { name: /меню|menu|навигац/i }).first().click({ timeout: 5_000 }).catch(() => {});
    await context.close();
  });
});
