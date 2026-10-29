import { expect, test, type Locator, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { loginAsDemoSeller, stubOzonConnectionReady } from "./helpers/auth";

/**
 * P1-MOBILE-COMPOSER: composer must remain inside the first viewport on phone sizes.
 * Viewports cover common iOS / Android widths used in the release gate.
 */
const VIEWPORTS = [
  { name: "390x844", width: 390, height: 844 },
  { name: "430x932", width: 430, height: 932 },
  { name: "360x800", width: 360, height: 800 },
  { name: "412x915", width: 412, height: 915 },
] as const;

const OUT_DIR = path.resolve(__dirname, "../../backend/debug/ai-chat-release-gate/mobile");

type ComposerBox = {
  x: number;
  y: number;
  width: number;
  height: number;
  bottom: number;
  viewportHeight: number;
};

const composerMeasurements: Array<{ viewport: string; scenario: string; box: ComposerBox }> = [];

async function openChat(page: Page): Promise<Locator> {
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

async function assertComposerInViewport(
  page: Page,
  composer: Locator,
  meta: { viewport: string; scenario: string },
) {
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

  const measured: ComposerBox = {
    x: box!.x,
    y: box!.y,
    width: box!.width,
    height: box!.height,
    bottom: box!.y + box!.height,
    viewportHeight: viewport!.height,
  };
  composerMeasurements.push({ viewport: meta.viewport, scenario: meta.scenario, box: measured });

  fs.mkdirSync(OUT_DIR, { recursive: true });
  const slug = `${meta.viewport}-${meta.scenario}`;
  await page.screenshot({
    path: path.join(OUT_DIR, `${slug}.png`),
    fullPage: false,
  });
}

test.afterAll(() => {
  if (composerMeasurements.length === 0) return;
  fs.mkdirSync(OUT_DIR, { recursive: true });
  fs.writeFileSync(
    path.join(OUT_DIR, "composer-bboxes.json"),
    `${JSON.stringify(composerMeasurements, null, 2)}\n`,
    "utf8",
  );
});

for (const vp of VIEWPORTS) {
  test.describe(`AI chat mobile composer ${vp.name}`, () => {
    test.use({ viewport: { width: vp.width, height: vp.height } });

    test("composer visible in first viewport", async ({ page }) => {
      const composer = await openChat(page);
      await assertComposerInViewport(page, composer, {
        viewport: vp.name,
        scenario: "initial",
      });
    });

    test("composer still in viewport after typing", async ({ page }) => {
      const composer = await openChat(page);
      await composer.fill("Короткий вопрос для проверки layout на мобильном.");
      await assertComposerInViewport(page, composer, {
        viewport: vp.name,
        scenario: "after-typing",
      });
    });
  });
}
