"use client";

import { useEffect, useSyncExternalStore } from "react";

import { apiGet, type Tree } from "./api";
import { onCatalogChange } from "./catalogEvents";

type State = { tree: Tree | null; error: unknown; loading: boolean };

const initial: State = { tree: null, error: null, loading: false };
let state: State = initial;
let pending: Promise<void> | null = null;
const listeners = new Set<() => void>();

function set(next: State) {
  state = next;
  for (const l of listeners) l();
}

function load(): Promise<void> {
  if (pending) return pending;
  set({ ...state, loading: true });
  pending = apiGet<Tree>("/v1/catalog/tree?depth=10").then(
    (tree) => set({ tree, error: null, loading: false }),
    (error: unknown) => {
      pending = null;
      set({ tree: state.tree, error, loading: false });
    },
  );
  return pending;
}

export function reloadCatalogTree(): void {
  pending = null;
  if (listeners.size > 0 || state.tree) void load();
}

onCatalogChange(reloadCatalogTree);

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function useCatalogTree(enabled = true): State & { reload: () => void } {
  const current = useSyncExternalStore(
    subscribe,
    () => state,
    () => initial,
  );
  useEffect(() => {
    if (enabled) void load();
  }, [enabled]);
  return { ...current, reload: reloadCatalogTree };
}
