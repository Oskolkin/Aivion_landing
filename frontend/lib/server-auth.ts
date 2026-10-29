import type { AuthResponse } from "@/lib/auth-types";
import { resolveServerApiBaseUrl } from "@/lib/env/api-base-url";
import { buildServerBackendHeaders } from "@/lib/server-request-headers";

export async function getCurrentUserServer(): Promise<AuthResponse | null> {
  const headers = await buildServerBackendHeaders();

  const res = await fetch(`${resolveServerApiBaseUrl()}/api/v1/auth/me`, {
    method: "GET",
    headers,
    cache: "no-store",
  });

  if (!res.ok) {
    return null;
  }

  return res.json() as Promise<AuthResponse>;
}
