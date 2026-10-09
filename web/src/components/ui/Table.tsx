import type { HTMLAttributes, TdHTMLAttributes, ThHTMLAttributes } from "react";

import { cn } from "@/lib/cn";

export function Table({ className, ...rest }: HTMLAttributes<HTMLTableElement>) {
  return (
    <div className="-mx-px max-h-[75dvh] overflow-auto rounded-card border border-line bg-surface shadow-elev-1">
      <table className={cn("w-full border-collapse text-sm", className)} {...rest} />
    </div>
  );
}

export function Th({ className, numeric, ...rest }: ThHTMLAttributes<HTMLTableCellElement> & { numeric?: boolean }) {
  return (
    <th
      scope="col"
      className={cn(
        "sticky top-0 z-[1] border-b border-line bg-surface-2/95 px-3 py-2.5 text-2xs font-semibold tracking-wide whitespace-nowrap text-muted uppercase backdrop-blur first:pl-4 last:pr-4",
        numeric ? "text-right" : "text-left",
        className,
      )}
      {...rest}
    />
  );
}

export function Td({ className, numeric, ...rest }: TdHTMLAttributes<HTMLTableCellElement> & { numeric?: boolean }) {
  return (
    <td
      className={cn("border-b border-line px-3 py-2.5 align-middle first:pl-4 last:pr-4", numeric && "text-right font-mono tabular-nums", className)}
      {...rest}
    />
  );
}

export function Tr({ className, ...rest }: HTMLAttributes<HTMLTableRowElement>) {
  return (
    <tr
      className={cn("motion-control animate-fade-in last:[&>td]:border-b-0 hover:bg-surface-2 aria-selected:bg-signal-soft/60", className)}
      {...rest}
    />
  );
}
