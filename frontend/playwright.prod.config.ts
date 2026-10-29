import { defineConfig, devices } from "@playwright/test";
import path from "node:path";

const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:3000";
const authFile = path.join(__dirname, "e2e/.auth/seller.json");

/**
 * Production acceptance config: uses already-running `next start` + backend.
 * Skips webServer and uses demo seller storageState from a lightweight login setup.
 */
export default defineConfig({
  testDir: "./e2e",
  testMatch: /dashboard-overview-prod\.spec\.ts|dashboard-live-review-local\.spec\.ts/,
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: [["list"]],
  timeout: 180_000,
  globalSetup: require.resolve("./e2e/prod-global-setup.ts"),
  use: {
    baseURL,
    storageState: authFile,
    trace: "off",
    screenshot: "only-on-failure",
    deviceScaleFactor: 1,
  },
  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
        deviceScaleFactor: 1,
      },
    },
  ],
});
