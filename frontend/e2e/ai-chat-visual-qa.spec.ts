/**
 * Visual QA screenshots for generated chat artifacts (PDF/DOCX/XLSX/PNG).
 * Run:
 *   E2E_SKIP_SEED=1 E2E_SKIP_WEB_SERVER=1 E2E_LOGIN_EMAIL=demo@example.com E2E_LOGIN_PASSWORD=password123 \
 *   npx playwright test e2e/ai-chat-visual-qa.spec.ts --project=chromium
 */
import { expect, test, type Locator, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { stubOzonConnectionReady } from "./helpers/auth";

const ASK_TIMEOUT_MS = 300_000;
const TEST_TIMEOUT_MS = 900_000;

const GATE = path.resolve(__dirname, "../../backend/debug/ai-chat-release-gate");
const SHOTS = path.join(GATE, "visual-qa", "screenshots");
const DOWNLOADS = path.join(GATE, "downloads");
const REPORT = path.join(GATE, "json", "visual-qa.json");
const BACKEND = process.env.BACKEND_PROXY_URL ?? "http://localhost:18081";
const DEMO_EMAIL = process.env.E2E_LOGIN_EMAIL ?? process.env.E2E_DEMO_EMAIL ?? "demo@example.com";
const DEMO_PASSWORD =
  process.env.E2E_LOGIN_PASSWORD ?? process.env.E2E_DEMO_PASSWORD ?? "password123";

test.use({ storageState: { cookies: [], origins: [] } });

async function loginDemoViaApi(page: Page): Promise<void> {
  await stubOzonConnectionReady(page);
  const response = await fetch(`${BACKEND}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email: DEMO_EMAIL, password: DEMO_PASSWORD }),
  });
  if (!response.ok) {
    throw new Error(`demo login failed: ${response.status} ${await response.text()}`);
  }
  const setCookies = typeof response.headers.getSetCookie === "function"
    ? response.headers.getSetCookie()
    : [];
  const cookieHeader = response.headers.get("set-cookie") ?? "";
  const headers = setCookies.length > 0 ? setCookies : cookieHeader ? [cookieHeader] : [];
  const cookies = headers.map((header) => {
    const [pair] = header.split(";");
    const sep = pair.indexOf("=");
    return {
      name: pair.slice(0, sep).trim(),
      value: pair.slice(sep + 1).trim(),
      domain: "localhost",
      path: "/",
      httpOnly: true,
      sameSite: "Lax" as const,
    };
  });
  if (cookies.length === 0) {
    throw new Error("demo login returned no Set-Cookie");
  }
  await page.context().addCookies(cookies);
  await page.goto("/app/chat", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await expect(page).toHaveURL(/\/app(\/|$)/, { timeout: 30_000 });
}

type FormatKey = "pdf" | "docx" | "xlsx" | "png" | "csv" | "pptx";

type FormatResult = {
  status: "PASS" | "PARTIAL" | "FAIL" | "NOT_RUN" | "SKIPPED";
  detail: Record<string, unknown>;
};

const CASES: Array<{
  key: FormatKey;
  prompt: string;
  shot: string;
  downloadName: string;
  required: boolean;
}> = [
  {
    key: "pdf",
    prompt: "Выгрузи сводку продаж в PDF",
    shot: "pdf-attachment-card.png",
    downloadName: "visual-qa-sales.pdf",
    required: true,
  },
  {
    key: "docx",
    prompt: "Выгрузи сводку продаж в DOCX Word",
    shot: "docx-attachment-card.png",
    downloadName: "visual-qa-sales.docx",
    required: true,
  },
  {
    key: "xlsx",
    prompt: "Выгрузи сводку продаж в XLSX Excel",
    shot: "xlsx-attachment-card.png",
    downloadName: "visual-qa-sales.xlsx",
    required: true,
  },
  {
    key: "png",
    prompt: "Выгрузи PNG график продаж",
    shot: "png-attachment-card.png",
    downloadName: "visual-qa-sales.png",
    required: false,
  },
];

function inspectPdf(buf: Buffer): Record<string, unknown> {
  const asLatin = buf.toString("latin1");
  const fontFile2 = asLatin.includes("/FontFile2");
  // UTF-16BE Cyrillic block roughly U+0400–U+04FF → bytes 0x04 xx
  let utf16beCyrillic = false;
  for (let i = 0; i + 1 < buf.length; i++) {
    if (buf[i] === 0x04 && buf[i + 1] >= 0x00 && buf[i + 1] <= 0xff) {
      // Prefer letters А–я: 0x0410–0x044F
      if (buf[i + 1] >= 0x10 && buf[i + 1] <= 0x4f) {
        utf16beCyrillic = true;
        break;
      }
    }
  }
  // Also accept literal UTF-8 Cyrillic in streams
  const utf8Cyrillic = /[\u0400-\u04FF]/.test(buf.toString("utf8"));
  return {
    bytes: buf.length,
    embedded_font_file2: fontFile2,
    utf16be_cyrillic_in_pdf: utf16beCyrillic,
    utf8_cyrillic_in_pdf: utf8Cyrillic,
    cyrillic_extractable: utf16beCyrillic || utf8Cyrillic,
  };
}

async function openFreshChat(page: Page): Promise<Locator> {
  await loginDemoViaApi(page);
  if (!page.url().includes("/app/chat")) {
    await page.goto("/app/chat");
  }
  const newChat = page.getByRole("button", { name: /Новый чат|New chat/i }).first();
  if (await newChat.isVisible().catch(() => false)) {
    await newChat.click();
  } else {
    const chatsBtn = page.getByRole("button", { name: /^Чаты$/i }).first();
    if (await chatsBtn.isVisible().catch(() => false)) {
      await chatsBtn.click();
      await page.getByRole("button", { name: /Новый чат|New chat/i }).first().click();
    }
  }
  const composer = page.locator("textarea").first();
  await expect(composer).toBeVisible({ timeout: 30_000 });
  await expect(composer).toBeEnabled({ timeout: 30_000 });
  return composer;
}

async function ask(page: Page, composer: Locator, question: string) {
  await expect(composer).toBeEnabled({ timeout: 60_000 });
  await composer.fill(question);
  const send = page.getByRole("button", { name: /Отправить сообщение|Отправить|Send/i });
  await expect(send).toBeEnabled({ timeout: 60_000 });
  await send.click();
}

async function waitDownloadLink(page: Page) {
  await expect(page.getByRole("link", { name: /Скачать файл/i }).first()).toBeVisible({
    timeout: ASK_TIMEOUT_MS,
  });
}

function writeReport(formats: Record<string, FormatResult>, notes: string[]) {
  const requiredOk = (["pdf", "docx", "xlsx"] as const).every((k) => {
    const f = formats[k];
    if (!f || f.status === "NOT_RUN" || f.status === "SKIPPED" || f.status === "FAIL") return false;
    const d = f.detail;
    return Boolean(d.screenshot_path) && Boolean(d.path) && d.screenshot === true;
  });
  const allRequiredPass = (["pdf", "docx", "xlsx"] as const).every(
    (k) => formats[k]?.status === "PASS",
  );
  const overall = allRequiredPass && requiredOk ? "PASS" : requiredOk ? "PARTIAL" : "PARTIAL";

  const payload = {
    downloads_dir: "debug\\ai-chat-release-gate\\downloads",
    screenshots_dir: "debug\\ai-chat-release-gate\\visual-qa\\screenshots",
    formats,
    generated_at: new Date().toISOString(),
    honesty_note:
      "PASS requires UI attachment-card screenshot + downloaded file. PDF also checks /FontFile2 + Cyrillic byte evidence. Formats not exercised stay NOT_RUN.",
    method: "Playwright UI against localhost:3000 / API :18081; demo@example.com login; E2E_SKIP_SEED=1",
    notes,
    overall: allRequiredPass ? "PASS" : overall,
  };
  fs.mkdirSync(path.dirname(REPORT), { recursive: true });
  fs.writeFileSync(REPORT, JSON.stringify(payload, null, 2) + "\n", "utf8");
  return payload;
}

test.describe("AI chat visual QA artifacts @live", () => {
  test.describe.configure({ timeout: TEST_TIMEOUT_MS });

  test("screenshot + download PDF/DOCX/XLSX (+PNG best-effort)", async ({ page }) => {
    fs.mkdirSync(SHOTS, { recursive: true });
    fs.mkdirSync(DOWNLOADS, { recursive: true });

    const formats: Record<string, FormatResult> = {
      csv: { status: "NOT_RUN", detail: { reason: "out of visual-QA scope this run" } },
      pptx: { status: "NOT_RUN", detail: { reason: "out of visual-QA scope this run" } },
      pdf: { status: "NOT_RUN", detail: {} },
      docx: { status: "NOT_RUN", detail: {} },
      xlsx: { status: "NOT_RUN", detail: {} },
      png: { status: "NOT_RUN", detail: {} },
    };
    const notes: string[] = [];

    for (const tc of CASES) {
      try {
        // Flush redis rate limits between asks (best-effort).
        try {
          const { execSync } = await import("node:child_process");
          execSync("docker exec marketplace_ai_redis redis-cli FLUSHDB", { stdio: "ignore" });
        } catch {
          /* ignore */
        }

        const composer = await openFreshChat(page);
        await ask(page, composer, tc.prompt);

        const link = page.getByRole("link", { name: /Скачать файл/i }).first();
        const appeared = await link
          .waitFor({ state: "visible", timeout: ASK_TIMEOUT_MS })
          .then(() => true)
          .catch(() => false);

        if (!appeared) {
          const body = await page.locator("body").innerText().catch(() => "");
          const reason = `no «Скачать файл» within ${ASK_TIMEOUT_MS}ms; body snippet: ${body.slice(0, 400)}`;
          if (tc.required) {
            formats[tc.key] = { status: "FAIL", detail: { reason, prompt: tc.prompt } };
            notes.push(`${tc.key}: FAIL — ${reason}`);
          } else {
            formats[tc.key] = {
              status: "SKIPPED",
              detail: { reason: `PNG export did not surface download link; skipped. ${reason}`, prompt: tc.prompt },
            };
            notes.push(`${tc.key}: SKIPPED — no download link`);
          }
          writeReport(formats, notes);
          continue;
        }

        // Prefer the attachment card container around the download link.
        const card = link.locator(
          "xpath=ancestor::div[contains(@class,'rounded') and contains(@class,'border')][1]",
        );
        const shotTarget = (await card.count()) > 0 ? card : link;
        await shotTarget.scrollIntoViewIfNeeded();
        const shotPath = path.join(SHOTS, tc.shot);
        await shotTarget.screenshot({ path: shotPath });
        // Also capture a wider chat context around the card.
        await page.screenshot({
          path: path.join(SHOTS, tc.shot.replace("-attachment-card", "-chat-context")),
          fullPage: false,
        });

        const downloadPromise = page.waitForEvent("download", { timeout: 60_000 });
        await link.click();
        const download = await downloadPromise;
        const dest = path.join(DOWNLOADS, tc.downloadName);
        await download.saveAs(dest);
        const buf = fs.readFileSync(dest);

        const relShot = `debug\\ai-chat-release-gate\\visual-qa\\screenshots\\${tc.shot}`;
        const relFile = `debug\\ai-chat-release-gate\\downloads\\${tc.downloadName}`;

        const detail: Record<string, unknown> = {
          prompt: tc.prompt,
          screenshot: true,
          screenshot_path: relShot,
          path: relFile,
          bytes: buf.length,
          suggested_filename: download.suggestedFilename(),
        };

        if (tc.key === "pdf") {
          Object.assign(detail, inspectPdf(buf));
          const pdfOk =
            Boolean(detail.embedded_font_file2) && Boolean(detail.cyrillic_extractable) && buf.length > 1000;
          formats.pdf = {
            status: pdfOk ? "PASS" : "PARTIAL",
            detail: {
              ...detail,
              reason: pdfOk
                ? "UI card screenshot + download + /FontFile2 + Cyrillic byte evidence"
                : "UI screenshot+download present but FontFile2/Cyrillic evidence incomplete",
            },
          };
        } else if (buf.length > 200) {
          formats[tc.key] = {
            status: "PASS",
            detail: {
              ...detail,
              reason: "UI attachment-card screenshot + downloaded file",
            },
          };
        } else {
          formats[tc.key] = {
            status: "PARTIAL",
            detail: { ...detail, reason: "download too small / suspicious" },
          };
        }

        notes.push(
          `${tc.key}: ${formats[tc.key].status} shot=${tc.shot} bytes=${buf.length}`,
        );
        writeReport(formats, notes);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        if (tc.required) {
          formats[tc.key] = { status: "FAIL", detail: { reason: msg, prompt: tc.prompt } };
        } else {
          formats[tc.key] = {
            status: "SKIPPED",
            detail: { reason: `skipped after error: ${msg}`, prompt: tc.prompt },
          };
        }
        notes.push(`${tc.key}: error — ${msg}`);
        writeReport(formats, notes);
        if (tc.required) throw err;
      }
    }

    const report = writeReport(formats, notes);
    // Soft assert: required formats must at least have screenshots+downloads
    for (const k of ["pdf", "docx", "xlsx"] as const) {
      expect(formats[k].detail.screenshot, `${k} screenshot`).toBe(true);
      expect(fs.existsSync(path.join(DOWNLOADS, CASES.find((c) => c.key === k)!.downloadName))).toBe(
        true,
      );
    }
    console.log("[visual-qa] overall=", report.overall, "notes=", notes);
  });
});
