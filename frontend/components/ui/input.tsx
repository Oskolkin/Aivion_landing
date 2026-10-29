import { forwardRef, type InputHTMLAttributes } from "react";
import { cn } from "@/components/ui/cn";

export type InputProps = InputHTMLAttributes<HTMLInputElement>;

const inputClass =
  "w-full rounded-aivion-md border border-aivion-border bg-aivion-surface px-3.5 py-2.5 text-sm text-aivion-text shadow-aivion-sm transition-colors placeholder:text-aivion-muted-soft hover:border-aivion-border-strong focus-visible:border-aivion-blue focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50";

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, ...props },
  ref,
) {
  return <input ref={ref} className={cn(inputClass, className)} {...props} />;
});

export type TextareaProps = React.TextareaHTMLAttributes<HTMLTextAreaElement>;

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { className, ...props },
  ref,
) {
  return (
    <textarea
      ref={ref}
      className={cn(inputClass, "min-h-[88px] resize-y", className)}
      {...props}
    />
  );
});

export type FormFieldProps = {
  label: string;
  hint?: string;
  error?: string;
  children: React.ReactNode;
  className?: string;
  htmlFor?: string;
};

export function FormField({ label, hint, error, children, className, htmlFor }: FormFieldProps) {
  return (
    <div className={cn("space-y-1.5", className)}>
      <label htmlFor={htmlFor} className="block text-sm font-medium text-aivion-text">
        {label}
      </label>
      {children}
      {hint && !error ? <p className="text-xs text-aivion-muted">{hint}</p> : null}
      {error ? <p className="text-xs text-aivion-danger">{error}</p> : null}
    </div>
  );
}
