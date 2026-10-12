"use client";

import { BellRing, BookOpenCheck, BookText, Bug, ChartLine, Link, ScrollText, Waypoints, type LucideIcon } from "lucide-react";
import { createElement, useEffect, useState } from "react";

import { cn } from "@/lib/cn";
import { loadedIcon, loadIcon } from "@/lib/iconSet";

const bundled: Record<string, LucideIcon> = {
  link: Link,
  "scroll-text": ScrollText,
  "chart-line": ChartLine,
  bug: Bug,
  "bell-ring": BellRing,
  waypoints: Waypoints,
  "book-open-check": BookOpenCheck,
  "book-text": BookText,
};

function useIcon(name: string): LucideIcon | null | undefined {
  const known = bundled[name] ?? loadedIcon(name);
  const [loaded, setLoaded] = useState<{ name: string; icon: LucideIcon | null } | null>(null);
  useEffect(() => {
    if (known !== undefined) return;
    let live = true;
    void loadIcon(name).then((icon) => {
      if (live) setLoaded({ name, icon });
    });
    return () => {
      live = false;
    };
  }, [name, known]);
  if (known !== undefined) return known;
  return loaded?.name === name ? loaded.icon : undefined;
}

export function SetIcon({ name, className }: { name: string | undefined | null; className?: string }) {
  const Comp = useIcon(name || "link");
  const cls = cn("size-4 shrink-0", className);
  if (Comp === undefined) return <span aria-hidden="true" className={cn(cls, "inline-block")} />;
  return createElement(Comp ?? Link, { "aria-hidden": true, className: cls });
}
