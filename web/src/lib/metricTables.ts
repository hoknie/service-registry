export type MetricTable = { id: string; metric: string; filter: string };

export const METRIC_TABLES_KEY = "svcr-metric-tables";
export const MAX_TABLES = 10;

export function readTables(raw: string | null, newId: () => string): MetricTable[] {
  let parsed: unknown = null;
  try {
    parsed = raw ? JSON.parse(raw) : null;
  } catch {
    parsed = null;
  }
  if (!Array.isArray(parsed)) return [{ id: newId(), metric: "", filter: "" }];
  const seen = new Set<string>();
  const out: MetricTable[] = [];
  for (const item of parsed) {
    if (!item || typeof item !== "object") continue;
    const { id, metric, filter } = item as Record<string, unknown>;
    const key = typeof id === "string" && id !== "" && !seen.has(id) ? id : newId();
    seen.add(key);
    out.push({ id: key, metric: typeof metric === "string" ? metric : "", filter: typeof filter === "string" ? filter : "" });
    if (out.length === MAX_TABLES) break;
  }
  return out;
}

export function addTable(tables: MetricTable[], id: string, metric = ""): MetricTable[] {
  return tables.length >= MAX_TABLES ? tables : [...tables, { id, metric, filter: "" }];
}

export function removeTable(tables: MetricTable[], id: string): MetricTable[] {
  return tables.filter((t) => t.id !== id);
}

export function updateTable(tables: MetricTable[], id: string, patch: Partial<Omit<MetricTable, "id">>): MetricTable[] {
  return tables.map((t) => (t.id === id ? { ...t, ...patch } : t));
}
