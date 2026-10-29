"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { AivionLogo, AivionMark } from "@/components/brand/aivion-logo";
import { Button } from "@/components/ui/button";
import { cn } from "@/components/ui/cn";
import { getAdminMe } from "@/lib/admin-api";
import { logout } from "@/lib/auth-api";
import { isDashboardPath } from "@/lib/dashboard-routes";
import { nav, ui } from "@/lib/ui-copy";

type NavItem =
  | { kind: "link"; href: string; label: string; icon: React.ReactNode }
  | { kind: "disabled"; label: string; hint: string; icon: React.ReactNode };

function NavIcon({ d }: { d: string }) {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden className="shrink-0">
      <path d={d} stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

const ICONS = {
  dashboard:
    "M3 10.5L12 3l9 7.5V20a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1v-9.5z",
  actions: "M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2",
  chat: "M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v10z",
  account: "M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2M12 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z",
  ozon: "M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zm0 6v4l3 3",
  admin: "M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zm7-3a7 7 0 1 1-14 0 7 7 0 0 1 14 0z",
};

const SELLER_NAV: NavItem[] = [
  { kind: "link", href: "/app/dashboard", label: nav.dashboard, icon: <NavIcon d={ICONS.dashboard} /> },
  { kind: "link", href: "/app/actions", label: nav.actions, icon: <NavIcon d={ICONS.actions} /> },
  { kind: "link", href: "/app/chat", label: nav.aiChat, icon: <NavIcon d={ICONS.chat} /> },
  { kind: "link", href: "/app/account", label: nav.sellerAccount, icon: <NavIcon d={ICONS.account} /> },
  {
    kind: "link",
    href: "/app/integrations/ozon",
    label: nav.ozonConnection,
    icon: <NavIcon d={ICONS.ozon} />,
  },
];

const ADMIN_ONLY_NAV: NavItem[] = [
  { kind: "link", href: "/app/admin", label: nav.adminSupport, icon: <NavIcon d={ICONS.admin} /> },
];

const SECTION_TITLE_RULES: Array<{ prefix: string; title: string }> = [
  { prefix: "/app/dashboard", title: nav.dashboard },
  { prefix: "/app/actions", title: nav.actions },
  { prefix: "/app/chat", title: nav.aiChat },
  { prefix: "/app/products", title: nav.products },
  { prefix: "/app/advertising", title: nav.advertising },
  { prefix: "/app/critical-skus", title: nav.criticalSkus },
  { prefix: "/app/stocks-replenishment", title: nav.stocksReplenishment },
  { prefix: "/app/alerts", title: nav.alerts },
  { prefix: "/app/recommendations", title: nav.recommendations },
  { prefix: "/app/integrations/ozon", title: nav.ozonConnection },
  { prefix: "/app/account", title: nav.sellerAccount },
  { prefix: "/app/admin", title: nav.adminSupport },
];

function normalizePath(pathname: string | null): string {
  if (!pathname) return "/app";
  if (pathname.length > 1 && pathname.endsWith("/")) {
    return pathname.slice(0, -1);
  }
  return pathname;
}

function sectionTitle(pathname: string | null, isAdminOnly: boolean): string {
  const p = normalizePath(pathname);
  if (isAdminOnly) return nav.adminSupport;
  if (p === "/app") return nav.dashboard;
  for (const { prefix, title } of SECTION_TITLE_RULES) {
    if (p === prefix || p.startsWith(`${prefix}/`)) return title;
  }
  return ui.productName;
}

function isChatPage(pathname: string | null): boolean {
  const p = normalizePath(pathname);
  return p === "/app/chat" || p.startsWith("/app/chat/");
}

function isNavActive(pathname: string | null, href: string): boolean {
  const p = normalizePath(pathname);
  if (href === "/app") return p === "/app";
  return p === href || p.startsWith(`${href}/`);
}

const navLinkClass = (active: boolean) =>
  cn(
    "flex items-center gap-2.5 rounded-aivion-md px-3 py-2 text-sm font-medium transition-colors",
    active
      ? "bg-[#eff8ff] text-aivion-blue"
      : "text-aivion-muted hover:bg-aivion-surface-soft hover:text-aivion-text",
  );

type AppShellProps = {
  children: React.ReactNode;
  userEmail: string;
  sellerAccountName: string | null;
  isAdminOnly: boolean;
  isAdmin: boolean;
  isDemoSeller?: boolean;
  isReviewMode?: boolean;
};

export default function AppShell({
  children,
  userEmail,
  sellerAccountName,
  isAdminOnly,
  isAdmin: isAdminFromAuth,
  isDemoSeller = false,
  isReviewMode = false,
}: AppShellProps) {
  const pathname = usePathname();
  const router = useRouter();
  const chatPage = isChatPage(pathname);
  const dashboardPage = isDashboardPath(pathname);
  const title = useMemo(() => sectionTitle(pathname, isAdminOnly), [pathname, isAdminOnly]);
  const homeHref = isAdminOnly ? "/app/admin" : "/app/dashboard";
  const accountLabel = isAdminOnly ? nav.adminSupport : (sellerAccountName ?? "");
  /** Dashboard section owns its PageHeader — suppress AppShell title strip to avoid duplicate «Дашборд». */
  const suppressShellTitle = chatPage || dashboardPage;

  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [showAdminNav, setShowAdminNav] = useState(isAdminFromAuth && !isReviewMode);
  const [adminChecked, setAdminChecked] = useState(isAdminOnly || isAdminFromAuth || isReviewMode);
  const [loggingOut, setLoggingOut] = useState(false);

  useEffect(() => {
    if (isAdminOnly || isAdminFromAuth || isReviewMode) return;
    let cancelled = false;
    void getAdminMe()
      .then((result) => {
        if (!cancelled && result.is_admin) setShowAdminNav(true);
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setAdminChecked(true);
      });
    return () => {
      cancelled = true;
    };
  }, [isAdminOnly, isAdminFromAuth, isReviewMode]);

  useEffect(() => {
    setMobileNavOpen(false);
  }, [pathname]);

  const handleLogout = useCallback(async () => {
    setLoggingOut(true);
    try {
      await logout();
    } catch {
      // Still navigate away
    } finally {
      setLoggingOut(false);
      router.push("/login");
      router.refresh();
    }
  }, [router]);

  const navItems = useMemo(() => {
    if (isAdminOnly) return ADMIN_ONLY_NAV;
    const items = SELLER_NAV.filter((item) => {
      if (item.kind !== "link") return true;
      if ((isDemoSeller || isReviewMode) && item.href === "/app/integrations/ozon") {
        return false;
      }
      if (isReviewMode && item.href === "/app/account") {
        return false;
      }
      return true;
    });
    if (showAdminNav && !isDemoSeller && !isReviewMode) {
      items.push({
        kind: "link",
        href: "/app/admin",
        label: nav.adminSupport,
        icon: <NavIcon d={ICONS.admin} />,
      });
    }
    return items;
  }, [isAdminOnly, showAdminNav, isDemoSeller, isReviewMode]);

  const renderNavItems = (onNavigate?: () => void) => (
    <nav className="flex flex-col gap-0.5 p-3" aria-label={ui.primaryNav}>
      {navItems.map((item) => {
        if (item.kind === "disabled") {
          return (
            <div
              key={item.label}
              className="flex items-center gap-2.5 rounded-aivion-md px-3 py-2 text-sm text-aivion-muted-soft"
              title={item.hint}
            >
              {item.icon}
              <span>
                {item.label}
                <span className="mt-0.5 block text-xs">{item.hint}</span>
              </span>
            </div>
          );
        }
        if (item.href === "/app/admin" && !isAdminOnly && (!adminChecked || !showAdminNav)) {
          return null;
        }
        const active = isNavActive(pathname, item.href);
        return (
          <Link
            key={item.href}
            href={item.href}
            className={navLinkClass(active)}
            onClick={() => onNavigate?.()}
            aria-current={active ? "page" : undefined}
          >
            {item.icon}
            {item.label}
          </Link>
        );
      })}
    </nav>
  );

  return (
    <div className={cn("flex flex-col overflow-x-hidden bg-aivion-bg md:flex-row", chatPage ? "h-dvh max-h-dvh overflow-hidden" : "min-h-screen")}>
      {/* Mobile header */}
      <div className="sticky top-0 z-30 flex shrink-0 flex-col border-b border-aivion-border bg-aivion-surface md:hidden">
        <div className="flex h-14 items-center justify-between gap-3 px-4">
          <Link href={homeHref} className="flex min-w-0 items-center gap-2">
            <AivionMark size="small" />
            {!dashboardPage ? (
              <span className="truncate text-sm font-semibold text-aivion-text">{title}</span>
            ) : (
              <span className="sr-only">{title}</span>
            )}
          </Link>
          <button
            type="button"
            className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-aivion-md border border-aivion-border bg-aivion-surface text-aivion-text hover:bg-aivion-surface-soft"
            aria-expanded={mobileNavOpen}
            aria-controls="app-mobile-nav"
            aria-label={mobileNavOpen ? ui.closeMenu : ui.openMenu}
            onClick={() => setMobileNavOpen((o) => !o)}
          >
            {mobileNavOpen ? (
              <svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
                <path
                  fill="currentColor"
                  d="M18.3 5.71a1 1 0 0 0-1.41 0L12 10.59 7.11 5.7A1 1 0 0 0 5.7 7.11L10.59 12l-4.9 4.89a1 1 0 1 0 1.41 1.41L12 13.41l4.89 4.9a1 1 0 0 0 1.41-1.41L13.41 12l4.9-4.89a1 1 0 0 0 0-1.4z"
                />
              </svg>
            ) : (
              <svg width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
                <path fill="currentColor" d="M4 6h16v2H4V6zm0 5h16v2H4v-2zm0 5h16v2H4v-2z" />
              </svg>
            )}
          </button>
        </div>
        {mobileNavOpen ? (
          <div
            id="app-mobile-nav"
            className="max-h-[min(70vh,calc(100dvh-3.5rem))] overflow-y-auto border-t border-aivion-border"
          >
            {renderNavItems(() => setMobileNavOpen(false))}
            <div className="space-y-3 border-t border-aivion-border p-3">
              <div className="text-xs text-aivion-muted">
                <div className="font-medium text-aivion-text">{userEmail}</div>
                {accountLabel ? <div>{accountLabel}</div> : null}
              </div>
              <Button variant="secondary" className="w-full" disabled={loggingOut} onClick={() => void handleLogout()}>
                {loggingOut ? ui.signingOut : ui.logout}
              </Button>
            </div>
          </div>
        ) : null}
      </div>

      {/* Desktop sidebar */}
      <aside className="hidden w-60 shrink-0 flex-col border-r border-aivion-border bg-aivion-surface md:flex">
        <div className="border-b border-aivion-border px-5 py-5">
          <Link href={homeHref} className="inline-flex transition-opacity hover:opacity-85">
            <AivionLogo size="default" />
          </Link>
          {accountLabel && !isAdminOnly ? (
            <p className="mt-3 truncate text-xs text-aivion-muted">{accountLabel}</p>
          ) : isAdminOnly ? (
            <p className="mt-3 text-xs text-aivion-muted">{ui.admin}</p>
          ) : null}
        </div>
        <div className="flex-1 overflow-y-auto">{renderNavItems()}</div>
        <div className="border-t border-aivion-border p-4">
          <p className="truncate text-xs text-aivion-muted">{userEmail}</p>
          <Button
            variant="ghost"
            className="mt-2 w-full justify-start px-2 text-aivion-muted"
            disabled={loggingOut}
            onClick={() => void handleLogout()}
          >
            {loggingOut ? ui.signingOut : ui.logout}
          </Button>
        </div>
      </aside>

      <div
        className={cn(
          "flex min-w-0 flex-1 flex-col",
          chatPage && "min-h-0 flex-1 overflow-hidden",
        )}
      >
        {!suppressShellTitle ? (
          <header
            className="hidden h-14 shrink-0 items-center justify-between gap-4 border-b border-aivion-border bg-aivion-surface px-6 md:flex"
            data-testid="app-shell-section-title"
          >
            <h1 className="truncate text-lg font-semibold text-aivion-text">{title}</h1>
          </header>
        ) : null}

        <main
          className={cn(
            "flex-1 min-w-0",
            chatPage ? "flex min-h-0 flex-col overflow-hidden" : "overflow-y-auto overflow-x-hidden",
          )}
        >
          {isReviewMode ? (
            <div
              className="border-b border-sky-200 bg-sky-50 px-4 py-2 text-center text-xs text-sky-900 sm:text-sm"
              data-testid="review-mode-banner"
              role="status"
            >
              Демонстрационный режим — только просмотр
            </div>
          ) : null}
          {children}
        </main>
      </div>
    </div>
  );
}
