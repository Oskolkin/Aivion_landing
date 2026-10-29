const { chromium } = require("playwright");
const path = require("path");
(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    storageState: path.join(__dirname, "e2e/.auth/seller.json"),
    viewport: { width: 1280, height: 720 },
  });
  const page = await context.newPage();
  const logs = [];
  page.on("console", (m) => logs.push(["console", m.type(), m.text()]));
  page.on("pageerror", (e) => logs.push(["pageerror", e.message]));
  const reqs = [];
  page.on("request", (r) => {
    if (/chat|ask|upload|attachment/i.test(r.url())) reqs.push({ method: r.method(), url: r.url() });
  });
  page.on("response", async (r) => {
    if (/chat|ask|upload|attachment/i.test(r.url())) {
      reqs.push({ status: r.status(), url: r.url() });
    }
  });

  // login flow like helper
  await page.route("**/api/v1/integrations/ozon/connection", async (route) => {
    if (route.request().method() !== "GET") { await route.continue(); return; }
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ connection_ready: true, status: "connected", missing: [], errors: [], seller_credentials_set: true, performance_secret_set: true }) });
  });
  await page.goto("http://localhost:3000/login", { waitUntil: "domcontentloaded", timeout: 30000 });
  await page.waitForTimeout(1000);
  const email = page.getByLabel("Email");
  if (await email.isVisible().catch(() => false)) {
    await email.fill("demo@example.com");
    await page.getByLabel(/Пароль/i).fill("password123");
    await page.getByRole("button", { name: "Вход" }).click();
    await page.waitForURL(/\/app(\/|$)/, { timeout: 30000 });
  } else {
    logs.push(["info", "already logged in / no email field", page.url()]);
  }
  await page.goto("http://localhost:3000/app/chat", { waitUntil: "domcontentloaded", timeout: 30000 });
  await page.waitForTimeout(2000);
  const newChat = page.getByRole("button", { name: /Новый чат|New chat/i }).first();
  if (await newChat.isVisible().catch(() => false)) await newChat.click();
  await page.waitForTimeout(1000);
  const composer = page.locator("textarea").first();
  await composer.waitFor({ state: "visible", timeout: 30000 });
  const fixtures = path.resolve(__dirname, "../backend/debug/ai-chat-audit/fixtures/control.txt");
  await page.locator('input[type="file"]').first().setInputFiles(fixtures);
  await page.waitForTimeout(5000);
  const body1 = (await page.locator("body").innerText()).slice(0, 2000);
  await page.screenshot({ path: path.resolve(__dirname, "../backend/debug/ai-chat-release-gate/screenshots/debug-txt-upload.png"), fullPage: true });
  const send = page.getByRole("button", { name: /Отправить|Send/i });
  const marker = "E2E-DEBUG-" + Date.now();
  await composer.fill("Какой контрольный код в файле? Процитируй. Маркер " + marker);
  const sendEnabled = await send.isEnabled().catch(() => false);
  const sendVisible = await send.isVisible().catch(() => false);
  await send.click({ force: true }).catch((e) => logs.push(["send-click-err", String(e)]));
  // wait up to 90s
  let found = false;
  for (let i = 0; i < 45; i++) {
    await page.waitForTimeout(2000);
    const t = await page.locator("body").innerText();
    if (t.includes(marker) || t.includes("TXT_CONTROL_16735") || /ошибк|error|не удалось/i.test(t)) {
      found = true;
      logs.push(["found-at-sec", String((i+1)*2), t.includes(marker)?"marker":"", t.includes("TXT_CONTROL_16735")?"control":""]);
      break;
    }
  }
  const body2 = (await page.locator("body").innerText()).slice(0, 3000);
  await page.screenshot({ path: path.resolve(__dirname, "../backend/debug/ai-chat-release-gate/screenshots/debug-txt-ask.png"), fullPage: true });
  console.log(JSON.stringify({ found, sendEnabled, sendVisible, url: page.url(), body1, body2, reqs: reqs.slice(-30), logs: logs.slice(-40) }, null, 2));
  await browser.close();
})().catch((e) => { console.error(e); process.exit(1); });
