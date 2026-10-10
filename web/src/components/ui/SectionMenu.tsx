import type { LucideIcon } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

export type SectionItem = { key: string; href: string; label: string; icon: LucideIcon; current: boolean };
export type SectionGroup = { key: string; title?: string; items: SectionItem[] };

type Props = { label: string; groups: SectionGroup[]; children: ReactNode };

export function SectionMenu({ label, groups, children }: Props) {
  return (
    <div className="grid gap-6 md:grid-cols-[14rem_minmax(0,1fr)]">
      <nav aria-label={label} className="md:sticky md:top-20 md:self-start">
        <div className="flex gap-4 overflow-x-auto md:grid md:gap-5 md:overflow-visible">
          {groups.map((g) => (
            <div key={g.key} className="shrink-0">
              {g.title && <p className="mb-1 px-3 text-xs font-medium tracking-wide text-muted uppercase">{g.title}</p>}
              <ul className="flex gap-1 md:grid">
                {g.items.map((item) => {
                  const Icon = item.icon;
                  return (
                    <li key={item.key} className="shrink-0">
                      <Link
                        href={item.href}
                        scroll={false}
                        aria-current={item.current ? "page" : undefined}
                        className={cn(
                          "motion-control flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium whitespace-nowrap",
                          item.current ? "bg-signal-soft text-signal" : "text-ink-2 hover:bg-surface-2 hover:text-ink",
                        )}
                      >
                        <Icon aria-hidden="true" className="size-4" />
                        {item.label}
                      </Link>
                    </li>
                  );
                })}
              </ul>
            </div>
          ))}
        </div>
      </nav>
      <div className="min-w-0">{children}</div>
    </div>
  );
}
