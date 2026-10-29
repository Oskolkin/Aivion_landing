import type { ReactNode } from "react";
import { cn } from "@/components/ui/cn";

export type ErrorStateProps = {
  title: string;
  message: string;
  action?: ReactNode;
  className?: string;
};

export function ErrorState({ title, message, action, className }: ErrorStateProps) {
  return (
    <div
      role="alert"
      className={cn(
        "rounded-aivion-lg border border-[#fecdca] bg-[#fef3f2] px-5 py-4 text-[#7a271a]",
        className,
      )}
    >
      <p className="text-sm font-semibold">{title}</p>
      <p className="mt-1 text-sm opacity-90">{message}</p>
      {action ? <div className="mt-3">{action}</div> : null}
    </div>
  );
}

export type AlertBannerProps = {
  title?: string;
  message: string;
  tone?: "info" | "warning" | "success" | "danger";
  action?: ReactNode;
  className?: string;
  "data-testid"?: string;
};

const bannerTone: Record<NonNullable<AlertBannerProps["tone"]>, string> = {
  info: "border-[#b2ddff] bg-[#eff8ff] text-[#175cd3]",
  warning: "border-[#fedf89] bg-[#fffaeb] text-[#b54708]",
  success: "border-[#abefc6] bg-[#ecfdf3] text-[#027a48]",
  danger: "border-[#fecdca] bg-[#fef3f2] text-[#b42318]",
};

export function AlertBanner({
  title,
  message,
  tone = "info",
  action,
  className,
  "data-testid": dataTestId,
}: AlertBannerProps) {
  return (
    <div
      data-testid={dataTestId}
      className={cn(
        "rounded-aivion-lg border px-5 py-3.5 text-sm",
        bannerTone[tone],
        className,
      )}
    >
      {title ? <p className="font-semibold">{title}</p> : null}
      <p className={title ? "mt-0.5" : undefined}>{message}</p>
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  );
}
