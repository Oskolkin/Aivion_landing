import { Loader2 } from "lucide-react";
import { cn } from "@/components/ui/cn";
import type { AttachmentVisualKind } from "@/lib/chat-display";

export function AttachmentTypeIcon({
  visual,
  size = "md",
  loading,
}: {
  visual: AttachmentVisualKind;
  size?: "sm" | "md";
  loading?: boolean;
}) {
  const dim = size === "sm" ? "h-9 w-9 text-[10px]" : "h-10 w-10 text-[11px]";

  return (
    <div className="relative shrink-0">
      <div
        className={cn(
          "flex items-center justify-center rounded-[10px] font-bold tracking-tight",
          dim,
          visual.containerClassName,
          visual.iconClassName,
        )}
        aria-hidden="true"
      >
        {visual.label}
      </div>
      {loading ? (
        <div className="absolute inset-0 flex items-center justify-center rounded-[10px] bg-white/55">
          <Loader2 className="h-4 w-4 animate-spin text-[#64748B]" aria-hidden="true" />
        </div>
      ) : null}
    </div>
  );
}
