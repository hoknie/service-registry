"use client";

import { Activity } from "lucide-react";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { avg, num, pushPoint, RUNTIME_TILES, type MetricsView, type Point } from "@/lib/metrics";

import { MetricTiles } from "./MetricTiles";
import { MetricsShell, type MetricsLabels } from "./MetricsShell";

export function StatsAdmin({ locale, labels: t }: { locale: Locale; labels: MetricsLabels }) {
  return (
    <MetricsShell locale={locale} t={t} title={t.stats.title} lead={t.stats.lead} glyph={Activity}>
      {({ view }) => <Tiles locale={locale} t={t} view={view} />}
    </MetricsShell>
  );
}

type History = { instance: string | null; stamp: string | null; points: Record<string, Point[]> };

function runtimeValues(v: MetricsView): Record<string, number | null> {
  const out: Record<string, number | null> = {};
  for (const [k, raw] of Object.entries(v.runtime)) out[k] = num(raw);
  out.gc_pause = avg(out.go_gc_duration_seconds_sum ?? null, out.go_gc_duration_seconds_count ?? null);
  return out;
}

function Tiles({ locale, t, view }: { locale: Locale; t: MetricsLabels; view: MetricsView }) {
  const [history, setHistory] = useState<History>({ instance: null, stamp: null, points: {} });

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setHistory((h) => {
      const base = h.instance === view.instance ? h : { instance: view.instance, stamp: null, points: {} };
      const stamp = view.sampled_at.runtime ?? null;
      if (!stamp || stamp === base.stamp) return base;
      const at = Date.parse(stamp);
      const points = { ...base.points };
      for (const [k, v] of Object.entries(runtimeValues(view))) {
        if (v !== null && Number.isFinite(v)) points[k] = pushPoint(points[k] ?? [], { t: at, v });
      }
      return { ...base, stamp, points };
    });
  }, [view]);

  return <MetricTiles locale={locale} t={t} tiles={RUNTIME_TILES} values={runtimeValues(view)} points={history.points} />;
}
