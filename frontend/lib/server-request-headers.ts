import { cookies, headers } from "next/headers";

export const LIVE_REVIEW_HEADER = "x-aivion-live-review";
export const LIVE_REVIEW_TOKEN_HEADER = "x-aivion-live-review-token";
export const LIVE_REVIEW_HOST_HEADER = "x-aivion-live-review-host";

const FORWARD_HEADER_NAMES = [
  LIVE_REVIEW_HEADER,
  LIVE_REVIEW_TOKEN_HEADER,
  LIVE_REVIEW_HOST_HEADER,
] as const;

/**
 * Build headers for server-side fetches to BACKEND_INTERNAL_URL.
 * Forwards Cookie and internal Live Review proxy headers injected by Caddy.
 * Must only be imported from Server Components / server modules.
 */
export async function buildServerBackendHeaders(
  extra?: Record<string, string>,
): Promise<Headers> {
  if (typeof window !== "undefined") {
    throw new Error("buildServerBackendHeaders is server-only");
  }

  const out = new Headers();
  out.set("Content-Type", "application/json");

  const cookieStore = await cookies();
  const cookieHeader = cookieStore.toString();
  if (cookieHeader) {
    out.set("Cookie", cookieHeader);
  }

  const incoming = await headers();
  for (const name of FORWARD_HEADER_NAMES) {
    const value = incoming.get(name);
    if (value != null && value.trim() !== "") {
      out.set(name, value.trim());
    }
  }

  if (extra) {
    for (const [key, value] of Object.entries(extra)) {
      if (value != null && value !== "") {
        out.set(key, value);
      }
    }
  }

  return out;
}

/** Test helper: merge cookie + live-review headers without Next runtime. */
export function mergeServerBackendHeaders(input: {
  cookie?: string | null;
  liveReview?: string | null;
  liveReviewToken?: string | null;
  liveReviewHost?: string | null;
  extra?: Record<string, string>;
}): Record<string, string> {
  const out: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (input.cookie) {
    out.Cookie = input.cookie;
  }
  if (input.liveReview) {
    out[LIVE_REVIEW_HEADER] = input.liveReview;
  }
  if (input.liveReviewToken) {
    out[LIVE_REVIEW_TOKEN_HEADER] = input.liveReviewToken;
  }
  if (input.liveReviewHost) {
    out[LIVE_REVIEW_HOST_HEADER] = input.liveReviewHost;
  }
  if (input.extra) {
    for (const [key, value] of Object.entries(input.extra)) {
      if (value) {
        out[key] = value;
      }
    }
  }
  return out;
}
