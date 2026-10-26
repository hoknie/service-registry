"use client";

import { Cpu, Database, FileStack, Gauge, Layers, MemoryStick, Recycle, Timer, Workflow, type LucideIcon } from "lucide-react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import { formatBytes, formatNumber, rateSeries, type Point, type Tile } from "@/lib/metrics";

import { StatCard } from "../ui/StatCard";
import { Sparkline } from "../ui/Sparkline";
import { Tooltip } from "../ui/Tooltip";
import type { MetricsLabels } from "./MetricsShell";

const icons: Record<string, LucideIcon> = {
  goroutines: Workflow,
  threads: Layers,
  gomaxprocs: Cpu,
  cpu: Cpu,
  heapInuse: MemoryStick,
  heapAlloc: MemoryStick,
  heapObjects: Database,
  sys: MemoryStick,
  stack: Layers,
  nextGc: Recycle,
  resident: MemoryStick,
  allocRate: Gauge,
  gcRate: Recycle,
  gcPause: Timer,
  fds: FileStack,
};

type Props = { locale: Locale; t: MetricsLabels; tiles: Tile[]; values: Record<string, number | null>; points: Record<string, Point[]> };

export function MetricTiles({ locale, t, tiles, values, points }: Props) {
  const byteUnits = [t.units.bytes.b, t.units.bytes.kib, t.units.bytes.mib, t.units.bytes.gib, t.units.bytes.tib];
  const show = (v: number, tile: Tile): string => {
    switch (tile.unit) {
      case "bytes":
        return formatBytes(v, locale, byteUnits);
      case "bytesPerSec":
        return `${formatBytes(v, locale, byteUnits)}${t.units.perSec}`;
      case "cores":
        return format(t.units.cores, { n: formatNumber(v, locale) });
      case "perMin":
        return format(t.units.perMin, { n: formatNumber(v * 60, locale) });
      case "seconds":
        return v < 1 ? format(t.units.ms, { n: formatNumber(v * 1000, locale) }) : format(t.units.s, { n: formatNumber(v, locale) });
      default:
        return formatNumber(v, locale);
    }
  };
  return (
    <section aria-label={t.runtime} className="grid grid-cols-[repeat(auto-fill,minmax(14rem,1fr))] gap-4">
      {tiles.map((tile) => {
        const raw = points[tile.metric] ?? [];
        const line = tile.rate ? rateSeries(raw) : raw;
        const current = tile.rate ? (line.at(-1)?.v ?? null) : values[tile.metric];
        const title = t.tiles[tile.key as keyof typeof t.tiles];
        const caption = tile.key === "fds" && values.process_max_fds != null ? format(t.ofMax, { n: formatNumber(values.process_max_fds, locale) }) : undefined;
        return (
          <StatCard key={tile.key} icon={icons[tile.key] ?? Gauge} title={title} className="min-h-32">
            {current === null || current === undefined ? (
              <Tooltip content={t.unavailable}>
                <p className="w-fit font-mono text-3xl font-semibold text-muted" tabIndex={0}>
                  —
                </p>
              </Tooltip>
            ) : (
              <p className="font-mono text-2xl font-semibold tracking-tight text-ink tabular-nums">{show(current, tile)}</p>
            )}
            {caption && <p className="mt-0.5 text-sm text-muted">{caption}</p>}
            <Sparkline points={line} className="mt-2" />
          </StatCard>
        );
      })}
    </section>
  );
}
