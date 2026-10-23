import type { GlobalScope, PersonalToken, Scope, TokenNode } from "./api";

export const TOKEN_MAX_NODES = 20;

export const BASE_SCOPES: readonly Scope[] = ["read", "write", "mcp"];

export const GLOBAL_SCOPES: readonly GlobalScope[] = ["users", "groups", "clusters", "settings", "catalog.root"];

export const SCOPES: readonly Scope[] = [...BASE_SCOPES, ...GLOBAL_SCOPES];

export function isGlobalScope(scope: Scope): scope is GlobalScope {
  return (GLOBAL_SCOPES as readonly Scope[]).includes(scope);
}

export type PickedNode = { id: string; label: string };

export type TokenAccess = "all" | "nodes";

export function tokenNodesBody(access: TokenAccess, picked: PickedNode[]): string[] | null {
  return access === "nodes" ? picked.map((n) => n.id) : null;
}

export function scopesForAccess(access: TokenAccess, scopes: Scope[]): Scope[] {
  return access === "nodes" ? scopes.filter((s) => !isGlobalScope(s)) : scopes;
}

export function canSubmitNodes(access: TokenAccess, picked: PickedNode[], global: boolean): boolean {
  if (access === "all") return true;
  return !global && picked.length > 0 && picked.length <= TOKEN_MAX_NODES;
}

export function showsCatalogAdmin(token: Pick<PersonalToken, "catalog_admin" | "scopes">): boolean {
  return token.catalog_admin && !token.scopes.some(isGlobalScope);
}

export function tokenNodeLabels(token: Pick<PersonalToken, "nodes">, all: string, deleted: string): string[] {
  if (!token.nodes) return [all];
  return token.nodes.map((n: TokenNode) => n.path ?? deleted);
}
