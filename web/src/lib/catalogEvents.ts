import { useSyncExternalStore } from "react";

let version = 0;
const listeners = new Set<() => void>();

export function catalogChanged(): void {
  version += 1;
  for (const l of listeners) l();
}

export function catalogVersion(): number {
  return version;
}

export function onCatalogChange(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function useCatalogVersion(): number {
  return useSyncExternalStore(onCatalogChange, catalogVersion, () => 0);
}
