"use client";

import { useSyncExternalStore } from "react";

const noop = () => () => {};

export function useShortcutLabel(): string {
  return useSyncExternalStore(
    noop,
    () => (/Mac|iPhone|iPad/.test(navigator.platform) ? "⌘K" : "Ctrl K"),
    () => "Ctrl K",
  );
}
