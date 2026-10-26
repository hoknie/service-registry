"use client";

import { Bell, Building2, Globe, Contact, GitBranch, KeyRound, Link2, LockKeyhole, Network, Plug, ScanSearch, Settings2, ShieldCheck, SlidersHorizontal, UsersRound } from "lucide-react";
import type { ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import type { CatalogNode } from "@/lib/api";
import type { Section } from "@/lib/nodeSettings";

import { SectionMenu } from "../ui/SectionMenu";
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
  const items = sections.map((s) => ({
    key: s,
    href: catalogHref(locale, node.id, "settings", branch, null, s),
    label: t[s],
    icon: icons[s],
    current: s === section,
  }));
  return (
    <SectionMenu label={labels.settings.menu} groups={[{ key: "sections", items }]}>
      {children}
    </SectionMenu>
  );
}
