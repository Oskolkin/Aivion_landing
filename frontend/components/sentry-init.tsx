"use client";

import { useEffect } from "react";
import { initSentryClient } from "@/lib/sentry/client";

export function SentryInit() {
  useEffect(() => {
    initSentryClient();
  }, []);

  return null;
}