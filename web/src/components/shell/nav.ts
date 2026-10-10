import { FileSearch, FolderTree, LayoutDashboard, Settings, type LucideIcon } from "lucide-react";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";

export type NavItem = { key: string; href: string; label: string; icon: LucideIcon; admin?: boolean };

export function navItems(locale: Locale, nav: Messages["header"]["nav"], superadmin: boolean): NavItem[] {
  const items: NavItem[] = [
    { key: "home", href: `/${locale}`, label: nav.home, icon: LayoutDashboard },
    { key: "catalog", href: `/${locale}/catalog`, label: nav.catalog, icon: FolderTree },
    { key: "docsSearch", href: `/${locale}/search`, label: nav.docsSearch, icon: FileSearch },
  ];
  if (superadmin) items.push({ key: "admin", href: `/${locale}/admin`, label: nav.admin, icon: Settings, admin: true });
  return items;
}

export function isActive(pathname: string, href: string, locale: Locale): boolean {
  return href === `/${locale}` ? pathname === href : pathname === href || pathname.startsWith(`${href}/`);
}
