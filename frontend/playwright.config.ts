import { defineConfig, devices } from "@playwright/test";
import path from "node:path";

const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:3000";
const backendUrl = process.env.BACKEND_PROXY_URL ?? "http://localhost:8081";
const useProductionServer = process.env.E2E_WEB_SERVER_MODE === "production";
const skipWebServer = Boolean(process.env.E2E_SKIP_WEB_SERVER);

const authFile = path.join(__dirname, "e2e/.auth/seller.json");

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  // @live specs require a pre-seeded demo seller / published pilot data — not CI fixtures.
  grepInvert: process.env.CI ? /@live/ : undefined,
  reporter: process.env.CI ? [["github"], ["list"]] : [["list"]],
  timeout: 120_000,
  globalSetup: require.resolve("./e2e/global-setup.ts"),
  use: {
    baseURL,
    storageState: authFile,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  webServer: skipWebServer
    ? undefined
    : {
        command: useProductionServer ? "npm run start" : "npm run dev",
        url: baseURL,
        reuseExistingServer: !process.env.CI,
        timeout: useProductionServer ? 180_000 : 120_000,
        env: {
          ...process.env,
          PORT: new URL(baseURL).port || "3000",
          NEXT_PUBLIC_API_BASE_URL: "",
          BACKEND_PROXY_URL: backendUrl,
          BACKEND_INTERNAL_URL: backendUrl,
        },
      },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
