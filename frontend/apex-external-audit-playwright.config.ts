import { defineConfig } from "@playwright/test";
import path from "node:path";

export default defineConfig({
  testDir: path.join(__dirname, "e2e"),
  testMatch: /external-audit-anonymous\.spec\.ts/,
  fullyParallel: false,
  workers: 1,
  timeout: 120_000,
  reporter: [["list"]],
  use: {
    baseURL: process.env.PUBLIC_DEMO_BASE_URL || "https://aivion.pro",
  },
});
