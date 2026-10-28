export type PlaceSection = "skills" | "mcp";

export type PlaceCrumb = { section: PlaceSection; kind: "root" | "space" | "folder"; id: string; space: string; spaceName: string; parent: string | null; global: string };

export type PlaceEntryKind = "space" | "folder" | "item";

export type PlaceEntry = { key: string; id: string; name: string; kind: PlaceEntryKind; space: string; href: string; group: "up" | "near" | "inside" };

export type PlaceRef = { id: string; name: string; organization_id?: string | null };

export type PlaceLevel = { spaces: PlaceRef[]; near: PlaceRef[]; folders: PlaceRef[]; items: PlaceRef[]; up: { kind: "space" | "folder"; ref: PlaceRef } | null };

export const MENU_LIMIT = 200;

const bases = { skills: { path: "skills", item: "skills/skill" }, mcp: { path: "mcp", item: "mcp/server" } } as const;

export function placeLink(locale: string, section: PlaceSection, kind: PlaceEntryKind, id: string): string {
  const b = bases[section];
  if (kind === "item") return `/${locale}/${b.item}?id=${id}`;
  return `/${locale}/${b.path}?${kind === "space" ? "space" : "folder"}=${id}`;
}

export function placeEntries(c: PlaceCrumb, last: boolean, level: PlaceLevel, locale: string): PlaceEntry[] {
  const entry = (group: PlaceEntry["group"], kind: PlaceEntryKind, r: PlaceRef): PlaceEntry => ({
    key: `${group}-${kind}-${r.id}`,
    id: r.id,
    name: r.name,
    kind,
    space: kind === "space" ? r.id : c.space,
    href: placeLink(locale, c.section, kind, r.id),
    group,
  });
  const cap = (list: PlaceRef[]) => list.slice(0, MENU_LIMIT);
  if (c.kind === "root") return cap(level.spaces).map((r) => entry("inside", "space", r));
  const out: PlaceEntry[] = [];
  if (last && level.up) out.push(entry("up", level.up.kind, level.up.ref));
  if (c.kind === "space") out.push(...cap(level.spaces).map((r) => entry("near", "space", r)));
  else out.push(...cap(level.near).map((r) => entry("near", "folder", r)));
  if (last) {
    out.push(...cap(level.folders).map((r) => entry("inside", "folder", r)));
    out.push(...cap(level.items).map((r) => entry("inside", "item", r)));
  }
  return out;
}

export function inSpace(list: PlaceRef[], space: string): PlaceRef[] {
  return list.filter((r) => (r.organization_id ?? "global") === space);
}


export type TrailCrumb = { label: string; href?: string; place?: PlaceCrumb };

type TrailInput = {
  section: PlaceSection;
  locale: string;
  title: string;
  global: string;
  space: string | null;
  spaceName: string;
  folders: { id: string; name: string }[];
  current: "root" | "space" | "folder" | "item";
};

export function placeTrail({ section, locale, title, global, space, spaceName, folders, current }: TrailInput): TrailCrumb[] {
  const base = { section, global, space: space ?? "", spaceName };
  const out: TrailCrumb[] = [{ label: title, href: `/${locale}/${bases[section].path}`, place: { ...base, kind: "root", id: "root", parent: null } }];
  if (!space) return out;
  out.push({ label: spaceName, href: placeLink(locale, section, "space", space), place: { ...base, kind: "space", id: space, parent: null } });
  folders.forEach((f, i) => out.push({ label: f.name, href: placeLink(locale, section, "folder", f.id), place: { ...base, kind: "folder", id: f.id, parent: folders[i - 1]?.id ?? null } }));
  const last = out[out.length - 1];
  if (last && current !== "item") delete last.href;
  return out;
}
