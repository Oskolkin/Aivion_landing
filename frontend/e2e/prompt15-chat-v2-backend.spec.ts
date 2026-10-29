import { expect, test, type Page } from "@playwright/test";
import { loginAsDemoSeller } from "./helpers/auth";
import {
  loadPrompt15E2EManifest,
  switchAnalysisCurrentPublication,
  type Prompt15E2EManifest,
} from "./helpers/seed";
import path from "node:path";
import os from "node:os";
import fs from "node:fs";

const backendUrl = process.env.BACKEND_PROXY_URL ?? "http://localhost:8081";

/**
 * Backend-backed Prompt 15 Chat v2.
 * Does not mock /api/v1/chat/* or /api/v1/actions.
 * Requires running API (AI_PROVIDER_MODE=fake) + Prompt15 E2E fixture from global-setup.
 *
 *   E2E_CHAT_BACKEND=1 npx playwright test e2e/prompt15-chat-v2-backend.spec.ts
 */
test.describe("Prompt 15 Chat v2 backend-backed", () => {
  test.skip(!process.env.E2E_CHAT_BACKEND, "set E2E_CHAT_BACKEND=1 for real API E2E");

  let manifest: Prompt15E2EManifest;

  test.beforeAll(() => {
    manifest = loadPrompt15E2EManifest();
    if (manifest.active_action_count < 2 || manifest.completed_action_count < 1) {
      throw new Error(
        `Actions fixture empty: active=${manifest.active_action_count} completed=${manifest.completed_action_count}`,
      );
    }
  });

  test.beforeEach(async () => {
    switchAnalysisCurrentPublication(manifest.current_publication_id, manifest.seller_account_id);
  });

  test.beforeEach(async ({ page }) => {
    await loginAsDemoSeller(page);
  });

  test("actions API preflight group singleton historical", async ({ request }) => {
    const active = await request.get(`${backendUrl}/api/v1/actions?status=active`);
    expect(active.ok(), `active ${active.status()}`).toBeTruthy();
    const activeBody = (await active.json()) as { groups?: { action_view_id?: string }[] };
    const ids = (activeBody.groups ?? []).map((g) => g.action_view_id);
    expect(ids).toContain(manifest.group_action_view_id);
    expect(ids).toContain(manifest.singleton_action_view_id);
    const done = await request.get(`${backendUrl}/api/v1/actions?status=done`);
    expect(done.ok(), `done ${done.status()}`).toBeTruthy();
    const doneBody = (await done.json()) as {
      groups?: { action_view_id?: string }[];
      history?: { action_view_id?: string }[];
    };
    const doneIds = [
      ...(doneBody.groups ?? []).map((g) => g.action_view_id),
      ...(doneBody.history ?? []).map((g) => g.action_view_id),
    ];
    expect(doneIds).toContain(manifest.historical_action_view_id);
  });

  test("A general Chat ask reaches real backend", async ({ page }) => {
    const chatCalls: string[] = [];
    page.on("request", (req) => {
      const u = req.url();
      if (u.includes("/api/v1/chat/")) chatCalls.push(`${req.method()} ${u}`);
    });
    await page.goto("/app/chat");
    await expect(page.getByLabel("Сообщение в чат")).toBeVisible({ timeout: 30_000 });
    await page.getByLabel("Сообщение в чат").fill("Сколько остатков?");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText(/остат|SKU|кабинет|уточните|факт|Не удалось подготовить/i).first()).toBeVisible({
      timeout: 90_000,
    });
    expect(chatCalls.some((c) => c.includes("/ask"))).toBeTruthy();
    expect(chatCalls.every((c) => !c.includes("mock"))).toBeTruthy();
  });

  test("B real multi-turn follow-up on one session", async ({ page }) => {
    await page.goto("/app/chat");
    await expect(page.getByLabel("Сообщение в чат")).toBeVisible({ timeout: 30_000 });
    await page.getByLabel("Сообщение в чат").fill("Почему падают продажи?");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText(/продаж|остат|кабинет|уточните|факт/i).first()).toBeVisible({ timeout: 90_000 });
    const urlAfterFirst = page.url();
    await page.getByLabel("Сообщение в чат").fill("А что с рекламой по нему?");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText(/реклам|кабинет|уточните|факт|товар/i).nth(1)).toBeVisible({ timeout: 90_000 });
    if (urlAfterFirst.includes("session=")) {
      expect(page.url()).toContain(new URL(urlAfterFirst, "http://localhost").searchParams.get("session") ?? "session=");
    }
  });

  test("C real P1 then P2 publication switch for SKU417 stock", async ({ page }) => {
    switchAnalysisCurrentPublication(manifest.p1_publication_id, manifest.seller_account_id);
    await sendChat(page, "Какой остаток у Polaris 417?");
    await expect(page.getByTestId("chat-assistant-message").nth(0)).toContainText("E2E_STOCK=0", {
      timeout: 90_000,
    });

    switchAnalysisCurrentPublication(manifest.p2_publication_id, manifest.seller_account_id);
    await page.getByLabel("Сообщение в чат").fill("А сколько сейчас?");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByTestId("chat-assistant-message").nth(1)).toContainText(/E2E_STOCK=300/, { timeout: 90_000 });
    await expect(page.getByTestId("chat-assistant-message").nth(0)).toContainText("E2E_STOCK=0");
    await expect(page.getByTestId("chat-assistant-message").nth(1)).not.toContainText("E2E_STOCK=0");
  });

  test("D Actions Спросить AI opens Action Chat via real handoff", async ({ page }) => {
    const handoffs: string[] = [];
    page.on("request", (req) => {
      if (req.url().includes("/api/v1/chat/action-handoffs")) handoffs.push(req.url());
    });
    await selectActionAndAsk(page, manifest.group_title);
    await expect(page).toHaveURL(/\/app\/chat/, { timeout: 60_000 });
    await expect(page.getByText("Чат по действию")).toBeVisible({ timeout: 30_000 });
    await expect(page.getByRole("heading", { name: manifest.group_title })).toBeVisible({ timeout: 30_000 });
    expect(handoffs.length).toBeGreaterThan(0);
  });

  test("E Action 4 to 3 after real DONE", async ({ page }) => {
    await page.goto("/app/actions");
    await expect(page.getByTestId("actions-list").first()).toBeVisible({ timeout: 30_000 });
    await page.getByRole("button", { name: new RegExp(manifest.group_title, "i") }).first().click();
    await expect(page.getByTestId("actions-detail").first()).toBeVisible();
    const done = page.getByRole("button", { name: "Сделано" }).first();
    await expect(done).toBeVisible({ timeout: 15_000 });
    await done.click();
    await expect(page.getByText(/3 активных/).first()).toBeVisible({ timeout: 30_000 });
    const askAfterDone = page.getByTestId("action-ask-ai").first();
    await expect(askAfterDone).toBeVisible({ timeout: 15_000 });
    await expect(page.getByTestId("actions-workspace")).not.toHaveAttribute("aria-busy", "true");
    await askAfterDone.click();
    await expect(page).toHaveURL(/\/app\/chat/, { timeout: 60_000 });
    await page.getByLabel("Сообщение в чат").fill("Что осталось сделать?");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText(/E2E_ACTION origin=4 current_active=3/i).first()).toBeVisible({ timeout: 90_000 });
  });

  test("F singleton Action Chat handoff", async ({ page }) => {
    await selectActionAndAsk(page, manifest.singleton_title);
    await expect(page).toHaveURL(/\/app\/chat/, { timeout: 60_000 });
    await page.getByLabel("Сообщение в чат").fill("Помоги выполнить это действие");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText(/E2E_ACTION origin=1 current_active=1/i).first()).toBeVisible({ timeout: 90_000 });
  });

  test("G historical Action retains origin publication", async ({ page }) => {
    await page.goto("/app/actions");
    const histTab = page.getByRole("tab", { name: /Выполнен/i }).first();
    await expect(histTab).toBeVisible({ timeout: 15_000 });
    await histTab.click();
    await expect(page.getByTestId("actions-list").first()).toBeVisible({ timeout: 30_000 });
    await page.getByRole("button", { name: new RegExp(manifest.historical_title, "i") }).first().click();
    await page.getByRole("link", { name: /Спросить AI/i }).first().click();
    await expect(page).toHaveURL(/\/app\/chat/, { timeout: 60_000 });
    await page.getByLabel("Сообщение в чат").fill("Почему Aivion тогда рекомендовал 1990?");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    const origin = manifest.historical_origin_publication_id;
    const current = manifest.historical_current_publication_id;
    await expect(page.getByTestId("chat-assistant-message").last()).toContainText(
      new RegExp(`E2E_HISTORICAL origin_pub=${origin} current_pub=${current}`, "i"),
      { timeout: 90_000 },
    );
    await expect(page.getByTestId("chat-assistant-message").last()).toContainText(/recommended_price=1990/i);
  });

  test("H attachment plus cabinet question uses real upload and ask", async ({ page }) => {
    await page.goto("/app/chat");
    await expect(page.getByLabel("Сообщение в чат")).toBeVisible({ timeout: 30_000 });
    const tmp = path.join(os.tmpdir(), "prompt15-plan.csv");
    fs.writeFileSync(tmp, "sku,qty\n417,500\n", "utf8");
    const fileInput = page.locator('input[type="file"]');
    if (await fileInput.count()) {
      await fileInput.first().setInputFiles(tmp);
      await expect(page.getByText(/prompt15-plan\.csv|plan\.csv/i).first()).toBeVisible({ timeout: 30_000 });
    }
    await page.getByLabel("Сообщение в чат").fill("Сравни план из файла с продажами и остатками.");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText(/E2E_MIXED attachment\+cabinet/i).first()).toBeVisible({ timeout: 90_000 });
  });

  test("I cancel in-flight generation leaves no assistant message", async ({ page }) => {
    await page.goto("/app/chat");
    await expect(page.getByLabel("Сообщение в чат")).toBeVisible({ timeout: 30_000 });
    await page.getByLabel("Сообщение в чат").fill("Подожди долго и расскажи про остатки Polaris 417");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    const stop = page.getByRole("button", { name: /Отменить генерацию|Остановить|Стоп/i });
    await expect(stop).toBeVisible({ timeout: 15_000 });
    await stop.click();
    await page.reload();
    await expect(page.getByLabel("Сообщение в чат")).toBeVisible({ timeout: 30_000 });
    const aivionAnswers = page.locator("article").filter({ has: page.getByText("Aivion", { exact: true }) });
    await expect(aivionAnswers).toHaveCount(0, { timeout: 15_000 });
  });

  test("J real duplicate client_request_id is idempotent", async ({ page }) => {
    await page.goto("/app/chat");
    await expect(page.getByLabel("Сообщение в чат")).toBeVisible({ timeout: 30_000 });
    const cookies = await page.context().cookies();
    const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join("; ");
    const clientRequestId = `e2e-idem-${Date.now()}`;
    const body = JSON.stringify({
      question: "Сколько остатков SKU417?",
      client_request_id: clientRequestId,
    });
    const headers = { "Content-Type": "application/json", Cookie: cookieHeader };
    const [r1, r2] = await Promise.all([
      page.request.post(`${backendUrl}/api/v1/chat/ask`, { headers, data: body }),
      page.request.post(`${backendUrl}/api/v1/chat/ask`, { headers, data: body }),
    ]);
    expect(r1.ok() || r2.ok()).toBeTruthy();
    const j1 = (await r1.json()) as { session_id?: number };
    const j2 = (await r2.json()) as { session_id?: number };
    const sessionId = j1.session_id ?? j2.session_id;
    expect(sessionId).toBeTruthy();
    const msgs = await page.request.get(`${backendUrl}/api/v1/chat/sessions/${sessionId}/messages`, { headers });
    expect(msgs.ok()).toBeTruthy();
    const payload = (await msgs.json()) as { items?: { role: string }[] };
    const userMsgs = (payload.items ?? []).filter((m) => m.role === "user");
    const assistantMsgs = (payload.items ?? []).filter((m) => m.role === "assistant");
    expect(userMsgs.length).toBe(1);
    expect(assistantMsgs.length).toBeLessThanOrEqual(1);
    const sess = await page.request.get(`${backendUrl}/api/v1/chat/sessions/${sessionId}`, { headers });
    expect(sess.ok()).toBeTruthy();
    const sessBody = (await sess.json()) as { turn_count?: number; last_turn_id?: number; status?: string };
    if (typeof sessBody.turn_count === "number") {
      expect(sessBody.turn_count).toBe(1);
    }
  });

  test("K export request downloads real CSV with SKU417", async ({ page }) => {
    await sendChat(page, "Выгрузи продажи по Polaris 417");
    const downloadLink = page.getByRole("link", { name: /Скачать файл|\.csv|выгруз|chat-export/i }).first();
    await expect(downloadLink).toBeVisible({ timeout: 90_000 });
    const downloadPromise = page.waitForEvent("download");
    await downloadLink.click();
    const download = await downloadPromise;
    const tmp = path.join(os.tmpdir(), download.suggestedFilename());
    await download.saveAs(tmp);
    const bytes = fs.readFileSync(tmp);
    expect(bytes.length).toBeGreaterThan(0);
    const text = bytes.toString("utf8");
    expect(text).toMatch(/Polaris|SKU417|9105|417/);
    expect(text).toMatch(/sellable_stock|ordered_units|ordered_amount/i);
  });

  test("L reload keeps export attachment on assistant message", async ({ page }) => {
    await sendChat(page, "Выгрузи продажи по Polaris 417");
    const assistantBlock = page.getByTestId("chat-assistant-message").last();
    await expect(assistantBlock.getByRole("link", { name: /Скачать файл|\.csv|выгруз|chat-export/i })).toBeVisible({
      timeout: 90_000,
    });
    const sessionUrl = page.url();
    await page.reload();
    await expect(page.getByLabel("Сообщение в чат")).toBeVisible({ timeout: 30_000 });
    expect(page.url()).toContain(new URL(sessionUrl, "http://localhost").searchParams.get("session") ?? "session=");
    const assistantAfter = page.getByTestId("chat-assistant-message").last();
    await expect(assistantAfter.getByRole("link", { name: /Скачать файл|\.csv|выгруз|chat-export/i })).toBeVisible({
      timeout: 30_000,
    });
    const userBlock = page.locator("div.flex.w-full.justify-end").last();
    await expect(userBlock.getByRole("link", { name: /Скачать файл|\.csv|выгруз|chat-export/i })).toHaveCount(0);
  });
});

async function sendChat(page: Page, text: string) {
  await page.goto("/app/chat");
  await expect(page.getByLabel("Сообщение в чат")).toBeVisible({ timeout: 30_000 });
  await page.getByLabel("Сообщение в чат").fill(text);
  await page.getByRole("button", { name: "Отправить сообщение" }).click();
}

async function selectActionAndAsk(page: Page, title: string) {
  await page.goto("/app/actions");
  await expect(page.getByTestId("actions-list").first()).toBeVisible({ timeout: 30_000 });
  await expect(page.getByTestId("actions-empty")).toHaveCount(0);
  await page.getByRole("button", { name: new RegExp(title, "i") }).first().click();
  await expect(page.getByTestId("actions-detail").first()).toBeVisible();
  const ask = page.getByRole("link", { name: /Спросить AI/i }).first();
  await expect(ask).toBeVisible({ timeout: 15_000 });
  await ask.click();
}
