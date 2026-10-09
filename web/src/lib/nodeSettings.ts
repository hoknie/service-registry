import type { NodeKind } from "./api";

export const SECTIONS = ["general", "branches", "links", "docs", "keys", "access", "connect", "forge"] as const;
export type Section = (typeof SECTIONS)[number];

export const PROJECT_TABS = ["overview", "about", "deployments", "events", "docs", "settings"] as const;
export const CONTAINER_TABS = ["overview", "settings"] as const;
export type NodeTab = (typeof PROJECT_TABS)[number];

export const LEGACY_TABS: Record<string, Section> = {
  branches: "branches",
  links: "links",
  "docs-settings": "docs",
  keys: "keys",
  access: "access",
  connect: "connect",
  forge: "forge",
};

type NodeLike = { kind: NodeKind; access: "read" | "navigate"; permissions?: string[] };

export function tabsOf(node: NodeLike | null): NodeTab[] {
  if (!node) return ["overview"];
  if (node.access !== "read") return ["overview"];
  return node.kind === "project" ? [...PROJECT_TABS] : [...CONTAINER_TABS];
}

export function sectionsOf(node: NodeLike | null): Section[] {
  if (!node || node.access !== "read") return [];
  const can = (p: string) => (node.permissions ?? []).includes(p);
  if (node.kind === "project") {
    return ["general", "branches", "links", "docs", ...(can("catalog.keys") ? (["keys"] as const) : []), ...(can("catalog.access") ? (["access"] as const) : []), "connect"];
  }
  return ["general", "forge", "links", "docs", ...(can("catalog.access") ? (["access"] as const) : [])];
}

export function resolveTab(
  node: NodeLike | null,
  tab: string | null,
  section: string | null,
): { tab: NodeTab; section: Section | null; redirect: { tab: NodeTab; section: Section } | null } {
  const tabs = tabsOf(node);
  const sections = sectionsOf(node);
  const legacy = tab ? LEGACY_TABS[tab] : undefined;
  if (legacy && tabs.includes("settings")) {
    const target = sections.includes(legacy) ? legacy : (sections[0] ?? "general");
    return { tab: "settings", section: target, redirect: { tab: "settings", section: target } };
  }
  const chosen = tabs.includes(tab as NodeTab) ? (tab as NodeTab) : "overview";
  if (chosen !== "settings") return { tab: chosen, section: null, redirect: null };
  const s = sections.includes(section as Section) ? (section as Section) : (sections[0] ?? "general");
  return { tab: "settings", section: s, redirect: null };
}
