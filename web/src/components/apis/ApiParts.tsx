"use client";

import { useEffect, useState } from "react";

import { methodTone } from "@/lib/apis";
import { cn } from "@/lib/cn";

const toneClass: Record<string, string> = {
  get: "text-method-get",
  post: "text-method-post",
  put: "text-method-put",
  delete: "text-method-delete",
  other: "text-method-other",
};

export function MethodBadge({ method, className }: { method: string; className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex min-w-16 justify-center rounded-md border border-line bg-surface-2 px-1.5 py-0.5 font-mono text-xs font-semibold",
        toneClass[methodTone(method)],
        className,
      )}
    >
      {method.toUpperCase()}
    </span>
  );
}

export function ApiMarkdown({ text, className }: { text: string | null | undefined; className?: string }) {
  const [html, setHtml] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    if (!text) return;
    import("@/lib/readme").then(({ renderReadme }) => live && setHtml(renderReadme(text, { branch: null })));
    return () => {
      live = false;
    };
  }, [text]);
  if (!text) return null;
  if (html === null) return <p className={cn("text-sm whitespace-pre-wrap text-ink-2", className)}>{text}</p>;
  return <div className={cn("readme max-w-none text-sm leading-relaxed text-ink-2", className)} dangerouslySetInnerHTML={{ __html: html }} />;
}

export function SectionTitle({ children }: { children: React.ReactNode }) {
  return <h3 className="mt-6 mb-2 text-sm font-semibold tracking-wide text-muted uppercase">{children}</h3>;
}
