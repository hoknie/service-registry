import type { CatalogTableRow, NodeKind } from "./api";

export const TREE_PAGE = 50;
export const MAX_OPEN = 50;

export type TreeActivity = "running" | "queued" | "failed";
export type TreeFilters = { q: string; kind: NodeKind | ""; label: string; activity: TreeActivity | "" };
export const NO_FILTERS: TreeFilters = { q: "", kind: "", label: "", activity: "" };

export type Branch = { rows: CatalogTableRow[]; total: number; loading: boolean; error: string | null };

export type FlatRow =
  | { type: "node"; row: CatalogTableRow; level: number; setsize: number; posinset: number; expandable: boolean; expanded: boolean }
  | { type: "more" | "loading" | "error"; parent: string; level: number; error?: string };

const KINDS = ["organization", "folder", "project"];
const ACTIVITIES = ["running", "queued", "failed"];

type Params = { get(name: string): string | null };

export function parseFilters(params: Params): TreeFilters {
  const kind = params.get("kind") ?? "";
  const activity = params.get("activity") ?? "";
  return {
    q: (params.get("q") ?? "").slice(0, 100),
    kind: KINDS.includes(kind) ? (kind as NodeKind) : "",
    label: params.get("label") ?? "",
    activity: ACTIVITIES.includes(activity) ? (activity as TreeActivity) : "",
  };
}

export function hasFilters(f: TreeFilters): boolean {
  return f.q.trim() !== "" || f.kind !== "" || f.label.trim() !== "" || f.activity !== "";
}

export function filtersQuery(f: TreeFilters): string {
  const out = new URLSearchParams();
  if (f.q.trim()) out.set("q", f.q.trim());
  if (f.kind) out.set("kind", f.kind);
  for (const l of f.label.split(",").map((x) => x.trim()).filter(Boolean).slice(0, 5)) out.append("label", l);
  if (f.activity) out.set("activity", f.activity);
  return out.toString();
}

export function parseOpen(value: string | null): string[] {
  const out: string[] = [];
  for (const id of (value ?? "").split(",")) {
    const v = id.trim();
    if (v && !out.includes(v)) out.push(v);
    if (out.length === MAX_OPEN) break;
  }
  return out;
}

export function openQuery(ids: string[]): string {
  return parseOpen(ids.join(",")).join(",");
}

export function toggleOpen(open: string[], id: string): string[] {
  return open.includes(id) ? open.filter((x) => x !== id) : [...open, id].slice(-MAX_OPEN);
}

export function groupByParent(items: CatalogTableRow[], root: string): Map<string, Branch> {
  const out = new Map<string, Branch>();
  for (const row of items) {
    const key = row.parent_id ?? root;
    const b = out.get(key) ?? { rows: [], total: 0, loading: false, error: null };
    b.rows.push(row);
    b.total = b.rows.length;
    out.set(key, b);
  }
  return out;
}

export function buildRows(byParent: Map<string, Branch>, open: Set<string>, root: string, lazy: boolean): FlatRow[] {
  const out: FlatRow[] = [];
  const walk = (parent: string, level: number) => {
    const b = byParent.get(parent);
    if (!b) return;
    b.rows.forEach((row, i) => {
      const loaded = byParent.get(row.id);
      const expandable = lazy ? row.children > 0 : !!loaded && loaded.rows.length > 0;
      const expanded = expandable && open.has(row.id);
      out.push({ type: "node", row, level, setsize: b.total, posinset: i + 1, expandable, expanded });
      if (!expanded) return;
      const child = byParent.get(row.id);
      if (!child || (child.loading && child.rows.length === 0)) out.push({ type: "loading", parent: row.id, level: level + 1 });
      else if (child.error && child.rows.length === 0) out.push({ type: "error", parent: row.id, level: level + 1, error: child.error });
      else walk(row.id, level + 1);
    });
    if (b.rows.length < b.total) out.push({ type: b.loading ? "loading" : b.error ? "error" : "more", parent, level, error: b.error ?? undefined });
  };
  walk(root, 1);
  return out;
}

export function highlight(text: string, q: string): { text: string; hit: boolean }[] {
  const needle = q.trim().toLowerCase();
  if (!needle) return [{ text, hit: false }];
  const out: { text: string; hit: boolean }[] = [];
  const lower = text.toLowerCase();
  let from = 0;
  for (let at = lower.indexOf(needle); at >= 0; at = lower.indexOf(needle, from)) {
    if (at > from) out.push({ text: text.slice(from, at), hit: false });
    out.push({ text: text.slice(at, at + needle.length), hit: true });
    from = at + needle.length;
  }
  if (from < text.length) out.push({ text: text.slice(from), hit: false });
  return out;
}
