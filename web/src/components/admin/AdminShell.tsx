"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";
import { ADMIN_GROUPS, adminHref, currentAdminGroup, currentAdminPath } from "@/lib/admin";
import { cn } from "@/lib/cn";

import { SectionMenu } from "../ui/SectionMenu";

type Props = { locale: Locale; nav: Messages["header"]["nav"]; menu: Messages["admin"]["menu"]; children: ReactNode };

export function AdminShell({ locale, nav, menu, children }: Props) {
  const current = currentAdminPath(usePathname(), locale);
  const group = currentAdminGroup(current);
  const items = group.sections.map((s) => ({ key: s.key, href: adminHref(locale, s.path), label: s.label(nav, menu), icon: s.icon, current: s.path === current }));
  return (
    <>
      <nav
        aria-label={menu.label}
        className="mb-6 flex max-w-full gap-1 overflow-x-auto rounded-xl border border-line bg-surface-2/80 p-1 shadow-elev-1 backdrop-blur [scrollbar-width:none]"
      >
        {ADMIN_GROUPS.map((g) => {
          const active = g.key === group.key;
          return (
            <Link
              key={g.key}
              href={adminHref(locale, g.sections[0]!.path)}
              aria-current={active ? "page" : undefined}
              className={cn(
                "motion-control relative inline-flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium whitespace-nowrap",
                active ? "bg-surface text-ink shadow-elev-2" : "text-muted hover:bg-surface hover:text-ink",
              )}
            >
              <g.icon aria-hidden="true" className={cn("size-4", active && "text-signal")} />
              {menu[g.key]}
            </Link>
          );
        })}
      </nav>
      <SectionMenu label={menu[group.key]} groups={[{ key: group.key, items }]}>
        {children}
      </SectionMenu>
    </>
  );
}
