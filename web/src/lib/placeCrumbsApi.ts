import { apiGet, type Tree } from "./api";
import { inSpace, type PlaceCrumb, type PlaceLevel, type PlaceRef, type PlaceSection } from "./placeCrumbs";

type Api = {
  folders: (parent: string | null) => `/${string}`;
  items: (folder: string | null, space: string) => `/${string}`;
  itemName: (raw: Record<string, unknown>) => string;
};

const apis: Record<PlaceSection, Api> = {
  skills: {
    folders: (parent) => `/v1/skills/folders${parent ? `?parent_id=${parent}` : ""}`,
    items: (folder, space) => `/v1/skills?${folder ? `folder_id=${folder}` : `scope=${space}&folder_id=root`}`,
    itemName: (raw) => String(raw.title || raw.name || ""),
  },
  mcp: {
    folders: (parent) => `/v1/mcp/folders${parent ? `?parent_id=${parent}` : ""}`,
    items: (folder, space) => `/v1/mcp/servers?${folder ? `folder_id=${folder}` : `${space === "global" ? "" : `organization_id=${space}&`}folder_id=root`}`,
    itemName: (raw) => String(raw.title || raw.slug || ""),
  },
};

type Folder = { id: string; name: string; organization_id: string | null; parent_id: string | null };

async function folders(api: Api, parent: string | null): Promise<Folder[]> {
  return (await apiGet<{ items: Folder[] }>(api.folders(parent))).items;
}

async function items(api: Api, folder: string | null, space: string): Promise<PlaceRef[]> {
  const page = await apiGet<{ items: Record<string, unknown>[] }>(api.items(folder, space));
  return page.items.map((raw) => ({ id: String(raw.id), name: api.itemName(raw) }));
}

export async function organizationSpaces(global: string): Promise<PlaceRef[]> {
  const tree = await apiGet<Tree>("/v1/catalog/tree?depth=1");
  return [{ id: "global", name: global }, ...tree.nodes.filter((n) => n.parent_id === null).map((n) => ({ id: n.id, name: n.name }))];
}

export async function loadPlaceLevel(c: PlaceCrumb, last: boolean): Promise<PlaceLevel> {
  const api = apis[c.section];
  const level: PlaceLevel = { spaces: [], near: [], folders: [], items: [], up: null };
  if (c.kind !== "folder") level.spaces = await organizationSpaces(c.global);
  if (c.kind === "root") return level;
  if (c.kind === "folder") {
    const siblings = await folders(api, c.parent);
    level.near = c.parent ? siblings : inSpace(siblings, c.space);
    if (last) {
      const up = c.parent ? await apiGet<Folder>(`/v1/${c.section}/folders/${c.parent}`) : null;
      level.up = up ? { kind: "folder", ref: up } : { kind: "space", ref: { id: c.space, name: c.spaceName } };
    }
  }
  if (last) {
    const folder = c.kind === "folder" ? c.id : null;
    const [sub, list] = await Promise.all([folders(api, folder), items(api, folder, c.space)]);
    level.folders = folder ? sub : inSpace(sub, c.space);
    level.items = list;
  }
  return level;
}
