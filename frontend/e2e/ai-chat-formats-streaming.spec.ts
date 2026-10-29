import { expect, test, type Locator, type Page } from "@playwright/test";
import { loginAsDemoSeller, stubOzonConnectionReady } from "./helpers/auth";
import path from "node:path";

const FIXTURES = path.resolve(__dirname, "../../backend/debug/ai-chat-audit/fixtures");

async function openFreshChat(page: Page) {
  await stubOzonConnectionReady(page);
  await loginAsDemoSeller(page);
  await page.goto("/app/chat");
  const newChat = page.getByRole("button", { name: /Новый чат|New chat/i }).first();
  if (await newChat.isVisible().catch(() => false)) {
    await newChat.click();
  }
  const composer = page.locator("textarea").first();
  await expect(composer).toBeVisible({ timeout: 30_000 });
  await expect(composer).toBeEnabled({ timeout: 30_000 });
  return composer;
}

async function expectMessageVisible(page: Page, text: string | RegExp) {
  await expect(page.getByText(text).first()).toBeVisible({ timeout: 60_000 });
}

async function waitForComposerIdle(composer: Locator) {
  await expect(composer).toBeEnabled({ timeout: 180_000 });
  await expect(composer).not.toHaveAttribute("aria-busy", "true", { timeout: 180_000 });
}

async function assertComposerInViewport(page: Page, composer: Locator) {
  await expect(composer).toBeVisible({ timeout: 30_000 });
  const box = await composer.boundingBox();
  expect(box, "composer boundingBox").toBeTruthy();
  const viewport = page.viewportSize();
  expect(viewport, "viewport size").toBeTruthy();

  expect(box!.y).toBeGreaterThanOrEqual(0);
  expect(box!.y + box!.height).toBeLessThanOrEqual(viewport!.height);

  const overflowX = await page.evaluate(() => {
    const el = document.documentElement;
    return el.scrollWidth - el.clientWidth;
  });
  expect(overflowX).toBeLessThanOrEqual(8);
}

test.describe("AI chat streaming and attachments", () => {
  test("streams a normal answer and keeps history after reload", async ({ page }) => {
    const composer = await openFreshChat(page);

    let streamRequestObserved = false;
    page.on("request", (request) => {
      if (request.method() === "POST" && /\/ask\/stream(?:\?|$)/.test(request.url())) {
        streamRequestObserved = true;
      }
    });

    const marker = `E2E-STREAM-${Date.now()}`;
    const question = `Какая выручка за период анализа в кабинете? Отметь ответ маркером ${marker}.`;
    await composer.fill(question);
    await page.getByRole("button", { name: /Отправить|Send/i }).click();

    await expectMessageVisible(page, marker);
    await waitForComposerIdle(composer);
    expect(streamRequestObserved).toBeTruthy();

    const href = page.url();
    await page.reload();
    await expect(page).toHaveURL(href);
    await expectMessageVisible(page, marker);
  });

  test("requests CSV export and exposes the generated attachment", async ({ page }) => {
    const composer = await openFreshChat(page);
    const marker = `E2E-CSV-${Date.now()}`;
    const question = `Выгрузи CSV со списком товаров с риском остатка. Упомяни ${marker}.`;
    await composer.fill(question);
    await page.getByRole("button", { name: /Отправить|Send/i }).click();
    await expectMessageVisible(page, /CSV|Скачать файл|подготовил/i);
    await expect(page.getByRole("link", { name: /Скачать файл/i }).first()).toBeVisible({
      timeout: 180_000,
    });
  });

  test("uploads txt fixture and answers control question", async ({ page }) => {
    const composer = await openFreshChat(page);

    const fileInput = page.locator('input[type="file"]').first();
    await fileInput.setInputFiles(path.join(FIXTURES, "control.txt"));
    await expect(page.getByText("control.txt").first()).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId("draft-attachment-ready").first()).toBeVisible({
      timeout: 120_000,
    });

    const question = "Какой контрольный код указан в приложенном файле? Процитируй точно.";
    await composer.fill(question);
    const send = page.getByRole("button", { name: /Отправить сообщение|Отправить|Send/i });
    await expect(send).toBeEnabled({ timeout: 60_000 });
    await send.click();
    await expectMessageVisible(page, /TXT_CONTROL_16735/);
  });
});

const MOBILE_VIEWPORTS = [
  { name: "390x844", width: 390, height: 844 },
  { name: "430x932", width: 430, height: 932 },
] as const;

for (const vp of MOBILE_VIEWPORTS) {
  test.describe(`AI chat mobile viewport ${vp.name}`, () => {
    test.use({ viewport: { width: vp.width, height: vp.height } });

    test("composer stays in viewport without horizontal overflow", async ({ page }) => {
      const composer = await openFreshChat(page);
      await assertComposerInViewport(page, composer);
    });
  });
}
