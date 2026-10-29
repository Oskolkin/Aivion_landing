"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { getAdminMe } from "@/lib/admin-api";

export default function AdminSupportLink() {
  const [checked, setChecked] = useState(false);
  const [isAdmin, setIsAdmin] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void getAdminMe()
      .then((result) => {
        if (!cancelled && result.is_admin) {
          setIsAdmin(true);
        }
      })
      .catch(() => {
        // Non-admin/forbidden errors are intentionally silent.
      })
      .finally(() => {
        if (!cancelled) {
          setChecked(true);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (!checked || !isAdmin) {
    return null;
  }

  return (
    <Link
      href="/app/admin"
      className="rounded border px-4 py-2 hover:bg-aivion-surface-soft"
    >
      <span className="block">Админка / поддержка</span>
      <span className="text-xs text-aivion-muted">
        Внутренние инструменты поддержки: клиенты, синхронизация, диагностика ИИ, отзывы и биллинг.
      </span>
    </Link>
  );
}
