"use client";

import { Bell, Building2, Contact, GitBranch, Globe, KeyRound, Link2, LockKeyhole, Network, Plug, ScanSearch, Settings2, ShieldCheck, SlidersHorizontal, UsersRound } from "lucide-react";
import type { ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import type { CatalogNode } from "@/lib/api";
import type { Section } from "@/lib/nodeSettings";

import { SettingsSections } from "../places/SettingsSections";
import { catalogHref, type CatalogLabels } from "./shared";

const icons = {
  general: SlidersHorizontal,
  owners: UsersRound,
  groups: Contact,
  branches: GitBranch,
  links: Link2,
  dependencies: Network,
  docs: Settings2,
  keys: KeyRound,
  access: ShieldCheck,
  notifications: Bell,
  connect: Plug,
  forge: Building2,
  secrets: LockKeyhole,
  scans: ScanSearch,
  publication: Globe,
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
    <SettingsSections
      label={labels.settings.menu}
      sections={sections.map((s) => ({ key: s, label: t[s], icon: icons[s] }))}
      current={section}
      href={(s) => catalogHref(locale, node.id, "settings", branch, null, s)}
    >
      {children}
    </SettingsSections>
  );
}
