import type { CatalogNode } from "./api";

export type TreeIndex = { byId: Map<string, CatalogNode>; children: Map<string | null, CatalogNode[]> };

function byName(a: CatalogNode, b: CatalogNode): number {
  return a.name.localeCompare(b.name, undefined, { sensitivity: "base" }) || a.slug.localeCompare(b.slug);
}

export function index(nodes: CatalogNode[]): TreeIndex {
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const children = new Map<string | null, CatalogNode[]>();
  for (const n of nodes) {
    const parent = n.parent_id && byId.has(n.parent_id) ? n.parent_id : null;
    const list = children.get(parent);
    if (list) list.push(n);
    else children.set(parent, [n]);
  }
  for (const list of children.values()) list.sort(byName);
  return { byId, children };
}

export function path(ix: TreeIndex, id: string): CatalogNode[] {
  const out: CatalogNode[] = [];
  const seen = new Set<string>();
  let node = ix.byId.get(id);
  while (node && !seen.has(node.id)) {
    seen.add(node.id);
    out.unshift(node);
    node = node.parent_id ? ix.byId.get(node.parent_id) : undefined;
  }
  return out;
}

export function childrenOf(ix: TreeIndex, id: string | null): CatalogNode[] {
  return ix.children.get(id) ?? [];
}

export function siblings(ix: TreeIndex, id: string): CatalogNode[] {
  const node = ix.byId.get(id);
  if (!node) return [];
  return childrenOf(ix, node.parent_id && ix.byId.has(node.parent_id) ? node.parent_id : null);
}

export function filterNodes(nodes: CatalogNode[], query: string): CatalogNode[] {
  const q = query.trim().toLowerCase();
  if (!q) return nodes;
  return nodes.filter((n) => n.name.toLowerCase().includes(q) || n.slug.toLowerCase().includes(q));
}
