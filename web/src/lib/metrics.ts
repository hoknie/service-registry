export type MetricNumber = number | "NaN" | "+Inf" | "-Inf";
export type MetricType = "counter" | "gauge" | "histogram" | "summary" | "untyped";

export type MetricFamilyInfo = { name: string; help: string; type: MetricType; series: number };

export type MetricsView = {
  instance: string;
  version: string;
  go_version: string;
  started_at: string | null;
  refresh_secs: number;
  sampled_at: Record<string, string>;
  runtime: Record<string, MetricNumber | null>;
  families: MetricFamilyInfo[];
};

export type MetricSeries = {
  labels: Record<string, string>;
  value?: MetricNumber;
  count?: MetricNumber;
  sum?: MetricNumber;
  buckets?: { le: MetricNumber; count: MetricNumber }[];
  quantiles?: { quantile: MetricNumber; value: MetricNumber }[];
};

export type MetricFamily = MetricFamilyInfo & { series: MetricSeries[] };

export type Point = { t: number; v: number };

export const MAX_POINTS = 120;
export const DEFAULT_METRIC = "svcr_http_requests_total";
export const METRICS_REFRESH_KEY = "svcr-metrics-refresh";

export type TileUnit = "count" | "bytes" | "cores" | "bytesPerSec" | "perMin" | "seconds";
export type Tile = { key: string; metric: string; unit: TileUnit; rate?: boolean };

export const RUNTIME_TILES: Tile[] = [
  { key: "goroutines", metric: "go_goroutines", unit: "count" },
  { key: "threads", metric: "go_threads", unit: "count" },
  { key: "gomaxprocs", metric: "go_sched_gomaxprocs_threads", unit: "count" },
  { key: "cpu", metric: "process_cpu_seconds_total", unit: "cores", rate: true },
  { key: "heapInuse", metric: "go_memstats_heap_inuse_bytes", unit: "bytes" },
  { key: "heapAlloc", metric: "go_memstats_heap_alloc_bytes", unit: "bytes" },
  { key: "heapObjects", metric: "go_memstats_heap_objects", unit: "count" },
  { key: "sys", metric: "go_memstats_sys_bytes", unit: "bytes" },
  { key: "stack", metric: "go_memstats_stack_inuse_bytes", unit: "bytes" },
  { key: "nextGc", metric: "go_memstats_next_gc_bytes", unit: "bytes" },
  { key: "resident", metric: "process_resident_memory_bytes", unit: "bytes" },
  { key: "allocRate", metric: "go_memstats_alloc_bytes_total", unit: "bytesPerSec", rate: true },
  { key: "gcRate", metric: "go_gc_duration_seconds_count", unit: "perMin", rate: true },
  { key: "gcPause", metric: "gc_pause", unit: "seconds" },
  { key: "fds", metric: "process_open_fds", unit: "count" },
];

export function num(v: MetricNumber | null | undefined): number | null {
  if (v === null || v === undefined) return null;
  if (typeof v === "number") return v;
  if (v === "+Inf") return Infinity;
  if (v === "-Inf") return -Infinity;
  return NaN;
}

export function rate(prev: Point | undefined, next: Point): number | null {
  if (!prev || next.t <= prev.t || next.v < prev.v) return null;
  return (next.v - prev.v) / ((next.t - prev.t) / 1000);
}

export function pushPoint(buffer: Point[], point: Point, max = MAX_POINTS): Point[] {
  const out = buffer.length > 0 && buffer[buffer.length - 1]!.t === point.t ? buffer.slice(0, -1) : buffer.slice();
  out.push(point);
  return out.length > max ? out.slice(out.length - max) : out;
}

export function rates(points: Point[]): Point[] {
  const out: Point[] = [];
  for (let i = 1; i < points.length; i++) {
    const r = rate(points[i - 1], points[i]!);
    if (r !== null) out.push({ t: points[i]!.t, v: r });
  }
  return out;
}

export function rateSeries(points: Point[]): Point[] {
  const line = rates(points);
  return line.length > 0 || points.length === 0 ? line : [{ t: points[0]!.t, v: 0 }];
}

export function avg(sum: number | null, count: number | null): number | null {
  if (sum === null || count === null || count === 0) return null;
  return sum / count;
}

export function seriesKey(family: string, labels: Record<string, string>): string {
  return (
    family +
    "{" +
    Object.keys(labels)
      .sort()
      .map((k) => `${k}=${labels[k]}`)
      .join(",") +
    "}"
  );
}

export function labelNames(series: MetricSeries[]): string[] {
  const names = new Set<string>();
  for (const s of series) for (const k of Object.keys(s.labels)) names.add(k);
  return [...names].sort();
}

export function matchesFilter(series: MetricSeries, filter: string): boolean {
  const q = filter.trim().toLowerCase();
  if (q === "") return true;
  return Object.values(series.labels).some((v) => v.toLowerCase().includes(q));
}

const BYTE_UNITS = ["B", "KiB", "MiB", "GiB", "TiB"] as const;

export function formatBytes(bytes: number, locale: string, units: readonly string[] = BYTE_UNITS): string {
  let v = Math.abs(bytes);
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  const text = new Intl.NumberFormat(locale, { maximumFractionDigits: i === 0 ? 0 : 1 }).format(bytes < 0 ? -v : v);
  return `${text} ${units[i]}`;
}

export function formatNumber(v: number, locale: string): string {
  if (!Number.isFinite(v)) return String(v);
  const digits = Math.abs(v) >= 100 || Number.isInteger(v) ? 0 : Math.abs(v) >= 1 ? 2 : 4;
  return new Intl.NumberFormat(locale, { maximumFractionDigits: digits }).format(v);
}

export function uptimeSeconds(startedAt: string | null, now: number): number | null {
  if (!startedAt) return null;
  const t = Date.parse(startedAt);
  return Number.isNaN(t) ? null : Math.max(0, Math.floor((now - t) / 1000));
}

export function pickFamily(families: MetricFamilyInfo[], wanted: string | null): string | null {
  if (wanted && families.some((f) => f.name === wanted)) return wanted;
  if (families.some((f) => f.name === DEFAULT_METRIC)) return DEFAULT_METRIC;
  return families[0]?.name ?? null;
}

export function familyPath(name: string): `/${string}` {
  return `/v1/system/metrics/family?name=${encodeURIComponent(name)}`;
}

export function formatUptime(secs: number, locale: string): string {
  const parts: [Intl.NumberFormatOptions["unit"], number][] = [
    ["day", Math.floor(secs / 86400)],
    ["hour", Math.floor((secs % 86400) / 3600)],
    ["minute", Math.floor((secs % 3600) / 60)],
  ];
  const shown = parts.filter(([, n]) => n > 0).slice(0, 2);
  if (shown.length === 0) return new Intl.NumberFormat(locale, { style: "unit", unit: "second", unitDisplay: "short" }).format(secs);
  return shown.map(([unit, n]) => new Intl.NumberFormat(locale, { style: "unit", unit, unitDisplay: "short" }).format(n)).join(" ");
}
