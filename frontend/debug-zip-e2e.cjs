const { chromium } = require("playwright");
const path = require("path");
(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    storageState: path.join(__dirname, "e2e/.auth/seller.json"),
    viewport: { width: 1400, height: 900 },
  });
  const page = await context.newPage();
  const net = [];
  page.on("response", async (r) => {
    if (/attachment|chat/i.test(r.url())) {
      let body = "";
      try { body = (await r.text()).slice(0, 300); } catch {}
      net.push({ status: r.status(), url: r.url(), body });
    }
  });
  await page.route("**/api/v1/integrations/ozon/connection", async (route) => {
    if (route.request().method() !== "GET") { await route.continue(); return; }
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ connection_ready: true, status: "connected", missing: [], errors: [], seller_credentials_set: true, performance_secret_set: true }) });
  });
  await page.goto("http://localhost:3000/app/chat", { waitUntil: "domcontentloaded", timeout: 60000 });
  await page.waitForTimeout(1500);
  if (await page.getByLabel("Email").isVisible().catch(() => false)) {
    await page.goto("http://localhost:3000/login", { waitUntil: "domcontentloaded" });
    await page.getByLabel("Email").fill("demo@example.com");
    await page.getByLabel(/Пароль/i).fill("password123");
    await page.getByRole("button", { name: "Вход" }).click();
    await page.waitForURL(/\/app/, { timeout: 60000 });
    await page.goto("http://localhost:3000/app/chat", { waitUntil: "domcontentloaded" });
  }
  const newChat = page.getByRole("button", { name: /Новый чат/i }).first();
  if (await newChat.isVisible()) await newChat.click();
  await page.locator("textarea").first().waitFor({ state: "visible", timeout: 30000 });
  const zip = path.resolve(__dirname, "../backend/debug/ai-chat-audit/fixtures/control.zip");
  await page.locator('input[type="file"]').first().setInputFiles(zip);
  for (let i = 0; i < 30; i++) {
    await page.waitForTimeout(1000);
    const ready = await page.getByTestId("draft-attachment-ready").count();
    const up = await page.getByTestId("draft-attachment-uploading").count();
    const err = await page.getByTestId("draft-attachment-error").count();
    const hasName = await page.getByText("control.zip").count();
    const errText = await page.locator("body").innerText();
    console.log(JSON.stringify({ t: i+1, ready, up, err, hasName, snippet: errText.includes("Не удалось") ? "fail-msg" : (errText.includes("Загрузка") ? "loading" : "other") }));
    if (ready > 0 || err > 0) break;
  }
  console.log("NET", JSON.stringify(net.slice(-10), null, 2));
  await page.screenshot({ path: path.resolve(__dirname, "../backend/debug/ai-chat-release-gate/screenshots/debug-zip-upload.png"), fullPage: true });
  await browser.close();
})().catch((e) => { console.error(e); process.exit(1); });
