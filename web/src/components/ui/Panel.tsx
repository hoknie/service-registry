import type { LucideIcon } from "lucide-react";
import type { HTMLAttributes, ReactNode } from "react";

import { cn } from "@/lib/cn";

export function Panel({
  title,
  description,
  actions,
  className,
  bodyClassName,
  icon: Icon,
  children,
  ...rest
}: Omit<HTMLAttributes<HTMLElement>, "title"> & {
  title?: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  bodyClassName?: string;
  icon?: LucideIcon;
}) {
  return (
    <section className={cn("animate-rise-in rounded-card border border-line bg-surface shadow-elev-1", className)} {...rest}>
      {(title || actions) && (
        <header className="flex flex-wrap items-start justify-between gap-3 border-b border-line px-4 py-3 sm:px-5">
          <div className="flex min-w-0 items-start gap-2.5">
            {Icon && (
              <span aria-hidden="true" className="mt-0.5 grid size-7 shrink-0 place-items-center rounded-lg bg-surface-3 text-ink-2">
                <Icon className="size-4" />
              </span>
            )}
            <div className="min-w-0">
            {title && <h2 className="text-lg">{title}</h2>}
            {description && <p className="mt-0.5 text-sm text-muted">{description}</p>}
            </div>
          </div>
          {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
        </header>
      )}
      <div className={cn("px-4 py-4 sm:px-5", bodyClassName)}>{children}</div>
    </section>
  );
}

export function PageHeader({
  title,
  lead,
  actions,
  icon,
  glyph,
  children,
}: {
  title: ReactNode;
  lead?: ReactNode;
  actions?: ReactNode;
  icon?: ReactNode;
  glyph?: LucideIcon;
  children?: ReactNode;
}) {
  const Icon = glyph;
  return (
    <header className="mb-6 flex animate-rise-in flex-wrap items-start justify-between gap-4">
      <div className="flex min-w-0 items-start gap-3.5">
        {Icon ? (
          <span
            aria-hidden="true"
            className="grid size-11 shrink-0 place-items-center rounded-xl bg-gradient-to-br from-signal-soft to-cobalt-soft text-signal ring-1 ring-line ring-inset"
          >
            <Icon className="size-5" />
          </span>
        ) : (
          icon
        )}
        <div className="min-w-0">
          <h1 className="break-words">{title}</h1>
          {lead && <p className="mt-1 max-w-[68ch] text-muted">{lead}</p>}
          {children}
        </div>
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </header>
  );
}
