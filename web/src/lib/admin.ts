import { Boxes, FileText, FolderTree, KeyRound, Layers, Link2, LockKeyhole, ScanSearch, Server, ShieldCheck, UserRound, UsersRound, type LucideIcon } from "lucide-react";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";

import type { RightSection } from "./rights";

export type AdminSection = { key: string; path: string; right: RightSection; icon: LucideIcon; label: (nav: Messages["header"]["nav"], menu: Messages["admin"]["menu"]) => string };
export type AdminGroup = { key: keyof Omit<Messages["admin"]["menu"], "label" | "globalSecrets">; icon: LucideIcon; sections: AdminSection[] };

export const ADMIN_GROUPS: AdminGroup[] = [
  {
    key: "access",
    icon: ShieldCheck,
    sections: [
      { key: "users", right: "users", path: "users", icon: UserRound, label: (n) => n.users },
      { key: "groups", right: "groups", path: "groups", icon: UsersRound, label: (n) => n.groups },
      { key: "tokens", right: "tokens", path: "tokens", icon: KeyRound, label: (n) => n.tokens },
    ],
  },
  { key: "secrets", icon: LockKeyhole, sections: [{ key: "secrets", right: "secrets", path: "secrets", icon: LockKeyhole, label: (_, m) => m.globalSecrets }] },
  {
    key: "catalog",
    icon: FolderTree,
    sections: [
      { key: "linkKinds", right: "link_kinds", path: "link-kinds", icon: Link2, label: (n) => n.linkKinds },
      { key: "environments", right: "environments", path: "environments", icon: Layers, label: (n) => n.environments },
    ],
  },
  { key: "infra", icon: Server, sections: [{ key: "clusters", right: "clusters", path: "clusters", icon: Boxes, label: (n) => n.clusters }] },
  { key: "docs", icon: FileText, sections: [{ key: "scans", right: "scans", path: "scans", icon: ScanSearch, label: (n) => n.scans }] },
];

export const ADMIN_PATHS = ADMIN_GROUPS.flatMap((g) => g.sections.map((s) => s.path));

export function adminHref(locale: Locale, path: string): string {
  return `/${locale}/admin/${path}`;
}

export function currentAdminPath(pathname: string, locale: Locale): string | null {
  const rest = pathname.startsWith(`/${locale}/admin/`) ? pathname.slice(`/${locale}/admin/`.length) : "";
  const first = rest.split("/")[0] ?? "";
  return ADMIN_PATHS.includes(first) ? first : null;
}

export function currentAdminGroup(path: string | null): AdminGroup {
  return ADMIN_GROUPS.find((g) => g.sections.some((s) => s.path === path)) ?? ADMIN_GROUPS[0]!;
}

export function visibleAdminGroups(visible: (section: RightSection) => boolean): AdminGroup[] {
  return ADMIN_GROUPS.map((g) => ({ ...g, sections: g.sections.filter((s) => visible(s.right)) })).filter((g) => g.sections.length > 0);
}
