"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { LoadingState } from "@/components/ui/loading-state";
import { getOzonUserConnection, type OzonUserConnectionDto } from "@/lib/ozon-api";

const CONNECTION_PATH = "/app/integrations/ozon";
const ALLOWED_WITHOUT_CONNECTION = new Set([CONNECTION_PATH, "/app/account"]);
/** Seller UI must not expose technical MVP/sync diagnostics. */
const TECHNICAL_SELLER_PATHS = new Set(["/app", "/app/sync-status"]);

function normalizePath(pathname: string | null): string {
  if (!pathname) return "/app";
  if (pathname.length > 1 && pathname.endsWith("/")) {
    return pathname.slice(0, -1);
  }
  return pathname;
}

function isAllowedPath(path: string): boolean {
  if (ALLOWED_WITHOUT_CONNECTION.has(path)) {
    return true;
  }
  return path.startsWith("/app/admin");
}

type ConnectionGuardProps = {
  isAdminOnly: boolean;
  isDemoSeller?: boolean;
  children: React.ReactNode;
};

export default function ConnectionGuard({
  isAdminOnly,
  isDemoSeller = false,
  children,
}: ConnectionGuardProps) {
  const pathname = usePathname();
  const router = useRouter();
  const path = useMemo(() => normalizePath(pathname), [pathname]);
  const allowed = isAllowedPath(path);
  const technicalPath = TECHNICAL_SELLER_PATHS.has(path);
  const mustCheck = !isAdminOnly && !isDemoSeller && !allowed;

  const [checked, setChecked] = useState(!mustCheck);
  const [connection, setConnection] = useState<OzonUserConnectionDto | null>(null);

  useEffect(() => {
    if (!mustCheck) {
      return;
    }

    let cancelled = false;
    void getOzonUserConnection()
      .then((state) => {
        if (cancelled) return;
        setConnection(state);
        if (technicalPath) {
          router.replace(state.connection_ready ? "/app/dashboard" : CONNECTION_PATH);
          return;
        }
        if (!state.connection_ready) {
          router.replace(CONNECTION_PATH);
        }
      })
      .catch(() => {
        if (!cancelled) {
          router.replace(CONNECTION_PATH);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setChecked(true);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [mustCheck, router, technicalPath]);

  if (isAdminOnly || isDemoSeller || allowed) {
    return <>{children}</>;
  }

  if (technicalPath) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center p-6">
        <LoadingState message="Перенаправление…" />
      </div>
    );
  }

  if (!checked) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center p-6">
        <LoadingState message="Проверка подключения Ozon…" />
      </div>
    );
  }

  if (!connection?.connection_ready) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center p-6">
        <LoadingState message="Перенаправление на подключение Ozon…" />
      </div>
    );
  }

  return <>{children}</>;
}
