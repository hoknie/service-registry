"use client";

import { useEffect, useState } from "react";

import { cn } from "@/lib/cn";

export function Markdown({ source, className }: { source: string; className?: string }) {
  const [html, setHtml] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    void import("@/lib/readme").then(({ renderReadme }) => {
      if (live) setHtml(renderReadme(source, { branch: null }));
    });
    return () => {
      live = false;
    };
  }, [source]);

  if (html === null) return <p className={cn("text-sm whitespace-pre-wrap text-ink-2", className)}>{source}</p>;
  return <div className={cn("readme max-w-none text-sm leading-relaxed text-ink-2", className)} dangerouslySetInnerHTML={{ __html: html }} />;
}
