"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";
import { ShieldAlert } from "lucide-react";

import { adminHref, currentAdminPath, visibleAdminGroups } from "@/lib/admin";
import { sectionVisible } from "@/lib/rights";
import { cn } from "@/lib/cn";

import { useSession } from "../SessionProvider";
import { EmptyState } from "../ui/EmptyState";
import { SectionMenu } from "../ui/SectionMenu";

type Props = { locale: Locale; nav: Messages["header"]["nav"]; menu: Messages["admin"]["menu"]; forbidden: string; children: ReactNode };

export function AdminShell({ locale, nav, menu, forbidden, children }: Props) {
  const current = currentAdminPath(usePathname(), locale);
  const { session } = useSession();
  const groups = visibleAdminGroups((s) => sectionVisible(session.status === "signed-in" ? session.user : null, s));
  const group = groups.find((g) => g.sections.some((s) => s.path === current)) ?? groups[0];
  if (!group) return null;
  const allowed = current === null || group.sections.some((s) => s.path === current);
  const items = group.sections.map((s) => ({ key: s.key, href: adminHref(locale, s.path), label: s.label(nav, menu), icon: s.icon, current: s.path === current }));
  return (
    <>
      <nav
        aria-label={menu.label}
        className="mb-6 flex max-w-full gap-1 overflow-x-auto rounded-xl border border-line bg-surface-2/80 p-1 shadow-elev-1 backdrop-blur [scrollbar-width:none]"
      >
        {groups.map((g) => {
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
        {allowed ? (
          children
        ) : (
          <div role="alert">
            <EmptyState icon={ShieldAlert} title={forbidden} className="mx-auto mt-8 max-w-lg" />
          </div>
        )}
      </SectionMenu>
    </>
  );
}
