import type { NodeKind } from "./api";

export const RECENT_NODES_KEY = "svcr-recent-nodes";
export const RECENT_LIMIT = 10;

export type RecentNode = { id: string; name: string; kind: NodeKind; path: string };

type Store = Pick<Storage, "getItem" | "setItem">;

const KINDS: readonly string[] = ["organization", "folder", "project"];

function store(): Store | null {
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

function valid(v: unknown): v is RecentNode {
  if (!v || typeof v !== "object") return false;
  const r = v as Record<string, unknown>;
  return typeof r.id === "string" && typeof r.name === "string" && typeof r.path === "string" && KINDS.includes(r.kind as string);
}

export function recentNodes(s: Store | null = store()): RecentNode[] {
  try {
    const parsed: unknown = JSON.parse(s?.getItem(RECENT_NODES_KEY) ?? "[]");
    return Array.isArray(parsed) ? parsed.filter(valid).slice(0, RECENT_LIMIT) : [];
  } catch {
    return [];
  }
}

function save(list: RecentNode[], s: Store | null) {
  try {
    s?.setItem(RECENT_NODES_KEY, JSON.stringify(list));
  } catch {
  }
}

export function recordRecentNode(node: RecentNode, s: Store | null = store()) {
  const { id, name, kind, path } = node;
  save([{ id, name, kind, path }, ...recentNodes(s).filter((r) => r.id !== id)].slice(0, RECENT_LIMIT), s);
}

export function forgetRecentNode(id: string, s: Store | null = store()) {
  const list = recentNodes(s);
  if (list.some((r) => r.id === id)) save(list.filter((r) => r.id !== id), s);
}
