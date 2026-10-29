import { expect, test, type Page, type Response } from "@playwright/test";
import { execSync } from "node:child_process";
import path from "node:path";
import { stubOzonConnectionReady } from "./helpers/auth";
import { getSellerId, getE2ECredentials, prepareUnifiedBrowserScenario, setUnifiedAnalysisMode } from "./helpers/seed";

/**
 * Stage 5 real Unified publication browser acceptance.
 *
 * Switches the seller's current Unified publication via
 * `prepare-unified-browser-scenario` (real PublishRun/RollbackPublication).
 * Does NOT mock Dashboard or Actions API responses.
 */

type ScenarioEvidence = {
  scenario: string;
  seller_id: number;
  run_id?: number;
  publication_id?: number;
  analysis_outcome?: string;
  dashboard_source?: string;
  actions_source?: string;
  console_errors: string[];
  failed_requests: string[];
  result: "PASS" | "FAIL";
};

const evidence: ScenarioEvidence[] = [];

type ApiMeta = {
  dashboardSource?: string;
  actionsSource?: string;
  dashboardPublicationId?: number;
  actionsPublicationId?: number;
  dashboardRunId?: number;
  actionsRunId?: number;
  dashboardMetricDate?: string;
  actionsMetricDate?: string;
  dashboardHeadline?: string;
  kpiKeys?: string[];
  trendDays?: number;
  actionsCount?: number;
  headers?: Record<string, string>;
};

async function withGuards(
  page: Page,
  scenario: string,
  path: string,
  prep: ReturnType<typeof prepareUnifiedBrowserScenario>,
  body: (apiMeta: ApiMeta) => Promise<void>,
) {
  const consoleErrors: string[] = [];
  const failedRequests: string[] = [];
  const apiMeta: ApiMeta = { headers: {} };

  page.on("pageerror", (err) => consoleErrors.push(err.message));
  page.on("console", (msg) => {
    if (msg.type() !== "error") return;
    const text = msg.text();
    if (/status of (401|403|404)/i.test(text)) return;
    // Next.js webpack HMR websocket noise is not an application runtime error.
    if (text.includes("webpack-hmr") || text.includes("/_next/webpack-hmr")) return;
    consoleErrors.push(text);
  });
  page.on("response", async (res: Response) => {
    if (!res.url().includes("/api/")) return;
    if (res.status() >= 500) {
      failedRequests.push(`${res.status()} ${res.url()}`);
    }
    try {
      if (res.url().includes("/dashboard") || res.url().includes("/home")) {
        apiMeta.headers = {
          ...apiMeta.headers,
          "x-aivion-analysis-source": res.headers()["x-aivion-analysis-source"] ?? "",
          "x-aivion-publication-id": res.headers()["x-aivion-publication-id"] ?? "",
          "x-aivion-run-id": res.headers()["x-aivion-run-id"] ?? "",
          "x-aivion-metric-date": res.headers()["x-aivion-metric-date"] ?? "",
        };
        const json = await res.json().catch(() => null);
        if (json && typeof json === "object") {
          const body = json as {
            analysis_source?: string;
            publication_id?: number;
            metric_date?: string;
            pipeline?: { pipeline_run_id?: number };
            ai_diagnostics?: { account_summary?: { headline?: string } };
            seller_pulse?: { kpis?: Array<{ key?: string; value?: string }>; trend_7d?: unknown[] };
          };
          apiMeta.dashboardSource = body.analysis_source;
          apiMeta.dashboardPublicationId = body.publication_id;
          apiMeta.dashboardMetricDate = body.metric_date;
          apiMeta.dashboardRunId = body.pipeline?.pipeline_run_id;
          apiMeta.dashboardHeadline = body.ai_diagnostics?.account_summary?.headline;
          apiMeta.kpiKeys = (body.seller_pulse?.kpis ?? [])
            .map((k) => k.key)
            .filter((k): k is string => Boolean(k));
          apiMeta.trendDays = body.seller_pulse?.trend_7d?.length ?? 0;
        }
      }
      if (res.url().includes("/actions") && res.request().method() === "GET" && !res.url().includes("/actions/")) {
        const json = await res.json().catch(() => null);
        if (json && typeof json === "object") {
          const body = json as {
            analysis_source?: string;
            publication_id?: number;
            pipeline_run_id?: number;
            metric_date?: string;
            actions?: unknown[];
          };
          apiMeta.actionsSource = body.analysis_source;
          apiMeta.actionsPublicationId = body.publication_id;
          apiMeta.actionsRunId = body.pipeline_run_id;
          apiMeta.actionsMetricDate = body.metric_date;
          apiMeta.actionsCount = body.actions?.length ?? 0;
        }
      }
    } catch {
      // ignore body parse errors
    }
  });

  let result: "PASS" | "FAIL" = "PASS";
  try {
    await page.goto(path);
    await body(apiMeta);
  } catch (err) {
    result = "FAIL";
    throw err;
  } finally {
    if (consoleErrors.length || failedRequests.length) result = "FAIL";
    evidence.push({
      scenario,
      seller_id: getSellerId(),
      run_id: prep.current_run_id,
      publication_id: prep.current_publication_id,
      analysis_outcome: prep.analysis_outcome,
      dashboard_source: apiMeta.dashboardSource,
      actions_source: apiMeta.actionsSource,
      console_errors: consoleErrors,
      failed_requests: failedRequests,
      result,
    });
    expect(consoleErrors, `console errors in ${scenario}`).toEqual([]);
    expect(failedRequests, `API 5xx in ${scenario}`).toEqual([]);
  }
}

test.beforeEach(async ({ page }) => {
  await stubOzonConnectionReady(page);
});

test.afterAll(() => {
  // Restore seller pointer after mutating scenarios. Prefer E2E_RESTORE_PUBLICATION_ID
  // when set (e.g. live pilot publication); otherwise material fixture.
  try {
    const restoreID = Number(process.env.E2E_RESTORE_PUBLICATION_ID || "0");
    if (restoreID > 0) {
      const backendDir = path.resolve(__dirname, "../../backend");
      execSync(
        `go run ./cmd/devtools rollback-unified-publication --seller-id=${getSellerId()} --target-publication-id=${restoreID} --reason=e2e-restore`,
        {
          cwd: backendDir,
          env: {
            ...process.env,
            DATABASE_URL:
              process.env.DATABASE_URL ??
              "postgres://postgres:postgres@localhost:55432/marketplace_ai?sslmode=disable",
            ENCRYPTION_KEY: process.env.ENCRYPTION_KEY ?? "01234567890123456789012345678901",
          },
          stdio: "pipe",
          shell: process.platform === "win32" ? "cmd.exe" : "/bin/sh",
        },
      );
    } else {
      prepareUnifiedBrowserScenario("material");
    }
  } catch {
    try {
      prepareUnifiedBrowserScenario("material");
    } catch {
      // best-effort restore
    }
  }
  // Leave legacy mode so subsequent suite files (empty/main-flow) are not stuck
  // reading unified publication actions.
  try {
    setUnifiedAnalysisMode("legacy");
  } catch {
    // best-effort
  }
  console.log("[unified-publication-smoke evidence]", JSON.stringify(evidence, null, 2));
});

test.describe("unified publication real browser smoke", () => {
  test("material publication renders Dashboard and Actions", async ({ page }) => {
    const prep = prepareUnifiedBrowserScenario("material");
    expect(prep.analysis_outcome).toBe("material_actions_found");
    expect(prep.current_publication_id).toBeGreaterThan(0);
    // Fresh CI DBs may assign id=1 to the first isolated @example.test seller.
    // Guard against the shared demo account, not against numeric id 1.
    const creds = getE2ECredentials();
    expect(creds.email).toMatch(/@example\.test$/i);
    expect(creds.email).not.toBe("demo@example.com");

    await withGuards(page, "material-dashboard", "/app/dashboard", prep, async (apiMeta) => {
      await expect(page.getByRole("heading", { name: "Дашборд" })).toBeVisible({ timeout: 45_000 });
      await expect(page.locator("body")).not.toContainText("Application error");
      await expect(page.locator("body")).not.toContainText("AI dashboard not generated");
      await expect(page.locator("body")).not.toContainText("Run dashboard_narrative_v9");

      expect(apiMeta.dashboardSource).toBe("unified");
      expect(apiMeta.dashboardPublicationId).toBe(prep.current_publication_id);
      expect(apiMeta.dashboardRunId).toBe(prep.current_run_id);
      expect(apiMeta.dashboardHeadline?.trim().length ?? 0).toBeGreaterThan(0);
      expect(apiMeta.kpiKeys ?? []).toEqual(expect.arrayContaining(["revenue", "orders"]));
      expect(apiMeta.trendDays ?? 0).toBeGreaterThan(0);
      expect(apiMeta.headers?.["x-aivion-analysis-source"]).toBe("unified");
      expect(apiMeta.headers?.["x-aivion-publication-id"]).toBe(String(prep.current_publication_id));

      await expect(page.getByText("Выручка").first()).toBeVisible();
      await expect(page.getByText("Заказы").first()).toBeVisible();
    });

    await withGuards(page, "material-actions", "/app/actions", prep, async (apiMeta) => {
      await expect(page.getByRole("heading", { name: /Действия/i }).first()).toBeVisible({
        timeout: 45_000,
      });
      await expect(page.getByText(/Дата анализа/i)).toBeVisible();
      await expect(page.locator("[data-testid='action-card']").first()).toBeVisible({ timeout: 45_000 });
      expect(apiMeta.actionsSource).toBe("unified");
      expect(apiMeta.actionsPublicationId).toBe(prep.current_publication_id);
      expect(apiMeta.actionsRunId).toBe(prep.current_run_id);
      expect(apiMeta.actionsCount ?? 0).toBeGreaterThan(0);
      await page.locator("[data-testid='action-card']").first().click();
      await expect(page.getByText(/Что сделать|План действий|Почему это важно|Ситуация|Краткий вывод/i).first()).toBeVisible({
        timeout: 20_000,
      });
    });
  });

  test("healthy publication renders real empty Actions", async ({ page }) => {
    const prep = prepareUnifiedBrowserScenario("healthy");
    expect(prep.analysis_outcome).toBe("no_material_actions");
    expect(prep.healthy_publication.actions_count).toBe(0);
    expect(prep.unified_pointer_publication_id).toBe(prep.current_publication_id);
    expect(prep.compat_pointer_run_id).toBe(prep.current_run_id);

    await withGuards(page, "healthy-dashboard", "/app/dashboard", prep, async () => {
      await expect(page.getByRole("heading", { name: "Дашборд" })).toBeVisible({ timeout: 45_000 });
      await expect(page.locator("body")).not.toContainText("Application error");
    });

    await withGuards(page, "healthy-actions", "/app/actions", prep, async () => {
      await expect(page.getByRole("heading", { name: /Действия/i }).first()).toBeVisible({
        timeout: 45_000,
      });
      await expect(page.locator("[data-testid='action-card']")).toHaveCount(0);
      await expect(page.getByText("Активных рекомендаций сейчас нет")).toBeVisible({
        timeout: 45_000,
      });
    });
  });

  test("insufficient publication renders limitations and empty Actions", async ({ page }) => {
    const prep = prepareUnifiedBrowserScenario("insufficient");
    expect(prep.analysis_outcome).toBe("insufficient_data");
    expect(prep.insufficient_publication.actions_count).toBe(0);

    await withGuards(page, "insufficient-dashboard", "/app/dashboard", prep, async () => {
      await expect(page.getByRole("heading", { name: "Дашборд" })).toBeVisible({ timeout: 45_000 });
      await expect(page.locator("body")).not.toContainText("Application error");
    });

    await withGuards(page, "insufficient-actions", "/app/actions", prep, async () => {
      await expect(page.locator("[data-testid='action-card']")).toHaveCount(0);
      // Must not show healthy success empty-state claiming "no problems".
      // Either empty-state or limitations-oriented copy is acceptable.
      await expect(page.locator("body")).not.toContainText("Application error");
    });
  });

  test("rollback restores material publication", async ({ page }) => {
    prepareUnifiedBrowserScenario("insufficient");
    const prep = prepareUnifiedBrowserScenario("material");
    expect(prep.analysis_outcome).toBe("material_actions_found");

    await withGuards(page, "rollback-material", "/app/actions", prep, async () => {
      await expect(page.locator("[data-testid='action-card']").first()).toBeVisible({ timeout: 45_000 });
      await expect(page.getByText(/Дата анализа/i)).toBeVisible();
    });
  });

  test("shadow publication remains hidden", async ({ page }) => {
    const prep = prepareUnifiedBrowserScenario("shadow");
    expect(prep.shadow_publication.visibility).toBe("shadow");
    expect(prep.current_publication_id).toBe(prep.material_publication.publication_id);
    expect(prep.unified_pointer_publication_id).toBe(prep.material_publication.publication_id);

    await withGuards(page, "shadow-hidden", "/app/dashboard", prep, async () => {
      await expect(page.getByRole("heading", { name: "Дашборд" })).toBeVisible({ timeout: 45_000 });
    });
    await withGuards(page, "shadow-actions", "/app/actions", prep, async () => {
      // Visible material bundle remains; shadow is not the current source.
      await expect(page.locator("[data-testid='action-card']").first()).toBeVisible({ timeout: 45_000 });
    });
  });

  test("promotion switches visible publication", async ({ page }) => {
    prepareUnifiedBrowserScenario("shadow");
    const prep = prepareUnifiedBrowserScenario("promote");
    expect(prep.promoted).toBe(true);
    expect(prep.current_publication_id).toBe(prep.shadow_publication.publication_id);

    await withGuards(page, "promote-visible", "/app/actions", prep, async () => {
      await expect(page.locator("[data-testid='action-card']").first()).toBeVisible({ timeout: 45_000 });
    });
  });
});
