import type { ReactNode } from "react";

import { Card } from "../ui/Card";

type Props = { href: string; icon: ReactNode; name: string; kind: string; slug?: string; children?: ReactNode };

export function PlaceCard({ href, icon, name, kind, slug, children }: Props) {
  return (
    <Card href={href} className="h-full">
      <span className="flex items-start gap-3">
        {icon}
        <span className="min-w-0 flex-1">
          <span className="block truncate font-medium text-ink group-hover:text-signal">{name}</span>
          <span className="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs text-muted">
            <span>{kind}</span>
            {slug && <code className="truncate">{slug}</code>}
          </span>
        </span>
      </span>
      {children}
    </Card>
  );
}

export function PlaceCards({ children }: { children: ReactNode }) {
  return <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">{children}</ul>;
}
