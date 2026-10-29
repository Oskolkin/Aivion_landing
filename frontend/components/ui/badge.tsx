import type { HTMLAttributes } from "react";
import { cn } from "@/components/ui/cn";

export type BadgeTone = "neutral" | "success" | "warning" | "danger" | "info" | "accent";

const toneClass: Record<BadgeTone, string> = {
  neutral: "bg-aivion-surface-soft text-aivion-text ring-aivion-border",
  success: "bg-[#ecfdf3] text-[#027a48] ring-[#abefc6]",
  warning: "bg-[#fffaeb] text-[#b54708] ring-[#fedf89]",
  danger: "bg-[#fef3f2] text-[#b42318] ring-[#fecdca]",
  info: "bg-[#eff8ff] text-[#175cd3] ring-[#b2ddff]",
  accent: "bg-[#ecfdfe] text-[#088a91] ring-[#99f0f4]",
};

export type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  tone?: BadgeTone;
};

/**
 * Generic chip/tag — NOT a status indicator.
 * Use for neutral labels, metric chips, tags, counts.
 * For statuses / severity / evidence / state use `StatusPill`
 * from "@/components/ui/status-pill" instead.
 */
export function Badge({ className, tone = "neutral", children, ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex max-w-full items-center rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset",
        toneClass[tone],
        className,
      )}
      {...props}
    >
      {children}
    </span>
  );
}
