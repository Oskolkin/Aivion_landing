/**
 * Playwright capture of seller live UI into a revision staging directory.
 * Never logs passwords, cookies, or tokens.
 */

import fs from "node:fs/promises";
import fsSync from "node:fs";
import path from "node:path";
import { chromium } from "playwright";
import { sanitizeObject, sanitizeText } from "./sanitize.mjs";

export const DEFAULT_BASE_URL = "https://aivion.pro";
export const DEFAULT_DEMO_EMAIL = "demo@example.com";

export const VIEWPORTS = {
  desktop: { width: 1440, height: 900 },
  laptop: { width: 1280, height: 800 },
  mobile: { width: 390, height: 844 },
};

const DASHBOARD_BLOCKS = [
  {
    name: "seller-pulse",
    testIds: ["seller-pulse-ai-resume", "seller-pulse-period-context"],
    texts: [/пульс продавца/i, /Seller Pulse/i, /пульс/i],
  },
  {
    name: "what-changed",
    testIds: ["what-changed-card"],
    texts: [/что изменилось/i, /What Changed/i],
  },
  {
    name: "funnel",
    testIds: ["funnel-chart-area", "funnel-comparison-bar-grid"],
    texts: [/воронк/i, /Sales Funnel/i],
  },
  {
    name: "advertising",
    testIds: ["ad-efficiency-module"],
    texts: [/реклам/i, /Advertising/i, /эффективность рекламы/i],
  },
  {
    name: "risk-drivers",
    testIds: ["health-improvement-block", "health-improvement-items"],
    texts: [/драйвер.*риск/i, /Risk Drivers/i, /риски/i, /требует внимания/i],
  },
  {
    name: "sku-focus",
    testIds: ["sku-focus-section"],
    texts: [/SKU Focus/i, /фокус.*SKU/i, /ключевые SKU/i],
  },
  {
    name: "stock",
    testIds: ["stock-coverage-module"],
    texts: [/остат/i, /покрыт.*запас/i, /Stock/i],
  },
];

/**
 * @param {{
 *   outDir: string,
 *   password: string,
 *   baseUrl?: string,
 *   email?: string,
 *   sellerId?: number|string,
 * }} options
 */
export async function captureBundle(options) {
  const outDir = options?.outDir;
  const password = options?.password;
  if (!outDir) throw new Error("captureBundle: outDir is required");
  if (!password) throw new Error("captureBundle: password is required");

  const baseUrl = String(options.baseUrl || process.env.BASE_URL || DEFAULT_BASE_URL).replace(/\/$/, "");
  const email = String(options.email || process.env.DEMO_EMAIL || DEFAULT_DEMO_EMAIL);
  const sellerId = Number(options.sellerId ?? 1) || 1;

  await ensureDirs(outDir);

  const consoleErrors = [];
  const failedRequests = [];
  const assetErrors = [];
  const notes = [];
  const pageMetadata = [];
  const viewportMetadata = [];
  let screenshotCount = 0;
  let chatContextPresent = false;
  let dashboardBlockCount = 0;
  let actionCount = 0;
  let actionDetailCount = 0;

  let browser;
  let publicationId = null;
  let sourceRunId = null;
  let sourceMode = null;
  let metricDate = null;

  try {
    browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({
      locale: "ru-RU",
      ignoreHTTPSErrors: true,
      viewport: VIEWPORTS.desktop,
    });
    const page = await context.newPage();

    attachDiagnostics(page, { consoleErrors, failedRequests, assetErrors });

    // --- Login screenshots (desktop + mobile) before auth ---
    await page.setViewportSize(VIEWPORTS.desktop);
    await page.goto(`${baseUrl}/login`, { waitUntil: "domcontentloaded", timeout: 90_000 });
    await settlePage(page);
    screenshotCount += await safeScreenshot(page, path.join(outDir, "screenshots", "desktop", "login.png"), {
      fullPage: true,
      notes,
      label: "desktop/login",
    });

    await page.setViewportSize(VIEWPORTS.mobile);
    await page.goto(`${baseUrl}/login`, { waitUntil: "domcontentloaded", timeout: 90_000 });
    await settlePage(page);
    screenshotCount += await safeScreenshot(page, path.join(outDir, "screenshots", "mobile", "login.png"), {
      fullPage: true,
      notes,
      label: "mobile/login",
    });

    // --- Authenticate on desktop ---
    await page.setViewportSize(VIEWPORTS.desktop);
    await page.goto(`${baseUrl}/login`, { waitUntil: "domcontentloaded", timeout: 90_000 });
    await page.locator('#email, input[type="email"]').first().fill(email);
    await page.locator('#password, input[type="password"]').first().fill(password);
    await page.getByRole("button", { name: /войти|вход/i }).click();

    try {
      await page.waitForURL(/\/app(\/|$)/, { timeout: 60_000 });
    } catch {
      /* checked below */
    }

    if (/\/login/i.test(page.url())) {
      throw new Error("captureBundle: still on /login after login attempt");
    }

    await gotoDashboard(page, baseUrl);
    await waitForDashboardReady(page);

    pageMetadata.push(await collectPageMeta(page, "post-login-dashboard"));

    // --- API JSON (cookies from logged-in context) ---
    const api = await fetchApiBundle(context, baseUrl, notes);
    const dashRaw = api.dashboard;
    const actionsRaw = api.actions;
    const actionDetailsRaw = api.actionDetails;
    const chatRaw = api.chat;
    const meRaw = api.me;

    const pub = buildPublicationManifest(dashRaw, meRaw, sellerId);
    publicationId = pub.publication_id;
    sourceRunId = pub.source_run_id;
    sourceMode = pub.source_mode;
    metricDate = pub.metric_date;

    await writeJson(path.join(outDir, "data", "dashboard.sanitized.json"), sanitizeObject(dashRaw ?? {}));
    await writeJson(path.join(outDir, "data", "actions.sanitized.json"), sanitizeObject(actionsRaw ?? {}));
    await writeJson(
      path.join(outDir, "data", "action-details.sanitized.json"),
      sanitizeObject(actionDetailsRaw ?? []),
    );
    await writeJson(path.join(outDir, "data", "publication-manifest.sanitized.json"), sanitizeObject(pub));
    await writeJson(path.join(outDir, "data", "chat-context.sanitized.json"), sanitizeObject(chatRaw ?? {}));

    const actionItems = extractActionItems(actionsRaw);
    actionCount = actionItems.length;
    actionDetailCount = Array.isArray(actionDetailsRaw) ? actionDetailsRaw.length : 0;
    chatContextPresent = Boolean(
      chatRaw &&
        ((Array.isArray(chatRaw.sessions) && chatRaw.sessions.length > 0) ||
          (Array.isArray(chatRaw.messages) && chatRaw.messages.length > 0) ||
          chatRaw.first_session),
    );

    // --- Desktop capture ---
    {
      const vp = "desktop";
      await page.setViewportSize(VIEWPORTS.desktop);
      await gotoDashboard(page, baseUrl);
      await waitForDashboardReady(page);
      viewportMetadata.push(await collectViewportMeta(page, vp));

      screenshotCount += await safeScreenshot(page, shotPath(outDir, vp, "dashboard-top.png"), {
        fullPage: false,
        notes,
        label: `${vp}/dashboard-top`,
      });
      screenshotCount += await safeScreenshot(page, shotPath(outDir, vp, "dashboard-full.png"), {
        fullPage: true,
        notes,
        label: `${vp}/dashboard-full`,
      });

      const blockHits = await captureDashboardBlocks(page, outDir, vp, notes);
      dashboardBlockCount = Math.max(dashboardBlockCount, blockHits);
      screenshotCount += blockHits;

      const dashText = sanitizeText(await safeInnerText(page));
      await writeText(path.join(outDir, "content", "dashboard.txt"), dashText);
      await writeText(path.join(outDir, "content", "dashboard.html"), sanitizeText(await safeInnerHTML(page)));

      const actionsShot = await captureActionsFlow(page, baseUrl, outDir, vp, notes, {
        detailPrefix: "action-detail",
        detailCount: 5,
        numbered: true,
      });
      screenshotCount += actionsShot.screenshots;
      await writeText(path.join(outDir, "content", "actions.txt"), sanitizeText(actionsShot.listText));
      await writeText(path.join(outDir, "content", "actions.html"), sanitizeText(actionsShot.listHtml));
      await writeText(path.join(outDir, "content", "action-details.txt"), sanitizeText(actionsShot.detailsText));
      await writeText(path.join(outDir, "content", "action-details.html"), sanitizeText(actionsShot.detailsHtml));

      const chatShot = await captureAiChat(page, baseUrl, outDir, vp, notes, {
        actionContextName: "ai-chat-action-context.png",
        directName: "ai-chat-direct.png",
      });
      screenshotCount += chatShot.screenshots;
      chatContextPresent = chatContextPresent || chatShot.contextPresent;
      await writeText(path.join(outDir, "content", "ai-chat-context.txt"), sanitizeText(chatShot.text));
      await writeText(path.join(outDir, "content", "ai-chat-context.html"), sanitizeText(chatShot.html));

      const fileShot = await captureFileWorkflowBestEffort(page, outDir, vp, notes);
      screenshotCount += fileShot;
    }

    // --- Laptop capture (subset) ---
    {
      const vp = "laptop";
      await page.setViewportSize(VIEWPORTS.laptop);
      await gotoDashboard(page, baseUrl);
      await waitForDashboardReady(page);
      viewportMetadata.push(await collectViewportMeta(page, vp));

      screenshotCount += await safeScreenshot(page, shotPath(outDir, vp, "dashboard-full.png"), {
        fullPage: true,
        notes,
        label: `${vp}/dashboard-full`,
      });

      const actionsShot = await captureActionsFlow(page, baseUrl, outDir, vp, notes, {
        detailPrefix: "action-detail",
        detailCount: 1,
        numbered: false,
      });
      screenshotCount += actionsShot.screenshots;

      const chatShot = await captureAiChat(page, baseUrl, outDir, vp, notes, {
        actionContextName: "ai-chat.png",
        directName: null,
        single: true,
      });
      screenshotCount += chatShot.screenshots;
    }

    // --- Mobile capture ---
    {
      const vp = "mobile";
      await page.setViewportSize(VIEWPORTS.mobile);
      await gotoDashboard(page, baseUrl);
      await waitForDashboardReady(page);
      viewportMetadata.push(await collectViewportMeta(page, vp));

      screenshotCount += await safeScreenshot(page, shotPath(outDir, vp, "dashboard-top.png"), {
        fullPage: false,
        notes,
        label: `${vp}/dashboard-top`,
      });
      screenshotCount += await safeScreenshot(page, shotPath(outDir, vp, "dashboard-full.png"), {
        fullPage: true,
        notes,
        label: `${vp}/dashboard-full`,
      });

      // navigation-open
      try {
        const menuBtn = page.getByRole("button", { name: /меню|открыть меню|menu/i }).first();
        if (await menuBtn.isVisible({ timeout: 3_000 }).catch(() => false)) {
          await menuBtn.click();
          await page.waitForTimeout(400);
          screenshotCount += await safeScreenshot(page, shotPath(outDir, vp, "navigation-open.png"), {
            fullPage: false,
            notes,
            label: `${vp}/navigation-open`,
          });
          let navText = "";
          try {
            navText = sanitizeText(await page.locator("#app-mobile-nav").innerText({ timeout: 2_000 }));
          } catch {
            navText = sanitizeText(await safeInnerText(page));
          }
          await writeText(path.join(outDir, "content", "navigation.txt"), navText);
          const closeBtn = page.getByRole("button", { name: /закрыть|close/i }).first();
          if (await closeBtn.isVisible({ timeout: 1_000 }).catch(() => false)) await closeBtn.click();
        } else {
          notes.push("mobile navigation menu button not found; skipped navigation-open.png");
          await writeText(path.join(outDir, "content", "navigation.txt"), "");
        }
      } catch (e) {
        notes.push(`mobile navigation capture failed: ${errMsg(e)}`);
        await writeText(path.join(outDir, "content", "navigation.txt"), "");
      }

      const actionsShot = await captureActionsFlow(page, baseUrl, outDir, vp, notes, {
        detailPrefix: "action-detail",
        detailCount: 1,
        numbered: false,
      });
      screenshotCount += actionsShot.screenshots;

      const chatShot = await captureAiChat(page, baseUrl, outDir, vp, notes, {
        actionContextName: "ai-chat.png",
        directName: null,
        single: true,
      });
      screenshotCount += chatShot.screenshots;
    }

    // Recount screenshots on disk (authoritative)
    screenshotCount = countPngs(path.join(outDir, "screenshots"));

    await writeVisibleContentMd(outDir);
    await writeMetricsReconciliation(outDir, {
      publicationId,
      sourceRunId,
      sourceMode,
      metricDate,
      dashboard: dashRaw,
      actions: actionsRaw,
      actionDetails: actionDetailsRaw,
      notes,
    });

    await writeJson(path.join(outDir, "diagnostics", "console-errors.json"), consoleErrors);
    await writeJson(path.join(outDir, "diagnostics", "failed-requests.json"), failedRequests);
    await writeJson(path.join(outDir, "diagnostics", "asset-errors.json"), assetErrors);
    await writeJson(path.join(outDir, "diagnostics", "page-metadata.json"), pageMetadata);
    await writeJson(path.join(outDir, "diagnostics", "viewport-metadata.json"), viewportMetadata);

    const summary = [
      "# Capture summary",
      "",
      `- base_url: ${baseUrl}`,
      `- email: ${email}`,
      `- seller_id: ${sellerId}`,
      `- publication_id: ${publicationId ?? "null"}`,
      `- source_run_id: ${sourceRunId ?? "null"}`,
      `- source_mode: ${sourceMode ?? "null"}`,
      `- metric_date: ${metricDate ?? "null"}`,
      `- dashboard_block_count: ${dashboardBlockCount}`,
      `- action_count: ${actionCount}`,
      `- action_detail_count: ${actionDetailCount}`,
      `- screenshot_count: ${screenshotCount}`,
      `- chat_context_present: ${chatContextPresent}`,
      `- console_errors: ${consoleErrors.length}`,
      `- failed_requests: ${failedRequests.length}`,
      `- asset_errors: ${assetErrors.length}`,
      "",
      "## Notes",
      "",
      ...(notes.length ? notes.map((n) => `- ${n}`) : ["- none"]),
      "",
    ].join("\n");
    await writeText(path.join(outDir, "diagnostics", "capture-summary.md"), summary);

    return {
      publication_id: publicationId,
      source_run_id: sourceRunId,
      source_mode: sourceMode,
      metric_date: metricDate,
      dashboard_block_count: dashboardBlockCount,
      action_count: actionCount,
      action_detail_count: actionDetailCount,
      screenshot_count: screenshotCount,
      chat_context_present: chatContextPresent,
      seller_id: sellerId,
    };
  } finally {
    if (browser) {
      try {
        await browser.close();
      } catch {
        /* ignore */
      }
    }
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function shotPath(outDir, vp, name) {
  return path.join(outDir, "screenshots", vp, name);
}

async function ensureDirs(outDir) {
  for (const rel of [
    "screenshots/desktop",
    "screenshots/laptop",
    "screenshots/mobile",
    "content",
    "data",
    "diagnostics",
  ]) {
    await fs.mkdir(path.join(outDir, rel), { recursive: true });
  }
}

function attachDiagnostics(page, { consoleErrors, failedRequests, assetErrors }) {
  page.on("console", (msg) => {
    if (msg.type() === "error") {
      consoleErrors.push({ type: msg.type(), text: truncate(msg.text(), 500), url: page.url() });
    }
  });
  page.on("pageerror", (err) => {
    consoleErrors.push({ type: "pageerror", text: truncate(String(err?.message || err), 500), url: page.url() });
  });
  page.on("requestfailed", (req) => {
    const failure = req.failure();
    const entry = {
      url: sanitizeUrl(req.url()),
      method: req.method(),
      errorText: failure?.errorText || "failed",
    };
    if (/\.(png|jpe?g|gif|webp|svg|woff2?|css|js)(\?|$)/i.test(req.url()) || req.resourceType() === "image") {
      assetErrors.push(entry);
    } else {
      failedRequests.push(entry);
    }
  });
  page.on("response", (res) => {
    if (res.status() >= 400) {
      failedRequests.push({
        url: sanitizeUrl(res.url()),
        method: res.request().method(),
        status: res.status(),
      });
    }
  });
}

function sanitizeUrl(url) {
  try {
    const u = new URL(url);
    u.searchParams.delete("token");
    u.searchParams.delete("access_token");
    u.searchParams.delete("signature");
    return u.toString();
  } catch {
    return String(url).slice(0, 300);
  }
}

function truncate(s, n) {
  const t = String(s || "");
  return t.length > n ? t.slice(0, n) + "…" : t;
}

function errMsg(e) {
  return truncate(e?.message || String(e), 200);
}

async function settlePage(page) {
  try {
    await page.waitForLoadState("networkidle", { timeout: 15_000 });
  } catch {
    /* idle-ish best effort */
  }
  try {
    await page.evaluate(async () => {
      if (document.fonts?.ready) await document.fonts.ready;
    });
  } catch {
    /* ignore */
  }
  await page.waitForTimeout(400);
}

async function waitForDashboardReady(page) {
  const pulse = page.getByText(/пульс продавца|Seller Pulse|пульс/i).first();
  await pulse.waitFor({ state: "visible", timeout: 90_000 }).catch(() => {});
  await settlePage(page);
  // Best-effort: wait out visible skeleton / spinner chrome
  for (let i = 0; i < 8; i++) {
    const busy = await page
      .evaluate(() => {
        const body = document.body?.innerText || "";
        if (/загрузка|loading|skeleton/i.test(body) && body.length < 80) return true;
        const spinners = document.querySelectorAll('[class*="animate-pulse"], [data-testid*="skeleton"]');
        for (const el of spinners) {
          const r = el.getBoundingClientRect();
          if (r.width > 40 && r.height > 20) return true;
        }
        return false;
      })
      .catch(() => false);
    if (!busy) break;
    await page.waitForTimeout(750);
  }
  await settlePage(page);
  await assertNotLoginForm(page);
}

async function gotoDashboard(page, baseUrl) {
  await page.goto(`${baseUrl}/app/dashboard`, { waitUntil: "domcontentloaded", timeout: 90_000 });
  if (/\/login/i.test(page.url())) {
    throw new Error("captureBundle: redirected to /login when opening dashboard");
  }
}

async function assertNotLoginForm(page) {
  if (/\/login/i.test(page.url())) {
    throw new Error("captureBundle: dashboard URL is /login");
  }
  const pwd = page.locator('#password, input[type="password"]');
  const email = page.locator('#email, input[type="email"]');
  const loginVisible =
    (await pwd.isVisible().catch(() => false)) && (await email.isVisible().catch(() => false));
  if (loginVisible) {
    throw new Error("captureBundle: dashboard shows login form");
  }
}

async function safeScreenshot(page, filePath, { fullPage = false, notes, label } = {}) {
  try {
    await fs.mkdir(path.dirname(filePath), { recursive: true });
    await page.screenshot({ path: filePath, fullPage: Boolean(fullPage), timeout: 60_000 });
    return 1;
  } catch (e) {
    notes?.push(`screenshot failed ${label || filePath}: ${errMsg(e)}`);
    return 0;
  }
}

async function captureElementOrScrollFallback(page, outDir, vp, name, block, notes) {
  const file = shotPath(outDir, vp, `${name}.png`);
  try {
    for (const id of block.testIds || []) {
      const loc = page.getByTestId(id).first();
      if (await loc.isVisible({ timeout: 2_000 }).catch(() => false)) {
        await loc.scrollIntoViewIfNeeded().catch(() => {});
        await page.waitForTimeout(200);
        await loc.screenshot({ path: file, timeout: 30_000 });
        return 1;
      }
    }
    for (const re of block.texts || []) {
      const loc = page.getByText(re).first();
      if (await loc.isVisible({ timeout: 1_500 }).catch(() => false)) {
        await loc.scrollIntoViewIfNeeded().catch(() => {});
        await page.waitForTimeout(200);
        // Prefer nearest section/article/card ancestor
        const box = await loc.boundingBox().catch(() => null);
        if (box) {
          await page.screenshot({
            path: file,
            clip: {
              x: Math.max(0, box.x - 8),
              y: Math.max(0, box.y - 8),
              width: Math.min(VIEWPORTS[vp]?.width || 1440, Math.max(box.width + 16, 600)),
              height: Math.min(VIEWPORTS[vp]?.height || 900, Math.max(box.height + 16, 320)),
            },
          });
          return 1;
        }
        await page.screenshot({ path: file, fullPage: false });
        return 1;
      }
    }
    notes.push(`${vp}/${name}: locator not found; full-page crop fallback`);
    await page.screenshot({ path: file, fullPage: false });
    return 1;
  } catch (e) {
    notes.push(`${vp}/${name} screenshot failed: ${errMsg(e)}`);
    return 0;
  }
}

async function captureDashboardBlocks(page, outDir, vp, notes) {
  let n = 0;
  for (const block of DASHBOARD_BLOCKS) {
    n += await captureElementOrScrollFallback(page, outDir, vp, block.name, block, notes);
  }
  return n;
}

async function captureActionsFlow(page, baseUrl, outDir, vp, notes, opts) {
  const result = {
    screenshots: 0,
    listText: "",
    listHtml: "",
    detailsText: "",
    detailsHtml: "",
  };
  try {
    await page.goto(`${baseUrl}/app/actions`, { waitUntil: "domcontentloaded", timeout: 90_000 });
    await settlePage(page);
    await assertNotLoginForm(page);
    const cards = page.getByTestId("action-card");
    await cards.first().waitFor({ state: "visible", timeout: 60_000 }).catch(() => {});
    result.screenshots += await safeScreenshot(page, shotPath(outDir, vp, "actions-list.png"), {
      fullPage: true,
      notes,
      label: `${vp}/actions-list`,
    });
    result.listText = await safeInnerText(page);
    result.listHtml = await safeInnerHTML(page);

    const count = Math.min(opts.detailCount || 5, await cards.count().catch(() => 0));
    const detailParts = [];
    for (let i = 0; i < count; i++) {
      try {
        await page.goto(`${baseUrl}/app/actions`, { waitUntil: "domcontentloaded", timeout: 60_000 });
        await settlePage(page);
        const card = page.getByTestId("action-card").nth(i);
        if (!(await card.isVisible({ timeout: 5_000 }).catch(() => false))) break;
        await card.click();
        await page.waitForTimeout(600);
        const detail = page.locator(".action-detail-container").first();
        await detail.waitFor({ state: "visible", timeout: 15_000 }).catch(() => {});
        const fname = opts.numbered
          ? `${opts.detailPrefix}-${String(i + 1).padStart(2, "0")}.png`
          : `${opts.detailPrefix}.png`;
        if (await detail.isVisible().catch(() => false)) {
          try {
            await detail.screenshot({ path: shotPath(outDir, vp, fname), timeout: 30_000 });
            result.screenshots += 1;
          } catch {
            result.screenshots += await safeScreenshot(page, shotPath(outDir, vp, fname), {
              fullPage: false,
              notes,
              label: `${vp}/${fname}`,
            });
          }
          detailParts.push(await detail.innerText().catch(() => ""));
        } else {
          result.screenshots += await safeScreenshot(page, shotPath(outDir, vp, fname), {
            fullPage: false,
            notes,
            label: `${vp}/${fname}`,
          });
        }
        if (!opts.numbered) break;
      } catch (e) {
        notes.push(`${vp} action-detail ${i + 1} failed: ${errMsg(e)}`);
      }
    }
    result.detailsText = detailParts.join("\n\n---\n\n");
    result.detailsHtml = result.detailsText;
  } catch (e) {
    notes.push(`${vp} actions flow failed: ${errMsg(e)}`);
  }
  return result;
}

async function captureAiChat(page, baseUrl, outDir, vp, notes, opts) {
  const result = { screenshots: 0, text: "", html: "", contextPresent: false };
  try {
    // From action: Спросить AI
    await page.goto(`${baseUrl}/app/actions`, { waitUntil: "domcontentloaded", timeout: 60_000 });
    await settlePage(page);
    const ask = page.getByTestId("action-ask-ai").or(page.getByRole("button", { name: /Спросить AI/i })).first();
    if (await ask.isVisible({ timeout: 10_000 }).catch(() => false)) {
      await ask.click();
      try {
        await page.waitForURL(/\/app\/chat/, { timeout: 60_000 });
      } catch {
        notes.push(`${vp}: ask-ai did not navigate to /app/chat`);
      }
      await settlePage(page);
      await page.waitForTimeout(800);
      result.contextPresent = /session=/.test(page.url());
      result.screenshots += await safeScreenshot(page, shotPath(outDir, vp, opts.actionContextName), {
        fullPage: true,
        notes,
        label: `${vp}/${opts.actionContextName}`,
      });
      result.text = await safeInnerText(page);
      result.html = await safeInnerHTML(page);
    } else {
      notes.push(`${vp}: Спросить AI / action-ask-ai not found`);
    }

    if (opts.directName && !opts.single) {
      await page.goto(`${baseUrl}/app/chat`, { waitUntil: "domcontentloaded", timeout: 60_000 });
      await settlePage(page);
      result.screenshots += await safeScreenshot(page, shotPath(outDir, vp, opts.directName), {
        fullPage: true,
        notes,
        label: `${vp}/${opts.directName}`,
      });
      if (!result.text) {
        result.text = await safeInnerText(page);
        result.html = await safeInnerHTML(page);
      }
    } else if (opts.single && !result.screenshots) {
      await page.goto(`${baseUrl}/app/chat`, { waitUntil: "domcontentloaded", timeout: 60_000 });
      await settlePage(page);
      result.screenshots += await safeScreenshot(page, shotPath(outDir, vp, opts.actionContextName), {
        fullPage: true,
        notes,
        label: `${vp}/${opts.actionContextName}`,
      });
      result.text = await safeInnerText(page);
      result.html = await safeInnerHTML(page);
    }
  } catch (e) {
    notes.push(`${vp} ai-chat capture failed: ${errMsg(e)}`);
  }
  return result;
}

async function captureFileWorkflowBestEffort(page, outDir, vp, notes) {
  let n = 0;
  try {
    const origin = new URL(page.url()).origin;
    if (!/\/app\/chat/i.test(page.url())) {
      await page.goto(`${origin}/app/chat`, { waitUntil: "domcontentloaded", timeout: 60_000 });
    }
    await settlePage(page);

    const candidates = [
      page.getByRole("button", { name: /прикреп|файл|upload|attach|влож/i }).first(),
      page.locator('input[type="file"]').first(),
      page.locator("[data-testid*='attach'], [data-testid*='upload']").first(),
    ];
    let attach = null;
    for (const loc of candidates) {
      if (await loc.isVisible({ timeout: 2_000 }).catch(() => false)) {
        attach = loc;
        break;
      }
    }

    if (attach) {
      await attach.scrollIntoViewIfNeeded().catch(() => {});
      n += await safeScreenshot(page, shotPath(outDir, vp, "file-upload.png"), {
        fullPage: false,
        notes,
        label: `${vp}/file-upload`,
      });
      notes.push(
        "file-upload UI found; extraction-ready.png and export-ready.png skipped without uploading demo fixtures (no hard fail)",
      );
    } else {
      notes.push(
        "file UI not reachable (chat attachments area); skipped file-upload.png, extraction-ready.png, export-ready.png",
      );
    }
  } catch (e) {
    notes.push(`file workflow best-effort failed: ${errMsg(e)}`);
  }
  return n;
}

async function safeInnerText(pageOrLocator) {
  try {
    if (pageOrLocator.innerText && pageOrLocator.locator) {
      return await pageOrLocator.innerText("body");
    }
    if (pageOrLocator.innerText) {
      return await pageOrLocator.innerText();
    }
    return await pageOrLocator.evaluate(() => document.body?.innerText || "");
  } catch {
    try {
      return await pageOrLocator.evaluate(() => document.body?.innerText || "");
    } catch {
      return "";
    }
  }
}

async function safeInnerHTML(page) {
  try {
    return await page.evaluate(() => document.body?.innerHTML || "");
  } catch {
    return "";
  }
}

async function collectPageMeta(page, label) {
  return {
    label,
    url: sanitizeUrl(page.url()),
    title: await page.title().catch(() => ""),
    viewport: page.viewportSize(),
    at: new Date().toISOString(),
  };
}

async function collectViewportMeta(page, vp) {
  return {
    viewport: vp,
    size: page.viewportSize(),
    url: sanitizeUrl(page.url()),
    at: new Date().toISOString(),
  };
}

async function fetchApiBundle(context, baseUrl, notes) {
  const req = context.request;
  const out = { me: null, dashboard: null, actions: null, actionDetails: [], chat: {} };

  out.me = await getJson(req, `${baseUrl}/api/v1/auth/me`, notes, "auth/me");
  out.dashboard = await getJson(req, `${baseUrl}/api/v1/dashboard/home`, notes, "dashboard/home");
  out.actions = await getJson(req, `${baseUrl}/api/v1/actions`, notes, "actions");

  const items = extractActionItems(out.actions).slice(0, 8);
  for (const item of items) {
    const key = item.action_key || item.id || item.action_id;
    if (!key) continue;
    const detail = await getJson(
      req,
      `${baseUrl}/api/v1/actions/${encodeURIComponent(String(key))}`,
      notes,
      `actions/${key}`,
    );
    if (detail) out.actionDetails.push(detail);
  }

  const sessions = await getJson(req, `${baseUrl}/api/v1/chat/sessions`, notes, "chat/sessions");
  out.chat = { sessions: sessions?.items || sessions?.sessions || sessions || [] };
  const first = Array.isArray(out.chat.sessions) ? out.chat.sessions[0] : null;
  const sid = first?.id || first?.session_id;
  if (sid) {
    out.chat.first_session = first;
    out.chat.messages = await getJson(
      req,
      `${baseUrl}/api/v1/chat/sessions/${encodeURIComponent(String(sid))}/messages`,
      notes,
      `chat/sessions/${sid}/messages`,
    );
  }
  return out;
}

async function getJson(request, url, notes, label) {
  try {
    const res = await request.get(url, { timeout: 60_000 });
    if (!res.ok()) {
      notes.push(`API ${label}: HTTP ${res.status()}`);
      return null;
    }
    return await res.json();
  } catch (e) {
    notes.push(`API ${label} failed: ${errMsg(e)}`);
    return null;
  }
}

export function extractActionItems(actionsRaw) {
  if (!actionsRaw) return [];
  if (Array.isArray(actionsRaw)) return actionsRaw;
  if (Array.isArray(actionsRaw.items)) return actionsRaw.items;
  if (Array.isArray(actionsRaw.actions)) return actionsRaw.actions;
  return [];
}

export function buildPublicationManifest(dashboard, me, sellerId) {
  const d = dashboard && typeof dashboard === "object" ? dashboard : {};
  const pipeline = d.pipeline || {};
  const publication_id = d.publication_id ?? null;
  const source_mode = d.analysis_source || d.source_mode || null;
  const metric_date = d.metric_date || d.effective_metric_date || null;
  const source_run_id =
    pipeline.pipeline_run_id ??
    d.pipeline_run_id ??
    pipeline.dashboard_run_id ??
    pipeline.actions_run_id ??
    null;

  return {
    seller_id: sellerId,
    publication_id,
    source_mode,
    analysis_source: d.analysis_source ?? null,
    metric_date,
    source_run_id: source_run_id != null ? String(source_run_id) : null,
    pipeline_run_id: pipeline.pipeline_run_id ?? d.pipeline_run_id ?? null,
    dashboard_run_id: pipeline.dashboard_run_id ?? null,
    actions_run_id: pipeline.actions_run_id ?? null,
    data_pack_hash: pipeline.data_pack_hash ?? null,
    me_seller_account_id: me?.seller_account_id ?? me?.seller_id ?? null,
  };
}

function countPngs(root) {
  if (!fsSync.existsSync(root)) return 0;
  let n = 0;
  const walk = (dir) => {
    for (const name of fsSync.readdirSync(dir)) {
      const full = path.join(dir, name);
      if (fsSync.statSync(full).isDirectory()) walk(full);
      else if (name.endsWith(".png")) n++;
    }
  };
  walk(root);
  return n;
}

async function writeJson(filePath, data) {
  await fs.mkdir(path.dirname(filePath), { recursive: true });
  await fs.writeFile(filePath, JSON.stringify(data, null, 2) + "\n", "utf8");
}

async function writeText(filePath, text) {
  await fs.mkdir(path.dirname(filePath), { recursive: true });
  await fs.writeFile(filePath, text == null ? "" : String(text), "utf8");
}

async function writeVisibleContentMd(outDir) {
  const files = [
    "dashboard.txt",
    "actions.txt",
    "action-details.txt",
    "ai-chat-context.txt",
    "navigation.txt",
  ];
  const parts = ["# Visible content", ""];
  for (const f of files) {
    const p = path.join(outDir, "content", f);
    let body = "";
    try {
      body = await fs.readFile(p, "utf8");
    } catch {
      body = "";
    }
    parts.push(`## ${f}`, "", sanitizeText(body).trim() || "_empty_", "");
  }
  await writeText(path.join(outDir, "visible-content.md"), parts.join("\n"));
}

async function writeMetricsReconciliation(outDir, ctx) {
  const lines = [
    "# Metrics reconciliation",
    "",
    "Factual listing of captured metrics. No automatic correctness judgment.",
    "",
    "## Identifiers",
    "",
    `- publication_id: ${ctx.publicationId ?? "null"}`,
    `- source_run_id: ${ctx.sourceRunId ?? "null"}`,
    `- source_mode: ${ctx.sourceMode ?? "null"}`,
    `- metric_date: ${ctx.metricDate ?? "null"}`,
    "",
    "## Dashboard fields",
    "",
  ];

  const d = ctx.dashboard || {};
  const list = [
    ["status", d.status],
    ["health_score", d.ai_diagnostics?.account_summary?.health_score],
    ["health_status_label", d.ai_diagnostics?.account_summary?.health_status_label],
    ["top_actions_count", (d.top_actions || d.ai_diagnostics?.top_actions || [])?.length],
    ["what_changed_summary", d.ai_diagnostics?.what_changed?.summary ? "present" : "empty"],
    ["sales_funnel", d.ai_diagnostics?.sales_funnel || d.sales_funnel ? "present" : "empty"],
    ["ad_efficiency", d.ai_diagnostics?.ad_efficiency || d.advertising ? "present" : "empty"],
    ["stock_coverage", d.ai_diagnostics?.stock_coverage || d.stock_coverage ? "present" : "empty"],
    ["sku_focus", d.ai_diagnostics?.sku_focus || d.sku_focus ? "present" : "empty"],
  ];
  for (const [k, v] of list) {
    lines.push(`- ${k}: ${v == null || v === "" ? "GAP" : v}`);
  }

  const actions = extractActionItems(ctx.actions);
  lines.push("", "## Actions", "", `- count: ${actions.length}`);
  if (!actions.length) lines.push("- GAP: actions list empty");
  for (const a of actions.slice(0, 12)) {
    lines.push(`- ${a.action_key || a.id || "?"}: ${a.title || a.name || a.headline || "(no title)"}`);
  }

  const details = Array.isArray(ctx.actionDetails) ? ctx.actionDetails : [];
  lines.push("", "## Action details", "", `- count: ${details.length}`);
  if (details.length < Math.min(5, actions.length)) {
    lines.push(`- GAP: fewer details (${details.length}) than expected sample`);
  }

  if (ctx.notes?.length) {
    lines.push("", "## Capture notes", "");
    for (const n of ctx.notes) lines.push(`- ${n}`);
  }
  lines.push("");
  await writeText(path.join(outDir, "metrics-reconciliation.md"), lines.join("\n"));
}
