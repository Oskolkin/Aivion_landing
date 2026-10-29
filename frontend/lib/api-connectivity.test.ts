import assert from "node:assert/strict";
import { PUBLIC_API_BASE_URL, resolveServerApiBaseUrl } from "@/lib/env/api-base-url";

async function probeHealth(baseUrl: string): Promise<void> {
  const origin = baseUrl.replace(/\/$/, "");
  const res = await fetch(`${origin}/health/live`, { method: "GET" });
  assert.equal(res.ok, true, `health/live failed for ${origin}: ${res.status}`);
  const body = (await res.json()) as { status?: string };
  assert.equal(body.status, "ok", `unexpected live payload from ${origin}`);
}

async function run(): Promise<void> {
  const serverBase = resolveServerApiBaseUrl();
  await probeHealth(serverBase);

  if (PUBLIC_API_BASE_URL && PUBLIC_API_BASE_URL !== serverBase) {
    await probeHealth(PUBLIC_API_BASE_URL);
  }

  console.log("api-connectivity.test.ts: ok");
}

void run();
