import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";
import type { NodeKind } from "@/lib/api";
import { LEGACY_TABS } from "@/lib/nodeSettings";

export type CatalogLabels = Messages["catalog"];
export type ErrorLabels = Messages["errors"];
export type { Note } from "../ui/Message";

export function catalogHref(locale: Locale, id: string | null, tab?: string, branch?: string | null, doc?: string | null, section?: string | null): string {
  if (!id) return `/${locale}/catalog`;
  let href = `/${locale}/catalog?node=${id}`;
  const legacy = tab ? LEGACY_TABS[tab] : undefined;
  if (legacy) href += `&tab=settings&section=${legacy}`;
  else if (tab && tab !== "overview") href += `&tab=${tab}`;
  if (!legacy && section) href += `&section=${section}`;
  if (branch) href += `&branch=${encodeURIComponent(branch)}`;
  if (doc) href += `&doc=${encodeURIComponent(doc)}`;
  return href;
}

export const FORGE_NAMES = { github: "GitHub", gitlab: "GitLab", forgejo: "Forgejo", gitea: "Gitea" } as const;

export function childKinds(parent: NodeKind | null): NodeKind[] {
  if (parent === null) return ["organization"];
  if (parent === "organization") return ["organization", "folder", "project"];
  if (parent === "folder") return ["folder", "project"];
  return [];
}

export { ago, when } from "@/i18n/time";

export function parseLabels(text: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const line of text.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const at = trimmed.indexOf("=");
    if (at < 0) out[trimmed] = "";
    else out[trimmed.slice(0, at).trim()] = trimmed.slice(at + 1).trim();
  }
  return out;
}

export function labelsText(labels: Record<string, string> | undefined): string {
  return Object.entries(labels ?? {})
    .map(([k, v]) => `${k}=${v}`)
    .join("\n");
}
