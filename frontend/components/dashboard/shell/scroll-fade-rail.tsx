"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { cn } from "@/components/ui/cn";

type Props = {
  children: ReactNode;
  className?: string;
  /** Scroll the element matching this selector into view when it changes. */
  activeSelector?: string;
  "aria-label"?: string;
  role?: string;
  "data-testid"?: string;
};

/**
 * Horizontal scroll container with edge fade affordance and auto-scroll to active item.
 */
export function ScrollFadeRail({
  children,
  className,
  activeSelector,
  "aria-label": ariaLabel,
  role,
  "data-testid": testId,
}: Props) {
  const scrollerRef = useRef<HTMLDivElement>(null);
  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(false);

  useEffect(() => {
    const el = scrollerRef.current;
    if (!el) return;

    const update = () => {
      const max = el.scrollWidth - el.clientWidth;
      setCanScrollLeft(el.scrollLeft > 2);
      setCanScrollRight(max - el.scrollLeft > 2);
    };

    update();
    el.addEventListener("scroll", update, { passive: true });
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => {
      el.removeEventListener("scroll", update);
      ro.disconnect();
    };
  }, [children]);

  useEffect(() => {
    if (!activeSelector || !scrollerRef.current) return;
    const active = scrollerRef.current.querySelector(activeSelector);
    if (active instanceof HTMLElement) {
      active.scrollIntoView({ inline: "nearest", block: "nearest", behavior: "smooth" });
    }
  }, [activeSelector, children]);

  return (
    <div className={cn("relative", className)} data-testid={testId}>
      <div
        ref={scrollerRef}
        role={role}
        aria-label={ariaLabel}
        className={cn(
          "overflow-x-auto overscroll-x-contain [scrollbar-width:none] [&::-webkit-scrollbar]:hidden",
          canScrollLeft && "mask-fade-left",
          canScrollRight && "mask-fade-right",
        )}
        style={{
          maskImage:
            canScrollLeft || canScrollRight
              ? `linear-gradient(to right, ${canScrollLeft ? "transparent" : "black"} 0, black ${canScrollLeft ? "28px" : "0"}, black calc(100% - ${canScrollRight ? "28px" : "0px"}), ${canScrollRight ? "transparent" : "black"} 100%)`
              : undefined,
          WebkitMaskImage:
            canScrollLeft || canScrollRight
              ? `linear-gradient(to right, ${canScrollLeft ? "transparent" : "black"} 0, black ${canScrollLeft ? "28px" : "0"}, black calc(100% - ${canScrollRight ? "28px" : "0px"}), ${canScrollRight ? "transparent" : "black"} 100%)`
              : undefined,
        }}
      >
        {children}
      </div>
      {canScrollLeft ? (
        <span
          aria-hidden
          className="pointer-events-none absolute inset-y-0 left-0 w-8 bg-gradient-to-r from-aivion-bg via-aivion-bg/80 to-transparent"
        />
      ) : null}
      {canScrollRight ? (
        <span
          aria-hidden
          className="pointer-events-none absolute inset-y-0 right-0 w-8 bg-gradient-to-l from-aivion-bg via-aivion-bg/80 to-transparent"
        />
      ) : null}
    </div>
  );
}
