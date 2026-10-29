import { cn } from "@/components/ui/cn";

export type AivionLogoSize = "small" | "default" | "large";

const MARK_SIZES: Record<AivionLogoSize, number> = {
  small: 24,
  default: 32,
  large: 40,
};

const WORDMARK_CLASS: Record<AivionLogoSize, string> = {
  small: "text-[15px] tracking-[-0.01em]",
  default: "text-[19px] tracking-[-0.015em]",
  large: "text-[24px] tracking-[-0.02em]",
};

type AivionMarkProps = {
  size?: AivionLogoSize;
  className?: string;
};

/** Geometric A mark — three separated elements forming an abstract letter A. */
export function AivionMark({ size = "default", className }: AivionMarkProps) {
  const px = MARK_SIZES[size];

  return (
    <svg
      width={px}
      height={px}
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
      className={cn("shrink-0", className)}
    >
      {/* Left leg — navy */}
      <path
        d="M10.5 12.5C10.5 11.4 11.4 10.5 12.5 10.5H18.8C19.6 10.5 20.3 10.9 20.6 11.6L16.6 35.8C16.4 36.7 15.5 37.3 14.6 37.3H9.2C8.1 37.3 7.2 36.4 7.2 35.3L10.5 12.5Z"
        fill="#061B45"
      />
      {/* Bottom bar — blue */}
      <path
        d="M15.8 31.2C15.8 30.1 16.7 29.2 17.8 29.2H30.2C31.3 29.2 32.2 30.1 32.2 31.2V35.3C32.2 36.4 31.3 37.3 30.2 37.3H17.8C16.7 37.3 15.8 36.4 15.8 35.3V31.2Z"
        fill="#0873F6"
      />
      {/* Right leg — turquoise */}
      <path
        d="M27.4 11.6C27.7 10.9 28.4 10.5 29.2 10.5H35.5C36.6 10.5 37.5 11.4 37.5 12.5L40.8 35.3C40.8 36.4 39.9 37.3 38.8 37.3H33.4C32.5 37.3 31.6 36.7 31.4 35.8L27.4 11.6Z"
        fill="#00D1D8"
      />
    </svg>
  );
}

type AivionLogoProps = {
  size?: AivionLogoSize;
  /** Show only the mark without wordmark */
  markOnly?: boolean;
  className?: string;
};

/** Horizontal lockup: mark + wordmark "Aivion". */
export function AivionLogo({ size = "default", markOnly = false, className }: AivionLogoProps) {
  const gapClass = size === "small" ? "gap-2" : size === "large" ? "gap-3.5" : "gap-2.5";

  if (markOnly) {
    return <AivionMark size={size} className={className} />;
  }

  return (
    <span className={cn("inline-flex items-center", gapClass, className)}>
      <AivionMark size={size} />
      <span
        className={cn(
          "font-medium leading-none text-aivion-navy",
          WORDMARK_CLASS[size],
        )}
        style={{ fontFamily: "var(--font-manrope), sans-serif" }}
      >
        Aivion
      </span>
    </span>
  );
}
