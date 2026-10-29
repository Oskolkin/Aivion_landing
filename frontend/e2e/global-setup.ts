import type { FullConfig } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { execSync } from "node:child_process";
import {
  E2E_DEMO_EMAIL,
  E2E_DEMO_PASSWORD,
  seedIsolatedE2ESeller,
  getSellerId,
} from "./helpers/seed";

const authFile = path.join(__dirname, ".auth", "seller.json");

function parseSetCookieHeader(header: string, domain: string) {
  const [pair] = header.split(";");
  const separator = pair.indexOf("=");
  const name = pair.slice(0, separator).trim();
  const value = pair.slice(separator + 1).trim();
  return {
    name,
    value,
    domain,
    path: "/",
    httpOnly: true,
    sameSite: "Lax" as const,
    expires: Math.floor(Date.now() / 1000) + 60 * 60 * 24 * 30,
    secure: false,
  };
}

/**
 * Global setup must NEVER reset demo seller 1 / demo@example.com by default.
 * Isolated fixture mode creates a unique e2e-{timestamp}@example.test seller.
 *
 * Opt-in destructive demo seed only when E2E_ALLOW_DEMO_RESET=1 (manual/CI only).
 * E2E_SKIP_SEED=1 skips seeding entirely (reuse existing .seller-id + auth).
 */
export default async function globalSetup(config: FullConfig): Promise<void> {
  // Chat attachment fixtures are gitignored under backend/debug; generate for CI/local.
  try {
    const backendDir = path.resolve(__dirname, "../../backend");
    execSync("go run ./cmd/genchatfixtures", {
      cwd: backendDir,
      stdio: "inherit",
      env: process.env,
    });
    console.log("[e2e] generated ai-chat-audit fixtures");
  } catch (err) {
    console.warn("[e2e] genchatfixtures failed:", err);
  }

  if (process.env.REDIS_ADDR || process.env.CI) {
    try {
      execSync("docker exec marketplace_ai_redis redis-cli FLUSHALL", { stdio: "ignore" });
    } catch {
      // Redis may be unavailable locally; login rate limit uses storageState after first run.
    }
  }

  let loginEmail = E2E_DEMO_EMAIL;
  let loginPassword = E2E_DEMO_PASSWORD;

  if (process.env.E2E_SKIP_SEED === "1") {
    console.log("[e2e] E2E_SKIP_SEED=1 — skipping seed; using existing seller/auth");
    try {
      console.log(`[e2e] existing seller_account_id=${getSellerId()}`);
    } catch {
      console.warn("[e2e] no seller id file; auth may fail if seller was never seeded");
    }
  } else if (process.env.E2E_ALLOW_DEMO_RESET === "1") {
    // Explicit destructive path — resets demo@example.com. Not the default.
    const { seedE2EBaseline } = await import("./helpers/seed");
    const sellerId = seedE2EBaseline();
    console.log(`[e2e] DEMO RESET seed completed for seller_account_id=${sellerId}`);
  } else if (process.env.E2E_CHAT_BACKEND === "1") {
    const { preparePrompt15E2EScenario } = await import("./helpers/seed");
    const manifest = preparePrompt15E2EScenario();
    loginEmail = manifest.email;
    loginPassword = manifest.password;
    console.log(
      `[e2e] Prompt15 backend fixture seller_account_id=${manifest.seller_account_id} email=${manifest.email} actions=${manifest.active_action_count}`,
    );
  } else {
    const fixture = seedIsolatedE2ESeller();
    loginEmail = fixture.email;
    loginPassword = fixture.password;
    console.log(
      `[e2e] isolated fixture seller_account_id=${fixture.sellerId} email=${fixture.email}`,
    );
  }

  const baseURL = config.projects[0]?.use?.baseURL ?? "http://localhost:3000";
  const origin = new URL(baseURL);
  const backendUrl = process.env.BACKEND_PROXY_URL ?? "http://localhost:8081";

  if (fs.existsSync(authFile) && process.env.E2E_REUSE_AUTH === "1") {
    console.log("[e2e] reusing existing auth storage state");
    return;
  }

  const email = process.env.E2E_LOGIN_EMAIL ?? loginEmail;
  const password = process.env.E2E_LOGIN_PASSWORD ?? loginPassword;

  const response = await fetch(`${backendUrl}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!response.ok) {
    throw new Error(`global auth login failed: ${response.status} ${await response.text()}`);
  }

  const cookies = response.headers
    .getSetCookie()
    .map((header) => parseSetCookieHeader(header, origin.hostname));

  fs.mkdirSync(path.dirname(authFile), { recursive: true });
  fs.writeFileSync(authFile, JSON.stringify({ cookies, origins: [] }, null, 2));
}
