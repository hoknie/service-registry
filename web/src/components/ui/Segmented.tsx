"use client";

import { RadioGroup as R } from "radix-ui";

import { cn } from "@/lib/cn";

type Option = { value: string; label: string };

type Props = { label: string; value: string; options: Option[]; onChange: (value: string) => void; className?: string };

export function Segmented({ label, value, options, onChange, className }: Props) {
  return (
    <R.Root
      aria-label={label}
      value={value}
      onValueChange={onChange}
      orientation="horizontal"
      className={cn("inline-flex rounded-lg border border-line bg-surface-2 p-0.5", className)}
    >
      {options.map((o) => (
        <R.Item
          key={o.value}
          value={o.value}
          className={cn(
            "motion-control rounded-md px-3 py-1.5 text-sm font-medium text-muted hover:text-ink",
            "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-signal",
            "data-[state=checked]:bg-surface data-[state=checked]:text-ink data-[state=checked]:shadow-sm",
          )}
        >
          {o.label}
        </R.Item>
      ))}
    </R.Root>
  );
}
