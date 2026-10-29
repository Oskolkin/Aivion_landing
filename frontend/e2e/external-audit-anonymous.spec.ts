/**
 * Anonymous validation of a published external-audit URL (@live).
 * Requires EXTERNAL_AUDIT_URL or EXTERNAL_AUDIT_URL_FILE (full URL, no auth).
 */
import { expect, test } from "@playwright/test";
import fs from "node:fs";

function loadUrl() {
  if (process.env.EXTERNAL_AUDIT_URL) return process.env.EXTERNAL_AUDIT_URL.replace(/\/?$/, "/");
  if (process.env.EXTERNAL_AUDIT_URL_FILE) {
    return fs.readFileSync(process.env.EXTERNAL_AUDIT_URL_FILE, "utf8").trim().replace(/\/?$/, "/");
  }
  return "";
}

const AUDIT_URL = loadUrl();

test.describe("external audit anonymous @live", () => {
  test.skip(!AUDIT_URL, "EXTERNAL_AUDIT_URL(_FILE) required");

  test("index and assets without cookies or redirect", async ({ browser }) => {
    const context = await browser.newContext({
      storageState: undefined,
      extraHTTPHeaders: {},
    });
    await context.clearCookies();
    const page = await context.newPage();

    const res = await page.goto(AUDIT_URL, { waitUntil: "domcontentloaded" });
    expect(res?.status()).toBe(200);
    expect(res?.headers()["set-cookie"]).toBeFalsy();
    expect(page.url()).toContain("/external-audit/");
    expect(page.url()).not.toContain("/login");
    await expect(page.getByRole("heading", { name: /Aivion External Audit/i })).toBeVisible();

    const dash = await context.request.get(`${AUDIT_URL}screenshots/desktop/dashboard-full.png`);
    expect(dash.status()).toBe(200);
    expect(dash.headers()["content-type"]).toMatch(/image\/png/);
    expect(dash.headers()["set-cookie"]).toBeFalsy();

    const mobile = await context.request.get(`${AUDIT_URL}screenshots/mobile/dashboard-full.png`);
    expect(mobile.status()).toBe(200);

    const manifest = await context.request.get(`${AUDIT_URL}manifest.json`);
    expect(manifest.status()).toBe(200);
    const m = await manifest.json();
    expect(m.bundle_revision).toBeTruthy();
    expect(m.seller_id).toBe(1);

    for (const path of [
      "links.html",
      "visible-content.txt",
      "metrics-reconciliation.txt",
      "revision-diff.txt",
      "visible-content.md",
      "data/dashboard.sanitized.json",
      "data/actions.sanitized.json",
      "revision-diff.md",
    ]) {
      const r = await context.request.get(`${AUDIT_URL}${path}`);
      expect(r.status(), path).toBe(200);
    }

    const txt = await context.request.get(`${AUDIT_URL}visible-content.txt`);
    expect(txt.headers()["content-type"] || "").toMatch(/text\/plain/i);

    const indexHtml = await page.content();
    expect(indexHtml).toMatch(/<a\s+[^>]*href=["'][^"']+\.png/i);
    expect(indexHtml).toMatch(/<img\s+[^>]*src=["'][^"']+\.png/i);
    expect(indexHtml.toLowerCase()).not.toContain("<script");
    const aCount = (indexHtml.match(/<a\s+[^>]*href=["'][^"']+\.png/gi) || []).length;
    const imgCount = (indexHtml.match(/<img\s+[^>]*src=["'][^"']+\.png/gi) || []).length;
    expect(aCount).toBeGreaterThanOrEqual(30);
    expect(imgCount).toBeGreaterThanOrEqual(30);
    await expect(page.getByRole("link", { name: "All direct audit files", exact: true })).toBeVisible();


    const post = await context.request.fetch(AUDIT_URL, { method: "POST" });
    expect(post.status()).toBe(405);

    const api = await context.request.get(`${AUDIT_URL}api/v1/dashboard/home`);
    expect([404, 405]).toContain(api.status());

    await context.close();
  });

  test("normal demo login still works", async ({ page }) => {
    const password = process.env.DEMO_PASSWORD || "";
    test.skip(!password, "DEMO_PASSWORD required");
    await page.goto("https://aivion.pro/login");
    await page.locator("#email").fill(process.env.PUBLIC_DEMO_EMAIL || "demo@example.com");
    await page.locator("#password").fill(password);
    await page.getByRole("button", { name: /войти|вход/i }).click();
    await expect(page).toHaveURL(/\/app/, { timeout: 30_000 });
  });
});
