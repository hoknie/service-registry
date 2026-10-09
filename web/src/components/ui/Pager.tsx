import { ChevronLeft, ChevronRight } from "lucide-react";

import { format } from "@/i18n/format";

import { Button } from "./Button";

type Labels = { label?: string; prev: string; next: string; range: string };

export function Pager({
  offset,
  shown,
  total,
  size,
  labels,
  onChange,
}: {
  offset: number;
  shown: number;
  total: number;
  size: number;
  labels: Labels;
  onChange: (offset: number) => void;
}) {
  if (total <= size && offset === 0) return null;
  return (
    <nav aria-label={labels.label ?? labels.range} className="mt-3 flex items-center justify-end gap-3 text-sm">
      <span className="text-muted tabular-nums">
        {format(labels.range, { from: shown ? offset + 1 : 0, to: offset + shown, total })}
      </span>
      <Button size="sm" disabled={offset === 0} onClick={() => onChange(Math.max(0, offset - size))}>
        <ChevronLeft aria-hidden="true" />
        {labels.prev}
      </Button>
      <Button size="sm" disabled={offset + size >= total} onClick={() => onChange(offset + size)}>
        {labels.next}
        <ChevronRight aria-hidden="true" />
      </Button>
    </nav>
  );
}
