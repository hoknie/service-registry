import { cn } from "@/lib/cn";

import { CopyButton } from "./CopyButton";

export function CodeBlock({ code, title, className }: { code: string; title?: string; className?: string }) {
  return (
    <figure className={cn("overflow-hidden rounded-lg border border-line bg-surface-2", className)}>
      <figcaption className="flex items-center justify-between gap-3 border-b border-line px-3 py-1.5">
        <span className="truncate text-xs font-medium text-muted">{title}</span>
        <CopyButton text={code} variant="ghost" />
      </figcaption>
      <pre className="overflow-x-auto p-4 text-[0.8125rem] leading-relaxed text-ink">
        <code>{code}</code>
      </pre>
    </figure>
  );
}
