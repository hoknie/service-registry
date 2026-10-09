export type StructuredKind = "json" | "yaml";

export function structuredKind(path: string): StructuredKind | null {
  if (/\.json$/i.test(path)) return "json";
  if (/\.ya?ml$/i.test(path)) return "yaml";
  return null;
}

export type ValueType = "object" | "array" | "string" | "number" | "boolean" | "null";

export function valueType(v: unknown): ValueType {
  if (v === null || v === undefined) return "null";
  if (Array.isArray(v)) return "array";
  if (typeof v === "string") return "string";
  if (typeof v === "number" || typeof v === "bigint") return "number";
  if (typeof v === "boolean") return "boolean";
  if (v instanceof Date) return "string";
  return typeof v === "object" ? "object" : "string";
}

export function entries(v: unknown): [string, unknown][] {
  if (Array.isArray(v)) return v.map((x, i) => [String(i), x]);
  if (v && typeof v === "object" && !(v instanceof Date)) {
    if (v instanceof Map) return [...v.entries()].map(([k, x]) => [String(k), x]);
    return Object.entries(v);
  }
  return [];
}

export function leafText(v: unknown): string {
  if (v === null || v === undefined) return "null";
  if (v instanceof Date) return JSON.stringify(v.toISOString());
  if (typeof v === "string") return JSON.stringify(v);
  return String(v);
}

export function parseJson(text: string): unknown[] {
  return [JSON.parse(text)];
}
