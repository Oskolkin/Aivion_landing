"use client";

import * as Sentry from "@sentry/nextjs";

let initialized = false;

export function initSentryClient() {
  if (initialized) return;
  const dsn = process.env.NEXT_PUBLIC_SENTRY_DSN;

  if (!dsn) return;

  Sentry.init({
    dsn,
    environment: process.env.NEXT_PUBLIC_APP_ENV || "local",
    release: process.env.NEXT_PUBLIC_SENTRY_RELEASE || "dev",
  });

  initialized = true;
}