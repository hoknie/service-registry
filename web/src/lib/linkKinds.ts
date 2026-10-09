"use client";

import { useEffect, useSyncExternalStore } from "react";

import { apiGet, type LinkKind } from "./api";

let kinds: LinkKind[] | null = null;
let loading: Promise<void> | null = null;
const listeners = new Set<() => void>();

function load() {
  if (kinds || loading) return;
  loading = apiGet<{ items: LinkKind[] }>("/v1/link-kinds")
    .then((r) => {
      kinds = r.items;
    })
    .catch(() => {
      kinds = [];
    })
    .finally(() => {
      loading = null;
      listeners.forEach((l) => l());
    });
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useLinkKinds(): LinkKind[] {
  const value = useSyncExternalStore(
    subscribe,
    () => kinds,
    () => null,
  );
  useEffect(load, []);
  return value ?? [];
}
