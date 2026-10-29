import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const LIVE_REVIEW_HEADERS = [
  "x-aivion-live-review",
  "x-aivion-live-review-token",
  "x-aivion-live-review-host",
] as const;

/**
 * Preserve Caddy-injected Live Review headers across Next rewrites to the backend.
 * Without this, some Next versions may drop hop-by-hop or non-standard headers
 * when proxying /api/v1/* — which would break SSR-independent client API calls
 * on the temporary review hostname.
 */
export function middleware(request: NextRequest) {
  const requestHeaders = new Headers(request.headers);
  let touched = false;
  for (const name of LIVE_REVIEW_HEADERS) {
    const value = request.headers.get(name);
    if (value != null && value.trim() !== "") {
      requestHeaders.set(name, value.trim());
      touched = true;
    }
  }
  if (!touched) {
    return NextResponse.next();
  }
  return NextResponse.next({
    request: {
      headers: requestHeaders,
    },
  });
}

export const config = {
  matcher: ["/api/v1/:path*", "/app/:path*", "/live-review-index"],
};
