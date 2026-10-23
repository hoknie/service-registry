import type { GlobalArea, GlobalScope, PersonalToken, Scope, TokenNode } from "./api";

export const TOKEN_MAX_NODES = 20;

export const BASE_SCOPES: readonly Scope[] = ["read", "write", "mcp"];

export const GLOBAL_AREAS: readonly GlobalArea[] = ["users", "groups", "clusters", "settings"];

export const CATALOG_ROOT: GlobalScope = "catalog.root";

export const GLOBAL_SCOPES: readonly GlobalScope[] = [
  ...GLOBAL_AREAS.flatMap((a): GlobalScope[] => [`${a}:read`, `${a}:write`]),
  CATALOG_ROOT,
];

export const SCOPES: readonly Scope[] = [...BASE_SCOPES, ...GLOBAL_SCOPES];

export function isGlobalScope(scope: Scope): scope is GlobalScope {
  return (GLOBAL_SCOPES as readonly Scope[]).includes(scope);
}

export function isWholeScope(scope: Scope): boolean {
  return isGlobalScope(scope) && scope !== CATALOG_ROOT;
}

export function toggleScope(current: Scope[], scope: Scope, on: boolean): Scope[] {
  const read = scope.endsWith(":write") ? (scope.replace(":write", ":read") as Scope) : null;
  return SCOPES.filter((s) => (s === scope ? on : (on && s === read) || current.includes(s)));
}

export type PickedNode = { id: string; label: string };

export type TokenAccess = "all" | "nodes";

export function tokenNodesBody(access: TokenAccess, picked: PickedNode[]): string[] | null {
  return access === "nodes" ? picked.map((n) => n.id) : null;
}

export function scopesForAccess(access: TokenAccess, scopes: Scope[]): Scope[] {
  return access === "nodes" ? scopes.filter((s) => !isWholeScope(s)) : scopes;
}

export function canSubmitNodes(access: TokenAccess, picked: PickedNode[], scopes: Scope[]): boolean {
  if (access === "all") return true;
  return !scopes.some(isWholeScope) && picked.length > 0 && picked.length <= TOKEN_MAX_NODES;
}

export function showsCatalogAdmin(token: Pick<PersonalToken, "catalog_admin" | "scopes" | "nodes">): boolean {
  return token.catalog_admin && (token.nodes !== null || !token.scopes.some(isGlobalScope));
}

export function tokenNodeLabels(token: Pick<PersonalToken, "nodes">, all: string, deleted: string): string[] {
  if (!token.nodes) return [all];
  return token.nodes.map((n: TokenNode) => n.path ?? deleted);
}
