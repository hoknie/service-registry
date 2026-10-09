"use client";

import { Building2, GitBranch, KeyRound, Link2, Plug, Settings2, ShieldCheck, SlidersHorizontal } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import type { CatalogNode } from "@/lib/api";
import { cn } from "@/lib/cn";
import type { Section } from "@/lib/nodeSettings";

import { catalogHref, type CatalogLabels } from "./shared";

const icons = {
  general: SlidersHorizontal,
  branches: GitBranch,
  links: Link2,
  docs: Settings2,
  keys: KeyRound,
  access: ShieldCheck,
  connect: Plug,
  forge: Building2,
} as const;

type Props = {
  node: CatalogNode;
  sections: Section[];
  section: Section;
  branch: string | null;
  locale: Locale;
  labels: CatalogLabels;
  children: ReactNode;
};

export function NodeSettings({ node, sections, section, branch, locale, labels, children }: Props) {
  const t = labels.settings.sections;
  return (
    <div className="grid gap-6 md:grid-cols-[14rem_minmax(0,1fr)]">
      <nav aria-label={labels.settings.menu} className="md:sticky md:top-20 md:self-start">
        <ul className="flex gap-1 overflow-x-auto md:grid md:overflow-visible">
          {sections.map((s) => {
            const Icon = icons[s];
            const current = s === section;
            return (
              <li key={s} className="shrink-0">
                <Link
                  href={catalogHref(locale, node.id, "settings", branch, null, s)}
                  scroll={false}
                  aria-current={current ? "page" : undefined}
                  className={cn(
                    "motion-control flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium whitespace-nowrap",
                    current ? "bg-signal-soft text-signal" : "text-ink-2 hover:bg-surface-2 hover:text-ink",
                  )}
                >
                  <Icon aria-hidden="true" className="size-4" />
                  {t[s]}
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>
      <div className="min-w-0">{children}</div>
    </div>
  );
}
