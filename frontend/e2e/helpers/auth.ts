import type { Page } from "@playwright/test";
import { E2E_DEMO_EMAIL, E2E_DEMO_PASSWORD } from "./seed";

export async function stubOzonConnectionReady(page: Page): Promise<void> {
  await page.route("**/api/v1/integrations/ozon/connection", async (route) => {
    if (route.request().method() !== "GET") {
      await route.continue();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        connection_ready: true,
        status: "connected",
        missing: [],
        errors: [],
        seller_credentials_set: true,
        performance_secret_set: true,
      }),
    });
  });
}

export async function loginAsDemoSeller(page: Page, options?: { skipNavigation?: boolean }): Promise<void> {
  await stubOzonConnectionReady(page);
  const loginEmail = process.env.E2E_LOGIN_EMAIL ?? E2E_DEMO_EMAIL;
  const loginPassword = process.env.E2E_LOGIN_PASSWORD ?? E2E_DEMO_PASSWORD;
  if (options?.skipNavigation) {
    return;
  }

  // Prefer existing session (storageState / prior tests) to avoid login rate limits.
  try {
    await page.goto("/app/chat", { waitUntil: "domcontentloaded", timeout: 60_000 });
  } catch {
    // Fall through to explicit login.
  }

  if (/\/app(\/|$)/.test(page.url())) {
    const emailField = page.getByLabel("Email");
    if (!(await emailField.isVisible().catch(() => false))) {
      return;
    }
  }

  await page.goto("/login", { waitUntil: "domcontentloaded", timeout: 60_000 });
  await page.getByLabel("Email").fill(loginEmail);
  await page.getByLabel(/Пароль/i).fill(loginPassword);
  await page.getByRole("button", { name: "Вход" }).click();
  await page.waitForURL(/\/app(\/|$)/, { timeout: 60_000 });
}
