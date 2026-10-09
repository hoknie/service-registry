import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

export function Skeleton({ className }: { className?: string }) {
  return <span aria-hidden="true" className={cn("block animate-shimmer rounded-md bg-surface-3", className)} />;
}

export function SkeletonRows({ rows = 4, label, className }: { rows?: number; label: string; className?: string }) {
  return (
    <div role="status" aria-live="polite" className={cn("grid gap-3", className)}>
      <span className="sr-only">{label}</span>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex items-center gap-3">
          <Skeleton className="size-5 shrink-0" />
          <Skeleton className={cn("h-4", i % 3 === 0 ? "w-2/3" : i % 3 === 1 ? "w-1/2" : "w-3/5")} />
        </div>
      ))}
    </div>
  );
}

function Status({ label, className, children }: { label: string; className?: string; children: ReactNode }) {
  return (
    <div role="status" aria-live="polite" className={className}>
      <span className="sr-only">{label}</span>
      {children}
    </div>
  );
}

const widths = ["w-2/3", "w-1/2", "w-3/5", "w-4/5", "w-2/5"];

export function SkeletonTable({ rows = 5, cols = 4, label, className }: { rows?: number; cols?: number; label: string; className?: string }) {
  return (
    <Status label={label} className={cn("overflow-hidden rounded-lg border border-line bg-surface", className)}>
      <div className="flex gap-4 border-b border-line bg-surface-2 px-4 py-2.5">
        {Array.from({ length: cols }, (_, c) => (
          <Skeleton key={c} className="h-3 flex-1" />
        ))}
      </div>
      {Array.from({ length: rows }, (_, r) => (
        <div key={r} className="flex items-center gap-4 border-b border-line px-4 py-3 last:border-b-0">
          {Array.from({ length: cols }, (_, c) => (
            <div key={c} className="flex-1">
              <Skeleton className={cn("h-4", widths[(r + c) % widths.length])} />
            </div>
          ))}
        </div>
      ))}
    </Status>
  );
}

export function SkeletonCards({ count = 4, label, className }: { count?: number; label: string; className?: string }) {
  return (
    <Status label={label} className={cn("grid gap-4 sm:grid-cols-2 xl:grid-cols-3", className)}>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="grid gap-3 rounded-lg border border-line bg-surface p-4">
          <Skeleton className="h-4 w-1/2" />
          <Skeleton className="h-6 w-1/3" />
          <Skeleton className="h-3 w-2/3" />
        </div>
      ))}
    </Status>
  );
}

export function SkeletonPanel({ lines = 5, label, className }: { lines?: number; label: string; className?: string }) {
  return (
    <Status label={label} className={cn("grid gap-3 rounded-lg border border-line bg-surface p-5", className)}>
      <Skeleton className="h-5 w-1/3" />
      {Array.from({ length: lines }, (_, i) => (
        <Skeleton key={i} className={cn("h-3.5", widths[i % widths.length])} />
      ))}
    </Status>
  );
}

export function SkeletonTree({ rows = 8, label, className }: { rows?: number; label: string; className?: string }) {
  const depth = [0, 1, 1, 2, 2, 1, 0, 1];
  return (
    <Status label={label} className={cn("grid gap-2", className)}>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex items-center gap-2" style={{ paddingLeft: `${(depth[i % depth.length] ?? 0) * 0.9 + 0.5}rem` }}>
          <Skeleton className="size-4 shrink-0" />
          <Skeleton className={cn("h-3.5", widths[i % widths.length])} />
        </div>
      ))}
    </Status>
  );
}
