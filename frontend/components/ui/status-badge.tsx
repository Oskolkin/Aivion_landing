import { cn } from "@/components/ui/cn";
import { statusLabelRu } from "@/lib/status-labels";

/** Normalized keys after lowercasing and replacing spaces with underscores */
export type StatusBadgeValue =
  | "completed"
  | "running"
  | "failed"
  | "pending"
  | "valid"
  | "invalid"
  | "missing"
  | "error"
  | "unknown"
  | "open"
  | "resolved"
  | "dismissed"
  | "critical"
  | "high"
  | "medium"
  | "low"
  | "active"
  | "trial"
  | "past_due"
  | "paused"
  | "cancelled"
  | "internal"
  | "success"
  | "succeeded"
  | "checking"
  | "draft"
  | "not_configured"
  | "not_connected";

const STYLE: Record<string, string> = {
  completed: "bg-[#ecfdf3] text-[#027a48] ring-[#abefc6]",
  success: "bg-[#ecfdf3] text-[#027a48] ring-[#abefc6]",
  succeeded: "bg-[#ecfdf3] text-[#027a48] ring-[#abefc6]",
  valid: "bg-[#ecfdf3] text-[#027a48] ring-[#abefc6]",
  resolved: "bg-[#ecfdf3] text-[#027a48] ring-[#abefc6]",
  active: "bg-[#ecfdf3] text-[#027a48] ring-[#abefc6]",

  running: "bg-[#eff8ff] text-[#175cd3] ring-[#b2ddff]",
  sync_in_progress: "bg-[#eff8ff] text-[#175cd3] ring-[#b2ddff]",
  pending: "bg-[#fffaeb] text-[#b54708] ring-[#fedf89]",
  sync_pending: "bg-[#fffaeb] text-[#b54708] ring-[#fedf89]",
  checking: "bg-[#eff8ff] text-[#175cd3] ring-[#b2ddff]",
  trial: "bg-[#eff8ff] text-[#175cd3] ring-[#b2ddff]",
  open: "bg-[#fffaeb] text-[#b54708] ring-[#fedf89]",
  medium: "bg-[#fffaeb] text-[#b54708] ring-[#fedf89]",

  failed: "bg-[#fef3f2] text-[#b42318] ring-[#fecdca]",
  sync_failed: "bg-[#fef3f2] text-[#b42318] ring-[#fecdca]",
  invalid: "bg-[#fef3f2] text-[#b42318] ring-[#fecdca]",
  error: "bg-[#fef3f2] text-[#b42318] ring-[#fecdca]",
  critical: "bg-[#fef3f2] text-[#b42318] ring-[#fecdca]",
  high: "bg-[#fff4ed] text-[#c4320a] ring-[#fddcab]",
  past_due: "bg-[#fef3f2] text-[#b42318] ring-[#fecdca]",

  missing: "bg-aivion-surface-soft text-aivion-muted ring-aivion-border",
  unknown: "bg-aivion-surface-soft text-aivion-muted ring-aivion-border",
  dismissed: "bg-aivion-surface-soft text-aivion-muted-soft ring-aivion-border",
  paused: "bg-aivion-surface-soft text-aivion-muted ring-aivion-border",
  cancelled: "bg-aivion-surface-soft text-aivion-muted-soft ring-aivion-border",
  internal: "bg-[#f4f3ff] text-[#5925dc] ring-[#d9d6fe]",
  draft: "bg-aivion-surface-soft text-aivion-muted ring-aivion-border",
  not_configured: "bg-aivion-surface-soft text-aivion-muted ring-aivion-border",
  not_connected: "bg-aivion-surface-soft text-aivion-muted ring-aivion-border",

  low: "bg-aivion-surface-soft text-aivion-text ring-aivion-border",
};

const DEFAULT_STYLE = "bg-aivion-surface-soft text-aivion-text ring-aivion-border";

function normalizeStatus(raw: string): string {
  return raw.trim().toLowerCase().replace(/\s+/g, "_").replace(/-/g, "_");
}

export type StatusBadgeProps = {
  status: string;
  label?: string;
  className?: string;
};

export function StatusBadge({ status, label, className }: StatusBadgeProps) {
  const key = normalizeStatus(status);
  const style = STYLE[key] ?? DEFAULT_STYLE;
  const text = label ?? statusLabelRu(status);

  return (
    <span
      className={cn(
        "inline-flex max-w-full items-center rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset",
        style,
        className,
      )}
    >
      {text}
    </span>
  );
}
