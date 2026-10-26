"use client";

import { ArrowDown, ArrowUp, ListX, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { ComboOption } from "@/lib/combobox";
import {
  avg,
  familyPath,
  formatNumber,
  labelNames,
  matchesFilter,
  num,
  pickFamily,
  pushPoint,
  rates,
  seriesKey,
  type MetricFamily,
  type MetricSeries,
  type Point,
} from "@/lib/metrics";
import type { MetricTable as Stored } from "@/lib/metricTables";
import { usePolledGet } from "@/lib/useAutoRefresh";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Combobox } from "../ui/Combobox";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Message } from "../ui/Message";
import { Sparkline } from "../ui/Sparkline";
import { Table, Td, Th, Tr } from "../ui/Table";
import type { Live, MetricsLabels } from "./MetricsShell";

type Props = {
  locale: Locale;
  t: MetricsLabels;
  live: Live;
  table: Stored;
  onChange: (patch: Partial<Omit<Stored, "id">>) => void;
  onRemove: () => void;
};
type Column = { key: string; label: string; numeric: boolean; value: (s: MetricSeries) => number | string | null };
type Sort = { key: string; desc: boolean };
type Track = { instance: string; points: Record<string, Point[]> };

function measure(type: MetricFamily["type"], s: MetricSeries): number | null {
  return num(type === "histogram" || type === "summary" ? s.count : s.value);
}

export function MetricTable({ locale, t, live, table, onChange, onRemove }: Props) {
  const { errors } = useUiText();
  const { interval, tick } = live;
  const families = live.view.families;
  const instance = live.view.instance;
  const name = pickFamily(families, table.metric || null);
  const filter = table.filter;
  const [sort, setSort] = useState<Sort | null>(null);
  const [gone, setGone] = useState<string | null>(null);
  const [track, setTrack] = useState<Track>({ instance, points: {} });
  const path = familyPath(name ?? "");
  const { page, failure, refresh } = usePolledGet<MetricFamily>(path, interval, errors);
  const family = page?.key === path ? page.data : null;
  const notFound = failure?.key === path && failure.text === errorText(errors, "not_found");

  useEffect(() => {
    if (tick > 0) refresh();
  }, [tick, refresh]);

  const lost = table.metric !== "" && name !== table.metric ? table.metric : null;
  useEffect(() => {
    const missing = notFound && name ? name : lost;
    if (!missing) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setGone(missing);
    onChange({ metric: "" });
  }, [notFound, name, lost, onChange]);

  useEffect(() => {
    if (!family) return;
    const at = Date.now();
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTrack((tr) => {
      const points = tr.instance === instance ? { ...tr.points } : {};
      for (const s of family.series) {
        const v = measure(family.type, s);
        if (v === null || !Number.isFinite(v)) continue;
        const key = seriesKey(family.name, s.labels);
        points[key] = pushPoint(points[key] ?? [], { t: at, v });
      }
      return { instance, points };
    });
  }, [family, instance]);

  const options: ComboOption[] = useMemo(
    () => families.map((f) => ({ value: f.name, label: f.name, detail: `${t.types[f.type]} · ${format(t.table.seriesCount, { n: f.series })}` })),
    [families, t],
  );

  const choose = (v: string) => {
    setGone(null);
    setSort(null);
    onChange({ metric: v });
  };

  const lineOf = (s: MetricSeries): Point[] => {
    if (!family) return [];
    const p = track.points[seriesKey(family.name, s.labels)] ?? [];
    return family.type === "gauge" || family.type === "untyped" ? p : rates(p);
  };
  const rateOf = (s: MetricSeries) => {
    const r = lineOf(s);
    return family?.type === "gauge" || family?.type === "untyped" ? null : (r.at(-1)?.v ?? null);
  };

  const columns: Column[] = useMemo(() => {
    if (!family) return [];
    const labels: Column[] = labelNames(family.series).map((l) => ({ key: `l:${l}`, label: l, numeric: false, value: (s) => s.labels[l] ?? "" }));
    const rest: Column[] =
      family.type === "histogram" || family.type === "summary"
        ? [
            { key: "count", label: t.table.count, numeric: true, value: (s) => num(s.count) },
            { key: "sum", label: t.table.sum, numeric: true, value: (s) => num(s.sum) },
            { key: "avg", label: t.table.avg, numeric: true, value: (s) => avg(num(s.sum), num(s.count)) },
          ]
        : [{ key: "value", label: t.table.value, numeric: true, value: (s) => num(s.value) }];
    return [...labels, ...rest];
  }, [family, t]);

  const rows = useMemo(() => {
    if (!family) return [];
    const kept = family.series.filter((s) => matchesFilter(s, filter));
    const col = sort && columns.find((c) => c.key === sort.key);
    if (!col) return kept;
    return [...kept].sort((a, b) => {
      const x = col.value(a);
      const y = col.value(b);
      const cmp = typeof x === "number" && typeof y === "number" ? x - y : String(x ?? "").localeCompare(String(y ?? ""));
      return sort!.desc ? -cmp : cmp;
    });
  }, [family, filter, sort, columns]);

  const counting = family !== null && family.type !== "gauge" && family.type !== "untyped";
  const show = (v: number | string | null) => (v === null ? "—" : typeof v === "number" ? formatNumber(v, locale) : v);

  return (
    <section aria-label={t.table.label} className="grid gap-4">
      <div className="flex flex-wrap items-end gap-3 rounded-card border border-line bg-surface p-4 shadow-elev-1">
        <Field label={t.table.metric} className="min-w-0 flex-[2_1_20rem]">
          {(p) => <Combobox {...p} value={name ?? ""} options={options} onChange={(v) => v && v !== name && choose(v)} />}
        </Field>
        <Field label={t.table.filter} className="min-w-0 flex-[1_1_14rem]">
          {(p) => <Input {...p} value={filter} placeholder={t.table.filterPlaceholder} onChange={(e) => onChange({ filter: e.target.value })} />}
        </Field>
        <Button variant="danger-ghost" onClick={onRemove}>
          <Trash2 aria-hidden="true" />
          {t.tables.remove}
        </Button>
        {family && (
          <p className="basis-full text-sm text-muted">
            <span className="font-mono text-ink">{t.types[family.type]}</span> · {family.help}
          </p>
        )}
      </div>
      {gone && <Message note={{ kind: "error", text: format(t.table.gone, { name: gone }) }} />}
      {failure?.key === path && !notFound && <Message note={{ kind: "error", text: failure.text }} />}
      {family && family.series.length === 0 && <EmptyState icon={ListX} title={t.table.empty} />}
      {family && family.series.length > 0 && rows.length === 0 && <EmptyState icon={ListX} title={t.table.noMatch} />}
      {family && rows.length > 0 && (
        <Table>
          <thead>
            <tr>
              {columns.map((c) => (
                <Th key={c.key} numeric={c.numeric} aria-sort={sort?.key === c.key ? (sort.desc ? "descending" : "ascending") : undefined}>
                  <button
                    type="button"
                    className="inline-flex items-center gap-1 uppercase hover:text-ink"
                    onClick={() => setSort((s) => (s?.key === c.key ? { key: c.key, desc: !s.desc } : { key: c.key, desc: c.numeric }))}
                  >
                    {c.label}
                    {sort?.key === c.key && (sort.desc ? <ArrowDown aria-hidden="true" className="size-3" /> : <ArrowUp aria-hidden="true" className="size-3" />)}
                  </button>
                </Th>
              ))}
              {counting && <Th numeric>{t.table.rate}</Th>}
              {family.type === "summary" && <Th>{t.table.quantiles}</Th>}
              <Th className="w-40">{t.table.chart}</Th>
            </tr>
          </thead>
          <tbody>
            {rows.map((s) => (
              <Tr key={seriesKey(family.name, s.labels)}>
                {columns.map((c) => (
                  <Td key={c.key} numeric={c.numeric} className={c.numeric ? undefined : "font-mono text-xs break-all"}>
                    {show(c.value(s))}
                  </Td>
                ))}
                {counting && <Td numeric>{rateOf(s) === null ? "—" : format(t.units.perSecValue, { n: formatNumber(rateOf(s)!, locale) })}</Td>}
                {family.type === "summary" && (
                  <Td className="font-mono text-xs whitespace-nowrap">
                    {(s.quantiles ?? []).map((q) => `p${formatNumber((num(q.quantile) ?? 0) * 100, locale)}=${show(num(q.value))}`).join(" ")}
                  </Td>
                )}
                <Td className="w-40">
                  <Sparkline points={lineOf(s)} />
                </Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      )}
    </section>
  );
}
