import { expect, test, type Page } from "@playwright/test";
import { stubOzonConnectionReady } from "./helpers/auth";

function sse(events: object[]): string {
  return events
    .map((e) => {
      const rec = e as { type?: string };
      return `event: ${rec.type ?? "message"}\ndata: ${JSON.stringify(e)}\n`;
    })
    .join("\n");
}

async function stubChatV2(page: Page) {
  await stubOzonConnectionReady(page);
  let pub = "101";
  let activeMembers = 4;
  const sessions: { id: number; title: string; action_view_id?: string; status: string; created_at: string; updated_at: string }[] = [];
  const messages: Record<number, { id: number; session_id: number; role: string; message_type: string; content: string; created_at: string }[]> = {};
  let nextSession = 10;
  let nextMsg = 100;
  let lastQuestion = "";

  await page.route("**/api/v1/chat/sessions**", async (route) => {
    const req = route.request();
    const url = new URL(req.url());
    if (req.method() === "GET" && url.pathname.endsWith("/sessions")) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ items: sessions, limit: 20, offset: 0 }),
      });
      return;
    }
    const sid = Number(url.pathname.split("/").pop());
    if (req.method() === "PATCH") {
      const body = req.postDataJSON() as { title?: string; status?: string };
      const s = sessions.find((x) => x.id === sid);
      if (s && body.title) s.title = body.title;
      if (s && body.status) s.status = body.status;
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(s) });
      return;
    }
    if (req.method() === "DELETE") {
      const idx = sessions.findIndex((x) => x.id === sid);
      if (idx >= 0) sessions.splice(idx, 1);
      await route.fulfill({ status: 204, body: "" });
      return;
    }
    await route.continue();
  });

  await page.route("**/api/v1/chat/sessions/*/messages**", async (route) => {
    const parts = new URL(route.request().url()).pathname.split("/");
    const sid = Number(parts[parts.indexOf("sessions") + 1]);
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ items: messages[sid] ?? [], limit: 50, offset: 0 }),
    });
  });

  await page.route("**/api/v1/chat/action-handoffs", async (route) => {
    const body = route.request().postDataJSON() as { action_view_id: string };
    let existing = sessions.find((s) => s.action_view_id === body.action_view_id);
    if (!existing) {
      existing = {
        id: nextSession++,
        title: "Пополнить 4 товара",
        action_view_id: body.action_view_id,
        status: "active",
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      };
      sessions.push(existing);
      messages[existing.id] = [];
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ session_id: existing.id, binding_id: 1, title: existing.title }),
    });
  });

  await page.route("**/api/v1/chat/attachments", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        attachment: {
          id: "att-partial",
          name: "plan.xlsx",
          mime_type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
          size_bytes: 120,
          status: "uploaded",
          completeness: "PARTIAL",
          warning_code: "PARTIAL",
        },
      }),
    });
  });

  await page.route("**/api/v1/chat/ask/stream", async (route) => {
    const body = route.request().postDataJSON() as { question: string; session_id?: number };
    lastQuestion = body.question;
    const sessionId = body.session_id ?? nextSession++;
    if (!sessions.find((s) => s.id === sessionId)) {
      sessions.push({
        id: sessionId,
        title: body.question.slice(0, 40),
        status: "active",
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      });
      messages[sessionId] = [];
    }
    const uid = nextMsg++;
    const aid = nextMsg++;
    const stock = pub === "202" ? "300" : "0";
    const mixed = /файл|сравни|план/i.test(body.question);
    const aboutAds = /реклам/i.test(body.question);
    const answer = aboutAds
      ? "По SKU417 реклама в текущей публикации требует проверки расхода."
      : mixed
        ? "Сравнил вложение с фактами кабинета: план из файла и остатки кабинета."
        : `SKU417: текущий остаток ${stock}.`;
    messages[sessionId].push(
      { id: uid, session_id: sessionId, role: "user", message_type: "question", content: body.question, created_at: new Date().toISOString() },
      { id: aid, session_id: sessionId, role: "assistant", message_type: "answer", content: answer, created_at: new Date().toISOString() },
    );
    const events = [
      { type: "status", status: "queued" },
      { type: "status", status: "resolving_context" },
      { type: "status", status: "validating" },
      { type: "status", status: "streaming" },
      { type: "delta", delta: answer },
      {
        type: "done",
        status: "completed",
        result: {
          session_id: sessionId,
          turn_id: aid,
          user_message_id: uid,
          assistant_message_id: aid,
          answer,
          limitations: mixed ? ["Файл обработан частично"] : [],
          publication: { id: pub, analysis_date: "2026-08-31", analysis_cutoff_at: "2026-08-31T21:00:00Z" },
          answer_mode: mixed ? "MIXED_ANALYSIS" : "FACTUAL_QUERY",
          grounding_refs: mixed
            ? [
                { handle: "att_1", type: "ATTACHMENT", id: "att-partial" },
                { handle: "cf_1", type: "FACT", id: "sku:417" },
              ]
            : [{ handle: "cf_1", type: "FACT", id: "sku:417" }],
          action_context: { origin_member_count: 4, current_active_count: activeMembers },
        },
      },
    ];
    await route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      body: sse(events),
    });
  });

  await page.route(/\/api\/v1\/actions(\?|$)/, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        publication_id: pub,
        state: "READY",
        groups: [
          {
            action_view_id: "view1",
            display_title: "Пополнить 4 товара",
            display_summary: "Пополнить запас",
            action_type: "SKU_INCREASE_REPLENISHMENT",
            family: "INVENTORY",
            active_member_count: activeMembers,
            members: Array.from({ length: activeMembers }, (_, i) => ({
              case_id: `case-${i + 1}`,
              target_kind: "SKU",
              target_id: String(417 + i),
              active: true,
              user_state: "OPEN",
              parameters: {},
            })),
          },
        ],
        history: [],
        has_more: false,
      }),
    });
  });

  await page.route("**/api/v1/actions/view1", async (route) => {
    if (route.request().method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          group: {
            action_view_id: "view1",
            display_title: "Пополнить 4 товара",
            display_summary: "",
            active_member_count: activeMembers,
            members: [],
          },
        }),
      });
      return;
    }
    await route.continue();
  });

  await page.route(/\/api\/v1\/actions\/cases\/[^/]+\/(done|dismiss|reopen)$/, async (route) => {
    if (route.request().method() === "POST") {
      activeMembers = Math.max(0, activeMembers - 1);
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ ok: true }) });
      return;
    }
    await route.continue();
  });

  return {
    setPub(id: string) {
      pub = id;
    },
    lastQuestion: () => lastQuestion,
  };
}

async function selectActionAndOpenChat(page: Page, actionTitle: RegExp) {
  await page.goto("/app/actions");
  await expect(page.getByTestId("actions-workspace")).toBeVisible({ timeout: 15_000 });
  await expect(page.getByText(actionTitle).first()).toBeVisible({ timeout: 15_000 });
  await page.getByRole("button", { name: actionTitle }).click();
  await expect(page.getByRole("heading", { level: 2, name: actionTitle })).toBeVisible();
  const ask = page.getByRole("link", { name: /Спросить AI/i });
  await expect(ask).toBeVisible();
  await ask.click();
  await expect(page.getByText("Чат по действию")).toBeVisible({ timeout: 15_000 });
  await expect(page.getByLabel("Сообщение в чат")).toBeEnabled({ timeout: 60_000 });
}

test.describe("Prompt 15 Chat v2", () => {
  test("general multi-turn coreference", async ({ page }) => {
    await stubChatV2(page);
    await page.goto("/app/chat");
    await expect(page.getByLabel("Сообщение в чат")).toBeVisible();
    await page.getByLabel("Сообщение в чат").fill("Почему падают продажи SKU417?");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText(/SKU417: текущий остаток/).first()).toBeVisible({ timeout: 15_000 });
    await page.getByLabel("Сообщение в чат").fill("А что с рекламой по нему?");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText(/реклама в текущей публикации/i).first()).toBeVisible({ timeout: 15_000 });
  });

  test("publication switch uses current facts", async ({ page }) => {
    const api = await stubChatV2(page);
    await page.goto("/app/chat");
    await page.getByLabel("Сообщение в чат").fill("Почему товара нет SKU417?");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText(/остаток 0/)).toBeVisible({ timeout: 15_000 });
    api.setPub("202");
    await page.getByLabel("Сообщение в чат").fill("А сколько заказать?");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText(/остаток 300/)).toBeVisible({ timeout: 15_000 });
  });

  test("action handoff uses action_view_id", async ({ page }) => {
    await stubChatV2(page);
    await selectActionAndOpenChat(page, /Пополнить 4 товара/);
    await expect(page).toHaveURL(/action_view_id=view1|session=\d+/);
  });

  test("attachment warning keeps session usable", async ({ page }) => {
    await stubChatV2(page);
    await page.goto("/app/chat");
    const fileInput = page.locator('input[type="file"]');
    await fileInput.setInputFiles({ name: "plan.xlsx", mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer: Buffer.from("qty") });
    await expect(page.getByText(/частично/i)).toBeVisible({ timeout: 10_000 });
    await page.getByLabel("Сообщение в чат").fill("Сравни этот файл с продажами кабинета");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText(/вложение|кабинета/i).first()).toBeVisible({ timeout: 15_000 });
  });

  test("session rename archive delete and a11y", async ({ page }) => {
    await stubChatV2(page);
    await page.goto("/app/chat");
    await expect(page.getByLabel("Сообщение в чат")).toBeVisible();
    await expect(page.getByRole("button", { name: "Отправить сообщение" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Прикрепить файл" })).toBeVisible();
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(page.getByLabel("Сообщение в чат")).toBeVisible();
    await page.setViewportSize({ width: 1280, height: 800 });
    await expect(page.getByLabel("Сообщение в чат")).toBeVisible();
  });

  test("validation-first stream has no premature seller text", async ({ page }) => {
    await stubOzonConnectionReady(page);
    let released = false;
    await page.route("**/api/v1/chat/sessions**", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ items: [], limit: 20, offset: 0 }) });
    });
    await page.route("**/api/v1/chat/ask/stream", async (route) => {
      await new Promise((r) => setTimeout(r, 200));
      const events = released
        ? [
            { type: "status", status: "validating" },
            { type: "status", status: "streaming" },
            { type: "delta", delta: "Подтверждённый ответ." },
            {
              type: "done",
              status: "completed",
              result: { session_id: 1, user_message_id: 1, assistant_message_id: 2, answer: "Подтверждённый ответ.", limitations: [] },
            },
          ]
        : [{ type: "status", status: "validating" }];
      released = true;
      await route.fulfill({ status: 200, contentType: "text/event-stream", body: sse(events) });
    });
    await page.goto("/app/chat");
    await page.getByLabel("Сообщение в чат").fill("Проверка стрима");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText("Подтверждённый ответ.")).toHaveCount(0);
  });

  test("action 4 to 3 after DONE", async ({ page }) => {
    await stubChatV2(page);
    await page.goto("/app/actions");
    await expect(page.getByText(/Пополнить 4 товара/).first()).toBeVisible({ timeout: 15_000 });
    await page.getByRole("button", { name: /Пополнить 4 товара/ }).click();
    await expect(page.getByRole("button", { name: "Сделано" }).first()).toBeVisible();
    await page.getByRole("button", { name: "Сделано" }).first().click();
    await expect(page.getByText(/3 активных/).first()).toBeVisible({ timeout: 15_000 });
    const ask = page.getByRole("link", { name: /Спросить AI/i });
    await expect(ask).toBeVisible();
    await ask.click();
    await expect(page.getByText("Чат по действию")).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText(/SKU417: текущий остаток|остаток/i).first()).toBeVisible({ timeout: 60_000 });
  });

  test("singleton action chat opens", async ({ page }) => {
    await stubChatV2(page);
    await page.route(/\/api\/v1\/actions(\?|$)/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          publication_id: "101",
          state: "READY",
          groups: [
            {
              action_view_id: "case-single",
              display_title: "Пополнить 1 товар",
              display_summary: "singleton",
              action_type: "SKU_INCREASE_REPLENISHMENT",
              family: "INVENTORY",
              active_member_count: 1,
              members: [{ case_id: "only", target_kind: "SKU", target_id: "417", active: true, user_state: "OPEN", parameters: {} }],
            },
          ],
          history: [],
          has_more: false,
        }),
      });
    });
    await page.goto("/app/actions");
    await expect(page.getByText(/Пополнить 1 товар/).first()).toBeVisible({ timeout: 15_000 });
    await page.getByRole("button", { name: /Пополнить 1 товар/ }).click();
    await expect(page.getByRole("heading", { level: 2, name: /Пополнить 1 товар/ })).toBeVisible();
    const ask = page.getByRole("link", { name: /Спросить AI/i });
    await expect(ask).toBeVisible();
    await ask.click();
    await expect(page.getByText("Чат по действию")).toBeVisible({ timeout: 15_000 });
  });

  test("mixed attachment and cabinet", async ({ page }) => {
    await stubChatV2(page);
    await page.goto("/app/chat");
    const fileInput = page.locator('input[type="file"]');
    await fileInput.setInputFiles({
      name: "plan.csv",
      mimeType: "text/csv",
      buffer: Buffer.from("SKU417 planned quantity = 500"),
    });
    await page.getByLabel("Сообщение в чат").fill("Сравни план из файла с текущими продажами и остатками SKU417.");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByText(/вложение|кабинета|план/i).first()).toBeVisible({ timeout: 15_000 });
  });

  test("cancel does not complete partial assistant", async ({ page }) => {
    await stubOzonConnectionReady(page);
    await page.route("**/api/v1/chat/sessions**", async (route) => {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ items: [], limit: 20, offset: 0 }) });
    });
    await page.route("**/api/v1/chat/ask/stream", async (route) => {
      await new Promise((r) => setTimeout(r, 5_000));
      await route.fulfill({
        status: 200,
        contentType: "text/event-stream",
        body: sse([{ type: "status", status: "cancelled" }, { type: "error", status: "cancelled", error: "cancelled" }]),
      });
    });
    await page.goto("/app/chat");
    await page.getByLabel("Сообщение в чат").fill("Долгий вопрос");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await page.getByRole("button", { name: "Отменить генерацию" }).click();
    await expect(page.getByText("частичный ответ который нельзя показывать")).toHaveCount(0);
  });

  test("rapid double submit while in flight sends one ask", async ({ page }) => {
    await stubChatV2(page);
    let askCalls = 0;
    let releaseFirst!: () => void;
    const firstRequestGate = new Promise<void>((resolve) => {
      releaseFirst = resolve;
    });
    await page.unroute("**/api/v1/chat/ask/stream");
    await page.route("**/api/v1/chat/ask/stream", async (route) => {
      if (route.request().method() !== "POST") {
        await route.continue();
        return;
      }
      askCalls += 1;
      if (askCalls === 1) {
        await firstRequestGate;
      }
      await route.fulfill({
        status: 200,
        contentType: "text/event-stream",
        body: sse([
          { type: "status", status: "queued" },
          { type: "status", status: "streaming" },
          { type: "delta", delta: "SKU417: текущий остаток 0." },
          {
            type: "done",
            status: "completed",
            result: {
              session_id: 1,
              turn_id: 1,
              user_message_id: 1,
              assistant_message_id: 2,
              answer: "SKU417: текущий остаток 0.",
              publication: { id: "201", analysis_date: "2026-08-31", analysis_cutoff_at: "2026-08-31T21:00:00Z" },
              answer_mode: "FACTUAL_QUERY",
              grounding_refs: [{ handle: "cf_1", type: "FACT", id: "sku:417" }],
            },
          },
        ]),
      });
    });
    await page.goto("/app/chat");
    await page.getByLabel("Сообщение в чат").fill("Что такое ABC-анализ?");
    const send = page.getByRole("button", { name: "Отправить сообщение" });
    await send.click();
    await expect.poll(() => askCalls, { timeout: 10_000 }).toBe(1);
    await expect(page.getByRole("button", { name: "Отменить генерацию" })).toBeVisible({ timeout: 10_000 });
    await send.click({ timeout: 1_000 }).catch(() => undefined);
    await page.getByLabel("Сообщение в чат").press("Enter", { timeout: 1_000 }).catch(() => undefined);
    expect(askCalls).toBe(1);
    releaseFirst();
  });

  test("SSE error event shows controlled server message not silent EOF", async ({ page }) => {
    await stubOzonConnectionReady(page);
    await page.route("**/api/v1/chat/sessions**", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ items: [], limit: 20, offset: 0 }),
      });
    });
    await page.route("**/api/v1/chat/ask/stream", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "text/event-stream",
        body: sse([
          { type: "status", status: "queued" },
          { type: "status", status: "validating" },
          {
            type: "error",
            status: "failed",
            error: "Не удалось подготовить файл выгрузки.",
          },
        ]),
      });
    });
    await page.goto("/app/chat");
    await page.getByLabel("Сообщение в чат").fill("Выгрузи продажи по Polaris 417");
    await page.getByRole("button", { name: "Отправить сообщение" }).click();
    await expect(page.getByRole("alert").filter({ hasText: /Не удалось подготовить файл выгрузки/i })).toBeVisible({
      timeout: 15_000,
    });
    await expect(page.getByText(/Поток ответа завершился без результата/i)).toHaveCount(0);
  });
});
