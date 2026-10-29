import { forwardRef, type ButtonHTMLAttributes } from "react";
import { cn } from "@/components/ui/cn";

export type ButtonVariant = "primary" | "secondary" | "danger" | "ghost" | "accent";

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
};

const variantClass: Record<ButtonVariant, string> = {
  primary:
    "bg-aivion-blue text-white shadow-aivion-sm hover:bg-[#0668e0] disabled:opacity-50 disabled:hover:bg-aivion-blue focus-visible:outline-aivion-blue",
  secondary:
    "border border-aivion-border bg-aivion-surface text-aivion-text shadow-aivion-sm hover:bg-aivion-surface-soft disabled:opacity-50 focus-visible:outline-aivion-blue",
  danger:
    "bg-aivion-danger text-white shadow-aivion-sm hover:bg-[#d93a2f] disabled:opacity-50 disabled:hover:bg-aivion-danger focus-visible:outline-aivion-danger",
  ghost:
    "text-aivion-text hover:bg-aivion-surface-soft disabled:opacity-50 focus-visible:outline-aivion-blue",
  accent:
    "bg-aivion-turquoise text-aivion-navy shadow-aivion-sm hover:bg-[#00bcc3] disabled:opacity-50 focus-visible:outline-aivion-turquoise",
};

const buttonBaseClass =
  "inline-flex items-center justify-center gap-2 rounded-aivion-md px-4 py-2 text-sm font-medium transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 disabled:cursor-not-allowed";

export function buttonClassNames(variant: ButtonVariant = "primary", className?: string): string {
  return cn(buttonBaseClass, variantClass[variant], className);
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { className, variant = "primary", type = "button", ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      className={buttonClassNames(variant, className)}
      {...props}
    />
  );
});
