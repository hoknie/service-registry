export function splitTags(text: string): string[] {
  const out: string[] = [];
  let depth = 0;
  let cur = "";
  const flush = () => {
    const t = cur.trim();
    if (t) out.push(t);
    cur = "";
  };
  for (const c of text) {
    if (c === "{") depth++;
    if (c === "}" && depth > 0) depth--;
    if (c === "\n" || c === "\r" || (c === "," && depth === 0)) flush();
    else cur += c;
  }
  flush();
  return out;
}

export function addTags(current: string[], incoming: string[]): string[] {
  const out = [...current];
  for (const t of incoming) if (!out.includes(t)) out.push(t);
  return out;
}

export const toTags = (text: string): string[] => text.split("\n").map((l) => l.trim()).filter(Boolean);

export const fromTags = (tags: string[]): string => tags.join("\n");

export function parseKeyValue(tag: string): { key: string; value: string } | null {
  const at = tag.indexOf("=");
  if (at <= 0) return null;
  const key = tag.slice(0, at).trim();
  if (!key || /\s/.test(key)) return null;
  return { key, value: tag.slice(at + 1).trim() };
}

export function validPattern(tag: string): boolean {
  let square = 0;
  let curly = 0;
  for (const c of tag) {
    if (c === "[") square++;
    else if (c === "]") square--;
    else if (c === "{") curly++;
    else if (c === "}") curly--;
    if (square < 0 || curly < 0) return false;
  }
  return square === 0 && curly === 0 && tag.length <= 200;
}

export const validDnsLabel = (tag: string): boolean => /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/.test(tag);

const LABEL_KEY = /^[a-z0-9](?:[a-z0-9._-]{0,61}[a-z0-9])?$/;

export function parseLabel(tag: string): { key: string; value: string } | null {
  const at = tag.indexOf("=");
  const key = (at < 0 ? tag : tag.slice(0, at)).trim();
  const value = at < 0 ? "" : tag.slice(at + 1).trim();
  if (!LABEL_KEY.test(key) || [...value].length > 63 || /\p{Cc}/u.test(value)) return null;
  return { key, value };
}

export function labelsOf(lines: string, draft = ""): Record<string, string> {
  const out: Record<string, string> = {};
  for (const tag of addTags(toTags(lines), splitTags(draft))) {
    const l = parseLabel(tag);
    if (l) out[l.key] = l.value;
  }
  return out;
}

export function labelsToLines(labels: Record<string, string> | undefined): string {
  return Object.entries(labels ?? {})
    .map(([k, v]) => (v ? `${k}=${v}` : k))
    .join("\n");
}
