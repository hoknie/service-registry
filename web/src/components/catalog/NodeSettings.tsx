"use client";

import { Building2, GitBranch, KeyRound, Link2, LockKeyhole, Plug, Settings2, ShieldCheck, SlidersHorizontal } from "lucide-react";
import type { ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import type { CatalogNode } from "@/lib/api";
import type { Section } from "@/lib/nodeSettings";

import { SectionMenu } from "../ui/SectionMenu";
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
  secrets: LockKeyhole,
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
