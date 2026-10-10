import type { Scan, ScanSource } from "./api";

export type CodeText = { title: string; text: string; hint: string; hintGit?: string };
export type CodeBook = Record<string, Record<string, CodeText>>;

export const SCAN_STATUSES = ["ok", "unchanged", "warning", "failed"] as const;
export const SCAN_PAGE = 50;

export function scanCodeText(book: CodeBook, code: string, source: ScanSource | null): CodeText {
  const [group, name] = code.includes(".") ? [code.slice(0, code.indexOf(".")), code.slice(code.indexOf(".") + 1)] : ["other", code];
  const found = book[group]?.[name];
  if (!found) return { title: code, text: "", hint: "" };
  return { title: found.title, text: found.text, hint: source === "local_git" && found.hintGit ? found.hintGit : found.hint };
}

export function scanCodes(scan: Scan): string[] {
  const out: string[] = [];
  if (scan.error) out.push(scan.error.code);
  for (const b of scan.branches) if (b.error && !out.includes(b.error)) out.push(b.error);
  for (const w of scan.warnings) if (!out.includes(w)) out.push(w);
  return out;
}

export function scanTotals(scan: Scan): { branches: number; files: number; skipped: number } {
  let files = 0;
  let skipped = 0;
  for (const b of scan.branches) {
    files += b.files;
    for (const n of Object.values(b.skipped)) skipped += n;
  }
  return { branches: scan.branches.length, files, skipped };
}

export type ScanParams = { project: string; kind: string; status: string[]; trigger: string; page: number };

export function readScanParams(get: (name: string) => string | null): ScanParams {
  const page = Number.parseInt(get("page") ?? "1", 10);
  const status = (get("status") ?? "")
    .split(",")
    .map((s) => s.trim())
    .filter((s): s is (typeof SCAN_STATUSES)[number] => (SCAN_STATUSES as readonly string[]).includes(s));
  return {
    project: get("project") ?? "",
    kind: get("kind") === "collect" || get("kind") === "index" ? (get("kind") as string) : "",
    status,
    trigger: get("trigger") === "schedule" || get("trigger") === "manual" ? (get("trigger") as string) : "",
    page: Number.isFinite(page) && page > 0 ? page : 1,
  };
}

export function scanAddress(p: ScanParams): string {
  const q = new URLSearchParams();
  if (p.project) q.set("project", p.project);
  if (p.kind) q.set("kind", p.kind);
  if (p.status.length) q.set("status", p.status.join(","));
  if (p.trigger) q.set("trigger", p.trigger);
  if (p.page > 1) q.set("page", String(p.page));
  return q.toString();
}

export function scanQuery(p: ScanParams): string {
  const q = new URLSearchParams();
  if (p.project) q.set("project", p.project);
  if (p.kind) q.set("kind", p.kind);
  if (p.status.length) q.set("status", p.status.join(","));
  if (p.trigger) q.set("trigger", p.trigger);
  q.set("limit", String(SCAN_PAGE));
  q.set("offset", String((p.page - 1) * SCAN_PAGE));
  return q.toString();
}

export function formatDuration(ms: number, locale: string): string {
  if (ms < 1000) return new Intl.NumberFormat(locale, { style: "unit", unit: "millisecond", unitDisplay: "short" }).format(ms);
  if (ms < 60000)
    return new Intl.NumberFormat(locale, { style: "unit", unit: "second", unitDisplay: "short", maximumFractionDigits: 1 }).format(ms / 1000);
  return new Intl.NumberFormat(locale, { style: "unit", unit: "minute", unitDisplay: "short", maximumFractionDigits: 1 }).format(ms / 60000);
}

export function seriesText(
  template: string,
  scan: { repeats: number; started_at: string; finished_at: string },
  show: (iso: string) => string,
): string | null {
  if (scan.repeats <= 0) return null;
  return template
    .replace("{n}", String(scan.repeats + 1))
    .replace("{from}", show(scan.started_at))
    .replace("{to}", show(scan.finished_at));
}

export function withScanAddress(base: string, address: string): string {
  if (!address) return base;
  return base + (base.includes("?") ? "&" : "?") + address;
}
