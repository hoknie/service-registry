"use client";

import { useSyncExternalStore } from "react";

export const CHILDREN_VIEW_KEY = "svcr-children-view";
export type ChildrenView = "cards" | "table";

function read(): ChildrenView {
  try {
    return window.localStorage.getItem(CHILDREN_VIEW_KEY) === "table" ? "table" : "cards";
  } catch {
    return "cards";
  }
}

function subscribe(onChange: () => void) {
  window.addEventListener(CHILDREN_VIEW_KEY, onChange);
  window.addEventListener("storage", onChange);
  return () => {
    window.removeEventListener(CHILDREN_VIEW_KEY, onChange);
    window.removeEventListener("storage", onChange);
  };
}

export function useChildrenView(): [ChildrenView, (view: ChildrenView) => void] {
  const view = useSyncExternalStore(subscribe, read, () => "cards" as const);
  const set = (next: ChildrenView) => {
    try {
      window.localStorage.setItem(CHILDREN_VIEW_KEY, next);
    } catch {
    }
    window.dispatchEvent(new Event(CHILDREN_VIEW_KEY));
  };
  return [view, set];
}
