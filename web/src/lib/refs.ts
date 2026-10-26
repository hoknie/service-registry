export type RefKind = "branch" | "tag";

export type RefPattern = { pattern: string; kind: RefKind; release: boolean };

export type Version = { kind: RefKind; name: string };

export const REF_EXAMPLES: readonly RefPattern[] = [
  { pattern: "feat/*", kind: "branch", release: false },
  { pattern: "release/*", kind: "branch", release: false },
  { pattern: "v*", kind: "tag", release: false },
  { pattern: "v*-release", kind: "tag", release: true },
];

const TAG_PREFIX = "tag:";

export function versionKey(kind: RefKind, name: string): string {
  return kind === "tag" ? `${TAG_PREFIX}${name}` : name;
}

export function splitVersion(key: string): Version {
  return key.startsWith(TAG_PREFIX) ? { kind: "tag", name: key.slice(TAG_PREFIX.length) } : { kind: "branch", name: key };
}

export function refQuery(key: string | null | undefined, extra: Record<string, string> = {}): URLSearchParams {
  const q = new URLSearchParams(extra);
  if (key) {
    const v = splitVersion(key);
    q.set(v.kind, v.name);
  }
  return q;
}

export function versionFromParams(params: URLSearchParams): string | null {
  const tag = params.get("tag");
  if (tag) return versionKey("tag", tag);
  const branch = params.get("branch");
  return branch ? branch : null;
}

export function withKind(row: RefPattern, kind: RefKind): RefPattern {
  return { ...row, kind, release: kind === "tag" ? row.release : false };
}

export function refsOf(rows: readonly RefPattern[]): RefPattern[] {
  return rows.map((r) => ({ ...r, pattern: r.pattern.trim() })).filter((r) => r.pattern !== "");
}

export function duplicateRows(rows: readonly RefPattern[]): Set<number> {
  const seen = new Map<string, number>();
  const out = new Set<number>();
  rows.forEach((r, i) => {
    const key = `${r.kind}\u0000${r.pattern.trim()}`;
    if (r.pattern.trim() === "") return;
    const first = seen.get(key);
    if (first === undefined) seen.set(key, i);
    else out.add(i);
  });
  return out;
}

export function invalidRows(rows: readonly RefPattern[], valid: (pattern: string) => boolean): boolean {
  return duplicateRows(rows).size > 0 || rows.some((r) => r.pattern.trim() !== "" && !valid(r.pattern.trim()));
}

export function addExample(rows: readonly RefPattern[], example: RefPattern): RefPattern[] {
  if (rows.some((r) => r.kind === example.kind && r.pattern.trim() === example.pattern)) return [...rows];
  return [...rows, { ...example }];
}
