"use client";

import { useEffect, useRef, useState } from "react";

import { cn } from "@/lib/cn";

import type { DocHeading } from "../knowledge/DocViewer";

export function DocToc({ headings, label }: { headings: DocHeading[]; label: string }) {
  const [active, setActive] = useState<string | null>(null);
  const top = Math.min(...headings.map((h) => h.level));
  const nav = useRef<HTMLElement>(null);

  useEffect(() => {
    const link = nav.current?.querySelector<HTMLElement>('[aria-current="location"]');
    if (!link) return;
    let box = link.parentElement;
    while (box && !(box.scrollHeight > box.clientHeight && getComputedStyle(box).overflowY !== "visible")) box = box.parentElement;
    if (!box || box === document.body || box === document.documentElement) return;
    const linkBox = link.getBoundingClientRect();
    const view = box.getBoundingClientRect();
    if (linkBox.top < view.top + 8) box.scrollTop -= view.top + 8 - linkBox.top;
    else if (linkBox.bottom > view.bottom - 8) box.scrollTop += linkBox.bottom - view.bottom + 8;
  }, [active]);

  useEffect(() => {
    if (headings.length === 0) return;
    const els = headings.map((h) => document.getElementById(h.id)).filter((e): e is HTMLElement => !!e);
    const observer = new IntersectionObserver(
      (entries) => {
        const seen = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (seen[0]) setActive(seen[0].target.id);
      },
      { rootMargin: "-72px 0px -65% 0px" },
    );
    els.forEach((e) => observer.observe(e));
    return () => observer.disconnect();
  }, [headings]);

  return (
    <nav ref={nav} aria-label={label}>
      <ul className="grid gap-0.5 border-l border-line">
        {headings.map((h) => (
          <li key={h.id}>
            <a
              href={`#${encodeURIComponent(h.id)}`}
              aria-current={active === h.id ? "location" : undefined}
              className={cn(
                "motion-control -ml-px block border-l-2 py-1 pr-1 text-sm leading-snug",
                active === h.id ? "border-signal font-medium text-signal" : "border-transparent text-ink-2 hover:border-line-strong hover:text-ink",
              )}
              style={{ paddingLeft: `${0.6 + (h.level - top) * 0.75}rem` }}
            >
              {h.text}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
