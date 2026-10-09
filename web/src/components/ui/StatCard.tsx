import { ArrowUpRight, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

import { Card } from "./Card";
import { Skeleton } from "./Skeleton";

const tones = {
  org: "bg-kind-org-soft text-kind-org",
  folder: "bg-kind-folder-soft text-kind-folder",
  project: "bg-kind-project-soft text-kind-project",
  cobalt: "bg-cobalt-soft text-cobalt",
  amber: "bg-amber-soft text-amber",
  signal: "bg-signal-soft text-signal",
} as const;

export type StatTone = keyof typeof tones;

type Props = {
  icon: LucideIcon;
  title: ReactNode;
  tone?: StatTone;
  value?: ReactNode;
  caption?: ReactNode;
  href?: string;
  open?: string;
  loading?: boolean;
  error?: string | null;
  empty?: ReactNode;
  children?: ReactNode;
  className?: string;
};

export function StatCard({ icon: Icon, title, tone = "signal", value, caption, href, open, loading, error, empty, children, className }: Props) {
  return (
    <Card href={href} className={cn("flex min-h-36 flex-col gap-3 overflow-hidden", className)}>
      <span
        aria-hidden="true"
        className={cn("pointer-events-none absolute -top-10 -right-10 size-32 rounded-full opacity-60 blur-2xl", tones[tone])}
      />
      <span className="relative flex items-center gap-2.5">
        <span aria-hidden="true" className={cn("grid size-8 place-items-center rounded-lg", tones[tone])}>
          <Icon className="size-4" />
        </span>
        <span className="text-sm font-medium text-ink-2">{title}</span>
        {href && (
          <ArrowUpRight
            aria-hidden="true"
            className="motion-control ml-auto size-4 text-muted group-hover:translate-x-0.5 group-hover:-translate-y-0.5 group-hover:text-ink"
          />
        )}
        {href && open && <span className="sr-only">{open}</span>}
      </span>
      <div className="relative min-w-0 flex-1">
        {loading ? (
          <div className="grid gap-2">
            <Skeleton className="h-7 w-16" />
            <Skeleton className="h-4 w-28" />
          </div>
        ) : error ? (
          <p className="text-sm text-danger">{error}</p>
        ) : empty ? (
          <p className="text-sm text-muted">{empty}</p>
        ) : (
          <>
            {value !== undefined && <p className="font-mono text-3xl font-semibold tracking-tight text-ink tabular-nums">{value}</p>}
            {caption && <p className="mt-0.5 text-sm text-muted">{caption}</p>}
            {children}
          </>
        )}
      </div>
    </Card>
  );
}
