import type { ReactNode } from "react";
import { CircleHelp } from "lucide-react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/components/ui/cn";

const emptyNoteVariants = cva(
  "flex items-start gap-2 rounded-md px-3 py-2 text-xs leading-5",
  {
    variants: {
      variant: {
        neutral: "border border-slate-200 bg-slate-50 text-slate-500",
        dashed: "border border-dashed border-slate-200 bg-white/60 text-slate-600",
        soft: "bg-slate-50/60 text-slate-500",
      },
    },
    defaultVariants: {
      variant: "dashed",
    },
  },
);

export type EmptyNoteProps = {
  children: ReactNode;
  icon?: ReactNode;
  variant?: NonNullable<VariantProps<typeof emptyNoteVariants>["variant"]>;
  className?: string;
  "data-testid"?: string;
};

/** Compact empty/placeholder state for use inside dashboard cards and sections. */
export function EmptyNote({ children, icon, variant = "dashed", className, ...rest }: EmptyNoteProps) {
  return (
    <div className={cn(emptyNoteVariants({ variant }), className)} {...rest}>
      {icon ?? <CircleHelp className="mt-0.5 h-3.5 w-3.5 shrink-0 text-slate-400" aria-hidden="true" />}
      <div className="min-w-0">{children}</div>
    </div>
  );
}
