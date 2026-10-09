import type { HTMLAttributes } from "react";

import { cn } from "@/lib/cn";

const tones = {
  neutral: "bg-surface-3 text-ink-2 border-line",
  signal: "bg-signal-soft text-signal border-transparent",
  cobalt: "bg-cobalt-soft text-cobalt border-transparent",
  amber: "bg-amber-soft text-amber border-transparent",
  danger: "bg-danger-soft text-danger border-transparent",
  outline: "bg-transparent text-muted border-line-strong",
} as const;

export type Tone = keyof typeof tones;

export function Badge({
  tone = "neutral",
  dot,
  className,
  children,
  ...rest
}: HTMLAttributes<HTMLSpanElement> & { tone?: Tone; dot?: boolean }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium whitespace-nowrap",
        tones[tone],
        className,
      )}
      {...rest}
    >
      {dot && <span aria-hidden="true" className="size-1.5 rounded-full bg-current shadow-[0_0_0_3px_color-mix(in_srgb,currentColor_18%,transparent)]" />}
      {children}
    </span>
  );
}

export function LabelChip({ name, value }: { name: string; value: string }) {
  return (
    <span className="inline-flex max-w-full items-center overflow-hidden rounded-md border border-line bg-surface-2 font-mono text-xs leading-5">
      <span className={cn("px-1.5 text-muted", value && "border-r border-line")}>{name}</span>
      {value && <span className="truncate px-1.5 text-ink">{value}</span>}
    </span>
  );
}
