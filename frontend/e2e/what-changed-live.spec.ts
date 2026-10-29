/**
 * Live acceptance for Dashboard «Что изменилось» against published seller=1.
 * Does NOT stub home API — uses real publication.
 */
import { test, expect } from "@playwright/test";
import path from "node:path";
import fs from "node:fs";
import { stubOzonConnectionReady } from "./helpers/auth";

test.describe("what-changed live publication 2x2 @live", () => {
  test("live dashboard shows 2x2 rich what-changed", async ({ page }) => {
    await stubOzonConnectionReady(page);
    await page.setViewportSize({ width: 1440, height: 1600 });
    await page.goto("/app/dashboard", { waitUntil: "domcontentloaded" });

    const card = page.getByTestId("what-changed-card");
    await expect(card).toBeVisible({ timeout: 45000 });

    const grid = page.getByTestId("what-changed-groups-grid");
    await expect(grid).toBeVisible();
    const gridClass = await grid.getAttribute("class");
    expect(gridClass ?? "").toContain("sm:grid-cols-2");
    expect(gridClass ?? "").not.toContain("grid-cols-4");

    const worsened = page.getByTestId("what-changed-group-worsened");
    const newRisks = page.getByTestId("what-changed-group-new_risks");
    const active = page.getByTestId("what-changed-group-unchanged_risks");
    const improved = page.getByTestId("what-changed-group-improved");

    await expect(worsened).toBeVisible();
    await expect(newRisks).toBeVisible();
    await expect(active).toBeVisible();
    await expect(improved).toBeVisible();

    // Active problems must not show false-zero empty history dump as the only content
    const activeText = await active.innerText();
    expect(activeText).not.toMatch(/Нет долговременных проблем/i);
    expect(activeText.length).toBeGreaterThan(40);

    const cardText = await card.innerText();
    expect(cardText).not.toMatch(/\b882\d{9}\b/);
    expect(cardText).not.toMatch(/Нет долговременных проблем/i);

    const [w, n, a, i] = await Promise.all([
      worsened.boundingBox(),
      newRisks.boundingBox(),
      active.boundingBox(),
      improved.boundingBox(),
    ]);
    expect(w && n && a && i).toBeTruthy();
    if (!w || !n || !a || !i) return;

    expect(w.x).toBeLessThan(n.x);
    expect(a.x).toBeLessThan(i.x);
    expect(w.y).toBeLessThan(a.y);
    expect(Math.abs(w.width - n.width) / w.width).toBeLessThan(0.05);
    expect(Math.abs(w.height - n.height) / Math.max(w.height, 1)).toBeLessThan(0.1);

    const outDir = path.join(process.cwd(), "..", "backend", "debug", "what-changed-screenshots");
    fs.mkdirSync(outDir, { recursive: true });
    fs.writeFileSync(
      path.join(outDir, "bounding-boxes-live-1440.json"),
      JSON.stringify({ worsened: w, new_risks: n, active: a, improved: i }, null, 2),
    );
    await card.screenshot({ path: path.join(outDir, "what-changed-live-1440.png") });
    await page.setViewportSize({ width: 1920, height: 1600 });
    await card.screenshot({ path: path.join(outDir, "what-changed-live-1920.png") });
    await page.setViewportSize({ width: 1366, height: 1600 });
    await card.screenshot({ path: path.join(outDir, "what-changed-live-1366.png") });
  });
});
