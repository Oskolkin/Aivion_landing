import type { ReactNode } from "react";
import Link from "next/link";
import { AivionLogo } from "@/components/brand/aivion-logo";
import { cn } from "@/components/ui/cn";

type AuthLayoutProps = {
  children: ReactNode;
  title: string;
  subtitle?: string;
  footer?: ReactNode;
  className?: string;
};

/** Branded centered layout for login/register pages. */
export function AuthLayout({ children, title, subtitle, footer, className }: AuthLayoutProps) {
  return (
    <div className="flex min-h-screen flex-col bg-aivion-bg">
      <header className="flex items-center justify-center px-6 py-8">
        <Link href="/login" className="transition-opacity hover:opacity-80">
          <AivionLogo size="large" />
        </Link>
      </header>

      <main className={cn("mx-auto w-full max-w-md flex-1 px-6 pb-12", className)}>
        <div className="rounded-aivion-xl border border-aivion-border bg-aivion-surface p-8 shadow-aivion-md">
          <div className="mb-6">
            <h1 className="text-2xl font-semibold text-aivion-text">{title}</h1>
            {subtitle ? <p className="mt-1.5 text-sm text-aivion-muted">{subtitle}</p> : null}
          </div>
          {children}
        </div>
        {footer ? <div className="mt-6 text-center text-sm text-aivion-muted">{footer}</div> : null}
      </main>

      <footer className="px-6 py-6 text-center text-xs text-aivion-muted-soft">
        © {new Date().getFullYear()} Aivion
      </footer>
    </div>
  );
}
