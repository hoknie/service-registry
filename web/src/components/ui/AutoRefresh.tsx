"use client";

import { RefreshCw } from "lucide-react";

import { REFRESH_INTERVALS, parseInterval, type RefreshInterval } from "@/lib/autoRefresh";

import { Button } from "./Button";
import { Select } from "./Select";

export type AutoRefreshLabels = { label: string; off: string; every: string; now: string };

type Props = { labels: AutoRefreshLabels; value: RefreshInterval; onChange: (v: RefreshInterval) => void; busy: boolean; onRefresh: () => void };

export function AutoRefresh({ labels, value, onChange, busy, onRefresh }: Props) {
  return (
    <div className="flex flex-wrap items-center justify-end gap-2">
      <Select aria-label={labels.label} value={String(value)} onChange={(e) => onChange(parseInterval(e.target.value))} className="w-44">
        {REFRESH_INTERVALS.map((s) => (
          <option key={s} value={String(s)}>
            {s === 0 ? `${labels.label}: ${labels.off}` : `${labels.label}: ${labels.every.replace("{n}", String(s))}`}
          </option>
        ))}
      </Select>
      <Button size="sm" busy={busy} disabled={busy} onClick={onRefresh}>
        <RefreshCw aria-hidden="true" />
        {labels.now}
      </Button>
    </div>
  );
}
