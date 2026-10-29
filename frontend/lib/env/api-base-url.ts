/**
 * Browser-facing API origin. Empty string means same-origin (/api/v1 via Next rewrite).
 * Prefer unset/empty in app shells that use next.config.ts rewrites.
 */
function readPublicApiBaseUrl(): string {
  const env = process.env.NEXT_PUBLIC_API_BASE_URL;
  if (env !== undefined) {
    return env.trim();
  }
  // Same-origin by default so client fetches hit Next rewrites, not a hard-coded API port.
  return "";
}

export const PUBLIC_API_BASE_URL = readPublicApiBaseUrl();

/** Server-side fetches in Docker/SSR use internal/proxy URL when set. */
export function resolveServerApiBaseUrl(): string {
  const internal = process.env.BACKEND_INTERNAL_URL?.trim();
  if (internal) {
    return internal;
  }
  const proxy = process.env.BACKEND_PROXY_URL?.trim();
  if (proxy) {
    return proxy;
  }
  if (PUBLIC_API_BASE_URL) {
    return PUBLIC_API_BASE_URL;
  }
  return "http://localhost:8081";
}
