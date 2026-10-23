import type { CatalogTable, CatalogTableRow } from "./api";

export const NODE_SEARCH_LIMIT = 50;

export type NodeHit = { node: CatalogTableRow; path: string };

export function nodeSearchQuery(query: string): string | null {
  const q = query.trim();
  if (!q || q.length > 100) return null;
  return `/v1/catalog/table?q=${encodeURIComponent(q)}`;
}

export function nodeHits(table: CatalogTable, limit = NODE_SEARCH_LIMIT): NodeHit[] {
  const byId = new Map(table.items.map((r) => [r.id, r]));
  const path = (r: CatalogTableRow) => {
    const names: string[] = [];
    let parent = r.parent_id ? byId.get(r.parent_id) : undefined;
    while (parent) {
      names.unshift(parent.name);
      parent = parent.parent_id ? byId.get(parent.parent_id) : undefined;
    }
    return names.join(" / ");
  };
  return table.items
    .filter((r) => r.match)
    .slice(0, limit)
    .map((node) => ({ node, path: path(node) }));
}
