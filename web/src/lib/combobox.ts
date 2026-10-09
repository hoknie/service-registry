export type ComboOption = { value: string; label: string; detail?: string; keywords?: string[] };

export type ComboMatch<T extends ComboOption> = { option: T; start: number; end: number };

export function filterOptions<T extends ComboOption>(options: T[], query: string, limit = 50): ComboMatch<T>[] {
  const q = query.trim().toLocaleLowerCase();
  const out: ComboMatch<T>[] = [];
  for (const option of options) {
    if (!q) {
      out.push({ option, start: -1, end: -1 });
    } else {
      const at = option.label.toLocaleLowerCase().indexOf(q);
      if (at >= 0) out.push({ option, start: at, end: at + q.length });
      else if ([option.detail ?? "", ...(option.keywords ?? [])].some((k) => k.toLocaleLowerCase().includes(q))) out.push({ option, start: -1, end: -1 });
    }
    if (out.length >= limit) break;
  }
  return out;
}

export function splitHighlight(text: string, start: number, end: number): [string, string, string] {
  if (start < 0 || end <= start) return [text, "", ""];
  return [text.slice(0, start), text.slice(start, end), text.slice(end)];
}
