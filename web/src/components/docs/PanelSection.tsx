"use client";

import { ChevronDown, type LucideIcon } from "lucide-react";
import { useId, useState, type ReactNode } from "react";

import { cn } from "@/lib/cn";

function stored(key: string): boolean {
  try {
    return window.localStorage.getItem(key) !== "collapsed";
  } catch {
    return true;
  }
}

export function PanelSection({ title, icon: Icon, storageKey, children }: { title: string; icon: LucideIcon; storageKey: string; children: ReactNode }) {
  const [open, setOpen] = useState(() => stored(storageKey));
  const id = useId();
  const toggle = () => {
    setOpen(!open);
    try {
      window.localStorage.setItem(storageKey, open ? "collapsed" : "expanded");
    } catch {
      return;
    }
  };
  return (
    <section className="grid min-w-0">
      <button
        type="button"
        onClick={toggle}
        aria-expanded={open}
        aria-controls={id}
        className="motion-control flex items-center gap-2 rounded-md px-1.5 py-1 text-left text-xs font-semibold tracking-wide text-muted uppercase hover:bg-surface-2 hover:text-ink"
      >
        <Icon aria-hidden="true" className="size-3.5 shrink-0" />
        <span className="min-w-0 flex-1 truncate">{title}</span>
        <ChevronDown aria-hidden="true" className={cn("motion-control size-3.5 shrink-0", !open && "-rotate-90")} />
      </button>
      <div id={id} hidden={!open} className="animate-fade-in min-w-0 pt-1.5">
        {children}
      </div>
    </section>
  );
}
