/**
 * Debug live dashboard funnel grouping. Usage:
 *   npx tsx lib/funnel-live-debug.tsx [path-to-dashboard-home.json]
 */
import { readFileSync, existsSync } from "node:fs";
import { join } from "node:path";

function loadEnvLocal(): void {
  const path = join(process.cwd(), ".env.local");
  if (!existsSync(path)) return;
  for (const line of readFileSync(path, "utf8").split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const eq = trimmed.indexOf("=");
    if (eq < 0) continue;
    const key = trimmed.slice(0, eq).trim();
    const value = trimmed.slice(eq + 1).trim();
    if (key.startsWith("NEXT_PUBLIC_")) process.env[key] = value;
  }
}

loadEnvLocal();

import { renderToStaticMarkup } from "react-dom/server";
import { FunnelAIAnalysisContent } from "@/components/dashboard/funnel-ai-analysis";
import { normalizeDashboardHome } from "@/lib/normalize-dashboard-home";
import {
  buildPreviousComparisonGroups,
  buildSevenDayComparisonGroups,
} from "@/lib/funnel-ai-analysis-view";

function apiBaseUrl(): string {
  return process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8081";
}

const defaultPath = join(process.cwd(), "..", "backend", "tmp", "live-verify-dashboard-home.json");

async function loadHome() {
  const arg = process.argv[2];
  if (arg && existsSync(arg)) {
    return normalizeDashboardHome(JSON.parse(readFileSync(arg, "utf8")));
  }
  if (existsSync(defaultPath)) {
    return normalizeDashboardHome(JSON.parse(readFileSync(defaultPath, "utf8")));
  }
  const { getDashboardHome } = await import("@/lib/dashboard-home-api");
  console.log("Fetching", `${apiBaseUrl()}/api/v1/dashboard/home`, "(needs auth cookie — use saved JSON)");
  return normalizeDashboardHome(await getDashboardHome());
}

function printGroups(label: string, groups: ReturnType<typeof buildPreviousComparisonGroups>) {
  console.log(`\n${label}`);
  for (const [name, items] of [
    ["Этапы", groups.stages],
    ["Конверсии", groups.conversions],
    ["Экономика", groups.business],
  ] as const) {
    console.log(`  ${name}:`);
    for (const it of items) {
      console.log(`    - ${it.key} | ${it.label} | ${it.delta ?? it.value ?? "—"}`);
    }
    if (items.length === 0) console.log("    (empty)");
  }
}

async function main() {
  console.log("API base:", apiBaseUrl());
  const home = await loadHome();
  const analysis = home.ai_diagnostics.funnel_analysis.ai_analysis;
  if (!analysis || analysis.status !== "available") {
    console.error("ai_analysis not available:", analysis?.status);
    process.exit(1);
  }

  const prev = buildPreviousComparisonGroups(analysis);
  const seven = buildSevenDayComparisonGroups(analysis);
  printGroups("previousComparisonGroups", prev);
  printGroups("sevenDayComparisonGroups", seven);

  const addInSevenStages = seven.stages.some((s) => s.key === "add_to_cart");
  const addInSevenConv = seven.conversions.some((s) => s.key === "add_to_cart");
  console.log("\nChecks:");
  console.log("  add_to_cart in 7d stages:", addInSevenStages);
  console.log("  add_to_cart in 7d conversions (BAD):", addInSevenConv);
  console.log("  AOV in 7d business:", seven.business.some((b) => b.key === "average_order_value"));

  const html = renderToStaticMarkup(
    <FunnelAIAnalysisContent
      analysis={analysis}
      primaryDropStage={home.ai_diagnostics.funnel_analysis.primary_drop_stage}
      primaryDropMessage={home.ai_diagnostics.funnel_analysis.primary_drop_message}
      salesStages={home.ai_diagnostics.sales_funnel.stages ?? []}
    />,
  );
  console.log("\nHTML markers:");
  console.log("  изменение количества:", html.includes("изменение количества"));
  console.log("  dashed empty previous:", html.includes("Нет достаточно данных для сравнения с прошлой сводкой."));
  console.log("  dashed empty 7d:", html.includes("Пока недостаточно истории для сравнения со средним уровнем."));
  console.log("  Корзина → заказ (legacy):", /Корзина → заказ[^а-я]/i.test(html));
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
