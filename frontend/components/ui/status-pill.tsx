import type { ReactNode } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/components/ui/cn";

const statusPillVariants = cva(
  "inline-flex max-w-full items-center rounded-full font-medium ring-1 ring-inset",
  {
    variants: {
      variant: {
        success: "bg-[#ecfdf3] text-[#027a48] ring-[#abefc6]",
        info: "bg-[#eff8ff] text-[#175cd3] ring-[#b2ddff]",
        warning: "bg-[#fffaeb] text-[#b54708] ring-[#fedf89]",
        neutral: "bg-slate-50 text-slate-600 ring-slate-200",
        danger: "bg-[#fef3f2] text-[#b42318] ring-[#fecdca]",
      },
      size: {
        sm: "px-2.5 py-0.5 text-xs",
        xs: "px-2 py-0.5 text-[11px]",
      },
    },
    defaultVariants: {
      variant: "neutral",
      size: "sm",
    },
  },
);

export type StatusPillVariant = NonNullable<VariantProps<typeof statusPillVariants>["variant"]>;

export type StatusPillProps = {
  children: ReactNode;
  variant?: StatusPillVariant;
  size?: NonNullable<VariantProps<typeof statusPillVariants>["size"]>;
  title?: string;
  className?: string;
};

export function StatusPill({ children, variant = "neutral", size = "sm", title, className }: StatusPillProps) {
  return (
    <span className={cn(statusPillVariants({ variant, size }), className)} title={title}>
      {children}
    </span>
  );
}
