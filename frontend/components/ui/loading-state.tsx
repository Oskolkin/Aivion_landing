import { cn } from "@/components/ui/cn";

export type LoadingStateProps = {
  message?: string;
  className?: string;
  /** Show skeleton cards instead of simple pulse */
  variant?: "simple" | "skeleton";
};

function SkeletonBar({ className }: { className?: string }) {
  return <div className={cn("animate-pulse rounded-aivion-sm bg-aivion-border", className)} aria-hidden />;
}

export function Skeleton({ className }: { className?: string }) {
  return <SkeletonBar className={cn("h-4 w-full", className)} />;
}

export function LoadingState({
  message = "Загрузка…",
  className,
  variant = "skeleton",
}: LoadingStateProps) {
  if (variant === "simple") {
    return (
      <div className={cn("space-y-3 p-6", className)} aria-busy="true" aria-live="polite">
        <div className="flex items-center gap-3">
          <span
            className="inline-block size-4 animate-spin rounded-full border-2 border-aivion-border border-t-aivion-blue"
            aria-hidden
          />
          <p className="text-sm text-aivion-muted">{message}</p>
        </div>
      </div>
    );
  }

  return (
    <div className={cn("space-y-4 p-6", className)} aria-busy="true" aria-live="polite">
      <p className="sr-only">{message}</p>
      <SkeletonBar className="h-6 w-48" />
      <SkeletonBar className="h-4 w-72" />
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <SkeletonBar className="h-28 rounded-aivion-lg" />
        <SkeletonBar className="h-28 rounded-aivion-lg" />
        <SkeletonBar className="h-28 rounded-aivion-lg" />
      </div>
    </div>
  );
}
