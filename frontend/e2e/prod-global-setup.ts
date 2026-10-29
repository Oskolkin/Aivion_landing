import type { FullConfig } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

const authFile = path.join(__dirname, ".auth", "seller.json");
const backendUrl = process.env.BACKEND_PROXY_URL ?? "http://127.0.0.1:8081";
const email = process.env.E2E_DEMO_EMAIL ?? "demo@example.com";
const password = process.env.E2E_DEMO_PASSWORD ?? "password123";

/**
 * Lightweight production setup: login against running backend, write storageState.
 * Does not reseed — expects seller 1 + unified publication already prepared.
 */
export default async function globalSetup(_config: FullConfig): Promise<void> {
  fs.mkdirSync(path.dirname(authFile), { recursive: true });

  const loginRes = await fetch(`${backendUrl}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!loginRes.ok) {
    throw new Error(`prod login failed: ${loginRes.status} ${await loginRes.text()}`);
  }

  const setCookie = loginRes.headers.getSetCookie?.() ?? [];
  const legacy = loginRes.headers.get("set-cookie");
  const cookieHeaders = setCookie.length > 0 ? setCookie : legacy ? [legacy] : [];
  if (cookieHeaders.length === 0) {
    throw new Error("prod login returned no Set-Cookie");
  }

  const cookies = cookieHeaders.map((header) => {
    const [pair] = header.split(";");
    const eq = pair.indexOf("=");
    const name = pair.slice(0, eq).trim();
    const value = pair.slice(eq + 1).trim();
    return {
      name,
      value,
      domain: "localhost",
      path: "/",
      httpOnly: true,
      sameSite: "Lax" as const,
    };
  });

  fs.writeFileSync(
    authFile,
    JSON.stringify({ cookies, origins: [] }, null, 2),
  );

  // Sanity: dashboard home available
  const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join("; ");
  const home = await fetch(`${backendUrl}/api/v1/dashboard/home`, {
    headers: { Cookie: cookieHeader },
  });
  if (!home.ok) {
    throw new Error(`dashboard/home not ready: ${home.status} ${await home.text()}`);
  }
  const body = (await home.json()) as { status?: string; metric_date?: string };
  console.log(
    `[prod-setup] dashboard status=${body.status} metric_date=${body.metric_date} cookies=${cookies.length}`,
  );
}
