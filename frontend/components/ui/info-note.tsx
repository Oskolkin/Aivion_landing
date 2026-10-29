import type { ReactNode } from "react";
import { Info, AlertTriangle } from "lucide-react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/components/ui/cn";

const infoNoteVariants = cva(
  "flex items-start gap-1.5 rounded-md border px-2.5 py-1.5 text-[11px] leading-relaxed",
  {
    variants: {
      variant: {
        neutral: "border-slate-200 bg-slate-50 text-slate-500",
        info: "border-sky-200 bg-sky-50/80 text-sky-900",
        warning: "border-amber-200 bg-amber-50/80 text-amber-900",
      },
    },
    defaultVariants: {
      variant: "neutral",
    },
  },
);

const ICON_BY_VARIANT = {
  neutral: Info,
  info: Info,
  warning: AlertTriangle,
} as const;

export type InfoNoteProps = {
  children: ReactNode;
  variant?: NonNullable<VariantProps<typeof infoNoteVariants>["variant"]>;
  icon?: ReactNode;
  className?: string;
  "data-testid"?: string;
};

export function InfoNote({ children, variant = "neutral", icon, className, ...rest }: InfoNoteProps) {
  const DefaultIcon = ICON_BY_VARIANT[variant];
  return (
    <div className={cn(infoNoteVariants({ variant }), className)} {...rest}>
      {icon ?? <DefaultIcon className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />}
      <div className="min-w-0">{children}</div>
    </div>
  );
}
