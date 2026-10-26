export type RightSection = "users" | "groups" | "tokens" | "clusters" | "environments" | "link_kinds" | "secrets" | "scans" | "catalog";
export type RightAction = "create" | "read" | "update" | "delete";
export type Rights = Partial<Record<RightSection, RightAction[]>>;

export const RIGHT_ACTIONS: RightAction[] = ["create", "read", "update", "delete"];

export const SECTION_ACTIONS: Record<RightSection, RightAction[]> = {
  users: ["create", "read", "update"],
  groups: ["create", "read", "update", "delete"],
  tokens: ["read", "delete"],
  clusters: ["create", "read", "update", "delete"],
  environments: ["create", "update", "delete"],
  link_kinds: ["create", "update", "delete"],
  secrets: ["create", "read", "update", "delete"],
  scans: ["read"],
  catalog: ["create", "read", "update", "delete"],
};

export const RIGHT_SECTIONS = Object.keys(SECTION_ACTIONS) as RightSection[];

type Holder = { is_superadmin: boolean; rights?: Rights; features?: { metrics?: boolean } } | null | undefined;

export const METRICS_RIGHTS: RightSection[] = ["secrets", "scans"];

export function metricsAllowed(user: Holder): boolean {
  return METRICS_RIGHTS.some((s) => can(user, s, "read"));
}

export function metricsVisible(user: Holder): boolean {
  return Boolean(user?.features?.metrics) && metricsAllowed(user);
}

export function can(user: Holder, section: RightSection, action: RightAction): boolean {
  if (!user) return false;
  return user.is_superadmin || (user.rights?.[section] ?? []).includes(action);
}

export function sectionVisible(user: Holder, section: RightSection): boolean {
  if (!user) return false;
  if (user.is_superadmin) return true;
  const held = user.rights?.[section] ?? [];
  return SECTION_ACTIONS[section].includes("read") ? held.includes("read") : held.length > 0;
}

const ADMIN_SECTIONS = RIGHT_SECTIONS.filter((s) => s !== "catalog");

export function adminVisible(user: Holder): boolean {
  return ADMIN_SECTIONS.some((s) => sectionVisible(user, s));
}

export function normalizeRights(rights: Rights): Rights {
  const out: Rights = {};
  for (const section of RIGHT_SECTIONS) {
    const held = SECTION_ACTIONS[section].filter((a) => (rights[section] ?? []).includes(a));
    if (held.length > 0) out[section] = held;
  }
  return out;
}

const AREA_SECTIONS: Record<string, RightSection[]> = {
  users: ["users"],
  groups: ["groups"],
  clusters: ["clusters"],
  settings: ["secrets", "scans", "link_kinds", "environments"],
};

export function scopeAllowed(user: Holder, scope: string): boolean {
  if (!user) return false;
  if (user.is_superadmin) return true;
  if (scope === "catalog.root") return (user.rights?.catalog ?? []).some((a) => a !== "read");
  const [area, half] = scope.split(":");
  const sections = area ? AREA_SECTIONS[area] : undefined;
  if (!sections || !half) return false;
  return sections.some((s) => (user.rights?.[s] ?? []).some((a) => (half === "read" ? a === "read" : a !== "read")));
}
