const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');
(async () => {
  const OUT = path.resolve('..','backend','debug','dashboard-audit-export','screenshots');
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch();
  const page = await browser.newPage();
  await page.goto('http://localhost:3000/login', { waitUntil: 'domcontentloaded' });
  await page.locator('input[type=\"email\"], input[name=\"email\"]').first().fill('demo@example.com');
  await page.locator('input[type=\"password\"]').first().fill('password123');
  await page.getByRole('button', { name: /войти|login/i }).click();
  await page.waitForTimeout(2500);
  for (const [w,h,name] of [[1920,1080,'dashboard-1920'],[1366,768,'dashboard-1366']]) {
    await page.setViewportSize({ width: w, height: h });
    await page.goto('http://localhost:3000/app/dashboard', { waitUntil: 'networkidle' });
    await page.waitForTimeout(2000);
    await page.screenshot({ path: path.join(OUT, name + '.png'), fullPage: true });
    console.log('wrote', name);
  }
  await browser.close();
})().catch(e => { console.error(e); process.exit(1); });
