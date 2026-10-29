import { clientEnv } from "@/lib/env/client";

export async function apiGet<T>(path: string): Promise<T> {
  const response = await fetch(`${clientEnv.apiBaseUrl}${path}`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
    cache: "no-store",
  });

  if (!response.ok) {
    throw new Error(`API request failed: ${response.status}`);
  }

  return response.json();
}