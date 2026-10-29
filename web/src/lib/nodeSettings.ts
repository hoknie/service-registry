import type { NodeKind } from "./api";

export const SECTIONS = ["general", "owners", "groups", "branches", "links", "dependencies", "docs", "keys", "access", "notifications", "connect", "repo-sync", "secrets", "scans", "publication", "polling"] as const;
export type Section = (typeof SECTIONS)[number];

export const PROJECT_TABS = ["overview", "docs", "details", "releases", "dependencies", "settings"] as const;
export const CONTAINER_TABS = ["overview", "details", "dependencies", "settings"] as const;
export const ORGANIZATION_TABS = ["overview", "details", "dependencies", "members", "settings"] as const;
export type NodeTab = (typeof PROJECT_TABS)[number] | (typeof ORGANIZATION_TABS)[number];

export const LEGACY_TABS: Record<string, Section> = {
  branches: "branches",
  links: "links",
  "docs-settings": "docs",
  keys: "keys",
  access: "access",
  connect: "connect",
  forge: "repo-sync",
};

export const RENAMED_SECTIONS: Record<string, Section> = {
  forge: "repo-sync",
};

export const RENAMED_TABS: Record<string, NodeTab> = {
  deployments: "releases",
  about: "docs",
  events: "overview",
};

type NodeLike = { kind: NodeKind; parent_id?: string | null; access: "read" | "navigate"; permissions?: string[] };


export function tabsOf(node: NodeLike | null): NodeTab[] {
  if (!node) return ["overview"];
  if (node.access !== "read") return ["overview"];
  if (node.kind === "project") return [...PROJECT_TABS];
  return node.kind === "organization" && !node.parent_id ? [...ORGANIZATION_TABS] : [...CONTAINER_TABS];
}

export function sectionsOf(node: NodeLike | null): Section[] {
  if (!node || node.access !== "read") return [];
  const can = (p: string) => (node.permissions ?? []).includes(p);
  if (node.kind === "project") {
    return [
      "general",
      "owners",
      "branches",
      "links",
      ...(can("catalog.write") ? (["dependencies"] as const) : []),
      "docs",
      "publication",
      "polling",
      ...(can("catalog.write") ? (["scans"] as const) : []),
      ...(can("catalog.keys") ? (["keys"] as const) : []),
      ...(can("catalog.access") ? (["access", "notifications"] as const) : []),
      "connect",
    ];
  }
  return [
    "general",
    "owners",
    ...(node.parent_id ? [] : (["groups"] as const)),
    "repo-sync",
    "secrets",
    "links",
    "docs",
    "publication",
    "polling",
    ...(can("catalog.access") ? (["access", "notifications"] as const) : []),
  ];
}

export function resolveTab(
  node: NodeLike | null,
  tab: string | null,
  section: string | null,
): { tab: NodeTab; section: Section | null; redirect: { tab: NodeTab; section: Section | null } | null } {
  const tabs = tabsOf(node);
  const sections = sectionsOf(node);
  const legacy = tab ? LEGACY_TABS[tab] : undefined;
  if (legacy && tabs.includes("settings")) {
    const target = sections.includes(legacy) ? legacy : (sections[0] ?? "general");
    return { tab: "settings", section: target, redirect: { tab: "settings", section: target } };
  }
  const renamed = tab ? RENAMED_TABS[tab] : undefined;
  if (renamed && node?.kind === "project" && tabs.includes(renamed)) return { tab: renamed, section: null, redirect: { tab: renamed, section: null } };
  const chosen = tabs.includes(tab as NodeTab) ? (tab as NodeTab) : "overview";
  if (chosen !== "settings") return { tab: chosen, section: null, redirect: null };
  const moved = section ? RENAMED_SECTIONS[section] : undefined;
  if (moved && sections.includes(moved)) return { tab: "settings", section: moved, redirect: { tab: "settings", section: moved } };
  const s = sections.includes(section as Section) ? (section as Section) : (sections[0] ?? "general");
  return { tab: "settings", section: s, redirect: null };
}
