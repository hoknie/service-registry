import type { LabelKey, LabelValue } from "./api";

export type Suggestion = { value: string; label: string; detail?: string };

export type LabelQuery = { key: string | null; q: string };

export function labelQuery(draft: string): LabelQuery {
  const at = draft.indexOf("=");
  if (at < 0) return { key: null, q: draft.trim() };
  return { key: draft.slice(0, at).trim(), q: draft.slice(at + 1).trim() };
}

export function labelsPath(q: LabelQuery): `/${string}` {
  const params = new URLSearchParams();
  if (q.key !== null) params.set("key", q.key);
  params.set("q", q.q);
  return `/v1/catalog/labels?${params.toString()}`;
}

export function labelSuggestions(q: LabelQuery, keys: LabelKey[] | null, values: LabelValue[] | null): Suggestion[] {
  if (q.key === null) return (keys ?? []).map((k) => ({ value: k.key, label: k.key, detail: String(k.count) }));
  const key = q.key;
  return (values ?? []).map((v) => ({ value: v.value, label: v.value === "" ? key : `${key}=${v.value}`, detail: String(v.count) }));
}

export function pickLabel(draft: string, s: Suggestion): { draft: string; commit: boolean } {
  const q = labelQuery(draft);
  if (q.key === null) return { draft: `${s.value}=`, commit: false };
  return { draft: s.value === "" ? q.key : `${q.key}=${s.value}`, commit: true };
}

export function lastSegment(text: string): { head: string; tail: string } {
  const at = text.lastIndexOf(",");
  if (at < 0) return { head: "", tail: text.trimStart() };
  return { head: text.slice(0, at + 1) + " ", tail: text.slice(at + 1).trimStart() };
}

export function pickFilter(text: string, s: Suggestion): string {
  const { head, tail } = lastSegment(text);
  return head + pickLabel(tail, s).draft;
}
