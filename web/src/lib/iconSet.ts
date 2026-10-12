"use client";

import type { LucideIcon } from "lucide-react";

type Imports = Record<string, () => Promise<{ default: LucideIcon }>>;

let imports: Promise<Imports> | null = null;
const icons = new Map<string, Promise<LucideIcon | null>>();
const ready = new Map<string, LucideIcon | null>();

function iconImports(): Promise<Imports> {
  imports ??= import("lucide-react/dynamicIconImports").then((m) => m.default as unknown as Imports);
  return imports;
}

export async function iconNames(): Promise<string[]> {
  return Object.keys(await iconImports());
}

export function loadedIcon(name: string): LucideIcon | null | undefined {
  return ready.get(name);
}

export function loadIcon(name: string): Promise<LucideIcon | null> {
  let p = icons.get(name);
  if (!p) {
    p = iconImports()
      .then((all) => {
        const load = Object.hasOwn(all, name) ? all[name] : undefined;
        return load ? load().then((m) => m.default) : null;
      })
      .catch(() => null)
      .then((icon) => {
        ready.set(name, icon);
        return icon;
      });
    icons.set(name, p);
  }
  return p;
}
