import type { PersonalToken, Scope, TokenNode } from "./api";

export const TOKEN_MAX_NODES = 20;

export type PickedNode = { id: string; label: string };

export type TokenAccess = "all" | "nodes";

export function tokenNodesBody(access: TokenAccess, picked: PickedNode[]): string[] | null {
  return access === "nodes" ? picked.map((n) => n.id) : null;
}

export function scopesForAccess(access: TokenAccess, scopes: Scope[]): Scope[] {
  return access === "nodes" ? scopes.filter((s) => s !== "admin") : scopes;
}

export function canSubmitNodes(access: TokenAccess, picked: PickedNode[], admin: boolean): boolean {
  if (access === "all") return true;
  return !admin && picked.length > 0 && picked.length <= TOKEN_MAX_NODES;
}

export function tokenNodeLabels(token: Pick<PersonalToken, "nodes">, all: string, deleted: string): string[] {
  if (!token.nodes) return [all];
  return token.nodes.map((n: TokenNode) => n.path ?? deleted);
}
