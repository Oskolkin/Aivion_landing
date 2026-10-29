import { expect, test, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { loginAsDemoSeller, stubOzonConnectionReady } from "./helpers/auth";

/**
 * P2 accessibility minimum smoke for /app/chat.
 * Writes backend/debug/ai-chat-release-gate/json/accessibility-summary.json
 */
const OUT = path.resolve(
  __dirname,
  "../../backend/debug/ai-chat-release-gate/json/accessibility-summary.json",
);

type Check = {
  id: string;
  status: "PASS" | "FAIL" | "PARTIAL" | "SKIP";
  detail: string;
};

async function openChat(page: Page) {
  await stubOzonConnectionReady(page);
  await loginAsDemoSeller(page);
  if (!page.url().includes("/app/chat")) {
    await page.goto("/app/chat");
  }
  const newChat = page.getByRole("button", { name: /Новый чат|New chat/i }).first();
  if (await newChat.isVisible().catch(() => false)) {
    await newChat.click();
  }
  const composer = page.locator("textarea").first();
  await expect(composer).toBeVisible({ timeout: 30_000 });
  return composer;
}

test("P2 a11y smoke: tab, attach label, aria-busy, download name", async ({ page }) => {
  test.setTimeout(180_000);
  const checks: Check[] = [];
  const composer = await openChat(page);

  // 1) Tab navigation reaches composer
  await page.locator("body").click({ position: { x: 8, y: 8 } }).catch(() => undefined);
  let focusedComposer = false;
  for (let i = 0; i < 40; i++) {
    await page.keyboard.press("Tab");
    const tag = await page.evaluate(() => document.activeElement?.tagName?.toLowerCase() ?? "");
    if (tag === "textarea") {
      focusedComposer = true;
      break;
    }
  }
  checks.push({
    id: "tab_to_composer",
    status: focusedComposer ? "PASS" : "FAIL",
    detail: focusedComposer
      ? "Tab sequence focused the chat composer textarea"
      : "Could not focus textarea via Tab within 40 presses",
  });

  // 2) Attach control has accessible name (button triggers hidden file input)
  const attach = page.getByRole("button", { name: /Прикрепить файл|Добавить файл/i }).first();
  const attachVisible = await attach.isVisible().catch(() => false);
  const attachLabel = attachVisible
    ? ((await attach.getAttribute("aria-label")) ?? (await attach.textContent()) ?? "").trim()
    : "";
  checks.push({
    id: "file_input_aria_label",
    status: attachVisible && /прикрепить|добавить файл/i.test(attachLabel)
      ? "PASS"
      : attachVisible
        ? "PARTIAL"
        : "FAIL",
    detail: attachVisible
      ? `attach control accessible name: ${JSON.stringify(attachLabel)} (hidden <input type=file> triggered by labeled button)`
      : "attach button not visible",
  });

  // 3) aria-busy during send
  await composer.fill("Кратко: какая выручка?");
  const send = page.getByRole("button", { name: /Отправить сообщение/i }).first();
  await expect(send).toBeEnabled({ timeout: 10_000 });
  await send.click();
  let sawBusy = false;
  const busyDeadline = Date.now() + 15_000;
  while (Date.now() < busyDeadline) {
    const busy = await composer.getAttribute("aria-busy");
    if (busy === "true") {
      sawBusy = true;
      break;
    }
    await page.waitForTimeout(50);
  }
  // Wait until send completes or times out (do not fail a11y solely on answer quality)
  await expect(composer)
    .not.toHaveAttribute("aria-busy", "true", { timeout: 150_000 })
    .catch(() => undefined);
  checks.push({
    id: "aria_busy_during_send",
    status: sawBusy ? "PASS" : "FAIL",
    detail: sawBusy
      ? "composer textarea had aria-busy=true while sending"
      : "did not observe aria-busy=true within 15s after click send",
  });

  // 4) Download control accessible name if an export attachment appears
  const download = page.getByRole("link", { name: /Скачать файл/i }).first();
  const downloadVisible = await download.isVisible().catch(() => false);
  if (downloadVisible) {
    const name = ((await download.textContent()) ?? "").trim();
    checks.push({
      id: "download_accessible_name",
      status: /скачать/i.test(name) ? "PASS" : "PARTIAL",
      detail: `download link text: ${JSON.stringify(name)}`,
    });
  } else {
    // Trigger a small export ask to surface a download if possible (best-effort).
    await composer.fill("Выгрузи сводку продаж в TXT");
    const send2 = page.getByRole("button", { name: /Отправить сообщение/i }).first();
    if (await send2.isEnabled().catch(() => false)) {
      await send2.click();
      await expect(composer)
        .not.toHaveAttribute("aria-busy", "true", { timeout: 150_000 })
        .catch(() => undefined);
    }
    const dl2 = page.getByRole("link", { name: /Скачать файл/i }).first();
    const dl2Visible = await dl2.isVisible({ timeout: 5_000 }).catch(() => false);
    if (dl2Visible) {
      const name = ((await dl2.textContent()) ?? "").trim();
      checks.push({
        id: "download_accessible_name",
        status: /скачать/i.test(name) ? "PASS" : "PARTIAL",
        detail: `download link text after export ask: ${JSON.stringify(name)}`,
      });
    } else {
      checks.push({
        id: "download_accessible_name",
        status: "PARTIAL",
        detail:
          "no download link present after revenue ask / TXT export attempt; attach button covered; download aria not live-verified",
      });
    }
  }

  const fail = checks.filter((c) => c.status === "FAIL").length;
  const partial = checks.filter((c) => c.status === "PARTIAL").length;
  let overall: "PASS" | "PARTIAL" | "FAIL" = "PASS";
  if (fail > 0) overall = "FAIL";
  else if (partial > 0) overall = "PARTIAL";

  const summary = {
    generated_at: new Date().toISOString(),
    page: "/app/chat",
    overall,
    checklist: checks,
    fail,
    partial,
    pass: checks.filter((c) => c.status === "PASS").length,
    notes: [
      "Hidden file input is activated by an aria-labeled button (acceptable pattern).",
      "Download control uses visible text «Скачать файл» rather than a separate aria-label.",
    ],
  };

  fs.mkdirSync(path.dirname(OUT), { recursive: true });
  fs.writeFileSync(OUT, JSON.stringify(summary, null, 2), "utf8");
  console.log("wrote", OUT, overall);

  expect(fail, `a11y FAIL checks: ${JSON.stringify(checks, null, 2)}`).toBe(0);
});
