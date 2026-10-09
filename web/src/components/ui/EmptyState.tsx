import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

export function EmptyState({
  icon: Icon,
  title,
  text,
  action,
  className,
}: {
  icon: LucideIcon;
  title: ReactNode;
  text?: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex animate-rise-in flex-col items-center rounded-card border border-dashed border-line-strong bg-surface/60 px-6 py-12 text-center",
        className,
      )}
    >
      <span
        aria-hidden="true"
        className="mb-4 grid size-14 place-items-center rounded-2xl bg-gradient-to-br from-signal-soft to-cobalt-soft text-signal ring-1 ring-line ring-inset"
      >
        <Icon className="size-6" />
      </span>
      <p className="font-medium text-ink">{title}</p>
      {text && <p className="mt-1 max-w-[52ch] text-sm text-muted">{text}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
