import { execSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

const repoRoot = path.resolve(__dirname, "../../..");
const backendDir = path.join(repoRoot, "backend");
const migrationsDir = path.join(backendDir, "migrations");
const e2eDir = path.join(__dirname, "..");
const sellerIdFile = path.join(e2eDir, ".seller-id");

export const E2E_DEMO_EMAIL = process.env.E2E_DEMO_EMAIL ?? "demo@example.com";
export const E2E_DEMO_PASSWORD = process.env.E2E_DEMO_PASSWORD ?? "password123";

const credentialsFile = path.join(e2eDir, ".e2e-credentials.json");

let cachedSellerId = Number(process.env.E2E_SELLER_ID ?? "0");
let cachedEmail = "";
let cachedPassword = "";

function seedEnv(): NodeJS.ProcessEnv {
  return {
    ...process.env,
    DATABASE_URL:
      process.env.DATABASE_URL ??
      "postgres://postgres:postgres@localhost:55432/marketplace_ai?sslmode=disable",
    ENCRYPTION_KEY: process.env.ENCRYPTION_KEY ?? "01234567890123456789012345678901",
    JWT_SECRET: process.env.JWT_SECRET ?? "dev-jwt-secret",
    APP_ENV: process.env.APP_ENV ?? "local",
    ALLOW_DEV_SEED: "1",
    AI_PROVIDER_MODE: process.env.AI_PROVIDER_MODE ?? "fake",
  };
}

function runGo(args: string[], extraEnv: Record<string, string> = {}, capture = false): string {
  const shell = process.platform === "win32" ? "cmd.exe" : "/bin/sh";
  const output = execSync(`go run ${args.join(" ")}`, {
    cwd: backendDir,
    env: { ...seedEnv(), ...extraEnv },
    stdio: capture ? "pipe" : "inherit",
    encoding: capture ? "utf8" : undefined,
    shell,
  });
  return capture ? String(output) : "";
}

function runGoTest(args: string[], extraEnv: Record<string, string> = {}, capture = false): string {
  const shell = process.platform === "win32" ? "cmd.exe" : "/bin/sh";
  const output = execSync(`go ${args.join(" ")}`, {
    cwd: backendDir,
    env: { ...seedEnv(), ...extraEnv },
    stdio: capture ? "pipe" : "inherit",
    encoding: capture ? "utf8" : undefined,
    shell,
  });
  return capture ? String(output) : "";
}

export function runMigrations(): void {
  const databaseUrl =
    process.env.DATABASE_URL ??
    "postgres://postgres:postgres@localhost:55432/marketplace_ai?sslmode=disable";
  const shell = process.platform === "win32" ? "cmd.exe" : "/bin/sh";

  // Prefer a local migrate binary (CI/cloud agents often already have it).
  // Docker overlay mounts are not always available in restricted VMs.
  try {
    execSync(`migrate -path "${migrationsDir}" -database "${databaseUrl}" up`, {
      stdio: "inherit",
      shell,
    });
    return;
  } catch {
    // Fall through to dockerized migrate.
  }

  const mountPath = process.platform === "win32" ? migrationsDir.replace(/\\/g, "/") : migrationsDir;
  execSync(
    `docker run --rm --network host -v "${mountPath}:/migrations" migrate/migrate -path /migrations -database "${databaseUrl}" up`,
    { stdio: "inherit", shell },
  );
}

export function getSellerId(): number {
  if (cachedSellerId > 0) {
    return cachedSellerId;
  }
  if (fs.existsSync(sellerIdFile)) {
    cachedSellerId = Number(fs.readFileSync(sellerIdFile, "utf8").trim());
    if (cachedSellerId > 0) {
      return cachedSellerId;
    }
  }
  throw new Error("seller id is not initialized; run global setup first");
}

export function getE2ECredentials(): { email: string; password: string; sellerId: number } {
  if (cachedEmail && cachedPassword && cachedSellerId > 0) {
    return { email: cachedEmail, password: cachedPassword, sellerId: cachedSellerId };
  }
  if (fs.existsSync(credentialsFile)) {
    const raw = JSON.parse(fs.readFileSync(credentialsFile, "utf8")) as {
      email?: string;
      password?: string;
      sellerId?: number;
    };
    if (raw.email && raw.password && raw.sellerId) {
      cachedEmail = raw.email;
      cachedPassword = raw.password;
      cachedSellerId = raw.sellerId;
      return { email: cachedEmail, password: cachedPassword, sellerId: cachedSellerId };
    }
  }
  return { email: E2E_DEMO_EMAIL, password: E2E_DEMO_PASSWORD, sellerId: getSellerId() };
}

function persistSellerId(sellerId: number): void {
  cachedSellerId = sellerId;
  fs.writeFileSync(sellerIdFile, String(sellerId));
}

function persistCredentials(email: string, password: string, sellerId: number): void {
  cachedEmail = email;
  cachedPassword = password;
  persistSellerId(sellerId);
  fs.writeFileSync(credentialsFile, JSON.stringify({ email, password, sellerId }, null, 2));
}

/** Destructive: resets demo@example.com. Only for explicit E2E_ALLOW_DEMO_RESET=1. */
export function seedMvpSeller(): number {
  const output = runGo(
    [
      "./cmd/dev-seed-mvp",
      `--email=${E2E_DEMO_EMAIL}`,
      `--password=${E2E_DEMO_PASSWORD}`,
      "--reset=true",
    ],
    {},
    true,
  );
  const match = output.match(/seller_account_id:\s*(\d+)/);
  if (!match) {
    throw new Error("failed to parse seller_account_id from dev-seed-mvp output");
  }
  const sellerId = Number(match[1]);
  persistCredentials(E2E_DEMO_EMAIL, E2E_DEMO_PASSWORD, sellerId);
  return sellerId;
}

/**
 * Creates a unique isolated E2E seller. Never touches demo seller 1.
 * Uses a fixed anchor metric date so fixtures are not tied to "today".
 */
export function seedIsolatedE2ESeller(anchorDate = "2026-06-01"): {
  sellerId: number;
  email: string;
  password: string;
} {
  runMigrations();
  const stamp = Date.now();
  const email = `e2e-unified-${stamp}@example.test`;
  const password = `e2e-pass-${stamp}`;
  const output = runGo(
    [
      "./cmd/dev-seed-mvp",
      `--email=${email}`,
      `--password=${password}`,
      "--reset=false",
      `--anchor-date=${anchorDate}`,
      "--with-admin-user=false",
      `--seller-name=E2E Unified ${stamp}`,
    ],
    {},
    true,
  );
  const match = output.match(/seller_account_id:\s*(\d+)/);
  if (!match) {
    throw new Error(`failed to parse seller_account_id from isolated seed:\n${output}`);
  }
  const sellerId = Number(match[1]);
  // On a fresh database the first seller is id=1 — that is fine as long as the
  // email is an isolated e2e-* fixture, not demo@example.com.
  if (email === E2E_DEMO_EMAIL || !email.includes("@example.test")) {
    throw new Error(`isolated E2E seed produced unexpected email=${email} seller_account_id=${sellerId}`);
  }
  persistCredentials(email, password, sellerId);
  seedOzonEnrichment("rich");
  seedActions("available");
  runFakeAIPipeline(anchorDate);
  return { sellerId, email, password };
}

export function seedActions(
  scenario: "available" | "checked" | "empty" | "mixed" = "available",
  opts?: { setCurrent?: boolean; metricDate?: string },
): void {
  const args = [
    "./cmd/devtools",
    "seed-actions",
    `--seller-id=${getSellerId()}`,
    `--scenario=${scenario}`,
  ];
  if (opts?.metricDate) {
    args.push(`--metric-date=${opts.metricDate}`);
  }
  // Opt-in: legacy E2E specs that run after Unified fixtures must retarget
  // seller_current_ai_snapshots onto a fresh generation run (published unified
  // action_items are FK-protected and cannot be deleted by empty seed).
  if (opts?.setCurrent) {
    args.push("--set-current=true");
  }
  runGo(args);
}

/** Legacy-mode Actions prep for suite files that must not see Unified fixtures. */
export function prepareLegacyActionsScenario(
  scenario: "available" | "checked" | "empty" | "mixed" = "available",
): void {
  setUnifiedAnalysisMode("legacy");
  try {
    seedActions(scenario, { setCurrent: true });
  } catch (error) {
    // Fresh isolated sellers may not yet have a published AI snapshot to
    // retarget. Bootstrap a material Unified fixture (creates a published
    // pipeline + snapshot), then force legacy and seed again.
    console.warn(
      "[e2e] set-current seed failed; bootstrapping published snapshot via unified fixture:",
      error instanceof Error ? error.message : error,
    );
    prepareUnifiedBrowserScenario("material");
    setUnifiedAnalysisMode("legacy");
    seedActions(scenario, { setCurrent: true });
  }
}

export function setUnifiedAnalysisMode(
  mode: "legacy" | "shadow" | "unified",
  opts?: { autoRun?: boolean; autoPublish?: boolean },
): void {
  runGo([
    "./cmd/devtools",
    "set-unified-analysis-mode",
    `--seller-id=${getSellerId()}`,
    `--mode=${mode}`,
    `--auto-run=${opts?.autoRun ?? false}`,
    `--auto-publish=${opts?.autoPublish ?? false}`,
  ]);
}

export type UnifiedBrowserScenarioPrep = {
  seller_id: number;
  scenario: string;
  analysis_outcome?: string;
  current_publication_id: number;
  current_run_id: number;
  unified_pointer_publication_id: number;
  compat_pointer_run_id: number;
  promoted?: boolean;
  material_publication: {
    publication_id: number;
    run_id: number;
    outcome: string;
    visibility: string;
    actions_count: number;
  };
  healthy_publication: {
    publication_id: number;
    run_id: number;
    outcome: string;
    visibility: string;
    actions_count: number;
  };
  insufficient_publication: {
    publication_id: number;
    run_id: number;
    outcome: string;
    visibility: string;
    actions_count: number;
  };
  shadow_publication: {
    publication_id: number;
    run_id: number;
    outcome: string;
    visibility: string;
    actions_count: number;
  };
};

export function prepareUnifiedBrowserScenario(
  scenario: "material" | "healthy" | "insufficient" | "shadow" | "promote",
): UnifiedBrowserScenarioPrep {
  const output = runGo(
    [
      "./cmd/devtools",
      "prepare-unified-browser-scenario",
      `--seller-id=${getSellerId()}`,
      `--scenario=${scenario}`,
    ],
    {},
    true,
  );
  // Devtool prints one JSON object; ignore any preceding go/runtime noise.
  const match = output.match(/\{[\s\S]*\}\s*$/);
  if (!match) {
    throw new Error(`prepare-unified-browser-scenario: no JSON in output:\n${output}`);
  }
  return JSON.parse(match[0]) as UnifiedBrowserScenarioPrep;
}

export function seedOzonEnrichment(scenario: "rich" | "minimal" | "edge" = "rich"): void {
  runGo([
    "./cmd/devtools",
    "seed-ozon-enrichment",
    `--seller-id=${getSellerId()}`,
    `--scenario=${scenario}`,
  ]);
}

function todayMetricDate(): string {
  return new Date().toISOString().slice(0, 10);
}

export function runFakeAIPipeline(metricDate?: string): void {
  const date = metricDate ?? todayMetricDate();
  try {
    runGo(
      [
        "./cmd/devtools",
        "run-ai-pipeline",
        `--seller-id=${getSellerId()}`,
        `--metric-date=${date}`,
        "--dashboard=true",
        "--actions=true",
        "--ai-enabled=false",
      ],
      {
        ALLOW_DEV_AI_PIPELINE: "1",
        SELLER_AI_PIPELINE_ENABLED: "1",
        ACTIONS_AI_FIRST: "1",
        AI_PROVIDER_MODE: "fake",
      },
    );
  } catch (error) {
    console.warn(
      "[e2e seed] run-ai-pipeline failed; continuing with devtools action seed only:",
      error instanceof Error ? error.message : error,
    );
  }
}

export function seedE2EBaseline(): number {
  runMigrations();
  const sellerId = seedMvpSeller();
  seedOzonEnrichment("rich");
  seedActions("available");
  runFakeAIPipeline();
  return sellerId;
}

export type Prompt15E2EManifest = {
  seller_account_id: number;
  email: string;
  password: string;
  p1_publication_id: number;
  p2_publication_id: number;
  p1_universe_id?: string;
  p2_universe_id?: string;
  current_publication_id: number;
  sku417_canonical_id: number;
  sku417_ozon_sku: number;
  group_id: string;
  group_action_view_id: string;
  group_title: string;
  group_case_ids: string[];
  singleton_case_id: string;
  singleton_action_view_id: string;
  singleton_title: string;
  historical_case_id: string;
  historical_action_view_id: string;
  historical_title: string;
  historical_origin_publication_id: number;
  historical_current_publication_id: number;
  active_action_count: number;
  completed_action_count: number;
};

const manifestFile = path.join(e2eDir, ".prompt15-e2e-manifest.json");

/** Canonical Prompt15 analysis_publications + Actions fixture (not Unified). */
export function preparePrompt15E2EScenario(): Prompt15E2EManifest {
  runMigrations();
  const databaseUrl =
    process.env.DATABASE_URL ??
    "postgres://postgres:postgres@localhost:55432/marketplace_ai?sslmode=disable";
  const output = runGoTest(
    [
      "test",
      "-tags=integration",
      "-count=1",
      "-timeout=30m",
      "-p",
      "1",
      "./internal/analysis/",
      "-run",
      "TestE2EPreparePrompt15BackendScenario",
    ],
    {
      PROMPT15_E2E_PREPARE: "1",
      TEST_DATABASE_URL: databaseUrl,
      E2E_PREPARE_OUTPUT_DIR: e2eDir,
    },
    true,
  );
  const match = output.match(/\{[\s\S]*\}\s*$/);
  if (!match) {
    throw new Error(`prepare Prompt15 E2E: no JSON in output:\n${output}`);
  }
  const manifest = JSON.parse(match[0]) as Prompt15E2EManifest;
  if (!manifest.email || !manifest.password || !manifest.group_action_view_id) {
    throw new Error("prepare Prompt15 E2E: incomplete manifest");
  }
  persistCredentials(manifest.email, manifest.password, manifest.seller_account_id);
  fs.writeFileSync(manifestFile, JSON.stringify(manifest, null, 2));
  return manifest;
}

export function loadPrompt15E2EManifest(): Prompt15E2EManifest {
  if (!fs.existsSync(manifestFile)) {
    throw new Error("Prompt15 E2E manifest missing; run preparePrompt15E2EScenario first");
  }
  return JSON.parse(fs.readFileSync(manifestFile, "utf8")) as Prompt15E2EManifest;
}

export function switchAnalysisCurrentPublication(publicationId: number, sellerId?: number): void {
  runGo([
    "./cmd/devtools",
    "switch-analysis-current-publication",
    `--seller-id=${sellerId ?? getSellerId()}`,
    `--publication-id=${publicationId}`,
  ]);
}
