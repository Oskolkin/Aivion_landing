import { defineConfig } from "@playwright/test";
import path from "node:path";

export default defineConfig({
  testDir: path.join(__dirname, "e2e"),
  testMatch: /live-review\.spec\.ts/,
  fullyParallel: false,
  workers: 1,
  timeout: 180_000,
  expect: { timeout: 30_000 },
  reporter: [["list"]],
  use: {
    baseURL: process.env.LIVE_REVIEW_BASE_URL || "https://example.invalid",
    screenshot: "only-on-failure",
  },
});
