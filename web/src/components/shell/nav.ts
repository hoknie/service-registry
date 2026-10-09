import { Boxes, FileSearch, FolderTree, KeyRound, Layers, LayoutDashboard, Link2, UserRound, UsersRound, type LucideIcon } from "lucide-react";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";

export type NavItem = { key: string; href: string; label: string; icon: LucideIcon; admin?: boolean };

export function navItems(locale: Locale, nav: Messages["header"]["nav"], superadmin: boolean): NavItem[] {
  const items: NavItem[] = [
    { key: "home", href: `/${locale}`, label: nav.home, icon: LayoutDashboard },
    { key: "catalog", href: `/${locale}/catalog`, label: nav.catalog, icon: FolderTree },
    { key: "docsSearch", href: `/${locale}/search`, label: nav.docsSearch, icon: FileSearch },
  ];
  if (superadmin) {
    items.push(
      { key: "users", href: `/${locale}/admin/users`, label: nav.users, icon: UserRound, admin: true },
      { key: "groups", href: `/${locale}/admin/groups`, label: nav.groups, icon: UsersRound, admin: true },
      { key: "tokens", href: `/${locale}/admin/tokens`, label: nav.tokens, icon: KeyRound, admin: true },
      { key: "linkKinds", href: `/${locale}/admin/link-kinds`, label: nav.linkKinds, icon: Link2, admin: true },
      { key: "clusters", href: `/${locale}/admin/clusters`, label: nav.clusters, icon: Boxes, admin: true },
      { key: "environments", href: `/${locale}/admin/environments`, label: nav.environments, icon: Layers, admin: true },
    );
  }
  return items;
}

export function isActive(pathname: string, href: string, locale: Locale): boolean {
  return href === `/${locale}` ? pathname === href : pathname === href || pathname.startsWith(`${href}/`);
}
