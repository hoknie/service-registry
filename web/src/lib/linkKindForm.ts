export const NAME_LOCALES = ["en", "es", "ru", "zh"] as const;
export type NameLocale = (typeof NAME_LOCALES)[number];

export function copySource(names: Partial<Record<NameLocale, string>>): string {
  for (const l of NAME_LOCALES) {
    const v = (names[l] ?? "").trim();
    if (v) return v;
  }
  return "";
}

export function namesBody(names: Partial<Record<NameLocale, string>>): Partial<Record<NameLocale, string>> {
  const out: Partial<Record<NameLocale, string>> = {};
  for (const l of NAME_LOCALES) {
    const v = (names[l] ?? "").trim();
    if (v) out[l] = v;
  }
  return out;
}

export function moveKey(keys: readonly string[], from: number, to: number): string[] {
  const out = [...keys];
  if (from < 0 || from >= out.length || to < 0 || to >= out.length || from === to) return out;
  out.splice(to, 0, ...out.splice(from, 1));
  return out;
}

export const ICON_NAME = /^[a-z0-9]+(-[a-z0-9]+)*$/;

export function searchIcons(names: readonly string[], query: string, limit = 60): string[] {
  const q = query.trim().toLowerCase().replace(/\s+/g, "-");
  if (!q) return names.slice(0, limit);
  const starts: string[] = [];
  const contains: string[] = [];
  for (const n of names) {
    if (n.startsWith(q)) starts.push(n);
    else if (n.includes(q)) contains.push(n);
    if (starts.length >= limit) break;
  }
  return [...starts, ...contains].slice(0, limit);
}
