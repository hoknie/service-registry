export type DependencySourceKind = "event" | "cluster" | "repository" | "manual";

export type DependencySource = { source: DependencySourceKind; origin: string; seen_at: string; id?: string; note?: string };

export type DependencyTarget =
  | { type: "project"; id: string; path: string; name: string }
  | { type: "hidden" }
  | { type: "external"; name: string }
  | { type: "unresolved"; ref: string };

export type DependencyItem = { target: DependencyTarget; sources: DependencySource[] };

export type Dependent = { project: { id: string; path: string; name: string }; sources: DependencySource[] };

export type Dependencies = { depends_on: DependencyItem[]; dependents: Dependent[]; hidden_dependents: number };

export type GraphNodeType = "project" | "external" | "unresolved";

export type DependencyGraphNode = { id: string; type: GraphNodeType; path: string; name: string; in_scope: boolean };

export type DependencyGraphEdge = { from: string; to: string; sources: DependencySourceKind[] };

export type DependencyGraphData = { nodes: DependencyGraphNode[]; edges: DependencyGraphEdge[]; hidden_edges: number; truncated: boolean };

export type DependencyView = "list" | "graph";

export const DEPTHS = [1, 2, 3] as const;

export function viewOf(param: string | null, container: boolean): DependencyView {
  if (container) return "graph";
  return param === "graph" ? "graph" : "list";
}

export function depthOf(param: string | null): number {
  const n = Number(param);
  return DEPTHS.includes(n as 1 | 2 | 3) ? n : 1;
}

export function withView(href: string, view: DependencyView, depth: number): string {
  if (view === "list") return href;
  return depth > 1 ? `${href}&view=graph&depth=${depth}` : `${href}&view=graph`;
}

export function targetKey(t: DependencyTarget): string {
  switch (t.type) {
    case "project":
      return t.id;
    case "external":
      return `external:${t.name}`;
    case "unresolved":
      return `ref:${t.ref}`;
  }
  return "hidden";
}

export function manualOf(items: DependencyItem[]): { item: DependencyItem; source: DependencySource }[] {
  const out: { item: DependencyItem; source: DependencySource }[] = [];
  for (const item of items) for (const source of item.sources) if (source.source === "manual" && source.id) out.push({ item, source });
  return out;
}

export function automaticOf(items: DependencyItem[]): { item: DependencyItem; source: DependencySource }[] {
  const out: { item: DependencyItem; source: DependencySource }[] = [];
  for (const item of items) for (const source of item.sources) if (source.source !== "manual") out.push({ item, source });
  return out;
}

export function externalTarget(name: string): string {
  return `external:${name.trim().toLowerCase()}`;
}
