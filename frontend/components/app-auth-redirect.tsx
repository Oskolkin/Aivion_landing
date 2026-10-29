"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";

const ADMIN_ONLY_PREFIX = "/app/admin";

function isAdminRoute(pathname: string): boolean {
  return pathname === ADMIN_ONLY_PREFIX || pathname.startsWith(`${ADMIN_ONLY_PREFIX}/`);
}

type AppAuthRedirectProps = {
  isAdminOnly: boolean;
  isReviewMode?: boolean;
};

export default function AppAuthRedirect({ isAdminOnly, isReviewMode = false }: AppAuthRedirectProps) {
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    if (!pathname) return;
    if (isReviewMode && isAdminRoute(pathname)) {
      router.replace("/app/dashboard");
      return;
    }
    if (!isAdminOnly) return;
    if (!isAdminRoute(pathname)) {
      router.replace("/app/admin");
    }
  }, [isAdminOnly, isReviewMode, pathname, router]);

  return null;
}
