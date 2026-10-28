export type PlaceRow<F, I> = { id: string; parent_id: string | null; children: number; folder?: F; item?: I };

export type ItemPage<I> = { items: I[]; next_cursor: string | null };

export type LevelPage<R> = { items: R[]; next: string | null };

export async function loadLevel<R>(cursor: string | null, folders: () => Promise<R[]>, items: (cursor: string | null) => Promise<ItemPage<R>>): Promise<LevelPage<R>> {
  if (cursor !== null) {
    const page = await items(cursor);
    return { items: page.items, next: page.next_cursor };
  }
  const [first, page] = await Promise.all([folders(), items(null)]);
  return { items: [...first, ...page.items], next: page.next_cursor };
}

export function countText(count: number, more: boolean): string {
  return more ? `${count}+` : String(count);
}

export function parentPath(path: string): string {
  const at = path.lastIndexOf("/");
  return at < 0 ? "" : path.slice(0, at);
}
