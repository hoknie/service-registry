"use client";

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

export type Crumb = { label: string; href?: string; node?: string };

type Api = { crumbs: Crumb[]; setCrumbs: (crumbs: Crumb[]) => void };

const Context = createContext<Api | null>(null);

export function CrumbsProvider({ children }: { children: ReactNode }) {
  const [crumbs, setCrumbs] = useState<Crumb[]>([]);
  return <Context.Provider value={{ crumbs, setCrumbs }}>{children}</Context.Provider>;
}

export function useCrumbTrail(): Crumb[] {
  return useContext(Context)?.crumbs ?? [];
}

export function Crumbs({ items }: { items: Crumb[] }) {
  const set = useContext(Context)?.setCrumbs;
  const key = JSON.stringify(items);
  useEffect(() => {
    if (!set) return;
    set(JSON.parse(key) as Crumb[]);
    return () => set([]);
  }, [set, key]);
  return null;
}
